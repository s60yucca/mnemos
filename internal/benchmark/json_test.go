//go:build benchmark

package benchmark

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBenchmarkReport_JSONRoundTrip(t *testing.T) {
	original := &BenchmarkReport{
		GeneratedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Results: []ScenarioResult{
			{
				Scenario: BenchmarkScenario{
					Name:      "test-scenario",
					Category:  CategoryColdStartToWarm,
					ProjectID: "test-project",
					Sessions:  10,
					Tasks: []Task{
						{
							Description:       "test task",
							Query:             "test query",
							RelevantMemoryIDs: []string{"mem-1", "mem-2"},
							GotchaMemoryIDs:   []string{"gotcha-1"},
							CorrectionIDs:     []string{"corr-1"},
							TokenBudget:       2000,
						},
					},
				},
				SteadyStateSession:    5,
				AvgPrecision:          0.85,
				AvgRecall:             0.90,
				AvgF1:                 0.87,
				AvgTokenEfficiency:    0.12,
				MistakePreventionRate: 0.75,
				CorrectionRate:        0.60,
				TransferRate:          0.80,
				Sessions: []SessionMetrics{
					{
						SessionNumber:          1,
						TokensConsumed:         100,
						TotalStoreTokens:       500,
						TokenEfficiency:        0.20,
						ContextPrecision:       0.80,
						ContextRecall:          0.85,
						F1Score:                0.82,
						TruePositives:          4,
						FalsePositives:         1,
						FalseNegatives:         1,
						TotalGotchas:           2,
						GotchasRetrieved:       1,
						CorrectionsRankedAbove: 1,
					},
				},
			},
		},
		Summary: ReportSummary{
			BestF1Scenario:         "test-scenario",
			BestEfficiencyScenario: "test-scenario",
			OverallAvgF1:           0.87,
			OverallAvgEfficiency:   0.12,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal back
	var decoded BenchmarkReport
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Assert key fields
	assert.Equal(t, original.GeneratedAt.UTC(), decoded.GeneratedAt.UTC())
	require.Len(t, decoded.Results, 1)

	result := decoded.Results[0]
	assert.Equal(t, "test-scenario", result.Scenario.Name)
	assert.Equal(t, CategoryColdStartToWarm, result.Scenario.Category)
	assert.Equal(t, 5, result.SteadyStateSession)
	assert.InDelta(t, 0.85, result.AvgPrecision, 1e-9)
	assert.InDelta(t, 0.90, result.AvgRecall, 1e-9)
	assert.InDelta(t, 0.87, result.AvgF1, 1e-9)
	assert.InDelta(t, 0.12, result.AvgTokenEfficiency, 1e-9)
	assert.InDelta(t, 0.75, result.MistakePreventionRate, 1e-9)
	assert.InDelta(t, 0.60, result.CorrectionRate, 1e-9)
	assert.InDelta(t, 0.80, result.TransferRate, 1e-9)

	require.Len(t, result.Sessions, 1)
	m := result.Sessions[0]
	assert.Equal(t, 1, m.SessionNumber)
	assert.Equal(t, 100, m.TokensConsumed)
	assert.InDelta(t, 0.20, m.TokenEfficiency, 1e-9)
	assert.Equal(t, 4, m.TruePositives)
	assert.Equal(t, 1, m.FalsePositives)
	assert.Equal(t, 1, m.CorrectionsRankedAbove)

	assert.Equal(t, "test-scenario", decoded.Summary.BestF1Scenario)
	assert.InDelta(t, 0.87, decoded.Summary.OverallAvgF1, 1e-9)

	// Verify JSON keys are snake_case (spot check)
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"generated_at"`)
	assert.Contains(t, jsonStr, `"avg_f1"`)
	assert.Contains(t, jsonStr, `"token_efficiency"`)
	assert.Contains(t, jsonStr, `"corrections_ranked_above"`)
	assert.Contains(t, jsonStr, `"mistake_prevention_rate"`)
	assert.Contains(t, jsonStr, `"correction_rate"`)
	assert.Contains(t, jsonStr, `"transfer_rate"`)
	assert.Contains(t, jsonStr, `"steady_state_session"`)
}
