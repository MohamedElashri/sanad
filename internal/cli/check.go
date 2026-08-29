package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/MohamedElashri/sanad/internal/policy"
	"github.com/MohamedElashri/sanad/internal/workflow"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	fresh               bool
	failPendingCooldown bool
	workflowPaths       []string
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
	var strict bool
	var failOnUpdates bool
	var allowPendingCooldown bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate workflow dependencies against sanad policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strict {
				checkOpts.fresh = true
				checkOpts.failPendingCooldown = true
			}
			if failOnUpdates || allowPendingCooldown {
				checkOpts.fresh = true
			}
			return withRepositoryRoot(opts, func() error { return runCheck(cmd, opts, checkOpts, defaultPlanResolver) })
		},
	}
	cmd.Flags().BoolVar(&checkOpts.fresh, "fresh", false, "resolve tracked refs and fail when eligible updates exist")
	cmd.Flags().BoolVar(&strict, "strict", false, "with freshness checks, also fail on cooldown-pending updates")
	cmd.Flags().BoolVar(&allowPendingCooldown, "allow-pending-cooldown", false, "deprecated compatibility alias for --fresh")
	cmd.Flags().BoolVar(&failOnUpdates, "fail-on-updates", false, "deprecated compatibility alias for --fresh")
	cmd.Flags().StringSliceVar(&checkOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	_ = cmd.Flags().MarkHidden("allow-pending-cooldown")
	_ = cmd.Flags().MarkHidden("fail-on-updates")
	return cmd
}

func runCheck(cmd *cobra.Command, opts *rootOptions, checkOpts *checkOptions, resolver planResolver) error {
	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}
	var plan planReport
	if checkOpts.fresh {
		resolver, err = configuredPlanResolver(cfg, resolver)
		if err != nil {
			return err
		}
		plan, err = buildPlanReport(cmd.Context(), cfg, checkOpts.workflowPaths, resolver, planNow())
	} else {
		plan, err = buildLocalCheckPlan(cfg, checkOpts.workflowPaths)
	}
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

func buildLocalCheckPlan(cfg config.Config, workflowPaths []string) (planReport, error) {
	paths := cfg.WorkflowPaths
	if len(workflowPaths) > 0 {
		paths = workflowPaths
	}
	files, err := workflow.DiscoverWorkflowFiles(paths)
	if err != nil {
		return planReport{}, err
	}
	uses, err := workflow.ExtractUsesFromFiles(files)
	if err != nil {
		return planReport{}, err
	}
	reconciliation, err := reconcileWorkflowUses(uses)
	if err != nil {
		return planReport{}, err
	}

	actionsByFile := make(map[string][]planAction)
	for _, use := range uses {
		parsed := actions.Parse(use.Raw)
		reconciled, _ := reconciliation.Use(use.File, use.NodePath)
		logicalRef := ""
		if reconciled.HasMetadata {
			logicalRef = reconciled.Metadata.LogicalRef
		}
		decision := localCheckDecision(cfg, use, parsed, logicalRef, reconciled.Error)
		action := planActionFromDecision(use, parsed, nil, decision)
		if reconciled.HasMetadata && reconciled.Error == nil {
			action.MetadataSource = string(reconciled.Metadata.Source)
		}
		actionsByFile[use.File] = append(actionsByFile[use.File], action)
	}

	report := planReport{Version: planReportVersion, Files: []planFile{}}
	for _, file := range files {
		if len(actionsByFile[file]) == 0 {
			continue
		}
		report.Files = append(report.Files, planFile{Path: file, Actions: actionsByFile[file]})
	}
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].Path < report.Files[j].Path })
	report.Summary = summarizePlan(report)
	return report, nil
}

func localCheckDecision(cfg config.Config, use workflow.UseNode, parsed actions.ParsedAction, logicalRef string, metadataErr error) policy.Decision {
	if metadataErr != nil {
		return policy.Decision{Kind: policy.DecisionErrorInvalid, Reason: metadataErr.Error(), CurrentSHA: currentPinnedSHA(parsed), LogicalRef: logicalRef}
	}
	opts := policyOptionsFromConfig(cfg, planNow())
	if parsed.Kind != actions.KindLocalAction && parsed.Kind != actions.KindDockerAction {
		ignore, err := policy.MatchIgnore(parsed, use.File, opts)
		if err != nil {
			return policy.Decision{Kind: policy.DecisionErrorUnsupported, Reason: err.Error(), CurrentSHA: currentPinnedSHA(parsed), LogicalRef: logicalRef}
		}
		if ignore.Ignored {
			return ignoredDecision(parsed, logicalRef, ignore)
		}
	}
	if parsed.Kind == actions.KindReusableWorkflow && !cfg.Updates.ReusableWorkflows {
		return policy.Decision{Kind: policy.DecisionErrorReusable, Reason: "reusable workflows are denied by policy", CurrentSHA: currentPinnedSHA(parsed), LogicalRef: logicalRef}
	}
	if parsed.Valid && parsed.Pinned {
		return policy.Decision{Kind: policy.DecisionUnchanged, Reason: "immutable full SHA pin accepted", CurrentSHA: parsed.Ref, LogicalRef: logicalRef}
	}
	if parsed.Valid && parsed.Ref != "" && (parsed.Kind == actions.KindGitHubAction || parsed.Kind == actions.KindReusableWorkflow) {
		return policy.Decision{Kind: policy.DecisionUpdate, Reason: "mutable action reference must be pinned to a full SHA", LogicalRef: parsed.Ref}
	}
	if parsed.Valid && parsed.Ref == "" && (parsed.Kind == actions.KindGitHubAction || parsed.Kind == actions.KindReusableWorkflow) {
		return policy.Decision{Kind: policy.DecisionErrorUnpinned, Reason: "unpinned GitHub action reference must be pinned before use"}
	}
	return policy.Evaluate(policy.Entry{File: use.File, Action: parsed, LogicalRef: logicalRef}, opts)
}

func buildCheckReport(plan planReport, opts *checkOptions) checkReport {
	report := checkReport{Version: checkReportVersion, Passed: true, Violations: []checkViolation{}}
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
		return true
	case policy.DecisionPending:
		return opts.failPendingCooldown
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
	style := styleForCommand(cmd)
	if report.Passed {
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s Checked %d action(s).\n",
			style.Wrap(colorSuccess, "All workflow dependencies comply with sanad policy."),
			report.Summary.Checked,
		)
		return nil
	}

	rows := make([]styledTableRow, 0, len(report.Violations))
	for _, violation := range report.Violations {
		rows = append(rows, styledTableRow{
			{Text: violation.File, Role: colorFile},
			{Text: fmt.Sprintf("%d", violation.Line), Role: colorLine},
			{Text: violation.Action},
			{Text: string(violation.Decision), Role: decisionColorRole(violation.Decision)},
			{Text: emptyDash(strings.TrimSpace(violation.Reason)), Role: colorReason},
		})
	}
	if err := printStyledTable(cmd.OutOrStdout(), style, []string{"FILE", "LINE", "ACTION", "DECISION", "REASON"}, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", style.Wrapf(colorDanger, "Check failed with %d violation(s).", report.Summary.Violations))
	return nil
}
