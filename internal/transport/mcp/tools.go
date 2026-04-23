package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

func (s *Server) registerTools() {
	// mnemos_store
	s.mcpServer.AddTool(mcp.NewTool("mnemos_store",
		mcp.WithDescription("Store a new memory in Mnemos"),
		mcp.WithString("content", mcp.Required(), mcp.Description("Memory content (1 byte to 100KB)")),
		mcp.WithString("summary", mcp.Description("Optional summary")),
		mcp.WithString("type", mcp.Description("Memory type: short_term|long_term|episodic|semantic|skill|compiled")),
		mcp.WithString("category", mcp.Description("Memory category")),
		mcp.WithString("project_id", mcp.Description("Project scope")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags")),
		mcp.WithString("source", mcp.Description("Source identifier")),
	), s.handleStore)

	// mnemos_search
	s.mcpServer.AddTool(mcp.NewTool("mnemos_search",
		mcp.WithDescription("Search memories using hybrid text+semantic search"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("project_id", mcp.Description("Filter by project")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 10)")),
		mcp.WithString("mode", mcp.Description("Search mode: text|semantic|hybrid (default hybrid)")),
	), s.handleSearch)

	// mnemos_get
	s.mcpServer.AddTool(mcp.NewTool("mnemos_get",
		mcp.WithDescription("Get a memory by ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
	), s.handleGet)

	// mnemos_update
	s.mcpServer.AddTool(mcp.NewTool("mnemos_update",
		mcp.WithDescription("Update a memory (PATCH semantics)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithString("content", mcp.Description("New content")),
		mcp.WithString("summary", mcp.Description("New summary")),
		mcp.WithString("tags", mcp.Description("New comma-separated tags")),
	), s.handleUpdate)

	// mnemos_delete
	s.mcpServer.AddTool(mcp.NewTool("mnemos_delete",
		mcp.WithDescription("Soft-delete a memory"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
	), s.handleDelete)

	// mnemos_relate
	s.mcpServer.AddTool(mcp.NewTool("mnemos_relate",
		mcp.WithDescription("Create a relation between two memories"),
		mcp.WithString("source_id", mcp.Required(), mcp.Description("Source memory ID")),
		mcp.WithString("target_id", mcp.Required(), mcp.Description("Target memory ID")),
		mcp.WithString("relation_type", mcp.Required(), mcp.Description("Relation type: relates_to|depends_on|contradicts|supersedes|derived_from|part_of|caused_by")),
		mcp.WithNumber("strength", mcp.Description("Relation strength [0.0, 1.0]")),
	), s.handleRelate)

	// mnemos_context
	s.mcpServer.AddTool(mcp.NewTool("mnemos_context",
		mcp.WithDescription("Assemble relevant context for a query within token budget"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Context query")),
		mcp.WithString("project_id", mcp.Description("Project scope")),
		mcp.WithNumber("max_tokens", mcp.Description("Token budget (default 4000)")),
		mcp.WithBoolean("include_relations", mcp.Description("Include related memories")),
	), s.handleContext)

	// mnemos_maintain
	s.mcpServer.AddTool(mcp.NewTool("mnemos_maintain",
		mcp.WithDescription("Run decay, archival, and GC maintenance"),
		mcp.WithString("project_id", mcp.Description("Project scope (empty = all)")),
	), s.handleMaintain)

	// mnemos_compile
	s.mcpServer.AddTool(mcp.NewTool("mnemos_compile",
		mcp.WithDescription("Distill knowledge into a compiled article"),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Title/subject of the compiled article")),
		mcp.WithString("content", mcp.Required(), mcp.Description("The compiled text")),
		mcp.WithString("project_id", mcp.Description("Project scope")),
		mcp.WithString("source_ids", mcp.Description("Comma-separated source memory IDs")),
	), s.handleCompile)
}

func (s *Server) handleStore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content := req.GetString("content", "")
	if content == "" {
		return mcpError("content is required"), nil
	}

	storeReq := &domain.StoreRequest{
		Content:   content,
		Summary:   req.GetString("summary", ""),
		Source:    req.GetString("source", ""),
		ProjectID: req.GetString("project_id", ""),
	}

	if t := req.GetString("type", ""); t != "" {
		storeReq.Type = domain.MemoryType(t)
	}
	if cat := req.GetString("category", ""); cat != "" {
		storeReq.Category = cat
	}
	if tags := req.GetString("tags", ""); tags != "" {
		storeReq.Tags = splitTags(tags)
	}

	result, err := s.mnemos.Store(ctx, storeReq)
	if err != nil {
		return mcpError(err.Error()), nil
	}

	// Instrument MCP call for active day detection
	observe.Feature("store_call", map[string]any{
		"project": storeReq.ProjectID,
	})

	// Instrument feature-specific events (Tasks 8-11)
	// These features fire during Store() but we log them here (MCP-only instrumentation per Design Decision 1)

	// Task 8: quality_gate - fires on every store
	if result.Memory != nil {
		action := "accept"
		if !result.Created {
			action = "merge"
		} else if result.QualityNote != "" {
			action = "fix"
		}
		observe.Feature("quality_gate", map[string]any{
			"score":   result.Memory.QualityScore,
			"action":  action,
			"project": storeReq.ProjectID,
		})
	}

	// Task 9: dedup - fires on every store (both hit and miss)
	observe.Feature("dedup", map[string]any{
		"hit":     !result.Created,
		"project": storeReq.ProjectID,
	})

	// Task 10: summarize - fires when auto-summarization occurs
	if result.Memory != nil && result.Memory.Summary != "" && storeReq.Summary == "" {
		observe.Feature("summarize", map[string]any{
			"memory_id": result.Memory.ID,
			"method":    "auto",
			"length":    len(result.Memory.Summary),
		})
	}

	// Task 11: file_link - fires when file paths are detected
	if result.Memory != nil && result.Memory.Metadata != nil {
		if relatedFiles, ok := result.Memory.Metadata["related_files"]; ok && relatedFiles != "" {
			fileCount := strings.Count(relatedFiles, ",") + 1
			observe.Feature("file_link", map[string]any{
				"memory_id": result.Memory.ID,
				"count":     fileCount,
			})
		}
	}

	out, _ := json.Marshal(result)
	return mcpText(string(out)), nil
}

func (s *Server) handleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return mcpError("query is required"), nil
	}

	projectID := req.GetString("project_id", "")
	limit := req.GetInt("limit", 10)
	mode := req.GetString("mode", "hybrid")

	var results []*storage.SearchResult
	var err error

	switch mode {
	case "text":
		results, err = s.mnemos.TextSearch(ctx, storage.TextSearchQuery{
			Query:     query,
			ProjectID: projectID,
			Limit:     limit,
		})
	case "semantic":
		results, err = s.mnemos.SemanticSearch(ctx, query, projectID, limit, 0.5)
	case "hybrid", "":
		results, err = s.mnemos.Search(ctx, query, projectID, limit)
	default:
		return mcpError("mode must be one of: text, semantic, hybrid"), nil
	}

	if err != nil {
		return mcpError(err.Error()), nil
	}

	// Instrument MCP call for active day detection
	observe.Feature("search_call", map[string]any{
		"project": projectID,
		"mode":    mode,
	})

	// Task 12: mmr - fires on every search call (diversity filter is always applied)
	if len(results) > 0 {
		observe.Feature("mmr", map[string]any{
			"selected": len(results),
			"lambda":   0.7, // default lambda from assembler
		})
	}

	for _, res := range results {
		if res.Memory != nil {
			if res.Memory.Type == domain.MemoryTypeCompiled {
				res.MatchSnippet = fmt.Sprintf("[%s] %s", "compiled", res.MatchSnippet)
				res.Memory.Content = fmt.Sprintf("### [compiled]\n%s", res.Memory.Content)
			} else if res.Memory.Type == domain.MemoryTypeSkill {
				res.MatchSnippet = fmt.Sprintf("[%s] %s", "skill", res.MatchSnippet)
				res.Memory.Content = fmt.Sprintf("### [skill]\n%s", res.Memory.Content)
			}
		}
	}

	out, _ := json.Marshal(results)
	return mcpText(string(out)), nil
}

func (s *Server) handleGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := req.GetString("id", "")
	if id == "" {
		return mcpError("id is required"), nil
	}

	mem, err := s.mnemos.Get(ctx, id)
	if err != nil {
		return mcpError(err.Error()), nil
	}

	out, _ := json.Marshal(mem)
	return mcpText(string(out)), nil
}

func (s *Server) handleUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := req.GetString("id", "")
	if id == "" {
		return mcpError("id is required"), nil
	}

	updateReq := &domain.UpdateRequest{ID: id}
	if c := req.GetString("content", ""); c != "" {
		updateReq.Content = &c
	}
	if s2 := req.GetString("summary", ""); s2 != "" {
		updateReq.Summary = &s2
	}
	if tags := req.GetString("tags", ""); tags != "" {
		updateReq.Tags = splitTags(tags)
	}

	mem, err := s.mnemos.Update(ctx, updateReq)
	if err != nil {
		return mcpError(err.Error()), nil
	}

	out, _ := json.Marshal(mem)
	return mcpText(string(out)), nil
}

func (s *Server) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := req.GetString("id", "")
	if id == "" {
		return mcpError("id is required"), nil
	}

	if err := s.mnemos.Delete(ctx, id); err != nil {
		return mcpError(err.Error()), nil
	}
	return mcpText(fmt.Sprintf(`{"deleted":true,"id":%q}`, id)), nil
}

func (s *Server) handleRelate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceID := req.GetString("source_id", "")
	targetID := req.GetString("target_id", "")
	relType := req.GetString("relation_type", "")

	if sourceID == "" || targetID == "" || relType == "" {
		return mcpError("source_id, target_id, and relation_type are required"), nil
	}

	strength := req.GetFloat("strength", 1.0)

	rel, err := s.mnemos.Relate(ctx, &domain.RelateRequest{
		SourceID:     sourceID,
		TargetID:     targetID,
		RelationType: domain.RelationType(relType),
		Strength:     strength,
	})
	if err != nil {
		return mcpError(err.Error()), nil
	}

	out, _ := json.Marshal(rel)
	return mcpText(string(out)), nil
}

func (s *Server) handleContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	if query == "" {
		return mcpError("query is required"), nil
	}

	projectID := req.GetString("project_id", "")
	maxTokens := req.GetInt("max_tokens", 4000)
	includeRelations := req.GetBool("include_relations", false)

	result, err := s.mnemos.AssembleContext(ctx, query, projectID, maxTokens, includeRelations, nil)
	if err != nil {
		return mcpError(err.Error()), nil
	}

	// Instrument MCP call for active day detection
	observe.Feature("context_call", map[string]any{
		"project": projectID,
	})

	// Task 12: mmr - fires on every context call (diversity filter is always applied)
	if result != nil && len(result.Memories) > 0 {
		observe.Feature("mmr", map[string]any{
			"selected": len(result.Memories),
			"lambda":   0.7, // default lambda from assembler
		})
	}

	out, _ := json.Marshal(result)
	return mcpText(string(out)), nil
}

func (s *Server) handleMaintain(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := req.GetString("project_id", "")
	if err := s.mnemos.Maintain(ctx, projectID); err != nil {
		return mcpError(err.Error()), nil
	}

	// Task 17: decay - fires on every maintenance operation
	observe.Feature("decay", map[string]any{
		"project": projectID,
		"outcome": "ok",
	})

	return mcpText(`{"status":"ok","message":"maintenance complete"}`), nil
}

func (s *Server) handleCompile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic := req.GetString("topic", "")
	if topic == "" {
		return mcpError("topic is required"), nil
	}
	content := req.GetString("content", "")
	if content == "" {
		return mcpError("content is required"), nil
	}

	projectID := req.GetString("project_id", "")
	sourceIDsStr := req.GetString("source_ids", "")

	// 1. Find previous versions to weaken
	var previousArts []*domain.Memory
	arts, err := s.mnemos.GetCompiledArticles(ctx, projectID, 50)
	if err == nil {
		for _, a := range arts {
			if a.Metadata != nil && strings.EqualFold(a.Metadata["topic"], topic) {
				previousArts = append(previousArts, a)
			}
		}
	}

	weakenedCount := 0
	for _, a := range previousArts {
		if err := s.mnemos.ReduceRelevance(ctx, a.ID, 0.5, 0.05); err == nil {
			weakenedCount++
		}
	}

	// 2. Store new article
	storeReq := &domain.StoreRequest{
		Content:   content,
		Type:      domain.MemoryTypeCompiled,
		ProjectID: projectID,
		Metadata: map[string]string{
			"topic":       topic,
			"compiled_by": "agent",
			"compiled_at": time.Now().UTC().Format(time.RFC3339),
			"version":     fmt.Sprintf("%d", weakenedCount+1),
		},
	}
	if sourceIDsStr != "" {
		storeReq.Metadata["source_ids"] = sourceIDsStr
	}

	res, err := s.mnemos.Store(ctx, storeReq)
	if err != nil {
		return mcpError(err.Error()), nil
	}
	articleID := res.Memory.ID

	// 3. Create relations and reduce source strength
	sourceCount := 0
	if sourceIDsStr != "" {
		sources := strings.Split(sourceIDsStr, ",")
		for _, srcID := range sources {
			srcID = strings.TrimSpace(srcID)
			if srcID == "" {
				continue
			}
			_, relErr := s.mnemos.Relate(ctx, &domain.RelateRequest{
				SourceID:     articleID,
				TargetID:     srcID,
				RelationType: domain.RelationTypeCompiledFrom,
				Strength:     1.0,
			})
			if relErr == nil {
				sourceCount++
				s.mnemos.ReduceRelevance(ctx, srcID, 0.3, 0.05)
			}
		}
	}

	msg := fmt.Sprintf("Compiled %d memories into '%s' (%d previous version weakened)", sourceCount, topic, weakenedCount)
	outMap := map[string]any{
		"article_id":                 articleID,
		"topic":                      topic,
		"source_count":               sourceCount,
		"previous_articles_weakened": weakenedCount,
		"message":                    msg,
	}

	// Task 16: compile - fires on every compile operation
	observe.Feature("compile", map[string]any{
		"topic":     topic,
		"sources":   sourceCount,
		"output_id": articleID,
		"outcome":   "ok",
	})

	out, _ := json.Marshal(outMap)
	return mcpText(string(out)), nil
}

// --- helpers ---

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
