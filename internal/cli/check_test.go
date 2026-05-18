package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/githubresolver"
)

func TestCheckPassesWhenManagedPinIsCurrent(t *testing.T) {
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
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+sha+" # sanad: ref=v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "All workflow dependencies comply") {
		t.Fatalf("missing success output:\n%s", out.String())
	}
}

func TestCheckFailsMutableTagReference(t *testing.T) {
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
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want check violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}
	text := out.String()
	for _, want := range []string{"actions/checkout", "update", "Check failed with 1 violation"} {
		if !strings.Contains(text, want) {
			t.Fatalf("check output missing %q:\n%s", want, text)
		}
	}
}

func TestCheckFailsShortSHAReference(t *testing.T) {
	installPlanTestResolver(t, fakePlanResolver{}, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@11bd719\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want short-SHA violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}
	if !strings.Contains(out.String(), "error-short-sha") {
		t.Fatalf("check output missing short-SHA decision:\n%s", out.String())
	}
}

func TestCheckJSONIncludesViolations(t *testing.T) {
	installPlanTestResolver(t, fakePlanResolver{}, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: owner/unpinned\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check", "--format", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want unpinned violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}

	var report checkReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("check output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Passed {
		t.Fatalf("Passed = true, want false: %#v", report)
	}
	if report.Summary.Violations != 1 || len(report.Violations) != 1 {
		t.Fatalf("unexpected violation count: %#v", report)
	}
	violation := report.Violations[0]
	if violation.Decision != "error-unpinned" {
		t.Fatalf("Decision = %q, want error-unpinned", violation.Decision)
	}
	if violation.ReasonCode != "unpinned-reference" {
		t.Fatalf("ReasonCode = %q, want unpinned-reference", violation.ReasonCode)
	}
	if violation.Raw != "owner/unpinned" {
		t.Fatalf("Raw = %q, want owner/unpinned", violation.Raw)
	}
}

func TestCheckUsesDefaultBranchPolicyForUnpinnedActions(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("7", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@default-branch": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "main",
			SHA:        sha,
			Kind:       githubresolver.KindBranch,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("[updates]\nunpinned = \"default-branch\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check", "--format", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want unpinned update violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}

	var report checkReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("check output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(report.Violations) != 1 {
		t.Fatalf("violations = %d, want 1: %#v", len(report.Violations), report)
	}
	violation := report.Violations[0]
	if violation.Decision != "update" || violation.LogicalRef != "main" || violation.CandidateSHA != sha {
		t.Fatalf("unexpected violation: %#v", violation)
	}
}

func TestCheckSARIFIncludesViolations(t *testing.T) {
	installPlanTestResolver(t, fakePlanResolver{}, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: owner/unpinned\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"check", "--format", "sarif"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want unpinned violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}

	var report struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("SARIF output is not valid JSON: %v\n%s", err, out.String())
	}
	if report.Version != "2.1.0" || len(report.Runs) != 1 {
		t.Fatalf("unexpected SARIF header: %#v", report)
	}
	run := report.Runs[0]
	if run.Tool.Driver.Name != "sanad" {
		t.Fatalf("SARIF tool name = %q, want sanad", run.Tool.Driver.Name)
	}
	if len(run.Tool.Driver.Rules) != 1 || run.Tool.Driver.Rules[0].ID != "unpinned-reference" {
		t.Fatalf("unexpected SARIF rules: %#v", run.Tool.Driver.Rules)
	}
	if len(run.Results) != 1 {
		t.Fatalf("unexpected SARIF results: %#v", run.Results)
	}
	result := run.Results[0]
	if result.RuleID != "unpinned-reference" || result.Level != "error" {
		t.Fatalf("unexpected SARIF result: %#v", result)
	}
	if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI == "" || result.Locations[0].PhysicalLocation.Region.StartLine != 4 {
		t.Fatalf("unexpected SARIF location: %#v", result.Locations)
	}
}

func TestCheckStrictControlsManagedUpdatesAndPendingCooldown(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("1", 40)
	nextSHA := strings.Repeat("2", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/setup-go@v5": {
			Owner:      "actions",
			Repo:       "setup-go",
			Ref:        "v5",
			SHA:        nextSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-2 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/setup-go@"+currentSHA+" # sanad: ref=v5\n")

	if err := executeCheckWithArgs("check"); err != nil {
		t.Fatalf("default check returned error for pending managed pin: %v", err)
	}
	err := executeCheckWithArgs("check", "--strict")
	if err == nil {
		t.Fatal("strict check returned nil error, want pending violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("strict ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}
	if err := executeCheckWithArgs("check", "--strict", "--allow-pending-cooldown"); err != nil {
		t.Fatalf("strict check with allow-pending-cooldown returned error: %v", err)
	}
}

func TestCheckFailOnUpdatesControlsManagedEligibleUpdates(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("3", 40)
	nextSHA := strings.Repeat("4", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/setup-go@v5": {
			Owner:      "actions",
			Repo:       "setup-go",
			Ref:        "v5",
			SHA:        nextSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/setup-go@"+currentSHA+" # sanad: ref=v5\n")

	if err := executeCheckWithArgs("check"); err != nil {
		t.Fatalf("default check returned error for eligible managed update: %v", err)
	}
	err := executeCheckWithArgs("check", "--fail-on-updates")
	if err == nil {
		t.Fatal("check --fail-on-updates returned nil error, want update violation")
	}
	if ExitCode(err) != exitPolicy {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitPolicy, err)
	}
}

func executeCheckWithArgs(args ...string) error {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}
