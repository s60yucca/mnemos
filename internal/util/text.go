package util

import (
	"math"
	"strings"
	"unicode"
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
