package benchmark

import (
	"fmt"
	"io"
	"strings"
)

const boxWidth = 72 // total width including │ borders

// ASCIIRenderer renders a BenchmarkReport as a box-drawing ASCII table.
type ASCIIRenderer struct {
	Writer io.Writer
}

// Render writes the benchmark report as a box-drawing table to r.Writer.
func (r *ASCIIRenderer) Render(report *BenchmarkReport) error {
	inner := boxWidth - 2 // width between the │ borders

	top := "┌" + strings.Repeat("─", inner) + "┐"
	bot := "└" + strings.Repeat("─", inner) + "┘"

	line := func(content string) string {
		// pad or truncate content to inner width
		runes := []rune(content)
		if len(runes) > inner {
			runes = runes[:inner]
		}
		padded := string(runes) + strings.Repeat(" ", inner-len(runes))
		return "│" + padded + "│"
	}

	blank := line("")

	lines := []string{
		top,
		line("  mnemos Retrieval Quality Benchmark"),
		blank,
		line("  Precision@K and F1 across session scenarios"),
		blank,
	}

	for i, result := range report.Results {
		sc := result.Scenario
		sessions := sc.Sessions

		// Line 1: scenario name + session count
		lines = append(lines, line(fmt.Sprintf("  %s (%d sessions)", sc.Name, sessions)))

		// Line 2: F1 + optional steady-state
		steady := ""
		if result.SteadyStateSession != -1 {
			steady = fmt.Sprintf(" (steady @s%d)", result.SteadyStateSession)
		}
		lines = append(lines, line(fmt.Sprintf("  F1:  %.2f%s", result.AvgF1, steady)))

		// Line 3: Precision + Recall
		lines = append(lines, line(fmt.Sprintf("  P:   %.2f  R: %.2f", result.AvgPrecision, result.AvgRecall)))

		// Line 4: Token efficiency
		lines = append(lines, line(fmt.Sprintf("  Eff: %.0f%% of store", result.AvgTokenEfficiency*100)))

		// Optional: Mistake Prevention Rate
		if result.MistakePreventionRate > 0 {
			lines = append(lines, line(fmt.Sprintf("  MPR: %.0f%%", result.MistakePreventionRate*100)))
		}

		// Optional: Correction Rate
		if result.CorrectionRate > 0 {
			lines = append(lines, line(fmt.Sprintf("  CorrRate: %.0f%%", result.CorrectionRate*100)))
		}

		// Blank separator between scenarios (not after the last one)
		if i < len(report.Results)-1 {
			lines = append(lines, blank)
		}
	}

	lines = append(lines, bot)

	_, err := fmt.Fprintln(r.Writer, strings.Join(lines, "\n"))
	return err
}
