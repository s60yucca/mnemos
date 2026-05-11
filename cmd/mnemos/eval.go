package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/spf13/cobra"
)

func newEvalCmd(mnemos *core.Mnemos) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate knowledge base quality",
		Long: `Evaluate the quality of stored memories for a project.

Measures duplication rate, quality score distribution, type/category
coverage, and staleness. Use this to assess whether your knowledge base
is compiling usefully or accumulating noise.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Fetch summary stats for all statuses
			stats, err := mnemos.Stats(ctx, project)
			if err != nil {
				return fmt.Errorf("stats: %w", err)
			}

			// Fetch active memories for detailed analysis
			memories, err := mnemos.List(ctx, storage.ListQuery{
				ProjectID: project,
				Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
				Limit:     10000,
				SortBy:    "created_at",
				SortDesc:  true,
			})
			if err != nil {
				return fmt.Errorf("list memories: %w", err)
			}

			printEvalHeader(project, len(memories), stats)
			if len(memories) == 0 {
				if archived := stats.ByStatus["archived"]; archived > 0 {
					fmt.Printf("(%d archived memories exist — run 'mnemos maintain' to restore or review them.)\n", archived)
				}
				return nil
			}

			printEvalHeader(project, len(memories), stats)
			printQualityDistribution(memories)
			printTypeDistribution(memories)
			printCategoryCoverage(memories)
			printDuplicationRate(memories)
			printStaleness(memories)
			printOverallScore(memories)

			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project ID to evaluate (empty = all projects)")
	return cmd
}

func printEvalHeader(project string, total int, stats *storage.Stats) {
	scope := project
	if scope == "" {
		scope = "(all projects)"
	}
	fmt.Printf("Knowledge Base Evaluation — %s\n", scope)
	fmt.Printf("  Total: %d active", total)
	if archived := stats.ByStatus["archived"]; archived > 0 {
		fmt.Printf(" | %d archived", archived)
	}
	if deleted := stats.ByStatus["deleted"]; deleted > 0 {
		fmt.Printf(" | %d deleted", deleted)
	}
	fmt.Println()
	fmt.Println()
}

func printQualityDistribution(memories []*domain.Memory) {
	if len(memories) == 0 {
		return
	}
	var sum, minQ, maxQ float64
	minQ = 1.0
	bands := map[string]int{"A (90–100%)": 0, "B (70–89%)": 0, "C (50–69%)": 0, "D (<50%)": 0}

	for _, m := range memories {
		q := m.QualityScore
		sum += q
		if q < minQ {
			minQ = q
		}
		if q > maxQ {
			maxQ = q
		}
		switch {
		case q >= 0.9:
			bands["A (90–100%)"]++
		case q >= 0.7:
			bands["B (70–89%)"]++
		case q >= 0.5:
			bands["C (50–69%)"]++
		default:
			bands["D (<50%)"]++
		}
	}

	avg := sum / float64(len(memories))
	fmt.Printf("Quality Score Distribution\n")
	fmt.Printf("  Range: %.2f – %.2f  |  Avg: %.2f\n", minQ, maxQ, avg)
	for _, label := range []string{"A (90–100%)", "B (70–89%)", "C (50–69%)", "D (<50%)"} {
		fmt.Printf("  %-16s %d\n", label, bands[label])
	}
	fmt.Println()
}

func printTypeDistribution(memories []*domain.Memory) {
	byType := map[domain.MemoryType]int{}
	for _, m := range memories {
		byType[m.Type]++
	}
	order := []domain.MemoryType{
		domain.MemoryTypeLongTerm, domain.MemoryTypeSemantic,
		domain.MemoryTypeEpisodic, domain.MemoryTypeSkill,
		domain.MemoryTypeCompiled, domain.MemoryTypeShortTerm,
	}
	fmt.Printf("Memory Type Distribution\n")
	for _, t := range order {
		if n, ok := byType[t]; ok {
			fmt.Printf("  %-12s %d\n", t, n)
		}
	}
	fmt.Println()
}

func printCategoryCoverage(memories []*domain.Memory) {
	byCat := map[string]int{}
	for _, m := range memories {
		if m.Category != "" {
			byCat[m.Category]++
		}
	}
	fmt.Printf("Category Coverage\n")
	var cats []string
	for c := range byCat {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		bar := strings.Repeat("█", byCat[c])
		if len(bar) > 40 {
			bar = bar[:40]
		}
		fmt.Printf("  %-16s %2d %s\n", c, byCat[c], bar)
	}
	fmt.Println()
}

func printDuplicationRate(memories []*domain.Memory) {
	const threshold = 0.75
	const sampleLimit = 500

	// Sample to keep it fast
	sample := memories
	if len(sample) > sampleLimit {
		sample = sample[:sampleLimit]
	}

	dupPairs := 0
	totalChecked := 0
	seen := make(map[string]bool)

	for i, a := range sample {
		if seen[a.ID] {
			continue
		}
		tokA := util.TokenSet(util.Tokenize(a.Content))
		for j := i + 1; j < len(sample); j++ {
			b := sample[j]
			if seen[b.ID] {
				continue
			}
			totalChecked++
			tokB := util.TokenSet(util.Tokenize(b.Content))
			if util.JaccardSimilarity(tokA, tokB) >= threshold {
				dupPairs++
				seen[b.ID] = true
			}
		}
	}

	dupRate := 0.0
	if len(sample) > 0 {
		dupRate = float64(dupPairs) / float64(len(sample))
	}

	fmt.Printf("Duplication\n")
	fmt.Printf("  Near-duplicate pairs: %d  |  Rate: %.0f%%\n", dupPairs, dupRate*100)
	switch {
	case dupRate < 0.05:
		fmt.Printf("  ✓ Clean — very few near-duplicates.\n")
	case dupRate < 0.15:
		fmt.Printf("  ⚠ Acceptable — some overlap; semantic dedup would help.\n")
	default:
		fmt.Printf("  ✗ Noisy — %.0f%% of memories have a near-duplicate. Consider pruning.\n", dupRate*100)
	}
	fmt.Println()
}

func printStaleness(memories []*domain.Memory) {
	now := time.Now().UTC()
	thirtyDays := now.Add(-30 * 24 * time.Hour)
	ninetyDays := now.Add(-90 * 24 * time.Hour)

	var inactive30, inactive90, lowRelevance int
	for _, m := range memories {
		if m.RelevanceScore < 0.2 {
			lowRelevance++
		}
		if m.LastAccessedAt.Before(thirtyDays) {
			inactive30++
		}
		if m.LastAccessedAt.Before(ninetyDays) {
			inactive90++
		}
	}

	fmt.Printf("Staleness\n")
	fmt.Printf("  Not accessed in 30 days: %d\n", inactive30)
	fmt.Printf("  Not accessed in 90 days: %d\n", inactive90)
	fmt.Printf("  Low relevance (<0.2):    %d\n", lowRelevance)

	total := len(memories)
	staleScore := float64(inactive30+lowRelevance) / float64(maxEval(total, 1))
	switch {
	case staleScore < 0.1:
		fmt.Printf("  ✓ Fresh — knowledge base is actively used.\n")
	case staleScore < 0.3:
		fmt.Printf("  ⚠ Some staleness — consider running 'mnemos maintain'.\n")
	default:
		fmt.Printf("  ✗ Stale — %.0f%% of memories may be outdated. Run 'mnemos maintain' and review.\n", staleScore*100)
	}
	fmt.Println()
}

func printOverallScore(memories []*domain.Memory) {
	// Simple heuristic: avg quality weighted by relevance, penalized by duplication and staleness
	var qualitySum float64
	for _, m := range memories {
		qualitySum += m.QualityScore * math.Max(m.RelevanceScore, 0.1)
	}
	avgWeighted := qualitySum / float64(maxEval(len(memories), 1))

	// Duplication penalty
	constThreshold := 0.75
	constSampleLimit := 500
	sample := memories
	if len(sample) > constSampleLimit {
		sample = sample[:constSampleLimit]
	}
	dupCount := 0
	dupSeen := make(map[string]bool)
	for i, a := range sample {
		if dupSeen[a.ID] {
			continue
		}
		tokA := util.TokenSet(util.Tokenize(a.Content))
		for j := i + 1; j < len(sample); j++ {
			b := sample[j]
			if dupSeen[b.ID] {
				continue
			}
			tokB := util.TokenSet(util.Tokenize(b.Content))
			if util.JaccardSimilarity(tokA, tokB) >= constThreshold {
				dupCount++
				dupSeen[b.ID] = true
			}
		}
	}
	dupPenalty := 0.0
	if len(sample) > 0 {
		dupPenalty = float64(dupCount) / float64(len(sample))
	}

	// Staleness penalty
	now := time.Now().UTC()
	thirtyDays := now.Add(-30 * 24 * time.Hour)
	var staleCount int
	for _, m := range memories {
		if m.LastAccessedAt.Before(thirtyDays) || m.RelevanceScore < 0.2 {
			staleCount++
		}
	}
	stalePenalty := 0.0
	if len(memories) > 0 {
		stalePenalty = float64(staleCount) / float64(len(memories))
	}

	score := avgWeighted*(1.0-dupPenalty*0.3)*(1.0-stalePenalty*0.4)
	score = math.Max(0, math.Min(1, score))

	grade := "A"
	switch {
	case score >= 0.9:
		grade = "A"
	case score >= 0.75:
		grade = "B"
	case score >= 0.6:
		grade = "C"
	case score >= 0.4:
		grade = "D"
	default:
		grade = "F"
	}

	fmt.Printf("Overall Score: %.2f (%s)\n", score, grade)
	fmt.Printf("  (weighted quality × (1 − %.0f%% dup) × (1 − %.0f%% stale))\n", dupPenalty*30, stalePenalty*40)
}

func maxEval(a, b int) int {
	if a > b {
		return a
	}
	return b
}
