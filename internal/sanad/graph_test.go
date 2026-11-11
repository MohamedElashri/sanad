package sanad

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestObservationKeysSupportGitRefDelimiters(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@release|candidate": action("release|candidate", "b")}})
	opts.Action, opts.To, opts.Write = "a/root", "release|candidate", true
	if r, err := Run(context.Background(), "update", opts); err != nil || !r.Success || !r.Changed {
		t.Fatal(r, err)
	}
	if r, err := Run(context.Background(), "status", opts); err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].Action != "a/root@release|candidate" {
		t.Fatalf("observation cannot be read: %+v %v", r, err)
	}
}

func TestLegacyObservationMigrationPreservesMaturity(t *testing.T) {
	for _, ref := range []string{"v1.0.0", "release|candidate", "release%name"} {
		d := Dependency{Host: "github.com", OwnerID: 1, RepoID: 2, Key: "a/root@" + ref, Commit: "sha1-test"}
		at := testNow.Add(-8 * 24 * time.Hour)
		legacy := observations{Version: 1, Seen: map[string]time.Time{"github.com|1|2|" + d.Key + "|" + d.Commit: at}}
		b, _ := json.Marshal(legacy)
		files := tree{StatePath: {Data: b, Mode: 0o644}}
		got, err := loadObservations(files, testNow)
		if err != nil || got.Version != 2 || !got.Seen[observationKey(d)].Equal(at) {
			t.Fatalf("legacy maturity lost: %+v %v", got, err)
		}
	}
}
