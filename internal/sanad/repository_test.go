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

func TestReadTreeFollowsLocalActionsInAnyDirectory(t *testing.T) {
	for _, dir := range []string{"vendor", "dist", "bin", "node_modules", ".cache"} {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			files := tree{
				".github/workflows/ci.yml": workflow("$/" + dir + "/local"),
				dir + "/local/action.yml":  {Data: []byte("runs:\n  using: composite\n  steps:\n    - uses: $/nested\n"), Mode: 0o644},
				"nested/action.yaml":       {Data: []byte("runs:\n  using: composite\n  steps: []\n"), Mode: 0o644},
				"unused/action.yml":        {Data: []byte("invalid: ["), Mode: 0o644},
			}
			for name, f := range files {
				if err := putFile(root, name, f); err != nil {
					t.Fatal(err)
				}
			}
			got, err := readTree(root, ConfigPath)
			if err != nil || len(got) != 3 {
				t.Fatalf("reachable metadata lost: %v %v", got, err)
			}
			for _, name := range []string{dir + "/local/action.yml", "nested/action.yaml"} {
				if _, ok := got[name]; !ok {
					t.Fatal("missing local metadata", name)
				}
			}
		})
	}
}
