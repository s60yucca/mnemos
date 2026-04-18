package memory

import (
	"context"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contentWithFilePaths contains file-path entities that reFilePath will match.
const contentWithFilePaths = "Updated auth/session.go and internal/core/search/engine.go to fix the token refresh bug."

// contentNoFilePaths has no slash-separated file paths.
const contentNoFilePaths = "Fixed a bug in the token refresh logic by resetting the expiry counter."

// TestManagerFile_StorePopulatesRelatedFiles verifies that Store sets
// Metadata["related_files"] when the content contains file-path entities.
func TestManagerFile_StorePopulatesRelatedFiles(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   contentWithFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)

	encoded := result.Memory.Metadata[RelatedFilesKey]
	assert.NotEmpty(t, encoded, "related_files key should be set when content has file paths")

	paths := parseRelatedFiles(encoded)
	assert.NotEmpty(t, paths)
	// Both paths from the content should be present
	assert.Contains(t, paths, "auth/session.go")
	assert.Contains(t, paths, "internal/core/search/engine.go")
}

// TestManagerFile_StoreNoFilePathsKeyAbsent verifies that Store does NOT set
// Metadata["related_files"] when the content has no file-path entities.
func TestManagerFile_StoreNoFilePathsKeyAbsent(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   contentNoFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)

	_, hasKey := result.Memory.Metadata[RelatedFilesKey]
	assert.False(t, hasKey, "related_files key should be absent when content has no file paths")
}

// TestManagerFile_StoreWithoutGatePopulatesRelatedFiles verifies the same behaviour
// for StoreWithoutGate.
func TestManagerFile_StoreWithoutGatePopulatesRelatedFiles(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	result, err := m.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   contentWithFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)

	encoded := result.Memory.Metadata[RelatedFilesKey]
	assert.NotEmpty(t, encoded, "related_files key should be set when content has file paths")

	paths := parseRelatedFiles(encoded)
	assert.Contains(t, paths, "auth/session.go")
}

// TestManagerFile_StoreWithoutGateNoFilePathsKeyAbsent verifies that StoreWithoutGate
// does not set the key when content has no file paths.
func TestManagerFile_StoreWithoutGateNoFilePathsKeyAbsent(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	result, err := m.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   contentNoFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)

	_, hasKey := result.Memory.Metadata[RelatedFilesKey]
	assert.False(t, hasKey, "related_files key should be absent when content has no file paths")
}

// TestManagerFile_UpdateContentChangeUpdatesKey verifies that updating a memory's
// content to include file paths populates the related_files key.
func TestManagerFile_UpdateContentChangeUpdatesKey(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	// Store without file paths first
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   contentNoFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)

	// Update to content with file paths
	newContent := contentWithFilePaths
	updated, err := m.Update(ctx, &domain.UpdateRequest{
		ID:      result.Memory.ID,
		Content: &newContent,
	})
	require.NoError(t, err)

	encoded := updated.Metadata[RelatedFilesKey]
	assert.NotEmpty(t, encoded, "related_files key should be set after update to file-path content")

	paths := parseRelatedFiles(encoded)
	assert.Contains(t, paths, "auth/session.go")
}

// TestManagerFile_UpdateContentChangeToNoPathsDeletesKey verifies that updating a
// memory's content to remove file paths deletes the related_files key.
func TestManagerFile_UpdateContentChangeToNoPathsDeletesKey(t *testing.T) {
	m := newManagerForSummarizeTest(t)
	ctx := context.Background()

	// Store with file paths
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   contentWithFilePaths,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, result.Created)
	require.NotEmpty(t, result.Memory.Metadata[RelatedFilesKey], "precondition: key should be set")

	// Update to content without file paths
	newContent := contentNoFilePaths
	updated, err := m.Update(ctx, &domain.UpdateRequest{
		ID:      result.Memory.ID,
		Content: &newContent,
	})
	require.NoError(t, err)

	_, hasKey := updated.Metadata[RelatedFilesKey]
	assert.False(t, hasKey, "related_files key should be deleted after update to no-file-path content")
}
