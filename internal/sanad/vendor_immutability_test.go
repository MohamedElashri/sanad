package sanad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestOrdinaryUpdatesKeepVendorSnapshotsImmutable(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{false: "vendor-only", true: "shared-direct-call"}[direct], func(t *testing.T) {
			root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0"), "b/child@v1.0.0": action("v1.0.0", "c")}, "")
			backend := &fakeBackend{t: t, definitions: map[string]lock.Action{"b/child@v1.1.0": action("v1.1.0", "d")}}
			opts := options(root, backend)
			opts.Action = "a/root"
			opts.Write = true
			archive := sourceArchive(t, map[string]string{"action.yml": "name: composite\nruns:\n  using: composite\n  steps:\n    - uses: b/child@v1.0.0\n", "LICENSE": "MIT fixture\n"})
			opts.Archives = fakeArchives{map[string][]byte{"sha1-" + strings.Repeat("a", 40): archive}}
			r, err := Run(context.Background(), "vendor", opts)
			if err != nil || !r.Success {
				t.Fatalf("vendor: %+v %v", r, err)
			}
			name := filepath.Join(root, VendorRoot, "a/root/action.yml")
			before, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if direct {
				workflowName := filepath.Join(root, ".github/workflows/ci.yml")
				data, err := os.ReadFile(workflowName)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data, []byte("      - uses: b/child@v1.0.0\n")...)
				if err := os.WriteFile(workflowName, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			opts.Action = "b/child"
			opts.Releases = fakeReleases{refs: []Release{{Ref: "v1.1.0"}}}
			r, err = Run(context.Background(), "update", opts)
			if err != nil || !r.Success {
				t.Fatalf("update: %+v %v", r, err)
			}
			after, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("ordinary update rewrote immutable vendored metadata")
			}
			files, err := readTree(root, ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			state, err := loadVendors(files)
			if err != nil {
				t.Fatal(err)
			}
			if err := vendorChecks(files, state); err != nil {
				t.Fatal(err)
			}
			workflowData, err := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
			if err != nil {
				t.Fatal(err)
			}
			if direct && !strings.Contains(string(workflowData), "b/child@v1.1.0") {
				t.Fatal("freezing vendor metadata also froze an editable direct call")
			}
		})
	}
}

type vendorEditingBackend struct{ Backend }

func (b vendorEditingBackend) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	result, err := b.Backend.Run(ctx, root, host, repo, args...)
	if err == nil && len(args) > 0 && args[0] == "--migrate-local-actions" {
		name := filepath.Join(root, VendorRoot, "a/root/action.yml")
		data, e := os.ReadFile(name)
		if e != nil {
			return result, e
		}
		err = os.WriteFile(name, append(data, []byte("# producer rewrite\n")...), 0o644)
	}
	return result, err
}
func TestSyncRejectsProducerRewritesOfVendorSnapshots(t *testing.T) {
	root, opts := vendorOptions(t)
	opts.Write = true
	r, err := Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	before, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	opts.Backend = vendorEditingBackend{opts.Backend}
	r, err = Run(context.Background(), "sync", opts)
	if err == nil || !r.Blocked() || r.Changed {
		t.Fatalf("producer rewrite was accepted: %+v %v", r, err)
	}
	after, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if changes := changedFiles(before, after); len(changes) != 0 {
		t.Fatalf("partial vendor rewrite: %v", changes)
	}
}
