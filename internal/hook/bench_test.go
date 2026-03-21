package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/hook"
)

// BenchmarkSessionStart measures the latency of the session-start hook.
// Requirement: < 200ms per call.
func BenchmarkSessionStart(b *testing.B) {
	projectDir := b.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	if err := os.Mkdir(mnemosDir, 0o755); err != nil {
		b.Fatal(err)
	}

	mn := newTestMnemos(&testing.T{})
	cfg := defaultHookConfig()
	d := hook.NewDispatcher(mn, cfg, silentLogger())

	payload, _ := json.Marshal(map[string]string{
		"task_description": "implement authentication middleware",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := fmt.Sprintf("bench-start-%d", i)
		input := benchInput("session-start", sessionID, projectDir, payload)
		out := benchDispatch(d, input)
		if out.Status == "error" {
			b.Fatalf("unexpected error: %s", out.Message)
		}
	}
}

// BenchmarkPromptSubmit measures the latency of the prompt-submit hook.
// Requirement: < 100ms per call.
func BenchmarkPromptSubmit(b *testing.B) {
	projectDir := b.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	if err := os.Mkdir(mnemosDir, 0o755); err != nil {
		b.Fatal(err)
	}

	mn := newTestMnemos(&testing.T{})
	cfg := defaultHookConfig()
	d := hook.NewDispatcher(mn, cfg, silentLogger())

	payload, _ := json.Marshal(map[string]string{
		"prompt_text": "how should I structure the JWT validation logic",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique session IDs to avoid cooldown skipping
		sessionID := fmt.Sprintf("bench-prompt-%d", i)
		input := benchInput("prompt-submit", sessionID, projectDir, payload)
		out := benchDispatch(d, input)
		if out.Status == "error" {
			b.Fatalf("unexpected error: %s", out.Message)
		}
	}
}

// BenchmarkSessionEnd measures the latency of the session-end hook.
// Requirement: < 100ms per call.
func BenchmarkSessionEnd(b *testing.B) {
	projectDir := b.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	if err := os.Mkdir(mnemosDir, 0o755); err != nil {
		b.Fatal(err)
	}

	mn := newTestMnemos(&testing.T{})
	cfg := defaultHookConfig()
	d := hook.NewDispatcher(mn, cfg, silentLogger())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID := fmt.Sprintf("bench-end-%d", i)

		// Create session state first
		startPayload, _ := json.Marshal(map[string]string{
			"task_description": "benchmark session end",
		})
		benchDispatch(d, benchInput("session-start", sessionID, projectDir, startPayload))

		// Now end it
		out := benchDispatch(d, benchInput("session-end", sessionID, projectDir, nil))
		if out.Status == "error" {
			b.Fatalf("unexpected error: %s", out.Message)
		}
	}
}

// benchInput builds a hook input JSON string for benchmarks.
func benchInput(hookType, sessionID, projectDir string, payload json.RawMessage) string {
	m := map[string]any{
		"hook":        hookType,
		"session_id":  sessionID,
		"project_dir": projectDir,
	}
	if payload != nil {
		m["payload"] = payload
	}
	data, _ := json.Marshal(m)
	return string(data)
}

// benchDispatch dispatches a hook and returns the output.
func benchDispatch(d *hook.Dispatcher, inputJSON string) hook.HookOutput {
	var buf bytes.Buffer
	d.Dispatch(context.Background(), bytes.NewBufferString(inputJSON), &buf)
	var out hook.HookOutput
	_ = json.NewDecoder(&buf).Decode(&out)
	return out
}

// silentLogger returns a logger that discards all output.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
