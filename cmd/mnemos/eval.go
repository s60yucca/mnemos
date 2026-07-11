package main

import (
	"context"
	"fmt"
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
			memories = filterUserFacingMemories(memories)

			if len(memories) == 0 {
				printEvalHeader(project, len(memories), stats)
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
			printFreshness(memories)
			printRetrievalUsefulness(memories)
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
	dupPairs, dupRate := duplicationStats(memories)

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

func printFreshness(memories []*domain.Memory) {
	now := time.Now().UTC()
	thirtyDays := now.Add(-30 * 24 * time.Hour)
	ninetyDays := now.Add(-90 * 24 * time.Hour)

	var inactive30, inactive90 int
	for _, m := range memories {
		if m.LastAccessedAt.Before(thirtyDays) {
			inactive30++
		}
		if m.LastAccessedAt.Before(ninetyDays) {
			inactive90++
		}
	}

	fmt.Printf("Freshness\n")
	fmt.Printf("  Not accessed in 30 days: %d\n", inactive30)
	fmt.Printf("  Not accessed in 90 days: %d\n", inactive90)

	total := len(memories)
	staleRate := float64(inactive30) / float64(maxEval(total, 1))
	switch {
	case staleRate < 0.1:
		fmt.Printf("  ✓ Fresh — knowledge base is actively used.\n")
	case staleRate < 0.3:
		fmt.Printf("  ⚠ Some staleness — a few active memories have not been retrieved recently.\n")
	default:
		fmt.Printf("  ✗ Stale — %.0f%% of memories have not been accessed recently. Run 'mnemos maintain', then review active memories that remain unused.\n", staleRate*100)
	}
	fmt.Println()
}

func printRetrievalUsefulness(memories []*domain.Memory) {
	var lowRelevance, neverAccessed int
	var relevanceSum float64
	for _, m := range memories {
		relevanceSum += m.RelevanceScore
		if m.RelevanceScore < 0.2 {
			lowRelevance++
		}
		if m.AccessCount == 0 {
			neverAccessed++
		}
	}

	avgRelevance := relevanceSum / float64(maxEval(len(memories), 1))
	lowRate := float64(lowRelevance) / float64(maxEval(len(memories), 1))

	fmt.Printf("Retrieval Usefulness\n")
	fmt.Printf("  Avg relevance:       %.2f\n", avgRelevance)
	fmt.Printf("  Low relevance (<0.2): %d\n", lowRelevance)
	fmt.Printf("  Never accessed:       %d\n", neverAccessed)
	switch {
	case lowRate < 0.2:
		fmt.Printf("  ✓ Useful — active memories are ranking well.\n")
	case lowRate < 0.6:
		fmt.Printf("  ⚠ Mixed — some active memories are not ranking strongly.\n")
	default:
		fmt.Printf("  ⚠ Low retrieval signal — active memories may be good but are not ranking for recent queries.\n")
	}
	fmt.Println()
}

func printOverallScore(memories []*domain.Memory) {
	metrics := calculateEvalMetrics(memories, 0)
	score := metrics.score
	stalePenalty := float64(metrics.stale) / float64(maxEval(metrics.active, 1))

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
	fmt.Printf("  (quality %.0f%% × duplicate penalty %.0f%% × freshness penalty %.0f%%)\n", metrics.quality*100, metrics.duplicate*30, stalePenalty*40)
	fmt.Printf("  Retrieval usefulness is reported separately and does not lower this quality score.\n")
}

func duplicationStats(memories []*domain.Memory) (int, float64) {
	const threshold = 0.75
	const sampleLimit = 500

	sample := memories
	if len(sample) > sampleLimit {
		sample = sample[:sampleLimit]
	}

	dupPairs := 0
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
	return dupPairs, dupRate
}

func maxEval(a, b int) int {
	if a > b {
		return a
	}
	return b
}
