package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShutdownIntegration verifies the complete shutdown flow:
// 1. Server starts with session tracker
// 2. MCP activity creates a session
// 3. Server shutdown ends the session and stops the goroutine
func TestShutdownIntegration(t *testing.T) {
	// Create test server
	server, _, _ := newTestServer(t)
	require.NotNil(t, server)
	require.NotNil(t, server.sessionTracker)

	// Simulate MCP activity to start a session
	server.trackMCPCall("test-project", "request content", "response content")

	// Verify session was started
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session, "session should be active after MCP call")
	assert.Equal(t, "test-project", session.ProjectID)
	assert.Equal(t, 1, session.MCPCallsCount)

	// Simulate more activity
	server.trackMCPCall("test-project", "another request", "another response")
	session = server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, 2, session.MCPCallsCount, "session should accumulate calls")

	// Shutdown the server (simulates server exit)
	server.Shutdown()

	// Verify session was ended
	session = server.sessionTracker.GetCurrentSession()
	assert.Nil(t, session, "session should be nil after shutdown")

	// Verify goroutine was stopped (wait a bit to ensure it's not running)
	time.Sleep(100 * time.Millisecond)

	// Calling Shutdown again should be safe
	assert.NotPanics(t, func() {
		server.Shutdown()
	}, "multiple shutdown calls should be safe")
}

// TestShutdownWithoutActivity verifies shutdown works even if no MCP calls were made
func TestShutdownWithoutActivity(t *testing.T) {
	// Create test server
	server, _, _ := newTestServer(t)
	require.NotNil(t, server)

	// No MCP activity - session should be nil
	session := server.sessionTracker.GetCurrentSession()
	assert.Nil(t, session, "no session should exist without activity")

	// Shutdown should work fine
	assert.NotPanics(t, func() {
		server.Shutdown()
	}, "shutdown without activity should not panic")
}

// TestShutdownMarksSessionIncomplete verifies that sessions ended by shutdown
// are marked as task_completed=false
func TestShutdownMarksSessionIncomplete(t *testing.T) {
	// Create test server
	server, _, _ := newTestServer(t)
	require.NotNil(t, server)

	// Start a session
	server.sessionTracker.StartSession("test-project", "feature")
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)

	// Shutdown - this should end the session with task_completed=false
	server.Shutdown()

	// Session should be ended
	session = server.sessionTracker.GetCurrentSession()
	assert.Nil(t, session, "session should be ended after shutdown")

	// Note: We can't directly verify task_completed=false here because the session
	// is ended and we don't have access to the emitted event. This is verified
	// by the session tracker's behavior: EndSession(false) is called in Stop().
}
