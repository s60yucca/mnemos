package hook

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

func handleSessionEnd(ctx context.Context, d *Dispatcher, input *HookInput) (*HookOutput, error) {
	// 1. RESOLVE SESSION ID
	sessionID := resolveSessionID(input)

	// 2. LOAD SESSION STATE
	stateManager := NewStateManager(resolveProjectDir(input), d.cfg)
	state := stateManager.Get(sessionID)
	if state == nil {
		return &HookOutput{
			Status:  "skipped",
			Message: "no session state found",
		}, nil
	}

	// 3. COUNT MEMORIES STORED DURING SESSION
	count, err := d.mnemos.CountMemoriesSince(ctx, state.ProjectID, state.StartedAt)
	if err != nil {
		slog.Warn("session_end: count failed, assuming 0", "err", err)
		count = 0
	}

	// 4. BREADCRUMB (conditional — off by default)
	if count == 0 && d.cfg.SessionEndBreadcrumb {
		_, _ = d.mnemos.Store(ctx, &domain.StoreRequest{
			Content: fmt.Sprintf("Session on '%s' (%s). No explicit durable learnings captured.",
				state.ActiveTopic, time.Since(state.StartedAt).Round(time.Minute)),
			Type:      domain.MemoryTypeEpisodic,
			Category:  "sessions",
			Tags:      []string{"auto-breadcrumb", "session-end"},
			ProjectID: state.ProjectID,
		})
	}

	// 5. LOG SESSION SUMMARY
	slog.Info("session ended",
		"session_id", sessionID,
		"duration", time.Since(state.StartedAt),
		"memories_stored", count,
		"searches", len(state.RecentSearches),
		"topic", state.ActiveTopic,
	)

	// 6. CLEANUP STATE
	_ = stateManager.Delete(sessionID)

	// 7. RETURN
	return &HookOutput{
		Status:  "ok",
		Message: fmt.Sprintf("session ended, %d memories", count),
		Metadata: map[string]any{
			"memories_stored":    count,
			"searches_performed": len(state.RecentSearches),
			"session_duration":   time.Since(state.StartedAt).String(),
		},
	}, nil
}
