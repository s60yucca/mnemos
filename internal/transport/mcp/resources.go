package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mnemos-dev/mnemos/internal/domain"
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
		return nil, err
	}

	data, _ := json.Marshal(stats)
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     fmt.Sprintf("%s", data),
		},
	}, nil
}
