package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mnemos-dev/mnemos/internal/benchmark"
	core "github.com/mnemos-dev/mnemos/internal/core"
)

// Server wraps the MCP server with Mnemos tools
type Server struct {
	mnemos         *core.Mnemos
	mcpServer      *server.MCPServer
	benchMode      benchmark.BenchMode
	dataDir        string
	sessionTracker *benchmark.SessionTracker
}

// NewServer creates and configures the MCP server
func NewServer(mnemos *core.Mnemos, version string) *Server {
	// Get data directory
	dataDir := getDataDir()

	// Read bench mode
	benchMode, err := benchmark.ReadBenchMode(dataDir)
	if err != nil {
		benchMode = benchmark.BenchModeOn // Default to ON
	}

	// Create session tracker
	sessionTracker, err := benchmark.NewSessionTracker(dataDir)
	if err != nil {
		// Log error but continue - session tracking is optional
		fmt.Fprintf(os.Stderr, "Warning: failed to create session tracker: %v\n", err)
	}

	s := &Server{
		mnemos:         mnemos,
		benchMode:      benchMode,
		dataDir:        dataDir,
		sessionTracker: sessionTracker,
	}

	s.mcpServer = server.NewMCPServer(
		"mnemos",
		version,
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

// Shutdown gracefully shuts down the MCP server and session tracker
func (s *Server) Shutdown() {
	if s.sessionTracker != nil {
		s.sessionTracker.Stop()
	}
}

// trackMCPCall records MCP activity for benchmark tracking
func (s *Server) trackMCPCall(projectID string, reqContent string, respContent string) {
	if s.sessionTracker != nil {
		s.sessionTracker.OnMCPCall(projectID, reqContent, respContent)
	}
}

func mcpError(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("error: %s", msg))
}

func mcpText(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

// getDataDir returns the mnemos data directory
func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mnemos")
	}
	return filepath.Join(home, ".mnemos")
}
