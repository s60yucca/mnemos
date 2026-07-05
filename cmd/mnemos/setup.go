package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newSetupCmd creates the "mnemos setup" parent command with client subcommands.
func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up Mnemos integration for an AI client",
		Long:  "Inject steering files, hook config, and MCP config for Claude, Kiro, or Cursor.",
	}

	cmd.AddCommand(
		newSetupClientCmd("claude"),
		newSetupClientCmd("kiro"),
		newSetupClientCmd("cursor"),
		newSetupClientCmd("gemini-cli"),
		newSetupClientCmd("codex"),
		newSetupClientCmd("trae"),
	)

	return cmd
}

// newSetupClientCmd creates a setup subcommand for a specific AI client.
func newSetupClientCmd(clientName string) *cobra.Command {
	var force bool
	var global bool
	var local bool
	var uninstall bool

	cmd := &cobra.Command{
		Use:   clientName,
		Short: fmt.Sprintf("Set up Mnemos integration for %s", clientName),
		RunE: func(cmd *cobra.Command, args []string) error {
			isGlobal := true
			if clientName != "codex" {
				var err error
				isGlobal, err = resolveScope(global, local)
				if err != nil {
					return err
				}
			}
			if uninstall {
				return runUninstall(clientName, isGlobal)
			}
			projectID, err := cmd.Flags().GetString("project")
			if err != nil {
				return err
			}
			return runSetup(clientName, force, isGlobal, projectID)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files without prompting")
	cmd.Flags().BoolVar(&global, "global", false, "install to home directory instead of current project")
	cmd.Flags().BoolVar(&local, "local", false, "install to current project (explicit)")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove all mnemos setup artifacts")

	return cmd
}

// resolveScope determines whether to install globally or locally.
// Returns an error if both --global and --local are set (mutually exclusive).
// If neither flag is set, checks if stdout is a TTY and prompts the user;
// in non-TTY environments (CI, scripts), defaults to local scope silently.
func resolveScope(globalFlag, localFlag bool) (bool, error) {
	if globalFlag && localFlag {
		return false, fmt.Errorf("--global and --local are mutually exclusive")
	}
	if globalFlag {
		return true, nil
	}
	if localFlag {
		return false, nil
	}
	// Neither flag: check TTY
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Print("Install globally (all projects) or locally (this project only)? [G/l]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		response = strings.TrimSpace(strings.ToLower(response))
		return response != "l" && response != "local", nil
	}
	return false, nil // non-TTY: default local, no output
}

// runUninstall removes all mnemos setup artifacts for the given client.
func runUninstall(clientName string, global bool) error {
	if clientName != "claude" {
		return fmt.Errorf("--uninstall is currently only supported for the claude client")
	}

	// Use global command name instead of absolute path
	binPath := "mnemos"

	if global {
		if err := setup.UninstallClaudeGlobal(binPath); err != nil {
			return fmt.Errorf("uninstall global: %w", err)
		}
		fmt.Println("✅ Global mnemos setup artifacts removed.")
	} else {
		if err := setup.UninstallClaudeLocal(); err != nil {
			return fmt.Errorf("uninstall local: %w", err)
		}
		fmt.Println("✅ Local mnemos setup artifacts removed.")
	}
	return nil
}

// runSetup performs the setup for a given client.
func runSetup(clientName string, force, global bool, projectIDOverride string) error {
	clientCfg, ok := setup.Clients[clientName]
	if !ok {
		return fmt.Errorf("unknown client %q — supported: claude, kiro, cursor, gemini-cli, codex, trae", clientName)
	}

	// Use global command name instead of absolute path.
	// This ensures the config continues working after package manager updates (e.g., brew).
	binPath := "mnemos"

	// Ensure global config exists (idempotent)
	if _, err := setup.EnsureGlobalConfig(); err != nil {
		return fmt.Errorf("ensure global config: %w", err)
	}
	projectID := ""
	if projectIDOverride != "" {
		var err error
		projectID, err = setup.DeriveProjectID(projectIDOverride)
		if err != nil {
			return fmt.Errorf("derive project id: %w", err)
		}
	}

	// Resolve base directory
	baseDir, err := resolveBaseDir(global)
	if err != nil {
		return err
	}

	// Capture cwd for duplicate MCP warning check
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working dir: %w", err)
	}

	writer := setup.NewWriter(baseDir, global, force)

	// Write template files
	for _, fm := range clientCfg.Files {
		content, err := setup.GetTemplate(fm.TemplatePath)
		if err != nil {
			return fmt.Errorf("load template %s: %w", fm.TemplatePath, err)
		}

		destPath := fm.LocalPath
		if global {
			destPath = fm.GlobalPath
		}
		targetPath := filepath.Join(baseDir, destPath)

		if err := writer.EnsureDir(filepath.Dir(targetPath)); err != nil {
			return fmt.Errorf("create dir for %s: %w", targetPath, err)
		}

		if fm.MergeMode {
			if _, err := writer.MergeMarkdownFile(targetPath, content, fm.MergeMarker); err != nil {
				return fmt.Errorf("merge %s: %w", targetPath, err)
			}
		} else if fm.MergeJSON {
			if err := setup.MergeClaudeSettings(targetPath, binPath, projectID); err != nil {
				return fmt.Errorf("merge %s: %w", targetPath, err)
			}
		} else {
			if _, err := writer.WriteFile(targetPath, content); err != nil {
				return fmt.Errorf("write %s: %w", targetPath, err)
			}
		}
	}

	// Merge MCP config — codex uses TOML format
	if clientName == "codex" {
		// Codex only supports global configuration
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home dir: %w", err)
		}

		configPath := filepath.Join(home, ".codex", "config.toml")
		changed, err := setup.MergeCodexTOML(configPath, binPath, projectIDOverride, force)
		if err != nil {
			return fmt.Errorf("merge codex config: %w", err)
		}

		if !changed {
			fmt.Printf("mnemos MCP server already configured in %s (use --force to overwrite)\n", configPath)
			return nil
		}

		// Check if file was modified (simple heuristic: if file exists and no error, assume success)
		// For skip detection, we'd need MergeCodexTOML to return a different signal
		// For now, always show success message
		fmt.Printf("✓ Configured mnemos MCP server in %s\n", configPath)
		fmt.Println("  Project scope: detected at runtime from each workspace")
		fmt.Printf("  Command: %s serve\n", binPath)
		fmt.Println()
		fmt.Println("  Restart Codex CLI and VSCode extension to activate.")
		return nil
	}

	// Trae uses app-managed global config paths; local config is .trae/mcp.json.
	if clientName == "trae" && global {
		paths, err := setup.MergeTraeGlobalMCP(binPath)
		if err != nil {
			return fmt.Errorf("merge trae global MCP: %w", err)
		}
		if len(paths) == 0 {
			return fmt.Errorf("could not locate Trae config directory; use local setup instead (creates .trae/mcp.json)")
		}
		fmt.Println("✓ Configured mnemos MCP server for Trae:")
		for _, p := range paths {
			fmt.Printf("  - %s\n", p)
		}
		fmt.Println()
		fmt.Println("Restart Trae SOLO to activate.")
		return nil
	}

	// Merge MCP config — claude global uses a different strategy
	if clientName == "claude" && global {
		// Use MergeClaudeGlobalMCP instead of generic MergeMCPConfig
		if err := setup.MergeClaudeGlobalMCP(binPath, projectID); err != nil {
			return fmt.Errorf("merge global MCP: %w", err)
		}

		// Install launchd plist on darwin (no-op on other platforms)
		if err := setup.InstallLaunchdPlist(binPath); err != nil {
			return fmt.Errorf("install launchd plist: %w", err)
		}

		// Check for duplicate MCP warning: project-scope .mcp.json takes precedence in Claude Code
		localMCP := filepath.Join(cwd, ".mcp.json")
		if localHasMnemos(localMCP) {
			fmt.Println("Warning: mnemos MCP entry found in both .mcp.json and ~/.claude.json. " +
				"Project-scope takes precedence in Claude Code.")
		}

		writer.Report()
		fmt.Println("Global MCP config updated: ~/.claude.json")
	} else {
		// Use generic MergeMCPConfig for local or non-claude clients
		mcpPath := clientCfg.MCPConfig.LocalPath
		if global {
			mcpPath = clientCfg.MCPConfig.GlobalPath
		}
		mcpTarget := filepath.Join(baseDir, mcpPath)

		if err := setup.MergeMCPConfig(mcpTarget, "mnemos", setup.MnemosMCPEntry{
			Command: binPath,
			Args:    []string{"serve"},
		}); err != nil {
			return fmt.Errorf("merge MCP config: %w", err)
		}
		if projectID != "" {
			if err := setup.MergeMCPEnv(mcpTarget, "mnemos", map[string]string{
				"MNEMOS_PROJECT_ID": projectID,
			}); err != nil {
				return fmt.Errorf("merge MCP env: %w", err)
			}
		}
		if clientName == "trae" {
			_ = setup.MergeMCPEnv(mcpTarget, "mnemos", map[string]string{
				"MNEMOS_CLIENT": "trae-solo",
			})
			paths, err := setup.MergeTraeGlobalMCP(binPath)
			if err == nil && len(paths) > 0 {
				fmt.Println("✓ Trae SOLO detected — global MCP config updated:")
				for _, p := range paths {
					fmt.Printf("  - %s\n", p)
				}
			}
		}

		// Check for duplicate MCP warning: if installing locally, warn if ~/.claude.json also has mnemos
		if clientName == "claude" && !global {
			home, _ := os.UserHomeDir()
			if home != "" {
				globalMCP := filepath.Join(home, ".claude.json")
				if localHasMnemos(globalMCP) {
					fmt.Println("Warning: mnemos MCP entry found in both .mcp.json and ~/.claude.json. " +
						"Project-scope takes precedence in Claude Code.")
				}
			}
		}

		writer.Report()
		fmt.Printf("MCP config updated: %s\n", mcpTarget)
	}

	if clientName == "gemini-cli" {
		fmt.Println("✅ Gemini CLI configured. Run `gemini` in this directory to verify mnemos is connected.")
	}
	return nil
}

// resolveBaseDir returns the base directory for setup.
// For global setup, returns the user's home directory.
// For local setup, returns the current working directory.
func resolveBaseDir(global bool) (string, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return home, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working dir: %w", err)
	}
	return cwd, nil
}

// localHasMnemos checks whether a JSON file at filePath contains a "mnemos" key
// under "mcpServers". Returns false on any error (file absent, parse failure, etc.).
func localHasMnemos(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return false
	}
	rawServers, ok := root["mcpServers"]
	if !ok {
		return false
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		return false
	}
	_, ok = servers["mnemos"]
	return ok
}
