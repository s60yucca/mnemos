package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDispatcher(t *testing.T) *hook.Dispatcher {
	t.Helper()
	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return hook.NewDispatcher(mn, cfg, logger)
}

func dispatch(t *testing.T, d *hook.Dispatcher, inputJSON string) hook.HookOutput {
	t.Helper()
	var buf bytes.Buffer
	d.Dispatch(context.Background(), strings.NewReader(inputJSON), &buf)
	var out hook.HookOutput
	require.NoError(t, json.NewDecoder(&buf).Decode(&out))
	return out
}

// TestSessionStart_NoPayload: session-start with empty payload should return
// status "skipped" and message "no task context".
func TestSessionStart_NoPayload(t *testing.T) {
	d := newTestDispatcher(t)

	input := `{"hook":"session-start","session_id":"test-session-no-payload"}`
	out := dispatch(t, d, input)

	assert.Equal(t, "skipped", out.Status)
	assert.Equal(t, "no task context", out.Message)
}

// TestSessionEnd_NoState: session-end with a session_id that has no state file
// should return status "skipped" and message "no session state found".
func TestSessionEnd_NoState(t *testing.T) {
	d := newTestDispatcher(t)

	input := `{"hook":"session-end","session_id":"nonexistent-session-xyz"}`
	out := dispatch(t, d, input)

	assert.Equal(t, "skipped", out.Status)
	assert.Equal(t, "no session state found", out.Message)
}

// TestPromptSubmit_CreatesStateIfMissing: prompt-submit with a specific prompt
// and no existing session state should return "ok" or "skipped" (not "error").
func TestPromptSubmit_CreatesStateIfMissing(t *testing.T) {
	d := newTestDispatcher(t)

	payload, err := json.Marshal(map[string]string{
		"prompt_text": "implement authentication with JWT tokens",
	})
	require.NoError(t, err)

	inputMap := map[string]any{
		"hook":       "prompt-submit",
		"session_id": "test-session-prompt-create",
		"payload":    json.RawMessage(payload),
	}
	inputJSON, err := json.Marshal(inputMap)
	require.NoError(t, err)

	out := dispatch(t, d, string(inputJSON))

	assert.NotEqual(t, "error", out.Status, "expected ok or skipped, got error: %s", out.Message)
}

func TestDispatcher_ClaudeUserPromptSubmitShape(t *testing.T) {
	projectDir := t.TempDir()

	d := newTestDispatcher(t)

	inputJSON, err := json.Marshal(map[string]any{
		"hook_event_name": "UserPromptSubmit",
		"session_id":      "claude-session-1",
		"cwd":             projectDir,
		"prompt":          "implement authentication with JWT tokens",
	})
	require.NoError(t, err)

	out := dispatch(t, d, string(inputJSON))

	// Claude-shaped input via hook_event_name should be normalised to "prompt-submit"
	// and return ok or skipped — never error.
	assert.NotEqual(t, "error", out.Status, "expected ok or skipped, got error: %s", out.Message)
	// State is written to the global ~/.mnemos/sessions directory, not a project-local
	// path — so we just verify the hook did not error rather than checking a local path.
}

func TestSessionStart_QueryUseless(t *testing.T) {
	d := newTestDispatcher(t)
	inputJSON := `{"hook":"session-start","session_id":"sess-useless","payload":{"task_description":"fix"}}`
	out := dispatch(t, d, inputJSON)
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, 0.0, out.Metadata["tokens_used"])
	assert.Equal(t, "useless_fallback", out.Metadata["query_source"])
}

func TestSessionStart_QueryBroad(t *testing.T) {
	d := newTestDispatcher(t)
	inputJSON := `{"hook":"session-start","session_id":"sess-broad","payload":{"task_description":"fix the issue"}}`
	out := dispatch(t, d, inputJSON)
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, "broad_query", out.Metadata["query_source"], "should delegate to assembleRecentContext")
}

func TestSessionStart_QuerySpecific(t *testing.T) {
	d := newTestDispatcher(t)
	// Specific query (>= 3 words, and tech term like 'JWT')
	inputJSON := `{"hook":"session-start","session_id":"sess-specific","payload":{"task_description":"fix the JWT token validation"}}`
	out := dispatch(t, d, inputJSON)
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, "task_specific_query", out.Metadata["query_source"], "should delegate to AssembleContext")
}

// TestFormatContextResult_IncludesAutopilotSection: when a project has an autopilot
// report, the formatted context should include the "Autopilot Suggestions" section.
func TestFormatContextResult_IncludesAutopilotSection(t *testing.T) {
	ctx := context.Background()
	mn := newTestMnemos(t)

	projectID := "test-project-autopilot"

	// Store an autopilot report memory
	_, err := mn.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   "## Autopilot Report (2024-01-01T00:00:00Z)\n\n**New relations discovered** (3): 3 relates_to relations created (e.g. via `internal/storage/sqlite/store.go`).\n\n**Suggestion**: New code relations discovered — use mnemos_context for updated graph.",
		Type:      domain.MemoryTypeSemantic,
		Category:  "autopilot",
		Tags:      []string{"autopilot-report", "auto-generated"},
		Source:    "autopilot-daemon",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	// Store a regular memory so AssembleContext returns results
	_, err = mn.Store(ctx, &domain.StoreRequest{
		Content:   "JWT authentication middleware implementation using internal/auth/jwt.go",
		Type:      domain.MemoryTypeSemantic,
		Category:  "implementation",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	result, err := mn.AssembleContext(ctx, "JWT authentication middleware", projectID, 2000, false, nil)
	require.NoError(t, err)

	// Use the updated formatContextResult with autopilot injection
	output := hook.FormatContextResultForTest(ctx, mn, projectID, result)

	assert.Contains(t, output, "Autopilot Suggestions", "section 3 should be present when report exists")
	assert.Contains(t, output, "New relations discovered", "report content should appear in output")
}

// TestFormatContextResult_OmitsAutopilotSection: when a project has no autopilot
// report, the formatted context should not include the "Autopilot Suggestions" section.
func TestFormatContextResult_OmitsAutopilotSection(t *testing.T) {
	ctx := context.Background()
	mn := newTestMnemos(t)

	projectID := "test-project-no-autopilot"

	// Store a regular memory so AssembleContext returns results
	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "JWT authentication middleware implementation using internal/auth/jwt.go",
		Type:      domain.MemoryTypeSemantic,
		Category:  "implementation",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	result, err := mn.AssembleContext(ctx, "JWT authentication middleware", projectID, 2000, false, nil)
	require.NoError(t, err)

	output := hook.FormatContextResultForTest(ctx, mn, projectID, result)

	assert.NotContains(t, output, "Autopilot Suggestions", "section should be absent when no report exists")
}

// TestFormatContextResult_AutopilotTokenCap: when the autopilot report exceeds 800
// characters, it should be truncated in the output.
func TestFormatContextResult_AutopilotTokenCap(t *testing.T) {
	ctx := context.Background()
	mn := newTestMnemos(t)

	projectID := "test-project-autopilot-cap"

	// Build a report content that exceeds 800 characters
	longContent := "## Autopilot Report (2024-01-01T00:00:00Z)\n\n"
	// Add enough sentences to exceed 800 chars
	for i := 0; i < 20; i++ {
		longContent += "This is a finding about internal/storage/sqlite/store.go entity. "
	}
	assert.Greater(t, len(longContent), 800, "test setup: content must exceed 800 chars")

	_, err := mn.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   longContent,
		Type:      domain.MemoryTypeSemantic,
		Category:  "autopilot",
		Tags:      []string{"autopilot-report", "auto-generated"},
		Source:    "autopilot-daemon",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	// Store a regular memory so AssembleContext returns results
	_, err = mn.Store(ctx, &domain.StoreRequest{
		Content:   "JWT authentication middleware implementation using internal/auth/jwt.go",
		Type:      domain.MemoryTypeSemantic,
		Category:  "implementation",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	result, err := mn.AssembleContext(ctx, "JWT authentication middleware", projectID, 2000, false, nil)
	require.NoError(t, err)

	output := hook.FormatContextResultForTest(ctx, mn, projectID, result)

	assert.Contains(t, output, "Autopilot Suggestions", "section should be present")

	// Extract just the autopilot section content to verify truncation
	autopilotIdx := strings.Index(output, "### Autopilot Suggestions\n\n")
	require.Greater(t, autopilotIdx, -1, "autopilot section header must be present")
	autopilotContent := output[autopilotIdx+len("### Autopilot Suggestions\n\n"):]
	// The injected content should be at most 800 chars
	assert.LessOrEqual(t, len(autopilotContent), 800+50, "autopilot content should be truncated to ~800 chars (with some slack for trailing newlines)")
}
