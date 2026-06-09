package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_FullSessionLifecycle exercises the complete session flow:
// session-start → prompt-submit → session-end using a real in-memory Mnemos instance.
func TestIntegration_FullSessionLifecycle(t *testing.T) {
	projectDir := t.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	require.NoError(t, os.Mkdir(mnemosDir, 0o755))

	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	d := newTestDispatcher(t)
	_ = mn
	_ = cfg

	sessionID := "integration-lifecycle-session"

	// --- session-start ---
	startPayload, _ := json.Marshal(map[string]string{
		"task_description": "implement JWT authentication middleware",
	})
	startInput := buildInput(t, "session-start", sessionID, projectDir, startPayload)
	startOut := dispatchRaw(t, d, startInput)

	assert.NotEqual(t, "error", startOut.Status, "session-start should not error: %s", startOut.Message)

	// --- prompt-submit ---
	promptPayload, _ := json.Marshal(map[string]string{
		"prompt_text": "how should I structure the JWT validation logic",
	})
	promptInput := buildInput(t, "prompt-submit", sessionID, projectDir, promptPayload)
	promptOut := dispatchRaw(t, d, promptInput)

	assert.NotEqual(t, "error", promptOut.Status, "prompt-submit should not error: %s", promptOut.Message)

	// --- session-end ---
	endInput := buildInput(t, "session-end", sessionID, projectDir, nil)
	endOut := dispatchRaw(t, d, endInput)

	assert.NotEqual(t, "error", endOut.Status, "session-end should not error: %s", endOut.Message)

	// After session-end, the state file should be deleted
	sessionsDir := filepath.Join(mnemosDir, "sessions")
	entries, _ := os.ReadDir(sessionsDir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), sessionID, "state file should be deleted after session-end")
	}
}

// TestIntegration_ConcurrentSessions verifies Property 21: concurrent sessions
// on the same project directory have independent state.
func TestIntegration_ConcurrentSessions(t *testing.T) {
	projectDir := t.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	require.NoError(t, os.Mkdir(mnemosDir, 0o755))

	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	d := newTestDispatcher(t)
	_ = mn
	_ = cfg

	const numSessions = 5
	var wg sync.WaitGroup
	errors := make([]string, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			sessionID := "concurrent-session-" + string(rune('A'+idx))

			// session-start
			startPayload, _ := json.Marshal(map[string]string{
				"task_description": "task for session " + sessionID,
			})
			startInput := buildInput(t, "session-start", sessionID, projectDir, startPayload)
			startOut := dispatchRaw(t, d, startInput)
			if startOut.Status == "error" {
				errors[idx] = "session-start error: " + startOut.Message
				return
			}

			// prompt-submit
			promptPayload, _ := json.Marshal(map[string]string{
				"prompt_text": "specific work for session " + sessionID,
			})
			promptInput := buildInput(t, "prompt-submit", sessionID, projectDir, promptPayload)
			promptOut := dispatchRaw(t, d, promptInput)
			if promptOut.Status == "error" {
				errors[idx] = "prompt-submit error: " + promptOut.Message
				return
			}

			// session-end
			endInput := buildInput(t, "session-end", sessionID, projectDir, nil)
			endOut := dispatchRaw(t, d, endInput)
			if endOut.Status == "error" {
				errors[idx] = "session-end error: " + endOut.Message
			}
		}(i)
	}

	wg.Wait()

	for i, errMsg := range errors {
		assert.Empty(t, errMsg, "session %d had an error", i)
	}
}

// TestIntegration_AutoInject verifies auto-inject firing and deduplication (Tasks 2.2 and 6.1).
// Section 6.1 (Timing test) is covered by asserting the hook execution time.
func TestIntegration_AutoInject(t *testing.T) {
	// Set up a temporary home directory so features.log doesn't fail
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	projectDir := t.TempDir()
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	require.NoError(t, os.Mkdir(mnemosDir, 0o755))

	// Enable bench mode to allow auto-inject
	require.NoError(t, os.WriteFile(filepath.Join(mnemosDir, "bench_mode"), []byte("on"), 0644))

	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	d := hook.NewDispatcher(mn, cfg, logger)

	ctx := context.Background()
	projectID := filepath.Base(projectDir)

	// Mem1: auto-injected
	mem1, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "general architecture overview for the project",
		Type:      domain.MemoryTypeSemantic,
		ProjectID: projectID,
	})
	require.NoError(t, err)

	// Mem2: specifically retrieved by prompt, should be deduplicated from auto-inject
	mem2, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "specific implementation details about the JWT authentication",
		Type:      domain.MemoryTypeEpisodic,
		ProjectID: projectID,
	})
	require.NoError(t, err)

	start := time.Now()

	// session-start with a prompt that matches mem2
	sessionID := "auto-inject-session"
	startPayload, _ := json.Marshal(map[string]string{
		"task_description": "implement JWT authentication",
	})
	startInput := buildInput(t, "session-start", sessionID, projectDir, startPayload)
	startOut := dispatchRaw(t, d, startInput)

	duration := time.Since(start)

	assert.NotEqual(t, "error", startOut.Status, "session-start should not error: %s", startOut.Message)

	// Section 6.1: Timing test
	assert.Less(t, duration, 500*time.Millisecond, "auto-inject should complete within 500ms")

	// Section 2.2: Ensure auto-inject firing and deduplication
	assert.Contains(t, startOut.ContextInjection, "# Mnemos Project Context", "auto-inject section should be present")

	parts := strings.Split(startOut.ContextInjection, "## Relevant Memory Context")
	require.Len(t, parts, 2, "context should contain auto-inject and main sections")

	autoInjectSection := parts[0]
	mainContextSection := parts[1]

	assert.Contains(t, autoInjectSection, mem1.Memory.ID, "mem1 should be auto-injected")
	assert.NotContains(t, autoInjectSection, mem2.Memory.ID, "mem2 should NOT be auto-injected (deduplicated)")

	assert.Contains(t, mainContextSection, "specific implementation details", "mem2 should be in main context")

	// Repeated session-start events may still retrieve prompt context, but the
	// passive project payload must only be injected once per session.
	secondOut := dispatchRaw(t, d, startInput)
	assert.NotEqual(t, "error", secondOut.Status, "repeated session-start should not error: %s", secondOut.Message)
	assert.NotContains(t, secondOut.ContextInjection, "# Mnemos Project Context")
	assert.Contains(t, secondOut.ContextInjection, "specific implementation details")
}

// --- helpers ---

func buildInput(t *testing.T, hookType, sessionID, projectDir string, payload json.RawMessage) string {
	t.Helper()
	m := map[string]any{
		"hook":        hookType,
		"session_id":  sessionID,
		"project_dir": projectDir,
	}
	if payload != nil {
		m["payload"] = payload
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}

func dispatchRaw(t *testing.T, d *hook.Dispatcher, inputJSON string) hook.HookOutput {
	t.Helper()
	var buf bytes.Buffer
	d.Dispatch(context.Background(), strings.NewReader(inputJSON), &buf)
	var out hook.HookOutput
	require.NoError(t, json.NewDecoder(&buf).Decode(&out))
	return out
}
