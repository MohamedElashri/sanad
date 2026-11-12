package sanad

import (
	"context"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestSubactionInheritsRepositoryTagFamily(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@tool/v1.0.0": action("tool/v1.0.0", "a")}, "")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("a/root/build@tool/v1.0.0")); err != nil {
		t.Fatal(err)
	}
	config := "version=1\n[native]\nenabled=true\n[actions.\"a/root\"]\ntag_prefix='tool/'\nprefer_tags=true\n[actions.\"a/root/build\"]\nlevel='patch'\n"
	if err := putFile(root, ConfigPath, file{Data: []byte(config), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@tool/v1.0.1": action("tool/v1.0.1", "b")}})
	opts.Write = true
	opts.Releases = taggedReleases{tags: []Release{{Ref: "tool/v1.0.1"}, {Ref: "tool/v1.1.0"}}}
	r, err := Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].Status != "eligible" || r.Decisions[0].To != "tool/v1.0.1" {
		t.Fatalf("repository tag policy was lost: %+v %v", r, err)
	}
}
