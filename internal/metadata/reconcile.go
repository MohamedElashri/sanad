package metadata

import (
	"fmt"
	"sort"

	"github.com/MohamedElashri/sanad/internal/actions"
)

type ReconciliationStatus string

const (
	ReconciliationMatched            ReconciliationStatus = "matched"
	ReconciliationMissingNode        ReconciliationStatus = "missing-node"
	ReconciliationActionMismatch     ReconciliationStatus = "action-mismatch"
	ReconciliationPinDrift           ReconciliationStatus = "pin-drift"
	ReconciliationLogicalRefConflict ReconciliationStatus = "logical-ref-conflict"
	ReconciliationCandidateDrift     ReconciliationStatus = "candidate-drift"
	ReconciliationInvalid            ReconciliationStatus = "invalid"
	ReconciliationDuplicate          ReconciliationStatus = "duplicate"
)

type ReconcileUse struct {
	File          string               `json:"file"`
	Node          string               `json:"node"`
	InlineComment string               `json:"inline_comment,omitempty"`
	Action        actions.ParsedAction `json:"action"`
}

type ReconciliationDiagnostic struct {
	Status     ReconciliationStatus `json:"status"`
	File       string               `json:"file,omitempty"`
	Node       string               `json:"node,omitempty"`
	Message    string               `json:"message,omitempty"`
	Repairable bool                 `json:"repairable"`
	Blocking   bool                 `json:"blocking"`
}

type ReconciledUse struct {
	Use                         ReconcileUse
	Metadata                    Metadata
	HasMetadata                 bool
	Entry                       LockfileEntry
	HasEntry                    bool
	CandidateHistoryPreservable bool
	Diagnostics                 []ReconciliationDiagnostic
	Error                       error
}

type Reconciliation struct {
	Uses        map[string]ReconciledUse
	Diagnostics []ReconciliationDiagnostic
}

func (r Reconciliation) Use(file string, node string) (ReconciledUse, bool) {
	value, ok := r.Uses[Key(file, node)]
	return value, ok
}

func (r Reconciliation) MetadataFor(file string, node string) (Metadata, bool, error) {
	value, ok := r.Use(file, node)
	if !ok {
		return Metadata{}, false, nil
	}
	return value.Metadata, value.HasMetadata, value.Error
}

func ReconcileLockfile(lockfile Lockfile, hasLockfile bool, uses []ReconcileUse) Reconciliation {
	result := Reconciliation{
		Uses: make(map[string]ReconciledUse, len(uses)),
	}

	for _, use := range uses {
		result.Uses[Key(use.File, use.Node)] = ReconciledUse{Use: use}
	}

	if !hasLockfile {
		reconcileComments(&result, uses, nil)
		sortReconciliation(&result)
		return result
	}

	if lockfile.Version != LockfileVersion {
		result.addDiagnostic(ReconciliationDiagnostic{
			Status:   ReconciliationInvalid,
			Message:  fmt.Sprintf("unsupported lockfile version %d: expected %d", lockfile.Version, LockfileVersion),
			Blocking: true,
		})
		reconcileComments(&result, uses, nil)
		sortReconciliation(&result)
		return result
	}

	entriesByKey := make(map[string][]LockfileEntry, len(lockfile.Entries))
	invalidKeys := make(map[string]struct{})
	for i, entry := range lockfile.Entries {
		if err := validateLockfileEntry(entry, i); err != nil {
			key := Key(entry.File, entry.Node)
			invalidKeys[key] = struct{}{}
			result.addDiagnostic(ReconciliationDiagnostic{
				Status:   ReconciliationInvalid,
				File:     entry.File,
				Node:     entry.Node,
				Message:  err.Error(),
				Blocking: true,
			})
			continue
		}
		entriesByKey[Key(entry.File, entry.Node)] = append(entriesByKey[Key(entry.File, entry.Node)], entry)
	}

	for key, entries := range entriesByKey {
		if len(entries) <= 1 {
			continue
		}
		for _, entry := range entries {
			result.addDiagnostic(ReconciliationDiagnostic{
				Status:   ReconciliationDuplicate,
				File:     entry.File,
				Node:     entry.Node,
				Message:  fmt.Sprintf("duplicate lockfile entry for %s %s", entry.File, entry.Node),
				Blocking: true,
			})
		}
		invalidKeys[key] = struct{}{}
	}

	reconcileComments(&result, uses, entriesByKey)
	for _, use := range uses {
		key := Key(use.File, use.Node)
		if _, invalid := invalidKeys[key]; invalid {
			diagnostic := ReconciliationDiagnostic{
				Status:   ReconciliationDuplicate,
				File:     use.File,
				Node:     use.Node,
				Message:  fmt.Sprintf("duplicate lockfile entry for %s %s", use.File, use.Node),
				Blocking: true,
			}
			if len(entriesByKey[key]) == 1 {
				diagnostic.Status = ReconciliationInvalid
				diagnostic.Message = "invalid lockfile entry for current workflow node"
			}
			value := result.Uses[key]
			value.Metadata = Metadata{}
			value.HasMetadata = false
			value.CandidateHistoryPreservable = false
			result.Uses[key] = value
			result.addUseDiagnostic(key, diagnostic)
			continue
		}

		entries := entriesByKey[key]
		if len(entries) == 0 {
			continue
		}
		reconcileEntry(&result, use, entries[0])
	}

	for key, entries := range entriesByKey {
		if _, invalid := invalidKeys[key]; invalid {
			continue
		}
		if _, ok := result.Uses[key]; ok {
			continue
		}
		for _, entry := range entries {
			result.addDiagnostic(ReconciliationDiagnostic{
				Status:     ReconciliationMissingNode,
				File:       entry.File,
				Node:       entry.Node,
				Message:    fmt.Sprintf("lockfile entry for %s %s has no matching workflow uses node", entry.File, entry.Node),
				Repairable: true,
			})
		}
	}

	sortReconciliation(&result)
	return result
}

func reconcileComments(result *Reconciliation, uses []ReconcileUse, entriesByKey map[string][]LockfileEntry) {
	for _, use := range uses {
		key := Key(use.File, use.Node)
		value := result.Uses[key]
		comment, err := ParseComment(use.InlineComment)
		if err != nil {
			diagnostic := ReconciliationDiagnostic{
				Status:   ReconciliationInvalid,
				File:     use.File,
				Node:     use.Node,
				Message:  err.Error(),
				Blocking: true,
			}
			value.Error = err
			value.Diagnostics = append(value.Diagnostics, diagnostic)
			result.Uses[key] = value
			result.addDiagnostic(diagnostic)
			continue
		}
		if !comment.Present {
			continue
		}
		value.Metadata = comment.Metadata
		value.HasMetadata = true
		result.Uses[key] = value

		if entriesByKey == nil || len(entriesByKey[key]) != 1 {
			continue
		}
		entry := entriesByKey[key][0]
		if entry.LogicalRef != comment.Metadata.LogicalRef {
			result.addUseDiagnostic(key, ReconciliationDiagnostic{
				Status:     ReconciliationLogicalRefConflict,
				File:       use.File,
				Node:       use.Node,
				Message:    fmt.Sprintf("lockfile ref %q disagrees with comment ref %q", entry.LogicalRef, comment.Metadata.LogicalRef),
				Repairable: true,
			})
		}
	}
}

func reconcileEntry(result *Reconciliation, use ReconcileUse, entry LockfileEntry) {
	key := Key(use.File, use.Node)
	value := result.Uses[key]
	value.Entry = entry
	value.HasEntry = true
	if value.Error != nil {
		result.Uses[key] = value
		return
	}
	addDiagnostic := func(diagnostic ReconciliationDiagnostic) {
		value.Diagnostics = append(value.Diagnostics, diagnostic)
		result.addDiagnostic(diagnostic)
	}

	actionMatch := !use.Action.Valid || lockfileEntryMatchesAction(entry, use.Action)
	if use.Action.Valid && !actionMatch {
		addDiagnostic(ReconciliationDiagnostic{
			Status:     ReconciliationActionMismatch,
			File:       use.File,
			Node:       use.Node,
			Message:    fmt.Sprintf("lockfile action %s does not match workflow action %s", lockfileActionIdentity(entry), parsedActionIdentity(use.Action)),
			Repairable: true,
		})
		if !value.HasMetadata {
			value.Metadata = Metadata{}
			value.HasMetadata = false
		}
	} else if !value.HasMetadata {
		value.Metadata = Metadata{
			LogicalRef: entry.LogicalRef,
			Source:     SourceLockfile,
		}
		value.HasMetadata = true
	}

	if use.Action.Valid && use.Action.Pinned && entry.PinnedSHA != "" && use.Action.Ref != entry.PinnedSHA {
		addDiagnostic(ReconciliationDiagnostic{
			Status:     ReconciliationPinDrift,
			File:       use.File,
			Node:       use.Node,
			Message:    fmt.Sprintf("workflow pin %q does not match lockfile pin %q", use.Action.Ref, entry.PinnedSHA),
			Repairable: true,
		})
	}

	value.CandidateHistoryPreservable = actionMatch && value.HasMetadata && value.Metadata.LogicalRef == entry.LogicalRef
	if entry.CandidateSHA != "" && !value.CandidateHistoryPreservable {
		addDiagnostic(ReconciliationDiagnostic{
			Status:     ReconciliationCandidateDrift,
			File:       use.File,
			Node:       use.Node,
			Message:    "lockfile candidate history does not match the current workflow action metadata",
			Repairable: true,
		})
	}

	if actionMatch && value.HasMetadata && value.Metadata.LogicalRef == entry.LogicalRef && !hasStaleDiagnostics(value.Diagnostics) {
		addDiagnostic(ReconciliationDiagnostic{
			Status: ReconciliationMatched,
			File:   use.File,
			Node:   use.Node,
		})
	}
	result.Uses[key] = value
}

func (r *Reconciliation) addUseDiagnostic(key string, diagnostic ReconciliationDiagnostic) {
	value := r.Uses[key]
	value.Diagnostics = append(value.Diagnostics, diagnostic)
	if diagnostic.Blocking && value.Error == nil {
		value.Error = fmt.Errorf("%s", diagnostic.Message)
	}
	r.Uses[key] = value
	r.addDiagnostic(diagnostic)
}

func (r *Reconciliation) addDiagnostic(diagnostic ReconciliationDiagnostic) {
	r.Diagnostics = append(r.Diagnostics, diagnostic)
}

func lockfileEntryMatchesAction(entry LockfileEntry, action actions.ParsedAction) bool {
	return entry.Owner == action.Owner &&
		entry.Repo == action.Repo &&
		entry.Path == action.Path &&
		entry.Kind == string(action.Kind)
}

func lockfileActionIdentity(entry LockfileEntry) string {
	value := entry.Owner + "/" + entry.Repo
	if entry.Path != "" {
		value += "/" + entry.Path
	}
	return value
}

func parsedActionIdentity(action actions.ParsedAction) string {
	value := action.Owner + "/" + action.Repo
	if action.Path != "" {
		value += "/" + action.Path
	}
	return value
}

func hasStaleDiagnostics(diagnostics []ReconciliationDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		switch diagnostic.Status {
		case ReconciliationActionMismatch,
			ReconciliationPinDrift,
			ReconciliationLogicalRefConflict,
			ReconciliationCandidateDrift,
			ReconciliationInvalid,
			ReconciliationDuplicate:
			return true
		}
	}
	return false
}

func sortReconciliation(result *Reconciliation) {
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		return diagnosticLess(result.Diagnostics[i], result.Diagnostics[j])
	})
	for key, value := range result.Uses {
		sort.SliceStable(value.Diagnostics, func(i, j int) bool {
			return diagnosticLess(value.Diagnostics[i], value.Diagnostics[j])
		})
		result.Uses[key] = value
	}
}

func diagnosticLess(left ReconciliationDiagnostic, right ReconciliationDiagnostic) bool {
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Node != right.Node {
		return left.Node < right.Node
	}
	if left.Status != right.Status {
		return left.Status < right.Status
	}
	return left.Message < right.Message
}
