package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkExtractUsesFromLargeWorkflow(b *testing.B) {
	path := filepath.Join(b.TempDir(), "large.yml")
	if err := os.WriteFile(path, []byte(largeWorkflow(1000)), 0o600); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		uses, err := ExtractUsesFromFile(path)
		if err != nil {
			b.Fatal(err)
		}
		if len(uses) != 1000 {
			b.Fatalf("ExtractUsesFromFile returned %d uses, want 1000", len(uses))
		}
	}
}

func TestExtractUsesFromLargeWorkflowDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.yml")
	if err := os.WriteFile(path, []byte(largeWorkflow(250)), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 250 || len(second) != 250 {
		t.Fatalf("use counts = %d and %d, want 250", len(first), len(second))
	}
	for i := range first {
		if first[i].NodePath != second[i].NodePath || first[i].Raw != second[i].Raw || first[i].Line != second[i].Line {
			t.Fatalf("use %d changed between runs: %#v vs %#v", i, first[i], second[i])
		}
	}
}

func largeWorkflow(actions int) string {
	var b strings.Builder
	b.WriteString("jobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n")
	for i := 0; i < actions; i++ {
		fmt.Fprintf(&b, "      - uses: owner/repo/action-%03d@v1\n", i)
	}
	return b.String()
}
