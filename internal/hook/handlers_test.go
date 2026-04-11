package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

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
