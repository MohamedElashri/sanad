package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzExtractUsesFromFile(f *testing.F) {
	seeds := []string{
		"",
		"jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n",
		"jobs:\n  test: [unterminated\n",
		"jobs:\n  reuse:\n    uses: org/repo/.github/workflows/reuse.yml@v1\n",
		"jobs:\n  test:\n    steps:\n      - uses: 123\n",
		"uses: docker://alpine:3.20\n",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "workflow.yml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		uses, err := ExtractUsesFromFile(path)
		if err != nil {
			return
		}
		for _, use := range uses {
			if use.File != path {
				t.Fatalf("File = %q, want %q", use.File, path)
			}
			if use.Line <= 0 {
				t.Fatalf("Line = %d, want positive", use.Line)
			}
			if use.LineIndex != use.Line-1 {
				t.Fatalf("LineIndex = %d, want %d", use.LineIndex, use.Line-1)
			}
		}
	})
}
