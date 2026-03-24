package setup

// FileMapping maps a template path to local and global destination paths.
type FileMapping struct {
	TemplatePath string // e.g. "templates/claude/CLAUDE.md"
	LocalPath    string // relative to project dir, e.g. "CLAUDE.md"
	GlobalPath   string // relative to home dir, e.g. "CLAUDE.md"
}

// MCPMapping defines the MCP config file paths.
type MCPMapping struct {
	LocalPath  string // relative to project dir
	GlobalPath string // relative to home dir
}

// ClientConfig defines all file paths for a specific AI client.
type ClientConfig struct {
	Name      string
	Files     []FileMapping
	MCPConfig MCPMapping
}

// Clients maps client names to their configurations.
var Clients = map[string]ClientConfig{
	"claude": {
		Name: "claude",
		Files: []FileMapping{
			{
				TemplatePath: "templates/claude/CLAUDE.md",
				LocalPath:    "CLAUDE.md",
				GlobalPath:   "CLAUDE.md",
			},
			{
				TemplatePath: "templates/claude/hooks.json",
				LocalPath:    ".claude/hooks.json",
				GlobalPath:   ".claude/hooks.json",
			},
		},
		MCPConfig: MCPMapping{
			LocalPath:  ".mcp.json",
			GlobalPath: ".mcp.json",
		},
	},
	"kiro": {
		Name: "kiro",
		Files: []FileMapping{
			{
				TemplatePath: "templates/kiro/steering/mnemos.md",
				LocalPath:    ".kiro/steering/mnemos.md",
				GlobalPath:   ".kiro/steering/mnemos.md",
			},
		},
		MCPConfig: MCPMapping{
			LocalPath:  ".kiro/settings/mcp.json",
			GlobalPath: ".kiro/settings/mcp.json",
		},
	},
	"cursor": {
		Name: "cursor",
		Files: []FileMapping{
			{
				TemplatePath: "templates/cursor/.cursorrules",
				LocalPath:    ".cursorrules",
				GlobalPath:   ".cursorrules",
			},
		},
		MCPConfig: MCPMapping{
			LocalPath:  ".mcp.json",
			GlobalPath: ".mcp.json",
		},
	},
}
