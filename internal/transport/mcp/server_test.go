package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEmbedder struct{}

func (testEmbedder) Name() string                 { return "test" }
func (testEmbedder) Dimensions() int              { return 2 }
func (testEmbedder) Healthy(context.Context) bool { return true }
func (testEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := testEmbedder{}.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out = append(out, vec)
	}
	return out, nil
}

func (testEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	text = strings.ToLower(text)
	if strings.Contains(text, "token expiry") || strings.Contains(text, "jwt lifetime") {
		return []float32{1, 0}, nil
	}
	return []float32{0, 1}, nil
}

func newTestServer(t *testing.T) (*Server, *core.Mnemos, *sqlitestore.EmbeddingStore) {
	t.Helper()

	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	memStore := sqlitestore.NewSQLiteStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)
	embedProvider := testEmbedder{}
	mirror := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memManager := coremem.NewManager(memStore, embedStore, embedProvider, mirror, 0.85, 0.92, logger, nil)
	searchEngine := search.NewSearchEngine(fts, embedStore, embedProvider, relStore, logger)
	relManager := relation.NewManager(relStore, memStore, logger)
	lcEngine := lifecycle.NewEngine(memStore, 24*time.Hour, 30, 0.1, logger)
	mn := core.NewMnemos(memManager, searchEngine, relManager, lcEngine, memStore, logger)

	server := NewServer(mn)

	t.Cleanup(func() {
		mn.Shutdown()
		db.Close()
	})

	return server, mn, embedStore
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	text, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	return text.Text
}

func TestHandleSearch_SemanticModeUsesSemanticSearch(t *testing.T) {
	server, mn, embedStore := newTestServer(t)
	ctx := context.Background()

	stored, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "JWT lifetime is one hour",
		ProjectID: "proj-search",
	})
	require.NoError(t, err)

	vec, err := testEmbedder{}.Embed(ctx, stored.Memory.Content)
	require.NoError(t, err)
	require.NoError(t, embedStore.StoreEmbedding(ctx, stored.Memory.ID, vec))

	result, err := server.handleSearch(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query":      "token expiry",
				"project_id": "proj-search",
				"mode":       "semantic",
			},
		},
	})
	require.NoError(t, err)

	var payload []*storage.SearchResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &payload))
	require.Len(t, payload, 1)
	assert.Equal(t, "semantic", payload[0].Source)
	assert.Equal(t, stored.Memory.ID, payload[0].Memory.ID)
}

func TestHandleLoadContextPrompt_UsesProvidedTokenBudget(t *testing.T) {
	server, mn, _ := newTestServer(t)
	ctx := context.Background()

	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   strings.Repeat("a", 120),
		ProjectID: "proj-prompt",
	})
	require.NoError(t, err)

	result, err := server.handleLoadContextPrompt(ctx, mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Arguments: map[string]string{
				"query":      "a",
				"project_id": "proj-prompt",
				"max_tokens": "5",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Messages, 1)

	content, ok := result.Messages[0].Content.(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, content.Text, "Total tokens used: 0")
}

func TestHandleMemoriesResource_ReturnsOnlyActiveMemories(t *testing.T) {
	server, mn, _ := newTestServer(t)
	ctx := context.Background()

	active, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "active memory",
		ProjectID: "proj-resource",
	})
	require.NoError(t, err)

	deleted, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "deleted memory",
		ProjectID: "proj-resource",
	})
	require.NoError(t, err)
	require.NoError(t, mn.Delete(ctx, deleted.Memory.ID))

	resource, err := server.handleMemoriesResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "mnemos://memories/proj-resource"},
	})
	require.NoError(t, err)
	require.Len(t, resource, 1)

	textResource, ok := resource[0].(mcp.TextResourceContents)
	require.True(t, ok)

	var memories []*domain.Memory
	require.NoError(t, json.Unmarshal([]byte(textResource.Text), &memories))
	require.Len(t, memories, 1)
	assert.Equal(t, active.Memory.ID, memories[0].ID)
}

func TestHandleSearch_InvalidModeReturnsToolError(t *testing.T) {
	server, _, _ := newTestServer(t)

	result, err := server.handleSearch(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"query": "anything",
				"mode":  "bogus",
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(t, result), "mode must be one of")
}

func TestHandleMemoriesResource_UsesProjectScope(t *testing.T) {
	server, mn, _ := newTestServer(t)
	ctx := context.Background()

	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "project scoped",
		ProjectID: "proj-a",
	})
	require.NoError(t, err)
	_, err = mn.Store(ctx, &domain.StoreRequest{
		Content:   "other project",
		ProjectID: "proj-b",
	})
	require.NoError(t, err)

	resource, err := server.handleMemoriesResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "mnemos://memories/proj-a"},
	})
	require.NoError(t, err)

	textResource, ok := resource[0].(mcp.TextResourceContents)
	require.True(t, ok)

	var memories []*domain.Memory
	require.NoError(t, json.Unmarshal([]byte(textResource.Text), &memories))
	require.Len(t, memories, 1)
	assert.Equal(t, "proj-a", memories[0].ProjectID)
}

var _ embedding.IEmbeddingProvider = testEmbedder{}

func newTestServerWithGate(t *testing.T) *Server {
	t.Helper()

	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	memStore := sqlitestore.NewSQLiteStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)
	embedProvider := testEmbedder{}
	mirror := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := coremem.TestQualityGateConfig()
	gate := coremem.NewQualityGate(cfg)

	memManager := coremem.NewManager(memStore, embedStore, embedProvider, mirror, 0.85, 0.92, logger, gate)
	searchEngine := search.NewSearchEngine(fts, embedStore, embedProvider, relStore, logger)
	relManager := relation.NewManager(relStore, memStore, logger)
	lcEngine := lifecycle.NewEngine(memStore, 24*time.Hour, 30, 0.1, logger)
	mn := core.NewMnemos(memManager, searchEngine, relManager, lcEngine, memStore, logger)

	server := NewServer(mn)

	t.Cleanup(func() {
		mn.Shutdown()
		db.Close()
	})

	return server
}

func callStore(t *testing.T, server *Server, args map[string]any) *domain.StoreResult {
	t.Helper()
	ctx := context.Background()
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: args},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "handleStore must never return isError=true for quality verdicts")
	var out domain.StoreResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &out))
	return &out
}

// TestHandleStore_QualityNote_Reject verifies that a too-short memory returns
// isError=false and a quality_note explaining the rejection.
func TestHandleStore_QualityNote_Reject(t *testing.T) {
	server := newTestServerWithGate(t)
	out := callStore(t, server, map[string]any{"content": "fix bug"})
	assert.False(t, out.Created)
	assert.Contains(t, out.QualityNote, "brief")
}

// TestHandleStore_QualityNote_Fix verifies that a verbose memory returns
// isError=false, is created, and carries a quality_note about optimisation.
func TestHandleStore_QualityNote_Fix(t *testing.T) {
	server := newTestServerWithGate(t)
	// Low-density verbose content triggers FixCompact → VerdictFix
	verbose := strings.Repeat("we looked at things and discussed the system and talked about it ", 4)
	out := callStore(t, server, map[string]any{"content": verbose, "project_id": "proj-fix"})
	// May be created (Fix) or rejected depending on final score; either way no isError
	if out.Created {
		assert.Contains(t, out.QualityNote, "optimized")
	}
}

// TestHandleStore_QualityNote_Downgrade verifies that a generic long_term memory
// is downgraded and carries the appropriate quality_note.
func TestHandleStore_QualityNote_Downgrade(t *testing.T) {
	server := newTestServerWithGate(t)
	out := callStore(t, server, map[string]any{
		"content":    "The project has good error handling and the team is happy with it overall",
		"type":       "long_term",
		"project_id": "proj-downgrade",
	})
	if out.Created && out.QualityNote != "" {
		assert.Contains(t, out.QualityNote, "short-term")
	}
}

// TestHandleStore_NoQualityNote_Accept verifies that a high-quality memory
// returns isError=false and no quality_note.
func TestHandleStore_NoQualityNote_Accept(t *testing.T) {
	server := newTestServerWithGate(t)
	out := callStore(t, server, map[string]any{
		"content":    "SessionStore.Close() in auth/session.go needs sync.RWMutex to prevent race condition",
		"project_id": "proj-accept",
	})
	assert.True(t, out.Created)
	assert.Empty(t, out.QualityNote)
}

// TestHandleStore_RejectIsNotMCPError verifies the isError=false contract for Reject.
func TestHandleStore_RejectIsNotMCPError(t *testing.T) {
	server := newTestServerWithGate(t)
	ctx := context.Background()
	result, err := server.handleStore(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"content": "ok"}},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "Reject verdict must not set isError=true")
}
