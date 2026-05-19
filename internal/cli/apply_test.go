package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/MohamedElashri/sanad/internal/githubresolver"
	"github.com/MohamedElashri/sanad/internal/metadata"
)

func TestApplyDryRunPrintsDiffWithoutWritingFiles(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("a", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed during dry-run\nwant:\n%s\ngot:\n%s", original, got)
	}
	if _, err := os.Stat(metadata.DefaultLockfilePath); !os.IsNotExist(err) {
		t.Fatalf("lockfile was written during dry-run, stat err = %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"--- .github/workflows/ci.yml",
		"@@ -1,4 +1,4 @@",
		"-      - uses: actions/checkout@v4",
		"+      - uses: actions/checkout@" + sha + " # sanad: ref=v4",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyYesWriteRewritesWorkflowAndUpdatesLockfile(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("b", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+sha+" # sanad: ref=v4") {
		t.Fatalf("workflow was not rewritten as expected:\n%s", got)
	}

	var lockfile metadata.Lockfile
	lockBytes, err := os.ReadFile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if err := json.Unmarshal(lockBytes, &lockfile); err != nil {
		t.Fatalf("lockfile is invalid JSON: %v\n%s", err, string(lockBytes))
	}
	if len(lockfile.Entries) != 1 {
		t.Fatalf("lockfile entries = %d, want 1: %#v", len(lockfile.Entries), lockfile.Entries)
	}
	entry := lockfile.Entries[0]
	if entry.File != ".github/workflows/ci.yml" || entry.Node != "jobs.test.steps[0].uses" {
		t.Fatalf("unexpected lockfile target: %#v", entry)
	}
	if entry.Owner != "actions" || entry.Repo != "checkout" || entry.LogicalRef != "v4" || entry.PinnedSHA != sha {
		t.Fatalf("unexpected lockfile entry: %#v", entry)
	}
	if !strings.Contains(out.String(), "Applied 1 workflow update(s) across 1 file(s).") {
		t.Fatalf("missing apply summary:\n%s", out.String())
	}
}

func TestApplyYesWritePinsUnpinnedLatestRelease(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("8", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@latest-release": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("[updates]\nunpinned = \"latest-release\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout\n")

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+sha+" # sanad: ref=v5") {
		t.Fatalf("workflow was not rewritten as expected:\n%s", got)
	}
	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	if lockfile.Entries[0].LogicalRef != "v5" || lockfile.Entries[0].PinnedSHA != sha {
		t.Fatalf("unexpected lockfile entry: %#v", lockfile.Entries[0])
	}
}

func TestApplyCommentsWriteFalseUsesLockfileWithoutInlineMetadata(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("9", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("[comments]\nwrite = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := readFileString(t, path)
	if strings.Contains(got, "sanad: ref=") {
		t.Fatalf("workflow contains inline metadata despite comments.write=false:\n%s", got)
	}
	if !strings.Contains(got, "actions/checkout@"+sha) {
		t.Fatalf("workflow was not rewritten as expected:\n%s", got)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	if lockfile.Entries[0].LogicalRef != "v4" || lockfile.Entries[0].PinnedSHA != sha {
		t.Fatalf("unexpected lockfile entry: %#v", lockfile.Entries[0])
	}
}

func TestApplyNonInteractiveRefusesWithoutYesWrite(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("c", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")
	original := readFileString(t, path)

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want non-interactive refusal")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed after refused apply\nwant:\n%s\ngot:\n%s", original, got)
	}
	if _, err := os.Stat(metadata.DefaultLockfilePath); !os.IsNotExist(err) {
		t.Fatalf("lockfile was written after refused apply, stat err = %v", err)
	}
}

func TestApplyYesWriteUpdatesLockfileWhenWorkflowAlreadyCurrent(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("d", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+sha+" # sanad: ref=v4\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed unexpectedly\nwant:\n%s\ngot:\n%s", original, got)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok {
		t.Fatal("lockfile was not written")
	}
	if len(lockfile.Entries) != 1 || lockfile.Entries[0].PinnedSHA != sha || lockfile.Entries[0].LogicalRef != "v4" {
		t.Fatalf("unexpected lockfile entries: %#v", lockfile.Entries)
	}
	if !strings.Contains(out.String(), "Updated lockfile; no workflow updates to apply.") {
		t.Fatalf("missing lockfile-only summary:\n%s", out.String())
	}
}

func TestApplyRetainsManagedPendingPinsInLockfile(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	checkoutSHA := strings.Repeat("e", 40)
	currentSetupSHA := strings.Repeat("1", 40)
	nextSetupSHA := strings.Repeat("2", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        checkoutSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
		"actions/setup-go@v5": {
			Owner:      "actions",
			Repo:       "setup-go",
			Ref:        "v5",
			SHA:        nextSetupSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-2 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      - uses: actions/checkout@v4",
		"      - uses: actions/setup-go@" + currentSetupSHA + " # sanad: ref=v5",
		"",
	}, "\n"))

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+checkoutSHA+" # sanad: ref=v4") {
		t.Fatalf("checkout was not updated:\n%s", got)
	}
	if !strings.Contains(got, "actions/setup-go@"+currentSetupSHA+" # sanad: ref=v5") {
		t.Fatalf("pending managed pin changed unexpectedly:\n%s", got)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok {
		t.Fatal("lockfile was not written")
	}
	if len(lockfile.Entries) != 2 {
		t.Fatalf("lockfile entries = %d, want 2: %#v", len(lockfile.Entries), lockfile.Entries)
	}
	for _, entry := range lockfile.Entries {
		if entry.Repo == "setup-go" && entry.PinnedSHA != currentSetupSHA {
			t.Fatalf("pending setup-go pin = %q, want current %q", entry.PinnedSHA, currentSetupSHA)
		}
	}
}

func TestApplyInteractivePinsUnpinnedActionFromExplicitRef(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("f", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader("e\nv4\ny\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+sha+" # sanad: ref=v4") {
		t.Fatalf("workflow was not rewritten as expected:\n%s", got)
	}
	if !strings.Contains(out.String(), "Found unpinned action") {
		t.Fatalf("interactive prompt missing from output:\n%s", out.String())
	}
}

func TestApplyInteractiveTracksLogicalRefForUnmanagedPinnedSHA(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("1", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        sha,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+sha+"\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader("t\nv4\ny\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed unexpectedly\nwant:\n%s\ngot:\n%s", original, got)
	}
	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	if lockfile.Entries[0].LogicalRef != "v4" || lockfile.Entries[0].PinnedSHA != sha {
		t.Fatalf("unexpected lockfile entry: %#v", lockfile.Entries[0])
	}
	if !strings.Contains(out.String(), "Found unmanaged pinned action") {
		t.Fatalf("interactive prompt missing from output:\n%s", out.String())
	}
}

func TestApplyInteractivePinsDeniedBranchHead(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("6", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"owner/repo@main": {
			Owner:      "owner",
			Repo:       "repo",
			Ref:        "main",
			SHA:        sha,
			Kind:       githubresolver.KindBranch,
			CommitTime: now.Add(-1 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: owner/repo@main\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader("p\nn\ny\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "owner/repo@"+sha+" # sanad: ref=main") {
		t.Fatalf("workflow was not rewritten as expected:\n%s", got)
	}
	if !strings.Contains(out.String(), "Found branch ref") {
		t.Fatalf("branch prompt missing from output:\n%s", out.String())
	}
}

func TestApplyInteractivePersistsBranchTrackingWhenRequested(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("6", 40)
	nextSHA := strings.Repeat("7", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"owner/repo@main": {
			Owner:      "owner",
			Repo:       "repo",
			Ref:        "main",
			SHA:        sha,
			Kind:       githubresolver.KindBranch,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: owner/repo@main\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader("p\ny\ny\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--interactive"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	configText := readFileString(t, config.DefaultPath)
	if !strings.Contains(configText, `branches = "track"`) {
		t.Fatalf("config did not persist branch tracking:\n%s", configText)
	}

	installPlanTestResolver(t, fakePlanResolver{
		"owner/repo@main": {
			Owner:      "owner",
			Repo:       "repo",
			Ref:        "main",
			SHA:        nextSHA,
			Kind:       githubresolver.KindBranch,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	cmd = NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("non-interactive apply after persistence returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "owner/repo@"+nextSHA+" # sanad: ref=main") {
		t.Fatalf("workflow was not updated by persisted branch policy:\n%s", got)
	}
}

func TestApplyFirstSeenCooldownRecordsPendingCandidate(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("6", 40)
	nextSHA := strings.Repeat("7", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"owner/repo@main": {
			Owner:      "owner",
			Repo:       "repo",
			Ref:        "main",
			SHA:        nextSHA,
			Kind:       githubresolver.KindBranch,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("cooldown_source = \"first-seen\"\n\n[updates]\nbranches = \"track\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: owner/repo@"+currentSHA+" # sanad: ref=main\n")

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--yes", "--write"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "owner/repo@"+currentSHA+" # sanad: ref=main") {
		t.Fatalf("workflow changed before first-seen cooldown elapsed:\n%s", got)
	}
	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 || lockfile.Entries[0].CandidateSHA != nextSHA {
		t.Fatalf("pending candidate was not recorded: ok=%v entries=%#v", ok, lockfile.Entries)
	}
}

func withTempWorkingDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	previousWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWorkingDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	return root
}

func writeApplyWorkflow(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(".github", "workflows", "ci.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFileString(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
