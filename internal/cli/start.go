package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newStartCommand(opts *rootOptions) *cobra.Command {
	startOpts := &applyOptions{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Initialize sanad and apply default settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(opts, func() error {
				if !cmd.Flags().Changed("write") && isTerminal(cmd.InOrStdin()) {
					startOpts.write = true
					startOpts.interactive = !startOpts.yes
				} else if startOpts.write && isTerminal(cmd.InOrStdin()) && !startOpts.yes {
					startOpts.interactive = true
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Using built-in defaults unless .sanad.toml overrides them.")
				fmt.Fprintln(cmd.OutOrStdout(), "Scanning workflows and preparing the initial pins...")
				return runApply(cmd, opts, startOpts, defaultPlanResolver)
			})
		},
	}
	cmd.Flags().BoolVar(&startOpts.diff, "diff", false, "show the unified workflow file diff")
	cmd.Flags().BoolVar(&startOpts.write, "write", false, "write workflow pins and lockfile")
	cmd.Flags().BoolVarP(&startOpts.yes, "yes", "y", false, "approve non-interactive writes")
	cmd.Flags().StringSliceVar(&startOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}
