package autopilot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
)

const autoCompileTopic = "Project Knowledge Base"

type autoCompileResult struct {
	Compiled    bool
	ArticleID   string
	Topic       string
	SourceCount int
	Outcome     string
}

func (d *AutopilotDaemon) autoCompileProject(ctx context.Context, projectID string, memories []*domain.Memory) (autoCompileResult, error) {
	result := autoCompileResult{Topic: autoCompileTopic, Outcome: "skipped"}
	if !d.cfg.AutoCompileEnabled {
		d.observeCompile(projectID, result, "disabled")
		return result, nil
	}

	minSources := d.cfg.MinAutoCompileSources
	if minSources <= 0 {
		minSources = 5
	}

	sources, reason, err := d.selectAutoCompileSources(ctx, projectID, memories, minSources)
	if err != nil {
		result.Outcome = "error:select_sources"
		d.observeCompile(projectID, result, result.Outcome)
		return result, err
	}
	if len(sources) < minSources {
		d.observeCompile(projectID, result, reason)
		return result, nil
	}

	weakened, err := d.weakenPreviousAutoCompiles(ctx, projectID, autoCompileTopic)
	if err != nil {
		result.Outcome = "error:weaken_previous"
		d.observeCompile(projectID, result, result.Outcome)
		return result, err
	}

	content := formatAutoCompiledArticle(projectID, sources, time.Now().UTC())
	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}

	res, err := d.mnemos.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   content,
		Type:      domain.MemoryTypeCompiled,
		Category:  "compiled",
		Tags:      []string{"auto-compiled", "autopilot"},
		Source:    "autopilot-daemon",
		ProjectID: projectID,
		Metadata: map[string]string{
			"topic":       autoCompileTopic,
			"compiled_by": "autopilot",
			"compiled_at": time.Now().UTC().Format(time.RFC3339),
			"source_ids":  strings.Join(sourceIDs, ","),
			"version":     fmt.Sprintf("%d", weakened+1),
		},
	})
	if err != nil {
		result.Outcome = "error:store"
		d.observeCompile(projectID, result, result.Outcome)
		return result, err
	}

	sourceCount := 0
	for _, source := range sources {
		_, relErr := d.mnemos.Relate(ctx, &domain.RelateRequest{
			SourceID:     res.Memory.ID,
			TargetID:     source.ID,
			RelationType: domain.RelationTypeCompiledFrom,
			Strength:     1.0,
			Metadata: map[string]string{
				"detected_by": "autopilot",
			},
		})
		if relErr == nil {
			sourceCount++
			_ = d.mnemos.ReduceRelevance(ctx, source.ID, 0.3, 0.05)
		}
	}

	result.Compiled = true
	result.ArticleID = res.Memory.ID
	result.SourceCount = sourceCount
	result.Outcome = "ok"
	d.observeCompile(projectID, result, "ok")
	return result, nil
}

func (d *AutopilotDaemon) selectAutoCompileSources(ctx context.Context, projectID string, memories []*domain.Memory, minSources int) ([]*domain.Memory, string, error) {
	articles, err := d.mnemos.GetCompiledArticles(ctx, projectID, d.cfg.MaxCompiledPerRun)
	if err != nil {
		return nil, "", err
	}

	var newestCompiled time.Time
	for _, article := range articles {
		if article.Metadata != nil && article.Metadata["compiled_by"] != "autopilot" {
			continue
		}
		if article.CreatedAt.After(newestCompiled) {
			newestCompiled = article.CreatedAt
		}
	}

	sources := make([]*domain.Memory, 0, len(memories))
	for _, memory := range memories {
		if memory.Type == domain.MemoryTypeCompiled || memory.Category == "autopilot" {
			continue
		}
		if !newestCompiled.IsZero() && !memory.CreatedAt.After(newestCompiled) {
			continue
		}
		sources = append(sources, memory)
	}

	if len(sources) < minSources {
		if newestCompiled.IsZero() {
			return sources, "not_enough_sources", nil
		}
		return sources, "not_enough_new_sources", nil
	}

	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].QualityScore == sources[j].QualityScore {
			return sources[i].CreatedAt.After(sources[j].CreatedAt)
		}
		return sources[i].QualityScore > sources[j].QualityScore
	})
	if len(sources) > minSources {
		sources = sources[:minSources]
	}
	return sources, "ready", nil
}

func (d *AutopilotDaemon) weakenPreviousAutoCompiles(ctx context.Context, projectID, topic string) (int, error) {
	articles, err := d.mnemos.GetCompiledArticles(ctx, projectID, d.cfg.MaxCompiledPerRun)
	if err != nil {
		return 0, err
	}

	weakened := 0
	for _, article := range articles {
		if article.Metadata == nil {
			continue
		}
		if article.Metadata["topic"] != topic || article.Metadata["compiled_by"] != "autopilot" {
			continue
		}
		if err := d.mnemos.ReduceRelevance(ctx, article.ID, 0.5, 0.05); err == nil {
			weakened++
		}
	}
	return weakened, nil
}

func (d *AutopilotDaemon) observeCompile(projectID string, result autoCompileResult, outcome string) {
	observe.Feature("compile", map[string]any{
		"project":   projectID,
		"topic":     result.Topic,
		"sources":   result.SourceCount,
		"output_id": result.ArticleID,
		"outcome":   outcome,
		"mode":      "autopilot",
	})
}

func formatAutoCompiledArticle(projectID string, sources []*domain.Memory, compiledAt time.Time) string {
	titleProject := projectID
	if titleProject == "" {
		titleProject = "global"
	}

	var lines []string
	lines = append(lines, "# "+autoCompileTopic)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Project: `%s`", titleProject))
	lines = append(lines, fmt.Sprintf("Compiled: %s", compiledAt.Format(time.RFC3339)))
	lines = append(lines, "")
	lines = append(lines, "## Current Knowledge")

	for _, source := range sources {
		text := source.Summary
		if text == "" {
			text = source.Content
		}
		text = strings.ReplaceAll(text, "\n", " ")
		if len(text) > 180 {
			text = text[:177] + "..."
		}
		label := source.Category
		if label == "" {
			label = source.Type.String()
		}
		lines = append(lines, fmt.Sprintf("- **%s**: %s", label, text))
	}

	lines = append(lines, "")
	lines = append(lines, "## Source Memories")
	for _, source := range sources {
		lines = append(lines, "- "+source.ID)
	}

	return strings.Join(lines, "\n")
}
