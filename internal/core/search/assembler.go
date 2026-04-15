package search

import (
	"strings"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// DetailLevel controls how much content is rendered for a memory.
type DetailLevel int

const (
	DetailFull    DetailLevel = iota // use mem.Content
	DetailSummary                    // use mem.Summary, fallback: Content[:100]
	DetailOneLine                    // first sentence of Content, up to 80 chars
)

// ContextAssembler encapsulates the MMR diversity filter and adaptive packing stages.
type ContextAssembler struct {
	lambda float64 // MMR trade-off: 0.0 = max diversity, 1.0 = max relevance
}

// NewContextAssembler constructs a ContextAssembler with lambda clamped to [0.0, 1.0].
func NewContextAssembler(lambda float64) *ContextAssembler {
	if lambda < 0.0 {
		lambda = 0.0
	}
	if lambda > 1.0 {
		lambda = 1.0
	}
	return &ContextAssembler{lambda: lambda}
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B| on lowercased tag strings.
// Returns 0.0 when both inputs are empty or nil.
func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}
	setA := make(map[string]struct{}, len(a))
	for _, t := range a {
		setA[strings.ToLower(t)] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, t := range b {
		setB[strings.ToLower(t)] = struct{}{}
	}
	intersection := 0
	for k := range setA {
		if _, ok := setB[k]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

// DiversityFilter selects a diverse subset of candidates using Maximal Marginal Relevance.
// It always includes the first (highest-scoring) candidate regardless of budget.
// After the first, it stops when cumulative estimateTokens would exceed budget.
func (a *ContextAssembler) DiversityFilter(candidates []*domain.Memory, budget int) []*domain.Memory {
	if len(candidates) == 0 {
		return []*domain.Memory{}
	}

	selected := make([]*domain.Memory, 0, len(candidates))
	remaining := make([]*domain.Memory, len(candidates))
	copy(remaining, candidates)

	usedTokens := 0

	for len(remaining) > 0 {
		bestIdx := 0
		bestScore := -1e18

		for i, candidate := range remaining {
			relevance := candidate.RelevanceScore

			maxSim := 0.0
			for _, s := range selected {
				sim := jaccardSimilarity(candidate.Tags, s.Tags)
				if candidate.Category != "" && candidate.Category == s.Category {
					sim += 0.3
				}
				if sim > maxSim {
					maxSim = sim
				}
			}

			score := a.lambda*relevance - (1-a.lambda)*maxSim
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		best := remaining[bestIdx]
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)

		tokens := estimateTokens(best.Content)
		// First candidate is always included regardless of budget.
		if len(selected) > 0 && usedTokens+tokens > budget {
			break
		}

		selected = append(selected, best)
		usedTokens += tokens
	}

	return selected
}

// sumTokens estimates total tokens for a slice of memories at the given detail level.
func sumTokens(memories []*domain.Memory, level DetailLevel) int {
	total := 0
	for _, m := range memories {
		total += estimateTokens(renderMemory(m, level))
	}
	return total
}

// renderMemory renders a memory at the given detail level.
func renderMemory(mem *domain.Memory, level DetailLevel) string {
	switch level {
	case DetailFull:
		return mem.Content
	case DetailSummary:
		if mem.Summary != "" {
			return mem.Summary
		}
		if len(mem.Content) <= 100 {
			return mem.Content
		}
		return mem.Content[:100]
	default: // DetailOneLine
		return oneLineContent(mem.Content)
	}
}

// oneLineContent extracts the first line of content up to 80 chars,
// splitting on ". ", ".\n", or "\n", whichever comes first.
// Falls back to word-boundary truncation at 80 chars.
func oneLineContent(content string) string {
	limit := 80
	if len(content) <= limit {
		// Check for delimiters within the full content
		for _, delim := range []string{". ", ".\n", "\n"} {
			if idx := strings.Index(content, delim); idx != -1 {
				result := strings.TrimRight(content[:idx+len(delim)], " \n")
				return result
			}
		}
		return content
	}

	// Search for delimiters within the first 80 chars
	search := content[:limit]
	minPos := limit
	for _, delim := range []string{". ", ".\n", "\n"} {
		if idx := strings.Index(search, delim); idx != -1 {
			pos := idx + len(delim)
			if pos < minPos {
				minPos = pos
			}
		}
	}

	result := strings.TrimRight(content[:minPos], " \n")
	if minPos == limit {
		// No delimiter found — truncate at nearest word boundary
		if idx := strings.LastIndex(result, " "); idx > 0 {
			result = result[:idx]
		}
	}
	return result
}

// PackWithBudget renders memories into a context string using adaptive detail levels.
// Budget is a target, not a hard ceiling — DiversityFilter enforces the hard ceiling upstream.
func (a *ContextAssembler) PackWithBudget(memories []*domain.Memory, budget int) string {
	if len(memories) == 0 {
		return ""
	}

	totalFull := sumTokens(memories, DetailFull)
	var parts []string

	switch {
	case totalFull <= budget:
		// All fit — use full content
		for _, m := range memories {
			parts = append(parts, renderMemory(m, DetailFull))
		}
	case totalFull <= budget*2:
		// Moderate pressure — top min(3,n) full, rest summary
		threshold := min(3, len(memories))
		for i, m := range memories {
			if i < threshold {
				parts = append(parts, renderMemory(m, DetailFull))
			} else {
				parts = append(parts, renderMemory(m, DetailSummary))
			}
		}
	default:
		// High pressure — top min(5,n) summary, rest one-line
		threshold := min(5, len(memories))
		for i, m := range memories {
			if i < threshold {
				parts = append(parts, renderMemory(m, DetailSummary))
			} else {
				parts = append(parts, renderMemory(m, DetailOneLine))
			}
		}
	}

	return strings.Join(parts, "\n\n")
}
