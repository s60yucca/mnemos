//go:build benchmark

package benchmark

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func makeMetrics(f1s []float64) []SessionMetrics {
	metrics := make([]SessionMetrics, len(f1s))
	for i, f := range f1s {
		metrics[i] = SessionMetrics{
			SessionNumber: i + 1,
			F1Score:       f,
		}
	}
	return metrics
}

func TestDetectSteadyState(t *testing.T) {
	tests := []struct {
		name       string
		metrics    []SessionMetrics
		windowSize int
		threshold  float64
		want       int
	}{
		{
			name:       "flat F1 curve",
			metrics:    makeMetrics([]float64{0.9, 0.9, 0.9, 0.9, 0.9}),
			windowSize: 3,
			threshold:  0.05,
			want:       1, // first window starts at session 1
		},
		{
			// Window [0.8, 0.85, 0.87] (sessions 4,5,6): mean=0.84, stddev≈0.029, CV≈0.035 < 0.05
			// This is the first stable window, so returns session 4.
			name:       "noisy then stable",
			metrics:    makeMetrics([]float64{0.1, 0.9, 0.2, 0.8, 0.85, 0.87, 0.86}),
			windowSize: 3,
			threshold:  0.05,
			want:       4,
		},
		{
			name:       "never converging",
			metrics:    makeMetrics([]float64{0.1, 0.9, 0.1, 0.9, 0.1, 0.9}),
			windowSize: 3,
			threshold:  0.05,
			want:       -1,
		},
		{
			name:       "too few metrics",
			metrics:    makeMetrics([]float64{0.9, 0.9}),
			windowSize: 3,
			threshold:  0.05,
			want:       -1,
		},
		{
			name:       "windowSize less than 2",
			metrics:    makeMetrics([]float64{0.9, 0.9, 0.9}),
			windowSize: 1,
			threshold:  0.05,
			want:       -1,
		},
		{
			name:       "empty metrics",
			metrics:    []SessionMetrics{},
			windowSize: 3,
			threshold:  0.05,
			want:       -1,
		},
		{
			name:       "windowSize zero",
			metrics:    makeMetrics([]float64{0.9, 0.9, 0.9}),
			windowSize: 0,
			threshold:  0.05,
			want:       -1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectSteadyState(tc.metrics, tc.windowSize, tc.threshold)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestProperty_DetectSteadyState_ValidResult verifies that DetectSteadyState
// always returns -1 or a valid SessionNumber from the input slice.
//
// Validates: Requirements 4.1
func TestProperty_DetectSteadyState_ValidResult(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 20).Draw(t, "n")
		windowSize := rapid.IntRange(1, 5).Draw(t, "windowSize")
		threshold := rapid.Float64Range(0.01, 0.5).Draw(t, "threshold")

		metrics := make([]SessionMetrics, n)
		for i := range metrics {
			metrics[i] = SessionMetrics{
				SessionNumber: i + 1,
				F1Score:       rapid.Float64Range(0, 1).Draw(t, fmt.Sprintf("f1_%d", i)),
			}
		}

		result := DetectSteadyState(metrics, windowSize, threshold)

		// Result must be -1 OR a valid SessionNumber within the input
		if result == -1 {
			return // valid
		}
		// Must be a session number that exists in the input
		found := false
		for _, m := range metrics {
			if m.SessionNumber == result {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("DetectSteadyState returned %d which is not a valid session number in input", result)
		}
	})
}
