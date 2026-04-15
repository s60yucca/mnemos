package search

import (
	"math"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// ---- helpers ----

func mem(id, content, summary, category string, tags []string, score float64) *domain.Memory {
	return &domain.Memory{
		ID:             id,
		Content:        content,
		Summary:        summary,
		Category:       category,
		Tags:           tags,
		RelevanceScore: score,
	}
}

// ---- jaccardSimilarity ----

func TestJaccardSimilarity_BothEmpty(t *testing.T) {
	if got := jaccardSimilarity(nil, nil); got != 0.0 {
		t.Errorf("both nil: want 0.0, got %f", got)
	}
	if got := jaccardSimilarity([]string{}, []string{}); got != 0.0 {
		t.Errorf("both empty: want 0.0, got %f", got)
	}
}

func TestJaccardSimilarity_IdenticalNonEmpty(t *testing.T) {
	tags := []string{"auth", "jwt", "security"}
	if got := jaccardSimilarity(tags, tags); got != 1.0 {
		t.Errorf("identical: want 1.0, got %f", got)
	}
}

func TestJaccardSimilarity_Disjoint(t *testing.T) {
	a := []string{"auth", "jwt"}
	b := []string{"database", "postgres"}
	if got := jaccardSimilarity(a, b); got != 0.0 {
		t.Errorf("disjoint: want 0.0, got %f", got)
	}
}

func TestJaccardSimilarity_PartialOverlap(t *testing.T) {
	a := []string{"auth", "jwt", "security"}
	b := []string{"auth", "jwt", "database"}
	// intersection=2, union=4 → 0.5
	got := jaccardSimilarity(a, b)
	if math.Abs(got-0.5) > 1e-9 {
		t.Errorf("partial overlap: want 0.5, got %f", got)
	}
}

func TestJaccardSimilarity_CaseInsensitive(t *testing.T) {
	a := []string{"Auth", "JWT"}
	b := []string{"auth", "jwt"}
	if got := jaccardSimilarity(a, b); got != 1.0 {
		t.Errorf("case-insensitive: want 1.0, got %f", got)
	}
}

// ---- DiversityFilter ----

func TestDiversityFilter_Empty(t *testing.T) {
	a := NewContextAssembler(0.7)
	result := a.DiversityFilter([]*domain.Memory{}, 1000)
	if len(result) != 0 {
		t.Errorf("empty candidates: want 0, got %d", len(result))
	}
}

func TestDiversityFilter_SingleCandidateAlwaysReturned(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Content is 400 chars → ~101 tokens, budget=1
	big := mem("1", strings.Repeat("x", 400), "", "code", nil, 0.9)
	result := a.DiversityFilter([]*domain.Memory{big}, 1)
	if len(result) != 1 {
		t.Errorf("single candidate: want 1, got %d", len(result))
	}
}

func TestDiversityFilter_Lambda1_SortedByRelevance(t *testing.T) {
	a := NewContextAssembler(1.0)
	candidates := []*domain.Memory{
		mem("1", "short", "", "code", []string{"a"}, 0.5),
		mem("2", "short", "", "code", []string{"b"}, 0.9),
		mem("3", "short", "", "code", []string{"c"}, 0.7),
	}
	result := a.DiversityFilter(candidates, 10000)
	if result[0].ID != "2" {
		t.Errorf("lambda=1.0: first should be highest relevance (id=2), got %s", result[0].ID)
	}
}

func TestDiversityFilter_CategoryBonusPenalisesSameCategory(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Two JWT/security memories and one unrelated
	jwt1 := mem("jwt1", "JWT auth 24h expiry", "", "security", []string{"auth", "jwt"}, 0.95)
	jwt2 := mem("jwt2", "JWT middleware validates token", "", "security", []string{"auth", "jwt", "middleware"}, 0.92)
	other := mem("other", "Rate limiter token bucket", "", "performance", []string{"rate-limit", "performance"}, 0.85)

	result := a.DiversityFilter([]*domain.Memory{jwt1, jwt2, other}, 10000)

	// jwt1 selected first (highest relevance). jwt2 should be penalised by category bonus.
	// With lambda=0.7 and category bonus, other should be preferred over jwt2.
	if len(result) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(result))
	}
	if result[0].ID != "jwt1" {
		t.Errorf("first should be jwt1, got %s", result[0].ID)
	}
	if result[1].ID != "other" {
		t.Errorf("second should be other (category bonus penalises jwt2), got %s", result[1].ID)
	}
}

func TestDiversityFilter_BudgetStopsSelection(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Each memory has ~26 tokens (100 chars / 4 + 1)
	m1 := mem("1", strings.Repeat("a", 100), "", "code", nil, 0.9)
	m2 := mem("2", strings.Repeat("b", 100), "", "code", nil, 0.8)
	m3 := mem("3", strings.Repeat("c", 100), "", "code", nil, 0.7)

	// Budget=30 → fits m1 (26 tokens), but m1+m2 = 52 > 30
	result := a.DiversityFilter([]*domain.Memory{m1, m2, m3}, 30)
	if len(result) != 1 {
		t.Errorf("budget stops at 1: want 1, got %d", len(result))
	}
}

// ---- PackWithBudget ----

func TestPackWithBudget_Empty(t *testing.T) {
	a := NewContextAssembler(0.7)
	if got := a.PackWithBudget([]*domain.Memory{}, 1000); got != "" {
		t.Errorf("empty: want empty string, got %q", got)
	}
}

func TestPackWithBudget_AllFit_DetailFull(t *testing.T) {
	a := NewContextAssembler(0.7)
	m1 := mem("1", "short content one", "", "code", nil, 1.0)
	m2 := mem("2", "short content two", "", "code", nil, 1.0)
	result := a.PackWithBudget([]*domain.Memory{m1, m2}, 10000)
	if !strings.Contains(result, "short content one") {
		t.Errorf("all fit: should contain full content of m1")
	}
	if !strings.Contains(result, "short content two") {
		t.Errorf("all fit: should contain full content of m2")
	}
}

func TestPackWithBudget_ModeratePressure_TopFullRestSummary(t *testing.T) {
	a := NewContextAssembler(0.7)
	// 5 memories, each ~26 tokens (100 chars). totalFull ≈ 130 tokens.
	// budget=100 → totalFull > budget but ≤ 2*budget → moderate pressure
	memories := []*domain.Memory{
		mem("1", strings.Repeat("a", 100), "summary1", "code", nil, 1.0),
		mem("2", strings.Repeat("b", 100), "summary2", "code", nil, 1.0),
		mem("3", strings.Repeat("c", 100), "summary3", "code", nil, 1.0),
		mem("4", strings.Repeat("d", 100), "summary4", "code", nil, 1.0),
		mem("5", strings.Repeat("e", 100), "summary5", "code", nil, 1.0),
	}
	result := a.PackWithBudget(memories, 100)
	// Top 3 should be full content (100 'a', 'b', 'c' chars)
	if !strings.Contains(result, strings.Repeat("a", 100)) {
		t.Errorf("moderate: m1 should be full content")
	}
	// Rest should be summary
	if !strings.Contains(result, "summary4") {
		t.Errorf("moderate: m4 should use summary")
	}
	if !strings.Contains(result, "summary5") {
		t.Errorf("moderate: m5 should use summary")
	}
}

func TestPackWithBudget_HighPressure_TopSummaryRestOneLine(t *testing.T) {
	a := NewContextAssembler(0.7)
	// 8 memories, each ~126 tokens (500 chars). totalFull ≈ 1008 tokens.
	// budget=200 → totalFull > 2*budget → high pressure
	memories := make([]*domain.Memory, 8)
	for i := range memories {
		memories[i] = mem(
			string(rune('1'+i)),
			strings.Repeat(string(rune('a'+i)), 500),
			"summary"+string(rune('1'+i)),
			"code", nil, 1.0,
		)
	}
	result := a.PackWithBudget(memories, 200)
	// Top 5 should use summary
	if !strings.Contains(result, "summary1") {
		t.Errorf("high pressure: m1 should use summary")
	}
	// Memories 6-8 should use one-line (first 80 chars of content)
	// Content is 500 repeated chars, so one-line = first 80 chars
	oneLine := strings.Repeat(string(rune('a'+5)), 80) // 'f' repeated 80 times
	if !strings.Contains(result, oneLine[:20]) {
		t.Errorf("high pressure: m6 should use one-line rendering")
	}
}

func TestPackWithBudget_DetailSummaryFallback(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Memory with empty summary — should fall back to first 100 chars of content
	content := strings.Repeat("x", 200)
	m := mem("1", content, "", "code", nil, 1.0)
	// Force moderate pressure: totalFull > budget but ≤ 2*budget
	// content = 200 chars → ~51 tokens. budget=40 → 51 > 40 but ≤ 80
	// With 2 memories, top min(3,2)=2 are full, so we need more memories
	// Use 4 memories to get rest into summary branch
	memories := []*domain.Memory{
		mem("0", strings.Repeat("y", 200), "", "code", nil, 1.0),
		mem("1", strings.Repeat("z", 200), "", "code", nil, 1.0),
		mem("2", strings.Repeat("w", 200), "", "code", nil, 1.0),
		m,
	}
	// totalFull ≈ 4*51 = 204 tokens. budget=150 → 204 > 150 but ≤ 300 → moderate
	result := a.PackWithBudget(memories, 150)
	// m is index 3 (rest), should use summary → fallback to first 100 chars
	if !strings.Contains(result, strings.Repeat("x", 100)) {
		t.Errorf("summary fallback: should contain first 100 chars of content")
	}
}

func TestPackWithBudget_ModeratePressure_TwoMemories(t *testing.T) {
	a := NewContextAssembler(0.7)
	// 2 memories, each ~51 tokens (200 chars). totalFull ≈ 102 tokens.
	// budget=80 → 102 > 80 but ≤ 160 → moderate pressure
	// min(3, 2) = 2 → both should be full
	m1 := mem("1", strings.Repeat("a", 200), "summary1", "code", nil, 1.0)
	m2 := mem("2", strings.Repeat("b", 200), "summary2", "code", nil, 1.0)
	result := a.PackWithBudget([]*domain.Memory{m1, m2}, 80)
	if !strings.Contains(result, strings.Repeat("a", 200)) {
		t.Errorf("2-memory moderate: m1 should be full content")
	}
	if !strings.Contains(result, strings.Repeat("b", 200)) {
		t.Errorf("2-memory moderate: m2 should be full content (min(3,2)=2)")
	}
}

func TestPackWithBudget_DetailOneLine_80CharCap(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Force high pressure with 6 memories
	memories := make([]*domain.Memory, 6)
	for i := range memories {
		// Content: 200 chars with no delimiters → truncated at 80 chars word boundary
		memories[i] = mem(
			string(rune('1'+i)),
			strings.Repeat(string(rune('a'+i)), 200),
			"summary"+string(rune('1'+i)),
			"code", nil, 1.0,
		)
	}
	// totalFull ≈ 6*51 = 306 tokens. budget=100 → 306 > 200 → high pressure
	result := a.PackWithBudget(memories, 100)
	// Memory at index 5 (6th) should be one-line: 80 chars of 'f'
	oneLine := strings.Repeat("f", 80)
	if !strings.Contains(result, oneLine) {
		t.Errorf("one-line 80-char cap: should contain 80 'f' chars, result=%q", result[:min(200, len(result))])
	}
}

func TestPackWithBudget_DetailOneLine_DelimiterDetection(t *testing.T) {
	a := NewContextAssembler(0.7)
	// Content with ". " delimiter within 80 chars
	content := "First sentence. Second sentence continues here with more text."
	// Force high pressure: 6 memories
	memories := make([]*domain.Memory, 6)
	for i := range memories {
		memories[i] = mem(
			string(rune('1'+i)),
			content,
			"summary"+string(rune('1'+i)),
			"code", nil, 1.0,
		)
	}
	// totalFull ≈ 6*16 = 96 tokens. budget=30 → 96 > 60 → high pressure
	result := a.PackWithBudget(memories, 30)
	// Memory at index 5 should be one-line: up to ". " delimiter
	if !strings.Contains(result, "First sentence") {
		t.Errorf("delimiter detection: should contain 'First sentence', got %q", result)
	}
}
