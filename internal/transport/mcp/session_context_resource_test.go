package mcp

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSessionContextResource_NoProject(t *testing.T) {
	// Setup empty directory to avoid project detection
	tempDir := t.TempDir()
	originalWD, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWD)

	s, _, _ := newTestServer(t)

	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "mnemos://session-context",
		},
	}

	contents, err := s.handleSessionContextResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	textContent, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "mnemos://session-context", textContent.URI)
	assert.Equal(t, "text/plain", textContent.MIMEType)
	assert.True(t, strings.HasPrefix(textContent.Text, "# No memories found for project"))
}

func TestHandleSessionContextResource_WithProject(t *testing.T) {
	// Provide a project ID
	os.Setenv("MNEMOS_PROJECT_ID", "test-project")
	defer os.Unsetenv("MNEMOS_PROJECT_ID")

	s, mn, _ := newTestServer(t)

	// Add a memory
	mn.Store(context.Background(), &domain.StoreRequest{
		Content:   "current project context overview test memory",
		Type:      domain.MemoryTypeLongTerm,
		ProjectID: "test-project",
	})

	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "mnemos://session-context",
		},
	}

	contents, err := s.handleSessionContextResource(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	textContent, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "mnemos://session-context", textContent.URI)
	assert.Equal(t, "text/plain", textContent.MIMEType)

	// Should contain the header for auto-inject payload
	assert.True(t, strings.HasPrefix(textContent.Text, "# Mnemos Project Context"))
	assert.Contains(t, textContent.Text, "test memory")
}
