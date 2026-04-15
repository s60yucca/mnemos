//go:build benchmark

package benchmark

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuiltinScenarios_RunAll verifies that all built-in scenarios run without error
// and produce metrics within valid ranges.
func TestBuiltinScenarios_RunAll(t *testing.T) {
	ctx := context.Background()
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))

	scenarios := BuiltinScenarios()
	assert.GreaterOrEqual(t, len(scenarios), 5)

	report, err := runner.Run(ctx, scenarios)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, len(scenarios), len(report.Results))

	for _, result := range report.Results {
		for _, m := range result.Sessions {
			assert.GreaterOrEqual(t, m.ContextPrecision, 0.0, "scenario %s: ContextPrecision must be >= 0", result.Scenario.Name)
			assert.LessOrEqual(t, m.ContextPrecision, 1.0, "scenario %s: ContextPrecision must be <= 1", result.Scenario.Name)

			assert.GreaterOrEqual(t, m.ContextRecall, 0.0, "scenario %s: ContextRecall must be >= 0", result.Scenario.Name)
			assert.LessOrEqual(t, m.ContextRecall, 1.0, "scenario %s: ContextRecall must be <= 1", result.Scenario.Name)

			assert.GreaterOrEqual(t, m.F1Score, 0.0, "scenario %s: F1Score must be >= 0", result.Scenario.Name)
			assert.LessOrEqual(t, m.F1Score, 1.0, "scenario %s: F1Score must be <= 1", result.Scenario.Name)

			assert.GreaterOrEqual(t, m.TokenEfficiency, 0.0, "scenario %s: TokenEfficiency must be >= 0", result.Scenario.Name)
			assert.LessOrEqual(t, m.TokenEfficiency, 1.0, "scenario %s: TokenEfficiency must be <= 1", result.Scenario.Name)

			assert.LessOrEqual(t, m.GotchasRetrieved, m.TotalGotchas, "scenario %s: GotchasRetrieved must be <= TotalGotchas", result.Scenario.Name)
		}
	}
}
