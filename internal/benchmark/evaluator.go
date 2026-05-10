package benchmark

import (
	"github.com/mnemos-dev/mnemos/internal/domain"
)

// estimateTokens estimates the token count for a string using the same
// formula as the search engine: len(text)/4 + 1.
func estimateTokens(text string) int {
	return len(text)/4 + 1
}

// MetricsEvaluator computes retrieval quality metrics for a single session.
type MetricsEvaluator struct{}

// Evaluate computes SessionMetrics by comparing retrieved memories against
// the task's ground-truth sets (relevant, gotcha, correction IDs).
func (e *MetricsEvaluator) Evaluate(retrieved []*domain.Memory, allStored []*domain.Memory, task Task) SessionMetrics {
	// Build ID sets
	retrievedIDs := make(map[string]struct{}, len(retrieved))
	for _, m := range retrieved {
		retrievedIDs[m.ID] = struct{}{}
	}

	relevantIDs := make(map[string]struct{}, len(task.RelevantMemoryIDs))
	for _, id := range task.RelevantMemoryIDs {
		relevantIDs[id] = struct{}{}
	}

	gotchaIDs := make(map[string]struct{}, len(task.GotchaMemoryIDs))
	for _, id := range task.GotchaMemoryIDs {
		gotchaIDs[id] = struct{}{}
	}

	// Precision, recall, F1
	truePositives := 0
	for id := range retrievedIDs {
		if _, ok := relevantIDs[id]; ok {
			truePositives++
		}
	}
	falsePositives := len(retrievedIDs) - truePositives
	falseNegatives := len(relevantIDs) - truePositives

	var precision float64
	if len(retrievedIDs) > 0 {
		precision = float64(truePositives) / float64(len(retrievedIDs))
	}

	var recall float64
	if len(relevantIDs) > 0 {
		recall = float64(truePositives) / float64(len(relevantIDs))
	} else {
		recall = 1.0 // vacuously true
	}

	var f1 float64
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	// Token efficiency
	totalStoreTokens := 0
	for _, m := range allStored {
		totalStoreTokens += estimateTokens(m.Content)
	}
	tokensConsumed := 0
	for _, m := range retrieved {
		tokensConsumed += estimateTokens(m.Content)
	}
	tokenEfficiency := float64(tokensConsumed) / float64(max(totalStoreTokens, 1))

	// Gotchas retrieved
	gotchasRetrieved := 0
	for id := range retrievedIDs {
		if _, ok := gotchaIDs[id]; ok {
			gotchasRetrieved++
		}
	}

	// Build index map for retrieved slice (ID → position)
	retrievedIndex := make(map[string]int, len(retrieved))
	for i, m := range retrieved {
		retrievedIndex[m.ID] = i
	}

	// Find minimum index of any gotcha present in retrieved
	minGotchaIdx := -1
	for _, gotchaID := range task.GotchaMemoryIDs {
		if idx, ok := retrievedIndex[gotchaID]; ok {
			if minGotchaIdx == -1 || idx < minGotchaIdx {
				minGotchaIdx = idx
			}
		}
	}

	// Count corrections that rank above all present gotchas
	correctionsRankedAbove := 0
	for _, corrID := range task.CorrectionIDs {
		corrIdx, ok := retrievedIndex[corrID]
		if !ok {
			continue // not retrieved, skip
		}
		// If no gotchas are present in retrieved, correction trivially ranks above
		if minGotchaIdx == -1 || corrIdx < minGotchaIdx {
			correctionsRankedAbove++
		}
	}

	return SessionMetrics{
		TokensConsumed:         tokensConsumed,
		TotalStoreTokens:       totalStoreTokens,
		TokenEfficiency:        tokenEfficiency,
		ContextPrecision:       precision,
		ContextRecall:          recall,
		F1Score:                f1,
		TruePositives:          truePositives,
		FalsePositives:         falsePositives,
		FalseNegatives:         falseNegatives,
		TotalGotchas:           len(gotchaIDs),
		GotchasRetrieved:       gotchasRetrieved,
		CorrectionsRankedAbove: correctionsRankedAbove,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
