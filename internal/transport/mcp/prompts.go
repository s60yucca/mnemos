package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

func (s *Server) registerPrompts() {
	// load_context prompt
	s.mcpServer.AddPrompt(mcp.NewPrompt("load_context",
		mcp.WithPromptDescription("Load relevant context at session start"),
		mcp.WithArgument("query", mcp.ArgumentDescription("What context to load"), mcp.RequiredArgument()),
		mcp.WithArgument("project_id", mcp.ArgumentDescription("Project scope")),
		mcp.WithArgument("max_tokens", mcp.ArgumentDescription("Token budget")),
	), s.handleLoadContextPrompt)

	// save_session prompt
	s.mcpServer.AddPrompt(mcp.NewPrompt("save_session",
		mcp.WithPromptDescription("Save important learnings at session end"),
		mcp.WithArgument("project_id", mcp.ArgumentDescription("Project scope")),
		mcp.WithArgument("session_summary", mcp.ArgumentDescription("Summary of what was accomplished")),
	), s.handleSaveSessionPrompt)
}

func (s *Server) handleLoadContextPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	query := req.Params.Arguments["query"]
	projectID := req.Params.Arguments["project_id"]
	maxTokens := 4000

	result, err := s.mnemos.AssembleContext(ctx, query, projectID, maxTokens, true)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(result.Memories)
	promptText := fmt.Sprintf(
		"Here is the relevant context for your session:\n\n%s\n\nTotal tokens used: %d",
		string(data), result.TotalTokens,
	)

	return &mcp.GetPromptResult{
		Description: "Loaded context from Mnemos memory engine",
		Messages: []mcp.PromptMessage{
			{
				Role:    mcp.RoleUser,
				Content: mcp.TextContent{Type: "text", Text: promptText},
			},
		},
	}, nil
}

func (s *Server) handleSaveSessionPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := req.Params.Arguments["project_id"]
	summary := req.Params.Arguments["session_summary"]

	// List recent memories to show what's already saved
	recent, _ := s.mnemos.List(ctx, storage.ListQuery{
		ProjectID: projectID,
		Limit:     5,
		SortBy:    "created_at",
		SortDesc:  true,
	})

	recentData, _ := json.Marshal(recent)
	promptText := fmt.Sprintf(
		"Session summary: %s\n\nRecent memories in project '%s':\n%s\n\nUse mnemos_store to save any important learnings from this session.",
		summary, projectID, string(recentData),
	)

	return &mcp.GetPromptResult{
		Description: "Save session learnings to Mnemos",
		Messages: []mcp.PromptMessage{
			{
				Role:    mcp.RoleUser,
				Content: mcp.TextContent{Type: "text", Text: promptText},
			},
		},
	}, nil
}
