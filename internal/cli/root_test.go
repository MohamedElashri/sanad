package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(args ...string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestOldCommandsAndFlagsAreRemoved(t *testing.T) {
	for _, args := range [][]string{{"apply"}, {"upgrade"}, {"lock"}, {"migrate"}, {"start"}, {"check", "--fresh"}, {"update", "--yes"}} {
		if _, err := execute(args...); err == nil {
			t.Fatalf("legacy interface accepted: %v", args)
		}
	}
}

func TestOperationalErrorsHaveOneJSONReport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".sanad.toml"), []byte("version=1\nunknown=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := execute("check", "--root", root, "--format", "json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatal(out, err)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal("stdout is not one JSON value", out, err)
	}
	if report["success"] != false || report["version"] != float64(1) {
		t.Fatal(report)
	}
}

func TestStatusRendersTablesByDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github/workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	lockfile := `version: v0.0.3
workflows: {}
dependencies:
  actions/checkout@v7.0.1:
    ref: v7.0.1
    commit: sha1-0123456789abcdef0123456789abcdef01234567
    owner_id: 1
    repo_id: 2
`
	if err := os.WriteFile(filepath.Join(root, ".github/workflows/actions.lock"), []byte(lockfile), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SANAD_BACKEND", filepath.Join(root, "missing-backend"))
	for _, command := range []string{"", "status", "doctor"} {
		args := []string{"--root", root, "--repo", "github.com/example/test"}
		if command != "" {
			args = append(args, command)
		}
		out, err := execute(args...)
		if err != nil {
			t.Fatal(out, err)
		}
		if !strings.Contains(out, "actions/checkout@v7.0.1") || strings.Contains(out, "| --- |") || strings.Contains(out, "\x1b") {
			t.Fatalf("%q did not render a plain table:\n%s", command, out)
		}
		out, err = execute(append(args, "--format", "markdown")...)
		if err != nil || !strings.Contains(out, "| --- | --- | --- |") {
			t.Fatalf("%q did not preserve raw Markdown:\n%s\n%v", command, out, err)
		}
	}
}

func TestReportFormatsPreserveExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		data string
		code int
	}{
		{"success", "", "", 0},
		{"blocked", ".github/sanad.lock.json", "{}", 1},
		{"operational error", ".sanad.toml", "version=1\nunknown=true\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("SANAD_BACKEND", filepath.Join(root, "missing-backend"))
			if tc.file != "" {
				path := filepath.Join(root, filepath.FromSlash(tc.file))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, format := range []string{"table", "markdown", "json"} {
				out, err := execute("status", "--root", root, "--repo", "github.com/example/test", "--format", format)
				if ExitCode(err) != tc.code {
					t.Fatalf("%s: got exit %d, want %d: %v\n%s", format, ExitCode(err), tc.code, err, out)
				}
			}
		})
	}
}

func TestVersionHelpAndCompletions(t *testing.T) {
	out, err := execute("version", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if v["backend_version"] != "v0.4.1" {
		t.Fatal(v)
	}
	out, err = execute("--help")
	if err != nil || !strings.Contains(out, "sync") || strings.Contains(out, "upgrade") {
		t.Fatal(out, err)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out, err := execute("completion", shell)
		if err != nil || len(out) == 0 {
			t.Fatal(shell, err)
		}
	}
}
