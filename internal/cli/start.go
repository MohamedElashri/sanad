package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/spf13/cobra"
)

func newStartCommand(opts *rootOptions) *cobra.Command {
	startOpts := &applyOptions{
		write:       true,
		yes:         true,
		interactive: false,
	}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Initialize sanad and apply default settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := opts.configPath
			if path == "" {
				path = config.DefaultPath
			}
			_, err := os.Stat(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// Generate default config
					defaultConfig := `workflow_paths = [".github/workflows"]
cooldown = "7d"
cooldown_source = "source"

[updates]
tags = "track"
branches = "deny"
unpinned = "deny"
reusable_workflows = true
`
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						return categorizedError{code: exitFileSystem, err: fmt.Errorf("write config %q: %w", path, err)}
					}
					if err := os.WriteFile(path, []byte(defaultConfig), 0o600); err != nil {
						return categorizedError{code: exitFileSystem, err: fmt.Errorf("write config %q: %w", path, err)}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Created default config at %s\n", path)
				} else {
					return categorizedError{code: exitConfig, err: fmt.Errorf("check config %q: %w", path, err)}
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Running initial scan and apply...\n")
			return runApply(cmd, opts, startOpts, defaultPlanResolver)
		},
	}
	cmd.Flags().StringSliceVar(&startOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}
