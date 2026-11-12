package sanad

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

func TestWorkflowAPIFailuresNeverEstablishCoverage(t *testing.T) {
	cases := []struct {
		name, endpoint string
		status         int
		body           string
		wantError      bool
	}{
		{"missing-repository", "repo", 404, "", true},
		{"server-failure", "repo", 500, "", true},
		{"missing-ref", "commit", 404, "", true},
		{"fork-only", "compare", 200, `{"status":"diverged"}`, true},
		{"missing-workflow", "workflow", 404, "", true},
		{"invalid-content", "workflow", 200, `{"type":"file","encoding":"base64","content":"not base64!"}`, true},
		{"wrong-content-kind", "workflow", 200, `{"type":"symlink","encoding":"base64","content":""}`, true},
		{"non-callable", "workflow", 200, encodedContent([]byte("on: push\njobs: {}\n")), true},
		{"invalid-lock", "lock", 200, encodedContent([]byte("version: [broken")), true},
		{"missing-lock", "lock", 404, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newAPI("github.com")
			api.tokens["github.com"] = "test"
			api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				endpoint := ""
				body := ""
				switch {
				case req.URL.Path == "/repos/org/ci":
					endpoint = "repo"
					body = `{"id":4,"owner":{"id":3},"default_branch":"main"}`
				case strings.Contains(req.URL.Path, "/commits/"):
					endpoint = "commit"
					body = `{"sha":"` + strings.Repeat("a", 40) + `"}`
				case strings.Contains(req.URL.Path, "/compare/"):
					endpoint = "compare"
					body = `{"status":"identical"}`
				case strings.Contains(req.URL.Path, "/git/ref/tags/"):
					endpoint = "tag"
					body = `{"ref":"refs/tags/v1.0.0"}`
				case strings.HasSuffix(req.URL.Path, "/actions.lock"):
					endpoint = "lock"
					body = encodedContent([]byte("version: v0.0.3\nworkflows:\n  .github/workflows/build.yml: []\ndependencies: {}\n"))
				case strings.Contains(req.URL.Path, "/contents/"):
					endpoint = "workflow"
					body = encodedContent([]byte("on: workflow_call\njobs: {}\n"))
				default:
					t.Fatalf("unexpected request %s", req.URL)
					return nil, nil
				}
				if endpoint == tc.endpoint {
					return response(tc.status, tc.body), nil
				}
				return response(200, body), nil
			})}
			pin, err := api.Workflow(context.Background(), "github.com", "org/ci/.github/workflows/build.yml", "v1.0.0", "")
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected API result: %+v %v", pin, err)
			}
			if pin.CalleeLocked {
				t.Fatal("failed or absent source metadata was reported as covered")
			}
		})
	}
}
func encodedContent(data []byte) string {
	return `{"type":"file","encoding":"base64","content":"` + base64.StdEncoding.EncodeToString(data) + `"}`
}
