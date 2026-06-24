package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testSHA      = "11bd71901bbe5b1630ceea73d27597364c9af683"
	testOtherSHA = "93397bea11091df50f3d7e59dc26a7711a8bcfbe"
)

func TestParseComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		present bool
		ref     string
		errSub  string
	}{
		{
			name:    "sanad ref",
			comment: "sanad: ref=v4",
			present: true,
			ref:     "v4",
		},
		{
			name:    "leading comment text",
			comment: "managed by sanad: ref=main",
			present: true,
			ref:     "main",
		},
		{
			name:    "quoted ref",
			comment: `sanad: ref="v5"`,
			present: true,
			ref:     "v5",
		},
		{
			name:    "non sanad comment",
			comment: "renovate: datasource=github-tags",
		},
		{
			name:    "missing ref",
			comment: "sanad: owner=actions",
			present: true,
			errSub:  "must include ref",
		},
		{
			name:    "empty ref",
			comment: "sanad: ref=",
			present: true,
			errSub:  "must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseComment(tt.comment)
			if tt.errSub != "" {
				if err == nil {
					t.Fatalf("ParseComment returned nil error, want %q", tt.errSub)
				}
				if !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseComment returned error: %v", err)
			}
			if got.Present != tt.present {
				t.Fatalf("Present = %v, want %v", got.Present, tt.present)
			}
			if got.Metadata.LogicalRef != tt.ref {
				t.Fatalf("LogicalRef = %q, want %q", got.Metadata.LogicalRef, tt.ref)
			}
		})
	}
}

func TestLoadLockfileMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultLockfilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{
  "version": 1,
  "entries": [
    {
      "file": ".github/workflows/ci.yml",
      "node": "jobs.test.steps[0].uses",
      "owner": "actions",
      "repo": "checkout",
      "kind": "github-action",
      "logical_ref": "v4",
      "pinned_sha": "` + testSHA + `"
    }
  ]
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok, err := LoadLockfileMetadata(path)
	if err != nil {
		t.Fatalf("LoadLockfileMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("LoadLockfileMetadata ok = false, want true")
	}
	key := Key(".github/workflows/ci.yml", "jobs.test.steps[0].uses")
	if got[key].Metadata.LogicalRef != "v4" {
		t.Fatalf("LogicalRef = %q, want v4", got[key].Metadata.LogicalRef)
	}
	if got[key].Metadata.Source != SourceLockfile {
		t.Fatalf("Source = %q, want lockfile", got[key].Metadata.Source)
	}
	if got[key].Entry.Owner != "actions" {
		t.Fatalf("Entry.Owner = %q, want actions", got[key].Entry.Owner)
	}
}

func TestLoadLockfileValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errSub  string
	}{
		{
			name:    "corrupt JSON",
			content: `{"version": 1,`,
			errSub:  "invalid JSON",
		},
		{
			name: "version mismatch",
			content: `{
  "version": 99,
  "entries": []
}
`,
			errSub: "unsupported lockfile version 99",
		},
		{
			name: "missing required field",
			content: `{
  "version": 1,
  "entries": [
    {
      "file": ".github/workflows/ci.yml",
      "node": "jobs.test.steps[0].uses",
      "owner": "actions",
      "repo": "checkout",
      "kind": "github-action",
      "pinned_sha": "` + testSHA + `"
    }
  ]
}
`,
			errSub: "logical_ref is required",
		},
		{
			name: "legacy candidate fields in version two",
			content: `{
  "version": 2,
  "entries": [{
    "file": ".github/workflows/ci.yml",
    "node": "jobs.test.steps[0].uses",
    "owner": "actions",
    "repo": "checkout",
    "kind": "github-action",
    "logical_ref": "v4",
    "pinned_sha": "` + testSHA + `",
    "candidate_sha": "` + testOtherSHA + `",
    "candidate_seen_at": "2026-05-01T12:00:00Z"
  }]
}`,
			errSub: "version 1 candidate fields",
		},
		{
			name: "short sha",
			content: `{
  "version": 1,
  "entries": [
    {
      "file": ".github/workflows/ci.yml",
      "node": "jobs.test.steps[0].uses",
      "owner": "actions",
      "repo": "checkout",
      "kind": "github-action",
      "logical_ref": "v4",
      "pinned_sha": "11bd719"
    }
  ]
}
`,
			errSub: "full 40-character SHA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), DefaultLockfilePath)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			_, _, err := LoadLockfile(path)
			if err == nil {
				t.Fatal("LoadLockfile returned nil error")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestUpdateLockfileRemovesStaleAndSortsDeterministically(t *testing.T) {
	existing := Lockfile{
		Version:     LockfileVersion,
		GeneratedBy: "sanad",
		Entries: []LockfileEntry{
			{
				File:       ".github/workflows/old.yml",
				Node:       "jobs.old.steps[0].uses",
				Owner:      "actions",
				Repo:       "checkout",
				Kind:       "github-action",
				LogicalRef: "v3",
				PinnedSHA:  testSHA,
			},
		},
	}
	active := []LockfileEntry{
		{
			File:       ".github/workflows/z.yml",
			Node:       "jobs.z.steps[0].uses",
			Owner:      "actions",
			Repo:       "setup-go",
			Kind:       "github-action",
			LogicalRef: "v5",
			PinnedSHA:  testOtherSHA,
		},
		{
			File:       ".github/workflows/a.yml",
			Node:       "jobs.a.steps[0].uses",
			Owner:      "actions",
			Repo:       "checkout",
			Kind:       "github-action",
			LogicalRef: "v4",
			PinnedSHA:  testSHA,
		},
	}

	got, err := UpdateLockfile(existing, active)
	if err != nil {
		t.Fatalf("UpdateLockfile returned error: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(got.Entries))
	}
	if got.Entries[0].File != ".github/workflows/a.yml" {
		t.Fatalf("first file = %q, want sorted a.yml", got.Entries[0].File)
	}
	if got.Entries[1].File != ".github/workflows/z.yml" {
		t.Fatalf("second file = %q, want sorted z.yml", got.Entries[1].File)
	}
}

func TestLoadLockfileMigratesVersionOneCandidateHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultLockfilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	seenAt := "2026-05-01T12:00:00Z"
	content := `{
  "version": 1,
  "entries": [{
    "file": ".github/workflows/ci.yml",
    "node": "jobs.test.steps[0].uses",
    "owner": "actions",
    "repo": "checkout",
    "kind": "github-action",
    "logical_ref": "v5",
    "pinned_sha": "` + testSHA + `",
    "candidate_sha": "` + testOtherSHA + `",
    "candidate_seen_at": "` + seenAt + `"
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	lockfile, ok, err := LoadLockfile(path)
	if err != nil || !ok {
		t.Fatalf("LoadLockfile returned ok=%v err=%v", ok, err)
	}
	if lockfile.Version != LockfileVersion || len(lockfile.Entries[0].Candidates) != 1 {
		t.Fatalf("lockfile was not migrated: %#v", lockfile)
	}
	candidate := lockfile.Entries[0].Candidates[0]
	if candidate.LogicalRef != "v5" || candidate.SHA != testOtherSHA || candidate.SeenAt != seenAt {
		t.Fatalf("unexpected migrated candidate: %#v", candidate)
	}
}

func TestSaveLockfileWritesDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultLockfilePath)
	lockfile, err := NewLockfile([]LockfileEntry{
		{
			File:       ".github/workflows/z.yml",
			Node:       "jobs.z.steps[0].uses",
			Owner:      "actions",
			Repo:       "setup-go",
			Kind:       "github-action",
			LogicalRef: "v5",
			PinnedSHA:  testOtherSHA,
		},
		{
			File:       ".github/workflows/a.yml",
			Node:       "jobs.a.steps[0].uses",
			Owner:      "actions",
			Repo:       "checkout",
			Kind:       "github-action",
			LogicalRef: "v4",
			PinnedSHA:  testSHA,
		},
	})
	if err != nil {
		t.Fatalf("NewLockfile returned error: %v", err)
	}

	if err := SaveLockfile(path, lockfile); err != nil {
		t.Fatalf("SaveLockfile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("lockfile does not end with newline")
	}

	var decoded Lockfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("saved lockfile is invalid JSON: %v", err)
	}
	if decoded.Version != LockfileVersion {
		t.Fatalf("Version = %d, want %d", decoded.Version, LockfileVersion)
	}
	if decoded.GeneratedBy != "sanad" {
		t.Fatalf("GeneratedBy = %q, want sanad", decoded.GeneratedBy)
	}
	if len(decoded.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(decoded.Entries))
	}
	if decoded.Entries[0].File != ".github/workflows/a.yml" || decoded.Entries[1].File != ".github/workflows/z.yml" {
		t.Fatalf("entries not sorted deterministically: %#v", decoded.Entries)
	}
}

func TestMergeMetadata(t *testing.T) {
	comment := CommentResult{
		Present: true,
		Metadata: Metadata{
			LogicalRef: "v4",
			Source:     SourceComment,
		},
	}
	lockfile := Metadata{
		LogicalRef: "v4",
		Source:     SourceLockfile,
	}

	got, ok, err := Merge(comment, lockfile, true)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}
	if !ok {
		t.Fatal("Merge ok = false, want true")
	}
	if got.Source != SourceLockfile {
		t.Fatalf("Source = %q, want lockfile", got.Source)
	}

	_, _, err = Merge(comment, Metadata{LogicalRef: "v5", Source: SourceLockfile}, true)
	if err == nil {
		t.Fatal("Merge returned nil error for conflicting metadata")
	}
}
