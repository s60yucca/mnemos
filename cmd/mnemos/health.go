package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/spf13/cobra"
)

// Event represents a parsed feature execution event from the log
type Event struct {
	Timestamp time.Time
	Feature   string
	Attrs     map[string]string
}

// Status represents the health status of a feature
type Status int

const (
	StatusFiring Status = iota
	StatusLow
	StatusNotFiring
	StatusUnknown
)

func (s Status) String() string {
	switch s {
	case StatusFiring:
		return "FIRING NORMALLY"
	case StatusLow:
		return "LOW ACTIVITY"
	case StatusNotFiring:
		return "NOT OBSERVED"
	case StatusUnknown:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

// parseLog reads features.log and returns structured events
func parseLog(logPath string, since time.Time, projectFilter string) ([]Event, error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Empty log is not an error
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse TSV format: timestamp\tfeature\tattributes
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			fmt.Fprintf(os.Stderr, "warning: malformed line %d (expected 3 tab-separated parts): %q\n", lineNum, line)
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid timestamp on line %d: %v\n", lineNum, err)
			continue
		}

		// Filter by time window
		if timestamp.Before(since) {
			continue
		}

		feature := parts[1]
		attrsStr := parts[2]

		// Parse attributes (space-separated key=value pairs)
		attrs := make(map[string]string)
		if attrsStr != "" {
			for _, pair := range strings.Split(attrsStr, " ") {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					attrs[kv[0]] = kv[1]
				}
			}
		}

		// Filter by project if specified. project_id is canonical; project is legacy.
		if projectFilter != "" {
			proj := attrs["project_id"]
			if proj == "" {
				proj = attrs["project"]
			}
			if proj != projectFilter {
				continue
			}
		}

		events = append(events, Event{
			Timestamp: timestamp,
			Feature:   feature,
			Attrs:     attrs,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return events, nil
}

// classifyFeature determines the health status of a feature based on events and baseline
// allEvents should contain ALL events (including MCP calls), featureEvents should contain only the feature's events
func classifyFeature(featureEvents []Event, allEvents []Event, baseline observe.Baseline, activeDays []time.Time) Status {
	featureName := ""
	if len(featureEvents) > 0 {
		featureName = featureEvents[0].Feature
	}
	return classifyFeatureNamed(featureName, featureEvents, allEvents, baseline, activeDays)
}

func classifyFeatureNamed(featureName string, featureEvents []Event, allEvents []Event, baseline observe.Baseline, activeDays []time.Time) Status {
	if len(featureEvents) == 0 {
		if baseline.RatioVsMCPCalls > 0 {
			for _, event := range allEvents {
				if isFeatureDenominator(featureName, event.Feature) {
					return StatusNotFiring
				}
			}
			return StatusUnknown
		}
		return StatusNotFiring
	}

	if len(activeDays) == 0 {
		// The feature fired, but there is not enough MCP activity to compare against baselines.
		return StatusLow
	}

	// Group feature events by day
	eventsByDay := make(map[string]int)
	for _, event := range featureEvents {
		day := event.Timestamp.Format("2006-01-02")
		eventsByDay[day]++
	}

	// Count eligible denominator events per day for ratio-based features.
	mcpCallsByDay := make(map[string]int)
	for _, event := range allEvents {
		if isFeatureDenominator(featureName, event.Feature) {
			day := event.Timestamp.Format("2006-01-02")
			mcpCallsByDay[day]++
		}
	}

	// Classify based on baseline type
	if baseline.RatioVsMCPCalls > 0 {
		// Ratio-based feature (per-call)
		daysAboveThreshold := 0
		daysWithData := 0

		for _, activeDay := range activeDays {
			dayStr := activeDay.Format("2006-01-02")
			mcpCalls := mcpCallsByDay[dayStr]
			if mcpCalls == 0 {
				continue // Skip days with no MCP calls
			}
			daysWithData++

			featureEvents := eventsByDay[dayStr]
			ratio := float64(featureEvents) / float64(mcpCalls)
			if ratio >= baseline.RatioVsMCPCalls {
				daysAboveThreshold++
			}
		}

		if daysWithData == 0 {
			return StatusUnknown
		}

		percentAbove := float64(daysAboveThreshold) / float64(daysWithData)
		if percentAbove >= 0.5 {
			return StatusFiring
		}
		return StatusLow
	}

	// MinDaily-based feature (always-on)
	daysAboveThreshold := 0
	for _, activeDay := range activeDays {
		dayStr := activeDay.Format("2006-01-02")
		if eventsByDay[dayStr] >= baseline.MinDaily {
			daysAboveThreshold++
		}
	}

	percentAbove := float64(daysAboveThreshold) / float64(len(activeDays))
	if percentAbove >= 0.5 {
		return StatusFiring
	}
	return StatusLow
}

func isFeatureDenominator(featureName, eventName string) bool {
	switch featureName {
	case "quality_gate", "dedup", "summarize", "file_link":
		return eventName == "store_call"
	case "mmr":
		return eventName == "context_call"
	case "auto_inject":
		return eventName == "auto_inject_attempt"
	default:
		return eventName == "store_call" || eventName == "context_call" || eventName == "search_call"
	}
}

// detectActiveDays identifies days with ≥ ActiveDayThreshold MCP calls
func detectActiveDays(events []Event) []time.Time {
	mcpCallsByDay := make(map[string]int)
	for _, event := range events {
		if event.Feature == "store_call" || event.Feature == "context_call" || event.Feature == "search_call" {
			day := event.Timestamp.Format("2006-01-02")
			mcpCallsByDay[day]++
		}
	}

	var activeDays []time.Time
	for dayStr, count := range mcpCallsByDay {
		if count >= observe.ActiveDayThreshold {
			day, _ := time.Parse("2006-01-02", dayStr)
			activeDays = append(activeDays, day)
		}
	}

	// Sort active days
	for i := 0; i < len(activeDays)-1; i++ {
		for j := i + 1; j < len(activeDays); j++ {
			if activeDays[i].After(activeDays[j]) {
				activeDays[i], activeDays[j] = activeDays[j], activeDays[i]
			}
		}
	}

	return activeDays
}

// renderReport generates human-readable output for health status
func renderReport(classifications map[string]Status, events []Event, window int) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("Feature Health Report (last %d days)\n", window))
	output.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Group features by status
	firing := []string{}
	low := []string{}
	notObserved := []string{}
	unknown := []string{}

	for feature, status := range classifications {
		switch status {
		case StatusFiring:
			firing = append(firing, feature)
		case StatusLow:
			low = append(low, feature)
		case StatusNotFiring:
			notObserved = append(notObserved, feature)
		case StatusUnknown:
			unknown = append(unknown, feature)
		}
	}

	if len(unknown) > 0 {
		output.WriteString("? UNKNOWN (" + fmt.Sprintf("%d", len(unknown)) + ")\n")
		for _, feature := range unknown {
			baseline := observe.Baselines[feature]
			output.WriteString(fmt.Sprintf("  • %s: %s\n", feature, baseline.Expected))
		}
		output.WriteString("\n")
	}

	// Display FIRING NORMALLY
	if len(firing) > 0 {
		output.WriteString("✓ FIRING NORMALLY (" + fmt.Sprintf("%d", len(firing)) + ")\n")
		for _, feature := range firing {
			baseline := observe.Baselines[feature]
			output.WriteString(fmt.Sprintf("  • %s: %s\n", feature, baseline.Expected))
		}
		output.WriteString("\n")
	}

	// Display LOW ACTIVITY
	if len(low) > 0 {
		output.WriteString("⚠ LOW ACTIVITY (" + fmt.Sprintf("%d", len(low)) + ")\n")
		for _, feature := range low {
			baseline := observe.Baselines[feature]
			output.WriteString(fmt.Sprintf("  • %s: %s\n", feature, baseline.Expected))
			if lastSeen := lastSeenForFeature(events, feature); !lastSeen.IsZero() {
				output.WriteString(fmt.Sprintf("    Last seen: %s (local)\n", lastSeen.Local().Format("2006-01-02 15:04:05")))
			}
		}
		output.WriteString("\n")
	}

	// Display NOT OBSERVED
	if len(notObserved) > 0 {
		output.WriteString("✗ NOT OBSERVED (" + fmt.Sprintf("%d", len(notObserved)) + ")\n")
		for _, feature := range notObserved {
			baseline := observe.Baselines[feature]
			output.WriteString(fmt.Sprintf("  • %s: %s\n", feature, baseline.Expected))
			if lastSeen := lastSeenForFeature(events, feature); !lastSeen.IsZero() {
				output.WriteString(fmt.Sprintf("    Last seen: %s (local)\n", lastSeen.Local().Format("2006-01-02 15:04:05")))
			}
		}
		output.WriteString("\n")
	}

	return output.String()
}

func lastSeenForFeature(events []Event, feature string) time.Time {
	var lastSeen time.Time
	for _, event := range events {
		if event.Feature == feature && event.Timestamp.After(lastSeen) {
			lastSeen = event.Timestamp
		}
	}
	return lastSeen
}

// renderFeatureDetail generates deep-dive output for a specific feature
func renderFeatureDetail(events []Event, featureName string) string {
	var output strings.Builder

	// Filter events for this feature
	var featureEvents []Event
	for _, e := range events {
		if e.Feature == featureName {
			featureEvents = append(featureEvents, e)
		}
	}

	if len(featureEvents) == 0 {
		return fmt.Sprintf("No events found for feature: %s\n", featureName)
	}

	output.WriteString(fmt.Sprintf("Feature Detail: %s\n", featureName))
	output.WriteString(strings.Repeat("=", 50) + "\n\n")
	output.WriteString(fmt.Sprintf("Total events: %d\n\n", len(featureEvents)))

	// Group by day
	eventsByDay := make(map[string]int)
	for _, event := range featureEvents {
		day := event.Timestamp.Format("2006-01-02")
		eventsByDay[day]++
	}

	// Show daily trend (simple ASCII bar chart)
	output.WriteString("Daily trend:\n")
	for day, count := range eventsByDay {
		bar := strings.Repeat("█", count/5) // Scale down for display
		if count > 0 && len(bar) == 0 {
			bar = "▌"
		}
		output.WriteString(fmt.Sprintf("  %s: %s (%d)\n", day, bar, count))
	}
	output.WriteString("\n")

	// Show recent samples (last 5)
	output.WriteString("Recent events:\n")
	start := len(featureEvents) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(featureEvents); i++ {
		event := featureEvents[i]
		output.WriteString(fmt.Sprintf("  %s: %v\n",
			event.Timestamp.Local().Format("2006-01-02 15:04:05"),
			event.Attrs))
	}

	return output.String()
}

// newHealthCmd creates the health command
func newHealthCmd(dataDir string) *cobra.Command {
	var (
		project string
		days    int
		feature string
		since   string
	)

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show feature health status",
		Long: `Analyze feature execution logs and report health status.

All times are interpreted as UTC; output is displayed in local timezone.
The --days flag uses a rolling 24-hour window (N × 24 hours ending at current time), not calendar days.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine log path
			logPath := filepath.Join(dataDir, "logs", "features.log")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				fmt.Println("No feature activity recorded. Have you run mnemos recently?")
				return nil
			}

			// Determine time window
			var sinceTime time.Time
			if since != "" {
				// Parse --since flag as UTC midnight
				parsed, parseErr := time.Parse("2006-01-02", since)
				if parseErr != nil {
					return fmt.Errorf("invalid --since format (expected YYYY-MM-DD): %w", parseErr)
				}
				sinceTime = parsed.UTC()
			} else {
				// Use --days flag (rolling window)
				sinceTime = time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
			}

			// Parse log
			events, err := parseLog(logPath, sinceTime, project)
			if err != nil {
				return err
			}

			if len(events) == 0 {
				fmt.Println("No feature activity in the specified time window.")
				return nil
			}

			// Detect active days
			activeDays := detectActiveDays(events)

			// If feature filter is specified, show detail view
			if feature != "" {
				fmt.Print(renderFeatureDetail(events, feature))
				return nil
			}

			// Classify all features
			classifications := make(map[string]Status)
			for featureName, baseline := range observe.Baselines {
				// Filter events for this feature
				var featureEvents []Event
				for _, e := range events {
					if e.Feature == featureName &&
						(featureName != "auto_inject" || (e.Attrs["outcome"] == "ok" && e.Attrs["payload"] == "true")) {
						featureEvents = append(featureEvents, e)
					}
				}

				status := classifyFeatureNamed(featureName, featureEvents, events, baseline, activeDays)
				classifications[featureName] = status
			}

			// Render report
			fmt.Print(renderReport(classifications, events, days))

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Filter to events for one project")
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to analyze (rolling 24-hour window)")
	cmd.Flags().StringVar(&feature, "feature", "", "Deep-dive into one feature")
	cmd.Flags().StringVar(&since, "since", "", "Specific start date (YYYY-MM-DD, UTC)")

	return cmd
}
