package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/metadata"
	"github.com/MohamedElashri/sanad/internal/workflow"
	"github.com/spf13/cobra"
)

type lockMode string

const (
	lockModeStatus  lockMode = "status"
	lockModeRefresh lockMode = "refresh"
	lockModeRepair  lockMode = "repair"
	lockModePrune   lockMode = "prune"
)

type lockOptions struct {
	workflowPaths []string
	dryRun        bool
	write         bool
}

type lockState struct {
	files          []string
	uses           []workflow.UseNode
	parsedByKey    map[string]actions.ParsedAction
	lockfile       metadata.Lockfile
	hasLockfile    bool
	lockfileValid  bool
	reconciliation metadata.Reconciliation
	scopePaths     []string
}

type lockReport struct {
	Version     int                    `json:"version"`
	Command     lockMode               `json:"command"`
	Lockfile    string                 `json:"lockfile"`
	DryRun      bool                   `json:"dry_run"`
	WouldWrite  bool                   `json:"would_write"`
	Wrote       bool                   `json:"wrote"`
	Summary     lockSummary            `json:"summary"`
	Entries     []lockReportEntry      `json:"entries,omitempty"`
	Diagnostics []lockDiagnosticReport `json:"diagnostics,omitempty"`
	Changes     []lockChange           `json:"changes,omitempty"`
	target      []metadata.LockfileEntry
}

type lockSummary struct {
	Workflows       int            `json:"workflows"`
	Uses            int            `json:"uses"`
	Managed         int            `json:"managed"`
	LockfileEntries int            `json:"lockfile_entries"`
	TargetEntries   int            `json:"target_entries"`
	Matched         int            `json:"matched"`
	Stale           int            `json:"stale"`
	Repairable      int            `json:"repairable"`
	Blocking        int            `json:"blocking"`
	Added           int            `json:"added"`
	Updated         int            `json:"updated"`
	Removed         int            `json:"removed"`
	Unchanged       int            `json:"unchanged"`
	Statuses        map[string]int `json:"statuses,omitempty"`
}

type lockReportEntry struct {
	File                      string                              `json:"file"`
	NodePath                  string                              `json:"node_path"`
	Line                      int                                 `json:"line,omitempty"`
	Raw                       string                              `json:"raw,omitempty"`
	Action                    string                              `json:"action,omitempty"`
	Kind                      actions.ActionKind                  `json:"kind,omitempty"`
	CurrentSHA                string                              `json:"current_sha,omitempty"`
	LogicalRef                string                              `json:"logical_ref,omitempty"`
	MetadataSource            string                              `json:"metadata_source,omitempty"`
	Managed                   bool                                `json:"managed"`
	Status                    []string                            `json:"status"`
	CandidateHistoryPreserved bool                                `json:"candidate_history_preserved"`
	Diagnostics               []metadata.ReconciliationDiagnostic `json:"diagnostics,omitempty"`
}

type lockDiagnosticReport struct {
	Status     metadata.ReconciliationStatus `json:"status"`
	File       string                        `json:"file,omitempty"`
	NodePath   string                        `json:"node_path,omitempty"`
	Line       int                           `json:"line,omitempty"`
	Action     string                        `json:"action,omitempty"`
	Message    string                        `json:"message,omitempty"`
	Repairable bool                          `json:"repairable"`
	Blocking   bool                          `json:"blocking"`
}

type lockChange struct {
	Kind       string `json:"kind"`
	File       string `json:"file"`
	NodePath   string `json:"node_path"`
	Action     string `json:"action,omitempty"`
	LogicalRef string `json:"logical_ref,omitempty"`
	PinnedSHA  string `json:"pinned_sha,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func newLockCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Inspect and repair the Sanad lockfile",
	}
	cmd.AddCommand(
		newLockStatusCommand(opts),
		newLockRefreshCommand(opts),
		newLockRepairCommand(opts),
		newLockPruneCommand(opts),
	)
	return cmd
}

func newLockStatusCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &lockOptions{}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report lockfile reconciliation status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockStatus(cmd, rootOpts, opts)
		},
	}
	cmd.Flags().StringSliceVar(&opts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
	return cmd
}

func newLockRefreshCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &lockOptions{}
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Regenerate lock entries for current managed workflow pins",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockMutation(cmd, rootOpts, opts, lockModeRefresh)
		},
	}
	addLockMutationFlags(cmd, opts)
	return cmd
}

func newLockRepairCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &lockOptions{}
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair safe stale lockfile entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockMutation(cmd, rootOpts, opts, lockModeRepair)
		},
	}
	addLockMutationFlags(cmd, opts)
	return cmd
}

func newLockPruneCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &lockOptions{}
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove lock entries for deleted workflow nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLockMutation(cmd, rootOpts, opts, lockModePrune)
		},
	}
	addLockMutationFlags(cmd, opts)
	return cmd
}

func addLockMutationFlags(cmd *cobra.Command, opts *lockOptions) {
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "show proposed lockfile changes without writing")
	cmd.Flags().BoolVar(&opts.write, "write", false, "write lockfile changes")
	cmd.Flags().StringSliceVar(&opts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")
}

func runLockStatus(cmd *cobra.Command, rootOpts *rootOptions, opts *lockOptions) error {
	state, err := buildLockState(rootOpts, opts.workflowPaths)
	if err != nil {
		return err
	}
	report, err := buildLockReport(state, lockModeStatus, false)
	if err != nil {
		return err
	}
	return printLockReport(cmd, rootOpts, report)
}

func runLockMutation(cmd *cobra.Command, rootOpts *rootOptions, opts *lockOptions, mode lockMode) error {
	if opts.write && opts.dryRun {
		return categorizedError{code: exitConfig, err: fmt.Errorf("--write and --dry-run cannot be used together")}
	}
	state, err := buildLockState(rootOpts, opts.workflowPaths)
	if err != nil {
		return err
	}
	report, err := buildLockReport(state, mode, !opts.write)
	if err != nil {
		return err
	}
	if report.Summary.Blocking > 0 {
		if printErr := printLockReport(cmd, rootOpts, report); printErr != nil {
			return printErr
		}
		return categorizedError{
			code: exitConfig,
			err:  fmt.Errorf("lockfile has %d blocking diagnostic(s)", report.Summary.Blocking),
		}
	}
	if opts.write && report.WouldWrite {
		if err := saveLockEntries(state.lockfile, report.target); err != nil {
			return err
		}
		report.Wrote = true
	}
	return printLockReport(cmd, rootOpts, report)
}

func buildLockState(rootOpts *rootOptions, workflowPaths []string) (lockState, error) {
	cfg, err := loadConfig(rootOpts)
	if err != nil {
		return lockState{}, err
	}

	paths := cfg.WorkflowPaths
	if len(workflowPaths) > 0 {
		paths = workflowPaths
	}
	files, err := workflow.DiscoverWorkflowFiles(paths)
	if err != nil {
		return lockState{}, err
	}
	uses, err := workflow.ExtractUsesFromFiles(files)
	if err != nil {
		return lockState{}, err
	}
	lockfile, hasLockfile, err := loadLockfileForReconciliation(metadata.DefaultLockfilePath)
	if err != nil {
		return lockState{}, categorizedError{code: exitConfig, err: err}
	}

	reconcileUses := make([]metadata.ReconcileUse, 0, len(uses))
	parsedByKey := make(map[string]actions.ParsedAction, len(uses))
	for _, use := range uses {
		parsed := actions.Parse(use.Raw)
		key := metadata.Key(use.File, use.NodePath)
		parsedByKey[key] = parsed
		reconcileUses = append(reconcileUses, metadata.ReconcileUse{
			File:          use.File,
			Node:          use.NodePath,
			InlineComment: use.InlineComment,
			Action:        parsed,
		})
	}

	lockfileValid := false
	if hasLockfile {
		lockfileValid = metadata.ValidateLockfile(lockfile) == nil
	}

	reconciliationLockfile := lockfile
	if len(workflowPaths) > 0 {
		reconciliationLockfile.Entries = filterLockEntriesByScope(lockfile.Entries, workflowPaths, true)
	}

	return lockState{
		files:          files,
		uses:           uses,
		parsedByKey:    parsedByKey,
		lockfile:       lockfile,
		hasLockfile:    hasLockfile,
		lockfileValid:  lockfileValid,
		reconciliation: metadata.ReconcileLockfile(reconciliationLockfile, hasLockfile, reconcileUses),
		scopePaths:     append([]string(nil), workflowPaths...),
	}, nil
}

func loadLockfileForReconciliation(path string) (metadata.Lockfile, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return metadata.Lockfile{}, false, nil
		}
		return metadata.Lockfile{}, false, fmt.Errorf("load lockfile %q: %w", path, err)
	}
	var lockfile metadata.Lockfile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return metadata.Lockfile{}, true, fmt.Errorf("load lockfile %q: invalid JSON: %w", path, err)
	}
	migrated, err := metadata.MigrateLockfile(lockfile)
	if err != nil {
		return lockfile, true, nil
	}
	return migrated, true, nil
}

func buildLockReport(state lockState, mode lockMode, dryRun bool) (lockReport, error) {
	target, err := targetLockEntries(state, mode)
	if err != nil {
		return lockReport{}, err
	}
	changes := lockChanges(state, target)
	report := lockReport{
		Version:     1,
		Command:     mode,
		Lockfile:    metadata.DefaultLockfilePath,
		DryRun:      dryRun,
		WouldWrite:  len(changes) > 0,
		Summary:     summarizeLockState(state, target, changes),
		Entries:     lockReportEntries(state),
		Diagnostics: lockDiagnosticReports(state),
		Changes:     changes,
		target:      target,
	}
	return report, nil
}

func targetLockEntries(state lockState, mode lockMode) ([]metadata.LockfileEntry, error) {
	switch mode {
	case lockModePrune:
		return pruneLockEntries(state), nil
	case lockModeRepair:
		return repairLockEntries(state)
	default:
		return refreshLockEntries(state)
	}
}

func refreshLockEntries(state lockState) ([]metadata.LockfileEntry, error) {
	entries := make([]metadata.LockfileEntry, 0, len(state.uses)+len(state.lockfile.Entries))
	if len(state.scopePaths) > 0 {
		entries = append(entries, filterLockEntriesByScope(state.lockfile.Entries, state.scopePaths, false)...)
	}
	for _, use := range state.uses {
		key := metadata.Key(use.File, use.NodePath)
		reconciled, ok := state.reconciliation.Use(use.File, use.NodePath)
		if !ok {
			continue
		}
		entry, ok := lockEntryForReconciledUse(use, state.parsedByKey[key], reconciled)
		if ok {
			entries = append(entries, entry)
		}
	}
	lockfile, err := metadata.NewLockfile(entries)
	if err != nil {
		return nil, categorizedError{code: exitInternal, err: err}
	}
	return lockfile.Entries, nil
}

func repairLockEntries(state lockState) ([]metadata.LockfileEntry, error) {
	entries, err := refreshLockEntries(state)
	if err != nil {
		return nil, err
	}
	present := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		present[metadata.Key(entry.File, entry.Node)] = struct{}{}
	}
	missing := make(map[string]struct{})
	for _, diagnostic := range state.reconciliation.Diagnostics {
		if diagnostic.Status == metadata.ReconciliationMissingNode {
			missing[metadata.Key(diagnostic.File, diagnostic.Node)] = struct{}{}
		}
	}
	for _, entry := range state.lockfile.Entries {
		key := metadata.Key(entry.File, entry.Node)
		if _, ok := missing[key]; !ok {
			continue
		}
		if _, ok := present[key]; ok {
			continue
		}
		entries = append(entries, entry)
	}
	lockfile, err := metadata.NewLockfile(entries)
	if err != nil {
		return nil, categorizedError{code: exitInternal, err: err}
	}
	return lockfile.Entries, nil
}

func filterLockEntriesByScope(entries []metadata.LockfileEntry, scopePaths []string, include bool) []metadata.LockfileEntry {
	filtered := make([]metadata.LockfileEntry, 0, len(entries))
	for _, entry := range entries {
		if lockEntryInWorkflowScope(entry, scopePaths) == include {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func lockEntryInWorkflowScope(entry metadata.LockfileEntry, scopePaths []string) bool {
	entryPath := filepath.Clean(entry.File)
	for _, scopePath := range scopePaths {
		scopePath = filepath.Clean(scopePath)
		relative, err := filepath.Rel(scopePath, entryPath)
		if err != nil {
			continue
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func pruneLockEntries(state lockState) []metadata.LockfileEntry {
	if !state.hasLockfile {
		return nil
	}
	missing := make(map[string]struct{})
	for _, diagnostic := range state.reconciliation.Diagnostics {
		if diagnostic.Status == metadata.ReconciliationMissingNode {
			missing[metadata.Key(diagnostic.File, diagnostic.Node)] = struct{}{}
		}
	}
	entries := make([]metadata.LockfileEntry, 0, len(state.lockfile.Entries))
	for _, entry := range state.lockfile.Entries {
		if _, ok := missing[metadata.Key(entry.File, entry.Node)]; ok {
			continue
		}
		entries = append(entries, entry)
	}
	metadata.SortLockfileEntries(entries)
	return entries
}

func lockEntryForReconciledUse(use workflow.UseNode, parsed actions.ParsedAction, reconciled metadata.ReconciledUse) (metadata.LockfileEntry, bool) {
	if reconciled.Error != nil || !reconciled.HasMetadata {
		return metadata.LockfileEntry{}, false
	}
	if !isLockablePinnedAction(parsed) {
		return metadata.LockfileEntry{}, false
	}
	if reconciled.Metadata.LogicalRef == "" {
		return metadata.LockfileEntry{}, false
	}

	entry := metadata.LockfileEntry{
		File:       use.File,
		Node:       use.NodePath,
		Owner:      parsed.Owner,
		Repo:       parsed.Repo,
		Path:       parsed.Path,
		Kind:       string(parsed.Kind),
		LogicalRef: reconciled.Metadata.LogicalRef,
		PinnedSHA:  parsed.Ref,
	}
	if reconciled.HasEntry && lockEntrySameCurrentPin(reconciled.Entry, entry) {
		entry.ResolvedAt = reconciled.Entry.ResolvedAt
		entry.Timestamp = reconciled.Entry.Timestamp
		entry.TimestampSource = reconciled.Entry.TimestampSource
	}
	if reconciled.HasEntry && reconciled.CandidateHistoryPreservable {
		entry.Candidates = append([]metadata.CandidateHistoryEntry(nil), reconciled.Entry.Candidates...)
	}
	return entry, true
}

func isLockablePinnedAction(parsed actions.ParsedAction) bool {
	if !parsed.Valid || !parsed.Pinned || parsed.Owner == "" || parsed.Repo == "" {
		return false
	}
	switch parsed.Kind {
	case actions.KindGitHubAction, actions.KindReusableWorkflow:
		return true
	default:
		return false
	}
}

func lockEntrySameCurrentPin(left metadata.LockfileEntry, right metadata.LockfileEntry) bool {
	return left.Owner == right.Owner &&
		left.Repo == right.Repo &&
		left.Path == right.Path &&
		left.Kind == right.Kind &&
		left.LogicalRef == right.LogicalRef &&
		left.PinnedSHA == right.PinnedSHA
}

func lockChanges(state lockState, target []metadata.LockfileEntry) []lockChange {
	if !state.hasLockfile {
		return lockChangesForMissingLockfile(target)
	}
	if !state.lockfileValid {
		return nil
	}

	existing := metadata.NormalizeLockfile(state.lockfile)
	existingByKey := make(map[string]metadata.LockfileEntry, len(existing.Entries))
	targetByKey := make(map[string]metadata.LockfileEntry, len(target))
	for _, entry := range existing.Entries {
		existingByKey[metadata.Key(entry.File, entry.Node)] = entry
	}
	for _, entry := range target {
		targetByKey[metadata.Key(entry.File, entry.Node)] = entry
	}

	var changes []lockChange
	for _, entry := range target {
		key := metadata.Key(entry.File, entry.Node)
		existingEntry, ok := existingByKey[key]
		if !ok {
			changes = append(changes, lockChangeFromEntry("add", entry, "current managed workflow pin is missing from the lockfile"))
			continue
		}
		if !reflect.DeepEqual(existingEntry, entry) {
			changes = append(changes, lockChangeFromEntry("update", entry, "lockfile entry will be reconciled with the current workflow pin"))
		}
	}
	for _, entry := range existing.Entries {
		if _, ok := targetByKey[metadata.Key(entry.File, entry.Node)]; ok {
			continue
		}
		changes = append(changes, lockChangeFromEntry("remove", entry, "lockfile entry is no longer active"))
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].File != changes[j].File {
			return changes[i].File < changes[j].File
		}
		if changes[i].NodePath != changes[j].NodePath {
			return changes[i].NodePath < changes[j].NodePath
		}
		return changes[i].Kind < changes[j].Kind
	})
	return changes
}

func lockChangesForMissingLockfile(target []metadata.LockfileEntry) []lockChange {
	changes := make([]lockChange, 0, len(target))
	for _, entry := range target {
		changes = append(changes, lockChangeFromEntry("add", entry, "lockfile does not exist"))
	}
	return changes
}

func lockChangeFromEntry(kind string, entry metadata.LockfileEntry, reason string) lockChange {
	return lockChange{
		Kind:       kind,
		File:       entry.File,
		NodePath:   entry.Node,
		Action:     actionName(entry.Owner, entry.Repo, entry.Path, ""),
		LogicalRef: entry.LogicalRef,
		PinnedSHA:  entry.PinnedSHA,
		Reason:     reason,
	}
}

func summarizeLockState(state lockState, target []metadata.LockfileEntry, changes []lockChange) lockSummary {
	summary := lockSummary{
		Workflows:       len(state.files),
		Uses:            len(state.uses),
		LockfileEntries: len(state.lockfile.Entries),
		TargetEntries:   len(target),
		Statuses:        make(map[string]int),
	}
	for _, use := range state.uses {
		key := metadata.Key(use.File, use.NodePath)
		reconciled, ok := state.reconciliation.Use(use.File, use.NodePath)
		if !ok {
			continue
		}
		if entry, ok := lockEntryForReconciledUse(use, state.parsedByKey[key], reconciled); ok && entry.LogicalRef != "" {
			summary.Managed++
		}
	}
	for _, diagnostic := range state.reconciliation.Diagnostics {
		summary.Statuses[string(diagnostic.Status)]++
		if diagnostic.Status == metadata.ReconciliationMatched {
			summary.Matched++
		} else {
			summary.Stale++
		}
		if diagnostic.Repairable {
			summary.Repairable++
		}
		if diagnostic.Blocking {
			summary.Blocking++
		}
	}
	for _, change := range changes {
		switch change.Kind {
		case "add":
			summary.Added++
		case "update":
			summary.Updated++
		case "remove":
			summary.Removed++
		}
	}
	summary.Unchanged = len(target) - summary.Added - summary.Updated
	if summary.Unchanged < 0 {
		summary.Unchanged = 0
	}
	if len(summary.Statuses) == 0 {
		summary.Statuses = nil
	}
	return summary
}

func lockReportEntries(state lockState) []lockReportEntry {
	entries := make([]lockReportEntry, 0, len(state.uses))
	for _, use := range state.uses {
		key := metadata.Key(use.File, use.NodePath)
		reconciled, ok := state.reconciliation.Use(use.File, use.NodePath)
		if !ok {
			continue
		}
		parsed := state.parsedByKey[key]
		entry, managed := lockEntryForReconciledUse(use, parsed, reconciled)
		if !managed && !reconciled.HasEntry && len(reconciled.Diagnostics) == 0 {
			continue
		}
		reportEntry := lockReportEntry{
			File:                      use.File,
			NodePath:                  use.NodePath,
			Line:                      use.Line,
			Raw:                       use.Raw,
			Action:                    parsedActionName(parsed, use.Raw),
			Kind:                      parsed.Kind,
			CurrentSHA:                currentPinnedSHA(parsed),
			Managed:                   managed,
			Status:                    lockEntryStatuses(reconciled, managed),
			CandidateHistoryPreserved: managed && len(entry.Candidates) > 0,
			Diagnostics:               reconciled.Diagnostics,
		}
		if reconciled.HasMetadata {
			reportEntry.LogicalRef = reconciled.Metadata.LogicalRef
			reportEntry.MetadataSource = string(reconciled.Metadata.Source)
		}
		entries = append(entries, reportEntry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].File != entries[j].File {
			return entries[i].File < entries[j].File
		}
		return entries[i].Line < entries[j].Line
	})
	return entries
}

func lockEntryStatuses(reconciled metadata.ReconciledUse, managed bool) []string {
	statuses := make([]string, 0, len(reconciled.Diagnostics))
	for _, diagnostic := range reconciled.Diagnostics {
		statuses = append(statuses, string(diagnostic.Status))
	}
	if len(statuses) > 0 {
		return statuses
	}
	if managed {
		if reconciled.HasEntry {
			return []string{"managed"}
		}
		return []string{"missing-entry"}
	}
	if reconciled.HasEntry {
		return []string{"unmanaged"}
	}
	return nil
}

func lockDiagnosticReports(state lockState) []lockDiagnosticReport {
	useByKey := make(map[string]workflow.UseNode, len(state.uses))
	for _, use := range state.uses {
		useByKey[metadata.Key(use.File, use.NodePath)] = use
	}
	reports := make([]lockDiagnosticReport, 0, len(state.reconciliation.Diagnostics))
	for _, diagnostic := range state.reconciliation.Diagnostics {
		report := lockDiagnosticReport{
			Status:     diagnostic.Status,
			File:       diagnostic.File,
			NodePath:   diagnostic.Node,
			Message:    diagnostic.Message,
			Repairable: diagnostic.Repairable,
			Blocking:   diagnostic.Blocking,
		}
		if use, ok := useByKey[metadata.Key(diagnostic.File, diagnostic.Node)]; ok {
			report.Line = use.Line
			report.Action = parsedActionName(actions.Parse(use.Raw), use.Raw)
		}
		reports = append(reports, report)
	}
	return reports
}

func parsedActionName(parsed actions.ParsedAction, fallback string) string {
	return actionName(parsed.Owner, parsed.Repo, parsed.Path, fallback)
}

func printLockReport(cmd *cobra.Command, rootOpts *rootOptions, report lockReport) error {
	switch rootOpts.format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "table":
		return printLockTable(cmd, report)
	default:
		return fmt.Errorf("unsupported format %q: expected table or json", rootOpts.format)
	}
}

func printLockTable(cmd *cobra.Command, report lockReport) error {
	style := styleForCommand(cmd)
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Lockfile: %s\n", report.Lockfile)
	_, _ = fmt.Fprintf(
		out,
		"%d workflow file(s), %d uses node(s), %d managed lock entr%s, %d existing lock entr%s.\n",
		report.Summary.Workflows,
		report.Summary.Uses,
		report.Summary.Managed,
		pluralY(report.Summary.Managed),
		report.Summary.LockfileEntries,
		pluralY(report.Summary.LockfileEntries),
	)
	_, _ = fmt.Fprintf(
		out,
		"%s matched, %s stale, %s repairable, %s blocking.\n",
		style.Wrapf(colorSuccess, "%d", report.Summary.Matched),
		style.Wrapf(colorWarning, "%d", report.Summary.Stale),
		style.Wrapf(colorWarning, "%d", report.Summary.Repairable),
		style.Wrapf(colorDanger, "%d", report.Summary.Blocking),
	)

	if len(report.Diagnostics) > 0 {
		_, _ = fmt.Fprintln(out, "\nDiagnostics:")
		rows := make([]styledTableRow, 0, len(report.Diagnostics))
		for _, diagnostic := range report.Diagnostics {
			rows = append(rows, styledTableRow{
				{Text: emptyDash(diagnostic.File), Role: colorFile},
				{Text: lineOrDash(diagnostic.Line), Role: colorLine},
				{Text: emptyDash(diagnostic.Action)},
				{Text: string(diagnostic.Status), Role: lockStatusColor(diagnostic.Blocking, diagnostic.Repairable, diagnostic.Status)},
				{Text: yesNo(diagnostic.Repairable), Role: boolColorRole(diagnostic.Repairable)},
				{Text: yesNo(diagnostic.Blocking), Role: boolColorRole(!diagnostic.Blocking)},
				{Text: emptyDash(diagnostic.Message), Role: colorReason},
			})
		}
		if err := printStyledTable(out, style, []string{"FILE", "LINE", "ACTION", "STATUS", "REPAIRABLE", "BLOCKING", "MESSAGE"}, rows); err != nil {
			return err
		}
	}

	if len(report.Changes) > 0 {
		_, _ = fmt.Fprintln(out, "\nPlanned lockfile changes:")
		rows := make([]styledTableRow, 0, len(report.Changes))
		for _, change := range report.Changes {
			rows = append(rows, styledTableRow{
				{Text: change.Kind, Role: lockChangeColor(change.Kind)},
				{Text: change.File, Role: colorFile},
				{Text: change.Action},
				{Text: shortSHAOrDash(change.PinnedSHA), Role: colorInfo},
				{Text: emptyDash(change.LogicalRef), Role: colorInfo},
				{Text: emptyDash(change.Reason), Role: colorReason},
			})
		}
		if err := printStyledTable(out, style, []string{"CHANGE", "FILE", "ACTION", "PIN", "REF", "REASON"}, rows); err != nil {
			return err
		}
	}

	switch {
	case report.Wrote:
		_, _ = fmt.Fprintf(out, "\nUpdated %s with %d change(s).\n", report.Lockfile, len(report.Changes))
	case report.Summary.Blocking > 0:
		_, _ = fmt.Fprintln(out, "\nLockfile has blocking diagnostics; no changes written.")
	case report.Command == lockModeStatus && report.WouldWrite:
		_, _ = fmt.Fprintln(out, "\nRun `sanad lock repair --write` or `sanad lock refresh --write` to update the lockfile.")
	case report.DryRun && report.WouldWrite:
		_, _ = fmt.Fprintf(out, "\nDry run only; no lockfile changed. Add --write to update %s.\n", report.Lockfile)
	case !report.WouldWrite:
		_, _ = fmt.Fprintln(out, "\nNo lockfile changes needed.")
	}
	return nil
}

func lineOrDash(line int) string {
	if line <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", line)
}

func pluralY(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}

func lockStatusColor(blocking bool, repairable bool, status metadata.ReconciliationStatus) colorRole {
	if blocking {
		return colorDanger
	}
	if repairable {
		return colorWarning
	}
	if status == metadata.ReconciliationMatched {
		return colorSuccess
	}
	return colorMuted
}

func lockChangeColor(kind string) colorRole {
	switch kind {
	case "add":
		return colorAdd
	case "update":
		return colorWarning
	case "remove":
		return colorDelete
	default:
		return colorNone
	}
}

func saveLockEntries(existing metadata.Lockfile, entries []metadata.LockfileEntry) error {
	updated, err := metadata.UpdateLockfile(existing, entries)
	if err != nil {
		return categorizedError{code: exitInternal, err: err}
	}
	if err := metadata.SaveLockfile(metadata.DefaultLockfilePath, updated); err != nil {
		return categorizedError{code: exitFileSystem, err: err}
	}
	return nil
}
