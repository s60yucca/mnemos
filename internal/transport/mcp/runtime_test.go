package mcp

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeInfoReportsLiveServerIdentity(t *testing.T) {
	t.Setenv("MNEMOS_PROJECT_ID", "Runtime Project")

	server := NewServer(nil, "test-version")
	server.startedAt = time.Now().UTC().Add(-2 * time.Second)
	t.Cleanup(server.Shutdown)

	info := server.runtimeInfo()

	assert.Equal(t, "test-version", info.Version)
	assert.Equal(t, os.Getpid(), info.PID)
	assert.Equal(t, os.Getppid(), info.PPID)
	assert.NotEmpty(t, info.Host)
	assert.NotEmpty(t, info.StartedAt)
	assert.NotEmpty(t, info.Executable)
	assert.NotEmpty(t, info.DataDir)
	assert.Equal(t, "runtime-project", info.ProjectID)
	assert.Equal(t, "env_var", info.ProjectStrategy)
	assert.Equal(t, "Runtime Project", info.EnvProjectID)
	assert.GreaterOrEqual(t, info.UptimeSeconds, int64(1))

	_, err := time.Parse(time.RFC3339, info.StartedAt)
	require.NoError(t, err)
}
