package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestJaccardSimilarity_Identical(t *testing.T) {
	a := TokenSet(Tokenize("the quick brown fox"))
	assert.Equal(t, 1.0, JaccardSimilarity(a, a))
}

func TestJaccardSimilarity_Disjoint(t *testing.T) {
	a := TokenSet(Tokenize("apple banana cherry"))
	b := TokenSet(Tokenize("dog elephant frog"))
	assert.Equal(t, 0.0, JaccardSimilarity(a, b))
}

func TestJaccardSimilarity_Partial(t *testing.T) {
	a := TokenSet(Tokenize("apple banana cherry"))
	b := TokenSet(Tokenize("apple banana grape"))
	score := JaccardSimilarity(a, b)
	assert.Greater(t, score, 0.0)
	assert.Less(t, score, 1.0)
}

func TestCosineSimilarity_Identical(t *testing.T) {
	v := []float32{1, 2, 3}
	score := CosineSimilarity(v, v)
	assert.InDelta(t, 1.0, score, 0.0001)
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	assert.InDelta(t, 0.0, CosineSimilarity(a, b), 0.0001)
}

// Property: Jaccard similarity is always in [0, 1]
func TestJaccardSimilarity_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		words := rapid.SliceOf(rapid.StringMatching(`[a-z]{2,8}`)).Draw(t, "words")
		if len(words) == 0 {
			return
		}
		mid := len(words) / 2
		a := TokenSet(words[:mid+1])
		b := TokenSet(words[mid:])
		score := JaccardSimilarity(a, b)
		if score < 0 || score > 1 {
			t.Fatalf("Jaccard score %f out of [0,1]", score)
		}
	})
}

// --- CountWords tests ---

func TestCountWords_Empty(t *testing.T) {
	assert.Equal(t, 0, CountWords(""))
}

func TestCountWords_Single(t *testing.T) {
	assert.Equal(t, 1, CountWords("hello"))
}

func TestCountWords_Two(t *testing.T) {
	assert.Equal(t, 2, CountWords("hello world"))
}

func TestCountWords_MultipleSpaces(t *testing.T) {
	assert.Equal(t, 2, CountWords("  spaces  between  "))
}

func TestCountWords_Newline(t *testing.T) {
	assert.Equal(t, 2, CountWords("newline\nhere"))
}

// --- InformationDensity tests ---

func TestInformationDensity_Empty(t *testing.T) {
	assert.Equal(t, 0.0, InformationDensity(""))
}

func TestInformationDensity_AllStopWords(t *testing.T) {
	// "the the the" — all stop words, Tokenize returns nothing → density = 0.0
	assert.Equal(t, 0.0, InformationDensity("the the the"))
}

func TestInformationDensity_HighDensity(t *testing.T) {
	// Code-heavy content should have high density
	score := InformationDensity("SessionStore.Close() mutex race condition")
	assert.GreaterOrEqual(t, score, 0.7)
}

func TestInformationDensity_LowDensity(t *testing.T) {
	// Filler-heavy sentence should have low density
	score := InformationDensity("We looked at things and discussed the system")
	assert.LessOrEqual(t, score, 0.3)
}

// --- HasProjectSpecificIdentifiers tests ---

func TestHasProjectSpecificIdentifiers_CamelCase(t *testing.T) {
	assert.True(t, HasProjectSpecificIdentifiers("SessionStore"))
}

func TestHasProjectSpecificIdentifiers_FilePath(t *testing.T) {
	assert.True(t, HasProjectSpecificIdentifiers("auth/middleware.go"))
}

func TestHasProjectSpecificIdentifiers_UpperSnake(t *testing.T) {
	assert.True(t, HasProjectSpecificIdentifiers("JWT_SECRET"))
}

func TestHasProjectSpecificIdentifiers_DottedCall(t *testing.T) {
	assert.True(t, HasProjectSpecificIdentifiers("store.Close()"))
}

func TestHasProjectSpecificIdentifiers_Generic(t *testing.T) {
	assert.False(t, HasProjectSpecificIdentifiers("the project uses Go"))
}

func TestHasProjectSpecificIdentifiers_Empty(t *testing.T) {
	assert.False(t, HasProjectSpecificIdentifiers(""))
}

// --- CompactContent tests ---

func TestCompactContent_FillerRemoved(t *testing.T) {
	input := "Basically the server crashed because of a nil pointer"
	result := CompactContent(input)
	assert.NotContains(t, result, "basically")
	assert.NotContains(t, result, "Basically")
	assert.Contains(t, result, "server crashed")
}

func TestCompactContent_CodeIdentifierPreserved(t *testing.T) {
	input := "SessionStore.Close() causes a race condition"
	result := CompactContent(input)
	assert.Contains(t, result, "SessionStore.Close()")
}

func TestCompactContent_EmptyInput(t *testing.T) {
	assert.Equal(t, "", CompactContent(""))
}

func TestCompactContent_NoFillers(t *testing.T) {
	input := "mutex deadlock in worker pool"
	result := CompactContent(input)
	assert.Equal(t, input, result)
}

func TestCompactContent_NeverLongerThanInput(t *testing.T) {
	inputs := []string{
		"basically the system works",
		"it turns out that the cache is stale",
		"we spent time on the auth module",
		"short",
		"",
	}
	for _, input := range inputs {
		result := CompactContent(input)
		assert.LessOrEqual(t, len(result), len(input), "result should never be longer than input for: %q", input)
	}
}
