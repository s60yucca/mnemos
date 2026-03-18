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
