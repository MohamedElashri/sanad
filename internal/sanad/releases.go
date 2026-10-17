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
	releases map[string][]Release
	dates    map[string]time.Time
}

func newAPI(host string) *githubAPI {
	return &githubAPI{home: host, client: &http.Client{Timeout: 30 * time.Second}, tokens: map[string]string{}, releases: map[string][]Release{}, dates: map[string]time.Time{}}
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
	var release Release
	code, err := a.get(ctx, d.Host, "/repos/"+nwo+"/releases/tags/"+url.PathEscape(ref), &release)
	if err != nil {
		return time.Time{}, err
	}
	at := release.Published
	if code == 404 || at.IsZero() {
		_, sha, _ := strings.Cut(d.Commit, "-")
		var commit struct {
			Commit struct {
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		}
		code, err = a.get(ctx, d.Host, "/repos/"+nwo+"/commits/"+url.PathEscape(sha), &commit)
		if err != nil {
			return at, err
		}
		if code == 404 {
			return at, fmt.Errorf("timestamp unavailable for %s", d.Key)
		}
		at = commit.Commit.Committer.Date
	}
	if at.IsZero() {
		return at, fmt.Errorf("timestamp unavailable for %s", d.Key)
	}
	a.dates[key] = at
	return at, nil
}
