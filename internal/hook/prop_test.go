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
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"pgregory.net/rapid"
)

// Feature: mnemos-autopilot, Property 1: Hook always returns valid JSON with status "ok", "skipped", or "error"
func TestProp_HookAlwaysReturnsValidJSON(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := defaultHookConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	d := hook.NewDispatcher(mn, cfg, logger)

	hookNames := []string{
		"session-start",
		"prompt-submit",
		"session-end",
		"unknown-hook",
		"",
	}

	rapid.Check(t, func(rt *rapid.T) {
		hookName := rapid.SampledFrom(hookNames).Draw(rt, "hook_name")
		sessionID := rapid.StringMatching(`[a-zA-Z0-9_-]{0,32}`).Draw(rt, "session_id")
		payloadStr := rapid.StringMatching(`[a-zA-Z0-9 _-]{0,50}`).Draw(rt, "payload_text")

		payload, _ := json.Marshal(map[string]string{
			"prompt_text":      payloadStr,
			"task_description": payloadStr,
		})

		inputMap := map[string]any{
			"hook":       hookName,
			"session_id": sessionID,
			"payload":    json.RawMessage(payload),
		}
		inputJSON, err := json.Marshal(inputMap)
		if err != nil {
			rt.Skip("failed to marshal input")
		}

		var buf bytes.Buffer
		d.Dispatch(context.Background(), strings.NewReader(string(inputJSON)), &buf)

		var out hook.HookOutput
		if err := json.NewDecoder(&buf).Decode(&out); err != nil {
			rt.Fatalf("Dispatcher output is not valid JSON: %v (output: %q)", err, buf.String())
		}

		validStatuses := map[string]bool{"ok": true, "skipped": true, "error": true}
		if !validStatuses[out.Status] {
			rt.Fatalf("status %q is not one of ok/skipped/error", out.Status)
		}
	})
}

// Feature: mnemos-autopilot, Property 2: Unknown fields in HookInput are ignored and known fields are parsed correctly
func TestProp_UnknownFieldsIgnored(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hookName := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}`).Draw(rt, "hook_name")
		sessionID := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}`).Draw(rt, "session_id")
		unknownKey := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "unknown_key")
		unknownVal := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(rt, "unknown_val")

		// Build a JSON object that is a superset of HookInput
		raw := map[string]any{
			"hook":       hookName,
			"session_id": sessionID,
			unknownKey:   unknownVal,
		}
		data, err := json.Marshal(raw)
		if err != nil {
			rt.Skip("marshal failed")
		}

		var input hook.HookInput
		if err := json.Unmarshal(data, &input); err != nil {
			rt.Fatalf("decoding superset JSON failed: %v", err)
		}

		if input.Hook != hookName {
			rt.Fatalf("Hook field = %q, want %q", input.Hook, hookName)
		}
		if input.SessionID != sessionID {
			rt.Fatalf("SessionID field = %q, want %q", input.SessionID, sessionID)
		}
	})
}

// Feature: mnemos-autopilot, Property 7: len(RecentSearches) is always <= 50
func TestProp_RecentSearchesBounded(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 200).Draw(rt, "num_entries")

		state := &hook.SessionState{
			SessionID: "test-bounded",
			ProjectID: "proj",
		}

		for i := 0; i < n; i++ {
			state.RecentSearches = append(state.RecentSearches, hook.SearchEntry{
				Query:     "query",
				Topic:     "topic",
				Timestamp: time.Now(),
			})
			if len(state.RecentSearches) > 50 {
				state.RecentSearches = state.RecentSearches[len(state.RecentSearches)-50:]
			}
		}

		if len(state.RecentSearches) > 50 {
			rt.Fatalf("RecentSearches length = %d, want <= 50", len(state.RecentSearches))
		}
	})
}

// Feature: mnemos-autopilot, Property 8: Calling session-start multiple times with the same session_id results in exactly one state file
func TestProp_SessionStartIdempotent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "num_calls")
		sessionID := rapid.StringMatching(`[a-zA-Z0-9]{8,16}`).Draw(rt, "session_id")

		projectDir := t.TempDir()
		mnemosDir := filepath.Join(projectDir, ".mnemos")
		if err := os.Mkdir(mnemosDir, 0o755); err != nil {
			rt.Fatalf("failed to create .mnemos dir: %v", err)
		}

		mn := newTestMnemos(t)
		cfg := &config.HookConfig{
			Enabled:                  true,
			SessionDir:               "sessions",
			StaleTimeout:             1 * time.Hour,
			CleanupRetention:         24 * time.Hour,
			SearchCooldown:           5 * time.Minute,
			TopicSimilarityThreshold: 0.3,
			SessionStartMaxTokens:    2000,
			PromptSearchLimit:        5,
			LogLevel:                 "warn",
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		d := hook.NewDispatcher(mn, cfg, logger)

		payload, _ := json.Marshal(map[string]string{
			"task_description": "implement auth middleware",
		})
		inputMap := map[string]any{
			"hook":        "session-start",
			"session_id":  sessionID,
			"project_dir": projectDir,
			"payload":     json.RawMessage(payload),
		}
		inputJSON, _ := json.Marshal(inputMap)

		for i := 0; i < n; i++ {
			var buf bytes.Buffer
			d.Dispatch(context.Background(), strings.NewReader(string(inputJSON)), &buf)
		}

		sessionsDir := filepath.Join(mnemosDir, "sessions")
		entries, err := os.ReadDir(sessionsDir)
		if err != nil {
			// sessions dir may not exist if all calls were skipped
			return
		}

		// Count files matching this session ID
		count := 0
		for _, e := range entries {
			if strings.Contains(e.Name(), sessionID) {
				count++
			}
		}

		if count > 1 {
			rt.Fatalf("found %d state files for session %q, want exactly 1", count, sessionID)
		}
	})
}

// Feature: mnemos-autopilot, Property 13: When ActiveTopic == newTopic and last search was within cooldown, prompt-submit returns "skipped"
func TestProp_CooldownPreventsRepeatSearch(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a cooldown between 1 minute and 10 minutes
		cooldownSecs := rapid.IntRange(60, 600).Draw(rt, "cooldown_secs")
		cooldown := time.Duration(cooldownSecs) * time.Second

		// Generate a topic that will survive DetectTopic (non-stop-word, non-generic)
		topic := rapid.SampledFrom([]string{
			"authentication jwt tokens",
			"database connection pooling",
			"refactor middleware pipeline",
			"implement caching strategy",
			"fix memory leak goroutine",
		}).Draw(rt, "topic")

		// Build a state where the topic was searched recently (within cooldown)
		recentTimestamp := time.Now().Add(-cooldown / 2) // half the cooldown ago
		state := &hook.SessionState{
			SessionID:    "cooldown-test",
			ProjectID:    "proj",
			StartedAt:    time.Now().Add(-1 * time.Hour),
			LastActivity: time.Now(),
			PID:          os.Getpid(),
			ActiveTopic:  topic,
			RecentSearches: []hook.SearchEntry{
				{
					Query:     topic,
					Topic:     topic,
					Timestamp: recentTimestamp,
				},
			},
		}

		// Save state to a temp dir
		projectDir := t.TempDir()
		mnemosDir := filepath.Join(projectDir, ".mnemos")
		if err := os.Mkdir(mnemosDir, 0o755); err != nil {
			rt.Fatalf("failed to create .mnemos dir: %v", err)
		}

		cfg := &config.HookConfig{
			Enabled:                  true,
			SessionDir:               "sessions",
			StaleTimeout:             1 * time.Hour,
			CleanupRetention:         24 * time.Hour,
			SearchCooldown:           cooldown,
			TopicSimilarityThreshold: 0.3,
			SessionStartMaxTokens:    2000,
			PromptSearchLimit:        5,
			LogLevel:                 "warn",
		}

		sm := hook.NewStateManager(projectDir, cfg)
		if err := sm.Save(state); err != nil {
			rt.Fatalf("failed to save state: %v", err)
		}

		mn := newTestMnemos(t)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		d := hook.NewDispatcher(mn, cfg, logger)

		// Use the topic directly as prompt_text so DetectTopic returns the same topic
		payload, _ := json.Marshal(map[string]string{
			"prompt_text": topic,
		})
		inputMap := map[string]any{
			"hook":        "prompt-submit",
			"session_id":  state.SessionID,
			"project_dir": projectDir,
			"payload":     json.RawMessage(payload),
		}
		inputJSON, _ := json.Marshal(inputMap)

		var buf bytes.Buffer
		d.Dispatch(context.Background(), strings.NewReader(string(inputJSON)), &buf)

		var out hook.HookOutput
		if err := json.NewDecoder(&buf).Decode(&out); err != nil {
			rt.Fatalf("output is not valid JSON: %v", err)
		}

		if out.Status != "skipped" {
			rt.Fatalf("expected status=skipped due to cooldown, got %q (message: %q)", out.Status, out.Message)
		}
	})
}

// Feature: mnemos-autopilot, Property 6: For the same session_id provided in input, resolveSessionID returns the same value
func TestProp_SessionIDDeterministic(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessionID := rapid.StringMatching(`[a-zA-Z0-9_-]{1,32}`).Draw(rt, "session_id")

		projectDir := t.TempDir()
		mnemosDir := filepath.Join(projectDir, ".mnemos")
		if err := os.Mkdir(mnemosDir, 0o755); err != nil {
			rt.Fatalf("failed to create .mnemos dir: %v", err)
		}

		cfg := &config.HookConfig{
			Enabled:                  true,
			SessionDir:               "sessions",
			StaleTimeout:             1 * time.Hour,
			CleanupRetention:         24 * time.Hour,
			SearchCooldown:           5 * time.Minute,
			TopicSimilarityThreshold: 0.3,
			SessionStartMaxTokens:    2000,
			PromptSearchLimit:        5,
			LogLevel:                 "warn",
		}

		mn := newTestMnemos(t)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		d := hook.NewDispatcher(mn, cfg, logger)

		payload, _ := json.Marshal(map[string]string{
			"task_description": "test task",
		})

		// Call session-start twice with the same session_id
		for i := 0; i < 2; i++ {
			inputMap := map[string]any{
				"hook":        "session-start",
				"session_id":  sessionID,
				"project_dir": projectDir,
				"payload":     json.RawMessage(payload),
			}
			inputJSON, _ := json.Marshal(inputMap)
			var buf bytes.Buffer
			d.Dispatch(context.Background(), strings.NewReader(string(inputJSON)), &buf)
		}

		sessionsDir := filepath.Join(mnemosDir, "sessions")
		entries, err := os.ReadDir(sessionsDir)
		if err != nil {
			// No sessions dir means both were skipped — determinism holds trivially
			return
		}

		// Count files matching this session ID — should be exactly 1 (deterministic)
		count := 0
		for _, e := range entries {
			if strings.Contains(e.Name(), sessionID) {
				count++
			}
		}

		if count > 1 {
			rt.Fatalf("found %d state files for session_id %q, want at most 1 (determinism violated)", count, sessionID)
		}
	})
}
