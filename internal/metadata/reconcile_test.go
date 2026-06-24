package metadata

import (
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
)

const reconcileNode = "jobs.test.steps[0].uses"

func TestReconcileLockfileMatchedEntry(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v4", testSHA),
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, ok := result.Use(use.File, use.Node)
	if !ok {
		t.Fatal("reconciled use not found")
	}
	if !got.HasMetadata {
		t.Fatal("HasMetadata = false, want true")
	}
	if got.Metadata.LogicalRef != "v4" || got.Metadata.Source != SourceLockfile {
		t.Fatalf("Metadata = %#v, want lockfile v4", got.Metadata)
	}
	if !got.CandidateHistoryPreservable {
		t.Fatal("CandidateHistoryPreservable = false, want true")
	}
	if !hasDiagnostic(got.Diagnostics, ReconciliationMatched) {
		t.Fatalf("diagnostics = %#v, want matched", got.Diagnostics)
	}
}

func TestReconcileLockfileCommentRefWinsOverLockfile(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v5", testSHA),
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "sanad: ref=v4")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if !got.HasMetadata {
		t.Fatal("HasMetadata = false, want true")
	}
	if got.Metadata.LogicalRef != "v4" || got.Metadata.Source != SourceComment {
		t.Fatalf("Metadata = %#v, want comment v4", got.Metadata)
	}
	if !got.CandidateHistoryPreservable {
		t.Fatal("CandidateHistoryPreservable = false, want true")
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationLogicalRefConflict, true, false)
}

func TestReconcileLockfilePinDriftUsesCurrentWorkflowPin(t *testing.T) {
	workflowSHA := testOtherSHA
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v4", testSHA),
		},
	}
	use := reconcileUse("actions/checkout@"+workflowSHA, "sanad: ref=v4")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if !got.HasMetadata || got.Metadata.LogicalRef != "v4" {
		t.Fatalf("Metadata = %#v, want usable v4 metadata", got.Metadata)
	}
	if !got.CandidateHistoryPreservable {
		t.Fatal("CandidateHistoryPreservable = false, want true")
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationPinDrift, true, false)
}

func TestReconcileLockfileActionMismatchWithoutCommentDropsMetadata(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v4", testSHA),
		},
	}
	use := reconcileUse("actions/setup-go@"+testSHA, "")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if got.HasMetadata {
		t.Fatalf("HasMetadata = true with %#v, want false", got.Metadata)
	}
	if got.CandidateHistoryPreservable {
		t.Fatal("CandidateHistoryPreservable = true, want false")
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationActionMismatch, true, false)
}

func TestReconcileLockfileActionMismatchWithCommentUsesComment(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v4", testSHA),
		},
	}
	use := reconcileUse("actions/setup-go@"+testSHA, "sanad: ref=v5")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if !got.HasMetadata {
		t.Fatal("HasMetadata = false, want true")
	}
	if got.Metadata.LogicalRef != "v5" || got.Metadata.Source != SourceComment {
		t.Fatalf("Metadata = %#v, want comment v5", got.Metadata)
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationActionMismatch, true, false)
}

func TestReconcileLockfileInvalidCommentBlocksLockfileFallback(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			reconcileLockfileEntry("actions", "checkout", "v4", testSHA),
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "sanad: owner=actions")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if got.Error == nil {
		t.Fatal("Error = nil, want invalid comment blocking error")
	}
	if got.HasMetadata {
		t.Fatalf("HasMetadata = true with %#v, want false", got.Metadata)
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationInvalid, false, true)
}

func TestReconcileLockfileMissingNodeReportsRepairableDiagnostic(t *testing.T) {
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			{
				File:       ".github/workflows/old.yml",
				Node:       "jobs.old.steps[0].uses",
				Owner:      "actions",
				Repo:       "checkout",
				Kind:       string(actions.KindGitHubAction),
				LogicalRef: "v4",
				PinnedSHA:  testSHA,
			},
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	assertDiagnostic(t, result.Diagnostics, ReconciliationMissingNode, true, false)
}

func TestReconcileLockfileDuplicateBlocksCurrentNode(t *testing.T) {
	entry := reconcileLockfileEntry("actions", "checkout", "v4", testSHA)
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			entry,
			entry,
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if got.Error == nil {
		t.Fatal("Error = nil, want duplicate blocking error")
	}
	if got.HasMetadata {
		t.Fatalf("HasMetadata = true with %#v, want false", got.Metadata)
	}
	assertDiagnostic(t, got.Diagnostics, ReconciliationDuplicate, false, true)
}

func TestReconcileLockfileInvalidEntryReportsBlockingDiagnostic(t *testing.T) {
	entry := reconcileLockfileEntry("actions", "checkout", "v4", "1234567")
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			entry,
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if got.Error == nil {
		t.Fatal("Error = nil, want invalid blocking error")
	}
	assertDiagnostic(t, result.Diagnostics, ReconciliationInvalid, false, true)
}

func TestReconcileLockfileCandidateDriftWhenHistoryCannotBePreserved(t *testing.T) {
	entry := reconcileLockfileEntry("actions", "checkout", "v4", testSHA)
	entry.Candidates = []CandidateHistoryEntry{{LogicalRef: "v5", SHA: testOtherSHA, SeenAt: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)}}
	lockfile := Lockfile{
		Version: LockfileVersion,
		Entries: []LockfileEntry{
			entry,
		},
	}
	use := reconcileUse("actions/checkout@"+testSHA, "sanad: ref=v5")

	result := ReconcileLockfile(lockfile, true, []ReconcileUse{use})
	got, _ := result.Use(use.File, use.Node)
	if !got.CandidateHistoryPreservable {
		t.Fatal("CandidateHistoryPreservable = false, want true")
	}
}

func reconcileUse(raw string, comment string) ReconcileUse {
	return ReconcileUse{
		File:          ".github/workflows/ci.yml",
		Node:          reconcileNode,
		InlineComment: comment,
		Action:        actions.Parse(raw),
	}
}

func reconcileLockfileEntry(owner string, repo string, logicalRef string, pinnedSHA string) LockfileEntry {
	return LockfileEntry{
		File:       ".github/workflows/ci.yml",
		Node:       reconcileNode,
		Owner:      owner,
		Repo:       repo,
		Kind:       string(actions.KindGitHubAction),
		LogicalRef: logicalRef,
		PinnedSHA:  pinnedSHA,
	}
}

func hasDiagnostic(diagnostics []ReconciliationDiagnostic, status ReconciliationStatus) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Status == status {
			return true
		}
	}
	return false
}

func assertDiagnostic(t *testing.T, diagnostics []ReconciliationDiagnostic, status ReconciliationStatus, repairable bool, blocking bool) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Status != status {
			continue
		}
		if diagnostic.Repairable != repairable {
			t.Fatalf("%s Repairable = %v, want %v", status, diagnostic.Repairable, repairable)
		}
		if diagnostic.Blocking != blocking {
			t.Fatalf("%s Blocking = %v, want %v", status, diagnostic.Blocking, blocking)
		}
		return
	}
	var statuses []string
	for _, diagnostic := range diagnostics {
		statuses = append(statuses, string(diagnostic.Status))
	}
	t.Fatalf("missing diagnostic %q in %s", status, strings.Join(statuses, ", "))
}
