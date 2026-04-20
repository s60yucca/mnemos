package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergeMarkdownFile_SentinelWrapping verifies that when MergeMarkdownFile appends
// content to an existing file, the injected content is wrapped in sentinel markers.
func TestMergeMarkdownFile_SentinelWrapping(t *testing.T) {
	t.Run("appends to existing file with sentinel markers", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "CLAUDE.md")

		existing := "# My Project\n\nSome existing content.\n"
		require.NoError(t, os.WriteFile(target, []byte(existing), 0o644))

		w := setup.NewWriter(dir, false, false)
		appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "<!-- mnemos:begin -->")
		require.NoError(t, err)
		assert.True(t, appended)

		content, err := os.ReadFile(target)
		require.NoError(t, err)
		s := string(content)

		// Original content preserved
		assert.Contains(t, s, "My Project")
		assert.Contains(t, s, "Some existing content.")

		// Sentinel markers present
		assert.Contains(t, s, "<!-- mnemos:begin -->")
		assert.Contains(t, s, "<!-- mnemos:end -->")

		// Template content inside the markers
		assert.Contains(t, s, "# Memory Integration")
		assert.Contains(t, s, "mnemos_store")

		// Markers appear in the correct order
		beginIdx := strings.Index(s, "<!-- mnemos:begin -->")
		endIdx := strings.Index(s, "<!-- mnemos:end -->")
		assert.Less(t, beginIdx, endIdx, "begin marker must come before end marker")
	})

	t.Run("creates new file with sentinel markers", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "CLAUDE.md")

		w := setup.NewWriter(dir, false, false)
		appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "<!-- mnemos:begin -->")
		require.NoError(t, err)
		assert.True(t, appended)

		content, err := os.ReadFile(target)
		require.NoError(t, err)
		s := string(content)

		// Sentinel markers present
		assert.Contains(t, s, "<!-- mnemos:begin -->")
		assert.Contains(t, s, "<!-- mnemos:end -->")

		// Template content inside the markers
		assert.Contains(t, s, "# Memory Integration")
		assert.Contains(t, s, "mnemos_store")
	})
}

// TestMergeMarkdownFile_SentinelIdempotent verifies that when the file already contains
// the "<!-- mnemos:begin -->" marker, MergeMarkdownFile does not append again.
func TestMergeMarkdownFile_SentinelIdempotent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	existing := "# My Project\n\n<!-- mnemos:begin -->\n# Memory Integration\nmnemos_store\n<!-- mnemos:end -->\n"
	require.NoError(t, os.WriteFile(target, []byte(existing), 0o644))

	w := setup.NewWriter(dir, false, false)
	appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "<!-- mnemos:begin -->")
	require.NoError(t, err)
	assert.False(t, appended, "should not append when sentinel marker already present")

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	// File unchanged
	assert.Equal(t, existing, string(content))
	// Only one begin marker
	assert.Equal(t, 1, strings.Count(string(content), "<!-- mnemos:begin -->"))
}

// TestMergeMarkdownFile_LegacyMigrationWarning verifies that when the file contains
// the old "mnemos_store" marker but NOT the new sentinel, the function skips appending
// (to avoid a duplicate block) and returns false without error.
func TestMergeMarkdownFile_LegacyMigrationWarning(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	// Legacy content: has mnemos_store but no sentinel markers
	legacy := "# My Project\n\n# Memory Integration\nmnemos_store\nsome content\n"
	require.NoError(t, os.WriteFile(target, []byte(legacy), 0o644))

	w := setup.NewWriter(dir, false, false)
	// Caller now passes "<!-- mnemos:begin -->" as the marker
	appended, err := w.MergeMarkdownFile(target, "# Memory Integration\nmnemos_store", "<!-- mnemos:begin -->")
	require.NoError(t, err)
	assert.False(t, appended, "should skip appending when legacy content detected")

	// File should be unchanged — no second block added
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, legacy, string(content))

	// No sentinel markers should have been added
	assert.NotContains(t, string(content), "<!-- mnemos:begin -->")
	assert.NotContains(t, string(content), "<!-- mnemos:end -->")
}
