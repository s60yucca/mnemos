package memory

// IssueType identifies the kind of quality problem found in a memory.
type IssueType string

const (
	IssueTooShort      IssueType = "too_short"
	IssueTooLong       IssueType = "too_long"
	IssueLowDensity    IssueType = "low_density"
	IssueNearDuplicate IssueType = "near_duplicate"
	IssueTooGeneric    IssueType = "too_generic"
)

// FixAction describes the remediation to apply for a quality issue.
type FixAction string

const (
	FixReject    FixAction = "reject"
	FixSummarize FixAction = "summarize"
	FixCompact   FixAction = "compact"
	FixMerge     FixAction = "merge"
	FixDowngrade FixAction = "downgrade"
)

// VerdictAction is the overall decision returned by the quality gate.
type VerdictAction string

const (
	VerdictAccept    VerdictAction = "accept"
	VerdictFix       VerdictAction = "accept_with_fixes"
	VerdictDowngrade VerdictAction = "downgrade"
	VerdictMerge     VerdictAction = "merge"
	VerdictReject    VerdictAction = "reject"
)

// QualityIssue describes a single quality problem and how to fix it.
// Metadata carries issue-specific context, e.g.:
//
//	{"similar_memory_id": "01ABC...", "similarity": 0.82}
type QualityIssue struct {
	Type     IssueType
	Fix      FixAction
	Reason   string
	Metadata map[string]any
}

// QualityVerdict is the result returned by QualityGate.Evaluate.
type QualityVerdict struct {
	Score  float64 // 0.0–1.0
	Action VerdictAction
	Issues []QualityIssue
}

// HasIssue returns true if any issue in the verdict has the given type.
func (v QualityVerdict) HasIssue(t IssueType) bool {
	for _, issue := range v.Issues {
		if issue.Type == t {
			return true
		}
	}
	return false
}

// GetIssue returns a pointer to the first issue with the given type, or nil.
func (v QualityVerdict) GetIssue(t IssueType) *QualityIssue {
	for i := range v.Issues {
		if v.Issues[i].Type == t {
			return &v.Issues[i]
		}
	}
	return nil
}
