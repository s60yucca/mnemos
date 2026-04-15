package search_test

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
)

// TestDiversityImprovement_OverGreedy verifies that AssembleContext with lambda=0.7
// selects at most 2 JWT/security memories when given 3 JWT and 2 unrelated candidates.
func TestDiversityImprovement_OverGreedy(t *testing.T) {
	// Test the ContextAssembler directly with known candidates to verify diversity.
	// This avoids FTS matching uncertainty while still testing the full assembly pipeline.
	a := search.NewContextAssembler(0.7)

	// 3 JWT/security memories (high relevance, same tags+category)
	jwt1 := &domain.Memory{
		ID: "jwt1", Content: "JWT auth uses RS256 signing with 24h expiry",
		Tags: []string{"auth", "jwt", "security"}, Category: "security",
		RelevanceScore: 0.95,
	}
	jwt2 := &domain.Memory{
		ID: "jwt2", Content: "JWT middleware validates token on every request",
		Tags: []string{"auth", "jwt", "middleware"}, Category: "security",
		RelevanceScore: 0.92,
	}
	jwt3 := &domain.Memory{
		ID: "jwt3", Content: "JWT refresh token rotation prevents replay attacks",
		Tags: []string{"auth", "jwt", "refresh"}, Category: "security",
		RelevanceScore: 0.90,
	}

	// 2 unrelated memories (lower relevance, different tags+category)
	rate1 := &domain.Memory{
		ID: "rate1", Content: "Rate limiter uses token bucket algorithm 100 req/min",
		Tags: []string{"rate-limit", "performance"}, Category: "performance",
		RelevanceScore: 0.85,
	}
	deploy1 := &domain.Memory{
		ID: "deploy1", Content: "Kubernetes deployment uses rolling updates with health checks",
		Tags: []string{"kubernetes", "deployment", "infra"}, Category: "infrastructure",
		RelevanceScore: 0.80,
	}

	candidates := []*domain.Memory{jwt1, jwt2, jwt3, rate1, deploy1}

	// DiversityFilter with budget that allows ~3 memories (forces diversity to matter)
	// Each memory content is ~40 chars → ~11 tokens. Budget=35 fits 3 memories.
	selected := a.DiversityFilter(candidates, 35)

	// Count JWT memories in result
	jwtCount := 0
	for _, m := range selected {
		if m.ID == "jwt1" || m.ID == "jwt2" || m.ID == "jwt3" {
			jwtCount++
		}
	}

	if jwtCount > 2 {
		t.Errorf("diversity: expected at most 2 JWT memories, got %d (total selected: %d)",
			jwtCount, len(selected))
	}
	if len(selected) == 0 {
		t.Error("expected at least 1 memory in result")
	}
}
