package memory

import (
	"fmt"
	"log/slog"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// QualityGate evaluates incoming memories against quality dimensions.
// A nil *QualityGate is valid and always returns VerdictAccept with score 1.0.
type QualityGate struct {
	cfg config.QualityGateConfig
}

// NewQualityGate returns a new QualityGate, or nil when cfg.Enabled == false.
func NewQualityGate(cfg config.QualityGateConfig) *QualityGate {
	if !cfg.Enabled {
		return nil
	}
	return &QualityGate{cfg: cfg}
}

// penalty returns the configured penalty for the given issue type key,
// falling back to the design-doc defaults if the key is absent.
func (g *QualityGate) penalty(key string) float64 {
	defaults := map[string]float64{
		"too_short":      1.0,
		"too_long":       0.1,
		"low_density":    0.3,
		"near_duplicate": 0.4,
		"too_generic":    0.2,
	}
	if g.cfg.Penalties != nil {
		if v, ok := g.cfg.Penalties[key]; ok {
			return v
		}
	}
	return defaults[key]
}

// checkLength returns issues related to word count.
func (g *QualityGate) checkLength(content string) []QualityIssue {
	var issues []QualityIssue
	words := util.CountWords(content)
	if words < g.cfg.MinWords {
		issues = append(issues, QualityIssue{
			Type:   IssueTooShort,
			Fix:    FixReject,
			Reason: fmt.Sprintf("only %d words (min %d)", words, g.cfg.MinWords),
		})
	}
	if words > g.cfg.MaxWords {
		issues = append(issues, QualityIssue{
			Type:   IssueTooLong,
			Fix:    FixSummarize,
			Reason: fmt.Sprintf("%d words exceeds max %d", words, g.cfg.MaxWords),
		})
	}
	return issues
}

// checkDensity returns issues related to information density.
func (g *QualityGate) checkDensity(content string) []QualityIssue {
	var issues []QualityIssue
	density := util.InformationDensity(content)
	if density < g.cfg.MinDensity {
		issues = append(issues, QualityIssue{
			Type:   IssueLowDensity,
			Fix:    FixCompact,
			Reason: fmt.Sprintf("density %.2f below threshold %.2f", density, g.cfg.MinDensity),
		})
	}
	return issues
}

// checkNearDuplicate returns issues when content is too similar to a recent memory.
func (g *QualityGate) checkNearDuplicate(content string, recent []*domain.Memory) []QualityIssue {
	var issues []QualityIssue
	tokA := util.TokenSet(util.Tokenize(content))
	for _, m := range recent {
		tokB := util.TokenSet(util.Tokenize(m.Content))
		sim := util.JaccardSimilarity(tokA, tokB)
		if sim >= g.cfg.DuplicateThreshold {
			issues = append(issues, QualityIssue{
				Type:   IssueNearDuplicate,
				Fix:    FixMerge,
				Reason: fmt.Sprintf("%.0f%% overlap with memory %s", sim*100, m.ID),
				Metadata: map[string]any{
					"similar_memory_id": m.ID,
					"similarity":        sim,
				},
			})
			break
		}
	}
	return issues
}

// checkSpecificity returns issues when a long_term memory lacks project-specific identifiers.
func (g *QualityGate) checkSpecificity(content string, memType domain.MemoryType) []QualityIssue {
	var issues []QualityIssue
	if g.cfg.RequireSpecific && memType == domain.MemoryTypeLongTerm {
		if !util.HasProjectSpecificIdentifiers(content) {
			issues = append(issues, QualityIssue{
				Type:   IssueTooGeneric,
				Fix:    FixDowngrade,
				Reason: "no project-specific identifiers; downgrading to short_term",
			})
		}
	}
	return issues
}

// computeScore calculates a quality score in [0.0, 1.0] from the given issues.
func (g *QualityGate) computeScore(issues []QualityIssue) float64 {
	total := 0.0
	for _, issue := range issues {
		total += g.penalty(string(issue.Type))
	}
	score := 1.0 - total
	if score < 0.0 {
		return 0.0
	}
	return score
}

// verdictFromScore maps a score and issues to a VerdictAction.
// Special overrides (FixReject, FixMerge, FixDowngrade) take priority over score bands.
func (g *QualityGate) verdictFromScore(score float64, issues []QualityIssue) VerdictAction {
	for _, issue := range issues {
		if issue.Fix == FixReject {
			return VerdictReject
		}
		if issue.Fix == FixMerge {
			return VerdictMerge
		}
		if issue.Fix == FixDowngrade {
			return VerdictDowngrade
		}
	}
	sb := g.cfg.ScoreBands
	switch {
	case score >= sb.Accept:
		return VerdictAccept
	case score >= sb.Fix:
		return VerdictFix
	case score >= sb.Downgrade:
		return VerdictDowngrade
	default:
		return VerdictReject
	}
}

// Evaluate runs all quality checks and returns a verdict.
// On a nil receiver it returns VerdictAccept with score 1.0 immediately.
// Any panic inside is recovered and also returns VerdictAccept (fail-open).
func (g *QualityGate) Evaluate(req *domain.StoreRequest, recent []*domain.Memory) (verdict QualityVerdict) {
	if g == nil {
		return QualityVerdict{Score: 1.0, Action: VerdictAccept}
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("quality_gate panic recovered", "panic", r)
			verdict = QualityVerdict{Score: 1.0, Action: VerdictAccept}
		}
	}()

	content := req.Content

	var issues []QualityIssue
	issues = append(issues, g.checkLength(content)...)
	issues = append(issues, g.checkDensity(content)...)
	issues = append(issues, g.checkNearDuplicate(content, recent)...)
	issues = append(issues, g.checkSpecificity(content, req.Type)...)

	score := g.computeScore(issues)
	action := g.verdictFromScore(score, issues)

	switch action {
	case VerdictReject, VerdictDowngrade:
		slog.Info("quality_gate",
			"action", action,
			"score", score,
			"issues_count", len(issues),
			"content_preview", truncate(content, 80),
		)
	default: // VerdictAccept, VerdictFix, VerdictMerge
		slog.Debug("quality_gate",
			"action", action,
			"score", score,
			"issues_count", len(issues),
			"content_preview", truncate(content, 80),
		)
	}

	return QualityVerdict{
		Score:  score,
		Action: action,
		Issues: issues,
	}
}

// truncate returns the first n bytes of s, or s if shorter.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// TestQualityGateConfig returns a QualityGateConfig suitable for use in tests.
// It enables the gate with tight thresholds so all verdict paths can be exercised.
func TestQualityGateConfig() config.QualityGateConfig {
	return config.QualityGateConfig{
		Enabled:            true,
		MinWords:           5,
		MaxWords:           200,
		MinDensity:         0.3,
		DuplicateThreshold: 0.8,
		RequireSpecific:    true,
		Penalties: map[string]float64{
			"too_short":      1.0,
			"too_long":       0.1,
			"low_density":    0.3,
			"near_duplicate": 0.4,
			"too_generic":    0.2,
		},
		ScoreBands: config.ScoreBandsConfig{
			Accept:    0.8,
			Fix:       0.5,
			Downgrade: 0.3,
		},
	}
}
