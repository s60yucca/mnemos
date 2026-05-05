package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/spf13/cobra"
)

// newAutopilotCmd creates the "mnemos autopilot" parent command.
// mnemos is needed by the run subcommand to call ListDistinctProjectIDs.
func newAutopilotCmd(daemon *autopilot.AutopilotDaemon, mnemos *core.Mnemos, dataDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autopilot",
		Short: "Autopilot daemon control",
		Long:  "Inspect and control the passive autopilot background daemon.",
	}

	cmd.AddCommand(newAutopilotStatusCmd(daemon, dataDir))
	cmd.AddCommand(newAutopilotRunCmd(daemon, mnemos))
	cmd.AddCommand(newAutopilotReportCmd(mnemos))

	return cmd
}

// newAutopilotStatusCmd prints the current daemon state as plain text.
// It reads the persisted state file written by the running daemon process,
// so the output reflects the actual running daemon rather than a fresh process.
func newAutopilotStatusCmd(daemon *autopilot.AutopilotDaemon, dataDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show autopilot daemon status",
		Run: func(cmd *cobra.Command, args []string) {
			// Try to read persisted state from the running daemon first.
			// Fall back to in-process state (always shows "not scheduled") only
			// if the state file doesn't exist yet (daemon has never completed a cycle).
			persisted, err := autopilot.ReadStateFile(dataDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read state file: %v\n", err)
			}

			if persisted != nil {
				// Show persisted state from the running daemon process.
				printDaemonStatus(persisted, true)
			} else {
				// No state file yet — daemon has never completed a cycle.
				s := daemon.Status()
				printDaemonStatus(&s, false)
				fmt.Println()
				fmt.Println("note: daemon runs inside 'mnemos serve' (spawned by your MCP client).")
				fmt.Println("      state file will appear after the first autopilot cycle completes.")
			}
		},
	}
}

func printDaemonStatus(s *autopilot.DaemonStatus, fromFile bool) {
	fmt.Printf("enabled:       %v\n", s.Enabled)

	// Display auto-inject config
	autoInjectCfg := hook.AutoInjectConfigFromEnv()
	fmt.Println("\nAuto-Inject Configuration:")
	fmt.Printf("  Enabled:        %v\n", autoInjectCfg.Enabled)
	fmt.Printf("  Budget:         %d tokens\n", autoInjectCfg.Budget)
	fmt.Printf("  Max Count:      %d memories\n", autoInjectCfg.MaxCount)
	fmt.Printf("  Summary Length: %d chars\n", autoInjectCfg.SummaryLength)
	fmt.Println()

	fmt.Printf("last_findings: %d\n", s.LastFindingCount)
	if s.NextRun.IsZero() {
		fmt.Println("next_run:      (not scheduled)")
	} else {
		// If next_run is in the past, the daemon is likely between cycles.
		if time.Now().After(s.NextRun) {
			fmt.Printf("next_run:      %s (overdue — cycle may be running)\n",
				s.NextRun.Format("2006-01-02T15:04:05Z07:00"))
		} else {
			fmt.Printf("next_run:      %s\n", s.NextRun.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	if len(s.LastRun) == 0 {
		fmt.Println("last_run:      (none)")
	} else {
		fmt.Println("last_run:")
		for proj, t := range s.LastRun {
			fmt.Printf("  %s: %s\n", proj, t.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	if fromFile && !s.UpdatedAt.IsZero() {
		fmt.Printf("state_updated: %s (pid %d)\n",
			s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), s.PID)
	}
}

// newAutopilotRunCmd triggers an immediate daemon run, optionally dry-run or scoped to a project.
func newAutopilotRunCmd(daemon *autopilot.AutopilotDaemon, mnemos *core.Mnemos) *cobra.Command {
	var dryRun bool
	var projectID string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Trigger an immediate autopilot run",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			prefix := ""
			if dryRun {
				prefix = "[DRY RUN] "
			}

			var projects []string
			if projectID != "" {
				projects = []string{projectID}
			} else {
				var err error
				projects, err = mnemos.ListDistinctProjectIDs(ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error listing projects: %v\n", err)
					return err
				}
			}

			if len(projects) == 0 {
				fmt.Println(prefix + "No active projects found.")
				return nil
			}

			for _, proj := range projects {
				findings, err := daemon.RunOnce(ctx, proj, dryRun)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%serror running autopilot for project %q: %v\n", prefix, proj, err)
					continue
				}
				fmt.Printf("%sproject %q: %d finding(s)\n", prefix, proj, len(findings))
				for _, f := range findings {
					fmt.Printf("%s  - %s\n", prefix, f.Type)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "run detectors without writing reports or creating relations")
	cmd.Flags().StringVar(&projectID, "project", "", "run only for this project ID")

	return cmd
}

// newAutopilotReportCmd prints the latest autopilot report for a project.
func newAutopilotReportCmd(mnemos *core.Mnemos) *cobra.Command {
	var projectID string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Print the latest autopilot report",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			report, err := mnemos.GetLatestAutopilotReport(ctx, projectID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error fetching report: %v\n", err)
				return err
			}
			if report == nil {
				fmt.Println("No autopilot report found.")
				return nil
			}
			fmt.Println(report.Content)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID to fetch report for")

	return cmd
}
