package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

type applyOptions struct {
	dryRun        bool
	interactive   bool
	write         bool
	yes           bool
	workflowPaths []string
}

type applyPlan struct {
	Report        planReport
	ChangesByFile map[string][]workflow.RewriteChange
	LockEntries   []metadata.LockfileEntry
	Blockers      []applyBlocker
}

type applyBlocker struct {
	File     string
	Line     int
	Decision policy.DecisionKind
	Reason   string
	Err      error
}

type categorizedError struct {
	code int
	err  error
}

type interactiveApplySession struct {
	reader                         *bufio.Reader
	out                            io.Writer
	branchTrackingPersistenceAsked bool
	persistBranchTracking          bool
}

func (e categorizedError) Error() string {
	return e.err.Error()
}

func (e categorizedError) Unwrap() error {
	return e.err
}

func newApplyCommand(opts *rootOptions) *cobra.Command {
	applyOpts := &applyOptions{}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply approved workflow pin updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, opts, applyOpts, defaultPlanResolver)
		},
	}
	cmd.Flags().BoolVar(&applyOpts.dryRun, "dry-run", false, "show proposed changes without writing files")
	cmd.Flags().BoolVar(&applyOpts.interactive, "interactive", false, "prompt for confirmation")
	cmd.Flags().BoolVar(&applyOpts.write, "write", false, "write changes to workflow files")
	cmd.Flags().BoolVarP(&applyOpts.yes, "yes", "y", false, "approve non-interactive writes")
	cmd.Flags().StringSliceVar(&applyOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}

func runApply(cmd *cobra.Command, opts *rootOptions, applyOpts *applyOptions, resolver planResolver) error {
	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}
	resolver, err = configuredPlanResolver(cfg, resolver)
	if err != nil {
		return err
	}

	interactive := applyInteractiveSession(cmd, applyOpts)
	plan, err := buildApplyPlan(cmd.Context(), cfg, applyOpts.workflowPaths, resolver, planNow(), interactive)
	if err != nil {
		return err
	}
	if len(plan.Blockers) > 0 {
		_ = printPlanTable(cmd, plan.Report)
		return categorizedError{code: blockerExitCode(plan.Blockers), err: blockersError(plan.Blockers)}
	}

	rewrites, err := buildWorkflowRewrites(plan.ChangesByFile, cfg.Comments)
	if err != nil {
		return err
	}
	if applyOpts.dryRun {
		if len(rewrites) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No workflow updates to apply.")
			return nil
		}
		return printApplyDiff(cmd.OutOrStdout(), rewrites, styleForCommand(cmd))
	}

	if len(rewrites) == 0 {
		lockfileWrite, err := applyLockfileWouldWrite(plan.LockEntries)
		if err != nil {
			return err
		}
		if lockfileWrite && (applyOpts.write || interactive != nil) {
			if err := authorizeApply(cmd, applyOpts, plan.Report, rewrites, interactive); err != nil {
				return err
			}
			if err := saveApplyLockfile(plan.LockEntries); err != nil {
				return err
			}
			if err := saveInteractiveConfigChoices(opts, interactive); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated lockfile; no workflow updates to apply.")
			return nil
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No workflow updates to apply.")
		return nil
	}

	if err := authorizeApply(cmd, applyOpts, plan.Report, rewrites, interactive); err != nil {
		return err
	}

	if err := writeWorkflowRewrites(rewrites); err != nil {
		return err
	}
	if err := saveApplyLockfile(plan.LockEntries); err != nil {
		return err
	}
	if err := saveInteractiveConfigChoices(opts, interactive); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Applied %d workflow update(s) across %d file(s).\n", countWorkflowUpdates(plan.ChangesByFile), len(rewrites))
	return nil
}

func buildApplyPlan(ctx context.Context, cfg config.Config, workflowPaths []string, resolver planResolver, now time.Time, interactive *interactiveApplySession) (applyPlan, error) {
	files, evaluated, err := evaluateWorkflowUses(ctx, cfg, workflowPaths, resolver, now)
	if err != nil {
		return applyPlan{}, err
	}

	plan := applyPlan{
		Report:        planReport{Version: 1},
		ChangesByFile: make(map[string][]workflow.RewriteChange),
	}
	actionsByFile := make(map[string][]planAction)
	for _, item := range evaluated {
		use := item.Use
		parsed := item.Parsed
		candidate := item.Candidate
		resolveErr := item.ResolveErr
		decision := item.Decision
		if interactive != nil {
			var promptErr error
			parsed, candidate, resolveErr, decision, promptErr = applyInteractiveDecision(ctx, interactive, cfg, resolver, now, use, parsed, candidate, resolveErr, decision, item.HasMetadata, item.MetadataErr)
			if promptErr != nil {
				return applyPlan{}, promptErr
			}
		}

		action := planActionFromDecision(use, parsed, candidate, decision)
		if item.HasMetadata && item.MetadataErr == nil {
			action.MetadataSource = string(item.Recovered.Source)
		}
		actionsByFile[use.File] = append(actionsByFile[use.File], action)

		if isBlockingDecision(decision.Kind) {
			plan.Blockers = append(plan.Blockers, applyBlocker{
				File:     use.File,
				Line:     use.Line,
				Decision: decision.Kind,
				Reason:   decision.Reason,
				Err:      errors.Join(item.MetadataErr, resolveErr),
			})
			continue
		}

		if decision.Kind == policy.DecisionUpdate {
			plan.ChangesByFile[use.File] = append(plan.ChangesByFile[use.File], workflow.RewriteChange{
				Use:      use,
				Action:   parsed,
				Decision: decision,
			})
		}
		if entry, ok := lockfileEntryForDecision(use, parsed, decision, candidate, now); ok {
			plan.LockEntries = append(plan.LockEntries, entry)
		}
	}

	for _, file := range files {
		if len(actionsByFile[file]) == 0 {
			continue
		}
		plan.Report.Files = append(plan.Report.Files, planFile{
			Path:    file,
			Actions: actionsByFile[file],
		})
	}
	sort.Slice(plan.Report.Files, func(i, j int) bool {
		return plan.Report.Files[i].Path < plan.Report.Files[j].Path
	})
	return plan, nil
}

func applyInteractiveSession(cmd *cobra.Command, opts *applyOptions) *interactiveApplySession {
	if opts.yes && opts.write {
		return nil
	}
	in := cmd.InOrStdin()
	if !opts.interactive && !isTerminal(in) {
		return nil
	}
	return &interactiveApplySession{
		reader: bufio.NewReader(in),
		out:    cmd.OutOrStdout(),
	}
}

func applyInteractiveDecision(
	ctx context.Context,
	session *interactiveApplySession,
	cfg config.Config,
	resolver planResolver,
	now time.Time,
	use workflow.UseNode,
	parsed actions.ParsedAction,
	candidate *githubresolver.ResolvedRef,
	resolveErr error,
	decision policy.Decision,
	hasMetadata bool,
	metadataErr error,
) (actions.ParsedAction, *githubresolver.ResolvedRef, error, policy.Decision, error) {
	if metadataErr != nil {
		return parsed, candidate, resolveErr, decision, nil
	}

	switch {
	case decision.Kind == policy.DecisionErrorUnpinned:
		return promptUnpinnedAction(ctx, session, cfg, resolver, now, use, parsed, candidate, resolveErr, decision)
	case decision.Kind == policy.DecisionErrorUnresolved && parsed.Valid && parsed.Ref == "" && !hasMetadata:
		return promptUnpinnedAction(ctx, session, cfg, resolver, now, use, parsed, candidate, resolveErr, decision)
	case decision.Kind == policy.DecisionErrorBranchDenied && candidate != nil:
		return promptBranchAction(session, use, parsed, candidate, resolveErr, decision)
	case decision.Kind == policy.DecisionUnchanged && parsed.Pinned && !hasMetadata:
		return promptMissingLogicalRef(ctx, session, cfg, resolver, now, use, parsed, candidate, resolveErr, decision)
	default:
		return parsed, candidate, resolveErr, decision, nil
	}
}

func promptUnpinnedAction(
	ctx context.Context,
	session *interactiveApplySession,
	cfg config.Config,
	resolver planResolver,
	now time.Time,
	use workflow.UseNode,
	parsed actions.ParsedAction,
	candidate *githubresolver.ResolvedRef,
	resolveErr error,
	decision policy.Decision,
) (actions.ParsedAction, *githubresolver.ResolvedRef, error, policy.Decision, error) {
	fmt.Fprintf(session.out, "\nFound unpinned action:\n\n  %s:%d\n  %s\n\n", use.File, use.Line, use.Raw)
	choice, err := session.choice("How should sanad manage it?", []interactiveChoice{
		{Key: "e", Label: "Enter an explicit ref to pin"},
		{Key: "s", Label: "Skip for now"},
		{Key: "d", Label: "Leave as a policy violation"},
	}, "d")
	if err != nil {
		return parsed, candidate, resolveErr, decision, categorizedError{code: exitInternal, err: err}
	}
	switch choice {
	case "e":
		ref, err := session.line("Logical ref to resolve: ")
		if err != nil {
			return parsed, candidate, resolveErr, decision, categorizedError{code: exitInternal, err: err}
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return parsed, candidate, resolveErr, decision, nil
		}
		nextParsed := actions.Parse(actionSelectorString(parsed) + "@" + ref)
		nextCandidate, nextResolveErr := resolvePlanCandidate(ctx, resolver, nextParsed, "", policy.UnpinnedDeny)
		nextDecision := policy.Evaluate(policy.Entry{
			Action:     nextParsed,
			Candidate:  nextCandidate,
			ResolveErr: nextResolveErr,
			Now:        now,
		}, policyOptionsFromConfig(cfg, now))
		if nextDecision.Kind == policy.DecisionErrorBranchDenied && nextCandidate != nil {
			return promptBranchAction(session, use, nextParsed, nextCandidate, nextResolveErr, nextDecision)
		}
		return nextParsed, nextCandidate, nextResolveErr, nextDecision, nil
	case "s":
		decision = interactiveSkipDecision(parsed, "skipped by interactive choice")
		return parsed, candidate, nil, decision, nil
	default:
		return parsed, candidate, resolveErr, decision, nil
	}
}

func promptMissingLogicalRef(
	ctx context.Context,
	session *interactiveApplySession,
	cfg config.Config,
	resolver planResolver,
	now time.Time,
	use workflow.UseNode,
	parsed actions.ParsedAction,
	candidate *githubresolver.ResolvedRef,
	resolveErr error,
	decision policy.Decision,
) (actions.ParsedAction, *githubresolver.ResolvedRef, error, policy.Decision, error) {
	fmt.Fprintf(session.out, "\nFound unmanaged pinned action:\n\n  %s:%d\n  %s\n\n", use.File, use.Line, use.Raw)
	choice, err := session.choice("How should sanad manage it?", []interactiveChoice{
		{Key: "t", Label: "Track a logical ref"},
		{Key: "s", Label: "Keep static pin"},
		{Key: "i", Label: "Ignore for now"},
	}, "s")
	if err != nil {
		return parsed, candidate, resolveErr, decision, categorizedError{code: exitInternal, err: err}
	}
	if choice != "t" {
		return parsed, candidate, resolveErr, decision, nil
	}

	ref, err := session.line("Logical ref to track: ")
	if err != nil {
		return parsed, candidate, resolveErr, decision, categorizedError{code: exitInternal, err: err}
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return parsed, candidate, resolveErr, decision, nil
	}

	nextCandidate, nextResolveErr := resolvePlanCandidate(ctx, resolver, parsed, ref, policy.UnpinnedDeny)
	nextDecision := policy.Evaluate(policy.Entry{
		Action:     parsed,
		Candidate:  nextCandidate,
		ResolveErr: nextResolveErr,
		LogicalRef: ref,
		Now:        now,
	}, policyOptionsFromConfig(cfg, now))
	if nextDecision.LogicalRef == "" {
		nextDecision.LogicalRef = ref
	}
	if nextDecision.Kind == policy.DecisionErrorBranchDenied && nextCandidate != nil {
		return promptBranchAction(session, use, parsed, nextCandidate, nextResolveErr, nextDecision)
	}
	return parsed, nextCandidate, nextResolveErr, nextDecision, nil
}

func promptBranchAction(
	session *interactiveApplySession,
	use workflow.UseNode,
	parsed actions.ParsedAction,
	candidate *githubresolver.ResolvedRef,
	resolveErr error,
	decision policy.Decision,
) (actions.ParsedAction, *githubresolver.ResolvedRef, error, policy.Decision, error) {
	fmt.Fprintf(session.out, "\nFound branch ref:\n\n  %s:%d\n  %s\n\n", use.File, use.Line, use.Raw)
	if candidate != nil {
		fmt.Fprintf(session.out, "Branch %s resolves to %s.\n\n", candidate.Ref, candidate.SHA)
	}
	choice, err := session.choice("How should sanad manage it?", []interactiveChoice{
		{Key: "p", Label: "Pin current branch head"},
		{Key: "s", Label: "Skip for now"},
		{Key: "d", Label: "Leave as a policy violation"},
	}, "d")
	if err != nil {
		return parsed, candidate, resolveErr, decision, categorizedError{code: exitInternal, err: err}
	}
	switch choice {
	case "p":
		if err := promptPersistBranchTracking(session); err != nil {
			return parsed, candidate, resolveErr, decision, err
		}
		logical := decision.LogicalRef
		if logical == "" {
			logical = parsed.Ref
		}
		current := currentPinnedSHA(parsed)
		if current != "" && candidate != nil && current == candidate.SHA {
			return parsed, candidate, resolveErr, policy.Decision{
				Kind:       policy.DecisionUnchanged,
				Reason:     "current SHA already matches branch head selected interactively",
				CurrentSHA: current,
				NewSHA:     candidate.SHA,
				LogicalRef: logical,
			}, nil
		}
		return parsed, candidate, resolveErr, policy.Decision{
			Kind:       policy.DecisionUpdate,
			Reason:     "branch head pinned by interactive choice",
			CurrentSHA: current,
			NewSHA:     candidate.SHA,
			LogicalRef: logical,
		}, nil
	case "s":
		decision = interactiveSkipDecision(parsed, "skipped by interactive choice")
		return parsed, candidate, nil, decision, nil
	default:
		return parsed, candidate, resolveErr, decision, nil
	}
}

func lockfileEntryForDecision(use workflow.UseNode, parsed actions.ParsedAction, decision policy.Decision, candidate *githubresolver.ResolvedRef, now time.Time) (metadata.LockfileEntry, bool) {
	var pinned string
	switch decision.Kind {
	case policy.DecisionUpdate:
		pinned = decision.NewSHA
	case policy.DecisionUnchanged, policy.DecisionPending:
		pinned = decision.CurrentSHA
	default:
		return metadata.LockfileEntry{}, false
	}
	if decision.LogicalRef == "" {
		return metadata.LockfileEntry{}, false
	}
	if parsed.Owner == "" || parsed.Repo == "" {
		return metadata.LockfileEntry{}, false
	}
	if pinned != "" && !actions.IsFullSHA(pinned) {
		return metadata.LockfileEntry{}, false
	}
	if pinned == "" && (candidate == nil || !actions.IsFullSHA(candidate.SHA)) {
		return metadata.LockfileEntry{}, false
	}

	entry := metadata.LockfileEntry{
		File:       use.File,
		Node:       use.NodePath,
		Owner:      parsed.Owner,
		Repo:       parsed.Repo,
		Path:       parsed.Path,
		Kind:       string(parsed.Kind),
		LogicalRef: decision.LogicalRef,
		PinnedSHA:  pinned,
		ResolvedAt: now.Format(time.RFC3339),
	}
	if decision.Kind == policy.DecisionPending && candidate != nil && actions.IsFullSHA(candidate.SHA) {
		entry.CandidateSHA = candidate.SHA
		seenAt := decision.CandidateSeenAt
		if seenAt.IsZero() {
			seenAt = now
		}
		entry.CandidateSeenAt = seenAt.UTC().Format(time.RFC3339)
	}
	if candidate != nil {
		if ts, source := resolvedTimestamp(*candidate); !ts.IsZero() {
			entry.Timestamp = ts.Format(time.RFC3339)
			entry.TimestampSource = source
		}
	}
	return entry, true
}

func resolvedTimestamp(candidate githubresolver.ResolvedRef) (time.Time, string) {
	switch candidate.Kind {
	case githubresolver.KindTag:
		if candidate.ReleaseTime != nil && !candidate.ReleaseTime.IsZero() {
			return *candidate.ReleaseTime, "release"
		}
		if candidate.TagTime != nil && !candidate.TagTime.IsZero() {
			return *candidate.TagTime, "tag"
		}
	}
	if !candidate.CommitTime.IsZero() {
		return candidate.CommitTime, "commit"
	}
	return time.Time{}, ""
}

type workflowRewrite struct {
	Path string
	Old  []byte
	New  []byte
	Perm os.FileMode
}

func buildWorkflowRewrites(changesByFile map[string][]workflow.RewriteChange, comments config.CommentsConfig) ([]workflowRewrite, error) {
	paths := make([]string, 0, len(changesByFile))
	for path := range changesByFile {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var rewrites []workflowRewrite
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, categorizedError{code: exitFileSystem, err: fmt.Errorf("read workflow %q: %w", path, err)}
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, categorizedError{code: exitFileSystem, err: fmt.Errorf("stat workflow %q: %w", path, err)}
		}
		rewritten, err := workflow.RewriteWorkflowBytes(data, changesByFile[path], workflow.RewriteOptions{WriteMetadataComment: comments.Write})
		if err != nil {
			return nil, categorizedError{code: exitUnsafeRewrite, err: err}
		}
		if bytes.Equal(data, rewritten) {
			continue
		}
		rewrites = append(rewrites, workflowRewrite{
			Path: path,
			Old:  data,
			New:  rewritten,
			Perm: info.Mode().Perm(),
		})
	}
	return rewrites, nil
}

func writeWorkflowRewrites(rewrites []workflowRewrite) error {
	for _, rewrite := range rewrites {
		if err := workflow.AtomicWriteFile(rewrite.Path, rewrite.New, rewrite.Perm); err != nil {
			return categorizedError{code: exitFileSystem, err: err}
		}
	}
	return nil
}

func saveApplyLockfile(entries []metadata.LockfileEntry) error {
	existing, _, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		return categorizedError{code: exitConfig, err: err}
	}
	updated, err := metadata.UpdateLockfile(existing, entries)
	if err != nil {
		return categorizedError{code: exitInternal, err: err}
	}
	if err := metadata.SaveLockfile(metadata.DefaultLockfilePath, updated); err != nil {
		return categorizedError{code: exitFileSystem, err: err}
	}
	return nil
}

func applyLockfileWouldWrite(entries []metadata.LockfileEntry) (bool, error) {
	if len(entries) > 0 {
		return true, nil
	}
	_, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		return false, categorizedError{code: exitConfig, err: err}
	}
	return ok, nil
}

func promptPersistBranchTracking(session *interactiveApplySession) error {
	if session.branchTrackingPersistenceAsked {
		return nil
	}
	session.branchTrackingPersistenceAsked = true
	ok, err := session.confirm("Persist branch tracking in config for future non-interactive runs? [y/N] ")
	if err != nil {
		return categorizedError{code: exitInternal, err: err}
	}
	session.persistBranchTracking = ok
	return nil
}

func saveInteractiveConfigChoices(opts *rootOptions, interactive *interactiveApplySession) error {
	if interactive == nil || !interactive.persistBranchTracking {
		return nil
	}
	if err := config.PersistBranchTracking(opts.configPath); err != nil {
		return categorizedError{code: exitConfig, err: err}
	}
	return nil
}

type interactiveChoice struct {
	Key   string
	Label string
}

func (s *interactiveApplySession) line(prompt string) (string, error) {
	_, _ = fmt.Fprint(s.out, prompt)
	line, err := s.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (s *interactiveApplySession) choice(prompt string, choices []interactiveChoice, defaultChoice string) (string, error) {
	for {
		_, _ = fmt.Fprintln(s.out, prompt)
		for _, choice := range choices {
			_, _ = fmt.Fprintf(s.out, "  %s) %s\n", choice.Key, choice.Label)
		}
		answer, err := s.line("Choice [" + defaultChoice + "]: ")
		if err != nil {
			return "", err
		}
		if answer == "" {
			answer = defaultChoice
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		for _, choice := range choices {
			if answer == choice.Key {
				return answer, nil
			}
		}
		_, _ = fmt.Fprintf(s.out, "Please choose one of %s.\n", choiceKeys(choices))
	}
}

func (s *interactiveApplySession) confirm(prompt string) (bool, error) {
	answer, err := s.line(prompt)
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func choiceKeys(choices []interactiveChoice) string {
	keys := make([]string, 0, len(choices))
	for _, choice := range choices {
		keys = append(keys, choice.Key)
	}
	return strings.Join(keys, "/")
}

func actionSelectorString(parsed actions.ParsedAction) string {
	return actionName(parsed.Owner, parsed.Repo, parsed.Path, "")
}

func interactiveSkipDecision(parsed actions.ParsedAction, reason string) policy.Decision {
	return policy.Decision{
		Kind:       policy.DecisionSkip,
		Reason:     reason,
		CurrentSHA: currentPinnedSHA(parsed),
		LogicalRef: parsed.Ref,
	}
}

func authorizeApply(cmd *cobra.Command, opts *applyOptions, report planReport, rewrites []workflowRewrite, interactive *interactiveApplySession) error {
	if opts.yes && opts.write {
		return nil
	}
	if interactive != nil {
		if err := printPlanTable(cmd, report); err != nil {
			return err
		}
		if len(rewrites) > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			if err := printApplyDiff(cmd.OutOrStdout(), rewrites, styleForCommand(cmd)); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		ok, err := interactive.confirm("Apply these workflow updates? [y/N] ")
		if err != nil {
			return categorizedError{code: exitInternal, err: err}
		}
		if !ok {
			return categorizedError{code: exitPolicy, err: fmt.Errorf("apply cancelled")}
		}
		return nil
	}
	return categorizedError{
		code: exitPolicy,
		err:  fmt.Errorf("refusing to prompt in non-interactive mode; pass --yes --write to apply"),
	}
}

func isTerminal(in io.Reader) bool {
	return isTerminalFile(in)
}

func printApplyDiff(out io.Writer, rewrites []workflowRewrite, style terminalStyle) error {
	for i, rewrite := range rewrites {
		if i > 0 {
			_, _ = fmt.Fprintln(out)
		}
		printStyledLine(out, style, colorDelete, fmt.Sprintf("--- %s\n", rewrite.Path))
		printStyledLine(out, style, colorAdd, fmt.Sprintf("+++ %s\n", rewrite.Path))
		printUnifiedHunk(out, string(rewrite.Old), string(rewrite.New), style)
	}
	return nil
}

func printUnifiedHunk(out io.Writer, oldText string, newText string, style terminalStyle) {
	oldLines := splitDiffLines(oldText)
	newLines := splitDiffLines(newText)
	printStyledLine(out, style, colorHunk, fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))
	for _, op := range diffLineOps(oldLines, newLines) {
		printDiffLine(out, op.prefix, op.line, style)
	}
}

type diffLineOp struct {
	prefix byte
	line   string
}

func diffLineOps(oldLines []string, newLines []string) []diffLineOp {
	lcs := make([][]int, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var ops []diffLineOp
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			ops = append(ops, diffLineOp{prefix: ' ', line: oldLines[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffLineOp{prefix: '-', line: oldLines[i]})
			i++
		default:
			ops = append(ops, diffLineOp{prefix: '+', line: newLines[j]})
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		ops = append(ops, diffLineOp{prefix: '-', line: oldLines[i]})
	}
	for ; j < len(newLines); j++ {
		ops = append(ops, diffLineOp{prefix: '+', line: newLines[j]})
	}
	return ops
}

func splitDiffLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func printDiffLine(out io.Writer, prefix byte, line string, style terminalStyle) {
	role := colorNone
	switch prefix {
	case '+':
		role = colorAdd
	case '-':
		role = colorDelete
	}
	printStyledLine(out, style, role, fmt.Sprintf("%c%s", prefix, line))
	if !strings.HasSuffix(line, "\n") {
		_, _ = fmt.Fprintln(out)
		printStyledLine(out, style, colorHunk, "\\ No newline at end of file\n")
	}
}

func blockersError(blockers []applyBlocker) error {
	var b strings.Builder
	fmt.Fprintf(&b, "apply blocked by %d plan decision(s):", len(blockers))
	for _, blocker := range blockers {
		fmt.Fprintf(&b, "\n%s:%d %s", blocker.File, blocker.Line, blocker.Decision)
		if blocker.Reason != "" {
			fmt.Fprintf(&b, ": %s", blocker.Reason)
		}
	}
	return errors.New(b.String())
}

func blockerExitCode(blockers []applyBlocker) int {
	code := exitPolicy
	for _, blocker := range blockers {
		var resolverErr *githubresolver.ResolverError
		if errors.As(blocker.Err, &resolverErr) {
			switch resolverErr.Kind {
			case githubresolver.ErrorRateLimit:
				return exitRateLimit
			case githubresolver.ErrorForbidden, githubresolver.ErrorGitHubAPI:
				code = maxExitCode(code, exitGitHubAPI)
			case githubresolver.ErrorInvalid, githubresolver.ErrorNotFound:
				code = maxExitCode(code, exitUnresolved)
			}
			continue
		}
		if blocker.Decision == policy.DecisionErrorUnresolved {
			code = maxExitCode(code, exitUnresolved)
		}
	}
	return code
}

func maxExitCode(left int, right int) int {
	if right > left {
		return right
	}
	return left
}

func isBlockingDecision(kind policy.DecisionKind) bool {
	switch kind {
	case policy.DecisionError,
		policy.DecisionErrorInvalid,
		policy.DecisionErrorUnpinned,
		policy.DecisionErrorShortSHA,
		policy.DecisionErrorTagDenied,
		policy.DecisionErrorBranchDenied,
		policy.DecisionErrorReusable,
		policy.DecisionErrorUnresolved,
		policy.DecisionErrorUnsupported:
		return true
	default:
		return false
	}
}

func countWorkflowUpdates(changesByFile map[string][]workflow.RewriteChange) int {
	total := 0
	for _, changes := range changesByFile {
		total += len(changes)
	}
	return total
}
