package memory

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// stripFencedCodeBlocks removes all ``` ... ``` regions from text.
// Content inside fences is discarded entirely.
func stripFencedCodeBlocks(text string) string {
	var b strings.Builder
	inFence := false
	i := 0
	for i < len(text) {
		if i+2 < len(text) && text[i] == '`' && text[i+1] == '`' && text[i+2] == '`' {
			inFence = !inFence
			i += 3
		} else if !inFence {
			b.WriteByte(text[i])
			i++
		} else {
			i++ // discard content inside fence
		}
	}
	return b.String()
}

// splitSentences splits text into sentences after pre-stripping fenced code blocks.
// Splits on ". ", "! ", "? ", and "\n\n".
// Does NOT split on "." immediately preceded by a digit (preserves "1. ", "0.7", etc.).
// Inline backtick characters are preserved.
// Returns an empty slice for empty input.
// All returned sentences are trimmed of leading/trailing whitespace.
func splitSentences(text string) []string {
	if text == "" {
		return []string{}
	}

	// Step 1: strip fenced code blocks
	text = stripFencedCodeBlocks(text)

	// Step 2: split on terminators
	var sentences []string
	var current strings.Builder

	i := 0
	for i < len(text) {
		// Check for "\n\n"
		if i+1 < len(text) && text[i] == '\n' && text[i+1] == '\n' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
			i += 2
			continue
		}

		// Check for ". ", "! ", "? " — but not digit-preceded "."
		if i+1 < len(text) && text[i+1] == ' ' {
			ch := text[i]
			if ch == '!' || ch == '?' {
				current.WriteByte(ch)
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
				i += 2 // skip the terminator and the space
				continue
			}
			if ch == '.' {
				// Only split if NOT preceded by a digit
				prevIsDigit := current.Len() > 0 && isDigit(current.String()[current.Len()-1])
				if !prevIsDigit {
					current.WriteByte(ch)
					s := strings.TrimSpace(current.String())
					if s != "" {
						sentences = append(sentences, s)
					}
					current.Reset()
					i += 2 // skip '.' and ' '
					continue
				}
			}
		}

		current.WriteByte(text[i])
		i++
	}

	// Flush remaining
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}

	if sentences == nil {
		return []string{}
	}
	return sentences
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// causalWords is the set of causal indicator words for informationScore bonuses.
var causalWords = map[string]bool{
	"because": true, "caused": true, "fixed": true, "reason": true,
	"due": true, "therefore": true, "resulted": true, "introduced": true,
	"resolved": true,
}

// isCamelCase returns true if the token contains at least one uppercase letter,
// at least one lowercase letter, and is not ALL_CAPS (i.e. not all uppercase/digit/underscore).
func isCamelCase(token string) bool {
	hasUpper := false
	hasLower := false
	for _, r := range token {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsLower(r) {
			hasLower = true
		}
	}
	return hasUpper && hasLower
}

// isFilePath returns true if the token contains "/" or ends with a known source extension.
func isFilePath(token string) bool {
	if strings.Contains(token, "/") {
		return true
	}
	extensions := []string{".go", ".ts", ".py", ".js", ".rs", ".java", ".rb", ".cpp", ".c", ".h"}
	for _, ext := range extensions {
		if strings.HasSuffix(token, ext) {
			return true
		}
	}
	return false
}

// isConfigKey returns true if the token is ALL_CAPS with at least one underscore and 5+ chars.
// Pattern: starts with [A-Z], rest are [A-Z0-9_], contains underscore, length >= 5.
func isConfigKey(token string) bool {
	if len(token) < 5 {
		return false
	}
	hasUnderscore := false
	for i, r := range token {
		if i == 0 {
			if !unicode.IsUpper(r) {
				return false
			}
		} else {
			if !unicode.IsUpper(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
		if r == '_' {
			hasUnderscore = true
		}
	}
	return hasUnderscore
}

// informationScore computes an information-density score for a sentence.
// Higher scores indicate more information-dense sentences.
// The score is clamped to a minimum of 0.0.
func informationScore(sentence string, isFirst bool) float64 {
	words := strings.Fields(sentence)
	totalWords := len(words)

	if totalWords == 0 {
		return 0.0
	}

	// Base score: lexical density (unique / total)
	uniqueWords := make(map[string]bool, totalWords)
	for _, w := range words {
		uniqueWords[strings.ToLower(w)] = true
	}
	score := float64(len(uniqueWords)) / float64(totalWords)

	// Bonus: isFirst
	if isFirst {
		score += 0.3
	}

	// Bonus: unique CamelCase tokens (+0.2 each)
	seenCamel := make(map[string]bool)
	for _, w := range words {
		if !seenCamel[w] && isCamelCase(w) {
			seenCamel[w] = true
			score += 0.2
		}
	}

	// Bonus: unique file-path-like tokens (+0.15 each)
	seenPath := make(map[string]bool)
	for _, w := range words {
		if !seenPath[w] && isFilePath(w) {
			seenPath[w] = true
			score += 0.15
		}
	}

	// Bonus: unique causal words (+0.1 each)
	seenCausal := make(map[string]bool)
	for _, w := range words {
		lower := strings.ToLower(w)
		if !seenCausal[lower] && causalWords[lower] {
			seenCausal[lower] = true
			score += 0.1
		}
	}

	// Bonus: unique config-key tokens (+0.1 each)
	seenConfig := make(map[string]bool)
	for _, w := range words {
		if !seenConfig[w] && isConfigKey(w) {
			seenConfig[w] = true
			score += 0.1
		}
	}

	// Penalty: short sentence (< 4 words)
	if totalWords < 4 {
		score -= 0.1
	}

	// Penalty: high stop-word ratio (> 0.6)
	stopCount := 0
	for _, w := range words {
		if util.StopWords[strings.ToLower(w)] {
			stopCount++
		}
	}
	if float64(stopCount)/float64(totalWords) > 0.6 {
		score -= 0.05
	}

	return math.Max(0.0, score)
}

// scoredSentence holds a sentence with its original position and computed score.
type scoredSentence struct {
	text          string
	originalIndex int
	score         float64
}

// ExtractSummary extracts a summary from content using extractive summarization.
// Returns "" for empty content or content with word count ≤ 10.
// Selects up to 3 sentences scored by informationScore plus per-type modifiers,
// restores them to original document order, and truncates to maxWords words.
func ExtractSummary(content string, memType domain.MemoryType, maxWords int) string {
	if content == "" {
		return ""
	}
	if len(strings.Fields(content)) <= 10 {
		return ""
	}

	sentences := splitSentences(content)
	if len(sentences) == 0 {
		return ""
	}

	lastIdx := len(sentences) - 1

	scored := make([]scoredSentence, len(sentences))
	for i, s := range sentences {
		base := informationScore(s, i == 0)

		var modifier float64
		switch memType {
		case domain.MemoryTypeSkill:
			if sentenceHasStepPattern(s) {
				modifier += 0.4
			}
			if strings.Contains(s, "`") {
				modifier += 0.3
			}
		case domain.MemoryTypeCompiled:
			if i == 0 {
				modifier += 0.3
			}
			if i == lastIdx {
				modifier += 0.3
			}
		case domain.MemoryTypeEpisodic, domain.MemoryTypeLongTerm:
			if i == lastIdx {
				modifier += 0.2
			}
			// short_term, semantic, unknown: no modifier
		}

		scored[i] = scoredSentence{
			text:          s,
			originalIndex: i,
			score:         base + modifier,
		}
	}

	// Sort descending by score
	sort.Slice(scored, func(a, b int) bool {
		return scored[a].score > scored[b].score
	})

	// Take top min(3, len) sentences
	take := 3
	if len(scored) < take {
		take = len(scored)
	}
	top := scored[:take]

	// Re-sort by original index to restore document order
	sort.Slice(top, func(a, b int) bool {
		return top[a].originalIndex < top[b].originalIndex
	})

	// Join and truncate to maxWords
	joined := strings.Join(func() []string {
		out := make([]string, len(top))
		for i, ss := range top {
			out[i] = ss.text
		}
		return out
	}(), " ")

	words := strings.Fields(joined)
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, " ")
}

// sentenceHasStepPattern returns true if any line within the sentence starts with
// a digit followed by ".", "-", or "*" (after trimming whitespace).
func sentenceHasStepPattern(sentence string) bool {
	for _, line := range strings.Split(sentence, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		// digit followed by "."
		if len(trimmed) >= 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' {
			return true
		}
		// bullet "-" or "*"
		if trimmed[0] == '-' || trimmed[0] == '*' {
			return true
		}
	}
	return false
}
