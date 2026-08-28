package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/metadata"
)

const lockTestNode = "jobs.test.steps[0].uses"

func TestLockStatusJSONReportsRepairablePinDrift(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("b", 40)
	oldSHA := strings.Repeat("a", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	writeTestLockfile(t, lockTestEntry("actions", "checkout", "v4", oldSHA))

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "lock", "status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var report lockReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("lock status output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Summary.Repairable != 1 || report.Summary.Blocking != 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if report.Summary.Statuses[string(metadata.ReconciliationPinDrift)] != 1 {
		t.Fatalf("pin-drift count missing from summary: %#v", report.Summary.Statuses)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Status != metadata.ReconciliationPinDrift {
		t.Fatalf("unexpected diagnostics: %#v", report.Diagnostics)
	}
	if len(report.Entries) != 1 || report.Entries[0].LogicalRef != "v4" {
		t.Fatalf("unexpected entries: %#v", report.Entries)
	}
}

func TestLockRefreshDryRunDoesNotWriteLockfile(t *testing.T) {
	withTempWorkingDir(t)
	sha := strings.Repeat("c", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+sha+" # sanad: ref=v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "refresh", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if _, err := os.Stat(metadata.DefaultLockfilePath); !os.IsNotExist(err) {
		t.Fatalf("lockfile was written during dry-run, stat err = %v", err)
	}
	for _, want := range []string{
		"Planned lockfile changes:",
		"add",
		"Dry run only; no lockfile changed.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestLockRefreshWriteCreatesLockfile(t *testing.T) {
	withTempWorkingDir(t)
	sha := strings.Repeat("d", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+sha+" # sanad: ref=v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "refresh", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	entry := lockfile.Entries[0]
	if entry.Owner != "actions" || entry.Repo != "checkout" || entry.LogicalRef != "v4" || entry.PinnedSHA != sha {
		t.Fatalf("unexpected lockfile entry: %#v", entry)
	}
	if !strings.Contains(out.String(), "Updated .github/sanad.lock.json with 1 change(s).") {
		t.Fatalf("missing write summary:\n%s", out.String())
	}
}

func TestLockRepairWriteFixesPinDriftAndPreservesCandidateHistory(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("e", 40)
	oldSHA := strings.Repeat("f", 40)
	candidateSHA := strings.Repeat("1", 40)
	seenAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	entry := lockTestEntry("actions", "checkout", "v4", oldSHA)
	entry.Candidates = []metadata.CandidateHistoryEntry{{LogicalRef: "v5", SHA: candidateSHA, SeenAt: seenAt}}
	writeTestLockfile(t, entry)

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "repair", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	got := lockfile.Entries[0]
	if got.PinnedSHA != currentSHA {
		t.Fatalf("PinnedSHA = %q, want %q", got.PinnedSHA, currentSHA)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].SHA != candidateSHA || got.Candidates[0].SeenAt != seenAt {
		t.Fatalf("candidate history was not preserved: %#v", got)
	}
}

func TestLockRepairDoesNotRemoveMissingEntries(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("a", 40)
	oldSHA := strings.Repeat("b", 40)
	missingSHA := strings.Repeat("c", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	missing := metadata.LockfileEntry{
		File:       ".github/workflows/removed.yml",
		Node:       "jobs.old.steps[0].uses",
		Owner:      "actions",
		Repo:       "setup-go",
		Kind:       string(actions.KindGitHubAction),
		LogicalRef: "v5",
		PinnedSHA:  missingSHA,
	}
	writeTestLockfile(t, lockTestEntry("actions", "checkout", "v4", oldSHA), missing)

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "repair", "--write", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	lockfile, _, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if len(lockfile.Entries) != 2 {
		t.Fatalf("repair removed a missing entry: %#v", lockfile.Entries)
	}
}

func TestLockRepairScopedWorkflowPreservesOutOfScopeEntries(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("d", 40)
	oldSHA := strings.Repeat("e", 40)
	otherSHA := strings.Repeat("f", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	otherPath := filepath.Join(".github", "workflows", "other.yml")
	if err := os.WriteFile(otherPath, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/setup-go@"+otherSHA+" # sanad: ref=v5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := metadata.LockfileEntry{
		File:       filepath.ToSlash(otherPath),
		Node:       lockTestNode,
		Owner:      "actions",
		Repo:       "setup-go",
		Kind:       string(actions.KindGitHubAction),
		LogicalRef: "v5",
		PinnedSHA:  otherSHA,
	}
	writeTestLockfile(t, lockTestEntry("actions", "checkout", "v4", oldSHA), other)

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "repair", "--workflows", ".github/workflows/ci.yml", "--write", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	lockfile, _, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if len(lockfile.Entries) != 2 {
		t.Fatalf("scoped repair removed out-of-scope entries: %#v", lockfile.Entries)
	}
	for _, entry := range lockfile.Entries {
		if entry.File == filepath.ToSlash(otherPath) && entry.PinnedSHA != otherSHA {
			t.Fatalf("scoped repair changed out-of-scope entry: %#v", entry)
		}
	}
}

func TestLockPruneWriteOnlyRemovesMissingNodes(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("2", 40)
	stalePin := strings.Repeat("3", 40)
	missingPin := strings.Repeat("4", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	missing := metadata.LockfileEntry{
		File:       ".github/workflows/old.yml",
		Node:       "jobs.old.steps[0].uses",
		Owner:      "actions",
		Repo:       "setup-go",
		Kind:       string(actions.KindGitHubAction),
		LogicalRef: "v5",
		PinnedSHA:  missingPin,
	}
	writeTestLockfile(t, lockTestEntry("actions", "checkout", "v4", stalePin), missing)

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "prune", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	got := lockfile.Entries[0]
	if got.File != ".github/workflows/ci.yml" || got.PinnedSHA != stalePin {
		t.Fatalf("prune changed the active stale entry instead of only removing missing nodes: %#v", got)
	}
}

func TestLockRepairWriteBlocksInvalidLockfileEntry(t *testing.T) {
	withTempWorkingDir(t)
	currentSHA := strings.Repeat("5", 40)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	if err := os.MkdirAll(filepath.Dir(metadata.DefaultLockfilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	invalidLockfile := `{
  "version": 1,
  "entries": [
    {
      "file": ".github/workflows/ci.yml",
      "node": "jobs.test.steps[0].uses",
      "owner": "actions",
      "repo": "checkout",
      "kind": "github-action",
      "logical_ref": "v4",
      "pinned_sha": "1234567"
    }
  ]
}
`
	if err := os.WriteFile(metadata.DefaultLockfilePath, []byte(invalidLockfile), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"lock", "repair", "--write"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want blocking lockfile error")
	}
	if ExitCode(err) != exitConfig {
		t.Fatalf("ExitCode = %d, want %d: %v", ExitCode(err), exitConfig, err)
	}
	if got := readFileString(t, metadata.DefaultLockfilePath); got != invalidLockfile {
		t.Fatalf("invalid lockfile was modified:\n%s", got)
	}
	if !strings.Contains(out.String(), "blocking") {
		t.Fatalf("output did not explain blocking diagnostics:\n%s", out.String())
	}
}

func writeTestLockfile(t *testing.T, entries ...metadata.LockfileEntry) {
	t.Helper()
	lockfile, err := metadata.NewLockfile(entries)
	if err != nil {
		t.Fatalf("NewLockfile returned error: %v", err)
	}
	if err := metadata.SaveLockfile(metadata.DefaultLockfilePath, lockfile); err != nil {
		t.Fatalf("SaveLockfile returned error: %v", err)
	}
}

func lockTestEntry(owner string, repo string, logicalRef string, pinnedSHA string) metadata.LockfileEntry {
	return metadata.LockfileEntry{
		File:       ".github/workflows/ci.yml",
		Node:       lockTestNode,
		Owner:      owner,
		Repo:       repo,
		Kind:       string(actions.KindGitHubAction),
		LogicalRef: logicalRef,
		PinnedSHA:  pinnedSHA,
	}
}
