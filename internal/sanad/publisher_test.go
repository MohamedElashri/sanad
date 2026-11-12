package sanad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

type rewritingPublisherBackend struct{ Backend }

func (b rewritingPublisherBackend) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	result, err := b.Backend.Run(ctx, root, host, repo, args...)
	if err == nil && len(args) > 0 && args[0] == "--migrate-local-actions" {
		p := filepath.Join(root, "export/action.yml")
		data, e := os.ReadFile(p)
		if e != nil {
			return result, e
		}
		err = os.WriteFile(p, []byte(strings.ReplaceAll(string(data), strings.Repeat("a", 40), "v1.0.0")), 0o644)
	}
	return result, err
}
func TestPublisherGuardProtectsUnusedExportsAndTransactions(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	config, _ := os.ReadFile(filepath.Join(root, ConfigPath))
	config = append(config, []byte("\n[publisher]\nactions=[\"export/action.yml\"]\n")...)
	if err := putFile(root, ConfigPath, file{Data: config, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	metadata := file{Data: []byte("name: exported\nruns:\n  using: composite\n  steps:\n    - uses: a/root@" + strings.Repeat("a", 40) + "\n"), Mode: 0o644}
	if err := putFile(root, "export/action.yml", metadata); err != nil {
		t.Fatal(err)
	}
	opts := options(root, rewritingPublisherBackend{&fakeBackend{t: t}})
	opts.Write = true
	before, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || !r.Blocked() || r.Changed {
		t.Fatalf("guard: %+v %v", r, err)
	}
	after, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if changes := changedFiles(before, after); len(changes) != 0 {
		t.Fatalf("partial transaction: %v", changes)
	}
	opts.Backend = &fakeBackend{t: t}
	r, err = Run(context.Background(), "check", opts)
	if err != nil || r.Blocked() {
		t.Fatalf("pinned export: %+v %v", r, err)
	}
	metadata.Data = []byte(strings.ReplaceAll(string(metadata.Data), strings.Repeat("a", 40), "v1"))
	if err := putFile(root, "export/action.yml", metadata); err != nil {
		t.Fatal(err)
	}
	r, err = Run(context.Background(), "status", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("unpinned unused export: %+v %v", r, err)
	}
}
