package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
)

const DefaultLockfilePath = ".github/sanad.lock.json"
const LockfileVersion = 1

type Source string

const (
	SourceComment  Source = "comment"
	SourceLockfile Source = "lockfile"
)

type Metadata struct {
	LogicalRef string
	Source     Source
}

type CommentResult struct {
	Metadata Metadata
	Present  bool
}

func ParseComment(comment string) (CommentResult, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return CommentResult{}, nil
	}

	index := strings.Index(comment, "sanad:")
	if index < 0 {
		return CommentResult{}, nil
	}

	fields := strings.Fields(comment[index+len("sanad:"):])
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key != "ref" {
			continue
		}
		value = strings.TrimSpace(strings.Trim(value, `"'`))
		if value == "" {
			return CommentResult{Present: true}, fmt.Errorf("sanad comment ref must not be empty")
		}
		return CommentResult{
			Metadata: Metadata{
				LogicalRef: value,
				Source:     SourceComment,
			},
			Present: true,
		}, nil
	}

	return CommentResult{Present: true}, fmt.Errorf("sanad comment must include ref metadata")
}

type Lockfile struct {
	Version     int             `json:"version"`
	GeneratedBy string          `json:"generated_by"`
	Entries     []LockfileEntry `json:"entries"`
}

type LockfileEntry struct {
	File            string `json:"file"`
	Node            string `json:"node"`
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	Path            string `json:"path"`
	Kind            string `json:"kind"`
	LogicalRef      string `json:"logical_ref"`
	PinnedSHA       string `json:"pinned_sha,omitempty"`
	CandidateSHA    string `json:"candidate_sha,omitempty"`
	CandidateSeenAt string `json:"candidate_seen_at,omitempty"`
	ResolvedAt      string `json:"resolved_at,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	TimestampSource string `json:"timestamp_source,omitempty"`
}

type LockfileMetadataValue struct {
	Metadata Metadata
	Entry    LockfileEntry
}

type LockfileMetadata map[string]LockfileMetadataValue

func LoadLockfileMetadata(path string) (LockfileMetadata, bool, error) {
	lockfile, ok, err := LoadLockfile(path)
	if err != nil || !ok {
		return nil, ok, err
	}

	values := make(LockfileMetadata)
	for _, entry := range lockfile.Entries {
		values[Key(entry.File, entry.Node)] = LockfileMetadataValue{
			Metadata: Metadata{
				LogicalRef: entry.LogicalRef,
				Source:     SourceLockfile,
			},
			Entry: entry,
		}
	}
	return values, true, nil
}

func LoadLockfile(path string) (Lockfile, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Lockfile{}, false, nil
		}
		return Lockfile{}, false, fmt.Errorf("load lockfile %q: %w", path, err)
	}

	var lockfile Lockfile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return Lockfile{}, true, fmt.Errorf("load lockfile %q: invalid JSON: %w", path, err)
	}
	if err := ValidateLockfile(lockfile); err != nil {
		return Lockfile{}, true, fmt.Errorf("load lockfile %q: %w", path, err)
	}
	return NormalizeLockfile(lockfile), true, nil
}

func ValidateLockfile(lockfile Lockfile) error {
	if lockfile.Version != LockfileVersion {
		return fmt.Errorf("unsupported lockfile version %d: expected %d", lockfile.Version, LockfileVersion)
	}

	seen := make(map[string]struct{}, len(lockfile.Entries))
	for i, entry := range lockfile.Entries {
		if err := validateLockfileEntry(entry, i); err != nil {
			return err
		}

		key := Key(entry.File, entry.Node)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate lockfile entry for %s %s", entry.File, entry.Node)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateLockfileEntry(entry LockfileEntry, i int) error {
	if entry.File == "" {
		return fmt.Errorf("entry %d file is required", i)
	}
	if entry.Node == "" {
		return fmt.Errorf("entry %d node is required", i)
	}
	if entry.Owner == "" {
		return fmt.Errorf("entry %d owner is required", i)
	}
	if entry.Repo == "" {
		return fmt.Errorf("entry %d repo is required", i)
	}
	if entry.Kind == "" {
		return fmt.Errorf("entry %d kind is required", i)
	}
	if entry.LogicalRef == "" {
		return fmt.Errorf("entry %d logical_ref is required", i)
	}
	if entry.PinnedSHA != "" && !actions.IsFullSHA(entry.PinnedSHA) {
		return fmt.Errorf("entry %d pinned_sha must be a full 40-character SHA", i)
	}
	if entry.CandidateSHA != "" && !actions.IsFullSHA(entry.CandidateSHA) {
		return fmt.Errorf("entry %d candidate_sha must be a full 40-character SHA", i)
	}
	if entry.PinnedSHA == "" && entry.CandidateSHA == "" {
		return fmt.Errorf("entry %d must include pinned_sha or candidate_sha", i)
	}
	if entry.CandidateSHA != "" {
		if entry.CandidateSeenAt == "" {
			return fmt.Errorf("entry %d candidate_seen_at is required when candidate_sha is set", i)
		}
		if _, err := time.Parse(time.RFC3339, entry.CandidateSeenAt); err != nil {
			return fmt.Errorf("entry %d candidate_seen_at must be RFC3339: %w", i, err)
		}
	} else if entry.CandidateSeenAt != "" {
		return fmt.Errorf("entry %d candidate_seen_at requires candidate_sha", i)
	}
	return nil
}

func NormalizeLockfile(lockfile Lockfile) Lockfile {
	if lockfile.Version == 0 {
		lockfile.Version = LockfileVersion
	}
	if lockfile.GeneratedBy == "" {
		lockfile.GeneratedBy = "sanad"
	}
	lockfile.Entries = append([]LockfileEntry(nil), lockfile.Entries...)
	SortLockfileEntries(lockfile.Entries)
	return lockfile
}

func NewLockfile(entries []LockfileEntry) (Lockfile, error) {
	lockfile := NormalizeLockfile(Lockfile{
		Version:     LockfileVersion,
		GeneratedBy: "sanad",
		Entries:     entries,
	})
	if err := ValidateLockfile(lockfile); err != nil {
		return Lockfile{}, err
	}
	return lockfile, nil
}

func UpdateLockfile(existing Lockfile, active []LockfileEntry) (Lockfile, error) {
	updated := NormalizeLockfile(existing)
	updated.Version = LockfileVersion
	updated.GeneratedBy = "sanad"
	updated.Entries = append([]LockfileEntry(nil), active...)
	if err := ValidateLockfile(updated); err != nil {
		return Lockfile{}, err
	}
	return NormalizeLockfile(updated), nil
}

func SaveLockfile(path string, lockfile Lockfile) error {
	normalized := NormalizeLockfile(lockfile)
	if err := ValidateLockfile(normalized); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write lockfile %q: %w", path, err)
	}
	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write lockfile %q: %w", path, err)
	}
	return nil
}

func SortLockfileEntries(entries []LockfileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		switch {
		case left.File != right.File:
			return left.File < right.File
		case left.Node != right.Node:
			return left.Node < right.Node
		case left.Owner != right.Owner:
			return left.Owner < right.Owner
		case left.Repo != right.Repo:
			return left.Repo < right.Repo
		case left.Path != right.Path:
			return left.Path < right.Path
		default:
			return left.LogicalRef < right.LogicalRef
		}
	})
}

func Key(file string, node string) string {
	return file + "\x00" + node
}

func Merge(comment CommentResult, lockfile Metadata, hasLockfile bool) (Metadata, bool, error) {
	if hasLockfile && comment.Present {
		if lockfile.LogicalRef != comment.Metadata.LogicalRef {
			return Metadata{}, false, fmt.Errorf("metadata conflict: lockfile ref %q disagrees with comment ref %q", lockfile.LogicalRef, comment.Metadata.LogicalRef)
		}
		return lockfile, true, nil
	}
	if hasLockfile {
		return lockfile, true, nil
	}
	if comment.Present {
		return comment.Metadata, true, nil
	}
	return Metadata{}, false, nil
}
