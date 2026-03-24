package setup_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriter_ForceOverwrite verifies that force=true overwrites an existing file.
func TestWriter_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")

	// Write initial content
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	w := setup.NewWriter(dir, false, true) // force=true
	written, err := w.WriteFile(target, "overwritten")
	require.NoError(t, err)
	assert.True(t, written)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "overwritten", string(content))
}

// TestWriter_ConfirmOverwrite verifies that force=false with a "y" response overwrites,
// and a "n" response does not overwrite. Uses a pipe to simulate stdin.
func TestWriter_ConfirmOverwrite(t *testing.T) {
	t.Run("confirm yes overwrites", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

		// Pipe "y\n" into stdin
		r, w, err := os.Pipe()
		require.NoError(t, err)
		_, err = io.WriteString(w, "y\n")
		require.NoError(t, err)
		w.Close()

		oldStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()

		writer := setup.NewWriter(dir, false, false) // force=false
		written, err := writer.WriteFile(target, "new content")
		r.Close()

		require.NoError(t, err)
		assert.True(t, written)

		content, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, "new content", string(content))
	})

	t.Run("confirm no does not overwrite", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

		// Pipe "n\n" into stdin
		r, w, err := os.Pipe()
		require.NoError(t, err)
		_, err = io.WriteString(w, "n\n")
		require.NoError(t, err)
		w.Close()

		oldStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = oldStdin }()

		writer := setup.NewWriter(dir, false, false) // force=false
		written, err := writer.WriteFile(target, "new content")
		r.Close()

		require.NoError(t, err)
		assert.False(t, written)

		content, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, "original", string(content))
	})
}

// TestMergeMCPConfig_NewFile verifies that MergeMCPConfig creates a new file with correct JSON.
func TestMergeMCPConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, ".mcp.json")

	entry := setup.MnemosMCPEntry{
		Command: "mnemos",
		Args:    []string{"hook"},
	}

	err := setup.MergeMCPConfig(filePath, "mnemos", entry)
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &root))

	rawServers, ok := root["mcpServers"]
	require.True(t, ok, "mcpServers key should exist")

	var servers map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rawServers, &servers))

	rawEntry, ok := servers["mnemos"]
	require.True(t, ok, "mnemos entry should exist")

	var got setup.MnemosMCPEntry
	require.NoError(t, json.Unmarshal(rawEntry, &got))
	assert.Equal(t, entry.Command, got.Command)
	assert.Equal(t, entry.Args, got.Args)
}

// TestMergeMCPConfig_ExistingFile verifies that MergeMCPConfig merges without duplicating entries.
func TestMergeMCPConfig_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, ".mcp.json")

	// Write an existing config with another server
	existing := `{
  "mcpServers": {
    "other-server": {"command": "other", "args": []}
  }
}`
	require.NoError(t, os.WriteFile(filePath, []byte(existing), 0o644))

	entry := setup.MnemosMCPEntry{
		Command: "mnemos",
		Args:    []string{"hook"},
	}

	// Call twice to verify idempotency
	require.NoError(t, setup.MergeMCPConfig(filePath, "mnemos", entry))
	require.NoError(t, setup.MergeMCPConfig(filePath, "mnemos", entry))

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &root))

	var servers map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(root["mcpServers"], &servers))

	// Both entries should exist
	assert.Contains(t, servers, "mnemos", "mnemos entry should be present")
	assert.Contains(t, servers, "other-server", "other-server should be preserved")

	// Exactly two entries — no duplicates
	assert.Len(t, servers, 2)
}


