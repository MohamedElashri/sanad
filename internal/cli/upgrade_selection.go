package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/MohamedElashri/sanad/internal/githubresolver"
	"github.com/MohamedElashri/sanad/internal/metadata"
	"github.com/MohamedElashri/sanad/internal/policy"
)

type effectiveUpgradePolicy struct {
	Level      string
	Constraint string
	Selection  string
}

type upgradeTargetSelection struct {
	Candidate        *githubresolver.ResolvedRef
	TargetLogicalRef string
	Decision         policy.Decision
	Candidates       []upgradeCandidate
	History          []metadata.CandidateHistoryEntry
	Policy           effectiveUpgradePolicy
}

type versionedRelease struct {
	Release githubresolver.Release
	Version *semver.Version
}

type upgradeDiscovery struct {
	resolver     planResolver
	releaseCache map[string][]githubresolver.Release
	resolveCache map[string]githubresolver.ResolvedRef
}

func newUpgradeDiscovery(resolver planResolver) *upgradeDiscovery {
	return &upgradeDiscovery{
		resolver:     resolver,
		releaseCache: make(map[string][]githubresolver.Release),
		resolveCache: make(map[string]githubresolver.ResolvedRef),
	}
}

func (d *upgradeDiscovery) releases(ctx context.Context, owner, repo string) ([]githubresolver.Release, error) {
	key := owner + "/" + repo
	if releases, ok := d.releaseCache[key]; ok {
		return releases, nil
	}
	resolver, ok := d.resolver.(releaseResolver)
	if !ok {
		return nil, fmt.Errorf("resolver does not support release discovery")
	}
	releases, err := resolver.ListReleases(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	d.releaseCache[key] = releases
	return releases, nil
}

func (d *upgradeDiscovery) resolve(ctx context.Context, selector githubresolver.ActionSelector) (githubresolver.ResolvedRef, error) {
	key := selector.Owner + "/" + selector.Repo + "@" + selector.Ref
	if resolved, ok := d.resolveCache[key]; ok {
		return resolved, nil
	}
	resolved, err := d.resolver.Resolve(ctx, selector)
	if err != nil {
		return githubresolver.ResolvedRef{}, err
	}
	d.resolveCache[key] = resolved
	return resolved, nil
}

func effectivePolicyForAction(cfg config.Config, opts *upgradeOptions, selector string) (effectiveUpgradePolicy, error) {
	effective := effectiveUpgradePolicy{
		Level:      cfg.Upgrade.Level,
		Constraint: cfg.Upgrade.Constraint,
		Selection:  cfg.Upgrade.Selection,
	}
	if override, ok := cfg.Upgrade.Actions[selector]; ok {
		if override.Level != "" || override.Constraint != "" {
			effective.Level = override.Level
			effective.Constraint = override.Constraint
		}
		if override.Selection != "" {
			effective.Selection = override.Selection
		}
	}
	if opts.levelSet {
		level, err := normalizeUpgradeLevelForCLI(opts.level)
		if err != nil {
			return effectiveUpgradePolicy{}, err
		}
		effective.Level, effective.Constraint = level, ""
	}
	if opts.constraintSet {
		constraint, err := normalizeUpgradeConstraintForCLI(opts.constraint)
		if err != nil {
			return effectiveUpgradePolicy{}, err
		}
		effective.Constraint, effective.Level = constraint, ""
	}
	if opts.selectionSet {
		selection, err := normalizeUpgradeSelectionForCLI(opts.selection)
		if err != nil {
			return effectiveUpgradePolicy{}, err
		}
		effective.Selection = selection
	}
	return effective, nil
}

func normalizeUpgradeConstraintForCLI(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--constraint must not be empty")
	}
	if _, err := semver.NewConstraint(value); err != nil {
		return "", fmt.Errorf("--constraint %q is invalid: %w", value, err)
	}
	return value, nil
}

func selectAutomaticUpgrade(
	ctx context.Context,
	discovery *upgradeDiscovery,
	parsed actions.ParsedAction,
	currentLogicalRef string,
	existingHistory []metadata.CandidateHistoryEntry,
	effective effectiveUpgradePolicy,
	cfg config.Config,
	now time.Time,
) (upgradeTargetSelection, error) {
	result := upgradeTargetSelection{Policy: effective, History: append([]metadata.CandidateHistoryEntry(nil), existingHistory...)}
	currentVersion, err := stableVersion(currentLogicalRef)
	if err != nil {
		return result, fmt.Errorf("current logical ref %q is not a stable SemVer version; use --to for an explicit upgrade", currentLogicalRef)
	}

	releases, err := discovery.releases(ctx, parsed.Owner, parsed.Repo)
	if err != nil {
		return result, err
	}
	result.History = retainPublishedCandidateHistory(result.History, releases)
	currentVersion, err = inferPartialCurrentVersion(ctx, discovery, parsed, currentLogicalRef, currentVersion, releases)
	if err != nil {
		return result, err
	}
	result.History = pruneAdoptedCandidateHistory(result.History, currentVersion)
	matching, err := matchingUpgradeReleases(releases, currentVersion, effective)
	if err != nil {
		return result, err
	}
	if len(matching) == 0 {
		result.TargetLogicalRef = currentLogicalRef
		result.Decision = policy.Decision{
			Kind:       policy.DecisionUnchanged,
			Reason:     "no newer stable release matches the effective upgrade policy",
			CurrentSHA: parsed.Ref,
			LogicalRef: currentLogicalRef,
		}
		return result, nil
	}

	limit := len(matching)
	if effective.Selection == "latest" {
		limit = 1
	}
	var firstCandidate *githubresolver.ResolvedRef
	var firstDecision policy.Decision
	for i := 0; i < limit; i++ {
		item := matching[i]
		resolved, resolveErr := discovery.resolve(ctx, githubresolver.ActionSelector{
			Owner: parsed.Owner,
			Repo:  parsed.Repo,
			Ref:   item.Release.TagName,
		})
		if resolveErr != nil {
			return result, resolveErr
		}
		if !item.Release.PublishedAt.IsZero() {
			resolved.ReleaseTime = timePointer(item.Release.PublishedAt)
		} else if !item.Release.CreatedAt.IsZero() {
			resolved.ReleaseTime = timePointer(item.Release.CreatedAt)
		}

		seenAt := candidateHistorySeenAt(result.History, resolved)
		if cfg.CooldownSource == string(policy.CooldownSourceFirstSeen) {
			if seenAt.IsZero() {
				seenAt = now
			}
			result.History = observeUpgradeCandidate(result.History, resolved, seenAt)
		}
		decision := policy.Evaluate(policy.Entry{
			Action:          parsed,
			Candidate:       &resolved,
			LogicalRef:      resolved.Ref,
			CurrentSHA:      parsed.Ref,
			CandidateSeenAt: seenAt,
			Now:             now,
		}, policyOptionsFromConfig(cfg, now))
		reportCandidate := upgradeCandidate{
			LogicalRef: resolved.Ref,
			Version:    item.Version.String(),
			SHA:        resolved.SHA,
			Decision:   decision.Kind,
			Reason:     decision.Reason,
		}
		if decision.Age != 0 {
			reportCandidate.Age = decision.Age.String()
		}
		result.Candidates = append(result.Candidates, reportCandidate)
		if i == 0 {
			candidateCopy := resolved
			firstCandidate = &candidateCopy
			firstDecision = decision
		}
		if decision.Kind == policy.DecisionUpdate || decision.Kind == policy.DecisionUnchanged {
			result.Candidate = &resolved
			result.TargetLogicalRef = resolved.Ref
			result.Decision = decision
			result.History = pruneAdoptedCandidateHistory(result.History, item.Version)
			return result, nil
		}
		if decision.Kind != policy.DecisionPending {
			result.Candidate = &resolved
			result.TargetLogicalRef = resolved.Ref
			result.Decision = decision
			return result, nil
		}
	}

	result.Candidate = firstCandidate
	result.TargetLogicalRef = firstCandidate.Ref
	result.Decision = firstDecision
	return result, nil
}

func matchingUpgradeReleases(releases []githubresolver.Release, current *semver.Version, effective effectiveUpgradePolicy) ([]versionedRelease, error) {
	var constraint *semver.Constraints
	var err error
	if effective.Constraint != "" {
		constraint, err = semver.NewConstraint(effective.Constraint)
		if err != nil {
			return nil, err
		}
	}
	matching := make([]versionedRelease, 0, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		version, parseErr := stableVersion(release.TagName)
		if parseErr != nil || !version.GreaterThan(current) {
			continue
		}
		if constraint != nil && !constraint.Check(version) {
			continue
		}
		if constraint == nil && !levelAllowsVersion(effective.Level, current, version) {
			continue
		}
		matching = append(matching, versionedRelease{Release: release, Version: version})
	}
	sort.SliceStable(matching, func(i, j int) bool {
		comparison := matching[i].Version.Compare(matching[j].Version)
		if comparison != 0 {
			return comparison > 0
		}
		leftTime := releaseTime(matching[i].Release)
		rightTime := releaseTime(matching[j].Release)
		if !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		return matching[i].Release.TagName < matching[j].Release.TagName
	})
	return matching, nil
}

func inferPartialCurrentVersion(ctx context.Context, discovery *upgradeDiscovery, parsed actions.ParsedAction, logicalRef string, baseline *semver.Version, releases []githubresolver.Release) (*semver.Version, error) {
	normalized := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(logicalRef), "v"), "V")
	parts := strings.Split(normalized, ".")
	if len(parts) >= 3 {
		return baseline, nil
	}
	var possible []versionedRelease
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		version, err := stableVersion(release.TagName)
		if err != nil || version.LessThan(baseline) || version.Major() != baseline.Major() {
			continue
		}
		if len(parts) == 2 && version.Minor() != baseline.Minor() {
			continue
		}
		possible = append(possible, versionedRelease{Release: release, Version: version})
	}
	sort.SliceStable(possible, func(i, j int) bool { return possible[i].Version.GreaterThan(possible[j].Version) })
	for _, item := range possible {
		resolved, err := discovery.resolve(ctx, githubresolver.ActionSelector{Owner: parsed.Owner, Repo: parsed.Repo, Ref: item.Release.TagName})
		if err != nil {
			return nil, err
		}
		if resolved.SHA == parsed.Ref {
			return item.Version, nil
		}
	}
	return baseline, nil
}

func stableVersion(value string) (*semver.Version, error) {
	version, err := semver.NewVersion(strings.TrimSpace(value))
	if err != nil || version.Prerelease() != "" {
		if err == nil {
			err = fmt.Errorf("prerelease versions are not automatic upgrade baselines")
		}
		return nil, err
	}
	return version, nil
}

func levelAllowsVersion(level string, current, candidate *semver.Version) bool {
	switch level {
	case "patch":
		return candidate.Major() == current.Major() && candidate.Minor() == current.Minor()
	case "minor":
		return candidate.Major() == current.Major()
	case "major":
		return true
	default:
		return false
	}
}

func candidateHistorySeenAt(history []metadata.CandidateHistoryEntry, candidate githubresolver.ResolvedRef) time.Time {
	for _, observed := range history {
		if observed.LogicalRef != candidate.Ref || observed.SHA != candidate.SHA {
			continue
		}
		seenAt, err := time.Parse(time.RFC3339, observed.SeenAt)
		if err == nil {
			return seenAt
		}
	}
	return time.Time{}
}

func observeUpgradeCandidate(history []metadata.CandidateHistoryEntry, candidate githubresolver.ResolvedRef, seenAt time.Time) []metadata.CandidateHistoryEntry {
	updated := make([]metadata.CandidateHistoryEntry, 0, len(history)+1)
	found := false
	for _, observed := range history {
		if observed.LogicalRef == candidate.Ref {
			if observed.SHA == candidate.SHA {
				updated = append(updated, observed)
				found = true
			}
			continue
		}
		updated = append(updated, observed)
	}
	if !found {
		updated = append(updated, metadata.CandidateHistoryEntry{
			LogicalRef: candidate.Ref,
			SHA:        candidate.SHA,
			SeenAt:     seenAt.UTC().Format(time.RFC3339),
		})
	}
	return updated
}

func pruneAdoptedCandidateHistory(history []metadata.CandidateHistoryEntry, adopted *semver.Version) []metadata.CandidateHistoryEntry {
	kept := make([]metadata.CandidateHistoryEntry, 0, len(history))
	for _, observed := range history {
		version, err := stableVersion(observed.LogicalRef)
		if err == nil && !version.GreaterThan(adopted) {
			continue
		}
		kept = append(kept, observed)
	}
	return kept
}

func retainPublishedCandidateHistory(history []metadata.CandidateHistoryEntry, releases []githubresolver.Release) []metadata.CandidateHistoryEntry {
	published := make(map[string]struct{}, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		if _, err := stableVersion(release.TagName); err == nil {
			published[release.TagName] = struct{}{}
		}
	}
	kept := make([]metadata.CandidateHistoryEntry, 0, len(history))
	for _, observed := range history {
		if _, ok := published[observed.LogicalRef]; ok {
			kept = append(kept, observed)
		}
	}
	return kept
}

func removeUpgradeCandidate(history []metadata.CandidateHistoryEntry, logicalRef string, candidate *githubresolver.ResolvedRef) []metadata.CandidateHistoryEntry {
	if candidate == nil {
		return history
	}
	kept := make([]metadata.CandidateHistoryEntry, 0, len(history))
	for _, observed := range history {
		if observed.LogicalRef == logicalRef && observed.SHA == candidate.SHA {
			continue
		}
		kept = append(kept, observed)
	}
	return kept
}

func releaseTime(release githubresolver.Release) time.Time {
	if !release.PublishedAt.IsZero() {
		return release.PublishedAt
	}
	return release.CreatedAt
}

func timePointer(value time.Time) *time.Time {
	copy := value
	return &copy
}

func candidateHistoriesEqual(left, right []metadata.CandidateHistoryEntry) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[metadata.CandidateHistoryEntry]int, len(left))
	for _, candidate := range left {
		counts[candidate]++
	}
	for _, candidate := range right {
		if counts[candidate] == 0 {
			return false
		}
		counts[candidate]--
	}
	return true
}
