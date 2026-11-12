package sanad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestActionAndWorkflowUpdatesShareTrustedObservationsAndPublisherGuard(t *testing.T) {
	root, opts := reusableFixture(t, "")
	opts.Write = true
	opts.Backend = &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}}
	opts.Releases = fakeReleases{refs: []Release{{Ref: "v1.1.0"}}}
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	config := `version=1
[native]
enabled=true
[observation]
minimum_age='7d'
[publisher]
actions=['published/action.yml']
[trust]
require_approval=true
[trust.repositories."github.com/a/root"]
owner_id=1
repo_id=2
[trust.repositories."github.com/org/ci"]
owner_id=3
repo_id=4
`
	if err := putFile(root, ConfigPath, file{Data: []byte(config), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	exported := []byte("name: exported\nruns:\n  using: composite\n  steps:\n    - uses: a/root@" + strings.Repeat("a", 40) + "\n")
	if err := putFile(root, "published/action.yml", file{Data: exported, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "trusted baseline and publisher policy")
	r, err = Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 2 || !r.Changed || len(r.Files) != 1 || r.Files[0].Path != StatePath {
		t.Fatalf("pending transaction %+v %v", r, err)
	}
	for _, d := range r.Decisions {
		if d.Status != "pending" {
			t.Fatalf("candidate skipped its waiting period: %+v", d)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "reviewed observations")
	opts.Base = git(t, root, "rev-parse", "HEAD")
	opts.Now = opts.Now.AddDate(0, 0, 8)
	r, err = Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 2 {
		t.Fatalf("mature transaction %+v %v", r, err)
	}
	for _, d := range r.Decisions {
		if d.Status != "eligible" {
			t.Fatalf("mature candidate blocked: %+v", d)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "published/action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(exported) {
		t.Fatal("updating the consumer weakened published metadata")
	}
	opts.Write = false
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Success || r.Verification != "online" {
		t.Fatalf("trusted base rejected mature action/workflow updates: %+v %v", r, err)
	}
	opts.Action = "org/ci"
	r, err = Run(context.Background(), "explain", opts)
	if err != nil || len(r.Explanations) != 1 || r.Explanations[0].Status != "installed" || len(r.Explanations[0].Paths) == 0 {
		t.Fatalf("explanation lost installed workflow: %+v %v", r, err)
	}
	if markdown := r.Markdown(); !strings.Contains(markdown, "org/ci/.github/workflows/build.yml@v1.1.0") || !strings.Contains(markdown, "owner 3, repository 4") {
		t.Fatalf("review output lost workflow identity: %s", markdown)
	}
}
