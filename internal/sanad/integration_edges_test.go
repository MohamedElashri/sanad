package sanad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestParallelCallsAndNestedActionMetadataUseExecutionSyntax(t *testing.T) {
	root := t.TempDir()
	files := tree{
		".github/workflows/ci.yml":            {Data: []byte("on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - parallel:\n          - uses: a/root@v1.0.0\n          - uses: $/.github/workflows/helper\n      - run: echo ok\n        env:\n          run: ordinary-data\n        with:\n          uses: ./ignored\n"), Mode: 0o644},
		".github/workflows/helper/action.yml": {Data: []byte("name: helper\nruns:\n  using: composite\n  steps:\n    - uses: b/child@v1.0.0\n"), Mode: 0o644},
		"ignored/action.yml":                  {Data: []byte("this: [is invalid"), Mode: 0o644},
	}
	for name, f := range files {
		if err := putFile(root, name, f); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded["ignored/action.yml"]; ok {
		t.Fatal("input named uses was treated as an action call")
	}
	refs, err := references(loaded)
	if err != nil || len(refs) != 2 {
		t.Fatalf("references %+v %v", refs, err)
	}
	if refs[0].Name != "a/root" || refs[1].Name != "b/child" {
		t.Fatalf("parallel or nested action omitted: %+v", refs)
	}
	if err := rewriteRefs(loaded, refs[:1], "v1.1.0"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loaded[".github/workflows/ci.yml"].Data), "a/root@v1.1.0") {
		t.Fatal("parallel call was not rewritten")
	}
	protections, err := protections(loaded, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	runs := 0
	for _, p := range protections {
		if p.Kind == "runtime" {
			runs++
		}
	}
	if runs != 1 {
		t.Fatalf("ordinary env data was reported as execution: %+v", protections)
	}
}
func TestCoverageRequiresPinsForEveryCompositeCallsite(t *testing.T) {
	deps := map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}
	files := tree{lock.Path: yamlLock(t, deps), ".github/workflows/ci.yml": workflow("$/local"), ".github/workflows/uncovered.yml": workflow("$/local"), "local/action.yml": {Data: []byte("runs:\n  using: composite\n  steps:\n    - uses: a/root@v1.0.0\n"), Mode: 0o644}}
	items, err := protections(files, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range items {
		if p.Kind == "action" && p.Subject == "a/root@v1.0.0" {
			if p.Status != "uncovered" {
				t.Fatalf("pin from another workflow masked a coverage gap: %+v", p)
			}
			return
		}
	}
	t.Fatal("missing composite protection entry")
}
func TestUnusedWorkflowTrackerCannotSupplyAnInstalledBaseline(t *testing.T) {
	root, opts := reusableFixture(t, "")
	opts.Write = true
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	files, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadWorkflowState(files)
	if err != nil {
		t.Fatal(err)
	}
	orphan := opts.Workflows.(fakeWorkflows).pins["v1.1.0"]
	state.Pins[workflowKey(orphan.Name, orphan.Commit)] = orphan
	if err := saveWorkflowState(files, state); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, WorkflowStatePath, files[WorkflowStatePath]); err != nil {
		t.Fatal(err)
	}
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("unused tracker accepted %+v %v", r, err)
	}
	// Even if a trusted historical commit contains such a record, only executed
	// calls establish its installed baseline for a later adoption check.
	if err := putFile(root, ConfigPath, file{Data: []byte("version=1\n[native]\nenabled=true\n[observation]\nminimum_age='7d'\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "historical tracker")
	opts.Base = git(t, root, "rev-parse", "HEAD")
	for key, pin := range state.Pins {
		if pin.Commit != orphan.Commit {
			delete(state.Pins, key)
		}
	}
	files[WorkflowStatePath] = file{}
	if err := saveWorkflowState(files, state); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, WorkflowStatePath, files[WorkflowStatePath]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".github/workflows/ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), strings.Repeat("c", 40), orphan.Commit))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("unused baseline bypass %+v %v", r, err)
	}
	found := false
	for _, f := range r.Findings {
		found = found || f.Code == "observation-pending"
	}
	if !found {
		t.Fatalf("wrong baseline finding: %+v", r.Findings)
	}
}
func TestCoverageOnlyExplicitlySkipsSourceApprovals(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ConfigPath, file{Data: []byte("version=1\n[native]\nenabled=true\n[trust]\nrequire_approval=true\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Offline = true
	opts.CoverageOnly = true
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Success {
		t.Fatalf("explicit coverage-only %+v %v", r, err)
	}
}

func TestTenantTokenOnlyAuthenticationSeparatesPublicCredential(t *testing.T) {
	t.Setenv("GH_TOKEN", "tenant-token")
	t.Setenv("SANAD_GITHUB_COM_TOKEN", "public-token")
	t.Setenv("GH_ACTIONS_LOCK_GITHUB_COM_TOKEN", "untrusted-override")
	api := newAPI("customer.ghe.com")
	if got := api.token(context.Background(), "customer.ghe.com"); got != "tenant-token" {
		t.Fatal("wrong tenant credential")
	}
	if got := api.token(context.Background(), "github.com"); got != "public-token" {
		t.Fatal("wrong public credential")
	}
	found := false
	for _, pair := range backendEnv("/snapshot", "customer.ghe.com", "owner/repo") {
		if pair == "GH_ACTIONS_LOCK_GITHUB_COM_TOKEN=untrusted-override" {
			t.Fatal("ambient backend override retained")
		}
		found = found || pair == "GH_ACTIONS_LOCK_GITHUB_COM_TOKEN=public-token"
	}
	if !found {
		t.Fatal("explicit public credential not forwarded to tenant backend")
	}
	for _, pair := range backendEnv("/snapshot", "github.com", "owner/repo") {
		if strings.HasPrefix(pair, "GH_ACTIONS_LOCK_GITHUB_COM_TOKEN=") {
			t.Fatal("tenant-only credential override applied to public run")
		}
	}
}
