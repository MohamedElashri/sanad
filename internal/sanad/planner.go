package sanad

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func targets(releases []Release, current string, p UpdatePolicy, explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	currentVersion, full := policyVersion(current, p)
	if !full {
		return nil
	}
	from, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil
	}
	var constraint *semver.Constraints
	if p.Constraint != "" {
		constraint, _ = semver.NewConstraint(p.Constraint)
	}
	type candidate struct {
		ref     string
		version *semver.Version
	}
	list := []candidate{}
	seen := map[string]bool{}
	for _, r := range releases {
		version, full := policyVersion(r.Ref, p)
		if !full {
			continue
		}
		v, err := semver.NewVersion(version)
		if err != nil || r.Draft || r.Prerelease || v.Prerelease() != "" || !v.GreaterThan(from) || seen[r.Ref] {
			continue
		}
		if constraint != nil && !constraint.Check(v) {
			continue
		}
		if p.Level == "patch" && (v.Major() != from.Major() || v.Minor() != from.Minor()) {
			continue
		}
		if p.Level == "minor" && v.Major() != from.Major() {
			continue
		}
		list = append(list, candidate{r.Ref, v})
		seen[r.Ref] = true
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].version.Equal(list[j].version) {
			return list[i].ref < list[j].ref
		}
		return list[i].version.GreaterThan(list[j].version)
	})
	out := []string{}
	for _, c := range list {
		out = append(out, c.ref)
	}
	return out
}

func assessReleaseAge(ctx context.Context, before, after graph, minimum time.Duration, now time.Time, source ReleaseSource) (bool, time.Time, error) {
	ready := true
	var until time.Time
	if minimum == 0 {
		return ready, until, nil
	}
	for _, c := range graphDiff(before, after) {
		if c.After == nil || c.Before != nil && c.Before.Commit == c.After.Commit {
			continue
		}
		at, err := source.Timestamp(ctx, *c.After)
		if err != nil {
			return false, until, err
		}
		if at.IsZero() || at.After(now) {
			return false, until, fmt.Errorf("invalid upstream timestamp for %s", c.Key)
		}
		next := at.Add(minimum)
		if next.After(now) {
			ready = false
			if next.After(until) {
				until = next
			}
		}
	}
	return ready, until, nil
}

func closure(g graph, key string, set map[string]bool) {
	if set[key] {
		return
	}
	set[key] = true
	for _, child := range g[key].Uses {
		closure(g, child, set)
	}
}

func validateScope(before, after graph, oldKey, newKey string) error {
	allowed := map[string]bool{}
	next := map[string]bool{}
	closure(before, oldKey, allowed)
	closure(after, newKey, next)
	for key := range next {
		allowed[key] = true
	}
	for _, change := range graphDiff(before, after) {
		if !allowed[change.Key] {
			return fmt.Errorf("native producer changed unrelated dependency %s; update refused", change.Key)
		}
	}
	return nil
}

func effectivePolicy(cfg Config, opts Options, name, nwo string) UpdatePolicy {
	p := cfg.Update
	for _, key := range []string{name, nwo} {
		if override, ok := cfg.Actions[canonicalActionName(key)]; ok && (override.Level != "" || override.Constraint != "" || override.TagPrefix != "" || override.PreferTags != nil) {
			if override.Level != "" || override.Constraint != "" {
				p.Level = override.Level
				p.Constraint = override.Constraint
			}
			if override.TagPrefix != "" {
				p.TagPrefix = override.TagPrefix
			}
			if override.PreferTags != nil {
				p.PreferTags = override.PreferTags
			}
			break
		}
	}
	if opts.Level != "" || opts.Constraint != "" {
		p.Level = opts.Level
		p.Constraint = opts.Constraint
	}
	return p
}
func lockParsePrefix(prefix string) bool {
	return lock.ParseActionRef("owner/repo@"+prefix+"1.2.3") != nil
}
func policyVersion(ref string, p UpdatePolicy) (string, bool) {
	if p.TagPrefix != "" {
		if !strings.HasPrefix(ref, p.TagPrefix) {
			return "", false
		}
		ref = strings.TrimPrefix(ref, p.TagPrefix)
	}
	v, ok := lock.ParseSemVer(ref)
	return ref, ok && v.IsFull() && v.IsStable()
}

func matchesAction(selected string, ref reference) bool {
	return selected == "" || canonicalActionName(selected) == canonicalActionName(ref.Name) || canonicalActionName(selected) == canonicalActionName(ref.NWO)
}

func validateTransitivePolicies(before, after graph, selected string, cfg Config, opts Options) error {
	for _, c := range graphDiff(before, after) {
		if c.After == nil || c.Key == selected || c.Before != nil && c.Before.Commit == c.After.Commit {
			continue
		}
		nwo, ref, _ := strings.Cut(c.Key, "@")
		policy := effectivePolicy(cfg, opts, nwo, nwo)
		versionRef := ref
		if policy.TagPrefix != "" {
			var ok bool
			versionRef, ok = policyVersion(ref, policy)
			if !ok {
				return fmt.Errorf("%s is outside tag family %s", c.Key, policy.TagPrefix)
			}
		}
		version, err := semver.NewVersion(versionRef)
		if policy.Constraint != "" {
			constraint, _ := semver.NewConstraint(policy.Constraint)
			if err != nil || !constraint.Check(version) {
				return fmt.Errorf("%s violates constraint %s", c.Key, policy.Constraint)
			}
		}
		if err != nil || policy.Level == "" || policy.Level == "major" {
			continue
		}
		previous := []string{}
		allowed := false
		for key, old := range before {
			oldNWO, oldRef, _ := strings.Cut(key, "@")
			if oldNWO != nwo || old.Host != c.After.Host {
				continue
			}
			oldVersion := oldRef
			if policy.TagPrefix != "" {
				var ok bool
				oldVersion, ok = policyVersion(oldRef, policy)
				if !ok {
					continue
				}
			}
			v, err := semver.NewVersion(oldVersion)
			if err != nil {
				continue
			}
			previous = append(previous, oldRef)
			if version.Major() == v.Major() && (policy.Level != "patch" || version.Minor() == v.Minor()) {
				allowed = true
				break
			}
		}
		if len(previous) > 0 && !allowed {
			sort.Strings(previous)
			return fmt.Errorf("%s exceeds %s update policy from %s", c.Key, policy.Level, strings.Join(previous, ", "))
		}
	}
	return nil
}

func copyTree(files tree) tree {
	out := tree{}
	for k, v := range files {
		out[k] = v
	}
	return out
}

func installTree(root string, before, after tree) error {
	for _, name := range changedFiles(before, after) {
		f, ok := after[name]
		if ok {
			if err := putFile(root, name, f); err != nil {
				return err
			}
		} else {
			path, err := safePath(root, name)
			if err != nil {
				return err
			}
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func planUpdates(ctx context.Context, work, host, repo string, cfg Config, opts Options, backend Backend, r *Report) error {
	if opts.To != "" && opts.Action == "" {
		return fmt.Errorf("--to requires --action")
	}
	if opts.To != "" && (lock.ParseActionRef("owner/repo@"+opts.To) == nil || strings.ContainsAny(opts.To, "'\" #\t")) {
		return fmt.Errorf("invalid target reference")
	}
	policy := cfg.Update
	if opts.Level != "" || opts.Constraint != "" {
		policy = UpdatePolicy{Level: opts.Level, Constraint: opts.Constraint}
	}
	if err := validatePolicy(policy); err != nil {
		return err
	}
	input, err := readTree(work, opts.Config)
	if err != nil {
		return err
	}
	initial, err := loadGraph(input, host)
	if err != nil {
		return err
	}
	state, err := loadObservations(input, opts.Now)
	if err != nil {
		return err
	}
	minimum, err := duration(cfg.Observation.MinimumAge)
	if err != nil {
		return err
	}
	age, err := releaseCooldown(opts.Root)
	if err != nil {
		return err
	}
	source := opts.Releases
	if source == nil {
		source = newAPI(host)
	}
	refs, err := references(input)
	if err != nil {
		return err
	}
	groups := map[string][]reference{}
	keys := []string{}
	for _, ref := range refs {
		if ref.Workflow {
			continue
		}
		key := strings.ToLower(ref.NWO) + "@" + ref.Ref
		if _, exists := initial[key]; !exists {
			continue
		}
		if !matchesAction(opts.Action, ref) {
			continue
		}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], ref)
	}
	if opts.Action != "" && len(keys) == 0 && !hasWorkflowSelection(refs, opts.Action) {
		return fmt.Errorf("no locked action matches %s", opts.Action)
	}
	sort.Strings(keys)
	observed := map[string]time.Time{}
	for _, key := range keys {
		group := groups[key]
		ref := group[0]
		effective := effectivePolicy(cfg, opts, ref.Name, ref.NWO)
		var releases []Release
		if opts.To == "" {
			_, versionRef := policyVersion(ref.Ref, effective)
			if !versionRef {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, Status: "frozen", Reason: "only full stable versions update automatically; choose an explicit version with --to"})
				continue
			}
			releases, err = listCandidates(ctx, source, initial[key].Host, ref.NWO, effective)
			if err != nil {
				return err
			}
		}
		candidates := targets(releases, ref.Ref, effective, opts.To)
		for _, sibling := range group[1:] {
			allowed := map[string]bool{}
			for _, target := range targets(releases, sibling.Ref, effectivePolicy(cfg, opts, sibling.Name, sibling.NWO), opts.To) {
				allowed[target] = true
			}
			filtered := []string{}
			for _, target := range candidates {

				if allowed[target] {
					filtered = append(filtered, target)
				}
			}
			candidates = filtered
		}
		if len(candidates) == 0 {
			r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, Status: "unchanged", Reason: "no newer stable release matches policy"})
			continue
		}
		for _, target := range candidates {
			if age > 0 && tagOnlyCandidate(releases, target) {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, To: target, Status: "blocked", Reason: "tag-only candidate has no release publication timestamp for cooldown policy"})
				continue
			}
			if target == ref.Ref {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, To: target, Status: "unchanged", Reason: "already tracking this ref; mutable-ref refresh is not supported by the scoped native producer"})
				continue
			}
			current, err := readTree(work, opts.Config)
			if err != nil {
				return err
			}
			before, err := loadGraph(current, host)
			if err != nil {
				return err
			}
			edited := copyTree(current)
			// Re-read source positions after earlier edits, including flow-style YAML.
			currentRefs, err := references(edited)
			if err != nil {
				return err
			}
			selected := []reference{}
			for _, candidate := range currentRefs {
				if strings.EqualFold(candidate.NWO, ref.NWO) && candidate.Ref == ref.Ref && matchesAction(opts.Action, candidate) {
					selected = append(selected, candidate)
				}
			}
			if err := rewriteRefs(edited, selected, target); err != nil {
				return err
			}
			dir, err := snapshot(edited)
			if err != nil {
				return err
			}
			native, runErr := backend.Run(ctx, dir, host, repo, "--no-narrow")
			if runErr != nil {
				_ = os.RemoveAll(dir)
				return runErr
			}
			candidateReport := NewReport("candidate")
			candidateReport.generated(native)
			if candidateReport.Blocked() {
				r.Findings = append(r.Findings, candidateReport.Findings...)
				_ = os.RemoveAll(dir)
				return nil
			}
			candidate, err := readTree(dir, opts.Config)
			_ = os.RemoveAll(dir)
			if err != nil {
				return err
			}
			after, err := loadGraph(candidate, host)
			if err != nil {
				return err
			}
			newKey := strings.ToLower(ref.NWO) + "@" + target
			if err := validateScope(before, after, key, newKey); err != nil {
				return err
			}
			if err := validateTrustedCandidate(after, cfg.Trust); err != nil {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, To: target, Status: "blocked", Reason: err.Error(), Changes: graphDiff(before, after)})
				continue
			}
			if err := validateTransitivePolicies(before, after, newKey, cfg, opts); err != nil {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: ref.Ref, To: target, Status: "blocked", Reason: err.Error()})
				continue
			}
			ready, until, err := assess(before, after, state, observed, minimum, opts.Now)
			if err != nil {
				r.finding("source-identity-changed", "error", err.Error())
				return nil
			}
			mature, ageUntil, err := assessReleaseAge(ctx, before, after, age, opts.Now, source)
			if err != nil {
				return err
			}
			if ageUntil.After(until) {
				until = ageUntil
			}
			decision := Decision{Action: ref.Name, From: ref.Ref, To: target, Status: "eligible", Reason: "all changed dependencies satisfy policy", Changes: graphDiff(before, after)}
			if !ready || !mature {
				decision.Status = "pending"
				decision.Reason = "waiting for release-age policy"
				if !ready {
					decision.Reason = "waiting for observations of " + strings.Join(pendingObservations(before, after, state, minimum, opts.Now), ", ")
				}
				decision.Reason += " until " + until.Format(time.RFC3339)
				decision.EligibleAt = &until
				r.Decisions = append(r.Decisions, decision)
				continue
			}
			r.Decisions = append(r.Decisions, decision)
			if err := installTree(work, current, candidate); err != nil {
				return err
			}
			break
		}
	}
	if minimum > 0 && len(observed) > 0 {
		for key, at := range observed {
			state.Seen[key] = at
		}
		files := tree{}
		if err := saveObservations(files, state); err != nil {
			return err
		}
		return putFile(work, StatePath, files[StatePath])
	}
	return nil
}
