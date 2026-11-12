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

func TestAutomaticTargetsRequireFullStableVersions(t *testing.T) {
	got := targets([]Release{{Ref: "v1.2"}, {Ref: "v1.2.0"}, {Ref: "v2"}, {Ref: "v2.0.0-beta.1"}}, "v1.0.0", UpdatePolicy{Level: "major"}, "")
	if len(got) != 1 || got[0] != "v1.2.0" {
		t.Fatalf("unexpected targets: %v", got)
	}
	if got := targets(nil, "v1.0.0", UpdatePolicy{Level: "minor"}, "v2"); len(got) != 1 || got[0] != "v2" {
		t.Fatal("explicit partial ref should remain available", got)
	}
}

func TestReleaseAgeIncludesTheLockedCommit(t *testing.T) {
	for _, tc := range []struct {
		published, committed, want string
	}{
		{"2026-01-01T00:00:00Z", "2026-10-16T00:00:00Z", "2026-10-16T00:00:00Z"},
		{"2026-10-10T00:00:00Z", "2026-09-01T00:00:00Z", "2026-10-10T00:00:00Z"},
	} {
		api := newAPI("github.com")
		api.tokens["github.com"] = "test-token"
		commitRequests := 0
		api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/releases/tags/") {
				return response(200, fmt.Sprintf(`{"tag_name":"v1.1.0","published_at":%q}`, tc.published)), nil
			}
			commitRequests++
			if !strings.HasSuffix(req.URL.Path, strings.Repeat("b", 40)) {
				t.Fatalf("age query did not use locked commit: %s", req.URL.Path)
			}
			return response(200, fmt.Sprintf(`{"commit":{"committer":{"date":%q}}}`, tc.committed)), nil
		})}
		d := Dependency{Host: "github.com", Key: "a/root@v1.1.0", Commit: "sha1-" + strings.Repeat("b", 40)}
		at, err := api.Timestamp(context.Background(), d)
		if err != nil || at.Format(time.RFC3339) != tc.want || commitRequests != 1 {
			t.Fatalf("incorrect locked-commit age: %s %v", at, err)
		}
		if tc.published < tc.committed {
			ready, _, err := assessReleaseAge(context.Background(), graph{}, graph{d.Key: d}, 7*24*time.Hour, testNow, api)
			if err != nil || ready {
				t.Fatal("old release made a new commit mature", ready, err)
			}
		}
	}
}

type taggedReleases struct {
	fakeReleases
	tags []Release
}

func (f taggedReleases) Tags(context.Context, string, string) ([]Release, error) { return f.tags, nil }
func TestTagFamiliesKeepVersionSelectionWithinNamespace(t *testing.T) {
	p := UpdatePolicy{Level: "minor", TagPrefix: "tool/"}
	list := []Release{{Ref: "v9.0.0"}, {Ref: "other/v1.9.0"}, {Ref: "tool/v1.2.0"}, {Ref: "tool/v2.0.0"}, {Ref: "tool/v1.3.0-rc.1"}, {Ref: "tool/v1.2"}}
	got := targets(list, "tool/v1.0.0", p, "")
	if len(got) != 1 || got[0] != "tool/v1.2.0" {
		t.Fatalf("family selection %v", got)
	}
	cfg := DefaultConfig()
	cfg.Actions["a/root"] = p
	effective := effectivePolicy(cfg, Options{Level: "patch"}, "a/root", "a/root")
	if effective.TagPrefix != "tool/" || effective.Level != "patch" {
		t.Fatalf("CLI erased family %+v", effective)
	}
}
func TestTagCatalogFallbackPreservesReleaseMetadata(t *testing.T) {
	source := taggedReleases{fakeReleases: fakeReleases{refs: []Release{{Ref: "v1.2.0", Draft: true}, {Ref: "v1.1.0"}}}, tags: []Release{{Ref: "v1.3.0"}, {Ref: "v1.2.0"}}}
	list, err := listCandidates(context.Background(), source, "github.com", "a/root", UpdatePolicy{PreferTags: boolValue(true)})
	if err != nil || len(list) != 3 || !list[0].Draft || !tagOnlyCandidate(list, "v1.3.0") || tagOnlyCandidate(list, "v1.1.0") {
		t.Fatalf("catalog %+v %v", list, err)
	}
	got := targets(list, "v1.0.0", UpdatePolicy{Level: "minor"}, "")
	if len(got) != 2 || got[0] != "v1.3.0" || got[1] != "v1.1.0" {
		t.Fatalf("draft was restored by tags: %v", got)
	}
}
func TestUnknownTagAgeCannotBypassCooldown(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ConfigPath, file{Data: []byte("version=1\n[native]\nenabled=true\n[update]\nprefer_tags=true\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if err := putFile(root, ".github/dependabot.yml", file{Data: []byte("version: 2\nupdates:\n  - package-ecosystem: github-actions\n    directory: /\n    schedule:\n      interval: weekly\n    cooldown:\n      default-days: 7\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}})
	opts.Releases = taggedReleases{fakeReleases: fakeReleases{refs: []Release{{Ref: "v1.1.0"}}, at: testNow.AddDate(0, 0, -30)}, tags: []Release{{Ref: "v1.2.0"}}}
	r, err := Run(context.Background(), "plan", opts)
	if err != nil || !r.Success || len(r.Decisions) != 2 || r.Decisions[0].Status != "blocked" || r.Decisions[1].Status != "eligible" {
		t.Fatalf("tag age %+v %v", r, err)
	}
}

func boolValue(value bool) *bool { return &value }
