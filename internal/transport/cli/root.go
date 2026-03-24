package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	core "github.com/mnemos-dev/mnemos/internal/core"
)

var (
	cfgFile   string
	projectID string
	logLevel  string
)

// NewRootCmd creates the root cobra command
func NewRootCmd(mnemos *core.Mnemos, version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "mnemos",
		Short: "Mnemos — AI memory engine",
		Long:  "Mnemos is a unified memory engine for AI coding agents.",
		Version: version,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	root.PersistentFlags().StringVarP(&projectID, "project", "p", "", "project ID scope")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug|info|warn|error)")

	root.AddCommand(
		newStoreCmd(mnemos),
		newGetCmd(mnemos),
		newListCmd(mnemos),
		newSearchCmd(mnemos),
		newUpdateCmd(mnemos),
		newDeleteCmd(mnemos),
		newRelateCmd(mnemos),
		newStatsCmd(mnemos),
		newMaintainCmd(mnemos),
		newServeCmd(mnemos, version),
		newInitCmd(),
		newVersionCmd(version),
	)

	return root
}

func printJSON(v any) {
	enc := newJSONEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
