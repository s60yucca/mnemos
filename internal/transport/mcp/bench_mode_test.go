package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServerWithBenchMode creates a test MCP server with specified bench mode
func newTestServerWithBenchMode(t *testing.T, benchMode benchmark.BenchMode) (*Server, string) {
	t.Helper()

	// Create test server
	server, _, _ := newTestServer(t)

	// Write bench mode to the server's dataDir so currentBenchMode() picks it up
	err := benchmark.WriteBenchMode(server.dataDir, benchMode)
	require.NoError(t, err)

	return server, server.dataDir
}

// TestModeOFF_ContextReturnsEmpty verifies that mnemos_context returns empty results when mode is OFF
func TestModeOFF_ContextReturnsEmpty(t *testing.T) {
	server, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOff)
	ctx := context.Background()

	// Store a memory first via MCP handler (still works in OFF mode)
	_, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "JWT authentication uses tokens in auth/jwt.go",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Call mnemos_context - should return empty results
	result, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "JWT authentication",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Parse response
	var contextResult map[string]any
	err = json.Unmarshal([]byte(toolText(t, result)), &contextResult)
	require.NoError(t, err)

	// Verify empty results
	memories, ok := contextResult["memories"].([]any)
	require.True(t, ok)
	assert.Empty(t, memories, "context should return empty memories in OFF mode")

	totalTokens, ok := contextResult["total_tokens"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(0), totalTokens, "total_tokens should be 0 in OFF mode")
}

// TestModeOFF_SearchReturnsEmpty verifies that mnemos_search returns empty results when mode is OFF
func TestModeOFF_SearchReturnsEmpty(t *testing.T) {
	server, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOff)
	ctx := context.Background()

	// Store a memory first via MCP handler
	_, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Session management in auth/session.go",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Call mnemos_search - should return empty results
	result, err := server.handleSearch(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "session",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	// Parse response
	var searchResults []any
	err = json.Unmarshal([]byte(toolText(t, result)), &searchResults)
	require.NoError(t, err)

	// Verify empty results
	assert.Empty(t, searchResults, "search should return empty results in OFF mode")
}

// TestModeOFF_StoreStillWorks verifies that mnemos_store STILL STORES with "bench_off_day" tag when mode is OFF
func TestModeOFF_StoreStillWorks(t *testing.T) {
	server, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOff)
	ctx := context.Background()

	// Call mnemos_store in OFF mode
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Database connection pooling in db/pool.go requires tuning",
				"project_id": "test-project",
				"tags":       "database,performance",
			},
		},
	})
	require.NoError(t, err)

	// Parse response
	var storeResult domain.StoreResult
	err = json.Unmarshal([]byte(toolText(t, result)), &storeResult)
	require.NoError(t, err)

	// Verify memory was created
	assert.True(t, storeResult.Created, "memory should be created even in OFF mode")
	assert.NotEmpty(t, storeResult.Memory.ID)

	// Verify memory exists in database and has bench_off_day tag
	stored, err := server.mnemos.Get(ctx, storeResult.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, "Database connection pooling in db/pool.go requires tuning", stored.Content)

	// CRITICAL: Verify "bench_off_day" tag was added
	assert.True(t, stored.HasTag("bench_off_day"), "memory should have bench_off_day tag in OFF mode")

	// Verify original tags are preserved
	assert.True(t, stored.HasTag("database"), "original tags should be preserved")
	assert.True(t, stored.HasTag("performance"), "original tags should be preserved")
}

// TestModeON_NormalBehavior verifies that MCP calls return normal results when mode is ON
func TestModeON_NormalBehavior(t *testing.T) {
	server, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOn)
	ctx := context.Background()

	// Store a memory via MCP handler
	storeResult, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "JWT authentication uses tokens in auth/jwt.go",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, storeResult.IsError)

	// Test mnemos_context returns results
	contextResult, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "JWT authentication",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var contextData map[string]any
	err = json.Unmarshal([]byte(toolText(t, contextResult)), &contextData)
	require.NoError(t, err)

	memories, ok := contextData["memories"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, memories, "context should return memories in ON mode")

	// Test mnemos_search returns results
	searchResult, err := server.handleSearch(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "JWT",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var searchData []*storage.SearchResult
	err = json.Unmarshal([]byte(toolText(t, searchResult)), &searchData)
	require.NoError(t, err)
	assert.NotEmpty(t, searchData, "search should return results in ON mode")
}

// TestModeON_StoreNoBenchTag verifies that mnemos_store does NOT add "bench_off_day" tag when mode is ON
func TestModeON_StoreNoBenchTag(t *testing.T) {
	server, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOn)
	ctx := context.Background()

	// Call mnemos_store in ON mode
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Cache invalidation strategy in cache/redis.go",
				"project_id": "test-project",
				"tags":       "cache,redis",
			},
		},
	})
	require.NoError(t, err)

	// Parse response
	var storeResult domain.StoreResult
	err = json.Unmarshal([]byte(toolText(t, result)), &storeResult)
	require.NoError(t, err)

	// Verify memory was created
	assert.True(t, storeResult.Created)

	// Verify memory exists in database
	stored, err := server.mnemos.Get(ctx, storeResult.Memory.ID)
	require.NoError(t, err)

	// Verify NO "bench_off_day" tag in ON mode
	assert.False(t, stored.HasTag("bench_off_day"), "memory should NOT have bench_off_day tag in ON mode")

	// Verify original tags are preserved
	assert.True(t, stored.HasTag("cache"))
	assert.True(t, stored.HasTag("redis"))
}

// TestModePersistence verifies that bench mode persists across server restarts
func TestModePersistence(t *testing.T) {
	dataDir := t.TempDir()

	// Write OFF mode
	err := benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff)
	require.NoError(t, err)

	// Read back - should be OFF
	mode, err := benchmark.ReadBenchMode(dataDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOff, mode)

	// Write ON mode
	err = benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOn)
	require.NoError(t, err)

	// Read back - should be ON
	mode, err = benchmark.ReadBenchMode(dataDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOn, mode)

	// Verify file content
	content, err := os.ReadFile(filepath.Join(dataDir, "bench_mode"))
	require.NoError(t, err)
	assert.Equal(t, "on", string(content))
}

// TestModeChangeEventEmission verifies that mode changes can be detected
func TestModeChangeEventEmission(t *testing.T) {
	// This test verifies the mode change logic
	// In practice, mode change events are emitted by the bench CLI command
	// Here we verify the WriteBenchMode function works correctly

	dataDir := t.TempDir()

	// Change from default (ON) to OFF
	err := benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff)
	require.NoError(t, err)

	mode, err := benchmark.ReadBenchMode(dataDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOff, mode)

	// Change from OFF to ON
	err = benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOn)
	require.NoError(t, err)

	mode, err = benchmark.ReadBenchMode(dataDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOn, mode)
}

// TestModeChange_NoRestart verifies that bench mode changes take effect immediately
// on the SAME running server without requiring a restart.
// This is the fix for the bench mode caching bug discovered in Day 1 dogfood.
func TestModeChange_NoRestart(t *testing.T) {
	// Start server with mode ON
	server, dataDir := newTestServerWithBenchMode(t, benchmark.BenchModeOn)
	ctx := context.Background()

	// Store a memory while ON
	storeResult, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Important architecture decision",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, storeResult.IsError)

	// Context should return results in ON mode
	ctxResult1, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "architecture",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	var data1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolText(t, ctxResult1)), &data1))
	memories1, _ := data1["memories"].([]any)
	assert.NotEmpty(t, memories1, "ON mode: context should return memories")

	// --- Switch mode to OFF without restarting the server ---
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff))

	// Context should now return empty on the SAME server instance
	ctxResult2, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "architecture",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	var data2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolText(t, ctxResult2)), &data2))
	memories2, _ := data2["memories"].([]any)
	assert.Empty(t, memories2, "OFF mode: context should return empty without restart")

	// --- Switch back to ON ---
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOn))

	// Context should return results again on the SAME server instance
	ctxResult3, err := server.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "architecture",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)
	var data3 map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolText(t, ctxResult3)), &data3))
	memories3, _ := data3["memories"].([]any)
	assert.NotEmpty(t, memories3, "ON mode again: context should return memories without restart")
}

// TestModeToggle_EndToEnd simulates a complete benchmark workflow
func TestModeToggle_EndToEnd(t *testing.T) {
	// Day 1: Mode ON
	server1, dataDir := newTestServerWithBenchMode(t, benchmark.BenchModeOn)
	ctx := context.Background()

	// Store memory on Day 1 (ON mode)
	result1, err := server1.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "API rate limiting in api/middleware.go",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var storeResult1 domain.StoreResult
	err = json.Unmarshal([]byte(toolText(t, result1)), &storeResult1)
	require.NoError(t, err)
	assert.True(t, storeResult1.Created)

	// Verify no bench_off_day tag
	mem1, err := server1.mnemos.Get(ctx, storeResult1.Memory.ID)
	require.NoError(t, err)
	assert.False(t, mem1.HasTag("bench_off_day"))

	// Context should return results
	contextResult1, err := server1.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "rate limiting",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var contextData1 map[string]any
	err = json.Unmarshal([]byte(toolText(t, contextResult1)), &contextData1)
	require.NoError(t, err)
	memories1, _ := contextData1["memories"].([]any)
	assert.NotEmpty(t, memories1, "ON mode should return memories")

	// Day 2: Mode OFF (simulate server restart with new mode)
	err = benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff)
	require.NoError(t, err)

	// Create new server with OFF mode
	server2, _ := newTestServerWithBenchMode(t, benchmark.BenchModeOff)

	// Store memory on Day 2 (OFF mode)
	result2, err := server2.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"content":    "Logging configuration in config/logger.go",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var storeResult2 domain.StoreResult
	err = json.Unmarshal([]byte(toolText(t, result2)), &storeResult2)
	require.NoError(t, err)
	assert.True(t, storeResult2.Created, "store should work in OFF mode")

	// Verify bench_off_day tag was added
	mem2, err := server2.mnemos.Get(ctx, storeResult2.Memory.ID)
	require.NoError(t, err)
	assert.True(t, mem2.HasTag("bench_off_day"), "OFF mode should add bench_off_day tag")

	// Context should return empty results
	contextResult2, err := server2.handleContext(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "logging",
				"project_id": "test-project",
			},
		},
	})
	require.NoError(t, err)

	var contextData2 map[string]any
	err = json.Unmarshal([]byte(toolText(t, contextResult2)), &contextData2)
	require.NoError(t, err)
	memories2, _ := contextData2["memories"].([]any)
	assert.Empty(t, memories2, "OFF mode should return empty memories")
}
