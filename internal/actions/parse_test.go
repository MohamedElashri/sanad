package actions

import (
	"strings"
	"testing"
)

func TestParseActionReferences(t *testing.T) {
	fullSHA := "11bd71901bbe5b1630ceea73d27597364c9af683"

	tests := []struct {
		name   string
		raw    string
		want   ParsedAction
		errSub string
	}{
		{
			name: "github action tag",
			raw:  "actions/checkout@v4",
			want: ParsedAction{
				Raw:   "actions/checkout@v4",
				Owner: "actions",
				Repo:  "checkout",
				Ref:   "v4",
				Kind:  KindGitHubAction,
				Valid: true,
			},
		},
		{
			name: "github action full sha",
			raw:  "actions/checkout@" + fullSHA,
			want: ParsedAction{
				Raw:    "actions/checkout@" + fullSHA,
				Owner:  "actions",
				Repo:   "checkout",
				Ref:    fullSHA,
				Kind:   KindGitHubAction,
				Pinned: true,
				Valid:  true,
			},
		},
		{
			name: "github action subdirectory",
			raw:  "owner/repo/path/to/action@v1",
			want: ParsedAction{
				Raw:   "owner/repo/path/to/action@v1",
				Owner: "owner",
				Repo:  "repo",
				Path:  "path/to/action",
				Ref:   "v1",
				Kind:  KindGitHubAction,
				Valid: true,
			},
		},
		{
			name: "reusable workflow",
			raw:  "owner/repo/.github/workflows/reuse.yml@v1",
			want: ParsedAction{
				Raw:   "owner/repo/.github/workflows/reuse.yml@v1",
				Owner: "owner",
				Repo:  "repo",
				Path:  ".github/workflows/reuse.yml",
				Ref:   "v1",
				Kind:  KindReusableWorkflow,
				Valid: true,
			},
		},
		{
			name: "reusable workflow yaml extension",
			raw:  "owner/repo/.github/workflows/reuse.yaml@main",
			want: ParsedAction{
				Raw:   "owner/repo/.github/workflows/reuse.yaml@main",
				Owner: "owner",
				Repo:  "repo",
				Path:  ".github/workflows/reuse.yaml",
				Ref:   "main",
				Kind:  KindReusableWorkflow,
				Valid: true,
			},
		},
		{
			name: "local action",
			raw:  "./.github/actions/local",
			want: ParsedAction{
				Raw:   "./.github/actions/local",
				Path:  "./.github/actions/local",
				Kind:  KindLocalAction,
				Valid: true,
			},
		},
		{
			name: "self repository action",
			raw:  "$/action",
			want: ParsedAction{
				Raw:   "$/action",
				Path:  "$/action",
				Kind:  KindLocalAction,
				Valid: true,
			},
		},
		{
			name: "docker action",
			raw:  "docker://alpine:3.20",
			want: ParsedAction{
				Raw:   "docker://alpine:3.20",
				Path:  "alpine:3.20",
				Kind:  KindDockerAction,
				Valid: true,
			},
		},
		{
			name: "unpinned github action",
			raw:  "owner/repo",
			want: ParsedAction{
				Raw:   "owner/repo",
				Owner: "owner",
				Repo:  "repo",
				Kind:  KindGitHubAction,
				Valid: true,
			},
		},
		{
			name: "short sha",
			raw:  "actions/checkout@11bd719",
			want: ParsedAction{
				Raw:   "actions/checkout@11bd719",
				Owner: "actions",
				Repo:  "checkout",
				Ref:   "11bd719",
				Kind:  KindInvalid,
			},
			errSub: "short SHA",
		},
		{
			name:   "owner only",
			raw:    "actions",
			want:   ParsedAction{Raw: "actions", Kind: KindInvalid},
			errSub: "owner and repo",
		},
		{
			name:   "unsupported url",
			raw:    "https://example.com/action",
			want:   ParsedAction{Raw: "https://example.com/action", Kind: KindInvalid},
			errSub: "unsupported",
		},
		{
			name:   "path without ref",
			raw:    "owner/repo/path/to/action",
			want:   ParsedAction{Raw: "owner/repo/path/to/action", Kind: KindInvalid},
			errSub: "@ref",
		},
		{
			name:   "empty docker image",
			raw:    "docker://",
			want:   ParsedAction{Raw: "docker://", Kind: KindInvalid},
			errSub: "missing an image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.raw)
			assertParsedAction(t, got, tt.want, tt.errSub)
		})
	}
}

func TestSHAClassification(t *testing.T) {
	tests := []struct {
		ref   string
		full  bool
		short bool
	}{
		{ref: "11bd71901bbe5b1630ceea73d27597364c9af683", full: true},
		{ref: "11BD71901BBE5B1630CEEA73D27597364C9AF683", full: true},
		{ref: "11bd719", short: true},
		{ref: "11bd71901bbe5b1630ceea73d27597364c9af68", short: true},
		{ref: "v4"},
		{ref: "main"},
		{ref: "not-hex"},
		{ref: "123456"},
	}

	for _, tt := range tests {
		if got := IsFullSHA(tt.ref); got != tt.full {
			t.Fatalf("IsFullSHA(%q) = %v, want %v", tt.ref, got, tt.full)
		}
		if got := IsShortSHA(tt.ref); got != tt.short {
			t.Fatalf("IsShortSHA(%q) = %v, want %v", tt.ref, got, tt.short)
		}
	}
}

func assertParsedAction(t *testing.T, got ParsedAction, want ParsedAction, errSub string) {
	t.Helper()

	if got.Raw != want.Raw {
		t.Fatalf("Raw = %q, want %q", got.Raw, want.Raw)
	}
	if got.Owner != want.Owner {
		t.Fatalf("Owner = %q, want %q", got.Owner, want.Owner)
	}
	if got.Repo != want.Repo {
		t.Fatalf("Repo = %q, want %q", got.Repo, want.Repo)
	}
	if got.Path != want.Path {
		t.Fatalf("Path = %q, want %q", got.Path, want.Path)
	}
	if got.Ref != want.Ref {
		t.Fatalf("Ref = %q, want %q", got.Ref, want.Ref)
	}
	if got.Kind != want.Kind {
		t.Fatalf("Kind = %q, want %q", got.Kind, want.Kind)
	}
	if got.Pinned != want.Pinned {
		t.Fatalf("Pinned = %v, want %v", got.Pinned, want.Pinned)
	}
	if got.Valid != want.Valid {
		t.Fatalf("Valid = %v, want %v", got.Valid, want.Valid)
	}
	if errSub == "" {
		if got.Error != "" {
			t.Fatalf("Error = %q, want empty", got.Error)
		}
		return
	}
	if got.Error == "" {
		t.Fatalf("Error is empty, want substring %q", errSub)
	}
	if !strings.Contains(got.Error, errSub) {
		t.Fatalf("Error = %q, want substring %q", got.Error, errSub)
	}
}
