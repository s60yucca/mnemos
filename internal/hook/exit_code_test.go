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
	"pgregory.net/rapid"
)

// dispatchBytes sends raw bytes to the dispatcher and returns the decoded output.
func dispatchBytes(t *testing.T, d *hook.Dispatcher, input []byte) hook.HookOutput {
	t.Helper()
	var buf bytes.Buffer
	d.Dispatch(context.Background(), bytes.NewReader(input), &buf)

	var out hook.HookOutput
	require.NoError(t, json.NewDecoder(&buf).Decode(&out), "dispatcher output must be valid JSON (got: %q)", buf.String())
	return out
}

// TestHookExitCode_EmptyStdin verifies that sending empty bytes to Dispatch
// returns valid JSON with status "error" — no panic, no non-zero exit.
func TestHookExitCode_EmptyStdin(t *testing.T) {
	d := newTestDispatcher(t)
	out := dispatchBytes(t, d, []byte{})

	assert.Equal(t, "error", out.Status, "empty stdin should produce status=error")
	assert.NotEmpty(t, out.Message, "error output should include a message")
}

// TestHookExitCode_InvalidJSON verifies that sending invalid JSON to Dispatch
// returns valid JSON with status "error" — no panic, no non-zero exit.
func TestHookExitCode_InvalidJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"bare string", `not json at all`},
		{"truncated object", `{"hook":"session-start"`},
		{"null bytes", string([]byte{0x00, 0x01, 0x02})},
		{"only whitespace", "   \t\n  "},
		{"array instead of object", `["session-start"]`},
		{"nested invalid", `{"hook": {"nested": true}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDispatcher(t)
			out := dispatchBytes(t, d, []byte(tc.input))

			assert.Equal(t, "error", out.Status, "invalid JSON %q should produce status=error", tc.input)
		})
	}
}

// TestHookExitCode_StorageError verifies that even when the dispatcher is given
// a valid hook input, it still returns valid JSON (never panics).
// We test this by sending valid inputs that exercise the storage path and
// verifying the output is always valid JSON with a known status.
func TestHookExitCode_StorageError(t *testing.T) {
	// Use a dispatcher with a real (in-memory) mnemos — the key property is
	// that the dispatcher never panics and always returns valid JSON.
	d := newTestDispatcher(t)

	hooks := []string{"session-start", "prompt-submit", "session-end"}
	for _, hookName := range hooks {
		t.Run(hookName, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{
				"task_description": "test task",
				"prompt_text":      "implement authentication middleware",
			})
			input := map[string]any{
				"hook":        hookName,
				"session_id":  "test-storage-error-session",
				"project_dir": t.TempDir(),
				"payload":     json.RawMessage(payload),
			}
			inputJSON, err := json.Marshal(input)
			require.NoError(t, err)

			out := dispatchBytes(t, d, inputJSON)

			validStatuses := map[string]bool{"ok": true, "skipped": true, "error": true}
			assert.True(t, validStatuses[out.Status],
				"hook %q must return ok/skipped/error, got %q", hookName, out.Status)
		})
	}
}

// Feature: mnemos-autopilot, Property 19: Hook process must exit with code 0 in all cases.
// Tested here as: the dispatcher NEVER panics and ALWAYS returns valid JSON
// regardless of input bytes.
//
// Validates: Requirements 2.7
func TestProp_HookExitCodeAlwaysZero(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	d := hook.NewDispatcher(mn, cfg, logger)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate arbitrary byte sequences — including valid JSON, invalid JSON,
		// empty input, binary garbage, etc.
		inputBytes := rapid.SliceOf(rapid.Byte()).Draw(rt, "input_bytes")

		var buf bytes.Buffer

		// The key property: this must NEVER panic
		d.Dispatch(context.Background(), bytes.NewReader(inputBytes), &buf)

		// The output must ALWAYS be valid JSON
		var out hook.HookOutput
		if err := json.NewDecoder(&buf).Decode(&out); err != nil {
			rt.Fatalf("Dispatcher output is not valid JSON for input %q: %v (output: %q)",
				inputBytes, err, buf.String())
		}

		// Status must be one of the three valid values
		validStatuses := map[string]bool{"ok": true, "skipped": true, "error": true}
		if !validStatuses[out.Status] {
			rt.Fatalf("status %q is not one of ok/skipped/error (input: %q)", out.Status, inputBytes)
		}
	})
}

// TestHookExitCode_NoPanicOnConcurrentCalls verifies the dispatcher handles
// concurrent calls without panicking — each call returns valid JSON.
func TestHookExitCode_NoPanicOnConcurrentCalls(t *testing.T) {
	d := newTestDispatcher(t)

	inputs := []string{
		`{}`,
		`{"hook":"session-start","session_id":"s1"}`,
		`{"hook":"prompt-submit","session_id":"s2","payload":{"prompt_text":"fix auth"}}`,
		`{"hook":"session-end","session_id":"s3"}`,
		`not valid json`,
		``,
	}

	results := make(chan hook.HookOutput, len(inputs))
	errors := make(chan error, len(inputs))

	for _, inp := range inputs {
		go func(input string) {
			var buf bytes.Buffer
			d.Dispatch(context.Background(), strings.NewReader(input), &buf)

			var out hook.HookOutput
			if err := json.NewDecoder(&buf).Decode(&out); err != nil {
				errors <- err
				return
			}
			results <- out
		}(inp)
	}

	for range inputs {
		select {
		case out := <-results:
			validStatuses := map[string]bool{"ok": true, "skipped": true, "error": true}
			assert.True(t, validStatuses[out.Status], "concurrent call returned invalid status %q", out.Status)
		case err := <-errors:
			t.Errorf("concurrent call produced invalid JSON: %v", err)
		}
	}
}
