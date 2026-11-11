package sanad

import (
	"strings"
	"testing"
)

func TestAliasesAndAnchorsPreserveSource(t *testing.T) {
	for _, scalar := range []string{"&ref a/root@v1.0.0", "&ref 'a/root@v1.0.0'", "!!str &ref \"a/root@v1.0.0\""} {
		text := "name: aliases\non: push\njobs:\n  first: &job\n    runs-on: ubuntu-latest\n    steps: &steps\n      - uses: " + scalar + " # keep\n      - uses: *ref\n  second: *job\n  third:\n    runs-on: ubuntu-latest\n    steps: *steps\n"
		files := tree{".github/workflows/ci.yml": {Data: []byte(text), Mode: 0o644}}
		refs, err := references(files)
		if err != nil || len(refs) != 1 {
			t.Fatalf("alias references: %+v %v", refs, err)
		}
		if err := rewriteRefs(files, refs, "v1.2.0"); err != nil {
			t.Fatal(err)
		}
		if got := string(files[".github/workflows/ci.yml"].Data); got != strings.ReplaceAll(text, "v1.0.0", "v1.2.0") {
			t.Fatalf("formatting changed:\n%s", got)
		}
	}
}

func TestAliasedLocalActionIsReachable(t *testing.T) {
	files := tree{
		".github/workflows/ci.yml": {Data: []byte("name: alias\non: push\nenv:\n  local: &local $/local\njobs:\n  test:\n    steps:\n      - uses: *local\n"), Mode: 0o644},
		"local/action.yml":         {Data: []byte("runs:\n  using: composite\n  steps:\n    - uses: a/root@v1.0.0\n"), Mode: 0o644},
	}
	refs, err := references(files)
	if err != nil || len(refs) != 1 || refs[0].File != "local/action.yml" {
		t.Fatalf("local alias lost: %+v %v", refs, err)
	}
}
