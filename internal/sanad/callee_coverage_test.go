package sanad

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

func TestCalleeCoverageRequiresEveryVisibleActionPin(t *testing.T) {
	const path = ".github/workflows/build.yml"
	cases := []struct {
		name, uses string
		keys       []string
		deps       map[string]lock.Action
		want       bool
	}{
		{"missing-pin", "a/root@v1.0.0", []string{}, map[string]lock.Action{}, false},
		{"pin-outside-workflow", "a/root@v1.0.0", []string{}, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, false},
		{"covered", "a/root@v1.0.0", []string{"a/root@v1.0.0"}, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, true},
		{"uninspected-local-composite", "$/local", []string{}, map[string]lock.Action{}, false},
		{"inline-sha-mismatch", "a/root@" + strings.Repeat("a", 40), []string{"a/root@" + strings.Repeat("a", 40)}, map[string]lock.Action{"a/root@" + strings.Repeat("a", 40): action("v1.0.0", "b")}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newAPI("github.com")
			api.tokens["github.com"] = "test"
			workflowData := []byte("on: workflow_call\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: " + tc.uses + "\n")
			lockData, err := yaml.Marshal(lock.File{Version: lock.Version, Dependencies: tc.deps, Workflows: map[string][]string{path: tc.keys}})
			if err != nil {
				t.Fatal(err)
			}
			api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.URL.Path == "/repos/org/ci":
					return response(200, `{"id":4,"owner":{"id":3},"default_branch":"main"}`), nil
				case strings.Contains(req.URL.Path, "/commits/"):
					return response(200, `{"sha":"`+strings.Repeat("f", 40)+`"}`), nil
				case strings.Contains(req.URL.Path, "/compare/"):
					return response(200, `{"status":"identical"}`), nil
				case strings.Contains(req.URL.Path, "/git/ref/tags/"):
					return response(200, `{"ref":"refs/tags/v1.0.0"}`), nil
				case strings.Contains(req.URL.Path, "/contents/"):
					data := workflowData
					if strings.HasSuffix(req.URL.Path, "actions.lock") {
						data = lockData
					}
					return response(200, `{"type":"file","encoding":"base64","content":"`+base64.StdEncoding.EncodeToString(data)+`"}`), nil
				default:
					t.Fatalf("unexpected endpoint %s", req.URL)
					return nil, nil
				}
			})}
			pin, err := api.Workflow(context.Background(), "github.com", "org/ci/"+path, "v1.0.0", "")
			if tc.name == "inline-sha-mismatch" {
				if err == nil {
					t.Fatal("inconsistent inline SHA was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pin.CalleeLocked != tc.want {
				t.Fatalf("callee coverage = %v, want %v", pin.CalleeLocked, tc.want)
			}
		})
	}
}
