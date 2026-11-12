package sanad

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestExplainShowsFullPathsAndCandidateWaits(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0"), "b/child@v1.0.0": action("v1.0.0", "b")}, "7d")
	for name, f := range (tree{".github/workflows/ci.yml": workflow("$/local"), "local/action.yml": {Data: []byte("name: local\nruns:\n  using: composite\n  steps:\n    - uses: a/root@v1.0.0\n"), Mode: 0o644}}) {
		if err := putFile(root, name, f); err != nil {
			t.Fatal(err)
		}
	}
	candidate := Dependency{Key: "b/child@v1.1.0", Host: "github.com", OwnerID: 1, RepoID: 2, Commit: "sha1-" + strings.Repeat("c", 40)}
	state := tree{}
	if err := saveObservations(state, observations{Version: 2, Seen: map[string]time.Time{observationKey(candidate): testNow}}); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, StatePath, state[StatePath]); err != nil {
		t.Fatal(err)
	}
	opts := options(root, nil)
	opts.Action = "b/child"
	r, err := Run(context.Background(), "explain", opts)
	if err != nil || len(r.Explanations) != 2 {
		t.Fatalf("explain %+v %v", r, err)
	}
	expected := []string{".github/workflows/ci.yml", "local/action.yml", "a/root@v1.0.0", "b/child@v1.0.0"}
	if !reflect.DeepEqual(r.Explanations[0].Paths, [][]string{expected}) {
		t.Fatalf("paths %+v", r.Explanations[0].Paths)
	}
	if e := r.Explanations[1]; e.Status != "pending" || e.EligibleAt == nil || !e.EligibleAt.Equal(testNow.Add(7*24*time.Hour)) {
		t.Fatalf("candidate %+v", e)
	}
}
func TestGraphReviewDistinguishesSourceIdentityFromCode(t *testing.T) {
	a := Dependency{Key: "a/root@v1.0.0", Host: "github.com", OwnerID: 1, RepoID: 2, Commit: "sha1-" + strings.Repeat("a", 40)}
	b := a
	b.Key = "a/root@v1.1.0"
	b.Commit = "sha1-" + strings.Repeat("b", 40)
	before := graph{a.Key: a}
	changes := graphDiff(before, graph{b.Key: b})
	for _, c := range changes {
		if c.After != nil {
			if !strings.Contains(c.CompareURL, "/compare/"+strings.Repeat("a", 40)+"..."+strings.Repeat("b", 40)) || c.NewSource || c.IdentityChanged {
				t.Fatalf("review %+v", c)
			}
		}
	}
	b.OwnerID = 9
	changes = graphDiff(before, graph{b.Key: b})
	for _, c := range changes {
		if c.After != nil && (!c.IdentityChanged || c.CompareURL != "") {
			t.Fatalf("identity %+v", c)
		}
	}
}
