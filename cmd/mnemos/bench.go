package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/spf13/cobra"
)

// newBenchCmd creates the bench command with subcommands
func newBenchCmd(dataDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark mode management and session export",
		Long:  `Manage benchmark mode (ON/OFF) and export session data for dogfooding analysis.`,
	}

	cmd.AddCommand(newBenchModeCmd(dataDir))
	cmd.AddCommand(newBenchExportCmd(dataDir))
	cmd.AddCommand(newBenchStatusCmd(dataDir))
	cmd.AddCommand(newBenchSessionStartCmd(dataDir))
	cmd.AddCommand(newBenchSessionEndCmd(dataDir))

	return cmd
}

// newBenchModeCmd creates the "mnemos bench mode" command
func newBenchModeCmd(dataDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode <on|off>",
		Short: "Set benchmark mode (ON or OFF)",
		Long: `Set benchmark mode to ON or OFF.

When mode is OFF, mnemos_context and mnemos_search return empty results,
simulating mnemos not being installed. Memories are still stored with
"bench_off_day" tag to prevent data loss.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modeStr := strings.ToLower(args[0])

			var mode benchmark.BenchMode
			switch modeStr {
			case "on":
				mode = benchmark.BenchModeOn
			case "off":
				mode = benchmark.BenchModeOff
			default:
				return fmt.Errorf("invalid mode: %s (must be 'on' or 'off')", modeStr)
			}

			// Read old mode
			oldMode, _ := benchmark.ReadBenchMode(dataDir)

			// Write new mode
			if err := benchmark.WriteBenchMode(dataDir, mode); err != nil {
				return fmt.Errorf("failed to write bench mode: %w", err)
			}

			// Emit mode change event
			observe.Feature("bench_mode_change", map[string]any{
				"old_mode": string(oldMode),
				"new_mode": string(mode),
			})

			fmt.Printf("Benchmark mode set to: %s\n", mode)
			return nil
		},
	}

	return cmd
}

// newBenchExportCmd creates the "mnemos bench export" command
func newBenchExportCmd(dataDir string) *cobra.Command {
	var (
		since        string
		project      string
		mode         string
		output       string
		includeMixed bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export benchmark sessions as CSV",
		Long: `Export benchmark session data from features.log as CSV.

The CSV includes: session_id, timestamp_start, timestamp_end, project_id, mode,
duration_ms, tokens_in, tokens_out, mcp_calls_count, task_completed, task_category.

Mixed-mode sessions (mode changed mid-session) are excluded by default.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine log path
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			logPath := filepath.Join(home, ".mnemos", "logs", "features.log")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				fmt.Println("No feature activity recorded. Have you run mnemos recently?")
				return nil
			}

			// Parse time filter
			var sinceTime time.Time
			if since != "" {
				sinceTime, err = time.Parse("2006-01-02", since)
				if err != nil {
					return fmt.Errorf("invalid --since format (expected YYYY-MM-DD): %w", err)
				}
				sinceTime = sinceTime.UTC()
			}

			// Parse log and extract sessions
			sessions, err := extractSessions(logPath, sinceTime, project, mode, includeMixed)
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found matching filters.")
				return nil
			}

			// Write CSV
			writer := csv.NewWriter(os.Stdout)
			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer f.Close()
				writer = csv.NewWriter(f)
			}

			// Write header
			writer.Write([]string{
				"session_id", "timestamp_start", "timestamp_end", "project_id", "mode",
				"duration_ms", "tokens_in", "tokens_out", "mcp_calls_count",
				"task_completed", "task_category", "provenance",
			})

			// Write rows
			for _, session := range sessions {
				writer.Write([]string{
					session.SessionID,
					session.TimestampStart,
					session.TimestampEnd,
					session.ProjectID,
					session.Mode,
					fmt.Sprintf("%d", session.DurationMS),
					fmt.Sprintf("%d", session.TokensIn),
					fmt.Sprintf("%d", session.TokensOut),
					fmt.Sprintf("%d", session.MCPCallsCount),
					fmt.Sprintf("%t", session.TaskCompleted),
					session.TaskCategory,
					session.Provenance,
				})
			}

			writer.Flush()
			if err := writer.Error(); err != nil {
				return fmt.Errorf("failed to write CSV: %w", err)
			}

			if output != "" {
				fmt.Printf("Exported %d sessions to %s\n", len(sessions), output)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Filter sessions after date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project ID")
	cmd.Flags().StringVar(&mode, "mode", "", "Filter by mode (on|off)")
	cmd.Flags().StringVar(&output, "output", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&includeMixed, "include-mixed", false, "Include mixed-mode sessions")

	return cmd
}

// newBenchStatusCmd creates the "mnemos bench status" command
func newBenchStatusCmd(dataDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current benchmark status",
		Long:  `Show current benchmark mode and session statistics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read current mode
			mode, err := benchmark.ReadBenchMode(dataDir)
			if err != nil {
				mode = benchmark.BenchModeOn // Default
			}

			fmt.Printf("Current benchmark mode: %s\n\n", strings.ToUpper(string(mode)))

			// Get log path
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			logPath := filepath.Join(home, ".mnemos", "logs", "features.log")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				fmt.Println("No session data available.")
				return nil
			}

			// Parse sessions from last 7 days
			since := time.Now().UTC().Add(-7 * 24 * time.Hour)
			sessions, err := extractSessions(logPath, since, "", "", false)
			if err != nil {
				return err
			}

			// Count sessions by mode
			onCount := 0
			offCount := 0
			for _, session := range sessions {
				if session.Mode == "on" {
					onCount++
				} else if session.Mode == "off" {
					offCount++
				}
			}

			fmt.Printf("Sessions in last 7 days:\n")
			fmt.Printf("  ON:  %d\n", onCount)
			fmt.Printf("  OFF: %d\n", offCount)
			fmt.Printf("  Total: %d\n", len(sessions))

			return nil
		},
	}

	return cmd
}

// newBenchSessionStartCmd creates the "mnemos bench session-start" command
func newBenchSessionStartCmd(dataDir string) *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "session-start",
		Short: "Manually start a benchmark session",
		Long: `Manually start a benchmark session with optional category.

This is typically not needed as sessions start automatically on first MCP call.
Use this if you want to explicitly mark session boundaries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Note: This command is a placeholder for manual session start
			// The actual session tracking happens in the MCP server
			// We just emit an event here for manual tracking

			sessionID := fmt.Sprintf("manual-%d", time.Now().Unix())

			observe.Feature("bench_session_start", map[string]any{
				"session_id": sessionID,
				"project_id": "manual",
				"mode":       "unknown",
				"category":   category,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"provenance": "production",
			})

			fmt.Printf("Session started: %s\n", sessionID)
			fmt.Println("Note: Automatic session tracking via MCP server is recommended.")

			return nil
		},
	}

	cmd.Flags().StringVar(&category, "category", "other", "Task category (refactor|feature|debug|docs|other)")

	return cmd
}

// newBenchSessionEndCmd creates the "mnemos bench session-end" command
func newBenchSessionEndCmd(dataDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session-end",
		Short: "Manually end the current benchmark session",
		Long: `Manually end the current benchmark session.

This is the recommended way to close sessions for accurate task boundaries.
The 10-minute inactivity timeout will auto-close if you forget.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Note: This command is a placeholder for manual session end
			// The actual session tracking happens in the MCP server
			// We just emit an event here for manual tracking

			observe.Feature("bench_session_end", map[string]any{
				"session_id":      "manual",
				"duration_ms":     0,
				"tokens_in":       0,
				"tokens_out":      0,
				"mcp_calls_count": 0,
				"task_completed":  true,
			})

			fmt.Println("Session end signal sent.")
			fmt.Println("Note: Automatic session tracking via MCP server is recommended.")

			return nil
		},
	}

	return cmd
}

// SessionRecord represents a parsed session from the log
type SessionRecord struct {
	SessionID      string
	TimestampStart string
	TimestampEnd   string
	ProjectID      string
	Mode           string
	DurationMS     int64
	TokensIn       int
	TokensOut      int
	MCPCallsCount  int
	TaskCompleted  bool
	TaskCategory   string
	Provenance     string
}

// extractSessions parses features.log and extracts session records
func extractSessions(logPath string, since time.Time, projectFilter string, modeFilter string, includeMixed bool) ([]SessionRecord, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	// Parse events
	sessionStarts := make(map[string]map[string]string) // session_id -> attrs
	var sessions []SessionRecord

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse TSV format: timestamp\tfeature\tattributes
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}

		if !since.IsZero() && timestamp.Before(since) {
			continue
		}

		feature := parts[1]
		attrsStr := parts[2]

		// Parse attributes
		attrs := make(map[string]string)
		if attrsStr != "" {
			for _, pair := range strings.Split(attrsStr, " ") {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					attrs[kv[0]] = kv[1]
				}
			}
		}

		// Track session starts
		if feature == "bench_session_start" {
			sessionID := attrs["session_id"]
			if sessionID != "" {
				sessionStarts[sessionID] = attrs
			}
		}

		// Match session ends with starts
		if feature == "bench_session_end" {
			sessionID := attrs["session_id"]
			if sessionID == "" {
				continue
			}

			startAttrs, ok := sessionStarts[sessionID]
			if !ok {
				continue // No matching start
			}

			// Build session record
			session := SessionRecord{
				SessionID:      sessionID,
				TimestampStart: startAttrs["timestamp"],
				TimestampEnd:   timestamp.Format(time.RFC3339),
				ProjectID:      startAttrs["project_id"],
				Mode:           startAttrs["mode"],
				TaskCategory:   startAttrs["category"],
				Provenance:     startAttrs["provenance"],
			}

			// Parse numeric fields
			fmt.Sscanf(attrs["duration_ms"], "%d", &session.DurationMS)
			fmt.Sscanf(attrs["tokens_in"], "%d", &session.TokensIn)
			fmt.Sscanf(attrs["tokens_out"], "%d", &session.TokensOut)
			fmt.Sscanf(attrs["mcp_calls_count"], "%d", &session.MCPCallsCount)
			session.TaskCompleted = attrs["task_completed"] == "true"

			// Apply filters
			if projectFilter != "" && session.ProjectID != projectFilter {
				continue
			}
			if modeFilter != "" && session.Mode != modeFilter {
				continue
			}

			// Skip mixed-mode sessions unless explicitly included
			if !includeMixed && session.Mode == "mode_mixed" {
				continue
			}

			sessions = append(sessions, session)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return sessions, nil
}
