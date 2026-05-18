package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanJSONOutput(t *testing.T) {
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
	cmd.SetArgs([]string{"--format", "json", "scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("scan output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("JSON entries = %d, want 1: %#v", len(got), got)
	}
	if got[0]["raw"] != "actions/checkout@v4" {
		t.Fatalf("raw = %#v, want actions/checkout@v4", got[0]["raw"])
	}
	if got[0]["node_path"] != "jobs.test.steps[0].uses" {
		t.Fatalf("node_path = %#v, want jobs.test.steps[0].uses", got[0]["node_path"])
	}
	if got[0]["owner"] != "actions" {
		t.Fatalf("owner = %#v, want actions", got[0]["owner"])
	}
	if got[0]["repo"] != "checkout" {
		t.Fatalf("repo = %#v, want checkout", got[0]["repo"])
	}
	if got[0]["ref"] != "v4" {
		t.Fatalf("ref = %#v, want v4", got[0]["ref"])
	}
	if got[0]["kind"] != "github-action" {
		t.Fatalf("kind = %#v, want github-action", got[0]["kind"])
	}
	if got[0]["valid"] != true {
		t.Fatalf("valid = %#v, want true", got[0]["valid"])
	}
	if got[0]["pinned"] != false {
		t.Fatalf("pinned = %#v, want false", got[0]["pinned"])
	}
}

func TestScanTableOutputReportsInvalidReferences(t *testing.T) {
	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      - uses: actions/checkout@11bd719",
		"      - uses: docker://alpine:3.20",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"FILE",
		"ACTION",
		"REF",
		"KIND",
		"PINNED",
		"VALID",
		"ERROR",
		"actions/checkout",
		"11bd719",
		"invalid",
		"short SHA refs are not accepted",
		"alpine:3.20",
		"docker-action",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("table output missing %q:\n%s", want, text)
		}
	}
}

func TestScanReportsIgnoredReferences(t *testing.T) {
	workflows := filepath.Join(t.TempDir(), ".github", "workflows")
	path := filepath.Join(workflows, "ci.yml")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(t.TempDir(), ".sanad.toml")
	if err := os.WriteFile(configPath, []byte("[ignore]\nactions = [\"actions/checkout@v4\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "--format", "json", "scan", "--workflows", workflows})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var got []scanEntry
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("scan output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("JSON entries = %d, want 1: %#v", len(got), got)
	}
	if !got[0].Ignored {
		t.Fatalf("Ignored = false, want true: %#v", got[0])
	}
	if got[0].IgnoreBy != "action" || got[0].IgnoreRule != "actions/checkout@v4" {
		t.Fatalf("ignore metadata = (%q, %q), want action/actions/checkout@v4", got[0].IgnoreBy, got[0].IgnoreRule)
	}
}
