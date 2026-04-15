package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/spf13/cobra"
)

// newAutopilotCmd creates the "mnemos autopilot" parent command.
// mnemos is needed by the run subcommand to call ListDistinctProjectIDs.
func newAutopilotCmd(daemon *autopilot.AutopilotDaemon, mnemos *core.Mnemos) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autopilot",
		Short: "Autopilot daemon control",
		Long:  "Inspect and control the passive autopilot background daemon.",
	}

	cmd.AddCommand(newAutopilotStatusCmd(daemon))
	cmd.AddCommand(newAutopilotRunCmd(daemon, mnemos))
	cmd.AddCommand(newAutopilotReportCmd(mnemos))

	return cmd
}

// newAutopilotStatusCmd prints the current daemon state as plain text.
func newAutopilotStatusCmd(daemon *autopilot.AutopilotDaemon) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show autopilot daemon status",
		Run: func(cmd *cobra.Command, args []string) {
			s := daemon.Status()
			fmt.Printf("enabled:       %v\n", s.Enabled)
			fmt.Printf("last_findings: %d\n", s.LastFindingCount)
			if s.NextRun.IsZero() {
				fmt.Println("next_run:      (not scheduled)")
			} else {
				fmt.Printf("next_run:      %s\n", s.NextRun.Format("2006-01-02T15:04:05Z07:00"))
			}
			if len(s.LastRun) == 0 {
				fmt.Println("last_run:      (none)")
			} else {
				fmt.Println("last_run:")
				for proj, t := range s.LastRun {
					fmt.Printf("  %s: %s\n", proj, t.Format("2006-01-02T15:04:05Z07:00"))
				}
			}
		},
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
