package sanad

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Release struct {
	TagOnly    bool      `json:"-"`
	Ref        string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
}
type ReleaseSource interface {
	List(context.Context, string, string) ([]Release, error)
	Timestamp(context.Context, Dependency) (time.Time, error)
}

type githubAPI struct {
	home     string
	client   *http.Client
	tokens   map[string]string
	tags     map[string][]Release
	releases map[string][]Release
	dates    map[string]time.Time
}

func newAPI(host string) *githubAPI {
	return &githubAPI{home: host, client: &http.Client{Timeout: 30 * time.Second}, tokens: map[string]string{}, tags: map[string][]Release{}, releases: map[string][]Release{}, dates: map[string]time.Time{}}
}

func (a *githubAPI) token(ctx context.Context, host string) string {
	if token, ok := a.tokens[host]; ok {
		return token
	}
	token := ""
	if host == a.home {
		token = os.Getenv("GH_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
	}
	if token == "" {
		cmd := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host)
		if host != a.home {
			cmd.Env = storedCredentialEnv()
		}
		if b, err := cmd.Output(); err == nil {
			token = strings.TrimSpace(string(b))
		}
	}
	a.tokens[host] = token
	return token
}

// gh auth token itself honors token environment variables, even with --hostname.
// Remove those overrides when looking up a different host's stored credentials.
func storedCredentialEnv() []string {
	env := []string{}
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if key != "GH_TOKEN" && key != "GITHUB_TOKEN" {
			env = append(env, pair)
		}
	}
	return env
}

func (a *githubAPI) get(ctx context.Context, host, path string, out any) (int, error) {
	base := "https://api." + host
	req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sanad")
	if token := a.token(ctx, host); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return res.StatusCode, nil
	}
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("GitHub API HTTP %d for %s%s", res.StatusCode, host, path)
	}
	return res.StatusCode, json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(out)
}

func (a *githubAPI) List(ctx context.Context, host, nwo string) ([]Release, error) {
	key := host + "/" + nwo
	if list, ok := a.releases[key]; ok {
		return list, nil
	}
	out := []Release{}
	for page := 1; page <= 100; page++ {
		var list []Release
		code, err := a.get(ctx, host, fmt.Sprintf("/repos/%s/releases?per_page=100&page=%d", nwo, page), &list)
		if err != nil {
			return nil, err
		}
		if code == 404 {
			return nil, fmt.Errorf("releases unavailable for %s/%s", host, nwo)
		}
		out = append(out, list...)
		if len(list) < 100 {
			a.releases[key] = out
			return out, nil
		}
	}
	return nil, fmt.Errorf("release pagination limit reached for %s", nwo)
}

func (a *githubAPI) Timestamp(ctx context.Context, d Dependency) (time.Time, error) {
	key := observationKey(d)
	if at, ok := a.dates[key]; ok {
		return at, nil
	}
	nwo, ref, _ := strings.Cut(d.Key, "@")
	parts := strings.Split(nwo, "/")
	if len(parts) > 2 {
		nwo = strings.Join(parts[:2], "/")
	}
	var release Release
	_, err := a.get(ctx, d.Host, "/repos/"+nwo+"/releases/tags/"+url.PathEscape(ref), &release)
	if err != nil {
		return time.Time{}, err
	}
	at := release.Published
	_, sha, _ := strings.Cut(d.Commit, "-")
	var commit struct {
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	code, err := a.get(ctx, d.Host, "/repos/"+nwo+"/commits/"+url.PathEscape(sha), &commit)
	if err != nil {
		return at, err
	}
	if code == 404 || commit.Commit.Committer.Date.IsZero() {
		return at, fmt.Errorf("timestamp unavailable for %s", d.Key)
	}
	// A release can outlive its original tag target. An old publication date
	// must not age a newer commit that the tag now resolves to.
	if commit.Commit.Committer.Date.After(at) {
		at = commit.Commit.Committer.Date
	}
	a.dates[key] = at
	return at, nil
}

// TagSource is optional for injected release providers; preferring tags never
// silently falls back to an implementation that only supplies releases.
type TagSource interface {
	Tags(context.Context, string, string) ([]Release, error)
}

func listCandidates(ctx context.Context, source ReleaseSource, host, nwo string, p UpdatePolicy) ([]Release, error) {
	releases, err := source.List(ctx, host, nwo)
	if err != nil {
		return nil, err
	}
	if p.PreferTags == nil || !*p.PreferTags {
		return releases, nil
	}
	tags, ok := source.(TagSource)
	if !ok {
		return nil, fmt.Errorf("release provider does not support prefer_tags")
	}
	list, err := tags.Tags(ctx, host, nwo)
	if err != nil {
		return nil, err
	}
	releases = append([]Release{}, releases...)
	known := map[string]bool{}
	for _, r := range releases {
		known[r.Ref] = true
	}
	for _, r := range list {
		if !known[r.Ref] {
			r.TagOnly = true
			releases = append(releases, r)
			known[r.Ref] = true
		}
	}
	return releases, nil
}
func tagOnlyCandidate(list []Release, ref string) bool {
	for _, r := range list {
		if r.Ref == ref {
			return r.TagOnly && r.Published.IsZero()
		}
	}
	return false
}
func (a *githubAPI) Tags(ctx context.Context, host, nwo string) ([]Release, error) {
	key := host + "/" + nwo
	if list, ok := a.tags[key]; ok {
		return list, nil
	}
	out := []Release{}
	for page := 1; page <= 100; page++ {
		var list []struct {
			Name string `json:"name"`
		}
		code, err := a.get(ctx, host, fmt.Sprintf("/repos/%s/tags?per_page=100&page=%d", nwo, page), &list)
		if err != nil {
			return nil, err
		}
		if code == 404 {
			return nil, fmt.Errorf("tags unavailable for %s/%s", host, nwo)
		}
		for _, tag := range list {
			out = append(out, Release{Ref: tag.Name, TagOnly: true})
		}
		if len(list) < 100 {
			if a.tags == nil {
				a.tags = map[string][]Release{}
			}
			a.tags[key] = out
			return out, nil
		}
	}
	return nil, fmt.Errorf("tag pagination limit reached for %s", nwo)
}
