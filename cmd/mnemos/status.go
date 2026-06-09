package main

import (
	"fmt"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/spf13/cobra"
)

func newStatusCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Mnemos knowledge-loop status",
		Run: func(cmd *cobra.Command, args []string) {
			autoInject := hook.AutoInjectConfigFromEnv()
			fmt.Fprintf(cmd.OutOrStdout(), "data_dir: %s\n", cfg.DataDir)
			fmt.Fprintf(cmd.OutOrStdout(), "auto_compile: %t\n", cfg.Autopilot.AutoCompileEnabled)
			fmt.Fprintf(cmd.OutOrStdout(), "hooks_enabled: %t\n", cfg.Hook.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "auto_inject: %t\n", cfg.Hook.Enabled && autoInject.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "min_auto_compile_sources: %d\n", cfg.Autopilot.MinAutoCompileSources)
		},
	}
}
