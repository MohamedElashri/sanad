package sanad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestTagPaginationCachingAndAuthentication(t *testing.T) {
	api := newAPI("github.com")
	api.tokens["github.com"] = "public-test-token"
	calls := 0
	api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Host != "api.github.com" || req.Header.Get("Authorization") != "Bearer public-test-token" {
			t.Fatal("wrong catalog credential or host")
		}
		switch req.URL.Query().Get("page") {
		case "1":
			items := []string{}
			for i := 0; i < 100; i++ {
				items = append(items, fmt.Sprintf(`{"name":"tool/v1.0.%d"}`, i))
			}
			return response(200, "["+strings.Join(items, ",")+"]"), nil
		case "2":
			return response(200, `[{"name":"tool/v1.1.0"}]`), nil
		default:
			t.Fatalf("unexpected page %s", req.URL)
			return nil, nil
		}
	})}
	tags, err := api.Tags(context.Background(), "github.com", "owner/repo")
	if err != nil || len(tags) != 101 {
		t.Fatal(len(tags), err)
	}
	for _, tag := range tags {
		if !tag.TagOnly || !tag.Published.IsZero() {
			t.Fatal("catalog invented a publication time")
		}
	}
	if _, err := api.Tags(context.Background(), "github.com", "owner/repo"); err != nil || calls != 2 {
		t.Fatal("catalog was not cached", calls, err)
	}
}
func TestTagCatalogErrorsDoNotBecomeEmptySuccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"not-found", 404, ""}, {"forbidden", 403, ""}, {"server-error", 500, ""}, {"malformed-json", 200, "["}} {
		t.Run(tc.name, func(t *testing.T) {
			api := newAPI("github.com")
			api.tokens["github.com"] = "test"
			api.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(tc.status, tc.body), nil })}
			if _, err := api.Tags(context.Background(), "github.com", "owner/repo"); err == nil {
				t.Fatal("catalog failure silently accepted")
			}
		})
	}
}
func TestArchiveRedirectStripsCredentialsAndPreservesExactCommit(t *testing.T) {
	api := newAPI("customer.ghe.com")
	api.tokens["customer.ghe.com"] = "tenant-secret"
	api.tokens["github.com"] = "public-secret"
	sha := strings.Repeat("a", 40)
	calls := 0
	api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Host == "api.github.com" {
			if req.Header.Get("Authorization") != "Bearer public-secret" || !strings.HasSuffix(req.URL.Path, "/tarball/"+sha) {
				t.Fatal("archive request used wrong credential or floating ref")
			}
			res := response(302, "")
			res.Header.Set("Location", "https://codeload.github.com/owner/repo/legacy.tar.gz/"+sha)
			return res, nil
		}
		if req.URL.Host != "codeload.github.com" || req.Header.Get("Authorization") != "" {
			t.Fatal("credential crossed download redirect")
		}
		return response(200, "archive fixture"), nil
	})}
	data, err := api.Archive(context.Background(), Dependency{Key: "owner/repo@v1.0.0", Host: "github.com", Commit: "sha1-" + sha})
	if err != nil || string(data) != "archive fixture" || calls != 2 {
		t.Fatal(string(data), calls, err)
	}
}
func TestArchiveTransportAndStatusFailuresAreErrors(t *testing.T) {
	for _, status := range []int{403, 404, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			api := newAPI("github.com")
			api.tokens["github.com"] = "test"
			api.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(status, "failure"), nil })}
			if _, err := api.Archive(context.Background(), Dependency{Key: "owner/repo@v1.0.0", Host: "github.com", Commit: "sha1-" + strings.Repeat("a", 40)}); err == nil {
				t.Fatal("archive failure accepted")
			}
		})
	}
	api := newAPI("github.com")
	api.tokens["github.com"] = "test"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })}
	if _, err := api.Archive(ctx, Dependency{Key: "owner/repo@v1", Host: "github.com", Commit: "sha1-" + strings.Repeat("a", 40)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	if _, err := api.Archive(context.Background(), Dependency{Commit: "invalid"}); err == nil {
		t.Fatal("invalid archive digest accepted")
	}
}
