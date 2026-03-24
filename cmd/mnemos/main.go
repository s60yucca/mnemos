package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/transport/cli"
	"github.com/mnemos-dev/mnemos/internal/util"
)

var version = "dev"

func main() {
	// Load config
	cfgFile := os.Getenv("MNEMOS_CONFIG")
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	// Logger
	logger := util.NewLogger(cfg.LogLevel, cfg.LogFormat)

	// Ensure data dir exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "data dir error:", err)
		os.Exit(1)
	}

	// Open SQLite
	db, err := sqlitestore.Open(cfg.DBPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "db error:", err)
		os.Exit(1)
	}

	// Storage adapters
	memStore := sqlitestore.NewSQLiteStore(db)
	ftsSearcher := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)

	// Embedding provider
	var embedProvider embedding.IEmbeddingProvider
	switch cfg.Embeddings.Provider {
	case "ollama":
		embedProvider = embedding.NewOllamaProvider(cfg.Embeddings.BaseURL, cfg.Embeddings.Model, cfg.Embeddings.Dims)
	case "openai":
		embedProvider = embedding.NewOpenAIProvider(cfg.Embeddings.APIKey, cfg.Embeddings.Model, cfg.Embeddings.Dims)
	default:
		// noop: leave embedProvider nil so SearchEngine skips the semantic path entirely.
		// Memories are still stored without embeddings; FTS handles all search.
	}

	// Markdown mirror
	mirror := markdown.NewMirror(cfg.Mirror.BaseDir, cfg.Mirror.Enabled)

	// Core engines
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

	// Facade
	mnemos := core.NewMnemos(memManager, searchEngine, relManager, lifecycleEngine, memStore, logger)
	mnemos.Start()
	defer mnemos.Shutdown()

	// CLI
	rootCmd := cli.NewRootCmd(mnemos, version)
	rootCmd.AddCommand(newHookCmd(cfg))
	rootCmd.AddCommand(newSetupCmd())
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
