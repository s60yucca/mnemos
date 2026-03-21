package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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
// and no existing session state should create a state file and return "ok" or "skipped"
// (not "error").
func TestPromptSubmit_CreatesStateIfMissing(t *testing.T) {
	projectDir := t.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	require.NoError(t, os.Mkdir(mnemosDir, 0o755))

	d := newTestDispatcher(t)

	payload, err := json.Marshal(map[string]string{
		"prompt_text": "implement authentication with JWT tokens",
	})
	require.NoError(t, err)

	inputMap := map[string]any{
		"hook":        "prompt-submit",
		"session_id":  "test-session-prompt-create",
		"project_dir": projectDir,
		"payload":     json.RawMessage(payload),
	}
	inputJSON, err := json.Marshal(inputMap)
	require.NoError(t, err)

	out := dispatch(t, d, string(inputJSON))

	assert.NotEqual(t, "error", out.Status, "expected ok or skipped, got error: %s", out.Message)

	// Verify a state file was created under projectDir/.mnemos/sessions/
	sessionsDir := filepath.Join(mnemosDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	require.NoError(t, err, "sessions directory should exist")
	assert.NotEmpty(t, entries, "at least one session state file should have been created")
}
