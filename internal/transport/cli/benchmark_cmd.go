package cli

import (
	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/spf13/cobra"
)

func addBenchmarkCmd(root *cobra.Command) {
	root.AddCommand(benchmark.NewBenchmarkCmd())
}
