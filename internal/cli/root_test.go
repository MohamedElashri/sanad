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

func TestVersionHelpAndCompletions(t *testing.T) {
	out, err := execute("version", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if v["backend_version"] != "v0.2.0" {
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
