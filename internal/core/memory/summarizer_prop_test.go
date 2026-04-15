package memory

import (
	"strings"
	"testing"
	"unicode"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"pgregory.net/rapid"
)

// printableRunesSummarizer is the set of printable ASCII runes for generating test strings.
// Named distinctly to avoid conflict with printableRunes in quality_gate_prop_test.go.
var printableRunesSummarizer = func() []rune {
	var runes []rune
	for r := rune(32); r < 127; r++ {
		if unicode.IsPrint(r) {
			runes = append(runes, r)
		}
	}
	return runes
}()

// allMemTypes is the set of all valid memory types for property tests.
var allMemTypes = []domain.MemoryType{
	domain.MemoryTypeShortTerm,
	domain.MemoryTypeLongTerm,
	domain.MemoryTypeEpisodic,
	domain.MemoryTypeSemantic,
	domain.MemoryTypeSkill,
	domain.MemoryTypeCompiled,
	"", // unknown/empty type
}

// **Validates: Requirements P1**
// P1 (Word Count): wordCount(ExtractSummary(content, memType, maxWords)) ≤ maxWords
func TestProp_ExtractSummary_WordCountBound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.StringOf(rapid.RuneFrom(printableRunesSummarizer)).Draw(rt, "content")
		memType := rapid.SampledFrom(allMemTypes).Draw(rt, "mem_type")
		maxWords := rapid.IntRange(1, 100).Draw(rt, "max_words")

		result := ExtractSummary(content, memType, maxWords)
		words := strings.Fields(result)
		if len(words) > maxWords {
			rt.Fatalf("word count %d > maxWords %d for result %q", len(words), maxWords, result)
		}
	})
}

// **Validates: Requirements P2**
// P2 (Extractive Guarantee): every whitespace-delimited token in ExtractSummary result
// is a substring of the original content.
func TestProp_ExtractSummary_Extractive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.StringOf(rapid.RuneFrom(printableRunesSummarizer)).Draw(rt, "content")
		memType := rapid.SampledFrom(allMemTypes).Draw(rt, "mem_type")
		maxWords := rapid.IntRange(1, 100).Draw(rt, "max_words")

		result := ExtractSummary(content, memType, maxWords)
		for _, token := range strings.Fields(result) {
			if !strings.Contains(content, token) {
				rt.Fatalf("token %q from result %q not found in content %q", token, result, content)
			}
		}
	})
}

// **Validates: Requirements P3**
// P3 (Non-negative Score): informationScore(s, isFirst) ≥ 0.0 for any string s.
func TestProp_InformationScore_NonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringOf(rapid.RuneFrom(printableRunesSummarizer)).Draw(rt, "s")
		isFirst := rapid.Bool().Draw(rt, "is_first")

		score := informationScore(s, isFirst)
		if score < 0.0 {
			rt.Fatalf("informationScore(%q, %v) = %f < 0.0", s, isFirst, score)
		}
	})
}

// **Validates: Requirements P4**
// P4 (Stability): len(ExtractSummary(content, memType, maxWords)) ≤ len(content)
func TestProp_ExtractSummary_NeverLonger(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.StringOf(rapid.RuneFrom(printableRunesSummarizer)).Draw(rt, "content")
		memType := rapid.SampledFrom(allMemTypes).Draw(rt, "mem_type")
		maxWords := rapid.IntRange(1, 100).Draw(rt, "max_words")

		result := ExtractSummary(content, memType, maxWords)
		if len(result) > len(content) {
			rt.Fatalf("len(result)=%d > len(content)=%d", len(result), len(content))
		}
	})
}
