package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionTracker_Integration verifies that SessionTracker is called on MCP operations
func TestSessionTracker_Integration(t *testing.T) {
	server, _, _ := newTestServer(t)
	ctx := context.Background()

	// Verify session tracker was created
	require.NotNil(t, server.sessionTracker, "SessionTracker should be initialized")

	// Store a memory - this should trigger session tracking
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Test memory for session tracking",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify session was started
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session, "Session should be started after first MCP call")
	assert.Equal(t, "test-project", session.ProjectID)
	assert.Equal(t, 1, session.MCPCallsCount)
	assert.Greater(t, session.TokensIn, 0, "Should have counted input tokens")
	assert.Greater(t, session.TokensOut, 0, "Should have counted output tokens")

	// Make another call - should accumulate in same session
	_, err = server.handleSearch(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "test query",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Verify session accumulated
	session = server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, 2, session.MCPCallsCount, "Should have 2 MCP calls")

	// Make a context call
	_, err = server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "context query",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Verify session accumulated again
	session = server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, 3, session.MCPCallsCount, "Should have 3 MCP calls")
}

// TestSessionTracker_TokenCounting verifies token counting works
func TestSessionTracker_TokenCounting(t *testing.T) {
	server, _, _ := newTestServer(t)
	ctx := context.Background()

	// Store a memory with known content
	content := "This is a test memory with some content that should be tokenized"
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    content,
				"project_id": "token-test",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify tokens were counted
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Greater(t, session.TokensIn, 0, "Input tokens should be > 0")
	assert.Greater(t, session.TokensOut, 0, "Output tokens should be > 0")

	// Tokens should be roughly proportional to content length
	// (tiktoken typically gives ~1 token per 4 characters for English)
	expectedMinTokens := len(content) / 6 // Conservative estimate
	assert.Greater(t, session.TokensIn, expectedMinTokens, "Token count should be reasonable")
}

// TestSessionTracker_MultipleProjects verifies session tracking across projects
func TestSessionTracker_MultipleProjects(t *testing.T) {
	server, _, _ := newTestServer(t)
	ctx := context.Background()

	// First call with project A
	_, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Memory for project A",
				"project_id": "project-a",
			},
		},
	})
	require.NoError(t, err)

	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	firstSessionID := session.ID
	assert.Equal(t, "project-a", session.ProjectID)

	// Second call with same project - should be same session
	_, err = server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Another memory for project A",
				"project_id": "project-a",
			},
		},
	})
	require.NoError(t, err)

	session = server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, firstSessionID, session.ID, "Should be same session")
	assert.Equal(t, 2, session.MCPCallsCount)
}

// TestSessionTracker_BenchModeOFF verifies tracking works in OFF mode
func TestSessionTracker_BenchModeOFF(t *testing.T) {
	server, mn, dataDir := newTestServer(t)
	ctx := context.Background()

	// Set bench mode to OFF by writing to disk (tests the real code path)
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff))

	// Store should still work and track
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Memory in OFF mode",
				"project_id": "off-project",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify session tracking still works
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session, "Session tracking should work in OFF mode")
	assert.Equal(t, "off-project", session.ProjectID)

	// Verify memory was stored with bench_off_day tag
	var storeResult domain.StoreResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &storeResult))
	require.True(t, storeResult.Created)

	mem, err := mn.Get(ctx, storeResult.Memory.ID)
	require.NoError(t, err)
	assert.Contains(t, mem.Tags, "bench_off_day", "Should have bench_off_day tag in OFF mode")

	// Context should return empty but still track
	contextResult, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "test query",
				"project_id": "off-project",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, contextResult.IsError)

	// Verify empty result
	var contextData map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolText(t, contextResult)), &contextData))
	memories := contextData["memories"].([]any)
	assert.Empty(t, memories, "Context should return empty in OFF mode")

	// But session should still track the call
	session = server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, 2, session.MCPCallsCount, "Should track both store and context calls")
}

// TestSessionTracker_AllHandlers verifies all handlers call trackMCPCall
func TestSessionTracker_AllHandlers(t *testing.T) {
	server, _, _ := newTestServer(t)
	ctx := context.Background()

	// Store a memory first
	storeResult, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Test memory",
				"project_id": "handler-test",
			},
		},
	})
	require.NoError(t, err)

	var stored domain.StoreResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, storeResult)), &stored))
	memID := stored.Memory.ID

	// Track initial call count
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	initialCount := session.MCPCallsCount

	// Test each handler
	handlers := []struct {
		name string
		call func() error
	}{
		{
			name: "handleSearch",
			call: func() error {
				_, err := server.handleSearch(ctx, mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Arguments: map[string]any{
							"query":      "test",
							"project_id": "handler-test",
						},
					},
				})
				return err
			},
		},
		{
			name: "handleContext",
			call: func() error {
				_, err := server.handleContext(ctx, mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Arguments: map[string]any{
							"query":      "test",
							"project_id": "handler-test",
						},
					},
				})
				return err
			},
		},
		{
			name: "handleGet",
			call: func() error {
				_, err := server.handleGet(ctx, mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Arguments: map[string]any{"id": memID},
					},
				})
				return err
			},
		},
		{
			name: "handleUpdate",
			call: func() error {
				_, err := server.handleUpdate(ctx, mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Arguments: map[string]any{
							"id":      memID,
							"content": "Updated content",
						},
					},
				})
				return err
			},
		},
		{
			name: "handleMaintain",
			call: func() error {
				_, err := server.handleMaintain(ctx, mcp.CallToolRequest{
					Params: mcp.CallToolParams{
						Arguments: map[string]any{"project_id": "handler-test"},
					},
				})
				return err
			},
		},
	}

	for i, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			err := h.call()
			require.NoError(t, err)

			session := server.sessionTracker.GetCurrentSession()
			require.NotNil(t, session)
			expectedCount := initialCount + i + 1
			assert.Equal(t, expectedCount, session.MCPCallsCount,
				"%s should increment MCP call count", h.name)
		})
	}
}

// TestSessionTracker_SessionDuration verifies session duration tracking
func TestSessionTracker_SessionDuration(t *testing.T) {
	server, _, _ := newTestServer(t)
	ctx := context.Background()

	// Record start time
	startTime := time.Now()

	// Make a call to start session
	_, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Test memory",
				"project_id": "duration-test",
			},
		},
	})
	require.NoError(t, err)

	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)

	// Verify start time is recent
	assert.WithinDuration(t, startTime, session.StartTime, 1*time.Second,
		"Session start time should be close to when first call was made")

	// Session should not have end time yet
	assert.True(t, session.EndTime.IsZero(), "Session should not have end time while active")
}

// TestMCPSessionIntegration_EventsEmittedToLog verifies that session_start and
// session_end events are emitted to features.log (Task 5.3).
func TestMCPSessionIntegration_EventsEmittedToLog(t *testing.T) {
	// Setup temp home directory for features.log
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	// Create log directory
	logDir := filepath.Join(testHome, ".mnemos", "logs")
	require.NoError(t, os.MkdirAll(logDir, 0755))
	logPath := filepath.Join(logDir, "features.log")

	// Create test server
	server, _, _ := newTestServer(t)
	defer server.Shutdown()

	ctx := context.Background()

	// Make first MCP call - should trigger session_start event
	_, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Test memory for session start event",
				"project_id": "event-test",
			},
		},
	})
	require.NoError(t, err)

	// Get session ID
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	sessionID := session.ID

	// Make more calls to accumulate tokens
	_, err = server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Second memory",
				"project_id": "event-test",
			},
		},
	})
	require.NoError(t, err)

	_, err = server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "test query",
				"project_id": "event-test",
			},
		},
	})
	require.NoError(t, err)

	// End session explicitly
	server.Shutdown()

	// Give a moment for async logging
	time.Sleep(100 * time.Millisecond)

	// Read features.log
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Verify session_start event
	var sessionStartFound bool
	for _, line := range lines {
		if strings.Contains(line, "bench_session_start") && strings.Contains(line, "session_id="+sessionID) {
			sessionStartFound = true

			// Verify line format: timestamp\tfeature_name\tattributes
			parts := strings.Split(line, "\t")
			require.Len(t, parts, 3, "session_start event should have 3 parts")

			// Verify timestamp is valid RFC3339
			_, err := time.Parse(time.RFC3339, parts[0])
			assert.NoError(t, err, "timestamp should be valid RFC3339")

			// Verify feature name
			assert.Equal(t, "bench_session_start", parts[1])

			// Verify attributes
			attrs := parts[2]
			assert.Contains(t, attrs, "session_id="+sessionID)
			assert.Contains(t, attrs, "project_id=event-test")
			assert.Contains(t, attrs, "mode=on")
			break
		}
	}
	assert.True(t, sessionStartFound, "session_start event should be emitted to features.log")

	// Verify session_end event
	var sessionEndFound bool
	var tokensIn, tokensOut, mcpCalls int
	for _, line := range lines {
		if strings.Contains(line, "bench_session_end") && strings.Contains(line, "session_id="+sessionID) {
			sessionEndFound = true

			// Verify line format
			parts := strings.Split(line, "\t")
			require.Len(t, parts, 3, "session_end event should have 3 parts")

			// Verify timestamp is valid RFC3339
			_, err := time.Parse(time.RFC3339, parts[0])
			assert.NoError(t, err, "timestamp should be valid RFC3339")

			// Verify feature name
			assert.Equal(t, "bench_session_end", parts[1])

			// Verify attributes
			attrs := parts[2]
			assert.Contains(t, attrs, "session_id="+sessionID)
			assert.Contains(t, attrs, "duration_ms=")
			assert.Contains(t, attrs, "tokens_in=")
			assert.Contains(t, attrs, "tokens_out=")
			assert.Contains(t, attrs, "mcp_calls_count=")
			assert.Contains(t, attrs, "task_completed=")

			// Extract token counts and mcp_calls_count
			for _, attr := range strings.Split(attrs, " ") {
				if strings.HasPrefix(attr, "tokens_in=") {
					fmt.Sscanf(attr, "tokens_in=%d", &tokensIn)
				}
				if strings.HasPrefix(attr, "tokens_out=") {
					fmt.Sscanf(attr, "tokens_out=%d", &tokensOut)
				}
				if strings.HasPrefix(attr, "mcp_calls_count=") {
					fmt.Sscanf(attr, "mcp_calls_count=%d", &mcpCalls)
				}
			}
			break
		}
	}
	assert.True(t, sessionEndFound, "session_end event should be emitted to features.log")
	assert.Equal(t, 3, mcpCalls, "should have 3 MCP calls")
	assert.Greater(t, tokensIn, 0, "tokens_in should be > 0")
	assert.Greater(t, tokensOut, 0, "tokens_out should be > 0")
}

// TestMCPSessionIntegration_InactivityTimeout verifies that session_end event
// is emitted after inactivity timeout (Task 5.3).
func TestMCPSessionIntegration_InactivityTimeout(t *testing.T) {
	// Setup temp home directory for features.log
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	// Create log directory
	logDir := filepath.Join(testHome, ".mnemos", "logs")
	require.NoError(t, os.MkdirAll(logDir, 0755))
	logPath := filepath.Join(logDir, "features.log")

	// Create temp data directory
	dataDir := t.TempDir()

	// Create session tracker with 1-second timeout for testing
	tracker, err := benchmark.NewSessionTrackerWithTimeout(dataDir, 1*time.Second)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start a session
	tracker.StartSession("timeout-test", "feature")

	// Make an MCP call to track activity
	tracker.OnMCPCall("timeout-test", "test request", "test response")

	// Verify session is active
	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	sessionID := session.ID

	// Wait for inactivity timeout (1 second + buffer)
	time.Sleep(1500 * time.Millisecond)

	// Verify session ended
	session = tracker.GetCurrentSession()
	assert.Nil(t, session, "session should be ended after inactivity timeout")

	// Read features.log
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Find session_end event
	var sessionEndFound bool
	for _, line := range lines {
		if strings.Contains(line, "bench_session_end") && strings.Contains(line, "session_id="+sessionID) {
			sessionEndFound = true

			// Verify attributes
			attrs := strings.Split(line, "\t")[2]
			assert.Contains(t, attrs, "session_id="+sessionID)
			assert.Contains(t, attrs, "task_completed=true", "should be marked as completed after timeout")
			break
		}
	}

	assert.True(t, sessionEndFound, "session_end event should be emitted after inactivity timeout")
}
