package lifecycle

import (
	"math"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

const (
	decayFloor = 0.05
	accessBoostFactor = 0.1
)

// ComputeDecayScore computes the exponential decay score for a memory
// score = max(floor, base * e^(-λ*t) * accessBoost * typeMultiplier)
// where t = hours since last access
func ComputeDecayScore(mem *domain.Memory, now time.Time) float64 {
	t := now.Sub(mem.LastAccessedAt).Hours()
	if t < 0 {
		t = 0
	}

	lambda := mem.Type.DefaultDecayRate()
	base := mem.RelevanceScore
	if base <= 0 {
		base = 1.0
	}

	accessBoost := 1.0 + math.Log1p(float64(mem.AccessCount))*accessBoostFactor
	score := base * math.Exp(-lambda*t) * accessBoost

	if score < decayFloor {
		score = decayFloor
	}
	return score
}
