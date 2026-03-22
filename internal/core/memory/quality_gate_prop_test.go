package memory

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
	"pgregory.net/rapid"
)

// printableRune generates printable ASCII runes suitable for content strings.
var printableRunes = []rune("abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._/-()")

func testGateProp() *QualityGate {
	return NewQualityGate(config.DefaultConfig().QualityGate)
}

// **Validates: Requirements 3.6**
// Property 7.2: verdict.Score ∈ [0.0, 1.0] for any arbitrary content string and memory type.
func TestProp_ScoreBounds(t *testing.T) {
	gate := testGateProp()
	memTypes := []domain.MemoryType{
		domain.MemoryTypeShortTerm,
		domain.MemoryTypeLongTerm,
		domain.MemoryTypeEpisodic,
		domain.MemoryTypeSemantic,
	}

	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.StringOf(rapid.RuneFrom(printableRunes)).Draw(rt, "content")
		if content == "" {
			rt.Skip("gate requires non-empty content")
		}
		memType := rapid.SampledFrom(memTypes).Draw(rt, "mem_type")

		req := &domain.StoreRequest{
			Content: content,
			Type:    memType,
		}
		verdict := gate.Evaluate(req, nil)

		if verdict.Score < 0.0 || verdict.Score > 1.0 {
			rt.Fatalf("Score = %f is outside [0.0, 1.0]", verdict.Score)
		}
	})
}

// **Validates: Requirements 3.6**
// Property 7.3: Adding a penalty issue never increases the score.
// We test this by computing scores for issue sets A and B = A + one extra issue,
// and asserting scoreB <= scoreA.
func TestProp_MoreIssuesNeverIncreaseScore(t *testing.T) {
	gate := testGateProp()

	allIssueTypes := []QualityIssue{
		{Type: IssueTooLong, Fix: FixSummarize},
		{Type: IssueLowDensity, Fix: FixCompact},
		{Type: IssueNearDuplicate, Fix: FixMerge},
		{Type: IssueTooGeneric, Fix: FixDowngrade},
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Pick a subset size for A (0 to len-1 so we can always add one more)
		subsetSize := rapid.IntRange(0, len(allIssueTypes)-1).Draw(rt, "subset_size")

		// Build issue set A as the first subsetSize issues
		issuesA := allIssueTypes[:subsetSize]

		// Build issue set B = A + one extra issue
		extraIdx := rapid.IntRange(subsetSize, len(allIssueTypes)-1).Draw(rt, "extra_idx")
		issuesB := append(issuesA, allIssueTypes[extraIdx]) //nolint:gocritic

		scoreA := gate.computeScore(issuesA)
		scoreB := gate.computeScore(issuesB)

		if scoreB > scoreA {
			rt.Fatalf("scoreB (%f) > scoreA (%f): adding an issue increased the score", scoreB, scoreA)
		}
	})
}

// **Validates: Requirements 3.7, 3.8**
// Property 7.4: No issues → VerdictAccept with score == 1.0.
// We construct a StoreRequest guaranteed to pass all checks:
// 7+ words, high density (code-like), specific identifiers, no duplicates.
func TestProp_NoIssuesImpliesAccept(t *testing.T) {
	gate := testGateProp()

	// These templates all have: 7+ words, camelCase/file-path identifiers, high density.
	goodTemplates := []string{
		"SessionStore.Close() needs mutex for thread safety in auth/session.go",
		"handleAuth() validates JWT_SECRET token expiry in middleware/auth.go",
		"DatabasePool.Connect() uses connection pooling in storage/sqlite/store.go",
		"RateLimiter.Allow() implements token bucket algorithm in api/limiter.go",
		"CacheManager.Evict() applies LRU policy in internal/cache/manager.go",
	}

	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.SampledFrom(goodTemplates).Draw(rt, "content")

		req := &domain.StoreRequest{
			Content: content,
			Type:    domain.MemoryTypeShortTerm, // avoid specificity check
		}
		verdict := gate.Evaluate(req, nil)

		if len(verdict.Issues) != 0 {
			rt.Fatalf("expected no issues for %q, got %v", content, verdict.Issues)
		}
		if verdict.Action != VerdictAccept {
			rt.Fatalf("expected VerdictAccept, got %q (score=%f)", verdict.Action, verdict.Score)
		}
		if verdict.Score != 1.0 {
			rt.Fatalf("expected score=1.0, got %f", verdict.Score)
		}
	})
}

// **Validates: Requirements 1.4**
// Property 7.5: InformationDensity(text) ∈ [0.0, 1.0] for all non-empty strings.
func TestProp_DensityBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		text := rapid.StringOf(rapid.RuneFrom(printableRunes)).Draw(rt, "text")
		if text == "" {
			rt.Skip("density is defined as 0.0 for empty string, not in scope")
		}

		density := util.InformationDensity(text)

		if density < 0.0 || density > 1.0 {
			rt.Fatalf("InformationDensity(%q) = %f is outside [0.0, 1.0]", text, density)
		}
	})
}

// **Validates: Requirements 3.7**
// Property 7.6: Verdict action is consistent with score bands.
// If score >= 0.8 and no FixReject/FixMerge/FixDowngrade override → VerdictAccept.
// If score < 0.3 and no FixReject/FixMerge/FixDowngrade override → VerdictReject.
func TestProp_VerdictConsistentWithScoreBands(t *testing.T) {
	gate := testGateProp()
	sb := config.DefaultConfig().QualityGate.ScoreBands

	// Issues that do NOT trigger FixReject, FixMerge, or FixDowngrade overrides.
	safeIssues := []QualityIssue{
		{Type: IssueTooLong, Fix: FixSummarize},
		{Type: IssueLowDensity, Fix: FixCompact},
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Pick a subset of safe issues (no FixReject/FixMerge/FixDowngrade)
		n := rapid.IntRange(0, len(safeIssues)).Draw(rt, "num_issues")
		issues := safeIssues[:n]

		score := gate.computeScore(issues)
		action := gate.verdictFromScore(score, issues)

		// If score >= accept threshold → must be VerdictAccept
		if score >= sb.Accept && action != VerdictAccept {
			rt.Fatalf("score=%f >= accept threshold %f but action=%q (want VerdictAccept)",
				score, sb.Accept, action)
		}

		// If score < downgrade threshold → must be VerdictReject
		if score < sb.Downgrade && action != VerdictReject {
			rt.Fatalf("score=%f < downgrade threshold %f but action=%q (want VerdictReject)",
				score, sb.Downgrade, action)
		}
	})
}

// **Validates: Requirements 1.6**
// Property 7.7: CompactContent never returns empty for non-empty input.
func TestProp_CompactContentNeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringOf(rapid.RuneFrom(printableRunes)).Draw(rt, "s")
		if s == "" {
			rt.Skip("empty input is a special case — CompactContent returns empty for empty")
		}

		result := util.CompactContent(s)

		if result == "" {
			rt.Fatalf("CompactContent(%q) returned empty string", s)
		}
	})
}

// **Validates: Requirements 1.6**
// Property 7.8: len(CompactContent(s)) <= len(s) for all strings.
func TestProp_CompactContentNeverLonger(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringOf(rapid.RuneFrom(printableRunes)).Draw(rt, "s")

		result := util.CompactContent(s)

		if len(result) > len(s) {
			rt.Fatalf("len(CompactContent(%q)) = %d > len(s) = %d", s, len(result), len(s))
		}
	})
}
