package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sanad %s\n", version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "commit %s\n", commit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "built %s\n", date)
			return nil
		},
	}
}
