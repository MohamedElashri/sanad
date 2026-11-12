package sanad

import (
	"context"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

const approvedRoot = `version=1
[native]
enabled=true
[trust]
require_approval=true
[trust.repositories."github.com/a/root"]
owner_id=1
repo_id=2
`

func TestTrustBlocksNewTransitiveSourceAndFallsBack(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ConfigPath, file{Data: []byte(approvedRoot), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{
		"a/root@v1.2.0":          action("v1.2.0", "b", "unreviewed/tool@v1.0.0"),
		"unreviewed/tool@v1.0.0": action("v1.0.0", "c"),
		"a/root@v1.1.0":          action("v1.1.0", "d"),
	}}
	opts := options(root, backend)
	opts.Write = true
	r, err := Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 2 || r.Decisions[0].Status != "blocked" || r.Decisions[1].Status != "eligible" {
		t.Fatalf("trust %+v %v", r, err)
	}
	files, _ := readTree(root, ConfigPath)
	if strings.Contains(string(files[lock.Path].Data), "unreviewed/tool") {
		t.Fatal("unapproved source was installed")
	}
}
func TestTrustApprovalsMustComeFromBase(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ConfigPath, file{Data: []byte(approvedRoot), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "approved policy")
	base := git(t, root, "rev-parse", "HEAD")
	config := approvedRoot + "\n[trust.repositories.\"github.com/new/action\"]\nowner_id=1\nrepo_id=2\n"
	if err := putFile(root, ConfigPath, file{Data: []byte(config), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, lock.Path, yamlLock(t, map[string]lock.Action{"new/action@v1.0.0": action("v1.0.0", "b")})); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, ".github/workflows/ci.yml", workflow("new/action@v1.0.0")); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Base = base
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("self-approval %+v %v", r, err)
	}
	opts.Base = ""
	_, err = Run(context.Background(), "check", opts)
	if err == nil {
		t.Fatal("trust check omitted trusted base")
	}
}
func TestTrustBindsNamesToNumericIdentity(t *testing.T) {
	p := TrustPolicy{RequireApproval: true, Repositories: map[string]SourceIdentity{"GitHub.com/Owner/Repo": {OwnerID: 1, RepoID: 2}}}
	if err := validateTrust(&p); err != nil {
		t.Fatal(err)
	}
	d := Dependency{Key: "owner/repo/path@v1.0.0", Host: "github.com", OwnerID: 9, RepoID: 2}
	findings := trustFindings(graph{d.Key: d}, p)
	if len(findings) != 1 || findings[0].Code != "source-identity-changed" {
		t.Fatalf("identity %+v", findings)
	}
}
