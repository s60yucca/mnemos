package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigPrefersProjectLocalConfig(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".mnemos"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".mnemos"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, ".mnemos", "config.yaml"),
		[]byte("data_dir: /tmp/project-mnemos\n"),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(homeDir, ".mnemos", "config.yaml"),
		[]byte("data_dir: /tmp/global-mnemos\n"),
		0o644,
	))

	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	t.Setenv("HOME", homeDir)

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/project-mnemos", cfg.DataDir)
}
