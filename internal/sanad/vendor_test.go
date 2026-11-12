package sanad

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func sourceArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tr := tar.NewWriter(gz)
	for name, data := range files {
		if err := tr.WriteHeader(&tar.Header{Name: "repo-sha/" + name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type fakeArchives struct{ data map[string][]byte }

func (f fakeArchives) Archive(_ context.Context, d Dependency) ([]byte, error) {
	b, ok := f.data[d.Commit]
	if !ok {
		return nil, fmt.Errorf("missing archive %s", d.Commit)
	}
	return b, nil
}
func vendorOptions(t *testing.T) (string, Options) {
	t.Helper()
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}})
	opts.Action = "a/root"
	first := sourceArchive(t, map[string]string{"action.yml": "name: action\nruns:\n  using: node24\n  main: dist/index.js\n", "dist/index.js": "console.log('first');\n", "LICENSE": "MIT license fixture\n"})
	second := sourceArchive(t, map[string]string{"action.yml": "name: action\nruns:\n  using: node24\n  main: dist/index.js\n", "dist/index.js": "console.log('second');\n", "LICENSE": "MIT license fixture\n"})
	opts.Archives = fakeArchives{map[string][]byte{"sha1-" + strings.Repeat("a", 40): first, "sha1-" + strings.Repeat("b", 40): second}}
	return root, opts
}
func TestVendorCopiesCompleteSourceAndUpdatesTransactionally(t *testing.T) {
	root, opts := vendorOptions(t)
	r, err := Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success || r.Changed {
		t.Fatalf("preview %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, VendorStatePath)); !os.IsNotExist(err) {
		t.Fatal("vendor preview wrote repository")
	}
	opts.Write = true
	r, err = Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success || !r.Changed {
		t.Fatalf("vendor %+v %v", r, err)
	}
	for _, name := range []string{"action.yml", "dist/index.js", "LICENSE"} {
		if _, err := os.Stat(filepath.Join(root, VendorRoot, "a/root", name)); err != nil {
			t.Fatalf("missing full source file %s: %v", name, err)
		}
	}
	workflow, _ := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if !strings.Contains(string(workflow), "$/.github/sanad/vendor/a/root") {
		t.Fatal("workflow not redirected to snapshot")
	}
	opts.To = "v1.1.0"
	r, err = Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success {
		t.Fatalf("vendor update %+v %v", r, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, VendorRoot, "a/root/dist/index.js"))
	if !strings.Contains(string(data), "second") {
		t.Fatal("vendor code not updated")
	}
	opts.Offline = true
	opts.Write = false
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Success {
		t.Fatalf("offline vendor %+v %v", r, err)
	}
	if err := os.WriteFile(filepath.Join(root, VendorRoot, "a/root/dist/index.js"), []byte("local edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.Write = true
	_, err = Run(context.Background(), "vendor", opts)
	if err == nil {
		t.Fatal("vendor overwrote a local edit")
	}
}
func TestVendorChecksUpstreamBytesAndRejectsUnexpectedFiles(t *testing.T) {
	root, opts := vendorOptions(t)
	opts.Write = true
	r, err := Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	// Forging the manifest alongside an edit still fails the online archive check.
	files, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadVendors(files)
	if err != nil {
		t.Fatal(err)
	}
	name := VendorRoot + "a/root/dist/index.js"
	f := files[name]
	f.Data = []byte("forged")
	files[name] = f
	entry := state.Repositories["a/root"]
	record := entry.Files[name]
	record.SHA256 = fileSHA(f.Data)
	entry.Files[name] = record
	state.Repositories["a/root"] = entry
	if err := saveVendors(files, state); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{name, VendorStatePath} {
		if err := putFile(root, name, files[name]); err != nil {
			t.Fatal(err)
		}
	}
	_, err = Run(context.Background(), "check", opts)
	if err == nil || !strings.Contains(err.Error(), "differs from upstream") {
		t.Fatalf("forged provenance accepted: %v", err)
	}
	if err := putFile(root, VendorRoot+"a/root/unexpected.js", file{Data: []byte("unexpected"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), "status", opts)
	if err == nil || !strings.Contains(err.Error(), "unexpected file") {
		t.Fatalf("unexpected code accepted: %v", err)
	}
}
func TestVendorArchiveRejectsUnsafePathsAndCollisions(t *testing.T) {
	for _, files := range []map[string]string{{"../escape": "x"}, {".git/config": "x"}, {".gitmodules": "x"}, {"a": "x", "A": "y"}, {"CON": "x"}} {
		if _, err := unpackArchive(sourceArchive(t, files), "a/root"); err == nil {
			t.Fatalf("accepted unsafe archive %v", files)
		}
	}
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tr := tar.NewWriter(gz)
	if err := tr.WriteHeader(&tar.Header{Name: "repo/link", Typeflag: tar.TypeSymlink, Linkname: "/outside"}); err != nil {
		t.Fatal(err)
	}
	tr.Close()
	gz.Close()
	if _, err := unpackArchive(out.Bytes(), "a/root"); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestVendorUpdateWaitsWithoutReplacingSource(t *testing.T) {
	root, opts := vendorOptions(t)
	opts.Write = true
	r, err := Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	config := []byte("version=1\n[native]\nenabled=true\n[observation]\nminimum_age='7d'\n")
	if err := putFile(root, ConfigPath, file{Data: config, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts.To = "v1.1.0"
	r, err = Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].Status != "pending" {
		t.Fatalf("pending %+v %v", r, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, VendorRoot, "a/root/dist/index.js"))
	if !strings.Contains(string(data), "first") {
		t.Fatal("pending source was installed")
	}
	opts.Now = opts.Now.AddDate(0, 0, 8)
	r, err = Run(context.Background(), "vendor", opts)
	if err != nil || !r.Success || r.Decisions[0].Status != "eligible" {
		t.Fatalf("mature %+v %v", r, err)
	}
}
func TestVendorNewFilesNeverOverwriteUntrackedData(t *testing.T) {
	root := t.TempDir()
	name := VendorRoot + "a/root/data.txt"
	if err := putFile(root, name, file{Data: []byte("untracked"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	before, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	after := copyTree(before)
	after[name] = file{Data: []byte("replacement"), Mode: 0o644}
	if err := applyTree(root, ConfigPath, before, after); err == nil {
		t.Fatal("overwrote untracked vendor data")
	}
	data, _ := os.ReadFile(filepath.Join(root, name))
	if string(data) != "untracked" {
		t.Fatal("untracked data changed")
	}
}
