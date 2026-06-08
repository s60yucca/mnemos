package autopilot

import (
	"context"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// helpers to build test findings

func staleFinding(topic string, newerCount int) Finding {
	return Finding{
		Type: FindingStaleCompiled,
		Metadata: map[string]any{
			"article_id":         "art-001",
			"article_topic":      topic,
			"newer_source_count": newerCount,
			"related_new_count":  0,
		},
	}
}

func relationsFinding(count int, sampleEntity string) Finding {
	return Finding{
		Type: FindingRelationsCreated,
		Metadata: map[string]any{
			"count":         count,
			"sample_entity": sampleEntity,
		},
	}
}

func contradictionFinding(memA, memB string, sharedEntities []string) Finding {
	return Finding{
		Type: FindingPotentialContradiction,
		Metadata: map[string]any{
			"memory_a":         memA,
			"memory_b":         memB,
			"shared_entities":  sharedEntities,
			"overlap_score":    0.5,
			"opposing_signals": []string{"works vs broken"},
		},
	}
}

var fixedTime = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

// TestReportFormat_StaleOnly: 1 stale finding → correct markdown, no relations/contradictions section
func TestReportFormat_StaleOnly(t *testing.T) {
	findings := []Finding{staleFinding("Authentication Flow", 3)}
	result := Format(findings, fixedTime)

	if !strings.Contains(result, "## Autopilot Report") {
		t.Error("missing report header")
	}
	if !strings.Contains(result, "**Stale compiled articles**") {
		t.Error("missing stale section")
	}
	if !strings.Contains(result, "Authentication Flow") {
		t.Error("missing article topic")
	}
	if !strings.Contains(result, "3 newer source memories") {
		t.Error("missing newer source count")
	}
	if strings.Contains(result, "**New relations discovered**") {
		t.Error("should not have relations section")
	}
	if strings.Contains(result, "**Potential contradictions**") {
		t.Error("should not have contradictions section")
	}
	if !strings.Contains(result, "**Suggestion**") {
		t.Error("missing suggestion line")
	}
	if !strings.Contains(result, "Consider recompiling") {
		t.Error("suggestion should mention recompiling for stale finding")
	}
}

func TestReportWriter_ArchivesOlderReports(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "report-archive"
	writer := NewReportWriter(env.mnemos)

	if err := writer.Write(ctx, projectID, []Finding{relationsFinding(1, "internal/a.go")}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writer.Write(ctx, projectID, []Finding{relationsFinding(2, "internal/b.go")}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	activeReports, err := env.mnemos.List(ctx, storage.ListQuery{
		ProjectID: projectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list active reports: %v", err)
	}
	activeAutopilotReports := 0
	for _, report := range activeReports {
		if report.Category == "autopilot" && report.Source == "autopilot-daemon" {
			activeAutopilotReports++
		}
	}
	if activeAutopilotReports != 1 {
		t.Fatalf("expected one active autopilot report, got %d", activeAutopilotReports)
	}

	archivedReports, err := env.mnemos.List(ctx, storage.ListQuery{
		ProjectID: projectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusArchived},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list archived reports: %v", err)
	}
	if len(archivedReports) != 1 {
		t.Fatalf("expected one archived report, got %d", len(archivedReports))
	}
}

// TestReportFormat_AllSections: all three finding types → all sections present
func TestReportFormat_AllSections(t *testing.T) {
	findings := []Finding{
		staleFinding("API Design", 2),
		relationsFinding(5, "internal/storage/sqlite/store.go"),
		contradictionFinding("abcdef1234567890", "fedcba0987654321", []string{"MNEMOS_DB_PATH"}),
	}
	result := Format(findings, fixedTime)

	if !strings.Contains(result, "**Stale compiled articles**") {
		t.Error("missing stale section")
	}
	if !strings.Contains(result, "**New relations discovered**") {
		t.Error("missing relations section")
	}
	if !strings.Contains(result, "**Potential contradictions**") {
		t.Error("missing contradictions section")
	}
	if !strings.Contains(result, "**Suggestion**") {
		t.Error("missing suggestion")
	}
}

// TestReportFormat_TokenBudget: large findings set → output ≤ 2000 chars
func TestReportFormat_TokenBudget(t *testing.T) {
	var findings []Finding
	for i := 0; i < 20; i++ {
		findings = append(findings, staleFinding("A very long article topic name that takes up space in the report output buffer", 99))
	}
	for i := 0; i < 10; i++ {
		findings = append(findings, relationsFound(50, "internal/some/very/long/path/to/a/file/that/is/quite/verbose.go"))
	}
	for i := 0; i < 10; i++ {
		findings = append(findings, contradictionFinding("abcdef1234567890abcd", "fedcba0987654321fedc", []string{"MNEMOS_VERY_LONG_CONFIG_KEY_NAME"}))
	}

	result := Format(findings, fixedTime)
	if len(result) > 2000 {
		t.Errorf("output exceeds 2000 chars: got %d", len(result))
	}
}

// relationsFound is an alias to avoid shadowing the package-level function name.
func relationsFound(count int, entity string) Finding {
	return relationsFinding(count, entity)
}

// TestReportFormat_OmitsEmptySections: no contradictions → no contradiction section in output
func TestReportFormat_OmitsEmptySections(t *testing.T) {
	findings := []Finding{
		staleFinding("Storage Layer", 1),
		relationsFound(3, "internal/storage/sqlite/store.go"),
	}
	result := Format(findings, fixedTime)

	if strings.Contains(result, "**Potential contradictions**") {
		t.Error("contradiction section should be omitted when no contradiction findings")
	}
	if !strings.Contains(result, "**Stale compiled articles**") {
		t.Error("stale section should be present")
	}
	if !strings.Contains(result, "**New relations discovered**") {
		t.Error("relations section should be present")
	}
}

// TestReportFormat_SuggestionPriority: stale + relations → suggestion mentions stale article
func TestReportFormat_SuggestionPriority(t *testing.T) {
	findings := []Finding{
		staleFinding("Core Architecture", 4),
		relationsFound(7, "internal/core/mnemos.go"),
	}
	result := Format(findings, fixedTime)

	if !strings.Contains(result, "Consider recompiling") {
		t.Error("suggestion should prioritize stale finding over relations")
	}
	if strings.Contains(result, "mnemos_context") {
		t.Error("relations suggestion should not appear when stale finding is present")
	}
}

// Task 5.4 — Property-based test

func generateFindings(staleCount, relCount, contradCount int) []Finding {
	var findings []Finding
	for i := 0; i < staleCount; i++ {
		findings = append(findings, Finding{
			Type: FindingStaleCompiled,
			Metadata: map[string]any{
				"article_id":         "art-id-that-is-long-enough",
				"article_topic":      "Topic Number",
				"newer_source_count": i + 1,
				"related_new_count":  0,
			},
		})
	}
	for i := 0; i < relCount; i++ {
		findings = append(findings, Finding{
			Type: FindingRelationsCreated,
			Metadata: map[string]any{
				"count":         i + 1,
				"sample_entity": "internal/some/path/file.go",
			},
		})
	}
	for i := 0; i < contradCount; i++ {
		findings = append(findings, Finding{
			Type: FindingPotentialContradiction,
			Metadata: map[string]any{
				"memory_a":         "abcdef1234567890",
				"memory_b":         "fedcba0987654321",
				"shared_entities":  []string{"MNEMOS_DB_PATH", "internal/storage/sqlite/store.go"},
				"overlap_score":    0.5,
				"opposing_signals": []string{"works vs broken"},
			},
		})
	}
	return findings
}

// TestProp_ReportTokenBudget: arbitrary finding counts → len(Format(findings, time.Now())) <= 2000
//
// Validates: Requirements REPORT-2.2
func TestProp_ReportTokenBudget(t *testing.T) {
	f := func(staleCount, relCount, contradCount uint8) bool {
		findings := generateFindings(int(staleCount), int(relCount), int(contradCount))
		result := Format(findings, time.Now())
		return len(result) <= 2000
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
