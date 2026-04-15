//go:build benchmark

package benchmark

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func mem(id, content string) *domain.Memory {
	return &domain.Memory{ID: id, Content: content}
}

func TestMetricsEvaluator_Evaluate(t *testing.T) {
	eval := &MetricsEvaluator{}

	tests := []struct {
		name      string
		retrieved []*domain.Memory
		allStored []*domain.Memory
		task      Task
		want      SessionMetrics
	}{
		{
			name:      "empty retrieved set",
			retrieved: []*domain.Memory{},
			allStored: []*domain.Memory{mem("a", "hello world")},
			task:      Task{RelevantMemoryIDs: []string{"a"}},
			want: SessionMetrics{
				ContextPrecision: 0.0,
				ContextRecall:    0.0,
				F1Score:          0.0,
				TruePositives:    0,
				FalsePositives:   0,
				FalseNegatives:   1,
			},
		},
		{
			name:      "empty retrieved, no relevant — recall vacuously 1",
			retrieved: []*domain.Memory{},
			allStored: []*domain.Memory{mem("a", "hello")},
			task:      Task{RelevantMemoryIDs: []string{}},
			want: SessionMetrics{
				ContextPrecision: 0.0,
				ContextRecall:    1.0,
				F1Score:          0.0,
				TruePositives:    0,
				FalsePositives:   0,
				FalseNegatives:   0,
			},
		},
		{
			name:      "full overlap — retrieved == relevant",
			retrieved: []*domain.Memory{mem("a", "x"), mem("b", "y")},
			allStored: []*domain.Memory{mem("a", "x"), mem("b", "y")},
			task:      Task{RelevantMemoryIDs: []string{"a", "b"}},
			want: SessionMetrics{
				ContextPrecision: 1.0,
				ContextRecall:    1.0,
				F1Score:          1.0,
				TruePositives:    2,
				FalsePositives:   0,
				FalseNegatives:   0,
			},
		},
		{
			name:      "partial overlap",
			retrieved: []*domain.Memory{mem("a", "x"), mem("b", "y"), mem("c", "z")},
			allStored: []*domain.Memory{mem("a", "x"), mem("b", "y"), mem("c", "z")},
			task:      Task{RelevantMemoryIDs: []string{"a", "b", "d"}},
			// TP=2, FP=1, FN=1, P=2/3, R=2/3, F1=2/3
			want: SessionMetrics{
				ContextPrecision: 2.0 / 3.0,
				ContextRecall:    2.0 / 3.0,
				F1Score:          2.0 / 3.0,
				TruePositives:    2,
				FalsePositives:   1,
				FalseNegatives:   1,
			},
		},
		{
			name:      "zero gotchas",
			retrieved: []*domain.Memory{mem("a", "x")},
			allStored: []*domain.Memory{mem("a", "x")},
			task:      Task{RelevantMemoryIDs: []string{"a"}, GotchaMemoryIDs: []string{}},
			want: SessionMetrics{
				ContextPrecision: 1.0,
				ContextRecall:    1.0,
				F1Score:          1.0,
				TruePositives:    1,
				TotalGotchas:     0,
				GotchasRetrieved: 0,
			},
		},
		{
			name:      "zero relevant IDs — recall is 1.0 vacuously",
			retrieved: []*domain.Memory{mem("a", "x")},
			allStored: []*domain.Memory{mem("a", "x")},
			task:      Task{RelevantMemoryIDs: []string{}},
			want: SessionMetrics{
				ContextPrecision: 0.0, // no TP, 1 retrieved → P=0
				ContextRecall:    1.0,
				F1Score:          0.0,
				TruePositives:    0,
				FalsePositives:   1,
				FalseNegatives:   0,
			},
		},
		{
			name: "correction ranks above all gotchas — counts",
			// retrieved order: corr(idx=0), gotcha(idx=1)
			// corr1 is relevant; gotcha1 is not → TP=1, FP=1, P=0.5, R=1.0, F1=2/3
			retrieved: []*domain.Memory{mem("corr1", "fix"), mem("gotcha1", "bad")},
			allStored: []*domain.Memory{mem("corr1", "fix"), mem("gotcha1", "bad")},
			task: Task{
				RelevantMemoryIDs: []string{"corr1"},
				GotchaMemoryIDs:   []string{"gotcha1"},
				CorrectionIDs:     []string{"corr1"},
			},
			want: SessionMetrics{
				ContextPrecision:       0.5,
				ContextRecall:          1.0,
				F1Score:                2.0 / 3.0,
				TruePositives:          1,
				FalsePositives:         1,
				FalseNegatives:         0,
				TotalGotchas:           1,
				GotchasRetrieved:       1,
				CorrectionsRankedAbove: 1,
			},
		},
		{
			name: "correction ranks below a gotcha — does not count",
			// retrieved order: gotcha(idx=0), corr(idx=1)
			retrieved: []*domain.Memory{mem("gotcha1", "bad"), mem("corr1", "fix")},
			allStored: []*domain.Memory{mem("gotcha1", "bad"), mem("corr1", "fix")},
			task: Task{
				RelevantMemoryIDs: []string{"corr1"},
				GotchaMemoryIDs:   []string{"gotcha1"},
				CorrectionIDs:     []string{"corr1"},
			},
			want: SessionMetrics{
				ContextPrecision:       0.5,
				ContextRecall:          1.0,
				F1Score:                2.0 / 3.0,
				TruePositives:          1,
				FalsePositives:         1,
				FalseNegatives:         0,
				TotalGotchas:           1,
				GotchasRetrieved:       1,
				CorrectionsRankedAbove: 0,
			},
		},
		{
			name:      "correction not in retrieved — does not count",
			retrieved: []*domain.Memory{mem("gotcha1", "bad")},
			allStored: []*domain.Memory{mem("gotcha1", "bad"), mem("corr1", "fix")},
			task: Task{
				RelevantMemoryIDs: []string{"corr1"},
				GotchaMemoryIDs:   []string{"gotcha1"},
				CorrectionIDs:     []string{"corr1"},
			},
			want: SessionMetrics{
				ContextPrecision:       0.0,
				ContextRecall:          0.0,
				F1Score:                0.0,
				TruePositives:          0,
				FalsePositives:         1,
				FalseNegatives:         1,
				TotalGotchas:           1,
				GotchasRetrieved:       1,
				CorrectionsRankedAbove: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := eval.Evaluate(tc.retrieved, tc.allStored, tc.task)
			assert.InDelta(t, tc.want.ContextPrecision, got.ContextPrecision, 1e-9, "ContextPrecision")
			assert.InDelta(t, tc.want.ContextRecall, got.ContextRecall, 1e-9, "ContextRecall")
			assert.InDelta(t, tc.want.F1Score, got.F1Score, 1e-9, "F1Score")
			assert.Equal(t, tc.want.TruePositives, got.TruePositives, "TruePositives")
			assert.Equal(t, tc.want.FalsePositives, got.FalsePositives, "FalsePositives")
			assert.Equal(t, tc.want.FalseNegatives, got.FalseNegatives, "FalseNegatives")
			assert.Equal(t, tc.want.TotalGotchas, got.TotalGotchas, "TotalGotchas")
			assert.Equal(t, tc.want.GotchasRetrieved, got.GotchasRetrieved, "GotchasRetrieved")
			assert.Equal(t, tc.want.CorrectionsRankedAbove, got.CorrectionsRankedAbove, "CorrectionsRankedAbove")
		})
	}
}
