package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewBenchmarkCmd returns the root "benchmark" cobra command with run and report subcommands.
func NewBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run retrieval quality benchmarks",
	}

	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newReportCmd())

	return cmd
}

func newRunCmd() *cobra.Command {
	var scenarioName string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmark scenarios and report results",
		RunE: func(cmd *cobra.Command, args []string) error {
			scenarios := BuiltinScenarios()

			if scenarioName != "" {
				filtered := filterScenarios(scenarios, scenarioName)
				if filtered == nil {
					names := scenarioNames(scenarios)
					fmt.Fprintf(cmd.ErrOrStderr(), "unknown scenario %q\nvalid scenarios:\n", scenarioName)
					for _, n := range names {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", n)
					}
					return fmt.Errorf("unknown scenario: %s", scenarioName)
				}
				scenarios = filtered
			}

			logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelInfo}))
			runner := NewRunner(logger)

			report, err := runner.Run(context.Background(), scenarios)
			if err != nil {
				return fmt.Errorf("benchmark run failed: %w", err)
			}

			if outputFile != "" {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal report: %w", err)
				}
				if err := os.WriteFile(outputFile, data, 0o644); err != nil {
					return fmt.Errorf("write output file: %w", err)
				}
				return nil
			}

			return (&ASCIIRenderer{Writer: cmd.OutOrStdout()}).Render(report)
		},
	}

	cmd.Flags().StringVar(&scenarioName, "scenario", "", "run only the named scenario (case-insensitive)")
	cmd.Flags().StringVar(&outputFile, "output", "", "write JSON report to file instead of ASCII to stdout")

	return cmd
}

func newReportCmd() *cobra.Command {
	var inputFile string
	var format string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a previously saved benchmark report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("--input <file> is required")
			}

			data, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("read input file: %w", err)
			}

			var report BenchmarkReport
			if err := json.Unmarshal(data, &report); err != nil {
				return fmt.Errorf("parse report JSON: %w", err)
			}

			if format == "json" {
				_, err := os.Stdout.Write(data)
				return err
			}

			return (&ASCIIRenderer{Writer: os.Stdout}).Render(&report)
		},
	}

	cmd.Flags().StringVar(&inputFile, "input", "", "JSON report file to read")
	cmd.Flags().StringVar(&format, "format", "ascii", "output format: ascii or json")

	return cmd
}

// filterScenarios returns a single-element slice matching name (case-insensitive), or nil if not found.
func filterScenarios(scenarios []BenchmarkScenario, name string) []BenchmarkScenario {
	lower := strings.ToLower(name)
	for _, s := range scenarios {
		if strings.ToLower(s.Name) == lower {
			return []BenchmarkScenario{s}
		}
	}
	return nil
}

// scenarioNames returns the names of all scenarios.
func scenarioNames(scenarios []BenchmarkScenario) []string {
	names := make([]string, len(scenarios))
	for i, s := range scenarios {
		names[i] = s.Name
	}
	return names
}
