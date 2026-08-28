package githubresolver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

type RefKind string

const (
	KindSHA    RefKind = "sha"
	KindTag    RefKind = "tag"
	KindBranch RefKind = "branch"
)

type ErrorKind string

const (
	ErrorInvalid   ErrorKind = "invalid"
	ErrorNotFound  ErrorKind = "not-found"
	ErrorForbidden ErrorKind = "forbidden"
	ErrorRateLimit ErrorKind = "rate-limit"
	ErrorGitHubAPI ErrorKind = "github-api"
)

type ActionSelector struct {
	Owner string
	Repo  string
	Ref   string
}

type ResolvedRef struct {
	Owner       string     `json:"owner"`
	Repo        string     `json:"repo"`
	Ref         string     `json:"ref"`
	SHA         string     `json:"sha"`
	Kind        RefKind    `json:"kind"`
	CommitTime  time.Time  `json:"commit_time"`
	TagTime     *time.Time `json:"tag_time,omitempty"`
	ReleaseTime *time.Time `json:"release_time,omitempty"`
}

type Release struct {
	TagName     string    `json:"tag_name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type ResolverError struct {
	Kind       ErrorKind
	Operation  string
	Owner      string
	Repo       string
	Ref        string
	StatusCode int
	Message    string
	Err        error
}

func (e *ResolverError) Error() string {
	target := e.Owner + "/" + e.Repo
	if e.Ref != "" {
		target += "@" + e.Ref
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s %s: %s (%d)", e.Operation, target, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s %s: %s", e.Operation, target, e.Message)
}

func (e *ResolverError) Unwrap() error {
	return e.Err
}

type Client struct {
	github *github.Client
}

type clientOptions struct {
	baseURL                  string
	httpClient               *http.Client
	token                    string
	tokenCameFromEnvironment bool
}

type Option func(*clientOptions)

func withBaseURL(baseURL string) Option {
	return func(opts *clientOptions) {
		opts.baseURL = baseURL
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(opts *clientOptions) {
		opts.httpClient = httpClient
	}
}

func WithToken(token string) Option {
	return func(opts *clientOptions) {
		opts.token = token
	}
}

func NewClient(options ...Option) (*Client, error) {
	opts := applyOptions(options)

	httpClient := opts.httpClient
	if opts.token != "" && shouldSendToken(opts) {
		ctx := context.Background()
		if httpClient != nil {
			ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
		}
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.token}))
	}

	client := github.NewClient(httpClient)
	if opts.baseURL != "" {
		baseURL, err := normalizeBaseURL(opts.baseURL)
		if err != nil {
			return nil, err
		}
		client.BaseURL = baseURL
	}

	return &Client{github: client}, nil
}

func NewClientFromEnv(options ...Option) (*Client, error) {
	token := TokenFromEnv()
	if token == "" {
		return NewClient(options...)
	}

	withToken := make([]Option, 0, len(options)+1)
	withToken = append(withToken, WithToken(token), withEnvironmentToken())
	withToken = append(withToken, options...)
	return NewClient(withToken...)
}

func withEnvironmentToken() Option {
	return func(opts *clientOptions) {
		opts.tokenCameFromEnvironment = true
	}
}

func TokenFromEnv() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (c *Client) Resolve(ctx context.Context, selector ActionSelector) (ResolvedRef, error) {
	if err := validateSelector(selector); err != nil {
		return ResolvedRef{}, err
	}

	if actions.IsFullSHA(selector.Ref) {
		commit, err := c.getCommit(ctx, selector.Owner, selector.Repo, selector.Ref)
		if err != nil {
			return ResolvedRef{}, err
		}
		return ResolvedRef{
			Owner:      selector.Owner,
			Repo:       selector.Repo,
			Ref:        selector.Ref,
			SHA:        selector.Ref,
			Kind:       KindSHA,
			CommitTime: commitTime(commit),
		}, nil
	}

	tag, err := c.ResolveTag(ctx, selector.Owner, selector.Repo, selector.Ref)
	if err == nil {
		return tag, nil
	}
	if !isKind(err, ErrorNotFound) {
		return ResolvedRef{}, err
	}

	branch, branchErr := c.ResolveBranch(ctx, selector.Owner, selector.Repo, selector.Ref)
	if branchErr == nil {
		return branch, nil
	}
	if isKind(branchErr, ErrorNotFound) {
		return ResolvedRef{}, &ResolverError{
			Kind:      ErrorNotFound,
			Operation: "resolve ref",
			Owner:     selector.Owner,
			Repo:      selector.Repo,
			Ref:       selector.Ref,
			Message:   "GitHub ref was not found as a tag or branch",
			Err:       errors.Join(err, branchErr),
		}
	}
	return ResolvedRef{}, branchErr
}

func (c *Client) ResolveTag(ctx context.Context, owner, repo, tag string) (ResolvedRef, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: tag}
	if err := validateSelector(selector); err != nil {
		return ResolvedRef{}, err
	}

	ref, _, err := c.github.Git.GetRef(ctx, owner, repo, "tags/"+tag)
	if err != nil {
		return ResolvedRef{}, wrapGitHubError("resolve tag", selector, err)
	}
	if ref == nil || ref.Object == nil || ref.Object.SHA == nil || ref.Object.Type == nil {
		return ResolvedRef{}, invalidRef("resolve tag", selector, "GitHub tag ref response was missing object data")
	}

	sha := ref.Object.GetSHA()
	var tagTime *time.Time
	switch ref.Object.GetType() {
	case "commit":
	case "tag":
		annotated, _, err := c.github.Git.GetTag(ctx, owner, repo, sha)
		if err != nil {
			return ResolvedRef{}, wrapGitHubError("resolve annotated tag", selector, err)
		}
		if annotated == nil || annotated.Object == nil || annotated.Object.GetType() != "commit" || annotated.Object.GetSHA() == "" {
			return ResolvedRef{}, invalidRef("resolve annotated tag", selector, "annotated tag does not point to a commit")
		}
		sha = annotated.Object.GetSHA()
		if annotated.Tagger != nil {
			tagTime = timestampTime(annotated.Tagger.Date)
		}
	default:
		return ResolvedRef{}, invalidRef("resolve tag", selector, "tag ref points to unsupported object type %q", ref.Object.GetType())
	}

	commit, err := c.getCommit(ctx, owner, repo, sha)
	if err != nil {
		return ResolvedRef{}, err
	}

	releaseTime, err := c.releaseTimestamp(ctx, owner, repo, tag)
	if err != nil {
		return ResolvedRef{}, err
	}

	return ResolvedRef{
		Owner:       owner,
		Repo:        repo,
		Ref:         tag,
		SHA:         sha,
		Kind:        KindTag,
		CommitTime:  commitTime(commit),
		TagTime:     tagTime,
		ReleaseTime: releaseTime,
	}, nil
}

func (c *Client) ResolveBranch(ctx context.Context, owner, repo, branch string) (ResolvedRef, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: branch}
	if err := validateSelector(selector); err != nil {
		return ResolvedRef{}, err
	}

	ref, _, err := c.github.Git.GetRef(ctx, owner, repo, "heads/"+branch)
	if err != nil {
		return ResolvedRef{}, wrapGitHubError("resolve branch", selector, err)
	}
	if ref == nil || ref.Object == nil || ref.Object.GetType() != "commit" || ref.Object.GetSHA() == "" {
		return ResolvedRef{}, invalidRef("resolve branch", selector, "branch ref does not point to a commit")
	}

	commit, err := c.getCommit(ctx, owner, repo, ref.Object.GetSHA())
	if err != nil {
		return ResolvedRef{}, err
	}

	return ResolvedRef{
		Owner:      owner,
		Repo:       repo,
		Ref:        branch,
		SHA:        ref.Object.GetSHA(),
		Kind:       KindBranch,
		CommitTime: commitTime(commit),
	}, nil
}

func (c *Client) ResolveDefaultBranch(ctx context.Context, owner, repo string) (ResolvedRef, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: "default-branch"}
	if err := validateRepository(selector); err != nil {
		return ResolvedRef{}, err
	}

	repository, _, err := c.github.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return ResolvedRef{}, wrapGitHubError("resolve default branch", selector, err)
	}
	if repository == nil || strings.TrimSpace(repository.GetDefaultBranch()) == "" {
		return ResolvedRef{}, invalidRef("resolve default branch", selector, "GitHub repository response was missing default branch")
	}

	return c.ResolveBranch(ctx, owner, repo, repository.GetDefaultBranch())
}

func (c *Client) ResolveLatestRelease(ctx context.Context, owner, repo string) (ResolvedRef, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: "latest-release"}
	if err := validateRepository(selector); err != nil {
		return ResolvedRef{}, err
	}

	release, _, err := c.github.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return ResolvedRef{}, wrapGitHubError("resolve latest release", selector, err)
	}
	if release == nil || strings.TrimSpace(release.GetTagName()) == "" {
		return ResolvedRef{}, invalidRef("resolve latest release", selector, "GitHub latest release response was missing tag name")
	}

	resolved, err := c.ResolveTag(ctx, owner, repo, release.GetTagName())
	if err != nil {
		return ResolvedRef{}, err
	}
	if release.PublishedAt != nil {
		resolved.ReleaseTime = release.PublishedAt.GetTime()
	} else if release.CreatedAt != nil {
		resolved.ReleaseTime = release.CreatedAt.GetTime()
	}
	return resolved, nil
}

func (c *Client) ListReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: "releases"}
	if err := validateRepository(selector); err != nil {
		return nil, err
	}

	options := &github.ListOptions{PerPage: 100}
	var releases []Release
	for {
		page, response, err := c.github.Repositories.ListReleases(ctx, owner, repo, options)
		if err != nil {
			return nil, wrapGitHubError("list releases", selector, err)
		}
		for _, release := range page {
			if release == nil {
				continue
			}
			item := Release{
				TagName:    release.GetTagName(),
				Draft:      release.GetDraft(),
				Prerelease: release.GetPrerelease(),
			}
			if release.PublishedAt != nil {
				if publishedAt := release.PublishedAt.GetTime(); publishedAt != nil {
					item.PublishedAt = *publishedAt
				}
			}
			if release.CreatedAt != nil {
				if createdAt := release.CreatedAt.GetTime(); createdAt != nil {
					item.CreatedAt = *createdAt
				}
			}
			releases = append(releases, item)
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}
	return releases, nil
}

func (c *Client) VerifyCommit(ctx context.Context, owner, repo, sha string) error {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return invalidRef("verify commit", ActionSelector{Owner: owner, Repo: repo, Ref: sha}, "owner and repo are required")
	}
	if !actions.IsFullSHA(sha) {
		return invalidRef("verify commit", ActionSelector{Owner: owner, Repo: repo, Ref: sha}, "commit SHA must be a full 40-character SHA")
	}
	_, err := c.getCommit(ctx, owner, repo, sha)
	return err
}

func (c *Client) getCommit(ctx context.Context, owner, repo, sha string) (*github.Commit, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: sha}
	commit, _, err := c.github.Git.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		return nil, wrapGitHubError("verify commit", selector, err)
	}
	if commit == nil || commit.GetSHA() == "" {
		return nil, invalidRef("verify commit", selector, "GitHub commit response was missing SHA")
	}
	return commit, nil
}

func (c *Client) releaseTimestamp(ctx context.Context, owner, repo, tag string) (*time.Time, error) {
	selector := ActionSelector{Owner: owner, Repo: repo, Ref: tag}
	release, _, err := c.github.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		if isGitHubNotFound(err) {
			return nil, nil
		}
		return nil, wrapGitHubError("fetch release", selector, err)
	}
	if release == nil {
		return nil, nil
	}
	if release.PublishedAt != nil {
		return release.PublishedAt.GetTime(), nil
	}
	if release.CreatedAt != nil {
		return release.CreatedAt.GetTime(), nil
	}
	return nil, nil
}

func applyOptions(options []Option) clientOptions {
	var opts clientOptions
	for _, option := range options {
		option(&opts)
	}
	return opts
}

func shouldSendToken(opts clientOptions) bool {
	if !opts.tokenCameFromEnvironment || opts.baseURL == "" {
		return true
	}
	baseURL, err := normalizeBaseURL(opts.baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(baseURL.Host, "api.github.com")
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	baseURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub base URL %q: %w", raw, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid GitHub base URL %q: expected absolute URL", raw)
	}
	return baseURL, nil
}

func validateSelector(selector ActionSelector) error {
	if strings.TrimSpace(selector.Owner) == "" || strings.TrimSpace(selector.Repo) == "" || strings.TrimSpace(selector.Ref) == "" {
		return invalidRef("resolve ref", selector, "owner, repo, and ref are required")
	}
	return nil
}

func validateRepository(selector ActionSelector) error {
	if strings.TrimSpace(selector.Owner) == "" || strings.TrimSpace(selector.Repo) == "" {
		return invalidRef("resolve ref", selector, "owner and repo are required")
	}
	return nil
}

func invalidRef(operation string, selector ActionSelector, format string, args ...any) error {
	return &ResolverError{
		Kind:      ErrorInvalid,
		Operation: operation,
		Owner:     selector.Owner,
		Repo:      selector.Repo,
		Ref:       selector.Ref,
		Message:   fmt.Sprintf(format, args...),
	}
}

func wrapGitHubError(operation string, selector ActionSelector, err error) error {
	if err == nil {
		return nil
	}

	var rateLimitErr *github.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return &ResolverError{
			Kind:       ErrorRateLimit,
			Operation:  operation,
			Owner:      selector.Owner,
			Repo:       selector.Repo,
			Ref:        selector.Ref,
			StatusCode: responseStatus(rateLimitErr.Response),
			Message:    "GitHub API rate limit exceeded",
			Err:        err,
		}
	}

	var abuseRateLimitErr *github.AbuseRateLimitError
	if errors.As(err, &abuseRateLimitErr) {
		return &ResolverError{
			Kind:       ErrorRateLimit,
			Operation:  operation,
			Owner:      selector.Owner,
			Repo:       selector.Repo,
			Ref:        selector.Ref,
			StatusCode: responseStatus(abuseRateLimitErr.Response),
			Message:    "GitHub API secondary rate limit exceeded",
			Err:        err,
		}
	}

	var responseErr *github.ErrorResponse
	if errors.As(err, &responseErr) {
		status := responseStatus(responseErr.Response)
		kind := ErrorGitHubAPI
		message := responseErr.Message
		switch status {
		case http.StatusNotFound:
			kind = ErrorNotFound
			message = "GitHub resource not found"
		case http.StatusForbidden:
			kind = ErrorForbidden
			if message == "" {
				message = "GitHub API access forbidden"
			}
		}
		if message == "" {
			message = "GitHub API request failed"
		}
		return &ResolverError{
			Kind:       kind,
			Operation:  operation,
			Owner:      selector.Owner,
			Repo:       selector.Repo,
			Ref:        selector.Ref,
			StatusCode: status,
			Message:    message,
			Err:        err,
		}
	}

	return &ResolverError{
		Kind:      ErrorGitHubAPI,
		Operation: operation,
		Owner:     selector.Owner,
		Repo:      selector.Repo,
		Ref:       selector.Ref,
		Message:   err.Error(),
		Err:       err,
	}
}

func isGitHubNotFound(err error) bool {
	var responseErr *github.ErrorResponse
	return errors.As(err, &responseErr) && responseStatus(responseErr.Response) == http.StatusNotFound
}

func isKind(err error, kind ErrorKind) bool {
	var resolverErr *ResolverError
	return errors.As(err, &resolverErr) && resolverErr.Kind == kind
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

func commitTime(commit *github.Commit) time.Time {
	if commit == nil {
		return time.Time{}
	}
	if commit.Committer != nil && commit.Committer.Date != nil {
		return commit.Committer.Date.Time
	}
	if commit.Author != nil && commit.Author.Date != nil {
		return commit.Author.Date.Time
	}
	return time.Time{}
}

func timestampTime(timestamp *github.Timestamp) *time.Time {
	if timestamp == nil {
		return nil
	}
	value := timestamp.Time
	return &value
}
