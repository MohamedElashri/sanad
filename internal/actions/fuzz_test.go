package actions

import "testing"

func FuzzParseActionReference(f *testing.F) {
	seeds := []string{
		"",
		"actions/checkout@v4",
		"actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683",
		"actions/checkout@11bd719",
		"owner/repo/path/to/action@v1",
		"owner/repo/.github/workflows/reuse.yml@v1",
		"./.github/actions/local",
		"docker://alpine:3.20",
		"https://example.com/action",
		"owner/repo",
		"owner//repo@v1",
		"owner/repo@",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		got := Parse(raw)
		if got.Kind == "" {
			t.Fatalf("Parse(%q) returned empty kind", raw)
		}
		if got.Valid && got.Error != "" {
			t.Fatalf("Parse(%q) returned valid action with error %q", raw, got.Error)
		}
		if !got.Valid && got.Error == "" {
			t.Fatalf("Parse(%q) returned invalid action without error", raw)
		}
		if got.Pinned && !IsFullSHA(got.Ref) {
			t.Fatalf("Parse(%q) returned pinned action with non-full SHA ref %q", raw, got.Ref)
		}
	})
}
