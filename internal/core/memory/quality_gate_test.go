package memory

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGate() *QualityGate {
	cfg := config.DefaultConfig().QualityGate
	return NewQualityGate(cfg)
}

// --- checkLength ---

func TestCheckLength_TooShort(t *testing.T) {
	g := testGate()
	issues := g.checkLength("fix bug")
	require.Len(t, issues, 1)
	assert.Equal(t, IssueTooShort, issues[0].Type)
	assert.Equal(t, FixReject, issues[0].Fix)
}

func TestCheckLength_TooLong(t *testing.T) {
	g := testGate()
	content := strings.Repeat("word ", 250)
	issues := g.checkLength(content)
	require.Len(t, issues, 1)
	assert.Equal(t, IssueTooLong, issues[0].Type)
	assert.Equal(t, FixSummarize, issues[0].Fix)
}

func TestCheckLength_OK(t *testing.T) {
	g := testGate()
	issues := g.checkLength("SessionStore.Close() needs mutex for thread safety")
	assert.Empty(t, issues)
}

// --- checkDensity ---

func TestCheckDensity_Low(t *testing.T) {
	g := testGate()
	issues := g.checkDensity("We looked at things and discussed the system and talked about it")
	require.Len(t, issues, 1)
	assert.Equal(t, IssueLowDensity, issues[0].Type)
}

func TestCheckDensity_OK(t *testing.T) {
	g := testGate()
	issues := g.checkDensity("SessionStore.Close() race condition: add sync.RWMutex")
	assert.Empty(t, issues)
}

// --- checkNearDuplicate ---

func TestCheckNearDuplicate_DuplicateFound(t *testing.T) {
	// Use a lower threshold to ensure the similar content triggers the check
	cfg := config.DefaultConfig().QualityGate
	cfg.DuplicateThreshold = 0.5
	g := NewQualityGate(cfg)

	content := "Auth uses JWT tokens for authentication"
	recent := []*domain.Memory{
		{ID: "mem-001", Content: "Auth uses JWT tokens for authentication and authorization"},
	}
	issues := g.checkNearDuplicate(content, recent)
	require.Len(t, issues, 1)
	assert.Equal(t, IssueNearDuplicate, issues[0].Type)
	assert.Equal(t, FixMerge, issues[0].Fix)
	require.NotNil(t, issues[0].Metadata)
	assert.Equal(t, "mem-001", issues[0].Metadata["similar_memory_id"])
}

func TestCheckNearDuplicate_NoDuplicate(t *testing.T) {
	g := testGate()
	content := "Rate limiter uses token bucket"
	recent := []*domain.Memory{
		{ID: "mem-002", Content: "Auth uses JWT"},
	}
	issues := g.checkNearDuplicate(content, recent)
	assert.Empty(t, issues)
}

func TestCheckNearDuplicate_EmptyRecent(t *testing.T) {
	g := testGate()
	issues := g.checkNearDuplicate("any content here", []*domain.Memory{})
	assert.Empty(t, issues)
}

// --- checkSpecificity ---

func TestCheckSpecificity_GenericLongTerm(t *testing.T) {
	g := testGate()
	issues := g.checkSpecificity("The project has good error handling", domain.MemoryTypeLongTerm)
	require.Len(t, issues, 1)
	assert.Equal(t, IssueTooGeneric, issues[0].Type)
	assert.Equal(t, FixDowngrade, issues[0].Fix)
}

func TestCheckSpecificity_SpecificLongTerm(t *testing.T) {
	g := testGate()
	issues := g.checkSpecificity("SessionStore.Close() in auth/session.go needs mutex", domain.MemoryTypeLongTerm)
	assert.Empty(t, issues)
}

func TestCheckSpecificity_GenericShortTerm(t *testing.T) {
	g := testGate()
	// short_term skips the specificity check entirely
	issues := g.checkSpecificity("The project has good error handling", domain.MemoryTypeShortTerm)
	assert.Empty(t, issues)
}

// --- Evaluate ---

func TestEvaluate_MultipleIssuesCompound(t *testing.T) {
	g := testGate()
	// 250 words (too long) + low density + generic (long_term) → multiple penalties
	content := strings.Repeat("the and or but ", 63) // ~252 words, all stop words → low density + too long
	req := &domain.StoreRequest{
		Content: content,
		Type:    domain.MemoryTypeLongTerm,
	}
	verdict := g.Evaluate(req, nil)
	// Score should be less than 1.0 due to multiple penalties
	assert.Less(t, verdict.Score, 1.0)
	assert.True(t, verdict.HasIssue(IssueTooLong))
	assert.True(t, verdict.HasIssue(IssueLowDensity))
}

func TestEvaluate_TooShortOverridesEverything(t *testing.T) {
	g := testGate()
	req := &domain.StoreRequest{
		Content: "ok",
		Type:    domain.MemoryTypeLongTerm,
	}
	verdict := g.Evaluate(req, nil)
	assert.Equal(t, VerdictReject, verdict.Action)
	assert.True(t, verdict.HasIssue(IssueTooShort))
}

func TestEvaluate_NilGateReturnsAccept(t *testing.T) {
	var gate *QualityGate = nil
	req := &domain.StoreRequest{Content: "some content here"}
	verdict := gate.Evaluate(req, nil)
	assert.Equal(t, 1.0, verdict.Score)
	assert.Equal(t, VerdictAccept, verdict.Action)
}

func TestEvaluate_PanicRecovery(t *testing.T) {
	g := testGate()
	// Passing nil req causes a nil pointer dereference inside Evaluate, which should be recovered
	verdict := g.Evaluate(nil, nil)
	assert.Equal(t, VerdictAccept, verdict.Action)
	assert.Equal(t, 1.0, verdict.Score)
}

// --- applyQualityFixes ---

func TestApplyQualityFixes_Compact(t *testing.T) {
	mgr := &Manager{logger: slog.Default()}
	req := &domain.StoreRequest{
		Content: "Basically we looked at things and the system uses mutex for thread safety",
	}
	verdict := QualityVerdict{
		Action: VerdictFix,
		Issues: []QualityIssue{
			{Type: IssueLowDensity, Fix: FixCompact},
		},
	}
	result, err := mgr.applyQualityFixes(req, verdict)
	require.NoError(t, err)
	assert.Nil(t, result) // pipeline continues
	// Content should be compacted (filler "basically" removed)
	assert.NotEqual(t, "Basically we looked at things and the system uses mutex for thread safety", req.Content)
}

func TestApplyQualityFixes_Summarize(t *testing.T) {
	mgr := &Manager{logger: slog.Default()}
	original := strings.Repeat("word ", 250)
	req := &domain.StoreRequest{Content: original}
	verdict := QualityVerdict{
		Action: VerdictFix,
		Issues: []QualityIssue{
			{Type: IssueTooLong, Fix: FixSummarize},
		},
	}
	result, err := mgr.applyQualityFixes(req, verdict)
	require.NoError(t, err)
	assert.Nil(t, result) // pipeline continues
	// Summary should be set, content unchanged
	assert.NotEmpty(t, req.Summary)
	assert.Equal(t, original, req.Content)
}

func TestApplyQualityFixes_Downgrade(t *testing.T) {
	mgr := &Manager{logger: slog.Default()}
	req := &domain.StoreRequest{
		Content: "The project has good error handling",
		Type:    domain.MemoryTypeLongTerm,
	}
	verdict := QualityVerdict{
		Action: VerdictDowngrade,
		Issues: []QualityIssue{
			{Type: IssueTooGeneric, Fix: FixDowngrade},
		},
	}
	result, err := mgr.applyQualityFixes(req, verdict)
	require.NoError(t, err)
	assert.Nil(t, result) // pipeline continues
	assert.Equal(t, domain.MemoryTypeShortTerm, req.Type)
	assert.Contains(t, req.Tags, "auto-downgraded")
}

func TestApplyQualityFixes_Merge(t *testing.T) {
	mgr := &Manager{logger: slog.Default()}
	req := &domain.StoreRequest{Content: "Auth uses JWT tokens"}
	existingID := "existing-mem-123"
	verdict := QualityVerdict{
		Action: VerdictMerge,
		Issues: []QualityIssue{
			{
				Type: IssueNearDuplicate,
				Fix:  FixMerge,
				Metadata: map[string]any{
					"similar_memory_id": existingID,
				},
			},
		},
	}
	result, err := mgr.applyQualityFixes(req, verdict)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Memory)
	assert.Equal(t, existingID, result.Memory.ID)
	assert.False(t, result.Created)
}
