package benchmark

import "math"

// DetectSteadyState returns the SessionNumber of the first session where the
// coefficient of variation (stddev/mean) of F1Score over a sliding window of
// windowSize sessions drops below threshold.
// Returns -1 if steady state is never reached.
// If windowSize < 2 or len(metrics) < windowSize, returns -1.
func DetectSteadyState(metrics []SessionMetrics, windowSize int, threshold float64) int {
	if windowSize < 2 || len(metrics) < windowSize {
		return -1
	}

	for i := windowSize; i <= len(metrics); i++ {
		window := metrics[i-windowSize : i]

		// Collect F1 scores and compute mean
		mean := 0.0
		for _, m := range window {
			mean += m.F1Score
		}
		mean /= float64(len(window))

		// Compute coefficient of variation
		var cv float64
		if mean == 0 {
			cv = 0 // treat as stable
		} else {
			// Population stddev
			variance := 0.0
			for _, m := range window {
				diff := m.F1Score - mean
				variance += diff * diff
			}
			variance /= float64(len(window))
			stddev := math.Sqrt(variance)
			cv = stddev / mean
		}

		if cv < threshold {
			return window[0].SessionNumber
		}
	}

	return -1
}
