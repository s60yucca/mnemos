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

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
)

func handleSessionStart(ctx context.Context, d *Dispatcher, input *HookInput) (*HookOutput, error) {
	// 1. PARSE PAYLOAD
	var payload SessionStartPayload
	if len(input.Payload) > 0 {
		_ = json.Unmarshal(input.Payload, &payload)
	}

	// 2. RESOLVE SESSION ID
	sessionID := resolveSessionID(input)

	// 3. DERIVE INITIAL QUERY — track where it came from
	query := ""
	if payload.TaskDescription != "" {
		query = payload.TaskDescription
	} else if payload.InitialPrompt != "" {
		query = payload.InitialPrompt
	} else if payload.WorkingDir != "" {
		query = filepath.Base(payload.WorkingDir)
		// Single directory component — not task-specific
	} else if projectDir := resolveProjectDir(input); projectDir != "" {
		// Claude Code's SessionStart payload provides cwd but no initial task prompt.
		// This is the broadest fallback — just the project directory name.
		query = filepath.Base(projectDir)
	}
	if query == "" {
		return &HookOutput{
			Status:  "skipped",
			Message: "no task context",
		}, nil
	}

	// 4. CLEAN STALE SESSIONS
	stateManager := NewStateManager(resolveProjectDir(input), d.cfg)
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

	// 6. ASSEMBLE CONTEXT — 3-tier dispatch
	quality := assessQueryQuality(query, state.ProjectID)

	if quality == QueryUseless {
		slog.Debug("session_start: query is useless, skipping context injection",
			"query", query, "project_id", state.ProjectID)
		return &HookOutput{
			Status:  "ok",
			Message: "session started, awaiting task-specific prompt for context",
			Metadata: map[string]any{
				"memories_found": 0,
				"tokens_used":    0,
				"query_source":   "useless_fallback",
			},
		}, nil
	}

	var contextString string
	var tokensUsed int
	var querySource string

	if quality == QuerySpecific {
		result, err := d.mnemos.AssembleContext(ctx, query, state.ProjectID, d.cfg.SessionStartMaxTokens, false)
		if err != nil {
			slog.Warn("session_start: context assembly failed", "err", err)
			return &HookOutput{
				Status:  "skipped",
				Message: "mnemos unavailable",
			}, nil
		}
		contextString = formatContextResult(ctx, d.mnemos, state.ProjectID, result)
		tokensUsed = result.TotalTokens
		querySource = "task_specific_query"
	} else { // QueryBroad
		contextString = assembleRecentContext(ctx, d.mnemos, state.ProjectID, d.cfg.SessionStartMaxTokens)
		if contextString == "" {
			return &HookOutput{
				Status:  "ok",
				Message: "session started, no recent context found",
				Metadata: map[string]any{
					"memories_found": 0,
					"tokens_used":    0,
					"query_source":   "broad_query",
				},
			}, nil
		}
		tokensUsed = len(contextString) / 4 // estimate
		querySource = "broad_query_recent_context"
	}

	// 7. RETURN CONTEXT
	return &HookOutput{
		ContextInjection: contextString,
		Status:           "ok",
		Metadata: map[string]any{
			"tokens_used":  tokensUsed,
			"query_source": querySource,
		},
		HookSpecificOutput: additionalContextOutput("SessionStart", contextString),
	}, nil
}

// formatContextResult formats a *search.ContextResult as markdown for the AI context window.
// It also queries the latest autopilot report for the project and injects it as section 3
// ("Autopilot Suggestions") when non-nil, between "Recent Skills" and "Last Session Summary".
func formatContextResult(ctx context.Context, m *core.Mnemos, projectID string, result *search.ContextResult) string {
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

	// Section 3: Autopilot Suggestions — injected after search results when available
	if projectID != "" {
		if report, err := m.GetLatestAutopilotReport(ctx, projectID); err == nil && report != nil {
			content := truncateAtSentenceBoundary(report.Content, 800)
			sb.WriteString("### Autopilot Suggestions\n\n")
			sb.WriteString(content)
			sb.WriteString("\n\n")
		} else if err != nil {
			slog.Debug("session_start: autopilot report query failed", "err", err)
		}
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

type QueryQuality int

const (
	QueryUseless QueryQuality = iota
	QueryBroad
	QuerySpecific
)

func assessQueryQuality(query, projectID string) QueryQuality {
	words := strings.Fields(strings.ToLower(query))
	if len(words) <= 1 || query == projectID {
		return QueryUseless
	}

	allStop := true
	for _, w := range words {
		if !stopWords[w] {
			allStop = false
			break
		}
	}
	if allStop {
		return QueryUseless
	}

	techTerms := extractTechnicalTerms(words)
	if len(techTerms) >= 1 && len(words) >= 3 {
		if len(techTerms) == 1 && techTerms[0] == projectID {
			return QueryBroad
		}
		return QuerySpecific
	}

	return QueryBroad
}

func assembleRecentContext(ctx context.Context, m *core.Mnemos, projectID string, maxTokens int) string {
	const (
		capCompiled  = 500
		capSkills    = 250
		capAutopilot = 200
		capSession   = 150
		capRecent    = 200
	)
	estimateTokens := func(s string) int { return len(s) / 4 }

	var sb strings.Builder
	totalTokens := 0

	addSection := func(title string, mems []*domain.Memory, cap int, truncator func(string, int) string) {
		if len(mems) == 0 {
			return
		}
		if totalTokens >= maxTokens {
			return
		}
		var secSb strings.Builder
		secSb.WriteString("### " + title + "\n\n")
		secTokens := estimateTokens(secSb.String())

		for _, mem := range mems {
			c := mem.Content
			if mem.Summary != "" {
				c = mem.Summary + "\n" + c
			}
			c = truncator(c, (cap-secTokens)*4)

			entry := fmt.Sprintf("- %s\n\n", c)
			tks := estimateTokens(entry)
			if secTokens+tks > cap {
				continue
			}
			secSb.WriteString(entry)
			secTokens += tks
		}

		if secTokens > estimateTokens("### "+title+"\n\n") {
			if totalTokens+secTokens > maxTokens {
				return
			}
			sb.WriteString(secSb.String())
			totalTokens += secTokens
		}
	}

	truncParagraph := func(s string, maxChars int) string {
		if len(s) <= maxChars {
			return s
		}
		truncated := s[:maxChars]
		if idx := strings.LastIndex(truncated, "\n\n"); idx > 0 {
			return truncated[:idx] + "\n...(truncated)"
		}
		return truncated + "..."
	}

	truncCodeBlock := func(s string, maxChars int) string {
		if len(s) <= maxChars {
			return s
		}
		truncated := s[:maxChars]
		if idx := strings.LastIndex(truncated, "```"); idx > 0 {
			if nextNewline := strings.Index(s[idx:], "\n"); nextNewline != -1 && idx+nextNewline < maxChars {
				return s[:idx+nextNewline] + "\n```\n...(truncated)"
			}
		}
		return truncated + "..."
	}

	truncLine := func(s string, maxChars int) string {
		if len(s) <= maxChars {
			return s
		}
		return s[:maxChars] + "..."
	}

	arts, _ := m.GetCompiledArticles(ctx, projectID, 3)
	addSection("Compiled Articles", arts, capCompiled, truncParagraph)

	memories, _ := m.GetRecentMemories(ctx, projectID, 30) // Get enough to filter
	var skills []*domain.Memory
	var recents []*domain.Memory
	for _, mm := range memories {
		if mm.Type == domain.MemoryTypeCompiled {
			continue // Already handled
		}
		if mm.Type == domain.MemoryTypeSkill && len(skills) < 3 {
			skills = append(skills, mm)
			continue
		}
		if len(recents) < 5 {
			recents = append(recents, mm)
		}
	}

	addSection("Recent Skills", skills, capSkills, truncCodeBlock)

	// Section 3: Autopilot Suggestions
	if projectID != "" {
		if report, err := m.GetLatestAutopilotReport(ctx, projectID); err == nil && report != nil {
			content := truncateAtSentenceBoundary(report.Content, 800)
			if totalTokens < maxTokens {
				secTokens := estimateTokens("### Autopilot Suggestions\n\n" + content + "\n\n")
				if secTokens <= capAutopilot && totalTokens+secTokens <= maxTokens {
					sb.WriteString("### Autopilot Suggestions\n\n")
					sb.WriteString(content)
					sb.WriteString("\n\n")
					totalTokens += secTokens
				}
			}
		} else if err != nil {
			slog.Debug("session_start: autopilot report query failed", "err", err)
		}
	}

	summary, _ := m.GetLastSessionSummary(ctx, projectID)
	if summary != nil {
		addSection("Last Session Summary", []*domain.Memory{summary}, capSession, truncParagraph)
	}

	addSection("Recent Activity", recents, capRecent, truncLine)

	if sb.Len() == 0 {
		return ""
	}
	return "## Recent Context\n\n" + sb.String()
}

// truncateAtSentenceBoundary truncates s to at most maxChars characters,
// preferring to cut at a sentence boundary (". ") to avoid mid-sentence truncation.
func truncateAtSentenceBoundary(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	truncated := s[:maxChars]
	// Try to cut at the last sentence boundary
	if idx := strings.LastIndex(truncated, ". "); idx > 0 {
		return truncated[:idx+1]
	}
	// Fall back to last newline
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		return truncated[:idx]
	}
	return truncated
}
