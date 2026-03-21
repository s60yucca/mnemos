package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core/search"
)

func handleSessionStart(ctx context.Context, d *Dispatcher, input *HookInput) (*HookOutput, error) {
	// 1. PARSE PAYLOAD
	var payload SessionStartPayload
	if len(input.Payload) > 0 {
		_ = json.Unmarshal(input.Payload, &payload)
	}

	// 2. RESOLVE SESSION ID
	sessionID := resolveSessionID(input)

	// 3. DERIVE INITIAL QUERY
	query := ""
	if payload.TaskDescription != "" {
		query = payload.TaskDescription
	} else if payload.InitialPrompt != "" {
		query = payload.InitialPrompt
	} else if payload.WorkingDir != "" {
		query = filepath.Base(payload.WorkingDir)
	}
	if query == "" {
		return &HookOutput{
			Status:  "skipped",
			Message: "no task context",
		}, nil
	}

	// 4. CLEAN STALE SESSIONS
	stateManager := NewStateManager(input.ProjectDir, d.cfg)
	_ = stateManager.CleanStale()

	// 5. UPSERT SESSION STATE
	state := stateManager.Get(sessionID)
	if state == nil {
		state = &SessionState{
			SessionID:    sessionID,
			ProjectID:    resolveProjectID(input),
			StartedAt:    time.Now(),
			PID:          os.Getpid(),
			InitialQuery: query,
			ActiveTopic:  DetectTopic(query),
		}
	}
	state.LastActivity = time.Now()
	if err := stateManager.Save(state); err != nil {
		slog.Warn("session_start: failed to save state", "err", err)
	}

	// 6. ASSEMBLE CONTEXT
	result, err := d.mnemos.AssembleContext(ctx, query, state.ProjectID, d.cfg.SessionStartMaxTokens, false)
	if err != nil {
		slog.Warn("session_start: context assembly failed", "err", err)
		return &HookOutput{
			Status:  "skipped",
			Message: "mnemos unavailable",
		}, nil
	}

	// 7. RETURN CONTEXT
	return &HookOutput{
		ContextInjection: formatContextResult(result),
		Status:           "ok",
		Metadata: map[string]any{
			"memories_found": len(result.Memories),
			"tokens_used":    result.TotalTokens,
		},
	}, nil
}

// formatContextResult formats a *search.ContextResult as markdown for the AI context window.
func formatContextResult(result *search.ContextResult) string {
	if result == nil || len(result.Memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Memory Context\n\n")

	for _, mem := range result.Memories {
		text := mem.Content
		if mem.Summary != "" {
			text = mem.Summary
		}
		if mem.Category != "" {
			sb.WriteString(fmt.Sprintf("### [%s] %s\n\n", mem.Category, mem.Type))
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}
