package sanad

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

var testNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func action(ref, sha string, uses ...string) lock.Action {
	return lock.Action{Ref: ref, Commit: "sha1-" + strings.Repeat(sha, 40), OwnerID: 1, RepoID: 2, Uses: uses}
}
func yamlLock(t *testing.T, deps map[string]lock.Action) file {
	t.Helper()
	keys := []string{}
	for key := range deps {
		keys = append(keys, key)
	}
	b, err := yaml.Marshal(lock.File{Version: lock.Version, Dependencies: deps, Workflows: map[string][]string{".github/workflows/ci.yml": keys}})
	if err != nil {
		t.Fatal(err)
	}
	return file{Data: b, Mode: 0o644}
}
func workflow(value string) file {
	return file{Data: []byte("name: test\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: " + value + "\n"), Mode: 0o644}
}
func fixture(t *testing.T, deps map[string]lock.Action, minimum string) string {
	t.Helper()
	root := t.TempDir()
	files := tree{".github/workflows/ci.yml": workflow("a/root@v1.0.0"), lock.Path: yamlLock(t, deps), ConfigPath: {Data: []byte("version=1\n[native]\nenabled=true\n[observation]\nminimum_age=\"" + minimum + "\"\n"), Mode: 0o644}}
	for name, f := range files {
		if err := putFile(root, name, f); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type fakeBackend struct {
	t           *testing.T
	definitions map[string]lock.Action
	fail        bool
	calls       int
}

func (b *fakeBackend) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	b.calls++
	valid := true
	r := NativeResult{CLI: BackendVersion, Schema: lock.Version, Valid: &valid}
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	files, err := readTree(root, ConfigPath)
	if err != nil {
		return r, err
	}
	for _, arg := range args {
		if arg == "--verify" || arg == "--verify-local" {
			if _, ok := files[lock.Path]; !ok {
				return r, errors.New("missing native lock")
			}
			_, err := loadGraph(files, host)
			return r, err
		}
	}
	var old lock.File
	if f, ok := files[lock.Path]; ok {
		old, err = lock.Parse(f.Data)
		if err != nil {
			return r, err
		}
	}
	deps := map[string]lock.Action{}
	var add func(string) error
	add = func(key string) error {
		if _, ok := deps[key]; ok {
			return nil
		}
		a, ok := old.Dependencies[key]
		if !ok {
			a, ok = b.definitions[key]
		}
		if !ok {
			return errors.New("missing definition: " + key)
		}
		deps[key] = a
		for _, child := range a.Uses {
			if err := add(child); err != nil {
				return err
			}
		}
		return nil
	}
	refs, err := references(files)
	if err != nil {
		return r, err
	}
	for _, ref := range refs {
		if err := add(strings.ToLower(ref.NWO) + "@" + ref.Ref); err != nil {
			return r, err
		}
	}
	f := yamlLock(b.t, deps)
	if err := putFile(root, lock.Path, f); err != nil {
		return r, err
	}
	if b.fail {
		return r, errors.New("producer failed after staging")
	}
	// Match the official producer's pre-fix diagnosis on exit 0.
	invalid := false
	r.Valid = &invalid
	return r, nil
}

type fakeReleases struct {
	refs []Release
	at   time.Time
}

func (f fakeReleases) List(context.Context, string, string) ([]Release, error)  { return f.refs, nil }
func (f fakeReleases) Timestamp(context.Context, Dependency) (time.Time, error) { return f.at, nil }
func options(root string, b Backend) Options {
	return Options{Root: root, Repo: "example/test", Backend: b, Config: ConfigPath, Now: testNow, Releases: fakeReleases{refs: []Release{{Ref: "v1.2.0"}, {Ref: "v1.1.0"}}, at: testNow.Add(-30 * 24 * time.Hour)}}
}

func TestPreviewIsIsolatedAndUpdatePreservesUnrelatedPins(t *testing.T) {
	deps := map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a"), "b/other@v2.0.0": action("v2.0.0", "b")}
	root := fixture(t, deps, "")
	f := workflow("a/root@v1.0.0")
	f.Data = append(f.Data, []byte("      - uses: b/other@v2.0.0\n")...)
	if err := putFile(root, ".github/workflows/ci.yml", f); err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.2.0": action("v1.2.0", "c"), "b/other@v2.0.0": action("v2.0.0", "f")}}
	opts := options(root, backend)
	opts.Action = "a/root"
	before, _ := readTree(root, ConfigPath)
	report, err := Run(context.Background(), "plan", opts)
	if err != nil || !report.Success || report.Changed {
		t.Fatalf("preview: %+v %v", report, err)
	}
	after, _ := readTree(root, ConfigPath)
	if len(changedFiles(before, after)) != 0 {
		t.Fatal("preview modified repository")
	}
	opts.Write = true
	report, err = Run(context.Background(), "update", opts)
	if err != nil || !report.Changed || !report.Success {
		t.Fatalf("update: %+v %v", report, err)
	}
	after, _ = readTree(root, ConfigPath)
	g, err := loadGraph(after, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	if g["b/other@v2.0.0"].Commit != "sha1-"+strings.Repeat("b", 40) {
		t.Fatal("unrelated pin moved")
	}
	if _, ok := g["a/root@v1.2.0"]; !ok {
		t.Fatal("selected action did not update")
	}
}

func TestTransitiveObservationBlocksThenSelectsOlderMatureCandidate(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.2.0": action("v1.2.0", "b", "c/nested@v2.0.0"), "c/nested@v2.0.0": action("v2.0.0", "c"), "a/root@v1.1.0": action("v1.1.0", "d")}}
	state := observations{Version: 1, Seen: map[string]time.Time{}}
	for _, d := range []Dependency{{Key: "a/root@v1.2.0", Host: "github.com", OwnerID: 1, RepoID: 2, Commit: "sha1-" + strings.Repeat("b", 40)}, {Key: "a/root@v1.1.0", Host: "github.com", OwnerID: 1, RepoID: 2, Commit: "sha1-" + strings.Repeat("d", 40)}} {
		state.Seen[observationKey(d)] = testNow.Add(-10 * 24 * time.Hour)
	}
	files := tree{}
	if err := saveObservations(files, state); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, StatePath, files[StatePath]); err != nil {
		t.Fatal(err)
	}
	opts := options(root, backend)
	opts.Write = true
	r, err := Run(context.Background(), "update", opts)
	if err != nil || !r.Success {
		t.Fatalf("%+v %v", r, err)
	}
	if len(r.Decisions) != 2 || r.Decisions[0].Status != "pending" || r.Decisions[1].To != "v1.1.0" || r.Decisions[1].Status != "eligible" {
		t.Fatalf("wrong decisions: %+v", r.Decisions)
	}
	after, _ := readTree(root, ConfigPath)
	g, _ := loadGraph(after, "github.com")
	if _, ok := g["a/root@v1.1.0"]; !ok {
		t.Fatal("older mature candidate was not installed")
	}
	observations, err := loadObservations(after, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations.Seen) != 3 {
		t.Fatalf("lost candidate histories: %+v", observations)
	}
}

func TestObservationOnlyWriteAndRetarget(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.2.0": action("v1.2.0", "b"), "a/root@v1.1.0": action("v1.1.0", "c")}}
	opts := options(root, backend)
	r, err := Run(context.Background(), "plan", opts)
	if err != nil || r.Changed {
		t.Fatal(r, err)
	}
	if _, err := os.Stat(filepath.Join(root, StatePath)); !os.IsNotExist(err) {
		t.Fatal("plan started observation clock")
	}
	opts.Write = true
	r, err = Run(context.Background(), "update", opts)
	if err != nil || !r.Changed || len(r.Files) != 1 || r.Files[0].Path != StatePath {
		t.Fatalf("observation-only: %+v %v", r, err)
	}
	opts.Now = testNow.Add(8 * 24 * time.Hour)
	backend.definitions["a/root@v1.2.0"] = action("v1.2.0", "d")
	r, err = Run(context.Background(), "update", opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.Decisions[0].Status != "pending" || r.Decisions[1].Status != "eligible" {
		t.Fatalf("retarget reused maturity: %+v", r.Decisions)
	}
}

func TestFailureAndConcurrentEditsLeaveExecutionStateUntouched(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	before, _ := readTree(root, ConfigPath)
	backend := &fakeBackend{t: t, fail: true, definitions: map[string]lock.Action{"a/root@v1.2.0": action("v1.2.0", "b")}}
	opts := options(root, backend)
	opts.Write = true
	if _, err := Run(context.Background(), "update", opts); err == nil {
		t.Fatal("expected generation failure")
	}
	after, _ := readTree(root, ConfigPath)
	if len(changedFiles(before, after)) != 0 {
		t.Fatal("failed update changed repository")
	}
	edited := copyTree(before)
	edited[".github/workflows/ci.yml"] = workflow("a/root@v1.2.0")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("a/root@manual")); err != nil {
		t.Fatal(err)
	}
	if err := applyTree(root, ConfigPath, before, edited); err == nil || !strings.Contains(err.Error(), "changed during") {
		t.Fatal("concurrent edit was not protected", err)
	}
}

func TestSyncCannotBypassWaitingPeriod(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("a/root@v1.2.0")); err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.2.0": action("v1.2.0", "b")}}
	opts := options(root, backend)
	opts.Write = true
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || len(r.Decisions) != 1 || r.Decisions[0].Status != "pending" {
		t.Fatalf("%+v %v", r, err)
	}
	files, _ := readTree(root, ConfigPath)
	g, _ := loadGraph(files, "github.com")
	if _, ok := g["a/root@v1.0.0"]; !ok {
		t.Fatal("sync advanced the lock")
	}
	if _, ok := files[StatePath]; !ok {
		t.Fatal("sync did not persist observations")
	}
}

func TestSourceIdentityAndFutureStateRejected(t *testing.T) {
	old := graph{"a/root@v1.0.0": {Key: "a/root@v1.0.0", Host: "github.com", RepoID: 1, OwnerID: 2, Commit: "sha1-" + strings.Repeat("a", 40)}}
	changed := graph{"a/root@v1.1.0": {Key: "a/root@v1.1.0", Host: "github.com", RepoID: 99, OwnerID: 2, Commit: "sha1-" + strings.Repeat("b", 40)}}
	if _, _, err := assess(old, changed, observations{}, map[string]time.Time{}, 0, testNow); err == nil {
		t.Fatal("identity substitution accepted")
	}
	f := tree{}
	s := observations{Version: 1, Seen: map[string]time.Time{"github.com|1|2|a/root@v1|sha1-a": testNow.Add(time.Hour)}}
	_ = saveObservations(f, s)
	if _, err := loadObservations(f, testNow); err == nil {
		t.Fatal("future observation accepted")
	}
	f[StatePath] = file{Data: []byte(`{"version":1,"seen":{}} {"version":1}`)}
	if _, err := loadObservations(f, testNow); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestConfigPolicyAndDuration(t *testing.T) {
	for _, text := range []string{"cooldown='7d'", "version=1\nunknown=true", "version=1\n[update]\nlevel='patch'\nconstraint='>1'", "version=1\n[observation]\nminimum_age='100000000d'"} {
		root := t.TempDir()
		_ = os.WriteFile(filepath.Join(root, ConfigPath), []byte(text), 0o644)
		if _, err := loadConfig(root, ConfigPath); err == nil {
			t.Fatalf("invalid config accepted: %s", text)
		}
	}
	d, err := duration("7d")
	if err != nil || d != 7*24*time.Hour {
		t.Fatal(d, err)
	}
	for _, text := range []string{"updates:\n - package-ecosystem: github-actions\n   cooldown:\n     semver-major-days: 7\n", "updates: [invalid", "updates:\n - package-ecosystem: github-actions\n - package-ecosystem: github-actions\n"} {
		root := t.TempDir()
		_ = putFile(root, ".github/dependabot.yml", file{Data: []byte(text), Mode: 0o644})
		if _, err := releaseCooldown(root); err == nil {
			t.Fatal("ambiguous or unsupported policy accepted")
		}
	}
}

func TestWorkflowFormattingAndReachability(t *testing.T) {
	text := "name: 'été'\r\non: push\r\njobs:\r\n  test:\r\n    steps: [{uses: 'a/root@v1.0.0'}, {uses: a/root/sub@v1.0.0}] # keep\r\n"
	files := tree{".github/workflows/ci.yml": {Data: []byte(text), Mode: 0o644}, "unused/action.yml": {Data: []byte("runs:\n  using: composite\n  steps:\n    - uses: a/root@v1.0.0\n"), Mode: 0o644}}
	refs, err := references(files)
	if err != nil || len(refs) != 2 {
		t.Fatal(refs, err)
	}
	if err := rewriteRefs(files, refs, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	if string(files[".github/workflows/ci.yml"].Data) != strings.ReplaceAll(text, "v1.0.0", "v1.2.0") {
		t.Fatal("formatting changed")
	}
	if !strings.Contains(string(files["unused/action.yml"].Data), "v1.0.0") {
		t.Fatal("unused action was rewritten")
	}
}

func TestRecoveryProtectsLaterChanges(t *testing.T) {
	root := t.TempDir()
	before := tree{".github/workflows/ci.yml": workflow("a/root@v1")}
	after := tree{".github/workflows/ci.yml": workflow("a/root@v2")}
	_ = putFile(root, ".github/workflows/ci.yml", after[".github/workflows/ci.yml"])
	b, _ := json.Marshal(journal{Before: before, After: after, Paths: []string{".github/workflows/ci.yml"}})
	_ = putFile(root, transactionPath, file{Data: b, Mode: 0o600})
	if err := recoverTransaction(root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if !reflect.DeepEqual(got, before[".github/workflows/ci.yml"].Data) {
		t.Fatal("not restored")
	}
	_ = putFile(root, transactionPath, file{Data: b, Mode: 0o600})
	_ = putFile(root, ".github/workflows/ci.yml", workflow("a/root@manual"))
	if err := recoverTransaction(root); err == nil {
		t.Fatal("recovery overwrote a later edit")
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	base := []string{"-C", root, "-c", "commit.gpgsign=false", "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "core.hooksPath=" + t.TempDir()}
	cmd := exec.Command("git", append(base, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(string(b), err)
	}
	return strings.TrimSpace(string(b))
}

func TestCheckUsesTrustedBaseObservations(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "baseline")
	base := git(t, root, "rev-parse", "HEAD")
	newDeps := map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}
	_ = putFile(root, lock.Path, yamlLock(t, newDeps))
	_ = putFile(root, ".github/workflows/ci.yml", workflow("a/root@v1.1.0"))
	files, _ := readTree(root, ConfigPath)
	g, _ := loadGraph(files, "github.com")
	s := observations{Version: 1, Seen: map[string]time.Time{}}
	for _, d := range g {
		s.Seen[observationKey(d)] = testNow.Add(-30 * 24 * time.Hour)
	}
	_ = saveObservations(files, s)
	_ = putFile(root, StatePath, files[StatePath])
	opts := options(root, &fakeBackend{t: t})
	opts.Base = base
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("PR supplied its own maturity: %+v %v", r, err)
	}
	if r.Findings[len(r.Findings)-1].Code != "observation-pending" {
		t.Fatalf("wrong finding: %+v", r.Findings)
	}
	opts.Base = ""
	if _, err := Run(context.Background(), "check", opts); err == nil {
		t.Fatal("policy check silently omitted trusted base")
	}
}

func TestInitialBaselineCanBootstrapObservationPolicy(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	lockData, err := os.ReadFile(filepath.Join(root, lock.Path))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, lock.Path)); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "before native adoption")
	base := git(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, lock.Path), lockData, 0o644); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Base = base
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Success {
		t.Fatalf("initial policy could not bootstrap: %+v %v", r, err)
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == "initial-baseline" {
			found = true
		}
	}
	if !found {
		t.Fatal("baseline acceptance was not surfaced for review")
	}
}

func TestBackendEnvironmentRemovesBypasses(t *testing.T) {
	t.Setenv("GH_ACTIONS_LOCK_INSECURE", "1")
	t.Setenv("GH_ACTIONS_LOCK_DEPENDABOT_PROXY", "1")
	t.Setenv("GH_ACTIONS_LOCK_CONFIG", "/untrusted")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	for _, value := range backendEnv("/snapshot", "github.com", "a/b") {
		if strings.HasPrefix(value, "GH_ACTIONS_LOCK_INSECURE=") || strings.HasPrefix(value, "GH_ACTIONS_LOCK_DEPENDABOT_PROXY=") || strings.HasPrefix(value, "GIT_CONFIG_COUNT=") {
			t.Fatalf("unsafe environment: %s", value)
		}
	}
}

func TestCrossHostCredentialsAndScopeAreProtected(t *testing.T) {
	t.Setenv("GH_TOKEN", "tenant-test-token")
	t.Setenv("GITHUB_TOKEN", "tenant-test-token")
	for _, pair := range storedCredentialEnv() {
		if strings.HasPrefix(pair, "GH_TOKEN=") || strings.HasPrefix(pair, "GITHUB_TOKEN=") {
			t.Fatal("cross-host credential lookup retained ambient token")
		}
	}
	before := graph{"a/root@v1": {Key: "a/root@v1", Commit: "old"}, "b/other@v1": {Key: "b/other@v1", Commit: "old"}}
	after := graph{"a/root@v2": {Key: "a/root@v2", Commit: "new"}, "b/other@v1": {Key: "b/other@v1", Commit: "changed"}}
	if err := validateScope(before, after, "a/root@v1", "a/root@v2"); err == nil {
		t.Fatal("producer moved an unrelated pin")
	}
}

func TestNativeBackendContract(t *testing.T) {
	path := os.Getenv("SANAD_TEST_BACKEND")
	if path == "" {
		t.Skip("set SANAD_TEST_BACKEND to the official v0.2.0 binary for the producer contract test")
	}
	if err := verifyBackend(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_TOKEN", "native-contract-test")
	t.Setenv("GITHUB_TOKEN", "")
	root := t.TempDir()
	files := tree{".github/workflows/ci.yml": workflow("./local"), "local/action.yml": {Data: []byte("name: local\nruns:\n  using: composite\n  steps:\n    - run: echo test\n      shell: bash\n"), Mode: 0o644}}
	for name, f := range files {
		if err := putFile(root, name, f); err != nil {
			t.Fatal(err)
		}
	}
	backend := NativeBackend{Path: path}
	opts := Options{Root: root, Repo: "example/test", ConfirmNative: true, Backend: backend, Now: testNow}
	r, err := Run(context.Background(), "init", opts)
	if err != nil || !r.Success || r.Changed {
		t.Fatalf("native preview: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, lock.Path)); !os.IsNotExist(err) {
		t.Fatal("native preview wrote original lock")
	}
	opts.Write = true
	r, err = Run(context.Background(), "init", opts)
	if err != nil || !r.Changed || !r.Success {
		t.Fatalf("native init: %+v %v", r, err)
	}
	bytes, _ := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if !strings.Contains(string(bytes), "$/local") {
		t.Fatal("native self-action migration did not run")
	}
	opts.Write = false
	opts.ConfirmNative = false
	opts.Offline = true
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Success || r.Verification != "offline" {
		t.Fatalf("offline: %+v %v", r, err)
	}
}

func TestCheckRejectsIncompleteNativeGraph(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps map[string]lock.Action
		keys []string
		want string
	}{
		{"missing workflow pin", map[string]lock.Action{}, []string{"a/root@v1.0.0"}, "references missing dependency"},
		{"missing transitive pin", map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0")}, []string{"a/root@v1.0.0", "b/child@v1.0.0"}, "references missing dependency"},
		{"missing uses target", map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0")}, nil, "uses missing dependency"},
		{"incomplete workflow closure", map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0"), "b/child@v1.0.0": action("v1.0.0", "b")}, []string{"a/root@v1.0.0"}, "omits transitive dependency"},
		{"complete graph", map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "b/child@v1.0.0"), "b/child@v1.0.0": action("v1.0.0", "b")}, []string{"a/root@v1.0.0", "b/child@v1.0.0"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, tc.deps, "")
			data, err := yaml.Marshal(lock.File{Version: lock.Version, Dependencies: tc.deps, Workflows: map[string][]string{".github/workflows/ci.yml": tc.keys}})
			if err != nil {
				t.Fatal(err)
			}
			if err := putFile(root, lock.Path, file{Data: data, Mode: 0o644}); err != nil {
				t.Fatal(err)
			}
			for _, offline := range []bool{false, true} {
				backend := &fakeBackend{t: t}
				opts := options(root, backend)
				opts.Offline = offline
				r, err := Run(context.Background(), "check", opts)
				if tc.want == "" {
					if err != nil || !r.Success {
						t.Fatalf("complete graph rejected: %+v %v", r, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tc.want) || r.Success || backend.calls != 0 {
					t.Fatalf("incomplete graph reached backend or passed (offline=%v): %+v %v, calls=%d", offline, r, err, backend.calls)
				}
			}
		})
	}
}
