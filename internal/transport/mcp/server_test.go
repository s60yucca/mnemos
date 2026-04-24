package mcp

import (
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerShutdown(t *testing.T) {
	// Create a minimal mnemos instance (nil is acceptable for this test)
	var mnemos *core.Mnemos

	// Create server
	server := NewServer(mnemos, "test-version")
	require.NotNil(t, server)

	// Verify session tracker was created
	require.NotNil(t, server.sessionTracker)

	// Start a session via the session tracker
	server.sessionTracker.StartSession("test-project", "feature")
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session, "session should be active before shutdown")

	// Call Shutdown
	server.Shutdown()

	// Verify session was ended (marked as incomplete)
	session = server.sessionTracker.GetCurrentSession()
	assert.Nil(t, session, "session should be nil after shutdown")
}

func TestServerShutdown_NoSessionTracker(t *testing.T) {
	// Create a server with nil session tracker
	server := &Server{
		sessionTracker: nil,
	}

	// Shutdown should not panic
	assert.NotPanics(t, func() {
		server.Shutdown()
	})
}

func TestServerShutdown_StopsInactivityGoroutine(t *testing.T) {
	// Create a minimal mnemos instance
	var mnemos *core.Mnemos

	// Create server
	server := NewServer(mnemos, "test-version")
	require.NotNil(t, server)
	require.NotNil(t, server.sessionTracker)

	// Start a session
	server.sessionTracker.StartSession("test-project", "feature")

	// Call Shutdown
	server.Shutdown()

	// Wait a bit to ensure goroutine has stopped
	time.Sleep(100 * time.Millisecond)

	// Verify the ticker was stopped by checking that the session tracker's stopChan is closed
	// We can't directly check this, but we can verify that calling Stop again doesn't panic
	assert.NotPanics(t, func() {
		server.sessionTracker.Stop()
	})
}

func TestServerShutdown_WithActiveSession(t *testing.T) {
	// Create a minimal mnemos instance
	var mnemos *core.Mnemos

	// Create server
	server := NewServer(mnemos, "test-version")
	require.NotNil(t, server)

	// Simulate MCP activity
	server.trackMCPCall("test-project", "request content", "response content")

	// Verify session was started
	session := server.sessionTracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, "test-project", session.ProjectID)
	assert.Equal(t, 1, session.MCPCallsCount)

	// Shutdown
	server.Shutdown()

	// Session should be ended
	session = server.sessionTracker.GetCurrentSession()
	assert.Nil(t, session)
}

func TestServerShutdown_MultipleCallsAreSafe(t *testing.T) {
	// Create a minimal mnemos instance
	var mnemos *core.Mnemos

	// Create server
	server := NewServer(mnemos, "test-version")
	require.NotNil(t, server)

	// Call Shutdown multiple times - should not panic
	assert.NotPanics(t, func() {
		server.Shutdown()
		server.Shutdown()
		server.Shutdown()
	})
}

func TestServerCreation_WithBenchMode(t *testing.T) {
	tempDir := t.TempDir()

	// Write bench mode OFF
	err := benchmark.WriteBenchMode(tempDir, benchmark.BenchModeOff)
	require.NoError(t, err)

	// Create server (it should read the bench mode)
	// Note: NewServer uses getDataDir() which returns ~/.mnemos, not our tempDir
	// So this test verifies the server creation doesn't panic, but can't verify mode loading
	var mnemos *core.Mnemos
	server := NewServer(mnemos, "test-version")
	require.NotNil(t, server)

	// Cleanup
	server.Shutdown()
}
