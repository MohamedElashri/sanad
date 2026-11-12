package sanad

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func FuzzWorkflowReferenceRewrite(f *testing.F) {
	f.Add([]byte("on: push\njobs:\n  a:\n    steps:\n      - uses: owner/repo@v1.0.0\n"), true)
	f.Add([]byte("x: &step {uses: 'owner/repo/path@main'}\njobs:\n  a:\n    steps:\n      - parallel: [*step, {uses: owner/other@v2.0.0}]\n"), true)
	f.Add([]byte("runs:\n  using: composite\n  steps: [{uses: owner/repo@v1.0.0}]\n"), false)
	f.Fuzz(func(t *testing.T, data []byte, workflow bool) {
		if len(data) > 32<<10 {
			return
		}
		name := "action.yml"
		if workflow {
			name = ".github/workflows/ci.yml"
		}
		files := tree{name: {Data: append([]byte{}, data...), Mode: 0o644}}
		active := map[string]bool{name: true}
		refs, err := referencesIn(files, active)
		if err != nil {
			return
		}
		if err := rewriteRefs(files, refs, "v9.9.9"); err != nil {
			return
		} // Source escaping that cannot be preserved is an expected error.
		after, err := referencesIn(files, active)
		if err != nil {
			t.Fatalf("successful rewrite produced invalid metadata: %v", err)
		}
		if len(after) != len(refs) {
			t.Fatalf("rewrite changed call count: %d -> %d", len(refs), len(after))
		}
		for i, ref := range after {
			if ref.Name != refs[i].Name || ref.Workflow != refs[i].Workflow || ref.Ref != "v9.9.9" {
				t.Fatalf("rewrite changed call identity: %+v -> %+v", refs[i], ref)
			}
		}
	})
}
func FuzzObservationRoundTrip(f *testing.F) {
	f.Add([]byte(`{"version":2,"seen":{}}`))
	f.Add([]byte(`{"version":1,"seen":{"github.com|1|2|a/root@release|candidate|sha1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":"2026-01-01T00:00:00Z"}}`))
	f.Add([]byte(`{"version":2,"seen":{"[\"github.com\",\"1\",\"2\",\"a/root@v1.0.0\",\"sha1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"]":"2026-01-01T00:00:00Z"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32<<10 {
			return
		}
		state, err := loadObservations(tree{StatePath: {Data: data}}, testNow)
		if err != nil {
			return
		}
		files := tree{}
		if err := saveObservations(files, state); err != nil {
			t.Fatalf("accepted state cannot be saved: %v", err)
		}
		loaded, err := loadObservations(files, testNow)
		if err != nil {
			t.Fatalf("saved state cannot be loaded: %v", err)
		}
		if len(state.Seen) != len(loaded.Seen) {
			t.Fatal("normalization lost observations")
		}
		for key, at := range state.Seen {
			if !loaded.Seen[key].Equal(at) {
				t.Fatal("normalization changed observation age")
			}
		}
		second := tree{}
		if err := saveObservations(second, loaded); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(files[StatePath].Data, second[StatePath].Data) {
			t.Fatal("state encoding is not canonical")
		}
	})
}
func FuzzVendorArchive(f *testing.F) {
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tr := tar.NewWriter(gz)
	data := []byte("name: fixture\nruns: {using: node24, main: index.js}\n")
	if err := tr.WriteHeader(&tar.Header{Name: "repo-sha/action.yml", Mode: 0o644, Size: int64(len(data))}); err != nil {
		f.Fatal(err)
	}
	if _, err := tr.Write(data); err != nil {
		f.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		f.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(buffer.Bytes())
	f.Add([]byte("not gzip"))
	root := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32<<10 {
			return
		}
		files, err := unpackArchive(data, "owner/repo")
		if err != nil {
			return
		}
		if len(files) == 0 || len(files) > 10000 {
			t.Fatal("invalid accepted archive size")
		}
		folded := map[string]bool{}
		total := 0
		for name, file := range files {
			if !strings.HasPrefix(name, VendorRoot+"owner/repo/") {
				t.Fatal("archive escaped vendor root")
			}
			if _, err := safePath(root, name); err != nil {
				t.Fatalf("accepted unsafe path %q", name)
			}
			lower := strings.ToLower(name)
			if folded[lower] {
				t.Fatal("case-colliding output")
			}
			folded[lower] = true
			if file.Mode != 0o644 && file.Mode != 0o755 {
				t.Fatal("unsafe file mode")
			}
			total += len(file.Data)
		}
		if total > 64<<20 {
			t.Fatal("archive exceeded extraction limit")
		}
	})
}
