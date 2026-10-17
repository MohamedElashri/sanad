package sanad

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

type Options struct {
	Root, Repo, Config, Base                  string
	Write, Offline, ConfirmNative, Automation bool
	Action, To, Level, Constraint, ActionRef  string
	Backend                                   Backend
	Releases                                  ReleaseSource
	Now                                       time.Time
}

func Run(ctx context.Context, command string, opts Options) (report Report, err error) {
	report = NewReport(command)
	defer func() {
		if err != nil {
			report.finding("operation-failed", "error", err.Error())
		}
		report.Success = err == nil && !report.Blocked()
	}()
	root, err := Root(ctx, opts.Root)
	if err != nil {
		return report, err
	}
	opts.Root = root
	if opts.Config == "" {
		opts.Config = ConfigPath
	}
	if _, err := safePath(root, opts.Config); err != nil {
		return report, err
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if command == "recover" {
		if !opts.Write {
			report.finding("recovery", "warning", "run sanad recover --write to restore files from the interrupted transaction")
			return report, nil
		}
		return report, recoverTransaction(root)
	}
	if _, err := os.Stat(filepath.Join(root, transactionPath)); err == nil {
		report.finding("interrupted-transaction", "error", "run sanad recover --write before continuing")
		return report, nil
	}
	if _, err := os.Stat(filepath.Join(root, ".github/sanad.lock.json")); err == nil {
		report.finding("legacy-state", "error", "remove .github/sanad.lock.json and old .sanad.toml; restore desired uses refs and run sanad init")
		return report, nil
	}
	cfg, err := loadConfig(root, opts.Config)
	if err != nil {
		return report, err
	}
	if opts.ConfirmNative {
		cfg.Native.Enabled = true
	}
	if cfg.Native.Enabled {
		report.NativeSupport = "confirmed"
	}
	input, err := readTree(root, opts.Config)
	if err != nil {
		return report, err
	}
	refs, err := references(input)
	if err != nil {
		return report, err
	}
	for _, f := range input {
		if strings.Contains(string(f.Data), "# sanad:") {
			report.finding("legacy-comments", "error", "remove old # sanad: comments and set desired readable uses refs before native onboarding")
			return report, nil
		}
	}
	host, repo, identityErr := repository(ctx, root, opts.Repo)
	if identityErr != nil {
		if command == "status" || command == "doctor" || opts.Offline {
			host = "github.com"
			repo = "local/repository"
			report.finding("repository-identity", "warning", identityErr.Error())
		} else {
			return report, identityErr
		}
	}
	old, err := loadGraph(input, host)
	if err != nil {
		return report, err
	}
	report.Dependencies = old.list()
	report.Coverage = coverage(input)
	for _, ref := range refs {
		if ref.Workflow {
			report.Findings = append(report.Findings, Finding{Code: "external-workflow", Severity: "warning", File: ref.File, Message: ref.Name + ": dependencies inside the called workflow use its defining repository's lockfile"})
		}
	}
	if !cfg.Native.Enabled {
		report.finding("native-support-unknown", "warning", "native enforcement is not confirmed; verify GitHub eligibility, then use init --confirm-native --write")
	}
	if command == "status" || command == "doctor" {
		if _, ok := input[lock.Path]; !ok {
			report.finding("not-initialized", "warning", "run sanad init to preview native onboarding")
		}
		if path, e := BackendPath(); e != nil {
			report.finding("backend", "warning", e.Error())
		} else if e = verifyBackend(path); e != nil {
			report.finding("backend", "warning", e.Error()+"; run sanad setup")
		}
		state, e := loadObservations(input, opts.Now)
		if e != nil {
			return report, e
		}
		minimum, e := duration(cfg.Observation.MinimumAge)
		if e != nil {
			return report, e
		}
		if minimum > 0 {
			keys := make([]string, 0, len(state.Seen))
			for key := range state.Seen {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				parts := strings.Split(key, "|")
				installed := false
				for _, d := range old {
					if observationKey(d) == key {
						installed = true
						break
					}
				}
				if installed {
					continue
				}
				until := state.Seen[key].Add(minimum)
				status := "observed"
				reason := "observation matured; run sanad plan"
				if until.After(opts.Now) {
					status = "pending"
					reason = "waiting until " + until.Format(time.RFC3339)
				}
				report.Decisions = append(report.Decisions, Decision{Action: parts[3], Status: status, Reason: reason, EligibleAt: &until})
			}
		}
		if command == "doctor" && newAPI(host).token(ctx, host) == "" {
			report.finding("authentication", "warning", "no GitHub credentials found; run gh auth login --hostname "+host+" or set GH_TOKEN")
		}
		return report, nil
	}
	backend := opts.Backend
	if backend == nil {
		native, e := EnsureBackend(ctx, !opts.Offline)
		if e != nil {
			return report, e
		}
		backend = native
	}
	work, err := snapshot(input)
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(work)
	if command == "check" {
		args := []string{"--verify"}
		report.Verification = "online"
		if opts.Offline {
			args = []string{"--verify-local"}
			report.Verification = "offline"
		}
		native, e := backend.Run(ctx, work, host, repo, args...)
		if e != nil {
			return report, e
		}
		report.native(native)
		minimum, e := duration(cfg.Observation.MinimumAge)
		if e != nil {
			return report, e
		}
		if opts.Offline && (opts.Base != "" || minimum > 0) {
			report.finding("offline-policy", "error", "offline coverage cannot enforce adoption policy; run an online check with --base")
			return report, nil
		}
		if opts.Offline {
			report.finding("offline-only", "warning", "coverage checked locally; source identity and commit reachability were not verified")
		}
		if !opts.Offline && (opts.Base != "" || minimum > 0) {
			if e := checkBase(ctx, opts, input, old, host, &report); e != nil {
				return report, e
			}
		}
		return report, nil
	}
	if command != "init" && command != "sync" && command != "plan" && command != "update" {
		return report, fmt.Errorf("unsupported command %s", command)
	}
	if opts.Write && !cfg.Native.Enabled {
		report.finding("native-support-required", "error", "verify native lockfile support for this repository, then run init --confirm-native --write")
		return report, nil
	}
	if opts.ConfirmNative {
		var b strings.Builder
		if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
			return report, err
		}
		inputConfig := file{Data: []byte(b.String()), Mode: 0o644}
		if err := putFile(work, opts.Config, inputConfig); err != nil {
			return report, err
		}
	}
	if opts.Automation {
		if command != "init" {
			return report, fmt.Errorf("--automation is only valid for init")
		}
		if err := automation(work, opts.ActionRef); err != nil {
			return report, err
		}
	}
	if command == "plan" || command == "update" {
		if _, ok := input[lock.Path]; !ok {
			return report, fmt.Errorf("run sanad init before planning updates")
		}
		verified, e := backend.Run(ctx, work, host, repo, "--verify")
		if e != nil {
			return report, e
		}
		report.native(verified)
		report.Verification = "online"
		if report.Blocked() {
			return report, nil
		}
		if e := planUpdates(ctx, work, host, repo, cfg, opts, backend, &report); e != nil {
			return report, e
		}
	} else {
		native, e := backend.Run(ctx, work, host, repo)
		if e != nil {
			return report, e
		}
		report.generated(native)
		if report.Blocked() {
			return report, nil
		}
		candidate, e := readTree(work, opts.Config)
		if e != nil {
			return report, e
		}
		next, e := loadGraph(candidate, host)
		if e != nil {
			return report, e
		}
		// init accepts a baseline only when no native lockfile exists yet.
		_, alreadyLocked := input[lock.Path]
		if command == "sync" || alreadyLocked {
			state, e := loadObservations(input, opts.Now)
			if e != nil {
				return report, e
			}
			observed := map[string]time.Time{}
			minimum, e := duration(cfg.Observation.MinimumAge)
			if e != nil {
				return report, e
			}
			ready, until, e := assess(old, next, state, observed, minimum, opts.Now)
			if e != nil {
				report.finding("source-identity-changed", "error", e.Error())
				return report, nil
			}
			api := opts.Releases
			if api == nil {
				api = newAPI(host)
			}
			age, e := releaseCooldown(root)
			if e != nil {
				return report, e
			}
			mature, ageUntil, e := assessReleaseAge(ctx, old, next, age, opts.Now, api)
			if e != nil {
				return report, e
			}
			if !ready || !mature {
				if ageUntil.After(until) {
					until = ageUntil
				}
				report.Decisions = append(report.Decisions, Decision{Action: "workflow changes", Status: "pending", Reason: "new dependencies have not satisfied adoption policy", EligibleAt: &until})
				pending := copyTree(input)
				if minimum > 0 {
					for key, at := range observed {
						state.Seen[key] = at
					}
					if err := saveObservations(pending, state); err != nil {
						return report, err
					}
				}
				// Only persist observations. The existing workflow may still need sync.
				return finish(root, opts.Config, input, pending, opts.Write, &report)
			}
		} else if cfg.Observation.MinimumAge != "" {
			state := observations{Version: 1, Seen: map[string]time.Time{}}
			for _, d := range next {
				state.Seen[observationKey(d)] = opts.Now
			}
			if err := saveObservations(candidate, state); err != nil {
				return report, err
			}
			if err := putFile(work, StatePath, candidate[StatePath]); err != nil {
				return report, err
			}
		}
	}
	final, e := readTree(work, opts.Config)
	if e != nil {
		return report, e
	}
	finalGraph, e := loadGraph(final, host)
	if e != nil {
		return report, e
	}
	verified, e := backend.Run(ctx, work, host, repo, "--verify")
	if e != nil {
		return report, e
	}
	report.native(verified)
	report.Verification = "online"
	if report.Blocked() {
		return report, nil
	}
	report.Dependencies = finalGraph.list()
	report.Coverage = coverage(final)
	report.Changes = graphDiff(old, finalGraph)
	if err := ctx.Err(); err != nil {
		return report, err
	}
	return finish(root, opts.Config, input, final, opts.Write, &report)
}

func finish(root, config string, before, after tree, write bool, r *Report) (Report, error) {
	for _, name := range changedFiles(before, after) {
		r.Files = append(r.Files, FileChange{Path: name, Diff: fileDiff(name, before[name], after[name])})
	}
	if write {
		if err := applyTree(root, config, before, after); err != nil {
			return *r, err
		}
		r.Changed = len(r.Files) > 0
	}
	return *r, nil
}

func baseFiles(ctx context.Context, root, base, config string) (tree, error) {
	if base == "" || strings.HasPrefix(base, "-") || strings.ContainsAny(base, "\n\r\x00") {
		return nil, fmt.Errorf("a trusted --base revision is required")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolve trusted base: %w", err)
	}
	hash := strings.TrimSpace(string(b))
	out := tree{}
	for _, name := range []string{lock.Path, StatePath, config, ".github/dependabot.yml", ".github/dependabot.yaml"} {
		cmd = exec.CommandContext(ctx, "git", "-C", root, "ls-tree", "--name-only", hash, "--", name)
		b, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(b)) == "" {
			continue
		}
		cmd = exec.CommandContext(ctx, "git", "-C", root, "show", hash+":"+name)
		b, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		out[name] = file{Data: b, Mode: 0o644}
	}
	return out, nil
}

func checkBase(ctx context.Context, opts Options, current tree, installed graph, host string, r *Report) error {
	base, err := baseFiles(ctx, opts.Root, opts.Base, opts.Config)
	if err != nil {
		return err
	}
	old, err := loadGraph(base, host)
	if err != nil {
		return err
	}
	r.Changes = graphDiff(old, installed)
	if _, exists := base[lock.Path]; !exists {
		r.finding("initial-baseline", "warning", "the trusted base has no native lockfile; review and accept these initial pins; adoption waiting periods apply to subsequent changes")
		return nil
	}
	state, err := loadObservations(base, opts.Now)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(opts.Root, opts.Config)
	if err != nil {
		return err
	}
	minimum, err := duration(cfg.Observation.MinimumAge)
	if err != nil {
		return err
	}
	// Apply at least the trusted base's waiting period, even if the PR lowers it.
	if f, ok := base[opts.Config]; ok {
		var previous Config
		if _, err := toml.Decode(string(f.Data), &previous); err != nil {
			return err
		}
		age, err := duration(previous.Observation.MinimumAge)
		if err != nil {
			return err
		}
		if age > minimum {
			minimum = age
		}
	}
	ready, until, err := assess(old, installed, state, map[string]time.Time{}, minimum, opts.Now)
	if err != nil {
		r.finding("source-identity-changed", "error", err.Error())
		return nil
	}
	if !ready {
		r.finding("observation-pending", "error", "changed dependencies lack trusted observations mature by "+until.Format(time.RFC3339))
	}
	if f, ok := current[opts.Config]; ok && !sameFile(f, true, base[opts.Config], base[opts.Config].Data != nil) {
		r.finding("policy-changed", "warning", "repository policy changed; review the configuration diff")
	}
	api := opts.Releases
	if api == nil {
		api = newAPI(host)
	}
	age, err := releaseCooldown(opts.Root)
	if err != nil {
		return err
	}
	// Parse base Dependabot policy through the same strict loader.
	tmp, err := snapshot(base)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	previousAge, err := releaseCooldown(tmp)
	if err != nil {
		return err
	}
	if previousAge > age {
		age = previousAge
	}
	mature, _, err := assessReleaseAge(ctx, old, installed, age, opts.Now, api)
	if err != nil {
		return err
	}
	if !mature {
		r.finding("release-cooldown", "error", "changed dependencies have not satisfied release-age policy")
	}
	return nil
}
