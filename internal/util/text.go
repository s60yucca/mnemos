package util

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Compiled regex patterns for HasProjectSpecificIdentifiers — compiled once at package init.
var (
	reCamelCase  = regexp.MustCompile(`[a-z][a-zA-Z]*[A-Z][a-zA-Z]*`)
	reFilePath   = regexp.MustCompile(`[\w/]+\.\w{1,4}`)
	reUpperSnake = regexp.MustCompile(`[A-Z][A-Z_]{2,}`)
	reDottedCall = regexp.MustCompile(`\w+\.\w+\(\)`)
)

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "but": {}, "in": {},
	"on": {}, "at": {}, "to": {}, "for": {}, "of": {}, "with": {}, "by": {},
	"from": {}, "is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {}, "did": {}, "will": {},
	"would": {}, "could": {}, "should": {}, "may": {}, "might": {}, "can": {},
	"this": {}, "that": {}, "these": {}, "those": {}, "it": {}, "its": {},
}

// Tokenize splits text into lowercase tokens, removing punctuation and stop words
func Tokenize(text string) []string {
	var tokens []string
	for _, word := range strings.Fields(text) {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, word)
		if len(clean) > 1 {
			if _, isStop := stopWords[clean]; !isStop {
				tokens = append(tokens, clean)
			}
		}
	}
	return tokens
}

// TokenSet returns a set of unique tokens
func TokenSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}

// JaccardSimilarity computes Jaccard similarity between two token sets
func JaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// CosineSimilarity computes cosine similarity between two float32 vectors
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Shingles generates k-shingles from a token list
func Shingles(tokens []string, k int) map[string]struct{} {
	set := make(map[string]struct{})
	if len(tokens) < k {
		set[strings.Join(tokens, " ")] = struct{}{}
		return set
	}
	for i := 0; i <= len(tokens)-k; i++ {
		set[strings.Join(tokens[i:i+k], " ")] = struct{}{}
	}
	return set
}

// ShingleSimilarity computes Jaccard similarity on k-shingles
func ShingleSimilarity(a, b string, k int) float64 {
	tokA := Tokenize(a)
	tokB := Tokenize(b)
	shA := Shingles(tokA, k)
	shB := Shingles(tokB, k)
	return JaccardSimilarity(shA, shB)
}

// CountWords returns the number of whitespace-separated words in text.
// Empty string returns 0.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// InformationDensity returns the ratio of unique meaningful words to total words.
// Formula: count(unique meaningful words) / count(total words)
// Meaningful = not in StopWords. Range: [0.0, 1.0]. Returns 0.0 for empty string.
func InformationDensity(text string) float64 {
	total := CountWords(text)
	if total == 0 {
		return 0.0
	}
	// Count unique meaningful words (not in StopWords, after lowercasing and stripping punctuation)
	unique := make(map[string]struct{})
	for _, word := range strings.Fields(text) {
		clean := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, word)
		if len(clean) > 1 && !StopWords[clean] {
			unique[clean] = struct{}{}
		}
	}
	density := float64(len(unique)) / float64(total)
	if density > 1.0 {
		return 1.0
	}
	return density
}

// HasProjectSpecificIdentifiers returns true if text contains at least one of:
//   - camelCase word:   [a-z][a-zA-Z]*[A-Z][a-zA-Z]*  (sessionStore, handleAuth)
//   - file path:        [\w/]+\.\w{1,4}                (auth/middleware.go)
//   - UPPER_SNAKE key:  [A-Z][A-Z_]{2,}               (JWT_SECRET)
//   - dotted call:      \w+\.\w+\(\)                   (store.Close())
//
// All four patterns are compiled once at package init and reused.
func HasProjectSpecificIdentifiers(text string) bool {
	return reCamelCase.MatchString(text) ||
		reFilePath.MatchString(text) ||
		reUpperSnake.MatchString(text) ||
		reDottedCall.MatchString(text)
}

// CompactContent removes filler phrases and collapses whitespace.
// Step 1: Remove filler phrases (case-insensitive, longest-first).
// Step 2: Collapse multiple spaces/newlines to a single space.
// Step 3: If result is shorter than original, return result; else return original.
// Never returns empty string — falls back to original if compaction produces nothing.
func CompactContent(text string) string {
	if text == "" {
		return text
	}
	result := text
	for _, phrase := range FillerPhrases {
		lowerPhrase := strings.ToLower(phrase)
		for {
			idx := strings.Index(strings.ToLower(result), lowerPhrase)
			if idx == -1 {
				break
			}
			result = result[:idx] + result[idx+len(phrase):]
		}
	}
	// Collapse multiple whitespace characters (spaces, tabs, newlines) to single space
	fields := strings.Fields(result)
	result = strings.Join(fields, " ")
	result = strings.TrimSpace(result)
	if result == "" || len(result) >= len(text) {
		return text
	}
	return result
}
