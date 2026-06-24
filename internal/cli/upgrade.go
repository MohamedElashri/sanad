package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/MohamedElashri/sanad/internal/githubresolver"
	"github.com/MohamedElashri/sanad/internal/metadata"
	"github.com/MohamedElashri/sanad/internal/policy"
	"github.com/MohamedElashri/sanad/internal/workflow"
	"github.com/spf13/cobra"
)

type upgradeOptions struct {
	action            string
	all               bool
	to                string
	latestRelease     bool
	latestReleaseMode string
	level             string
	constraint        string
	selection         string
	levelSet          bool
	constraintSet     bool
	selectionSet      bool
	dryRun            bool
	diff              bool
	write             bool
	workflowPaths     []string
}

type upgradePlan struct {
	Report              upgradeReport
	ChangesByFile       map[string][]workflow.RewriteChange
	LockEntries         []metadata.LockfileEntry
	Blockers            []applyBlocker
	ObservationsChanged bool
}

type upgradeReport struct {
	Version int             `json:"version"`
	Summary upgradeSummary  `json:"summary"`
	Actions []upgradeAction `json:"actions"`
}

type upgradeSummary struct {
	Matched   int `json:"matched"`
	Updates   int `json:"updates"`
	Pending   int `json:"pending_cooldown"`
	Unchanged int `json:"unchanged"`
	Blocked   int `json:"blocked"`
}

type upgradeAction struct {
	File              string              `json:"file"`
	NodePath          string              `json:"node_path"`
	Line              int                 `json:"line"`
	Action            string              `json:"action"`
	CurrentLogicalRef string              `json:"current_logical_ref"`
	TargetLogicalRef  string              `json:"target_logical_ref"`
	CurrentSHA        string              `json:"current_sha"`
	CandidateSHA      string              `json:"candidate_sha,omitempty"`
	Decision          policy.DecisionKind `json:"decision"`
	ReasonCode        string              `json:"reason_code"`
	Reason            string              `json:"reason,omitempty"`
	Age               string              `json:"age,omitempty"`
	Level             string              `json:"level,omitempty"`
	Constraint        string              `json:"constraint,omitempty"`
	Selection         string              `json:"selection,omitempty"`
	SelectedRelease   string              `json:"selected_release,omitempty"`
	Candidates        []upgradeCandidate  `json:"candidates,omitempty"`
}

type upgradeCandidate struct {
	LogicalRef string              `json:"logical_ref"`
	Version    string              `json:"version"`
	SHA        string              `json:"sha,omitempty"`
	Decision   policy.DecisionKind `json:"decision"`
	Reason     string              `json:"reason,omitempty"`
	Age        string              `json:"age,omitempty"`
}

func newUpgradeCommand(opts *rootOptions) *cobra.Command {
	upgradeOpts := &upgradeOptions{}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Move managed workflow pins to a new logical ref",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, opts, upgradeOpts, defaultPlanResolver)
		},
	}
	cmd.Flags().StringVar(&upgradeOpts.action, "action", "", "managed action selector to upgrade, such as actions/checkout")
	cmd.Flags().BoolVar(&upgradeOpts.all, "all", false, "upgrade all managed action pins")
	cmd.Flags().StringVar(&upgradeOpts.to, "to", "", "target logical ref, such as v5")
	cmd.Flags().BoolVar(&upgradeOpts.latestRelease, "latest-release", false, "compatibility alias for --selection latest")
	cmd.Flags().StringVar(&upgradeOpts.latestReleaseMode, "latest-release-mode", "", "deprecated compatibility override for upgrade.latest_release")
	cmd.Flags().StringVar(&upgradeOpts.level, "level", "", "maximum automatic SemVer change: major, minor, or patch")
	cmd.Flags().StringVar(&upgradeOpts.constraint, "constraint", "", "automatic release SemVer constraint")
	cmd.Flags().StringVar(&upgradeOpts.selection, "selection", "", "automatic release selection: latest-eligible or latest")
	cmd.Flags().BoolVar(&upgradeOpts.dryRun, "dry-run", false, "show proposed changes without writing files")
	cmd.Flags().BoolVar(&upgradeOpts.diff, "diff", false, "show the unified workflow file diff")
	cmd.Flags().BoolVar(&upgradeOpts.write, "write", false, "write changes to workflow files and lockfile")
	cmd.Flags().StringSliceVar(&upgradeOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}

func runUpgrade(cmd *cobra.Command, rootOpts *rootOptions, upgradeOpts *upgradeOptions, resolver planResolver) error {
	cfg, err := loadConfig(rootOpts)
	if err != nil {
		return err
	}
	effectiveOpts := upgradeOptionsWithDefaults(cmd, upgradeOpts)
	if err := validateUpgradeOptions(&effectiveOpts); err != nil {
		return categorizedError{code: exitConfig, err: err}
	}
	if effectiveOpts.diff && rootOpts.format != "table" {
		return categorizedError{code: exitConfig, err: fmt.Errorf("--diff requires --format table")}
	}
	if effectiveOpts.latestReleaseMode != "" {
		latestRelease, err := normalizeUpgradeLatestReleaseForCLI(effectiveOpts.latestReleaseMode)
		if err != nil {
			return categorizedError{code: exitConfig, err: err}
		}
		cfg.Upgrade.LatestRelease = latestRelease
	}

	resolver, err = configuredPlanResolver(cfg, resolver)
	if err != nil {
		return err
	}

	plan, err := buildUpgradePlan(cmd.Context(), cfg, &effectiveOpts, resolver, planNow())
	if err != nil {
		return err
	}
	if plan.Report.Summary.Matched == 0 {
		return categorizedError{code: exitPolicy, err: fmt.Errorf("upgrade matched no managed action pins")}
	}

	rewrites, err := buildWorkflowRewrites(plan.ChangesByFile, cfg.Comments)
	if err != nil {
		return err
	}

	if rootOpts.format == "json" {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan.Report); err != nil {
			return err
		}
	} else if rootOpts.format == "table" {
		if err := printUpgradeTable(cmd, plan.Report); err != nil {
			return err
		}
		if effectiveOpts.diff && len(rewrites) > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			if err := printApplyDiff(cmd.OutOrStdout(), rewrites, styleForCommand(cmd)); err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("unsupported format %q: expected table or json", rootOpts.format)
	}

	if len(plan.Blockers) > 0 {
		return categorizedError{code: blockerExitCode(plan.Blockers), err: upgradeBlockersError(plan.Blockers)}
	}
	if effectiveOpts.dryRun || !effectiveOpts.write {
		if len(rewrites) > 0 && rootOpts.format == "table" {
			if effectiveOpts.diff {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nDry run only; no files changed. Add --write to apply these upgrades.")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nDry run only; no files changed. Add --diff to show the patch or --write to apply these upgrades.")
			}
		}
		return nil
	}
	if len(rewrites) == 0 {
		if plan.ObservationsChanged {
			if err := saveApplyLockfile(plan.LockEntries); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated lockfile observations; no eligible upgrades to apply.")
			return nil
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No eligible upgrades to apply.")
		return nil
	}

	if err := writeWorkflowRewrites(rewrites); err != nil {
		return err
	}
	if err := saveApplyLockfile(plan.LockEntries); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Applied %d logical ref upgrade(s) across %d file(s).\n", countWorkflowUpdates(plan.ChangesByFile), len(rewrites))
	return nil
}

func upgradeOptionsWithDefaults(cmd *cobra.Command, opts *upgradeOptions) upgradeOptions {
	effective := *opts
	flags := cmd.Flags()
	effective.levelSet = flags.Changed("level")
	effective.constraintSet = flags.Changed("constraint")
	effective.selectionSet = flags.Changed("selection")
	if !flags.Changed("action") && !flags.Changed("all") {
		effective.all = true
	}
	if effective.latestRelease {
		if effective.selectionSet && strings.TrimSpace(effective.selection) != "latest" {
			effective.selection = strings.TrimSpace(effective.selection)
		} else {
			effective.selection = "latest"
			effective.selectionSet = true
		}
	}
	return effective
}

func validateUpgradeOptions(opts *upgradeOptions) error {
	if opts.write && opts.dryRun {
		return fmt.Errorf("--write and --dry-run cannot be used together")
	}
	if opts.all == (strings.TrimSpace(opts.action) != "") {
		return fmt.Errorf("pass exactly one of --action or --all")
	}
	if opts.levelSet && opts.constraintSet {
		return fmt.Errorf("--level and --constraint cannot be used together")
	}
	if opts.to != "" && (opts.latestRelease || opts.levelSet || opts.constraintSet || opts.selectionSet) {
		return fmt.Errorf("--to cannot be combined with automatic release policy flags")
	}
	if opts.latestRelease && opts.selectionSet && strings.TrimSpace(opts.selection) != "latest" {
		return fmt.Errorf("--latest-release cannot be combined with --selection %q", opts.selection)
	}
	if opts.levelSet {
		if _, err := normalizeUpgradeLevelForCLI(opts.level); err != nil {
			return err
		}
	}
	if opts.constraintSet {
		if _, err := normalizeUpgradeConstraintForCLI(opts.constraint); err != nil {
			return err
		}
	}
	if opts.selectionSet {
		if _, err := normalizeUpgradeSelectionForCLI(opts.selection); err != nil {
			return err
		}
	}
	if strings.Contains(opts.action, "@") {
		return fmt.Errorf("--action must not include @ref")
	}
	target := strings.TrimSpace(opts.to)
	if target != "" && (actions.IsFullSHA(target) || actions.IsShortSHA(target)) {
		return fmt.Errorf("--to must be a logical ref, not a commit SHA")
	}
	return nil
}

func normalizeUpgradeLevelForCLI(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "major", "minor", "patch":
		return value, nil
	default:
		return "", fmt.Errorf("--level %q is not supported; expected major, minor, or patch", value)
	}
}

func normalizeUpgradeSelectionForCLI(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case "latest-eligible", "latest":
		return value, nil
	default:
		return "", fmt.Errorf("--selection %q is not supported; expected latest-eligible or latest", value)
	}
}

func normalizeUpgradeLatestReleaseForCLI(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case config.DefaultUpgradeLatestRelease, "release":
		return config.DefaultUpgradeLatestRelease, nil
	case "":
		return "", fmt.Errorf("--latest-release-mode must not be empty")
	default:
		return "", fmt.Errorf("--latest-release-mode %q is not supported; expected %q", value, config.DefaultUpgradeLatestRelease)
	}
}

func buildUpgradePlan(ctx context.Context, cfg config.Config, opts *upgradeOptions, resolver planResolver, now time.Time) (upgradePlan, error) {
	paths := cfg.WorkflowPaths
	if len(opts.workflowPaths) > 0 {
		paths = opts.workflowPaths
	}

	files, err := workflow.DiscoverWorkflowFiles(paths)
	if err != nil {
		return upgradePlan{}, err
	}
	uses, err := workflow.ExtractUsesFromFiles(files)
	if err != nil {
		return upgradePlan{}, err
	}
	reconciliation, err := reconcileWorkflowUses(uses)
	if err != nil {
		return upgradePlan{}, err
	}

	plan := upgradePlan{
		Report:        upgradeReport{Version: 2},
		ChangesByFile: make(map[string][]workflow.RewriteChange),
	}
	discovery := newUpgradeDiscovery(resolver)

	for _, use := range uses {
		parsed := actions.Parse(use.Raw)
		reconciled, _ := reconciliation.Use(use.File, use.NodePath)
		recovered := reconciled.Metadata
		hasMetadata := reconciled.HasMetadata
		metadataErr := reconciled.Error
		if currentEntry, ok := currentUpgradeLockEntry(use, parsed, recovered, hasMetadata, reconciled.Entry, reconciled.HasEntry && reconciled.CandidateHistoryPreservable, now); ok {
			plan.LockEntries = append(plan.LockEntries, currentEntry)
		}
		if metadataErr != nil || !isManagedUpgradeCandidate(parsed, hasMetadata) || !upgradeMatchesAction(parsed, opts) {
			continue
		}

		var candidate *githubresolver.ResolvedRef
		var targetLogicalRef string
		var resolveErr error
		var decision policy.Decision
		var effectivePolicy effectiveUpgradePolicy
		var candidateReports []upgradeCandidate
		var history []metadata.CandidateHistoryEntry
		if reconciled.CandidateHistoryPreservable {
			history = append(history, reconciled.Entry.Candidates...)
		}
		if strings.TrimSpace(opts.to) != "" {
			candidate, targetLogicalRef, resolveErr = resolveExplicitUpgradeTarget(ctx, discovery, parsed, opts.to)
			candidateSeenAt := candidateObservationTime(
				reconciled.Entry,
				reconciled.HasEntry && reconciled.CandidateHistoryPreservable,
				hasMetadata,
				candidate,
				now,
				cfg.CooldownSource,
			)
			if candidate != nil && cfg.CooldownSource == string(policy.CooldownSourceFirstSeen) {
				history = observeUpgradeCandidate(history, *candidate, candidateSeenAt)
			}
			decision = policy.Evaluate(policy.Entry{
				File:            use.File,
				Action:          parsed,
				Candidate:       candidate,
				ResolveErr:      resolveErr,
				LogicalRef:      targetLogicalRef,
				CurrentSHA:      parsed.Ref,
				CandidateSeenAt: candidateSeenAt,
				Now:             now,
			}, policyOptionsFromConfig(cfg, now))
			if decision.Kind == policy.DecisionUpdate || decision.Kind == policy.DecisionUnchanged {
				history = removeUpgradeCandidate(history, targetLogicalRef, candidate)
			}
		} else {
			effectivePolicy, resolveErr = effectivePolicyForAction(cfg, opts, actionSelectorString(parsed))
			if resolveErr == nil {
				selection, err := selectAutomaticUpgrade(ctx, discovery, parsed, recovered.LogicalRef, history, effectivePolicy, cfg, now)
				resolveErr = err
				candidate = selection.Candidate
				targetLogicalRef = selection.TargetLogicalRef
				decision = selection.Decision
				candidateReports = selection.Candidates
				history = selection.History
			}
			if resolveErr != nil {
				decision = policy.Evaluate(policy.Entry{
					File:       use.File,
					Action:     parsed,
					Candidate:  candidate,
					ResolveErr: resolveErr,
					LogicalRef: targetLogicalRef,
					CurrentSHA: parsed.Ref,
					Now:        now,
				}, policyOptionsFromConfig(cfg, now))
			}
		}
		if decision.Kind == policy.DecisionUnchanged && recovered.LogicalRef != targetLogicalRef {
			decision = policy.Decision{
				Kind:       policy.DecisionUpdate,
				Reason:     "logical ref metadata will be updated",
				CurrentSHA: parsed.Ref,
				NewSHA:     parsed.Ref,
				LogicalRef: targetLogicalRef,
			}
		}

		action := upgradeActionFromDecision(use, parsed, recovered.LogicalRef, targetLogicalRef, candidate, decision)
		action.Level = effectivePolicy.Level
		action.Constraint = effectivePolicy.Constraint
		action.Selection = effectivePolicy.Selection
		action.SelectedRelease = targetLogicalRef
		action.Candidates = candidateReports
		if len(candidateReports) > 1 && (decision.Kind == policy.DecisionUpdate || decision.Kind == policy.DecisionUnchanged) {
			action.Reason = fmt.Sprintf("%s; selected %s after %d newer release(s) were ineligible", action.Reason, targetLogicalRef, len(candidateReports)-1)
		}
		if cfg.CooldownSource == string(policy.CooldownSourceFirstSeen) && !candidateHistoriesEqual(reconciled.Entry.Candidates, history) {
			plan.ObservationsChanged = true
		}
		plan.Report.Actions = append(plan.Report.Actions, action)
		plan.Report.Summary.Matched++
		switch decision.Kind {
		case policy.DecisionUpdate:
			plan.Report.Summary.Updates++
			plan.ChangesByFile[use.File] = append(plan.ChangesByFile[use.File], workflow.RewriteChange{
				Use:      use,
				Action:   parsed,
				Decision: decision,
			})
			replaceUpgradeLockEntry(&plan.LockEntries, use, parsed, recovered.LogicalRef, decision, candidate, history, now)
		case policy.DecisionPending:
			plan.Report.Summary.Pending++
			replaceUpgradeLockEntry(&plan.LockEntries, use, parsed, recovered.LogicalRef, decision, candidate, history, now)
		case policy.DecisionUnchanged:
			plan.Report.Summary.Unchanged++
			replaceUpgradeLockEntry(&plan.LockEntries, use, parsed, recovered.LogicalRef, decision, candidate, history, now)
		default:
			if isBlockingDecision(decision.Kind) {
				plan.Report.Summary.Blocked++
				plan.Blockers = append(plan.Blockers, applyBlocker{
					File:     use.File,
					Line:     use.Line,
					Decision: decision.Kind,
					Reason:   decision.Reason,
					Err:      resolveErr,
				})
			}
		}
	}

	sort.Slice(plan.Report.Actions, func(i, j int) bool {
		left := plan.Report.Actions[i]
		right := plan.Report.Actions[j]
		if left.File != right.File {
			return left.File < right.File
		}
		return left.Line < right.Line
	})
	return plan, nil
}

func isManagedUpgradeCandidate(parsed actions.ParsedAction, hasMetadata bool) bool {
	if !parsed.Valid || !parsed.Pinned || !hasMetadata {
		return false
	}
	return parsed.Kind == actions.KindGitHubAction || parsed.Kind == actions.KindReusableWorkflow
}

func upgradeMatchesAction(parsed actions.ParsedAction, opts *upgradeOptions) bool {
	if opts.all {
		return true
	}
	return actionSelectorString(parsed) == strings.TrimSpace(opts.action)
}

func resolveExplicitUpgradeTarget(ctx context.Context, discovery *upgradeDiscovery, parsed actions.ParsedAction, target string) (*githubresolver.ResolvedRef, string, error) {
	if discovery == nil || discovery.resolver == nil {
		return nil, "", fmt.Errorf("resolver is required")
	}
	target = strings.TrimSpace(target)
	targetParsed := actions.Parse(actionSelectorString(parsed) + "@" + target)
	if !targetParsed.Valid {
		return nil, target, fmt.Errorf("invalid target ref %q: %s", target, targetParsed.Error)
	}
	resolved, err := discovery.resolve(ctx, githubresolver.ActionSelector{
		Owner: parsed.Owner,
		Repo:  parsed.Repo,
		Ref:   target,
	})
	if err != nil {
		return nil, target, err
	}
	return &resolved, resolved.Ref, nil
}

func upgradeActionFromDecision(use workflow.UseNode, parsed actions.ParsedAction, currentLogicalRef string, targetLogicalRef string, candidate *githubresolver.ResolvedRef, decision policy.Decision) upgradeAction {
	action := upgradeAction{
		File:              use.File,
		NodePath:          use.NodePath,
		Line:              use.Line,
		Action:            actionSelectorString(parsed),
		CurrentLogicalRef: currentLogicalRef,
		TargetLogicalRef:  targetLogicalRef,
		CurrentSHA:        parsed.Ref,
		Decision:          decision.Kind,
		ReasonCode:        reasonCodeForDecision(decision.Kind),
		Reason:            decision.Reason,
	}
	if candidate != nil {
		action.CandidateSHA = candidate.SHA
	}
	if decision.NewSHA != "" {
		action.CandidateSHA = decision.NewSHA
	}
	if decision.Age != 0 {
		action.Age = decision.Age.String()
	}
	return action
}

func currentUpgradeLockEntry(use workflow.UseNode, parsed actions.ParsedAction, recovered metadata.Metadata, hasMetadata bool, previous metadata.LockfileEntry, preserveHistory bool, now time.Time) (metadata.LockfileEntry, bool) {
	if !isManagedUpgradeCandidate(parsed, hasMetadata) {
		return metadata.LockfileEntry{}, false
	}
	entry, ok := lockfileEntryForDecision(use, parsed, policy.Decision{
		Kind:       policy.DecisionUnchanged,
		CurrentSHA: parsed.Ref,
		LogicalRef: recovered.LogicalRef,
	}, nil, now)
	if ok && preserveHistory {
		entry.Candidates = append([]metadata.CandidateHistoryEntry(nil), previous.Candidates...)
	}
	return entry, ok
}

func replaceUpgradeLockEntry(entries *[]metadata.LockfileEntry, use workflow.UseNode, parsed actions.ParsedAction, activeLogicalRef string, decision policy.Decision, candidate *githubresolver.ResolvedRef, history []metadata.CandidateHistoryEntry, now time.Time) {
	lockDecision := decision
	if decision.Kind == policy.DecisionPending {
		lockDecision.LogicalRef = activeLogicalRef
	}
	entry, ok := lockfileEntryForDecision(use, parsed, lockDecision, candidate, now)
	if !ok {
		return
	}
	entry.Candidates = append([]metadata.CandidateHistoryEntry(nil), history...)
	key := metadata.Key(use.File, use.NodePath)
	for i := range *entries {
		if metadata.Key((*entries)[i].File, (*entries)[i].Node) == key {
			(*entries)[i] = entry
			return
		}
	}
	*entries = append(*entries, entry)
}

func printUpgradeTable(cmd *cobra.Command, report upgradeReport) error {
	style := styleForCommand(cmd)
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Matched %d managed action pin(s): %s update(s), %s pending cooldown, %s unchanged, %s blocked.\n\n",
		report.Summary.Matched,
		style.Wrapf(colorWarning, "%d", report.Summary.Updates),
		style.Wrapf(colorWarning, "%d", report.Summary.Pending),
		style.Wrapf(colorSuccess, "%d", report.Summary.Unchanged),
		style.Wrapf(colorDanger, "%d", report.Summary.Blocked),
	)
	if len(report.Actions) == 0 {
		return nil
	}
	rows := make([]styledTableRow, 0, len(report.Actions))
	for _, action := range report.Actions {
		rows = append(rows, styledTableRow{
			{Text: action.File, Role: colorFile},
			{Text: fmt.Sprintf("%d", action.Line), Role: colorLine},
			{Text: action.Action},
			{Text: emptyDash(action.CurrentLogicalRef), Role: refColorRole(action.CurrentLogicalRef)},
			{Text: emptyDash(action.TargetLogicalRef), Role: refColorRole(action.TargetLogicalRef)},
			{Text: shortSHAOrDash(action.CurrentSHA), Role: currentSHAColorRole(action.Decision, action.CurrentSHA)},
			{Text: shortSHAOrDash(action.CandidateSHA), Role: candidateSHAColorRole(action.Decision, action.CandidateSHA)},
			{Text: string(action.Decision), Role: decisionColorRole(action.Decision)},
			{Text: emptyDash(action.Reason), Role: colorReason},
		})
	}
	return printStyledTable(
		cmd.OutOrStdout(),
		style,
		[]string{"FILE", "LINE", "ACTION", "FROM", "TO", "CURRENT", "CANDIDATE", "DECISION", "REASON"},
		rows,
	)
}

func upgradeBlockersError(blockers []applyBlocker) error {
	return errors.New(strings.Replace(blockersError(blockers).Error(), "apply blocked", "upgrade blocked", 1))
}
