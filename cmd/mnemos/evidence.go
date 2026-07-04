package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/spf13/cobra"
)

type EvidencePack struct {
	GeneratedAt   string                   `json:"generated_at"`
	Version       string                   `json:"version"`
	ProjectID     string                   `json:"project_id,omitempty"`
	Check         CheckReport              `json:"check"`
	FeatureHealth map[string]CheckStatus   `json:"feature_health"`
	Eval          EvidenceEvalSummary      `json:"eval"`
	Benchmark     EvidenceBenchmarkSummary `json:"benchmark"`
	Notes         []string                 `json:"notes,omitempty"`
}

type EvidenceEvalSummary struct {
	Active        int     `json:"active"`
	Archived      int     `json:"archived"`
	Quality       float64 `json:"quality"`
	DuplicateRate float64 `json:"duplicate_rate"`
	Stale         int     `json:"stale"`
	AvgRelevance  float64 `json:"avg_relevance"`
	LowRelevance  int     `json:"low_relevance"`
	NeverAccessed int     `json:"never_accessed"`
	Score         float64 `json:"score"`
}

type EvidenceBenchmarkSummary struct {
	WindowDays int `json:"window_days"`
	ON         int `json:"on"`
	OFF        int `json:"off"`
	Total      int `json:"total"`
}

func newEvidenceCmd(cfg *config.Config, mnemos *core.Mnemos, buildVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Export launch evidence for the knowledge loop",
	}
	cmd.AddCommand(newEvidenceExportCmd(cfg, mnemos, buildVersion))
	return cmd
}

func newEvidenceExportCmd(cfg *config.Config, mnemos *core.Mnemos, buildVersion string) *cobra.Command {
	var (
		project string
		format  string
		output  string
		redact  bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export check, health, eval, and benchmark evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			pack, err := buildEvidencePack(cmd.Context(), cfg, mnemos, buildVersion, project, redact)
			if err != nil {
				return err
			}

			var rendered string
			switch strings.ToLower(format) {
			case "json", "":
				data, err := json.MarshalIndent(pack, "", "  ")
				if err != nil {
					return err
				}
				rendered = string(data) + "\n"
			case "markdown", "md":
				rendered = renderEvidenceMarkdown(pack)
			default:
				return fmt.Errorf("format must be json or markdown")
			}

			if output != "" {
				return os.WriteFile(output, []byte(rendered), 0o600)
			}
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project ID to evaluate (empty = all projects)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json|markdown")
	cmd.Flags().StringVar(&output, "output", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&redact, "redact", false, "Redact project identifiers from exported evidence")
	return cmd
}

func buildEvidencePack(ctx context.Context, cfg *config.Config, mnemos *core.Mnemos, buildVersion, project string, redact bool) (*EvidencePack, error) {
	check := runCheck(ctx, cfg, mnemos, buildVersion, checkOptions{project: project, launch: true})
	featureHealth, err := buildEvidenceFeatureHealth(cfg.DataDir, project)
	if err != nil {
		return nil, err
	}
	eval, err := buildEvidenceEval(ctx, mnemos, project)
	if err != nil {
		return nil, err
	}
	bench, err := buildEvidenceBenchmark(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	pack := &EvidencePack{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Version:       buildVersion,
		ProjectID:     project,
		Check:         check,
		FeatureHealth: featureHealth,
		Eval:          eval,
		Benchmark:     bench,
	}
	if bench.OFF == 0 {
		pack.Notes = append(pack.Notes, "Benchmark evidence has no OFF sessions yet; collect an OFF cohort before public launch.")
	}
	if redact {
		pack.ProjectID = ""
		pack.Check.ProjectID = ""
		redactProjectIDs(pack.Check.Signals)
	}
	return pack, nil
}

func buildEvidenceFeatureHealth(dataDir, project string) (map[string]CheckStatus, error) {
	events, err := parseLog(benchmarkLogPath(dataDir), time.Now().UTC().Add(-7*24*time.Hour), project)
	if err != nil {
		return nil, err
	}
	activeDays := detectActiveDays(events)
	out := make(map[string]CheckStatus, len(observe.Baselines))
	for feature, baseline := range observe.Baselines {
		var featureEvents []Event
		for _, event := range events {
			if event.Feature == feature {
				featureEvents = append(featureEvents, event)
			}
		}
		switch classifyFeatureNamed(feature, featureEvents, events, baseline, activeDays) {
		case StatusFiring:
			out[feature] = CheckPass
		case StatusLow:
			out[feature] = CheckWarn
		case StatusNotFiring:
			out[feature] = CheckFail
		default:
			out[feature] = CheckUnknown
		}
	}
	return out, nil
}

func buildEvidenceEval(ctx context.Context, mnemos *core.Mnemos, project string) (EvidenceEvalSummary, error) {
	stats, err := mnemos.Stats(ctx, project)
	if err != nil {
		return EvidenceEvalSummary{}, fmt.Errorf("stats: %w", err)
	}
	memories, err := mnemos.List(ctx, storage.ListQuery{
		ProjectID: project,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     10000,
		SortBy:    "created_at",
		SortDesc:  true,
	})
	if err != nil {
		return EvidenceEvalSummary{}, fmt.Errorf("list memories: %w", err)
	}
	memories = filterUserFacingMemories(memories)
	metrics := calculateEvalMetrics(memories, stats.ByStatus["archived"])
	_, dupRate := duplicationStats(memories)
	return EvidenceEvalSummary{
		Active:        metrics.active,
		Archived:      metrics.archived,
		Quality:       metrics.quality,
		DuplicateRate: dupRate,
		Stale:         int(metrics.stale),
		AvgRelevance:  metrics.relevance,
		LowRelevance:  metrics.lowRelevance,
		NeverAccessed: metrics.neverAccessed,
		Score:         metrics.score,
	}, nil
}

func buildEvidenceBenchmark(dataDir string) (EvidenceBenchmarkSummary, error) {
	summary := EvidenceBenchmarkSummary{WindowDays: 7}
	logPath := benchmarkLogPath(dataDir)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return summary, nil
	}
	sessions, err := extractSessions(logPath, time.Now().UTC().Add(-7*24*time.Hour), "", "", false)
	if err != nil {
		return summary, err
	}
	for _, session := range sessions {
		switch session.Mode {
		case "on":
			summary.ON++
		case "off":
			summary.OFF++
		}
	}
	summary.Total = len(sessions)
	return summary, nil
}

func redactProjectIDs(signals []CheckSignal) {
	for i := range signals {
		signals[i].Scope = ""
	}
}

func renderEvidenceMarkdown(pack *EvidencePack) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Mnemos Evidence Pack\n\n")
	fmt.Fprintf(&b, "- Generated: `%s`\n", pack.GeneratedAt)
	fmt.Fprintf(&b, "- Version: `%s`\n", pack.Version)
	if pack.ProjectID != "" {
		fmt.Fprintf(&b, "- Project: `%s`\n", pack.ProjectID)
	}
	fmt.Fprintf(&b, "- Check: **%s** — %s\n", pack.Check.Status, pack.Check.Summary)
	fmt.Fprintf(&b, "- Eval score: **%.2f** (%d active, %.0f%% duplicate rate)\n", pack.Eval.Score, pack.Eval.Active, pack.Eval.DuplicateRate*100)
	fmt.Fprintf(&b, "- Benchmark: `%d ON / %d OFF / %d total`\n\n", pack.Benchmark.ON, pack.Benchmark.OFF, pack.Benchmark.Total)

	fmt.Fprintf(&b, "## Feature Health\n\n")
	features := make([]string, 0, len(pack.FeatureHealth))
	for feature := range pack.FeatureHealth {
		features = append(features, feature)
	}
	sort.Strings(features)
	for _, feature := range features {
		status := pack.FeatureHealth[feature]
		fmt.Fprintf(&b, "- `%s`: `%s`\n", feature, status)
	}
	if len(pack.Notes) > 0 {
		fmt.Fprintf(&b, "\n## Notes\n\n")
		for _, note := range pack.Notes {
			fmt.Fprintf(&b, "- %s\n", note)
		}
	}
	return b.String()
}
