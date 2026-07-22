package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/githubresolver"
)

func TestColorAlwaysColorizesHumanOutput(t *testing.T) {
	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@11bd719\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--color", "always", "audit", "scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("expected ANSI color in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "short SHA refs are not accepted") {
		t.Fatalf("missing scan content:\n%s", out.String())
	}
}

func TestColorNeverWinsOverForcedEnvironment(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")

	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--color", "never", "audit", "scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("did not expect ANSI color in output:\n%s", out.String())
	}
}

func TestColorAlwaysDoesNotAffectJSONOutput(t *testing.T) {
	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--color", "always", "--format", "json", "audit", "scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("did not expect ANSI color in JSON output:\n%s", out.String())
	}
	var entries []scanEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("scan output is not valid JSON: %v\n%s", err, out.String())
	}
}

func TestPlanColorizesCurrentAndCandidateDifferently(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("a", 40)
	candidateSHA := strings.Repeat("b", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v4": {
			Owner:      "actions",
			Repo:       "checkout",
			Ref:        "v4",
			SHA:        candidateSHA,
			Kind:       githubresolver.KindTag,
			CommitTime: now.Add(-15 * 24 * time.Hour),
		},
	}, now)

	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + currentSHA + " # sanad: ref=v4\n"
	if err := os.WriteFile(path, []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--color", "always", "audit", "plan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "\x1b[31maaaaaaaaaaaa") {
		t.Fatalf("current SHA was not colored as the old value:\n%s", text)
	}
	if !strings.Contains(text, "\x1b[32mbbbbbbbbbbbb") {
		t.Fatalf("candidate SHA was not colored as the new value:\n%s", text)
	}
	if !strings.Contains(text, "\x1b[36m"+path) {
		t.Fatalf("file column was not colored with its stable column color:\n%s", text)
	}
	if !strings.Contains(text, "\x1b[90m4") {
		t.Fatalf("line column was not colored with its stable column color:\n%s", text)
	}
	if !strings.Contains(text, "\x1b[35m") {
		t.Fatalf("reason column was not colored with its stable column color:\n%s", text)
	}
}

func TestInvalidColorModeReturnsConfigError(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--color", "sparkles", "version"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error for invalid color mode")
	}
	if ExitCode(err) != exitConfig {
		t.Fatalf("ExitCode = %d, want %d; error: %v", ExitCode(err), exitConfig, err)
	}
}

func TestColorThemeTunesWarningColor(t *testing.T) {
	if got := newTerminalStyle(terminalThemeDark).Wrap(colorWarning, "pending"); !strings.Contains(got, "\x1b[33m") {
		t.Fatalf("dark theme warning = %q, want ANSI yellow", got)
	}
	if got := newTerminalStyle(terminalThemeLight).Wrap(colorWarning, "pending"); !strings.Contains(got, "\x1b[35m") {
		t.Fatalf("light theme warning = %q, want ANSI magenta", got)
	}
}
