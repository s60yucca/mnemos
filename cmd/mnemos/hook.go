package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/spf13/cobra"
)

// newHookCmd creates the "mnemos hook" parent command with three subcommands.
// Each subcommand initializes Mnemos in InitLight mode (no background workers),
// dispatches the hook, and always exits 0.
func newHookCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "AI client hook integration",
		Long:  "Hook subcommands called by AI clients (Claude, Kiro, Cursor) via stdin/stdout.",
	}

	cmd.AddCommand(
		newHookSubCmd("session-start", cfg),
		newHookSubCmd("prompt-submit", cfg),
		newHookSubCmd("session-end", cfg),
	)

	return cmd
}

// newHookSubCmd creates a single hook subcommand that reads JSON from stdin,
// dispatches to the hook handler, and writes JSON to stdout.
func newHookSubCmd(hookType string, cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   hookType,
		Short: fmt.Sprintf("Handle %s hook", hookType),
		// SilenceErrors + SilenceUsage ensure cobra never writes to stdout on error,
		// which would corrupt the JSON output expected by AI clients.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runHook(cfg, hookType)
			return nil // always return nil — exit 0
		},
	}
}

// runHook initializes Mnemos with InitLight, dispatches the hook, and returns.
// All errors are handled internally; this function never causes a non-zero exit.
func runHook(cfg *config.Config, hookType string) {
	ctx := context.Background()

	// Build Mnemos with InitLight (no background workers, no embedding health-check)
	mnemos, cleanup, err := buildLightMnemos(cfg)
	if err != nil {
		// Even if we can't build Mnemos, we must write valid JSON to stdout.
		// The dispatcher handles nil mnemos gracefully via the hook logger.
		writeErrorJSON(hookType, err.Error())
		return
	}
	defer cleanup()

	logger := hook.NewHookLogger("", cfg.Hook.LogLevel)
	dispatcher := hook.NewDispatcher(mnemos, &cfg.Hook, logger)
	dispatcher.Dispatch(ctx, os.Stdin, os.Stdout)
}

// buildLightMnemos wires up all dependencies and returns a Mnemos instance
// initialized in InitLight mode, plus a cleanup function to close resources.
func buildLightMnemos(cfg *config.Config) (*core.Mnemos, func(), error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("data dir: %w", err)
	}

	db, err := sqlitestore.Open(cfg.DBPath())
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	memStore := sqlitestore.NewSQLiteStore(db)
	ftsSearcher := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)

	// Use the configured embedding provider for search, but skip health-check
	// (InitLight mode). Noop → nil so SearchEngine skips semantic path entirely.
	var embedProvider embedding.IEmbeddingProvider
	switch cfg.Embeddings.Provider {
	case "ollama":
		embedProvider = embedding.NewOllamaProvider(cfg.Embeddings.BaseURL, cfg.Embeddings.Model, cfg.Embeddings.Dims)
	case "openai":
		embedProvider = embedding.NewOpenAIProvider(cfg.Embeddings.APIKey, cfg.Embeddings.Model, cfg.Embeddings.Dims)
	default:
		// noop: leave nil
	}

	mirror := markdown.NewMirror(cfg.Mirror.BaseDir, false) // mirror disabled for hooks

	logger := util.NewLogger(cfg.LogLevel, cfg.LogFormat)

	memManager := coremem.NewManager(
		memStore, embedStore, embedProvider, mirror,
		cfg.Dedup.FuzzyThreshold, cfg.Dedup.SemanticThreshold,
		logger,
		coremem.NewQualityGate(cfg.QualityGate),
	)

	searchEngine := search.NewSearchEngine(ftsSearcher, embedStore, embedProvider, relStore, logger)
	relManager := relation.NewManager(relStore, memStore, logger)

	decayInterval := cfg.Lifecycle.DecayInterval
	if decayInterval == 0 {
		decayInterval = 24 * time.Hour
	}
	lifecycleEngine := lifecycle.NewEngine(
		memStore,
		decayInterval,
		cfg.Lifecycle.GCRetentionDays,
		cfg.Lifecycle.ArchiveThreshold,
		logger,
	)

	mnemos := core.NewMnemosWithMode(
		core.InitLight,
		memManager, searchEngine, relManager, lifecycleEngine,
		memStore, embedProvider, logger,
	)

	cleanup := func() {
		mnemos.Shutdown()
		db.Close()
	}

	return mnemos, cleanup, nil
}

// writeErrorJSON writes a minimal error HookOutput JSON to stdout.
// Used as last-resort when Mnemos initialization fails.
func writeErrorJSON(hookType, msg string) {
	fmt.Fprintf(os.Stdout, `{"status":"error","message":%q}`+"\n", msg)
	_ = hookType // reserved for future structured logging
}
