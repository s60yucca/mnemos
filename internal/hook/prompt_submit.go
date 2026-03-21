package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/storage"
)

func handlePromptSubmit(ctx context.Context, d *Dispatcher, input *HookInput) (*HookOutput, error) {
	var payload PromptSubmitPayload
	if len(input.Payload) > 0 {
		_ = json.Unmarshal(input.Payload, &payload)
	}
	promptText := payload.PromptText
	if promptText == "" {
		return &HookOutput{Status: "skipped", Message: "empty prompt"}, nil
	}
	if IsGenericPrompt(promptText) {
		return &HookOutput{Status: "skipped", Message: "generic prompt"}, nil
	}
	sessionID := resolveSessionID(input)
	stateManager := NewStateManager(input.ProjectDir, d.cfg)
	state := stateManager.Get(sessionID)
	if state == nil {
		state = &SessionState{
			SessionID: sessionID,
			ProjectID: resolveProjectID(input),
			StartedAt: time.Now(),
			PID:       os.Getpid(),
		}
	}
	newTopic := DetectTopic(promptText)
	if newTopic == "" {
		return &HookOutput{Status: "skipped", Message: "no topic detected"}, nil
	}
	threshold := d.cfg.TopicSimilarityThreshold
	if state.ActiveTopic != "" && !TopicChanged(newTopic, state.ActiveTopic, threshold) {
		lastSearch := findLastSearchForTopic(state, newTopic)
		if lastSearch != nil && time.Since(lastSearch.Timestamp) < d.cfg.SearchCooldown {
			return &HookOutput{Status: "skipped", Message: "cooldown active for topic"}, nil
		}
	}
	results, err := d.mnemos.Search(ctx, newTopic, state.ProjectID, d.cfg.PromptSearchLimit)
	if err != nil {
		slog.Warn("prompt_submit: search failed", "err", err)
		return &HookOutput{Status: "skipped", Message: "search unavailable"}, nil
	}
	now := time.Now()
	state.ActiveTopic = newTopic
	state.RecentSearches = append(state.RecentSearches, SearchEntry{
		Query:     newTopic,
		Topic:     newTopic,
		Timestamp: now,
	})
	if len(state.RecentSearches) > 50 {
		state.RecentSearches = state.RecentSearches[len(state.RecentSearches)-50:]
	}
	state.LastActivity = now
	if err := stateManager.Save(state); err != nil {
		slog.Warn("prompt_submit: failed to save state", "err", err)
	}
	if len(results) == 0 {
		return &HookOutput{Status: "ok", Message: "searched, no results"}, nil
	}
	return &HookOutput{
		ContextInjection: formatSearchResults(results),
		Status:           "ok",
		Metadata:         map[string]any{"memories_found": len(results)},
	}, nil
}

func findLastSearchForTopic(state *SessionState, newTopic string) *SearchEntry {
	for i := len(state.RecentSearches) - 1; i >= 0; i-- {
		if state.RecentSearches[i].Topic == newTopic {
			return &state.RecentSearches[i]
		}
	}
	return nil
}

func formatSearchResults(results []*storage.SearchResult) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Relevant Memories\n\n")
	for i, r := range results {
		if r.Memory == nil {
			continue
		}
		mem := r.Memory
		if mem.Category != "" {
			sb.WriteString(fmt.Sprintf("### %d. [%s] %s\n\n", i+1, mem.Category, mem.Type))
		} else {
			sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, mem.Type))
		}
		content := mem.Content
		if mem.Summary != "" {
			content = mem.Summary
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
		if len(mem.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("**Tags:** %s\n\n", strings.Join(mem.Tags, ", ")))
		}
		score := r.HybridScore
		if score == 0 {
			score = r.TextScore
		}
		if score > 0 {
			sb.WriteString(fmt.Sprintf("**Relevance:** %.2f\n\n", score))
		}
	}
	return strings.TrimRight(sb.String(), "\n") + "\n"
}
