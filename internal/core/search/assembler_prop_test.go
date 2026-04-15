package search

import (
	"math"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"pgregory.net/rapid"
)

// genTag generates a random lowercase tag string.
var genTag = rapid.StringMatching(`[a-z]{1,10}`)

// genTags generates a slice of 0-5 tags.
var genTags = rapid.SliceOfN(genTag, 0, 5)

// genMemory generates a random *domain.Memory with unique ID.
func genMemory(t *rapid.T, id string) *domain.Memory {
	content := rapid.StringN(1, 200, -1).Draw(t, "content")
	summary := rapid.StringN(0, 100, -1).Draw(t, "summary")
	category := rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "category")
	tags := genTags.Draw(t, "tags")
	score := rapid.Float64Range(0.0, 1.0).Draw(t, "score")
	return &domain.Memory{
		ID:             id,
		Content:        content,
		Summary:        summary,
		Category:       category,
		Tags:           tags,
		RelevanceScore: score,
	}
}

// genCandidates generates a slice of 0-10 memories with unique IDs.
func genCandidates(t *rapid.T) []*domain.Memory {
	n := rapid.IntRange(0, 10).Draw(t, "n")
	mems := make([]*domain.Memory, n)
	for i := range mems {
		mems[i] = genMemory(t, strings.Repeat(string(rune('a'+i%26)), i+1))
	}
	return mems
}

// P1+P2: DiversityFilter result is a subset of candidates with no duplicate IDs.
func TestProperty_DiversityFilter_SubsetNoDuplicates(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		candidates := genCandidates(t)
		budget := rapid.IntRange(1, 10000).Draw(t, "budget")
		lambda := rapid.Float64Range(0.0, 1.0).Draw(t, "lambda")

		a := NewContextAssembler(lambda)
		result := a.DiversityFilter(candidates, budget)

		// Build candidate ID set
		candidateIDs := make(map[string]struct{}, len(candidates))
		for _, m := range candidates {
			candidateIDs[m.ID] = struct{}{}
		}

		// P1: every result element is in candidates
		for _, m := range result {
			if _, ok := candidateIDs[m.ID]; !ok {
				t.Fatalf("P1 violated: result contains ID %q not in candidates", m.ID)
			}
		}

		// P2: no duplicate IDs in result
		seen := make(map[string]struct{}, len(result))
		for _, m := range result {
			if _, dup := seen[m.ID]; dup {
				t.Fatalf("P2 violated: duplicate ID %q in result", m.ID)
			}
			seen[m.ID] = struct{}{}
		}
	})
}

// P3+P4+P5+P6: jaccardSimilarity properties.
func TestProperty_JaccardSimilarity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := genTags.Draw(t, "a")
		b := genTags.Draw(t, "b")

		sim := jaccardSimilarity(a, b)

		// P4: range [0, 1]
		if sim < 0.0 || sim > 1.0 {
			t.Fatalf("P4 violated: jaccardSimilarity(%v, %v) = %f out of [0,1]", a, b, sim)
		}

		// P3: symmetry
		simBA := jaccardSimilarity(b, a)
		if math.Abs(sim-simBA) > 1e-12 {
			t.Fatalf("P3 violated: jaccardSimilarity(%v,%v)=%f != jaccardSimilarity(%v,%v)=%f", a, b, sim, b, a, simBA)
		}
	})
}

func TestProperty_JaccardSimilarity_Identity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// P5: identity — non-empty a → jaccardSimilarity(a, a) == 1.0
		a := rapid.SliceOfN(genTag, 1, 5).Draw(t, "a")
		if got := jaccardSimilarity(a, a); got != 1.0 {
			t.Fatalf("P5 violated: jaccardSimilarity(%v, %v) = %f, want 1.0", a, a, got)
		}
	})
}

func TestProperty_JaccardSimilarity_Empty(t *testing.T) {
	// P6: empty cases
	if got := jaccardSimilarity(nil, nil); got != 0.0 {
		t.Fatalf("P6 violated: jaccardSimilarity(nil, nil) = %f, want 0.0", got)
	}
	if got := jaccardSimilarity([]string{}, []string{}); got != 0.0 {
		t.Fatalf("P6 violated: jaccardSimilarity([], []) = %f, want 0.0", got)
	}
}

// P7: PackWithBudget with budget=MaxInt renders all memories at DetailFull.
func TestProperty_PackWithBudget_MaxBudgetAllFull(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(t, "n")
		memories := make([]*domain.Memory, n)
		for i := range memories {
			memories[i] = genMemory(t, strings.Repeat(string(rune('a'+i%26)), i+1))
		}

		a := NewContextAssembler(0.7)
		result := a.PackWithBudget(memories, math.MaxInt)

		// Every memory's full content must appear in the result
		for _, m := range memories {
			if !strings.Contains(result, m.Content) {
				t.Fatalf("P7 violated: full content of memory %q not in result", m.ID)
			}
		}
	})
}

// P8: DiversityFilter budget invariant — sum of tokens ≤ budget + max single memory tokens.
func TestProperty_DiversityFilter_BudgetInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Non-empty candidates
		n := rapid.IntRange(1, 10).Draw(t, "n")
		candidates := make([]*domain.Memory, n)
		for i := range candidates {
			candidates[i] = genMemory(t, strings.Repeat(string(rune('a'+i%26)), i+1))
		}
		budget := rapid.IntRange(1, 5000).Draw(t, "budget")
		lambda := rapid.Float64Range(0.0, 1.0).Draw(t, "lambda")

		a := NewContextAssembler(lambda)
		result := a.DiversityFilter(candidates, budget)

		if len(result) == 0 {
			t.Fatal("P8: non-empty candidates must produce at least 1 result")
		}

		// Compute max single memory tokens
		maxSingle := 0
		for _, m := range candidates {
			if tok := estimateTokens(m.Content); tok > maxSingle {
				maxSingle = tok
			}
		}

		// Sum tokens of result
		total := 0
		for _, m := range result {
			total += estimateTokens(m.Content)
		}

		if total > budget+maxSingle {
			t.Fatalf("P8 violated: sumTokens=%d > budget(%d)+maxSingle(%d)=%d",
				total, budget, maxSingle, budget+maxSingle)
		}
	})
}
