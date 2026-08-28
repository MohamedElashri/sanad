package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryRootDiscoversGitAncestor(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "tools", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := repositoryRoot(nested, "")
	if err != nil {
		t.Fatalf("repositoryRoot returned error: %v", err)
	}
	if got != root {
		t.Fatalf("repositoryRoot = %q, want %q", got, root)
	}
}

func TestRepositoryRootHonorsExplicitRoot(t *testing.T) {
	root := t.TempDir()
	got, err := repositoryRoot(".", root)
	if err != nil {
		t.Fatalf("repositoryRoot returned error: %v", err)
	}
	if got != root {
		t.Fatalf("repositoryRoot = %q, want %q", got, root)
	}
}

func TestScanRunsFromRepositorySubdirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte("jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "tools", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"scan", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("scan returned error: %v", err)
	}
	var entries []scanEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("invalid scan JSON: %v", err)
	}
	if len(entries) != 1 || entries[0].Raw != "actions/checkout@v4" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if current, err := os.Getwd(); err != nil || current != nested {
		t.Fatalf("working directory not restored: current=%q err=%v", current, err)
	}
}
