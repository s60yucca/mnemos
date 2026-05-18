package autopilot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
)

// ReportWriter formats findings and stores them as autopilot report memories.
type ReportWriter struct {
	mnemos *core.Mnemos
}

// NewReportWriter constructs a ReportWriter.
func NewReportWriter(mnemos *core.Mnemos) *ReportWriter {
	return &ReportWriter{mnemos: mnemos}
}

// Write formats findings into a report memory and stores it, bypassing the quality gate.
func (w *ReportWriter) Write(ctx context.Context, projectID string, findings []Finding) error {
	content := Format(findings, time.Now().UTC())
	_, err := w.mnemos.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   content,
		Type:      domain.MemoryTypeSemantic,
		Category:  "autopilot",
		Tags:      []string{"autopilot-report", "auto-generated"},
		Source:    "autopilot-daemon",
		ProjectID: projectID,
	})
	return err
}

// Format renders findings as structured markdown (≤2000 characters).
func Format(findings []Finding, timestamp time.Time) string {
	var lines []string
	lines = append(lines, "## Autopilot Report ("+timestamp.Format(time.RFC3339)+")")
	lines = append(lines, "")

	stale := filterFindings(findings, FindingStaleCompiled)
	if len(stale) > 0 {
		lines = append(lines, fmt.Sprintf("**Stale compiled articles** (%d):", len(stale)))
		cap := 3
		if len(stale) < cap {
			cap = len(stale)
		}
		for _, f := range stale[:cap] {
			topic, _ := f.Metadata["article_topic"].(string)
			count, _ := f.Metadata["newer_source_count"].(int)
			lines = append(lines, fmt.Sprintf("- %q has %d newer source memories since compilation.", topic, count))
		}
	}

	relations := filterFindings(findings, FindingRelationsCreated)
	if len(relations) > 0 {
		total := 0
		for _, f := range relations {
			if c, ok := f.Metadata["count"].(int); ok {
				total += c
			}
		}
		entity, _ := relations[0].Metadata["sample_entity"].(string)
		lines = append(lines, fmt.Sprintf("**New relations discovered** (%d):", total))
		lines = append(lines, fmt.Sprintf("- %d relates_to relations created (e.g. via `%s`).", total, entity))
	}

	contradictions := filterFindings(findings, FindingPotentialContradiction)
	if len(contradictions) > 0 {
		lines = append(lines, fmt.Sprintf("**Potential contradictions** (%d):", len(contradictions)))
		cap := 2
		if len(contradictions) < cap {
			cap = len(contradictions)
		}
		for _, f := range contradictions[:cap] {
			memA, _ := f.Metadata["memory_a"].(string)
			memB, _ := f.Metadata["memory_b"].(string)
			if len(memA) > 8 {
				memA = memA[:8]
			}
			if len(memB) > 8 {
				memB = memB[:8]
			}
			entity := ""
			if se, ok := f.Metadata["shared_entities"].([]string); ok && len(se) > 0 {
				entity = se[0]
			}
			lines = append(lines, fmt.Sprintf("- Memories %s... and %s... share `%s` with opposing signals.", memA, memB, entity))
		}
	}

	autoCompiled := filterFindings(findings, FindingAutoCompiled)
	if len(autoCompiled) > 0 {
		lines = append(lines, fmt.Sprintf("**Auto-compiled articles** (%d):", len(autoCompiled)))
		for _, f := range autoCompiled {
			topic, _ := f.Metadata["topic"].(string)
			count, _ := f.Metadata["source_count"].(int)
			lines = append(lines, fmt.Sprintf("- %q compiled from %d source memories.", topic, count))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "**Suggestion**: "+buildSuggestion(findings))

	result := strings.Join(lines, "\n")
	if len(result) > 2000 {
		result = truncateAtSentence(result, 2000)
	}
	return result
}

// buildSuggestion returns a single actionable sentence based on finding priority.
func buildSuggestion(findings []Finding) string {
	stale := filterFindings(findings, FindingStaleCompiled)
	if len(stale) > 0 {
		topic, _ := stale[0].Metadata["article_topic"].(string)
		return fmt.Sprintf("Consider recompiling %q with recent findings.", topic)
	}
	relations := filterFindings(findings, FindingRelationsCreated)
	if len(relations) > 0 {
		return "New code relations discovered — use mnemos_context for updated graph."
	}
	contradictions := filterFindings(findings, FindingPotentialContradiction)
	if len(contradictions) > 0 {
		memA, _ := contradictions[0].Metadata["memory_a"].(string)
		memB, _ := contradictions[0].Metadata["memory_b"].(string)
		if len(memA) > 8 {
			memA = memA[:8]
		}
		if len(memB) > 8 {
			memB = memB[:8]
		}
		return fmt.Sprintf("Review memories %s... and %s... for conflicts.", memA, memB)
	}
	autoCompiled := filterFindings(findings, FindingAutoCompiled)
	if len(autoCompiled) > 0 {
		return "A new compiled knowledge article was created automatically."
	}
	return "No actionable findings."
}

// filterFindings returns findings of the given type.
func filterFindings(findings []Finding, t FindingType) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Type == t {
			out = append(out, f)
		}
	}
	return out
}

// truncateAtSentence truncates s to at most maxLen characters at a sentence boundary.
func truncateAtSentence(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find the last ". " or ".\n" before maxLen
	sub := s[:maxLen]
	lastDot := -1
	for i := len(sub) - 2; i >= 0; i-- {
		if sub[i] == '.' && (sub[i+1] == ' ' || sub[i+1] == '\n') {
			lastDot = i
			break
		}
	}
	if lastDot >= 0 {
		return s[:lastDot+1]
	}
	return s[:maxLen]
}
