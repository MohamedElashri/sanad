package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/MohamedElashri/sanad/internal/policy"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	strict               bool
	allowPendingCooldown bool
	failOnUpdates        bool
	workflowPaths        []string
}

type checkReport struct {
	Version    int              `json:"version"`
	Passed     bool             `json:"passed"`
	Summary    checkSummary     `json:"summary"`
	Violations []checkViolation `json:"violations"`
}

type checkSummary struct {
	Checked    int `json:"checked"`
	Violations int `json:"violations"`
	Updates    int `json:"updates"`
	Pending    int `json:"pending_cooldown"`
	Skipped    int `json:"skipped"`
}

type checkViolation struct {
	File         string              `json:"file"`
	NodePath     string              `json:"node_path"`
	Line         int                 `json:"line"`
	Column       int                 `json:"column"`
	Raw          string              `json:"raw"`
	Action       string              `json:"action"`
	Decision     policy.DecisionKind `json:"decision"`
	ReasonCode   string              `json:"reason_code"`
	Reason       string              `json:"reason,omitempty"`
	CurrentSHA   string              `json:"current_sha,omitempty"`
	CandidateSHA string              `json:"candidate_sha,omitempty"`
	LogicalRef   string              `json:"logical_ref,omitempty"`
}

func newCheckCommand(opts *rootOptions) *cobra.Command {
	checkOpts := &checkOptions{}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate workflow dependencies against sanad policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, opts, checkOpts, defaultPlanResolver)
		},
	}
	cmd.Flags().BoolVar(&checkOpts.strict, "strict", false, "fail on available and pending managed updates")
	cmd.Flags().BoolVar(&checkOpts.allowPendingCooldown, "allow-pending-cooldown", false, "do not fail on pending cooldown updates")
	cmd.Flags().BoolVar(&checkOpts.failOnUpdates, "fail-on-updates", false, "fail when already-pinned managed actions have eligible updates")
	cmd.Flags().StringSliceVar(&checkOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}

func runCheck(cmd *cobra.Command, opts *rootOptions, checkOpts *checkOptions, resolver planResolver) error {
	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}
	resolver, err = configuredPlanResolver(cfg, resolver)
	if err != nil {
		return err
	}

	plan, err := buildPlanReport(cmd.Context(), cfg, checkOpts.workflowPaths, resolver, planNow())
	if err != nil {
		return err
	}
	report := buildCheckReport(plan, checkOpts)

	switch opts.format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	case "sarif":
		if err := printCheckSARIF(cmd.OutOrStdout(), report); err != nil {
			return err
		}
	case "table":
		if err := printCheckTable(cmd, report); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format %q: expected table, json, or sarif", opts.format)
	}

	if !report.Passed {
		return categorizedError{
			code: exitPolicy,
			err:  fmt.Errorf("check failed with %d violation(s)", report.Summary.Violations),
		}
	}
	return nil
}

func buildCheckReport(plan planReport, opts *checkOptions) checkReport {
	report := checkReport{Version: 1, Passed: true}
	for _, file := range plan.Files {
		for _, action := range file.Actions {
			report.Summary.Checked++
			switch action.Decision {
			case policy.DecisionUpdate:
				report.Summary.Updates++
			case policy.DecisionPending:
				report.Summary.Pending++
			case policy.DecisionSkip, policy.DecisionSkipLocalAction, policy.DecisionSkipDockerAction, policy.DecisionSkipIgnored:
				report.Summary.Skipped++
			}

			if checkActionViolates(action, opts) {
				report.Violations = append(report.Violations, checkViolationFromPlanAction(file.Path, action))
			}
		}
	}
	report.Summary.Violations = len(report.Violations)
	report.Passed = report.Summary.Violations == 0
	return report
}

func checkActionViolates(action planAction, opts *checkOptions) bool {
	if isBlockingDecision(action.Decision) {
		return true
	}

	switch action.Decision {
	case policy.DecisionUpdate:
		if action.CurrentSHA == "" {
			return true
		}
		return opts.strict || opts.failOnUpdates
	case policy.DecisionPending:
		if action.CurrentSHA == "" {
			return true
		}
		return opts.strict && !opts.allowPendingCooldown
	default:
		return false
	}
}

func checkViolationFromPlanAction(file string, action planAction) checkViolation {
	return checkViolation{
		File:         file,
		NodePath:     action.NodePath,
		Line:         action.Line,
		Column:       action.Column,
		Raw:          action.Raw,
		Action:       planActionName(action),
		Decision:     action.Decision,
		ReasonCode:   action.ReasonCode,
		Reason:       action.Reason,
		CurrentSHA:   action.CurrentSHA,
		CandidateSHA: action.CandidateSHA,
		LogicalRef:   action.LogicalRef,
	}
}

func printCheckTable(cmd *cobra.Command, report checkReport) error {
	if report.Passed {
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			"All workflow dependencies comply with sanad policy. Checked %d action(s).\n",
			report.Summary.Checked,
		)
		return nil
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "FILE\tLINE\tACTION\tDECISION\tREASON")
	for _, violation := range report.Violations {
		_, _ = fmt.Fprintf(
			writer,
			"%s\t%d\t%s\t%s\t%s\n",
			violation.File,
			violation.Line,
			violation.Action,
			violation.Decision,
			emptyDash(strings.TrimSpace(violation.Reason)),
		)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCheck failed with %d violation(s).\n", report.Summary.Violations)
	return nil
}
