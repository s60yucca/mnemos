package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	"github.com/mnemos-dev/mnemos/internal/config"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/transport/cli"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/spf13/cobra"
)

var version = "dev"

//go:embed version.txt
var embeddedVersion string

func resolveVersion() string {
	if version != "dev" {
		return version // ldflags override takes priority
	}
	if v := strings.TrimSpace(embeddedVersion); v != "" {
		return v
	}
	return "dev"
}

func main() {
	// Load config
	cfgFile := configPathFromArgs(os.Args[1:])
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	observe.SetDataDir(cfg.DataDir)
	_ = os.Setenv("MNEMOS_DATA_DIR", cfg.DataDir)
	if projectID := projectIDFromArgs(os.Args[1:]); projectID != "" {
		_ = os.Setenv("MNEMOS_PROJECT_ID", projectID)
	}

	// Logger
	logger := util.NewLogger(cfg.LogLevel, cfg.LogFormat)

	if isCheckInvocation(os.Args[1:]) {
		if err := runReadOnlyCheck(cfg, logger, resolveVersion()); err != nil {
			if !isCheckFailure(err) {
				fmt.Fprintln(os.Stderr, "check error:", err)
			}
			os.Exit(1)
		}
		return
	}
	if isDoctorInvocation(os.Args[1:]) {
		if err := runDoctorOnly(cfg, resolveVersion()); err != nil {
			fmt.Fprintln(os.Stderr, "doctor error:", err)
			os.Exit(1)
		}
		return
	}

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

	searchEngine := search.NewSearchEngine(ftsSearcher, embedStore, embedProvider, relStore, logger, cfg.Hook.MMRLambda, cfg.Hook.FileBoost)
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
		cfg.Lifecycle.AutoArchive,
		logger,
	)

	// Facade
	mnemos := core.NewMnemos(memManager, searchEngine, relManager, lifecycleEngine, memStore, logger)
	defer mnemos.Shutdown()

	// Autopilot daemon is constructed for control commands, but only the server
	// runtime owns background workers.
	daemon := autopilot.NewAutopilotDaemon(mnemos, cfg.Autopilot, cfg.DataDir, logger, autopilot.NewReportWriter(mnemos))
	if isServerInvocation(os.Args[1:]) {
		mnemos.Start()
		daemon.Start()
		defer daemon.Stop()
	}

	// CLI
	rootCmd := cli.NewRootCmd(mnemos, resolveVersion())
	rootCmd.AddCommand(newHookCmd(cfg))
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newAutopilotCmd(daemon, mnemos, cfg.DataDir))
	rootCmd.AddCommand(newBackfillCmd(mnemos))
	rootCmd.AddCommand(newHealthCmd(cfg.DataDir))
	rootCmd.AddCommand(newStatusCmd(cfg))
	rootCmd.AddCommand(newBenchCmd(cfg.DataDir))
	rootCmd.AddCommand(newEvalCmd(mnemos))
	rootCmd.AddCommand(newEvidenceCmd(cfg, mnemos, resolveVersion()))
	rootCmd.AddCommand(newDoctorCmd(cfg, resolveVersion()))
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}

func configPathFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "--config=") {
			return strings.TrimPrefix(args[i], "--config=")
		}
	}
	return os.Getenv("MNEMOS_CONFIG")
}

func projectIDFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project", "-p":
			if i+1 < len(args) {
				return args[i+1]
			}
		default:
			if strings.HasPrefix(args[i], "--project=") {
				return strings.TrimPrefix(args[i], "--project=")
			}
			if strings.HasPrefix(args[i], "-p=") {
				return strings.TrimPrefix(args[i], "-p=")
			}
		}
	}
	return ""
}

func isServerInvocation(args []string) bool {
	return selectedCommand(args) == "serve"
}

func selectedCommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "--project", "-p", "--log-level":
			i++
		default:
			if !strings.HasPrefix(args[i], "-") {
				return args[i]
			}
		}
	}
	return ""
}

func isCheckInvocation(args []string) bool {
	return selectedCommand(args) == "check"
}

func isDoctorInvocation(args []string) bool {
	return selectedCommand(args) == "doctor"
}

func runDoctorOnly(cfg *config.Config, buildVersion string) error {
	root := &cobra.Command{Use: "mnemos", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newDoctorCmd(cfg, buildVersion))
	root.SetArgs(doctorCommandArgs(os.Args[1:]))
	return root.Execute()
}

func doctorCommandArgs(args []string) []string {
	for i, arg := range args {
		if arg == "doctor" {
			return args[i:]
		}
	}
	return args
}

func runReadOnlyCheck(cfg *config.Config, logger *slog.Logger, buildVersion string) error {
	db, err := sqlitestore.OpenReadOnly(cfg.DBPath())
	if err != nil {
		report := CheckReport{
			Status:  CheckFail,
			Summary: "Knowledge loop has a blocking failure.",
			Signals: []CheckSignal{failedSignal("database", true, "global", err)},
			ActionItems: []ActionItem{{
				Severity: CheckFail,
				Message:  "Initialize Mnemos or verify the configured data directory.",
				Command:  "mnemos init",
			}},
		}
		if hasArg(os.Args[1:], "--json") {
			if renderErr := renderCheckJSON(os.Stdout, report); renderErr != nil {
				return renderErr
			}
		} else {
			renderCheckText(os.Stdout, report, hasArg(os.Args[1:], "--verbose"))
		}
		return checkFailedError{}
	}
	defer db.Close()

	memStore := sqlitestore.NewSQLiteStore(db)
	ftsSearcher := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)
	mirror := markdown.NewMirror("", false)
	memManager := coremem.NewManager(
		memStore, embedStore, nil, mirror,
		cfg.Dedup.FuzzyThreshold, cfg.Dedup.SemanticThreshold,
		logger, nil,
	)
	searchEngine := search.NewSearchEngine(ftsSearcher, embedStore, nil, relStore, logger, cfg.Hook.MMRLambda, cfg.Hook.FileBoost)
	relManager := relation.NewManager(relStore, memStore, logger)
	lifecycleEngine := lifecycle.NewEngine(
		memStore,
		cfg.Lifecycle.DecayInterval,
		cfg.Lifecycle.GCRetentionDays,
		cfg.Lifecycle.ArchiveThreshold,
		false,
		logger,
	)
	mnemos := core.NewMnemos(memManager, searchEngine, relManager, lifecycleEngine, memStore, logger)
	defer mnemos.Shutdown()

	root := &cobra.Command{Use: "mnemos", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newCheckCmd(cfg, mnemos, buildVersion))
	root.SetArgs(checkCommandArgs(os.Args[1:]))
	return root.Execute()
}

func checkCommandArgs(args []string) []string {
	for i, arg := range args {
		if arg == "check" {
			return args[i:]
		}
	}
	return args
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
