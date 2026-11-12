package sanad

import (
	"context"
	"strings"
	"testing"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestSourceIdentityIsBoundToRepositoryAcrossCallTypes(t *testing.T) {
	pairs := [][2]string{
		{"org/ci@v1.0.0", "org/ci/.github/workflows/build.yml@v2.0.0"},
		{"org/ci/.github/workflows/build.yml@v1.0.0", "org/ci/.github/workflows/deploy.yml@v2.0.0"},
		{"org/ci/.github/workflows/build.yml@v1.0.0", "org/ci@v2.0.0"},
		{"ORG/CI/.github/workflows/build.yml@v1.0.0", "org/ci/.github/workflows/deploy.yml@v2.0.0"},
	}
	for _, pair := range pairs {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			old := Dependency{Key: pair[0], Host: "github.com", OwnerID: 1, RepoID: 2, Commit: "sha1-" + strings.Repeat("a", 40)}
			next := old
			next.Key = pair[1]
			next.Commit = "sha1-" + strings.Repeat("b", 40)
			next.OwnerID = 9
			_, _, err := assess(graph{old.Key: old}, graph{next.Key: next}, observations{Seen: map[string]time.Time{}}, map[string]time.Time{}, 0, testNow)
			if err == nil {
				t.Fatal("changing the call type or workflow path hid a repository identity change")
			}
			next.OwnerID = old.OwnerID
			ready, _, err := assess(graph{old.Key: old}, graph{next.Key: next}, observations{Seen: map[string]time.Time{}}, map[string]time.Time{}, 0, testNow)
			if err != nil || !ready {
				t.Fatalf("same repository identity was rejected: %v", err)
			}
		})
	}
}
func TestTrustedBaseRejectsIdentityChangeFromActionToWorkflow(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"org/ci@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("org/ci@v1.0.0")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "trusted action identity")
	base := git(t, root, "rev-parse", "HEAD")
	pin := WorkflowPin{Name: "org/ci/.github/workflows/build.yml", Ref: "v2.0.0", Commit: strings.Repeat("b", 40), Host: "github.com", OwnerID: 9, RepoID: 2, CalleeLocked: true}
	files := tree{lock.Path: yamlLock(t, map[string]lock.Action{}), ".github/workflows/ci.yml": {Data: []byte("on: push\njobs:\n  build:\n    uses: " + pin.Name + "@" + pin.Commit + "\n"), Mode: 0o644}}
	if err := saveWorkflowState(files, workflowState{Version: 1, Pins: map[string]WorkflowPin{workflowKey(pin.Name, pin.Commit): pin}}); err != nil {
		t.Fatal(err)
	}
	for name, f := range files {
		if err := putFile(root, name, f); err != nil {
			t.Fatal(err)
		}
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Base = base
	opts.Workflows = fakeWorkflows{pins: map[string]WorkflowPin{pin.Ref: pin}}
	r, err := Run(context.Background(), "check", opts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range r.Findings {
		found = found || f.Code == "source-identity-changed"
	}
	if r.Success || !found {
		t.Fatalf("trusted identity was bypassed: %+v", r)
	}
}
