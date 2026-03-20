package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

func (s *Server) registerTools() {
	// mnemos_store
	s.mcpServer.AddTool(mcp.NewTool("mnemos_store",
		mcp.WithDescription("Store a new memory in Mnemos"),
		mcp.WithString("content", mcp.Required(), mcp.Description("Memory content (1 byte to 100KB)")),
		mcp.WithString("summary", mcp.Description("Optional summary")),
		mcp.WithString("type", mcp.Description("Memory type: short_term|long_term|episodic|semantic")),
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

	result, err := s.mnemos.AssembleContext(ctx, query, projectID, maxTokens, includeRelations)
	if err != nil {
		return mcpError(err.Error()), nil
	}

	out, _ := json.Marshal(result)
	return mcpText(string(out)), nil
}

func (s *Server) handleMaintain(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := req.GetString("project_id", "")
	if err := s.mnemos.Maintain(ctx, projectID); err != nil {
		return mcpError(err.Error()), nil
	}
	return mcpText(`{"status":"ok","message":"maintenance complete"}`), nil
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
