package sanad

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestReleasePaginationAndTimestampFallback(t *testing.T) {
	api := newAPI("github.com")
	api.tokens["github.com"] = "public-test-token"
	calls := 0
	api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Host != "api.github.com" || req.Header.Get("Authorization") != "Bearer public-test-token" {
			t.Fatal("unexpected API target or credentials")
		}
		switch {
		case req.URL.Path == "/repos/a/root/releases":
			if req.URL.Query().Get("page") == "1" {
				items := []string{}
				for i := 0; i < 100; i++ {
					items = append(items, fmt.Sprintf(`{"tag_name":"v1.0.%d"}`, i))
				}
				return response(200, "["+strings.Join(items, ",")+"]"), nil
			}
			return response(200, `[{"tag_name":"v1.1.0"}]`), nil
		case strings.Contains(req.URL.Path, "/releases/tags/"):
			return response(404, ""), nil
		case strings.Contains(req.URL.Path, "/commits/"):
			return response(200, `{"commit":{"committer":{"date":"2026-10-01T00:00:00Z"}}}`), nil
		default:
			t.Fatalf("unexpected endpoint %s", req.URL.Path)
			return nil, nil
		}
	})}
	list, err := api.List(context.Background(), "github.com", "a/root")
	if err != nil || len(list) != 101 {
		t.Fatal(len(list), err)
	}
	if _, err := api.List(context.Background(), "github.com", "a/root"); err != nil || calls != 2 {
		t.Fatal("release cache missed", calls, err)
	}
	d := Dependency{Host: "github.com", Key: "a/root@v1.1.0", Commit: "sha1-" + strings.Repeat("a", 40)}
	at, err := api.Timestamp(context.Background(), d)
	if err != nil || at != time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatal(at, err)
	}
	_, err = api.Timestamp(context.Background(), d)
	if err != nil || calls != 4 {
		t.Fatal("timestamp cache missed", calls, err)
	}
}

func TestPartialRefsDoNotSelectOlderReleaseCode(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1": action("v1", "a")}, "")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("a/root@v1")); err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), "plan", options(root, &fakeBackend{t: t}))
	if err != nil || len(r.Decisions) != 1 || r.Decisions[0].Status != "frozen" {
		t.Fatalf("partial ref was treated as an installed full version: %+v %v", r, err)
	}
}
