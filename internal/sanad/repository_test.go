package sanad

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryRejectsUnmanagedFiles(t *testing.T) {
	for _, name := range []string{".git/hooks/post-checkout", ".git/hooks/action.yml", "README.md", "nested/.git/action.yaml"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			j := journal{Before: tree{name: {Data: []byte("payload"), Mode: 0o755}}, After: tree{}, Paths: []string{name}}
			b, _ := json.Marshal(j)
			if err := putFile(root, transactionPath, file{Data: b, Mode: 0o600}); err != nil {
				t.Fatal(err)
			}
			if err := recoverTransaction(root, ConfigPath); err == nil {
				t.Fatal("unmanaged recovery was accepted")
			}
			if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
				t.Fatal("recovery created an unmanaged file", err)
			}
		})
	}
}

func TestRecoveryRequiresExactChangedPaths(t *testing.T) {
	root := t.TempDir()
	name := ".github/workflows/ci.yml"
	before := tree{name: workflow("a/root@v1")}
	after := tree{name: workflow("a/root@v2")}
	for _, paths := range [][]string{{name, name}, {ConfigPath}, {name, ConfigPath}} {
		if err := validateJournal(root, ConfigPath, journal{Before: before, After: after, Paths: paths}); err == nil {
			t.Fatalf("invalid paths accepted: %v", paths)
		}
	}
}
