package hook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/hook"
)

func FuzzDispatcher(f *testing.F) {
	// Seed corpus
	f.Add([]byte(`{"hook":"session-start","session_id":"abc"}`))
	f.Add([]byte(`{"hook":"prompt-submit","session_id":"abc","payload":{"prompt_text":"hello"}}`))
	f.Add([]byte(`{"hook":"session-end","session_id":"abc"}`))
	f.Add([]byte(`{"hook":"unknown-hook"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"hook":"session-start"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		mn := newTestMnemos(t)
		cfg := defaultHookConfig()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 4}))

		d := hook.NewDispatcher(mn, cfg, logger)

		var buf bytes.Buffer
		d.Dispatch(context.Background(), bytes.NewReader(data), &buf)

		// Output must always be valid JSON
		var out hook.HookOutput
		if err := json.NewDecoder(&buf).Decode(&out); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}

		// Status must be one of the valid values
		switch out.Status {
		case "ok", "skipped", "error":
			// valid
		default:
			t.Fatalf("unexpected status %q, want ok|skipped|error", out.Status)
		}
	})
}
