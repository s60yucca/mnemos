package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	core "github.com/mnemos-dev/mnemos/internal/core"
)

// Server wraps the MCP server with Mnemos tools
type Server struct {
	mnemos    *core.Mnemos
	mcpServer *server.MCPServer
}

// NewServer creates and configures the MCP server
func NewServer(mnemos *core.Mnemos) *Server {
	s := &Server{mnemos: mnemos}

	s.mcpServer = server.NewMCPServer(
		"mnemos",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s
}

// ServeStdio starts the MCP server on stdio
func (s *Server) ServeStdio(ctx context.Context) error {
	return server.ServeStdio(s.mcpServer)
}

func mcpError(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("error: %s", msg))
}

func mcpText(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}
