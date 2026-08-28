package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/githubresolver"
	"github.com/MohamedElashri/sanad/internal/metadata"
)

func TestUpgradeDryRunExplicitTargetShowsDiffWithoutWriting(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("1", 40)
	targetSHA := strings.Repeat("2", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", "v5", "--dry-run", "--diff"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed during dry-run\nwant:\n%s\ngot:\n%s", original, got)
	}

	text := out.String()
	for _, want := range []string{
		"Matched 1 managed action pin(s)",
		"v4",
		"v5",
		"update",
		"-      - uses: actions/checkout@" + currentSHA + " # sanad: ref=v4",
		"+      - uses: actions/checkout@" + targetSHA + " # sanad: ref=v5",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("upgrade dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestUpgradeWriteUpdatesWorkflowAndLockfile(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("3", 40)
	targetSHA := strings.Repeat("4", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", "v5", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+targetSHA+" # sanad: ref=v5") {
		t.Fatalf("workflow was not upgraded:\n%s", got)
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
	if entry.LogicalRef != "v5" || entry.PinnedSHA != targetSHA {
		t.Fatalf("unexpected lockfile entry: %#v", entry)
	}
	if !strings.Contains(out.String(), "Applied 1 logical ref upgrade(s) across 1 file(s).") {
		t.Fatalf("missing write summary:\n%s", out.String())
	}
}

func TestUpgradeWriteIgnoresStaleLockfilePinDrift(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("1", 40)
	lockfileSHA := strings.Repeat("2", 40)
	targetSHA := strings.Repeat("3", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	writeTestLockfile(t, lockTestEntry("actions", "checkout", "v4", lockfileSHA))

	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", "v5", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+targetSHA+" # sanad: ref=v5") {
		t.Fatalf("workflow was not upgraded:\n%s", got)
	}

	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		t.Fatalf("LoadLockfile returned error: %v", err)
	}
	if !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile state ok=%v entries=%#v", ok, lockfile.Entries)
	}
	if lockfile.Entries[0].LogicalRef != "v5" || lockfile.Entries[0].PinnedSHA != targetSHA {
		t.Fatalf("unexpected lockfile entry: %#v", lockfile.Entries[0])
	}
}

func TestUpgradeLatestReleaseUsesConfiguredReleaseMode(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("5", 40)
	targetSHA := strings.Repeat("6", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@latest-release": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v6",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("[upgrade]\nlatest_release = \"release\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v5\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--latest-release", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "actions/checkout@"+targetSHA+" # sanad: ref=v6") {
		t.Fatalf("workflow was not upgraded to latest release:\n%s\noutput:\n%s", got, out.String())
	}
}

func TestUpgradeBareCommandDefaultsToAllLatestReleaseDryRun(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("a", 40)
	targetSHA := strings.Repeat("b", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@latest-release": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v6",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v5\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed during default dry-run\nwant:\n%s\ngot:\n%s", original, got)
	}

	text := out.String()
	for _, want := range []string{
		"Matched 1 managed action pin(s)",
		"v5",
		"v6",
		"update",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("upgrade default output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "--- .github/workflows/ci.yml") || strings.Contains(text, "@@ -") {
		t.Fatalf("default upgrade unexpectedly printed a diff:\n%s", text)
	}
	if !strings.Contains(text, "Add --diff to show the patch") {
		t.Fatalf("default upgrade did not advertise --diff:\n%s", text)
	}
}

func TestUpgradeJSONReportVersionTwoIncludesEffectivePolicyAndCandidates(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", strings.Repeat("b", 40), now.Add(-20*24*time.Hour)),
	}, now)
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+strings.Repeat("a", 40)+" # sanad: ref=v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "upgrade"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var report upgradeReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON report: %v\n%s", err, out.String())
	}
	if report.Version != 2 || len(report.Actions) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	action := report.Actions[0]
	if action.Level != "major" || action.Selection != "latest-eligible" || action.SelectedRelease != "v5.0.0" || len(action.Candidates) != 1 {
		t.Fatalf("missing policy or candidate audit fields: %#v", action)
	}
}

func TestUpgradeNoOpWhenTargetAlreadyCurrent(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("7", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        currentSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v5\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", "v5", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed for no-op upgrade\nwant:\n%s\ngot:\n%s", original, got)
	}
	if !strings.Contains(out.String(), "No eligible upgrades to apply.") {
		t.Fatalf("missing no-op output:\n%s", out.String())
	}
}

func TestUpgradeRespectsCooldown(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("8", 40)
	targetSHA := strings.Repeat("9", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v5",
			SHA:        targetSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-2 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)

	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	original := readFileString(t, path)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", "v5", "--write", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed while cooldown was pending\nwant:\n%s\ngot:\n%s", original, got)
	}
	if !strings.Contains(out.String(), "pending-cooldown") {
		t.Fatalf("cooldown decision missing from output:\n%s", out.String())
	}
}

func TestUpgradeRejectsSHATarget(t *testing.T) {
	withTempWorkingDir(t)
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--action", "actions/checkout", "--to", strings.Repeat("a", 40)})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error for SHA target")
	}
	if ExitCode(err) != exitConfig {
		t.Fatalf("ExitCode = %d, want %d; err=%v", ExitCode(err), exitConfig, err)
	}
}

func TestValidateUpgradeAutomaticPolicyFlags(t *testing.T) {
	for _, opts := range []upgradeOptions{
		{all: true, level: "minor", levelSet: true, constraint: "< 6", constraintSet: true},
		{all: true, to: "v5", level: "minor", levelSet: true},
		{all: true, latestRelease: true, selection: "latest-eligible", selectionSet: true},
		{all: true, level: "breaking", levelSet: true},
		{all: true, constraint: "not a range", constraintSet: true},
		{all: true, selection: "oldest", selectionSet: true},
	} {
		if err := validateUpgradeOptions(&opts); err == nil {
			t.Fatalf("validateUpgradeOptions(%#v) returned nil error", opts)
		}
	}
	if err := validateUpgradeOptions(&upgradeOptions{all: true, level: "minor", levelSet: true, selection: "latest-eligible", selectionSet: true}); err != nil {
		t.Fatalf("valid automatic options returned error: %v", err)
	}
	if err := validateUpgradeOptions(&upgradeOptions{action: "actions/checkout", to: "v5"}); err != nil {
		t.Fatalf("valid explicit options returned error: %v", err)
	}
}
