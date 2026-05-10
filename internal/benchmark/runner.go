package benchmark

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Runner orchestrates benchmark scenario execution.
type Runner struct {
	logger *slog.Logger
}

// NewRunner creates a Runner with the given logger.
func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{logger: logger}
}

// Run executes all scenarios using a single shared simulator, resetting between scenarios.
// It returns a BenchmarkReport with aggregated results and a summary.
func (r *Runner) Run(ctx context.Context, scenarios []BenchmarkScenario) (*BenchmarkReport, error) {
	sim, err := NewSessionSimulator()
	if err != nil {
		return nil, fmt.Errorf("create simulator: %w", err)
	}
	defer sim.Close()

	var results []ScenarioResult

	for i, scenario := range scenarios {
		r.logger.Info("running scenario", "name", scenario.Name, "index", i+1, "total", len(scenarios))

		if i > 0 {
			if err := sim.resetForNextScenario(ctx); err != nil {
				return nil, fmt.Errorf("reset before scenario %q: %w", scenario.Name, err)
			}
		}

		allMetrics, err := sim.RunScenario(ctx, scenario)
		if err != nil {
			return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
		}

		result := aggregateResult(scenario, allMetrics)
		results = append(results, result)
	}

	summary := buildSummary(results)

	return &BenchmarkReport{
		GeneratedAt: time.Now(),
		Results:     results,
		Summary:     summary,
	}, nil
}

// RunOne executes a single scenario with its own simulator instance.
func (r *Runner) RunOne(ctx context.Context, s BenchmarkScenario) (*ScenarioResult, error) {
	// Task 5.3: input validation
	if s.Sessions < 1 {
		return nil, fmt.Errorf("scenario %q: Sessions must be >= 1, got %d", s.Name, s.Sessions)
	}
	if len(s.Tasks) < 1 {
		return nil, fmt.Errorf("scenario %q: Tasks must be non-empty", s.Name)
	}

	sim, err := NewSessionSimulator()
	if err != nil {
		return nil, fmt.Errorf("create simulator: %w", err)
	}
	defer sim.Close()

	r.logger.Info("running scenario", "name", s.Name)

	allMetrics, err := sim.RunScenario(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("scenario %q: %w", s.Name, err)
	}

	result := aggregateResult(s, allMetrics)
	return &result, nil
}

// aggregateResult computes a ScenarioResult from raw session metrics.
func aggregateResult(scenario BenchmarkScenario, allMetrics []SessionMetrics) ScenarioResult {
	n := float64(len(allMetrics))

	var (
		sumPrecision        float64
		sumRecall           float64
		sumF1               float64
		sumEfficiency       float64
		sumTotalGotchas     int
		sumGotchasRetrieved int
		sumCorrectionsAbove int
	)

	for _, m := range allMetrics {
		sumPrecision += m.ContextPrecision
		sumRecall += m.ContextRecall
		sumF1 += m.F1Score
		sumEfficiency += m.TokenEfficiency
		sumTotalGotchas += m.TotalGotchas
		sumGotchasRetrieved += m.GotchasRetrieved
		sumCorrectionsAbove += m.CorrectionsRankedAbove
	}

	var avgPrecision, avgRecall, avgF1, avgEfficiency float64
	if n > 0 {
		avgPrecision = sumPrecision / n
		avgRecall = sumRecall / n
		avgF1 = sumF1 / n
		avgEfficiency = sumEfficiency / n
	}

	// MistakePreventionRate: fraction of gotchas avoided
	mistakePreventionRate := float64(sumTotalGotchas-sumGotchasRetrieved) / float64(max1(sumTotalGotchas))

	// CorrectionRate: corrections ranked above gotchas / total possible correction slots
	totalCorrectionIDs := 0
	for _, task := range scenario.Tasks {
		totalCorrectionIDs += len(task.CorrectionIDs)
	}
	totalCorrectionIDs *= scenario.Sessions
	correctionRate := float64(sumCorrectionsAbove) / float64(max1(totalCorrectionIDs))

	// TransferRate: fraction of (session, task) pairs where ContextRecall > 0
	var recallPositive int
	for _, m := range allMetrics {
		if m.ContextRecall > 0 {
			recallPositive++
		}
	}
	var transferRate float64
	if len(allMetrics) > 0 {
		transferRate = float64(recallPositive) / float64(len(allMetrics))
	}

	steadyState := DetectSteadyState(allMetrics, 3, 0.05)

	return ScenarioResult{
		Scenario:              scenario,
		Sessions:              allMetrics,
		SteadyStateSession:    steadyState,
		AvgPrecision:          avgPrecision,
		AvgRecall:             avgRecall,
		AvgF1:                 avgF1,
		AvgTokenEfficiency:    avgEfficiency,
		MistakePreventionRate: mistakePreventionRate,
		CorrectionRate:        correctionRate,
		TransferRate:          transferRate,
	}
}

// buildSummary computes the ReportSummary across all scenario results.
func buildSummary(results []ScenarioResult) ReportSummary {
	if len(results) == 0 {
		return ReportSummary{}
	}

	bestF1Name := results[0].Scenario.Name
	bestEffName := results[0].Scenario.Name
	bestF1 := results[0].AvgF1
	bestEff := results[0].AvgTokenEfficiency

	var sumF1, sumEff float64
	for _, r := range results {
		sumF1 += r.AvgF1
		sumEff += r.AvgTokenEfficiency

		if r.AvgF1 > bestF1 {
			bestF1 = r.AvgF1
			bestF1Name = r.Scenario.Name
		}
		// Lower efficiency = more selective = better
		if r.AvgTokenEfficiency < bestEff {
			bestEff = r.AvgTokenEfficiency
			bestEffName = r.Scenario.Name
		}
	}

	n := float64(len(results))
	return ReportSummary{
		BestF1Scenario:         bestF1Name,
		BestEfficiencyScenario: bestEffName,
		OverallAvgF1:           sumF1 / n,
		OverallAvgEfficiency:   sumEff / n,
	}
}

// max1 returns v if v >= 1, otherwise 1. Used to avoid division by zero.
func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
