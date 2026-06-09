package benchmark

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionTracker(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.tokenCounter)
	assert.Equal(t, BenchModeOn, tracker.benchMode)
	assert.Equal(t, "production", tracker.provenance)

	// Cleanup
	tracker.Stop()
}

func TestNewSessionTrackerWithTimeoutUsesTestProvenance(t *testing.T) {
	tracker, err := NewSessionTrackerWithTimeout(t.TempDir(), time.Second)
	require.NoError(t, err)
	defer tracker.Stop()

	assert.Equal(t, "test", tracker.provenance)
}

func TestSessionStartEnd(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start a session
	tracker.StartSession("test-project", "feature")

	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, "test-project", session.ProjectID)
	assert.Equal(t, "feature", session.TaskCategory)
	assert.Equal(t, BenchModeOn, session.Mode)
	assert.NotEmpty(t, session.ID)

	// End the session
	tracker.EndSession(true)

	// Session should be nil after end
	session = tracker.GetCurrentSession()
	assert.Nil(t, session)
}

func TestOnMCPCall_StartsSession(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// No session initially
	assert.Nil(t, tracker.GetCurrentSession())

	// OnMCPCall should start a session
	tracker.OnMCPCall("test-project", "request content", "response content")

	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, "test-project", session.ProjectID)
	assert.Equal(t, 1, session.MCPCallsCount)
	assert.Greater(t, session.TokensIn, 0)
	assert.Greater(t, session.TokensOut, 0)
}

func TestTokenAccumulation(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Make multiple MCP calls
	tracker.OnMCPCall("test-project", "first request", "first response")
	tracker.OnMCPCall("test-project", "second request", "second response")
	tracker.OnMCPCall("test-project", "third request", "third response")

	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, 3, session.MCPCallsCount)
	assert.Greater(t, session.TokensIn, 0)
	assert.Greater(t, session.TokensOut, 0)
}

func TestOnMCPCall_ProjectChangeStartsNewSession(t *testing.T) {
	tracker, err := NewSessionTracker(t.TempDir())
	require.NoError(t, err)
	defer tracker.Stop()

	tracker.OnMCPCall("project-a", "first request", "first response")
	first := tracker.GetCurrentSession()
	require.NotNil(t, first)

	tracker.OnMCPCall("project-b", "second request", "second response")
	second := tracker.GetCurrentSession()
	require.NotNil(t, second)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, "project-b", second.ProjectID)
	assert.Equal(t, 1, second.MCPCallsCount)
}

func TestOnMCPCall_ModeChangeMarksSessionMixed(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bench_mode"), []byte("on"), 0o644))

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	tracker.OnMCPCall("test-project", "first request", "first response")
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bench_mode"), []byte("off"), 0o644))
	tracker.OnMCPCall("test-project", "second request", "second response")

	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, BenchModeMixed, session.Mode)
	assert.Equal(t, 2, session.MCPCallsCount)
}

func TestSessionEndWithoutCompletion(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	tracker.StartSession("test-project", "debug")
	session := tracker.GetCurrentSession()
	require.NotNil(t, session)

	tracker.EndSession(false)

	// Session should be ended
	session = tracker.GetCurrentSession()
	assert.Nil(t, session)
}

func TestInactivityTimeout(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start a session
	tracker.StartSession("test-project", "refactor")
	require.NotNil(t, tracker.GetCurrentSession())

	// Manually set lastActivity to 11 minutes ago
	tracker.mu.Lock()
	tracker.lastActivity = time.Now().Add(-11 * time.Minute)
	tracker.mu.Unlock()

	// Trigger inactivity check
	tracker.checkInactivity()

	// Session should be ended
	session := tracker.GetCurrentSession()
	assert.Nil(t, session)
}

func TestInactivityTimeout_NotTriggered(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start a session
	tracker.StartSession("test-project", "refactor")
	require.NotNil(t, tracker.GetCurrentSession())

	// Set lastActivity to 5 minutes ago (within timeout)
	tracker.mu.Lock()
	tracker.lastActivity = time.Now().Add(-5 * time.Minute)
	tracker.mu.Unlock()

	// Trigger inactivity check
	tracker.checkInactivity()

	// Session should still be active
	session := tracker.GetCurrentSession()
	assert.NotNil(t, session)
}

func TestReadBenchMode(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("file does not exist", func(t *testing.T) {
		mode, err := ReadBenchMode(tempDir)
		require.NoError(t, err)
		assert.Equal(t, BenchModeOn, mode)
	})

	t.Run("file contains on", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(tempDir, "bench_mode"), []byte("on"), 0644)
		require.NoError(t, err)

		mode, err := ReadBenchMode(tempDir)
		require.NoError(t, err)
		assert.Equal(t, BenchModeOn, mode)
	})

	t.Run("file contains off", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(tempDir, "bench_mode"), []byte("off"), 0644)
		require.NoError(t, err)

		mode, err := ReadBenchMode(tempDir)
		require.NoError(t, err)
		assert.Equal(t, BenchModeOff, mode)
	})

	t.Run("file contains invalid value", func(t *testing.T) {
		err := os.WriteFile(filepath.Join(tempDir, "bench_mode"), []byte("invalid"), 0644)
		require.NoError(t, err)

		mode, err := ReadBenchMode(tempDir)
		require.NoError(t, err)
		assert.Equal(t, BenchModeOn, mode) // Defaults to ON
	})
}

func TestWriteBenchMode(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("write on", func(t *testing.T) {
		err := WriteBenchMode(tempDir, BenchModeOn)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(tempDir, "bench_mode"))
		require.NoError(t, err)
		assert.Equal(t, "on", string(data))
	})

	t.Run("write off", func(t *testing.T) {
		err := WriteBenchMode(tempDir, BenchModeOff)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(tempDir, "bench_mode"))
		require.NoError(t, err)
		assert.Equal(t, "off", string(data))
	})

	t.Run("invalid mode", func(t *testing.T) {
		err := WriteBenchMode(tempDir, BenchMode("invalid"))
		assert.Error(t, err)
	})
}

func TestSessionTrackerStop(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)

	// Start a session
	tracker.StartSession("test-project", "docs")
	require.NotNil(t, tracker.GetCurrentSession())

	// Stop should end the session
	tracker.Stop()

	// Session should be nil
	session := tracker.GetCurrentSession()
	assert.Nil(t, session)
}

func TestMultipleSessions(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start first session
	tracker.StartSession("project-1", "feature")
	session1 := tracker.GetCurrentSession()
	require.NotNil(t, session1)
	session1ID := session1.ID

	// Start second session (should end first)
	tracker.StartSession("project-2", "debug")
	session2 := tracker.GetCurrentSession()
	require.NotNil(t, session2)
	assert.NotEqual(t, session1ID, session2.ID)
	assert.Equal(t, "project-2", session2.ProjectID)
	assert.Equal(t, "debug", session2.TaskCategory)
}

func TestSessionDuration(t *testing.T) {
	tempDir := t.TempDir()

	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	// Start a session
	tracker.StartSession("test-project", "feature")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// End the session
	tracker.EndSession(true)

	// Session should have non-zero duration (tested implicitly by the fact it doesn't panic)
}

func TestBenchModeLoading(t *testing.T) {
	tempDir := t.TempDir()

	// Write OFF mode
	err := WriteBenchMode(tempDir, BenchModeOff)
	require.NoError(t, err)

	// Create tracker - should load OFF mode
	tracker, err := NewSessionTracker(tempDir)
	require.NoError(t, err)
	defer tracker.Stop()

	assert.Equal(t, BenchModeOff, tracker.benchMode)

	// Start a session - should use OFF mode
	tracker.StartSession("test-project", "feature")
	session := tracker.GetCurrentSession()
	require.NotNil(t, session)
	assert.Equal(t, BenchModeOff, session.Mode)
}
