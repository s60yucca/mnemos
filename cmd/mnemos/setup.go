package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/spf13/cobra"
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
	)

	return cmd
}

// newSetupClientCmd creates a setup subcommand for a specific AI client.
func newSetupClientCmd(clientName string) *cobra.Command {
	var force bool
	var global bool

	cmd := &cobra.Command{
		Use:   clientName,
		Short: fmt.Sprintf("Set up Mnemos integration for %s", clientName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(clientName, force, global)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files without prompting")
	cmd.Flags().BoolVar(&global, "global", false, "install to home directory instead of current project")

	return cmd
}

// runSetup performs the setup for a given client.
func runSetup(clientName string, force, global bool) error {
	clientCfg, ok := setup.Clients[clientName]
	if !ok {
		return fmt.Errorf("unknown client %q — supported: claude, kiro, cursor", clientName)
	}

	// Resolve base directory
	baseDir, err := resolveBaseDir(global)
	if err != nil {
		return err
	}

	writer := setup.NewWriter(baseDir, global, force)

	// Ensure .mnemos dir (only for local setup)
	if !global {
		if err := writer.EnsureMnemosDir(); err != nil {
			return fmt.Errorf("ensure .mnemos dir: %w", err)
		}
	}

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

		if _, err := writer.WriteFile(targetPath, content); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
	}

	// Merge MCP config
	mcpPath := clientCfg.MCPConfig.LocalPath
	if global {
		mcpPath = clientCfg.MCPConfig.GlobalPath
	}
	mcpTarget := filepath.Join(baseDir, mcpPath)

	if err := setup.MergeMCPConfig(mcpTarget, "mnemos", setup.MnemosMCPEntry{
		Command: "mnemos",
		Args:    []string{"serve"},
	}); err != nil {
		return fmt.Errorf("merge MCP config: %w", err)
	}

	writer.Report()
	fmt.Printf("MCP config updated: %s\n", mcpTarget)
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
