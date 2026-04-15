//go:build benchmark

package benchmark

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASCIIRenderer_Render(t *testing.T) {
	report := &BenchmarkReport{
		GeneratedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Results: []ScenarioResult{
			{
				Scenario: BenchmarkScenario{
					Name:     "cold-start-to-warm",
					Sessions: 20,
				},
				SteadyStateSession: 7,
				AvgPrecision:       0.93,
				AvgRecall:          0.89,
				AvgF1:              0.91,
				AvgTokenEfficiency: 0.12,
			},
			{
				Scenario: BenchmarkScenario{
					Name:     "mistake-prevention",
					Sessions: 5,
				},
				SteadyStateSession:    -1,
				AvgPrecision:          0.85,
				AvgRecall:             0.80,
				AvgF1:                 0.82,
				AvgTokenEfficiency:    0.15,
				MistakePreventionRate: 0.90,
			},
		},
		Summary: ReportSummary{
			BestF1Scenario:         "cold-start-to-warm",
			BestEfficiencyScenario: "cold-start-to-warm",
			OverallAvgF1:           0.865,
			OverallAvgEfficiency:   0.135,
		},
	}

	var buf bytes.Buffer
	r := &ASCIIRenderer{Writer: &buf}
	err := r.Render(report)
	require.NoError(t, err)

	output := buf.String()

	// Verify structural requirements
	assert.True(t, strings.HasPrefix(output, "┌"), "output must start with ┌")
	assert.True(t, strings.HasSuffix(strings.TrimRight(output, "\n"), "┘"), "output must end with ┘")
	assert.Contains(t, output, "mnemos Retrieval Quality Benchmark")
	assert.NotContains(t, output, "token multiplier")
	assert.NotContains(t, output, "savings")
	assert.Contains(t, output, "cold-start-to-warm")
	assert.Contains(t, output, "mistake-prevention")
	assert.Contains(t, output, "steady @s7")
	assert.Contains(t, output, "0.91")         // AvgF1
	assert.Contains(t, output, "12% of store") // TokenEfficiency
}
