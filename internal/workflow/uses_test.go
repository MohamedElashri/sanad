package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractUsesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".github", "workflows", "ci.yml")
	writeWorkflow(t, path, strings.Join([]string{
		"name: ci",
		"jobs:",
		"  test:",
		"    runs-on: ubuntu-latest",
		"    steps:",
		"      - name: Say text",
		"        run: 'echo \"uses: actions/nope@v1\"'",
		"      - uses: actions/checkout@v4 # sanad: ref=v4",
		"      - uses: \"actions/setup-go@v5\"",
		"  reuse:",
		"    uses: org/repo/.github/workflows/reuse.yml@v1",
		"",
	}, "\n"))

	got, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatalf("ExtractUsesFromFile returned error: %v", err)
	}

	values := make([]string, 0, len(got))
	paths := make([]string, 0, len(got))
	lines := make([]int, 0, len(got))
	for _, use := range got {
		values = append(values, use.Raw)
		paths = append(paths, use.NodePath)
		lines = append(lines, use.Line)

		if use.File != path {
			t.Fatalf("File = %q, want %q", use.File, path)
		}
		if use.Column <= 0 {
			t.Fatalf("Column = %d, want positive column", use.Column)
		}
		if use.LineIndex != use.Line-1 {
			t.Fatalf("LineIndex = %d, want %d", use.LineIndex, use.Line-1)
		}
		if !strings.Contains(use.HeadLine, "uses:") {
			t.Fatalf("HeadLine = %q, want original uses line", use.HeadLine)
		}
	}
	if got[0].InlineComment != "# sanad: ref=v4" {
		t.Fatalf("InlineComment = %q, want sanad ref comment", got[0].InlineComment)
	}

	wantValues := []string{
		"actions/checkout@v4",
		"actions/setup-go@v5",
		"org/repo/.github/workflows/reuse.yml@v1",
	}
	if !reflect.DeepEqual(values, wantValues) {
		t.Fatalf("Raw values = %#v, want %#v", values, wantValues)
	}

	wantPaths := []string{
		"jobs.test.steps[1].uses",
		"jobs.test.steps[2].uses",
		"jobs.reuse.uses",
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("Node paths = %#v, want %#v", paths, wantPaths)
	}

	wantLines := []int{8, 9, 11}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Fatalf("Lines = %#v, want %#v", lines, wantLines)
	}
}

func TestExtractUsesFromFileInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yml")
	writeWorkflow(t, path, "jobs:\n  test: [unterminated\n")

	_, err := ExtractUsesFromFile(path)
	if err == nil {
		t.Fatal("ExtractUsesFromFile returned nil error, want parse error")
	}
	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Fatalf("error = %q, want invalid YAML context", err)
	}
}

func writeWorkflow(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
