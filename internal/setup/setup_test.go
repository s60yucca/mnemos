package setup_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestMergeMarkdownFile_NewFile verifies that MergeMarkdownFile creates the file when it doesn't exist.
func TestMergeMarkdownFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	w := setup.NewWriter(dir, false, false)
	appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "mnemos_store")
	require.NoError(t, err)
	assert.True(t, appended)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(content), "mnemos_store")
}

// TestMergeMarkdownFile_AppendsWhenMarkerAbsent verifies that existing CLAUDE.md gets the section appended.
func TestMergeMarkdownFile_AppendsWhenMarkerAbsent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	existing := "# My Project\n\nSome existing content.\n"
	require.NoError(t, os.WriteFile(target, []byte(existing), 0o644))

	w := setup.NewWriter(dir, false, false)
	appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "mnemos_store")
	require.NoError(t, err)
	assert.True(t, appended)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	// Original content preserved
	assert.Contains(t, string(content), "My Project")
	assert.Contains(t, string(content), "Some existing content.")
	// Mnemos section appended
	assert.Contains(t, string(content), "mnemos_store")
}

// TestMergeMarkdownFile_IdempotentWhenMarkerPresent verifies that running setup twice doesn't duplicate the section.
func TestMergeMarkdownFile_IdempotentWhenMarkerPresent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	existing := "# My Project\n\n# Memory Integration\nmnemos_store\n"
	require.NoError(t, os.WriteFile(target, []byte(existing), 0o644))

	w := setup.NewWriter(dir, false, false)
	appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "mnemos_store")
	require.NoError(t, err)
	assert.False(t, appended, "should not append when marker already present")

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	// Content unchanged
	assert.Equal(t, existing, string(content))
	// Only one occurrence of the marker
	assert.Equal(t, 1, strings.Count(string(content), "mnemos_store"))
}
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

// TestMergeClaudeSettings_NewFile verifies that MergeClaudeSettings creates the file when it doesn't exist.
func TestMergeClaudeSettings_NewFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "settings.json")

	require.NoError(t, setup.MergeClaudeSettings(filePath, "/usr/local/bin/mnemos"))

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &root))

	var hooks map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(root["hooks"], &hooks))

	assert.Contains(t, hooks, "SessionStart")
	assert.Contains(t, hooks, "UserPromptSubmit")
	assert.Contains(t, hooks, "SessionEnd")
}

// TestMergeClaudeSettings_PreservesExistingHooks verifies that existing hooks are not removed.
func TestMergeClaudeSettings_PreservesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "settings.json")

	existing := `{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "my-existing-hook"}]}]
  }
}`
	require.NoError(t, os.WriteFile(filePath, []byte(existing), 0o644))

	require.NoError(t, setup.MergeClaudeSettings(filePath, "/usr/local/bin/mnemos"))

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Both the existing hook and the mnemos hook should be present
	assert.Contains(t, string(data), "my-existing-hook")
	assert.Contains(t, string(data), "hook session-start")
}

// TestMergeClaudeSettings_Idempotent verifies that running setup twice doesn't duplicate hooks.
func TestMergeClaudeSettings_Idempotent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "settings.json")

	require.NoError(t, setup.MergeClaudeSettings(filePath, "/usr/local/bin/mnemos"))
	require.NoError(t, setup.MergeClaudeSettings(filePath, "/usr/local/bin/mnemos"))

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Each mnemos command should appear exactly once
	assert.Equal(t, 1, strings.Count(string(data), "hook session-start"))
	assert.Equal(t, 1, strings.Count(string(data), "hook prompt-submit"))
	assert.Equal(t, 1, strings.Count(string(data), "hook session-end"))
}
