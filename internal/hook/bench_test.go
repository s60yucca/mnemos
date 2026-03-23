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

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/hook"
)

func init() {
	// Suppress the default slog logger during benchmarks so INFO lines from
	// session_end.go don't pollute bench output.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
}

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

// BenchmarkAssembleContext measures context assembly with a populated in-memory store.
// Exercises the MMR diversity filter + adaptive packing path.
// Requirement: < 50ms per call (token budget 2000, 20 candidate memories).
func BenchmarkAssembleContext(b *testing.B) {
	mn := newTestMnemos(&testing.T{})
	ctx := context.Background()

	// Seed 20 memories across 5 categories to exercise diversity filtering.
	seeds := []struct{ content, category string }{
		{"SessionStore.Close() race condition: add sync.RWMutex on shared map access in auth/session.go", "bugs"},
		{"JWT tokens expire after 24h; refresh handled by middleware/auth.go RefreshToken()", "architecture"},
		{"Rate limiter uses token bucket algorithm: 100 req/min default, config in config.yaml", "architecture"},
		{"Deploy: run migrations before code deploy; rollback script at scripts/rollback.sh", "deployment"},
		{"Convention: wrap errors with fmt.Errorf, never panic in request handlers", "conventions"},
		{"DatabasePool.Connect() uses WAL mode; max 1 writer, N readers via read-only connections", "architecture"},
		{"CacheManager.Evict() applies LRU policy; TTL default 5m, configurable per key prefix", "architecture"},
		{"handleAuth() validates JWT_SECRET from env; missing key returns 401 not 500", "bugs"},
		{"Embedding provider health-check skipped in InitLight mode to keep hook startup < 200ms", "architecture"},
		{"QualityGate rejects memories < 5 words; downgrades generic long_term to short_term", "conventions"},
		{"Markdown mirror syncs async; failures logged but never block Store() path", "architecture"},
		{"FTS5 index rebuilt on mnemos maintain; safe to run while server is live", "deployment"},
		{"ULID used for all IDs: sortable, URL-safe, no collision risk at expected scale", "conventions"},
		{"Decay formula: relevance = max(floor, base × e^(-λt) × accessBoost × typeMultiplier)", "architecture"},
		{"SessionState stored as JSON files under .mnemos/sessions/; one file per session", "architecture"},
		{"TopicChanged uses Jaccard similarity on word sets; threshold 0.3 triggers new search", "conventions"},
		{"mnemos setup claude creates CLAUDE.md + .claude/hooks.json + .mcp.json idempotently", "deployment"},
		{"Background workers: DecayWorker 1h, GCWorker 6h, MarkdownSync 5m — ticker-based", "architecture"},
		{"Dedup: hash → Jaccard fuzzy (0.85) → semantic (0.92); merge on fuzzy/semantic hit", "conventions"},
		{"REST API optional; disabled by default; enable with server.rest_enabled: true", "architecture"},
	}

	for _, s := range seeds {
		_, _ = mn.Store(ctx, &domain.StoreRequest{
			Content:   s.content,
			Category:  s.category,
			ProjectID: "bench-project",
		})
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := mn.AssembleContext(ctx, "authentication middleware JWT session", "bench-project", 2000, false)
		if err != nil {
			b.Fatalf("AssembleContext error: %v", err)
		}
		if result == nil {
			b.Fatal("nil result")
		}
	}
}
