package sanad

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

type fakeWorkflows struct{ pins map[string]WorkflowPin }

func (f fakeWorkflows) Workflow(_ context.Context, host, name, ref, sha string) (WorkflowPin, error) {
	p, ok := f.pins[ref]
	if !ok {
		return p, fmt.Errorf("unknown workflow %s", ref)
	}
	if sha != "" && p.Commit != sha {
		return p, fmt.Errorf("wrong frozen workflow")
	}
	return p, nil
}
func reusableFixture(t *testing.T, minimum string) (string, Options) {
	t.Helper()
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, minimum)
	f, _ := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	f = append(f, []byte("  reuse:\n    uses: org/ci/.github/workflows/build.yml@v1.0.0\n")...)
	if err := putFile(root, ".github/workflows/ci.yml", file{Data: f, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	opts.Workflows = fakeWorkflows{map[string]WorkflowPin{
		"v1.0.0": {Name: "org/ci/.github/workflows/build.yml", Ref: "v1.0.0", Commit: strings.Repeat("c", 40), Host: "github.com", OwnerID: 3, RepoID: 4, CalleeLocked: true},
		"v1.1.0": {Name: "org/ci/.github/workflows/build.yml", Ref: "v1.1.0", Commit: strings.Repeat("d", 40), Host: "github.com", OwnerID: 3, RepoID: 4},
	}}
	return root, opts
}
func TestReusableWorkflowPinPreviewUpdateAndOfflineCheck(t *testing.T) {
	root, opts := reusableFixture(t, "")
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || !r.Success || len(r.Files) != 2 {
		t.Fatalf("preview %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, WorkflowStatePath)); !os.IsNotExist(err) {
		t.Fatal("preview wrote tracking state")
	}
	opts.Write = true
	r, err = Run(context.Background(), "sync", opts)
	if err != nil || !r.Changed {
		t.Fatalf("sync %+v %v", r, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if !strings.Contains(string(data), "build.yml@"+strings.Repeat("c", 40)) {
		t.Fatal("call was not SHA pinned")
	}
	opts.Action = "org/ci/.github/workflows/build.yml"
	opts.To = "v1.1.0"
	r, err = Run(context.Background(), "update", opts)
	if err != nil || !r.Success || len(r.Decisions) != 1 || r.Decisions[0].Status != "eligible" {
		t.Fatalf("update %+v %v", r, err)
	}
	data, _ = os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if !strings.Contains(string(data), "build.yml@"+strings.Repeat("d", 40)) {
		t.Fatal("workflow did not advance")
	}
	opts.Offline = true
	opts.Write = false
	r, err = Run(context.Background(), "check", opts)
	if err != nil || !r.Success {
		t.Fatalf("offline %+v %v", r, err)
	}
}
func TestReusableWorkflowObservationsAndIdentity(t *testing.T) {
	root, opts := reusableFixture(t, "7d")
	opts.Write = true
	// An existing native lockfile makes additions subject to adoption policy.
	r, err := Run(context.Background(), "sync", opts)
	if err != nil || r.Blocked() || len(r.Decisions) != 1 || r.Decisions[0].Status != "pending" {
		t.Fatalf("observation %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, WorkflowStatePath)); !os.IsNotExist(err) {
		t.Fatal("pending call adopted")
	}
	opts.Now = opts.Now.AddDate(0, 0, 8)
	r, err = Run(context.Background(), "sync", opts)
	if err != nil || !r.Changed {
		t.Fatalf("mature %+v %v", r, err)
	}
	f := opts.Workflows.(fakeWorkflows)
	p := f.pins["v1.0.0"]
	p.OwnerID = 99
	f.pins["v1.0.0"] = p
	opts.Workflows = f
	r, err = Run(context.Background(), "check", opts)
	if err == nil || !r.Blocked() {
		t.Fatalf("identity changed %+v %v", r, err)
	}
}
func TestWorkflowAPIInspectsExactCommitAndRejectsForkOnlySHA(t *testing.T) {
	for _, status := range []string{"ahead", "behind"} {
		t.Run(status, func(t *testing.T) {
			api := newAPI("github.com")
			api.tokens["github.com"] = "test"
			sha := strings.Repeat("e", 40)
			contentCalls := 0
			api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.URL.Path == "/repos/org/ci":
					return response(200, `{"id":4,"owner":{"id":3},"default_branch":"main"}`), nil
				case strings.Contains(req.URL.Path, "/commits/"):
					return response(200, `{"sha":"`+sha+`"}`), nil
				case strings.Contains(req.URL.Path, "/compare/"):
					return response(200, `{"status":"`+status+`"}`), nil
				case strings.Contains(req.URL.Path, "/contents/"):
					contentCalls++
					if req.URL.Query().Get("ref") != sha {
						t.Fatal("callee inspection used floating ref")
					}
					if strings.HasSuffix(req.URL.Path, "actions.lock") {
						return response(404, ""), nil
					}
					b := base64.StdEncoding.EncodeToString([]byte("on: workflow_call\njobs: {}\n"))
					return response(200, `{"type":"file","encoding":"base64","content":"`+b+`"}`), nil
				default:
					t.Fatalf("unexpected endpoint: %s", req.URL)
					return nil, nil
				}
			})}
			p, err := api.Workflow(context.Background(), "github.com", "org/ci/.github/workflows/build.yml", sha, sha)
			if status == "behind" {
				if err == nil || contentCalls != 0 {
					t.Fatal("accepted fork-only commit")
				}
				return
			}
			if err != nil || p.Commit != sha || p.CalleeLocked || contentCalls != 2 {
				t.Fatalf("pin %+v %v", p, err)
			}
		})
	}
}

func TestReusableWorkflowFamilyTagsCannotRetargetFrozenPins(t *testing.T) {
	api := newAPI("github.com")
	api.tokens["github.com"] = "test"
	api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/repos/org/ci":
			return response(200, `{"id":4,"owner":{"id":3},"default_branch":"main"}`), nil
		case strings.Contains(req.URL.Path, "/commits/"):
			return response(200, `{"sha":"`+strings.Repeat("b", 40)+`"}`), nil
		case strings.Contains(req.URL.Path, "/compare/"):
			return response(200, `{"status":"ahead"}`), nil
		case strings.Contains(req.URL.Path, "/git/ref/tags/"):
			return response(200, `{"ref":"refs/tags/build/v1.0.0"}`), nil
		default:
			t.Fatalf("unexpected endpoint after retarget: %s", req.URL)
			return nil, nil
		}
	})}
	if _, err := api.Workflow(context.Background(), "github.com", "org/ci/.github/workflows/build.yml", "build/v1.0.0", strings.Repeat("a", 40)); err == nil {
		t.Fatal("retargeted family tag was accepted")
	}
}
