package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMnemos creates a *core.Mnemos backed by an in-memory SQLite database.
func newTestMnemos(t *testing.T) *core.Mnemos {
	t.Helper()

	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	relStore := sqlitestore.NewRelationStore(db)

	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memMgr := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	return core.NewMnemosWithMode(
		core.InitLight,
		memMgr,
		searchEng,
		relMgr,
		lcEngine,
		store,
		nil,
		logger,
	)
}

// defaultHookConfig returns a HookConfig with hooks enabled.
func defaultHookConfig() *config.HookConfig {
	return &config.HookConfig{
		Enabled:                  true,
		SessionDir:               "sessions",
		StaleTimeout:             1 * time.Hour,
		CleanupRetention:         24 * time.Hour,
		SearchCooldown:           5 * time.Minute,
		TopicSimilarityThreshold: 0.3,
		SessionStartMaxTokens:    2000,
		PromptSearchLimit:        5,
		LogLevel:                 "warn",
	}
}

func TestDispatcher_UnknownHook(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	d := hook.NewDispatcher(mn, cfg, logger)

	input := `{"hook":"totally-unknown-hook","session_id":"test-session"}`
	var buf bytes.Buffer

	d.Dispatch(context.Background(), strings.NewReader(input), &buf)

	var out hook.HookOutput
	require.NoError(t, json.NewDecoder(&buf).Decode(&out))

	assert.Equal(t, "error", out.Status)
	assert.Contains(t, strings.ToLower(out.Message), "unknown hook")
}

func TestHookConfig_Disabled(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	cfg.Enabled = false
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	d := hook.NewDispatcher(mn, cfg, logger)

	hookNames := []string{"session-start", "prompt-submit", "session-end", "totally-unknown-hook"}

	for _, hookName := range hookNames {
		t.Run(hookName, func(t *testing.T) {
			input := `{"hook":"` + hookName + `","session_id":"test-session"}`
			var buf bytes.Buffer

			d.Dispatch(context.Background(), strings.NewReader(input), &buf)

			var out hook.HookOutput
			require.NoError(t, json.NewDecoder(&buf).Decode(&out))

			assert.Equal(t, "skipped", out.Status)
			assert.Equal(t, "hooks disabled", out.Message)
		})
	}
}
