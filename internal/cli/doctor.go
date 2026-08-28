package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type doctorReport struct {
	Version int         `json:"version"`
	Lock    lockReport  `json:"lock"`
	Policy  checkReport `json:"policy"`
}

func newDoctorCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &lockOptions{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose policy and lockfile health, and repair safe lockfile drift",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(rootOpts, func() error {
				return runDoctor(cmd, rootOpts, opts)
			})
		},
	}
	cmd.Flags().BoolVar(&opts.write, "write", false, "repair safe lockfile drift")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "approve non-interactive repairs")
	cmd.Flags().StringSliceVar(&opts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}

func runDoctor(cmd *cobra.Command, rootOpts *rootOptions, opts *lockOptions) error {
	state, err := buildLockState(rootOpts, opts.workflowPaths)
	if err != nil {
		return err
	}
	lock, err := buildLockReport(state, lockModeRepair, !opts.write)
	if err != nil {
		return err
	}
	if opts.write && lock.WouldWrite && lock.Summary.Blocking == 0 {
		if err := authorizeWrite(cmd, opts.yes, "Write these lockfile changes? [y/N] "); err != nil {
			return err
		}
		if err := saveLockEntries(state.lockfile, lock.target); err != nil {
			return err
		}
		lock.Wrote = true
	}

	cfg, err := loadConfig(rootOpts)
	if err != nil {
		return err
	}
	plan, err := buildLocalCheckPlan(cfg, opts.workflowPaths)
	if err != nil {
		return err
	}
	policy := buildCheckReport(plan, &checkOptions{workflowPaths: opts.workflowPaths})
	report := doctorReport{Version: 1, Lock: lock, Policy: policy}

	switch rootOpts.format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	case "table":
		if err := printLockTable(cmd, report.Lock); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		if err := printCheckTable(cmd, report.Policy); err != nil {
			return err
		}
	default:
		return categorizedError{code: exitConfig, err: fmt.Errorf("unsupported format %q: expected table or json", rootOpts.format)}
	}

	if lock.Summary.Blocking > 0 {
		return categorizedError{code: exitConfig, err: fmt.Errorf("lockfile has %d blocking diagnostic(s)", lock.Summary.Blocking)}
	}
	if !policy.Passed {
		return categorizedError{code: exitPolicy, err: fmt.Errorf("check failed with %d violation(s)", policy.Summary.Violations)}
	}
	return nil
}
