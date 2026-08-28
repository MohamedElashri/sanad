package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommitFileTransactionRollsBackEarlierFiles(t *testing.T) {
	root := withTempWorkingDir(t)
	first := filepath.Join(root, "first.yml")
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(first, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}

	err := commitFileTransaction([]transactionFile{
		{path: first, data: []byte("new\n"), perm: 0o640},
		{path: blocked, data: []byte("cannot replace directory\n"), perm: 0o600},
	})
	if err == nil {
		t.Fatal("commitFileTransaction returned nil error")
	}
	data, readErr := os.ReadFile(first)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old\n" {
		t.Fatalf("first file was not rolled back: %q", data)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".sanad-*")); len(matches) != 0 {
		t.Fatalf("transaction left temporary files: %#v", matches)
	}
}
