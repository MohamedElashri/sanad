package cli

import (
	"errors"

	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/spf13/cobra"
)

const (
	exitSuccess       = 0
	exitPolicy        = 1
	exitConfig        = 2
	exitUnresolved    = 3
	exitGitHubAPI     = 4
	exitRateLimit     = 5
	exitUnsafeRewrite = 6
	exitFileSystem    = 7
	exitInternal      = 8
)

type configError struct {
	err error
}

func (e configError) Error() string {
	return e.err.Error()
}

func (e configError) Unwrap() error {
	return e.err
}

type rootOptions struct {
	configPath string
	format     string
	color      string
	root       string
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "sanad",
		Short:         "Pin GitHub Actions workflow dependencies to immutable commit SHAs",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(opts, func() error { return runCheck(cmd, opts, &checkOptions{}, nil) })
		},
	}

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", config.DefaultPath, "path to config file")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "table", "output format (table, json, or command-specific formats)")
	cmd.PersistentFlags().StringVar(&opts.color, "color", colorModeAuto, "colorize human output: auto, always, or never")
	cmd.PersistentFlags().StringVar(&opts.root, "root", "", "repository root (discovered from .git by default)")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if _, err := colorSettingsForCommand(cmd); err != nil {
			return categorizedError{code: exitConfig, err: err}
		}
		return nil
	}

	cmd.AddCommand(
		newStartCommand(opts),
		newScanCommand(opts),
		newPlanCommand(opts),
		newCheckCommand(opts),
		newApplyCommand(opts),
		newUpgradeCommand(opts),
		newLockCommand(opts),
		newDoctorCommand(opts),
		newConfigCommand(opts),
		newCompletionCommand(),
		newVersionCommand(),
	)

	return cmd
}

func loadConfig(opts *rootOptions) (config.Config, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return config.Config{}, configError{err: err}
	}
	return cfg, nil
}

func ExitCode(err error) int {
	if err == nil {
		return exitSuccess
	}

	var categorized categorizedError
	if errors.As(err, &categorized) {
		return categorized.code
	}

	var cfgErr configError
	if errors.As(err, &cfgErr) {
		return exitConfig
	}

	return exitInternal
}
