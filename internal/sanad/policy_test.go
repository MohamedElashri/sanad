package sanad

import (
	"context"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestCooldownRequiresOnlineTrustedBase(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ".github/dependabot.yml", file{Data: []byte("version: 2\nupdates:\n - package-ecosystem: github-actions\n   cooldown:\n     default-days: 7\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	if _, err := Run(context.Background(), "check", opts); err == nil {
		t.Fatal("cooldown check accepted a missing trusted base")
	}
	opts.Offline = true
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("offline check accepted cooldown policy: %+v %v", r, err)
	}
}

func TestTransitiveConstraintsSelectOlderAllowedCandidate(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a", "c/child@v1.0.0"), "c/child@v1.0.0": action("v1.0.0", "b")}, "")
	if err := putFile(root, ConfigPath, file{Data: []byte("version=1\n[native]\nenabled=true\n[actions.\"c/child\"]\nconstraint='< 2'\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{t: t, definitions: map[string]lock.Action{
		"a/root@v1.2.0":  action("v1.2.0", "c", "c/child@v2.0.0"),
		"c/child@v2.0.0": action("v2.0.0", "d"),
		"a/root@v1.1.0":  action("v1.1.0", "e", "c/child@v1.0.0"),
	}}
	opts := options(root, backend)
	opts.Write = true
	r, err := Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 2 || r.Decisions[0].Status != "blocked" || r.Decisions[1].To != "v1.1.0" {
		t.Fatalf("wrong constraint decisions: %+v %v", r, err)
	}
	files, _ := readTree(root, ConfigPath)
	if strings.Contains(string(files[lock.Path].Data), "c/child@v2.0.0") {
		t.Fatal("forbidden transitive version was installed")
	}
}

func TestTransitiveUpdateLevels(t *testing.T) {
	before := graph{"c/child@v1.0.0": {Key: "c/child@v1.0.0", Host: "github.com", Commit: "old"}}
	for _, level := range []string{"patch", "minor"} {
		cfg := DefaultConfig()
		cfg.Update.Level = level
		after := graph{"c/child@v2.0.0": {Key: "c/child@v2.0.0", Host: "github.com", Commit: "new"}}
		if err := validateTransitivePolicies(before, after, "a/root@v1.1.0", cfg, Options{}); err == nil {
			t.Fatalf("%s allowed a transitive major update", level)
		}
	}
}

func TestTransitiveLevelsUseAnyCompatibleInstalledBaseline(t *testing.T) {
	before := graph{
		"c/child@v1.0.0": {Key: "c/child@v1.0.0", Host: "github.com", Commit: "one"},
		"c/child@v2.0.0": {Key: "c/child@v2.0.0", Host: "github.com", Commit: "two"},
	}
	for _, level := range []string{"patch", "minor"} {
		cfg := DefaultConfig()
		cfg.Update.Level = level
		for _, target := range []string{"v1.0.1", "v2.0.1", "v3.0.0"} {
			after := graph{"c/child@" + target: {Key: "c/child@" + target, Host: "github.com", Commit: "new"}}
			err := validateTransitivePolicies(before, after, "a/root@v1.1.0", cfg, Options{})
			if (err != nil) != (target == "v3.0.0") {
				t.Fatalf("%s policy with %s: %v", level, target, err)
			}
		}
	}
}

func TestSiblingPoliciesApplyRegardlessOfOrderAndRepositoryCase(t *testing.T) {
	for _, names := range [][]string{{"a/root/loose", "a/root/strict"}, {"a/root/strict", "a/root/loose"}} {
		root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
		f := workflow(names[0] + "@v1.0.0")
		f.Data = append(f.Data, []byte("      - uses: "+names[1]+"@v1.0.0\n")...)
		if err := putFile(root, ".github/workflows/ci.yml", f); err != nil {
			t.Fatal(err)
		}
		if err := putFile(root, ConfigPath, file{Data: []byte("version=1\n[native]\nenabled=true\n[actions.\"A/Root/strict\"]\nlevel='patch'\n"), Mode: 0o644}); err != nil {
			t.Fatal(err)
		}
		opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.0.1": action("v1.0.1", "b")}})
		opts.Releases = fakeReleases{refs: []Release{{Ref: "v1.2.0"}, {Ref: "v1.0.1"}}}
		r, err := Run(context.Background(), "plan", opts)
		if err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].To != "v1.0.1" {
			t.Fatalf("sibling policy ignored: %+v %v", r, err)
		}
	}
}

func TestScopedUpdatesPreserveSubpathCase(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	f := workflow("a/root/Build@v1.0.0")
	f.Data = append(f.Data, []byte("      - uses: a/root/build@v1.0.0\n")...)
	if err := putFile(root, ".github/workflows/ci.yml", f); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}})
	opts.Action, opts.To, opts.Write = "A/Root/Build", "v1.1.0", true
	r, err := Run(context.Background(), "update", opts)
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	files, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(files[".github/workflows/ci.yml"].Data)
	if !strings.Contains(got, "a/root/Build@v1.1.0") || !strings.Contains(got, "a/root/build@v1.0.0") {
		t.Fatalf("wrong subpath updated: %s", got)
	}
}

func TestExplicitDeveloperCoverageCheckSkipsAdoptionOnly(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	opts := options(root, &fakeBackend{t: t})
	opts.Offline = true
	opts.CoverageOnly = true
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Success || r.Verification != "offline" {
		t.Fatalf("developer coverage %+v %v", r, err)
	}
	opts.Base = "HEAD"
	_, err = Run(context.Background(), "check", opts)
	if err == nil {
		t.Fatal("coverage-only accepted policy comparison")
	}
}
