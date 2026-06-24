package githubresolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	commitSHA    = "11bd71901bbe5b1630ceea73d27597364c9af683"
	branchSHA    = "93397bea11091df50f3d7e59dc26a7711a8bcfbe"
	tagObjectSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestResolveFullSHAVerifiesCommit(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/actions/checkout/git/commits/"+commitSHA {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{
			"sha": commitSHA,
			"committer": map[string]string{
				"date": "2026-05-01T12:00:00Z",
			},
		})
	})

	client := newTestClient(t, server)
	got, err := client.Resolve(context.Background(), ActionSelector{
		Owner: "actions",
		Repo:  "checkout",
		Ref:   commitSHA,
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if got.Kind != KindSHA {
		t.Fatalf("Kind = %q, want %q", got.Kind, KindSHA)
	}
	if got.SHA != commitSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, commitSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-01T12:00:00Z")
}

func TestResolveBranch(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout/git/ref/tags/main":
			writeError(w, http.StatusNotFound, "Not Found")
		case "/repos/actions/checkout/git/ref/heads/main":
			writeJSON(t, w, gitRef("refs/heads/main", "commit", branchSHA))
		case "/repos/actions/checkout/git/commits/" + branchSHA:
			writeJSON(t, w, commit(branchSHA, "2026-05-02T12:00:00Z"))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	})

	client := newTestClient(t, server)
	got, err := client.Resolve(context.Background(), ActionSelector{
		Owner: "actions",
		Repo:  "checkout",
		Ref:   "main",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if got.Kind != KindBranch {
		t.Fatalf("Kind = %q, want %q", got.Kind, KindBranch)
	}
	if got.SHA != branchSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, branchSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-02T12:00:00Z")
}

func TestResolveDefaultBranch(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout":
			writeJSON(t, w, map[string]string{"default_branch": "main"})
		case "/repos/actions/checkout/git/ref/heads/main":
			writeJSON(t, w, gitRef("refs/heads/main", "commit", branchSHA))
		case "/repos/actions/checkout/git/commits/" + branchSHA:
			writeJSON(t, w, commit(branchSHA, "2026-05-02T12:00:00Z"))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	})

	client := newTestClient(t, server)
	got, err := client.ResolveDefaultBranch(context.Background(), "actions", "checkout")
	if err != nil {
		t.Fatalf("ResolveDefaultBranch returned error: %v", err)
	}

	if got.Kind != KindBranch {
		t.Fatalf("Kind = %q, want %q", got.Kind, KindBranch)
	}
	if got.Ref != "main" {
		t.Fatalf("Ref = %q, want main", got.Ref)
	}
	if got.SHA != branchSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, branchSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-02T12:00:00Z")
}

func TestResolveLightweightTagWithReleaseTimestamp(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout/git/ref/tags/v4":
			writeJSON(t, w, gitRef("refs/tags/v4", "commit", commitSHA))
		case "/repos/actions/checkout/git/commits/" + commitSHA:
			writeJSON(t, w, commit(commitSHA, "2026-05-03T12:00:00Z"))
		case "/repos/actions/checkout/releases/tags/v4":
			writeJSON(t, w, map[string]any{
				"tag_name":     "v4",
				"published_at": "2026-05-04T12:00:00Z",
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	})

	client := newTestClient(t, server)
	got, err := client.ResolveTag(context.Background(), "actions", "checkout", "v4")
	if err != nil {
		t.Fatalf("ResolveTag returned error: %v", err)
	}

	if got.Kind != KindTag {
		t.Fatalf("Kind = %q, want %q", got.Kind, KindTag)
	}
	if got.SHA != commitSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, commitSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-03T12:00:00Z")
	assertTimePtr(t, got.ReleaseTime, "2026-05-04T12:00:00Z")
	if got.TagTime != nil {
		t.Fatalf("TagTime = %v, want nil for lightweight tag", got.TagTime)
	}
}

func TestResolveAnnotatedTag(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout/git/ref/tags/v4":
			writeJSON(t, w, gitRef("refs/tags/v4", "tag", tagObjectSHA))
		case "/repos/actions/checkout/git/tags/" + tagObjectSHA:
			writeJSON(t, w, map[string]any{
				"sha": tagObjectSHA,
				"tagger": map[string]string{
					"date": "2026-05-05T12:00:00Z",
				},
				"object": map[string]string{
					"type": "commit",
					"sha":  commitSHA,
				},
			})
		case "/repos/actions/checkout/git/commits/" + commitSHA:
			writeJSON(t, w, commit(commitSHA, "2026-05-06T12:00:00Z"))
		case "/repos/actions/checkout/releases/tags/v4":
			writeError(w, http.StatusNotFound, "Not Found")
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	})

	client := newTestClient(t, server)
	got, err := client.ResolveTag(context.Background(), "actions", "checkout", "v4")
	if err != nil {
		t.Fatalf("ResolveTag returned error: %v", err)
	}

	if got.SHA != commitSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, commitSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-06T12:00:00Z")
	assertTimePtr(t, got.TagTime, "2026-05-05T12:00:00Z")
	if got.ReleaseTime != nil {
		t.Fatalf("ReleaseTime = %v, want nil when no release exists", got.ReleaseTime)
	}
}

func TestResolveLatestRelease(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout/releases/latest":
			writeJSON(t, w, map[string]any{
				"tag_name":     "v5",
				"published_at": "2026-05-08T12:00:00Z",
			})
		case "/repos/actions/checkout/git/ref/tags/v5":
			writeJSON(t, w, gitRef("refs/tags/v5", "commit", commitSHA))
		case "/repos/actions/checkout/git/commits/" + commitSHA:
			writeJSON(t, w, commit(commitSHA, "2026-05-07T12:00:00Z"))
		case "/repos/actions/checkout/releases/tags/v5":
			writeJSON(t, w, map[string]any{
				"tag_name":     "v5",
				"published_at": "2026-05-08T12:00:00Z",
			})
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	})

	client := newTestClient(t, server)
	got, err := client.ResolveLatestRelease(context.Background(), "actions", "checkout")
	if err != nil {
		t.Fatalf("ResolveLatestRelease returned error: %v", err)
	}

	if got.Kind != KindTag {
		t.Fatalf("Kind = %q, want %q", got.Kind, KindTag)
	}
	if got.Ref != "v5" {
		t.Fatalf("Ref = %q, want v5", got.Ref)
	}
	if got.SHA != commitSHA {
		t.Fatalf("SHA = %q, want %q", got.SHA, commitSHA)
	}
	assertTime(t, got.CommitTime, "2026-05-07T12:00:00Z")
	assertTimePtr(t, got.ReleaseTime, "2026-05-08T12:00:00Z")
}

func TestResolveLatestReleaseReportsNoReleases(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/actions/checkout/releases/latest" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		writeError(w, http.StatusNotFound, "Not Found")
	})

	client := newTestClient(t, server)
	err := func() error {
		_, err := client.ResolveLatestRelease(context.Background(), "actions", "checkout")
		return err
	}()
	var resolverErr *ResolverError
	if !errors.As(err, &resolverErr) {
		t.Fatalf("error type = %T, want *ResolverError", err)
	}
	if resolverErr.Kind != ErrorNotFound {
		t.Fatalf("error kind = %s, want %s; error: %v", resolverErr.Kind, ErrorNotFound, err)
	}
	if !strings.Contains(err.Error(), "actions/checkout@latest-release") {
		t.Fatalf("error %q does not include latest-release selector", err)
	}
}

func TestListReleasesPaginatesAndPreservesReleaseMetadata(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/actions/checkout/releases" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		switch r.URL.Query().Get("page") {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/repos/actions/checkout/releases?page=2>; rel="next"`, r.Host))
			writeJSON(t, w, []map[string]any{{"tag_name": "v6.0.0", "published_at": "2026-06-20T12:00:00Z"}})
		case "2":
			writeJSON(t, w, []map[string]any{{"tag_name": "v5.0.0", "created_at": "2026-05-01T12:00:00Z", "prerelease": true}})
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
		}
	})

	client := newTestClient(t, server)
	releases, err := client.ListReleases(context.Background(), "actions", "checkout")
	if err != nil {
		t.Fatalf("ListReleases returned error: %v", err)
	}
	if len(releases) != 2 || releases[0].TagName != "v6.0.0" || releases[1].TagName != "v5.0.0" || !releases[1].Prerelease {
		t.Fatalf("unexpected releases: %#v", releases)
	}
	assertTime(t, releases[0].PublishedAt, "2026-06-20T12:00:00Z")
	assertTime(t, releases[1].CreatedAt, "2026-05-01T12:00:00Z")
}

func TestResolveDiscoveryReportsAPIFailure(t *testing.T) {
	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/actions/checkout" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		writeError(w, http.StatusInternalServerError, "Server Error")
	})

	client := newTestClient(t, server)
	err := func() error {
		_, err := client.ResolveDefaultBranch(context.Background(), "actions", "checkout")
		return err
	}()
	var resolverErr *ResolverError
	if !errors.As(err, &resolverErr) {
		t.Fatalf("error type = %T, want *ResolverError", err)
	}
	if resolverErr.Kind != ErrorGitHubAPI {
		t.Fatalf("error kind = %s, want %s; error: %v", resolverErr.Kind, ErrorGitHubAPI, err)
	}
	if !strings.Contains(err.Error(), "resolve default branch") {
		t.Fatalf("error %q does not include operation", err)
	}
}

func TestVerifyCommitReportsNotFoundForbiddenAndRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    map[string]string
		wantKind   ErrorKind
	}{
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantKind:   ErrorNotFound,
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			wantKind:   ErrorForbidden,
		},
		{
			name:       "rate limit",
			statusCode: http.StatusForbidden,
			headers: map[string]string{
				"X-RateLimit-Remaining": "0",
				"X-RateLimit-Reset":     "1770000000",
			},
			wantKind: ErrorRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.headers {
					w.Header().Set(key, value)
				}
				writeError(w, tt.statusCode, http.StatusText(tt.statusCode))
			})
			client := newTestClient(t, server)

			err := client.VerifyCommit(context.Background(), "actions", "checkout", commitSHA)
			assertResolverError(t, err, tt.wantKind)
		})
	}
}

func TestTokenFromEnvAndAuthenticatedClient(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "primary-token")
	t.Setenv("GH_TOKEN", "fallback-token")
	if got := TokenFromEnv(); got != "primary-token" {
		t.Fatalf("TokenFromEnv = %q, want primary-token", got)
	}

	t.Setenv("GITHUB_TOKEN", "")
	if got := TokenFromEnv(); got != "fallback-token" {
		t.Fatalf("TokenFromEnv fallback = %q, want fallback-token", got)
	}

	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q, want Bearer secret-token", got)
		}
		writeJSON(t, w, gitRef("refs/heads/main", "commit", branchSHA))
	})

	client, err := NewClient(
		WithHTTPClient(server.Client()),
		withBaseURL(server.URL),
		WithToken("secret-token"),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	_, _, err = client.github.Git.GetRef(context.Background(), "actions", "checkout", "heads/main")
	if err != nil {
		t.Fatalf("GetRef returned error: %v", err)
	}
}

func TestNewClientFromEnvDoesNotSendTokenToCustomBaseURLByDefault(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")

	server := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want empty", got)
		}
		writeJSON(t, w, gitRef("refs/heads/main", "commit", branchSHA))
	})

	client, err := NewClientFromEnv(WithHTTPClient(server.Client()), withBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClientFromEnv returned error: %v", err)
	}
	_, _, err = client.github.Git.GetRef(context.Background(), "actions", "checkout", "heads/main")
	if err != nil {
		t.Fatalf("GetRef returned error: %v", err)
	}
}

func TestVerifyCommitReportsNetworkFailure(t *testing.T) {
	client, err := NewClient(WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network unavailable")
		}),
	}))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	err = client.VerifyCommit(context.Background(), "actions", "checkout", commitSHA)
	assertResolverError(t, err, ErrorGitHubAPI)
	if !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("error = %q, want network context", err)
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := NewClient(WithHTTPClient(server.Client()), withBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func fakeGitHub(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func gitRef(ref, objectType, sha string) map[string]any {
	return map[string]any{
		"ref": ref,
		"object": map[string]string{
			"type": objectType,
			"sha":  sha,
		},
	}
}

func commit(sha, date string) map[string]any {
	return map[string]any{
		"sha": sha,
		"committer": map[string]string{
			"date": date,
		},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`{"message":` + quote(message) + `}`))
}

func quote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func assertTime(t *testing.T, got time.Time, want string) {
	t.Helper()

	wantTime, err := time.Parse(time.RFC3339, want)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(wantTime) {
		t.Fatalf("time = %s, want %s", got.Format(time.RFC3339), want)
	}
}

func assertTimePtr(t *testing.T, got *time.Time, want string) {
	t.Helper()

	if got == nil {
		t.Fatalf("time = nil, want %s", want)
	}
	assertTime(t, *got, want)
}

func assertResolverError(t *testing.T, err error, wantKind ErrorKind) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want kind %s", wantKind)
	}
	var resolverErr *ResolverError
	if !errors.As(err, &resolverErr) {
		t.Fatalf("error type = %T, want *ResolverError", err)
	}
	if resolverErr.Kind != wantKind {
		t.Fatalf("error kind = %s, want %s; error: %v", resolverErr.Kind, wantKind, err)
	}
	if !strings.Contains(err.Error(), "actions/checkout@"+commitSHA) {
		t.Fatalf("error %q does not include selector", err)
	}
}
