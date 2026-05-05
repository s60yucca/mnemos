package mcp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/config"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

// newTestServer creates a test MCP server with in-memory storage
func newTestServer(t *testing.T) (*Server, *core.Mnemos, string) {
	t.Helper()

	// Create temp data directory
	dataDir := t.TempDir()

	// Create in-memory SQLite database
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	// Storage adapters
	memStore := sqlitestore.NewSQLiteStore(db)
	ftsSearcher := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)

	// No embedding provider for tests (FTS only)
	var embedProvider embedding.IEmbeddingProvider = nil

	// Core engines
	memManager := coremem.NewManager(
		memStore, embedStore, embedProvider, nil, // nil mirror for tests
		0.85, 0.90, // dedup thresholds
		slog.Default(), // use default logger for tests
		coremem.NewQualityGate(config.QualityGateConfig{
			Enabled:  false, // Disable quality gate for tests
			MinWords: 1,
			MaxWords: 10000,
		}),
	)

	searchEngine := search.NewSearchEngine(ftsSearcher, embedStore, embedProvider, relStore, slog.Default(), 0.7, 1.5)
	relManager := relation.NewManager(relStore, memStore, slog.Default())
	lifecycleEngine := lifecycle.NewEngine(memStore, 24*time.Hour, 30, 0.1, slog.Default())

	// Create mnemos facade
	mnemos := core.NewMnemos(memManager, searchEngine, relManager, lifecycleEngine, memStore, slog.Default())

	// Create MCP server
	server := NewServer(mnemos, "test-version")

	// Override dataDir for test isolation
	server.dataDir = dataDir

	return server, mnemos, dataDir
}

// toolText extracts text content from a CallToolResult
func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if result.IsError {
		t.Fatalf("tool result is an error: %v", result.Content)
	}

	if len(result.Content) == 0 {
		return ""
	}

	// Extract text from first content item
	content := result.Content[0]
	if textContent, ok := content.(mcp.TextContent); ok {
		return textContent.Text
	}

	t.Fatalf("expected text content, got: %T", content)
	return ""
}
