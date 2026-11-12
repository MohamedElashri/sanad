package sanad

import (
	"context"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestSharedSubactionsRespectEachReleaseCatalogRegardlessOfOrder(t *testing.T) {
	for _, order := range [][]string{{"tags", "releases"}, {"releases", "tags"}} {
		t.Run(strings.Join(order, "-"), func(t *testing.T) {
			root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
			f := workflow("a/root/" + order[0] + "@v1.0.0")
			f.Data = append(f.Data, []byte("      - uses: a/root/"+order[1]+"@v1.0.0\n")...)
			if err := putFile(root, ".github/workflows/ci.yml", f); err != nil {
				t.Fatal(err)
			}
			config := "version=1\n[native]\nenabled=true\n[actions.\"a/root/tags\"]\nprefer_tags=true\n[actions.\"a/root/releases\"]\nprefer_tags=false\n"
			if err := putFile(root, ConfigPath, file{Data: []byte(config), Mode: 0o644}); err != nil {
				t.Fatal(err)
			}
			opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b"), "a/root@v1.2.0": action("v1.2.0", "c")}})
			opts.Releases = taggedReleases{fakeReleases: fakeReleases{refs: []Release{{Ref: "v1.1.0"}}}, tags: []Release{{Ref: "v1.2.0"}}}
			r, err := Run(context.Background(), "plan", opts)
			if err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].To != "v1.1.0" {
				t.Fatalf("catalog policy depends on callsite order: %+v %v", r, err)
			}
		})
	}
}
