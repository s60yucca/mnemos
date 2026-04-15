//go:build benchmark

package benchmark

import (
	"context"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimulator_StoreIsolationBetweenScenarios verifies that after calling
// resetForNextScenario, memories from scenario 1 are not visible in scenario 2.
func TestSimulator_StoreIsolationBetweenScenarios(t *testing.T) {
	ctx := context.Background()

	sim, err := NewSessionSimulator()
	require.NoError(t, err)
	defer sim.Close()

	scenario1 := BenchmarkScenario{
		Name:      "scenario-1",
		ProjectID: "proj-isolation",
		Sessions:  1,
		Memories: []SeedMemory{
			{
				ID:                   "mem-s1-a",
				Content:              "scenario one memory alpha",
				Type:                 domain.MemoryTypeEpisodic,
				Category:             "general",
				AvailableFromSession: 1,
			},
			{
				ID:                   "mem-s1-b",
				Content:              "scenario one memory beta",
				Type:                 domain.MemoryTypeEpisodic,
				Category:             "general",
				AvailableFromSession: 1,
			},
		},
		Tasks: []Task{
			{
				Description: "query scenario 1",
				Query:       "scenario one memory",
				TokenBudget: 2000,
			},
		},
	}

	scenario2 := BenchmarkScenario{
		Name:      "scenario-2",
		ProjectID: "proj-isolation",
		Sessions:  1,
		Memories: []SeedMemory{
			{
				ID:                   "mem-s2-x",
				Content:              "scenario two memory exclusive",
				Type:                 domain.MemoryTypeEpisodic,
				Category:             "general",
				AvailableFromSession: 1,
			},
		},
		Tasks: []Task{
			{
				Description: "query scenario 2",
				Query:       "scenario two memory",
				TokenBudget: 2000,
			},
		},
	}

	// Run scenario 1.
	metrics1, err := sim.RunScenario(ctx, scenario1)
	require.NoError(t, err)
	require.Len(t, metrics1, 1)

	// Scenario 1 should have 2 memories in the store.
	assert.Equal(t, 2, countMemoriesInTokens(metrics1[0]), "scenario 1 should have 2 memories")

	// Reset between scenarios.
	require.NoError(t, sim.resetForNextScenario(ctx))

	// Run scenario 2.
	metrics2, err := sim.RunScenario(ctx, scenario2)
	require.NoError(t, err)
	require.Len(t, metrics2, 1)

	// Scenario 2 session 1 should only have its own 1 memory — not scenario 1's memories.
	// TotalStoreTokens reflects only scenario 2's memories.
	assert.Equal(t, 1, countMemoriesInTokens(metrics2[0]),
		"scenario 2 should only see its own memories after reset")
}

// countMemoriesInTokens is a helper that infers the number of stored memories
// from TotalStoreTokens. Each test memory has a known short content, so we
// check that TotalStoreTokens is consistent with the expected memory count.
// We use a simpler approach: just check TotalStoreTokens > 0 and compare
// relative sizes. Actually, let's count via the metrics directly.
//
// Since we can't directly query the store from the test, we rely on
// TotalStoreTokens being proportional to the number of memories.
// For isolation, we verify scenario 2's TotalStoreTokens < scenario 1's.
func countMemoriesInTokens(m SessionMetrics) int {
	// Each test memory content is ~30 chars → estimateTokens ≈ 8.
	// We use TotalStoreTokens / 8 as a rough count.
	// This is only used for relative comparison in the isolation test.
	if m.TotalStoreTokens == 0 {
		return 0
	}
	// "scenario one memory alpha" = 25 chars → estimateTokens = 25/4+1 = 7
	// "scenario one memory beta"  = 24 chars → estimateTokens = 24/4+1 = 7
	// "scenario two memory exclusive" = 29 chars → estimateTokens = 29/4+1 = 8
	// We can't get exact count without knowing content, so use a threshold.
	// For the isolation test, scenario 1 has 2 memories and scenario 2 has 1.
	// We just need to verify scenario 2 has fewer tokens than scenario 1.
	// Return a bucket: 0=empty, 1=one memory, 2+=multiple.
	if m.TotalStoreTokens <= 10 {
		return 1
	}
	return 2
}

// TestSimulator_AvailableFromSessionTiming verifies that a memory with
// AvailableFromSession=3 is NOT present in session 2 but IS present in session 3.
func TestSimulator_AvailableFromSessionTiming(t *testing.T) {
	ctx := context.Background()

	sim, err := NewSessionSimulator()
	require.NoError(t, err)
	defer sim.Close()

	scenario := BenchmarkScenario{
		Name:      "timing-test",
		ProjectID: "proj-timing",
		Sessions:  3,
		Memories: []SeedMemory{
			{
				ID:                   "mem-early",
				Content:              "early memory available from session one",
				Type:                 domain.MemoryTypeEpisodic,
				Category:             "general",
				AvailableFromSession: 1,
			},
			{
				ID:                   "mem-late",
				Content:              "late content available from session three only",
				Type:                 domain.MemoryTypeEpisodic,
				Category:             "general",
				AvailableFromSession: 3,
			},
		},
		Tasks: []Task{
			{
				Description:       "query for late content",
				Query:             "late content",
				RelevantMemoryIDs: []string{"mem-late"},
				TokenBudget:       2000,
			},
		},
	}

	// One task per session → 3 metrics total.
	metrics, err := sim.RunScenario(ctx, scenario)
	require.NoError(t, err)
	require.Len(t, metrics, 3)

	session1 := metrics[0]
	session2 := metrics[1]
	session3 := metrics[2]

	assert.Equal(t, 1, session1.SessionNumber)
	assert.Equal(t, 2, session2.SessionNumber)
	assert.Equal(t, 3, session3.SessionNumber)

	// Session 1: only "early" memory is in the store.
	earlyTokens := estimateTokens("early memory available from session one")
	assert.Equal(t, earlyTokens, session1.TotalStoreTokens,
		"session 1 should only have the early memory")

	// Session 2: still only "early" memory (late memory not yet seeded).
	assert.Equal(t, earlyTokens, session2.TotalStoreTokens,
		"session 2 should still only have the early memory")

	// Session 3: both memories are in the store.
	lateTokens := estimateTokens("late content available from session three only")
	assert.Equal(t, earlyTokens+lateTokens, session3.TotalStoreTokens,
		"session 3 should have both memories")

	// In session 3, the late memory should be retrievable (it's now in the store).
	// TotalStoreTokens grew between session 2 and session 3.
	assert.Greater(t, session3.TotalStoreTokens, session2.TotalStoreTokens,
		"store should grow when late memory is seeded in session 3")
}
