package sanad

import (
	"context"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestProtectionSurfacesDoNotOverstateCoverage(t *testing.T) {
	sha := strings.Repeat("a", 64)
	data := `on: push
jobs:
  build:
    runs-on: ubuntu-24.04
    container: node:22
    services:
      db:
        image: postgres@sha256:` + sha + `
    steps:
      - uses: a/root@v1.0.0
      - uses: docker://alpine:latest
      - uses: actions/setup-node@v7
        with:
          node-version: 24
      - run: npx some-tool
  reuse:
    uses: org/ci/.github/workflows/build.yml@` + strings.Repeat("b", 40) + "\n"
	files := tree{".github/workflows/ci.yml": {Data: []byte(data), Mode: 0o644}, lock.Path: yamlLock(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")})}
	items, err := protections(files, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range items {
		got[p.Kind+":"+p.Status] = true
	}
	for _, key := range []string{"runner:unknown", "container:mutable", "container:digest", "runtime:uninspected", "runtime-tool:uninspected", "action:native-lock", "action:uncovered", "reusable-workflow:inline-sha"} {
		if !got[key] {
			t.Errorf("missing %s: %+v", key, items)
		}
	}
}
func TestZizmorSARIFIsExplicitAndCanBlockWrites(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	report := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"zizmor"}},"results":[{"ruleId":"unpinned-images","level":"error","message":{"text":"Pin the image digest"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":".github/workflows/ci.yml"},"region":{"startLine":8}}}]}]}]}`
	if err := putFile(root, "lint.sarif", file{Data: []byte(report), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Write = true
	opts.ZizmorReport = "lint.sarif"
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || !r.Blocked() || r.Changed {
		t.Fatalf("lint %+v %v", r, err)
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == "zizmor-unpinned-images" {
			found = true
		}
	}
	if !found {
		t.Fatal("linter finding omitted")
	}
	opts.ZizmorReport = "../outside.sarif"
	_, err = Run(context.Background(), "status", opts)
	if err == nil {
		t.Fatal("accepted report outside repository")
	}
}
