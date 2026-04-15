//go:build benchmark

package benchmark

// **Validates: Requirements 2.3**

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"pgregory.net/rapid"
)

// genMemoryIDs generates a slice of unique string IDs.
func genMemoryIDs(t *rapid.T, label string, maxLen int) []string {
	n := rapid.IntRange(0, maxLen).Draw(t, label+"_n")
	ids := make([]string, n)
	seen := make(map[string]bool, n)
	for i := range ids {
		var id string
		for {
			id = rapid.StringMatching(`[a-z][a-z0-9]{0,7}`).Draw(t, label+"_id")
			if !seen[id] {
				break
			}
		}
		seen[id] = true
		ids[i] = id
	}
	return ids
}

// buildMemories converts a slice of IDs into *domain.Memory values with some content.
func buildMemories(ids []string) []*domain.Memory {
	mems := make([]*domain.Memory, len(ids))
	for i, id := range ids {
		mems[i] = &domain.Memory{ID: id, Content: id + " content data"}
	}
	return mems
}

// subset picks a random subset of ids.
func subset(t *rapid.T, ids []string, label string) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if rapid.Bool().Draw(t, label+"_include_"+id) {
			result = append(result, id)
		}
	}
	return result
}

func TestProperty_ContextPrecisionInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allIDs := genMemoryIDs(t, "all", 10)
		retrievedIDs := subset(t, allIDs, "retrieved")
		relevantIDs := subset(t, allIDs, "relevant")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(allIDs)
		task := Task{RelevantMemoryIDs: relevantIDs}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.ContextPrecision < 0.0 || m.ContextPrecision > 1.0 {
			t.Fatalf("ContextPrecision %v out of [0,1]", m.ContextPrecision)
		}
	})
}

func TestProperty_ContextRecallInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allIDs := genMemoryIDs(t, "all", 10)
		retrievedIDs := subset(t, allIDs, "retrieved")
		relevantIDs := subset(t, allIDs, "relevant")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(allIDs)
		task := Task{RelevantMemoryIDs: relevantIDs}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.ContextRecall < 0.0 || m.ContextRecall > 1.0 {
			t.Fatalf("ContextRecall %v out of [0,1]", m.ContextRecall)
		}
	})
}

func TestProperty_F1ScoreInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allIDs := genMemoryIDs(t, "all", 10)
		retrievedIDs := subset(t, allIDs, "retrieved")
		relevantIDs := subset(t, allIDs, "relevant")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(allIDs)
		task := Task{RelevantMemoryIDs: relevantIDs}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.F1Score < 0.0 || m.F1Score > 1.0 {
			t.Fatalf("F1Score %v out of [0,1]", m.F1Score)
		}
	})
}

func TestProperty_TokenEfficiencyInRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// allStored must be non-empty to keep efficiency meaningful
		storeIDs := genMemoryIDs(t, "store", 10)
		if len(storeIDs) == 0 {
			storeIDs = []string{"seed"}
		}
		retrievedIDs := subset(t, storeIDs, "retrieved")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(storeIDs)
		task := Task{}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.TokenEfficiency < 0.0 || m.TokenEfficiency > 1.0 {
			t.Fatalf("TokenEfficiency %v out of [0,1]", m.TokenEfficiency)
		}
	})
}

func TestProperty_TPplusFPequalsRetrieved(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allIDs := genMemoryIDs(t, "all", 10)
		retrievedIDs := subset(t, allIDs, "retrieved")
		relevantIDs := subset(t, allIDs, "relevant")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(allIDs)
		task := Task{RelevantMemoryIDs: relevantIDs}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.TruePositives+m.FalsePositives != len(retrieved) {
			t.Fatalf("TP(%d) + FP(%d) != len(retrieved)(%d)",
				m.TruePositives, m.FalsePositives, len(retrieved))
		}
	})
}

func TestProperty_GotchasRetrievedLEQTotalGotchas(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allIDs := genMemoryIDs(t, "all", 10)
		retrievedIDs := subset(t, allIDs, "retrieved")
		gotchaIDs := subset(t, allIDs, "gotcha")

		retrieved := buildMemories(retrievedIDs)
		allStored := buildMemories(allIDs)
		task := Task{GotchaMemoryIDs: gotchaIDs}

		eval := &MetricsEvaluator{}
		m := eval.Evaluate(retrieved, allStored, task)

		if m.GotchasRetrieved > m.TotalGotchas {
			t.Fatalf("GotchasRetrieved(%d) > TotalGotchas(%d)",
				m.GotchasRetrieved, m.TotalGotchas)
		}
	})
}
