package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/MohamedElashri/sanad/internal/githubresolver"
	"github.com/MohamedElashri/sanad/internal/metadata"
	"github.com/MohamedElashri/sanad/internal/policy"
)

func TestSelectAutomaticUpgradeLatestEligibleFallsBackWithinCooldown(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("1", 40)
	resolver := fakePlanResolver{
		"actions/checkout@v6.0.0": resolvedRelease("actions", "checkout", "v6.0.0", strings.Repeat("6", 40), now.Add(-2*24*time.Hour)),
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", strings.Repeat("5", 40), now.Add(-20*24*time.Hour)),
	}
	cfg := config.Default()
	result, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(resolver), actions.Parse("actions/checkout@"+currentSHA), "v4", nil, effectiveUpgradePolicy{Level: "major", Selection: "latest-eligible"}, cfg, now)
	if err != nil {
		t.Fatalf("selectAutomaticUpgrade returned error: %v", err)
	}
	if result.TargetLogicalRef != "v5.0.0" || result.Decision.Kind != policy.DecisionUpdate {
		t.Fatalf("selection = %q %q, want v5.0.0 update", result.TargetLogicalRef, result.Decision.Kind)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].Decision != policy.DecisionPending {
		t.Fatalf("candidates = %#v, want cooling v6 followed by eligible v5", result.Candidates)
	}
}

func TestSelectAutomaticUpgradeLatestWaitsForNewest(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	resolver := fakePlanResolver{
		"actions/checkout@v6.0.0": resolvedRelease("actions", "checkout", "v6.0.0", strings.Repeat("6", 40), now.Add(-2*24*time.Hour)),
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", strings.Repeat("5", 40), now.Add(-20*24*time.Hour)),
	}
	result, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(resolver), actions.Parse("actions/checkout@"+strings.Repeat("1", 40)), "v4", nil, effectiveUpgradePolicy{Level: "major", Selection: "latest"}, config.Default(), now)
	if err != nil {
		t.Fatalf("selectAutomaticUpgrade returned error: %v", err)
	}
	if result.TargetLogicalRef != "v6.0.0" || result.Decision.Kind != policy.DecisionPending || len(result.Candidates) != 1 {
		t.Fatalf("unexpected latest selection: %#v", result)
	}
}

func TestMatchingUpgradeReleasesHonorsLevelsConstraintsAndStableOnly(t *testing.T) {
	now := time.Now().UTC()
	releases := []githubresolver.Release{
		{TagName: "v5.0.0", PublishedAt: now},
		{TagName: "v4.3.0", PublishedAt: now},
		{TagName: "v4.2.2", PublishedAt: now},
		{TagName: "v4.2.3-rc.1", Prerelease: true, PublishedAt: now},
		{TagName: "stable", PublishedAt: now},
		{TagName: "v9.0.0", Draft: true, PublishedAt: now},
	}
	current, _ := stableVersion("v4.2")

	patches, err := matchingUpgradeReleases(releases, current, effectiveUpgradePolicy{Level: "patch"})
	if err != nil || len(patches) != 1 || patches[0].Release.TagName != "v4.2.2" {
		t.Fatalf("patch matches = %#v, err=%v", patches, err)
	}
	minors, err := matchingUpgradeReleases(releases, current, effectiveUpgradePolicy{Level: "minor"})
	if err != nil || len(minors) != 2 || minors[0].Release.TagName != "v4.3.0" {
		t.Fatalf("minor matches = %#v, err=%v", minors, err)
	}
	constrained, err := matchingUpgradeReleases(releases, current, effectiveUpgradePolicy{Constraint: ">= 4.3, < 5"})
	if err != nil || len(constrained) != 1 || constrained[0].Release.TagName != "v4.3.0" {
		t.Fatalf("constraint matches = %#v, err=%v", constrained, err)
	}
}

func TestSelectAutomaticUpgradeInfersPartialRefVersionToPreventDowngrade(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("3", 40)
	resolver := fakePlanResolver{
		"actions/checkout@v4.3.0": resolvedRelease("actions", "checkout", "v4.3.0", currentSHA, now.Add(-30*24*time.Hour)),
		"actions/checkout@v4.2.0": resolvedRelease("actions", "checkout", "v4.2.0", strings.Repeat("2", 40), now.Add(-40*24*time.Hour)),
	}
	result, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(resolver), actions.Parse("actions/checkout@"+currentSHA), "v4", nil, effectiveUpgradePolicy{Constraint: "< 4.3", Selection: "latest-eligible"}, config.Default(), now)
	if err != nil {
		t.Fatalf("selectAutomaticUpgrade returned error: %v", err)
	}
	if result.Decision.Kind != policy.DecisionUnchanged || result.Candidate != nil {
		t.Fatalf("partial current ref allowed a downgrade: %#v", result)
	}
}

func TestEffectiveUpgradePolicyPrecedence(t *testing.T) {
	cfg := config.Default()
	cfg.Upgrade.Actions["actions/checkout"] = config.UpgradePolicy{Level: "minor", Selection: "latest"}
	effective, err := effectivePolicyForAction(cfg, &upgradeOptions{constraint: "< 6", constraintSet: true, selection: "latest-eligible", selectionSet: true}, "actions/checkout")
	if err != nil {
		t.Fatalf("effectivePolicyForAction returned error: %v", err)
	}
	if effective.Level != "" || effective.Constraint != "< 6" || effective.Selection != "latest-eligible" {
		t.Fatalf("unexpected effective policy: %#v", effective)
	}
}

func TestSelectAutomaticUpgradeFirstSeenTracksHistoryAndResetsRetag(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	oldSeen := now.Add(-20 * 24 * time.Hour).Format(time.RFC3339)
	oldV6SHA := strings.Repeat("a", 40)
	newV6SHA := strings.Repeat("6", 40)
	v5SHA := strings.Repeat("5", 40)
	history := []metadata.CandidateHistoryEntry{
		{LogicalRef: "v6.0.0", SHA: oldV6SHA, SeenAt: oldSeen},
		{LogicalRef: "v5.0.0", SHA: v5SHA, SeenAt: oldSeen},
	}
	resolver := fakePlanResolver{
		"actions/checkout@v6.0.0": resolvedRelease("actions", "checkout", "v6.0.0", newV6SHA, now.Add(-30*24*time.Hour)),
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", v5SHA, now.Add(-30*24*time.Hour)),
	}
	cfg := config.Default()
	cfg.CooldownSource = string(policy.CooldownSourceFirstSeen)
	result, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(resolver), actions.Parse("actions/checkout@"+strings.Repeat("1", 40)), "v4", history, effectiveUpgradePolicy{Level: "major", Selection: "latest-eligible"}, cfg, now)
	if err != nil {
		t.Fatalf("selectAutomaticUpgrade returned error: %v", err)
	}
	if result.TargetLogicalRef != "v5.0.0" || result.Decision.Kind != policy.DecisionUpdate {
		t.Fatalf("unexpected first-seen selection: %#v", result)
	}
	if len(result.History) != 1 || result.History[0].LogicalRef != "v6.0.0" || result.History[0].SHA != newV6SHA || result.History[0].SeenAt != now.Format(time.RFC3339) {
		t.Fatalf("retagged history was not reset and retained: %#v", result.History)
	}
}

func TestSelectAutomaticUpgradePrunesDeletedReleaseHistory(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	v5SHA := strings.Repeat("5", 40)
	history := []metadata.CandidateHistoryEntry{
		{LogicalRef: "v6.0.0", SHA: strings.Repeat("6", 40), SeenAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339)},
		{LogicalRef: "v5.0.0", SHA: v5SHA, SeenAt: now.Format(time.RFC3339)},
	}
	resolver := fakePlanResolver{
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", v5SHA, now.Add(-30*24*time.Hour)),
	}
	cfg := config.Default()
	cfg.CooldownSource = string(policy.CooldownSourceFirstSeen)
	result, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(resolver), actions.Parse("actions/checkout@"+strings.Repeat("1", 40)), "v4", history, effectiveUpgradePolicy{Level: "major", Selection: "latest-eligible"}, cfg, now)
	if err != nil {
		t.Fatalf("selectAutomaticUpgrade returned error: %v", err)
	}
	if len(result.History) != 1 || result.History[0].LogicalRef != "v5.0.0" {
		t.Fatalf("deleted release history was not pruned: %#v", result.History)
	}
}

func TestSelectAutomaticUpgradeRejectsNonSemverCurrentRef(t *testing.T) {
	_, err := selectAutomaticUpgrade(context.Background(), newUpgradeDiscovery(fakePlanResolver{}), actions.Parse("actions/checkout@"+strings.Repeat("1", 40)), "stable", nil, effectiveUpgradePolicy{Level: "major", Selection: "latest-eligible"}, config.Default(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "use --to") {
		t.Fatalf("error = %v, want explicit upgrade guidance", err)
	}
}

func TestUpgradeFirstSeenWritePersistsObservationsWithoutChangingWorkflow(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	currentSHA := strings.Repeat("1", 40)
	installPlanTestResolver(t, fakePlanResolver{
		"actions/checkout@v5.0.0": resolvedRelease("actions", "checkout", "v5.0.0", strings.Repeat("5", 40), now.Add(-30*24*time.Hour)),
	}, now)
	withTempWorkingDir(t)
	if err := os.WriteFile(".sanad.toml", []byte("cooldown_source = \"first-seen\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+currentSHA+" # sanad: ref=v4\n")
	original := readFileString(t, path)
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"upgrade", "--write"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := readFileString(t, path); got != original {
		t.Fatalf("workflow changed while candidate was first seen\nwant:\n%s\ngot:\n%s", original, got)
	}
	lockfile, ok, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil || !ok || len(lockfile.Entries) != 1 {
		t.Fatalf("unexpected lockfile: ok=%v err=%v entries=%#v", ok, err, lockfile.Entries)
	}
	entry := lockfile.Entries[0]
	if entry.LogicalRef != "v4" || len(entry.Candidates) != 1 || entry.Candidates[0].LogicalRef != "v5.0.0" {
		t.Fatalf("unexpected pending observation: %#v", entry)
	}
	if !strings.Contains(out.String(), "Updated lockfile observations") {
		t.Fatalf("missing lock-only write message: %s", out.String())
	}
}

func resolvedRelease(owner, repo, ref, sha string, publishedAt time.Time) githubresolver.ResolvedRef {
	return githubresolver.ResolvedRef{
		Owner:       owner,
		Repo:        repo,
		Ref:         ref,
		SHA:         sha,
		Kind:        githubresolver.KindTag,
		CommitTime:  publishedAt,
		ReleaseTime: timePointer(publishedAt),
	}
}
