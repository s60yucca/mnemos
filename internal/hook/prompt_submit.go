package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

func handlePromptSubmit(ctx context.Context, d *Dispatcher, input *HookInput) (*HookOutput, error) {
	var payload PromptSubmitPayload
	if len(input.Payload) > 0 {
		_ = json.Unmarshal(input.Payload, &payload)
	}
	promptText := payload.PromptText
	if promptText == "" {
		promptText = input.Prompt
	}
	if promptText == "" {
		return &HookOutput{Status: "skipped", Message: "empty prompt"}, nil
	}
	if IsGenericPrompt(promptText) {
		return &HookOutput{Status: "skipped", Message: "generic prompt"}, nil
	}

	// Detect storable moments early — before the cooldown check so the nudge
	// always surfaces regardless of whether a memory search fires.
	nudge := detectStorableMoment(promptText)

	sessionID := resolveSessionID(input)
	stateManager := NewStateManager(resolveProjectDir(input), d.cfg)
	state := stateManager.Get(sessionID)
	if state == nil {
		state = &SessionState{
			SessionID: sessionID,
			ProjectID: resolveProjectID(input),
			StartedAt: time.Now(),
			PID:       os.Getpid(),
		}
	}

	// Apply nudge throttle before any path that might emit one.
	// Rules (ALL must pass):
	//   1. Session is at least 5 minutes old (don't nudge right at session start)
	//   2. Last nudge was more than 10 minutes ago (don't nag every prompt)
	nudge = throttleNudge(nudge, state)
	newTopic := DetectTopic(promptText)
	if newTopic == "" {
		// Even with no topic, surface a nudge if we detected a storable moment
		if nudge != nil {
			state.LastNudgeAt = time.Now()
			_ = stateManager.Save(state)
			nudgeCtx := fmt.Sprintf("💡 **Store reminder**: %s", nudge.hint)
			return &HookOutput{
				ContextInjection:   nudgeCtx,
				Status:             "ok",
				Metadata:           map[string]any{"storable_signal": nudge.signalType},
				HookSpecificOutput: additionalContextOutput("UserPromptSubmit", nudgeCtx),
			}, nil
		}
		return &HookOutput{Status: "skipped", Message: "no topic detected"}, nil
	}
	threshold := d.cfg.TopicSimilarityThreshold
	topicChanged := false
	if state.ActiveTopic != "" && !TopicChanged(newTopic, state.ActiveTopic, threshold) {
		lastSearch := findLastSearchForTopic(state, newTopic)
		if lastSearch != nil && time.Since(lastSearch.Timestamp) < d.cfg.SearchCooldown {
			// Cooldown active for search — but still show a nudge if the moment warrants it
			if nudge != nil {
				state.LastNudgeAt = time.Now()
				_ = stateManager.Save(state)
				nudgeCtx := fmt.Sprintf("💡 **Store reminder**: %s", nudge.hint)
				return &HookOutput{
					ContextInjection:   nudgeCtx,
					Status:             "ok",
					Metadata:           map[string]any{"storable_signal": nudge.signalType, "search": "cooldown"},
					HookSpecificOutput: additionalContextOutput("UserPromptSubmit", nudgeCtx),
				}, nil
			}
			return &HookOutput{Status: "skipped", Message: "cooldown active for topic"}, nil
		}
	} else if state.ActiveTopic != "" {
		// Topic changed - set flag for instrumentation
		topicChanged = true
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

	// Task 18: topic_shift - fires when topic changes
	if topicChanged {
		observe.Feature("topic_shift", map[string]any{
			"changed": true,
			"topic":   newTopic,
		})
	}

	// Build final context: search results + optional store nudge
	var parts []string
	if len(results) > 0 {
		parts = append(parts, formatSearchResults(results))
	}
	if nudge != nil {
		state.LastNudgeAt = time.Now()
		parts = append(parts, fmt.Sprintf("💡 **Store reminder**: %s", nudge.hint))
	}
	if len(parts) == 0 {
		return &HookOutput{Status: "ok", Message: "searched, no results"}, nil
	}
	ctx2 := strings.Join(parts, "\n")
	meta := map[string]any{"memories_found": len(results)}
	if nudge != nil {
		meta["storable_signal"] = nudge.signalType
	}
	return &HookOutput{
		ContextInjection:   ctx2,
		Status:             "ok",
		Metadata:           meta,
		HookSpecificOutput: additionalContextOutput("UserPromptSubmit", ctx2),
	}, nil
}

func findLastSearchForTopic(state *SessionState, newTopic string) *SearchEntry {
	// Use Jaccard similarity instead of exact string match: topics like
	// "jwt auth" and "jwt authentication" are the same for cooldown purposes.
	for i := len(state.RecentSearches) - 1; i >= 0; i-- {
		e := &state.RecentSearches[i]
		if !TopicChanged(newTopic, e.Topic, 0.4) { // similarity >= 0.4 → same topic
			return e
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

func additionalContextOutput(eventName, context string) *HookSpecificOutput {
	if strings.TrimSpace(context) == "" {
		return nil
	}
	return &HookSpecificOutput{
		HookEventName:     eventName,
		AdditionalContext: context,
	}
}

// storeNudge holds a detected storable-moment signal.
type storeNudge struct {
	signalType string
	hint       string
}

// detectStorableMoment scans for signals in the user's prompt that indicate
// something durable just happened: a task completed, a discovery was made, or
// a technical decision was taken. These are the moments steering files miss
// because the agent is focused on the task, not on memory management.
//
// Signal detection operates on the user (human) prompt, not the agent response,
// because UserPromptSubmit fires before the agent replies. Common patterns:
//
//	"that fixed it" / "working now" → completion
//	"the issue was" / "turns out" → discovery / root cause
//	"let's go with" / "decided to" → technical decision
func detectStorableMoment(prompt string) *storeNudge {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return nil
	}

	// --- Completion signals ---
	completionPhrases := []string{
		"fixed it", "that fixed", "working now", "it works", "that works",
		"tests pass", "test passes", "build succeeded", "build passed",
		"deployed", "merged", "resolved", "all good now", "problem solved",
		"issue is closed", "pr is merged", "ship it", "shipped",
	}
	for _, phrase := range completionPhrases {
		if strings.Contains(lower, phrase) {
			return &storeNudge{
				signalType: "completion",
				hint:       "Something was just completed or fixed — store the outcome and approach via `mnemos_store`.",
			}
		}
	}

	// --- Discovery / root-cause signals ---
	discoveryPhrases := []string{
		"the issue was", "root cause", "turns out", "found the bug",
		"realized that", "the problem is", "it was because", "the reason is",
		"figured out", "it was the", "found out", "discovered that",
	}
	for _, phrase := range discoveryPhrases {
		if strings.Contains(lower, phrase) {
			return &storeNudge{
				signalType: "discovery",
				hint:       "A root cause or key finding was mentioned — store it via `mnemos_store` so it's not lost.",
			}
		}
	}

	// --- Decision signals ---
	decisionPhrases := []string{
		"let's go with", "decided to", "we'll use", "going with", "switching to",
		"the approach is", "we decided", "final decision", "using instead",
	}
	for _, phrase := range decisionPhrases {
		if strings.Contains(lower, phrase) {
			return &storeNudge{
				signalType: "decision",
				hint:       "A technical decision was just made — store it with the rationale via `mnemos_store`.",
			}
		}
	}

	return nil
}

// throttleNudge applies per-session nudge rate limiting.
// Returns nil (suppressed) when:
//   - The session is younger than 5 minutes (agent is still loading context)
//   - The last nudge was within 10 minutes (avoid nagging)
//
// Returns nudge unchanged when throttle conditions are not met.
func throttleNudge(nudge *storeNudge, state *SessionState) *storeNudge {
	if nudge == nil {
		return nil
	}
	const (
		minSessionAge = 5 * time.Minute
		nudgeCooldown = 10 * time.Minute
	)
	if time.Since(state.StartedAt) < minSessionAge {
		return nil // too early in session
	}
	if !state.LastNudgeAt.IsZero() && time.Since(state.LastNudgeAt) < nudgeCooldown {
		return nil // nudged recently
	}
	return nudge
}
