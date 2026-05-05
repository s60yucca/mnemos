package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

func (s *Server) registerResources() {
	// mnemos://memories/{project_id}
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"mnemos://memories/{project_id}",
			"Active memories for a project",
			mcp.WithTemplateDescription("List all active memories for the given project_id"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleMemoriesResource,
	)

	// mnemos://stats
	s.mcpServer.AddResource(
		mcp.NewResource(
			"mnemos://stats",
			"Storage statistics",
			mcp.WithResourceDescription("Overall storage statistics"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleStatsResource,
	)

	// mnemos://session-context
	s.mcpServer.AddResource(
		mcp.NewResource(
			"mnemos://session-context",
			"Current project context from mnemos memory base",
			mcp.WithResourceDescription("Pull-based auto-inject for Codex and other non-hook clients"),
			mcp.WithMIMEType("text/plain"),
		),
		s.handleSessionContextResource,
	)
}

func (s *Server) handleMemoriesResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	projectID := req.Params.URI
	// Extract project_id from URI: mnemos://memories/{project_id}
	const prefix = "mnemos://memories/"
	if len(projectID) > len(prefix) {
		projectID = projectID[len(prefix):]
	} else {
		projectID = ""
	}

	memories, err := s.mnemos.List(ctx, storage.ListQuery{
		ProjectID: projectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     100,
	})
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(memories)
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleStatsResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats, err := s.mnemos.Stats(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	bytes, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      req.Params.URI,
		MIMEType: "application/json",
		Text:     string(bytes),
	}}, nil
}

func (s *Server) handleSessionContextResource(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	detector := hook.NewProjectDetector(cwd, s.dataDir)
	projectID, _, _ := detector.Detect()

	if projectID == "" {
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      "mnemos://session-context",
			MIMEType: "text/plain",
			Text:     "# No project context detected",
		}}, nil
	}

	cfg := hook.AutoInjectConfigFromEnv()
	injector := hook.NewAutoInjector(s.mnemos, cfg, s.dataDir)
	payload, _, _ := injector.Run(ctx, "codex-resource", projectID, "codex", nil)
	if payload == "" {
		payload = "# No memories found for project " + projectID
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      "mnemos://session-context",
		MIMEType: "text/plain",
		Text:     payload,
	}}, nil
}
