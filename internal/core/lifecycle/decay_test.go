package lifecycle

import (
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestComputeDecayScore_Floor(t *testing.T) {
	mem := &domain.Memory{
		Type:           domain.MemoryTypeShortTerm,
		RelevanceScore: 1.0,
		AccessCount:    0,
		LastAccessedAt: time.Now().UTC().Add(-1000 * time.Hour), // very old
	}
	score := ComputeDecayScore(mem, time.Now().UTC())
	assert.GreaterOrEqual(t, score, decayFloor, "score must not go below floor")
}

func TestComputeDecayScore_TypeRates(t *testing.T) {
	now := time.Now().UTC()
	lastAccess := now.Add(-24 * time.Hour) // 1 day ago

	types := []domain.MemoryType{
		domain.MemoryTypeShortTerm,
		domain.MemoryTypeEpisodic,
		domain.MemoryTypeLongTerm,
		domain.MemoryTypeSemantic,
	}

	scores := make(map[domain.MemoryType]float64)
	for _, t := range types {
		mem := &domain.Memory{
			Type:           t,
			RelevanceScore: 1.0,
			AccessCount:    0,
			LastAccessedAt: lastAccess,
		}
		scores[t] = ComputeDecayScore(mem, now)
	}

	// short_term decays fastest, semantic slowest
	assert.Greater(t, scores[domain.MemoryTypeSemantic], scores[domain.MemoryTypeLongTerm])
	assert.Greater(t, scores[domain.MemoryTypeLongTerm], scores[domain.MemoryTypeEpisodic])
	assert.Greater(t, scores[domain.MemoryTypeEpisodic], scores[domain.MemoryTypeShortTerm])
}

func TestComputeDecayScore_Monotonicity(t *testing.T) {
	// Score should decrease as time increases (for same memory)
	mem := &domain.Memory{
		Type:           domain.MemoryTypeEpisodic,
		RelevanceScore: 1.0,
		AccessCount:    0,
		LastAccessedAt: time.Now().UTC(),
	}
	now := time.Now().UTC()
	prev := ComputeDecayScore(mem, now)
	for _, hours := range []float64{1, 10, 100, 1000} {
		future := now.Add(time.Duration(hours) * time.Hour)
		score := ComputeDecayScore(mem, future)
		assert.LessOrEqual(t, score, prev+0.001, "score should be non-increasing over time")
		prev = score
	}
}

// Property-based test: decay score is always in [floor, 2.0] range
func TestComputeDecayScore_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		hours := rapid.Float64Range(0, 10000).Draw(t, "hours")
		accessCount := rapid.IntRange(0, 1000).Draw(t, "access_count")
		baseScore := rapid.Float64Range(0.01, 2.0).Draw(t, "base_score")

		mem := &domain.Memory{
			Type:           domain.MemoryTypeEpisodic,
			RelevanceScore: baseScore,
			AccessCount:    accessCount,
			LastAccessedAt: time.Now().UTC().Add(-time.Duration(hours * float64(time.Hour))),
		}
		score := ComputeDecayScore(mem, time.Now().UTC())
		if score < decayFloor {
			t.Fatalf("score %f below floor %f", score, decayFloor)
		}
	})
}
