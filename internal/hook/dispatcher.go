package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
)

// Dispatcher routes hook requests to the appropriate handler.
type Dispatcher struct {
	mnemos       *core.Mnemos
	cfg          *config.HookConfig
	stateManager *StateManager
	logger       *slog.Logger
}

// NewDispatcher creates a new Dispatcher. The StateManager is initialized with
// an empty projectDir — the actual project dir is resolved per-request from input.
func NewDispatcher(mnemos *core.Mnemos, cfg *config.HookConfig, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		mnemos:       mnemos,
		cfg:          cfg,
		stateManager: NewStateManager("", cfg),
		logger:       logger,
	}
}

// Dispatch reads HookInput from r, routes to the appropriate handler, and writes
// HookOutput as JSON to w. It always writes valid JSON and never returns an error —
// errors are encoded into HookOutput.
func (d *Dispatcher) Dispatch(ctx context.Context, r io.Reader, w io.Writer) {
	start := time.Now()

	var input HookInput
	dec := json.NewDecoder(r)
	if err := dec.Decode(&input); err != nil {
		d.writeOutput(w, &HookOutput{
			Status:  "error",
			Message: fmt.Sprintf("invalid JSON input: %s", err.Error()),
		})
		d.logCompletion(input, time.Since(start), "error")
		return
	}

	input.Hook = normalizeHookName(&input)

	if !d.cfg.Enabled {
		d.writeOutput(w, &HookOutput{
			Status:  "skipped",
			Message: "hooks disabled",
		})
		d.logCompletion(input, time.Since(start), "skipped")
		return
	}

	var (
		out *HookOutput
		err error
	)

	switch input.Hook {
	case "session-start":
		out, err = handleSessionStart(ctx, d, &input)
	case "prompt-submit":
		out, err = handlePromptSubmit(ctx, d, &input)
	case "session-end":
		out, err = handleSessionEnd(ctx, d, &input)
	default:
		out = &HookOutput{
			Status:  "error",
			Message: fmt.Sprintf("unknown hook: %s", input.Hook),
		}
	}

	if err != nil {
		out = &HookOutput{
			Status:  "error",
			Message: err.Error(),
		}
	}

	if out == nil {
		out = &HookOutput{Status: "error", Message: "handler returned nil output"}
	}

	d.writeOutput(w, out)
	d.logCompletion(input, time.Since(start), out.Status)
}

// writeOutput encodes out as JSON to w. If encoding fails, writes a fallback error JSON.
func (d *Dispatcher) writeOutput(w io.Writer, out *HookOutput) {
	if err := json.NewEncoder(w).Encode(out); err != nil {
		// Last-resort fallback — must not fail
		_, _ = fmt.Fprintf(w, `{"status":"error","message":"failed to encode output: %s"}`+"\n", err.Error())
	}
}

// logCompletion writes a structured log entry after each hook dispatch.
func (d *Dispatcher) logCompletion(input HookInput, duration time.Duration, status string) {
	d.logger.Info("hook completed",
		slog.String("hook_type", input.Hook),
		slog.String("session_id", input.SessionID),
		slog.String("project_id", resolveProjectID(&input)),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.String("status", status),
	)
}
