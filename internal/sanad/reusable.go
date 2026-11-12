package sanad

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

const WorkflowStatePath = ".github/sanad/workflows.json"

type WorkflowPin struct {
	Name         string `json:"name"`
	Ref          string `json:"ref"`
	Commit       string `json:"commit"`
	Host         string `json:"host"`
	OwnerID      int64  `json:"owner_id"`
	RepoID       int64  `json:"repo_id"`
	CalleeLocked bool   `json:"callee_locked"`
}

type WorkflowSource interface {
	// An empty commit resolves a new ref. Otherwise verify that exact pinned commit.
	Workflow(context.Context, string, string, string, string) (WorkflowPin, error)
}

type workflowState struct {
	Version int                    `json:"version"`
	Pins    map[string]WorkflowPin `json:"pins"`
}

func workflowKey(name, sha string) string {
	return canonicalActionName(name) + "@" + strings.ToLower(sha)
}
func loadWorkflowState(files tree) (workflowState, error) {
	state := workflowState{Version: 1, Pins: map[string]WorkflowPin{}}
	if f, ok := files[WorkflowStatePath]; ok {
		dec := json.NewDecoder(strings.NewReader(string(f.Data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&state); err != nil {
			return state, err
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return state, fmt.Errorf("invalid trailing reusable workflow state")
		}
		if state.Version != 1 || state.Pins == nil {
			return state, fmt.Errorf("unsupported reusable workflow state")
		}
		for key, p := range state.Pins {
			if !fullSHA.MatchString(p.Commit) || p.OwnerID <= 0 || p.RepoID <= 0 || key != workflowKey(p.Name, p.Commit) || lock.ParseReusableWorkflowRef(p.Name+"@"+p.Ref) == nil || p.Host != "github.com" && !validTenant(p.Host) {
				return state, fmt.Errorf("invalid reusable workflow pin %s", key)
			}
		}
	}
	return state, nil
}
func saveWorkflowState(files tree, state workflowState) error {
	if len(state.Pins) == 0 {
		delete(files, WorkflowStatePath)
		return nil
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	files[WorkflowStatePath] = file{Data: append(b, '\n'), Mode: 0o644}
	return nil
}
func workflowDependency(p WorkflowPin) Dependency {
	return Dependency{Key: p.Name + "@" + p.Ref, Host: p.Host, Commit: "sha1-" + p.Commit, OwnerID: p.OwnerID, RepoID: p.RepoID, Uses: []string{}, Workflows: []string{}}
}
func loadPolicyGraph(files tree, host string) (graph, error) {
	g, err := loadGraph(files, host)
	if err != nil {
		return nil, err
	}
	state, err := loadWorkflowState(files)
	if err != nil {
		return nil, err
	}
	refs, err := references(files)
	if err != nil {
		return nil, err
	}
	for _, p := range state.Pins {
		if host == "github.com" && p.Host != host || host != "github.com" && p.Host != "github.com" && p.Host != host {
			return nil, fmt.Errorf("reusable workflow is bound to a different host")
		}
		d := workflowDependency(p)
		g[d.Key] = d
	}
	for _, ref := range refs {
		if !ref.Workflow {
			continue
		}
		if p, ok := state.Pins[workflowKey(ref.Name, ref.Ref)]; ok {
			if host == "github.com" && p.Host != host || host != "github.com" && p.Host != "github.com" && p.Host != host {
				return nil, fmt.Errorf("reusable workflow is bound to a different host")
			}
			d := workflowDependency(p)
			if existing, ok := g[d.Key]; ok {
				d = existing
			}
			d.Workflows = append(d.Workflows, ref.File)
			sort.Strings(d.Workflows)
			d.Workflows = unique(d.Workflows)
			g[d.Key] = d
		}
	}
	return g, nil
}
func workflowSource(opts Options, host string) WorkflowSource {
	if opts.Workflows != nil {
		return opts.Workflows
	}
	return newAPI(host)
}
func reconcileWorkflows(ctx context.Context, files tree, host string, opts Options, verify bool, r *Report) error {
	state, err := loadWorkflowState(files)
	if err != nil {
		return err
	}
	refs, err := references(files)
	if err != nil {
		return err
	}
	next := workflowState{Version: 1, Pins: map[string]WorkflowPin{}}
	source := workflowSource(opts, host)
	for _, ref := range refs {
		if !ref.Workflow {
			continue
		}
		sha, hint := "", ref.Ref
		old, known := state.Pins[workflowKey(ref.Name, ref.Ref)]
		if fullSHA.MatchString(ref.Ref) {
			sha = strings.ToLower(ref.Ref)
			if known {
				hint = old.Ref
			}
		}
		if verify && (sha == "" || !known) {
			r.Findings = append(r.Findings, Finding{Code: "workflow-untracked", Severity: "error", File: ref.File, Message: ref.Name + ": run sanad sync to record and pin this reusable workflow"})
			continue
		}
		if opts.Offline {
			next.Pins[workflowKey(ref.Name, sha)] = old
			continue
		}
		pin, err := source.Workflow(ctx, host, ref.Name, hint, sha)
		if err != nil {
			return err
		}
		if !fullSHA.MatchString(pin.Commit) || pin.OwnerID <= 0 || pin.RepoID <= 0 || canonicalActionName(pin.Name) != canonicalActionName(ref.Name) || pin.Host != host && pin.Host != "github.com" {
			return fmt.Errorf("invalid workflow resolution for %s", ref.Name)
		}
		if sha != "" && !strings.EqualFold(pin.Commit, sha) {
			return fmt.Errorf("workflow resolver changed frozen pin %s", ref.Name)
		}
		if known && (pin.OwnerID != old.OwnerID || pin.RepoID != old.RepoID || pin.Host != old.Host) {
			return fmt.Errorf("source identity changed for %s", ref.Name)
		}
		if previous, ok := next.Pins[workflowKey(ref.Name, pin.Commit)]; ok && previous.Ref != pin.Ref {
			return fmt.Errorf("%s resolves different tracked refs to the same SHA; select a single version for these calls", ref.Name)
		}
		next.Pins[workflowKey(ref.Name, pin.Commit)] = pin
		if !pin.CalleeLocked {
			r.Findings = append(r.Findings, Finding{Code: "callee-lock-missing", Severity: "warning", File: ref.File, Message: ref.Name + ": the called workflow has no structural lockfile coverage at its pinned commit; its dependencies are outside the caller's lockfile"})
		}
	}
	if verify {
		return nil
	}
	// Re-read positions after every edit (flow YAML may have multiple uses per line).
	for _, ref := range refs {
		if !ref.Workflow {
			continue
		}
		hint := ref.Ref
		if p, ok := state.Pins[workflowKey(ref.Name, ref.Ref)]; ok {
			hint = p.Ref
		}
		for _, pin := range next.Pins {
			if canonicalActionName(pin.Name) == canonicalActionName(ref.Name) && pin.Ref == hint {
				current, err := references(files)
				if err != nil {
					return err
				}
				selected := []reference{}
				for _, c := range current {
					if c.Workflow && c.Name == ref.Name && c.Ref == ref.Ref {
						selected = append(selected, c)
					}
				}
				if err := rewriteRefs(files, selected, pin.Commit); err != nil {
					return err
				}
				break
			}
		}
	}
	return saveWorkflowState(files, next)
}

func (a *githubAPI) content(ctx context.Context, host, nwo, path, sha string) ([]byte, bool, error) {
	var data struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Type     string `json:"type"`
	}
	code, err := a.get(ctx, host, "/repos/"+nwo+"/contents/"+path+"?ref="+url.QueryEscape(sha), &data)
	if err != nil || code == http.StatusNotFound {
		return nil, false, err
	}
	if data.Encoding != "base64" || data.Type != "file" {
		return nil, false, fmt.Errorf("cannot inspect %s at %s", path, sha)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(data.Content, "\n", ""))
	return b, true, err
}
func (a *githubAPI) Workflow(ctx context.Context, host, name, ref, sha string) (WorkflowPin, error) {
	parts := strings.SplitN(name, "/", 3)
	if len(parts) != 3 || lock.ParseReusableWorkflowRef(name+"@"+ref) == nil {
		return WorkflowPin{}, fmt.Errorf("invalid reusable workflow %s", name)
	}
	nwo := strings.Join(parts[:2], "/")
	var repo struct {
		ID    int64 `json:"id"`
		Owner struct {
			ID int64 `json:"id"`
		} `json:"owner"`
		Default string `json:"default_branch"`
	}
	code, err := a.get(ctx, host, "/repos/"+nwo, &repo)
	if err != nil {
		return WorkflowPin{}, err
	}
	if code == 404 && host != "github.com" {
		return a.Workflow(ctx, "github.com", name, ref, sha)
	}
	if code == 404 || repo.ID <= 0 || repo.Owner.ID <= 0 || repo.Default == "" {
		return WorkflowPin{}, fmt.Errorf("workflow source unavailable: %s", nwo)
	}
	var resolved struct {
		SHA string `json:"sha"`
	}
	code, err = a.get(ctx, host, "/repos/"+nwo+"/commits/"+url.PathEscape(ref), &resolved)
	if err != nil {
		return WorkflowPin{}, err
	}
	if code == 404 || !fullSHA.MatchString(resolved.SHA) {
		return WorkflowPin{}, fmt.Errorf("workflow ref unavailable: %s@%s", name, ref)
	}
	if sha == "" {
		sha = strings.ToLower(resolved.SHA)
	}
	// Raw commit lookup also returns fork-only objects. Establish ancestry from a
	// repository-owned ref; for an unlabelled SHA use the default branch.
	anchor := ref
	if fullSHA.MatchString(ref) {
		anchor = repo.Default
	}
	var comparison struct {
		Status string `json:"status"`
	}
	code, err = a.get(ctx, host, "/repos/"+nwo+"/compare/"+url.PathEscape(sha)+"..."+url.PathEscape(anchor), &comparison)
	if err != nil {
		return WorkflowPin{}, err
	}
	if code == 404 || (comparison.Status != "ahead" && comparison.Status != "identical") {
		return WorkflowPin{}, fmt.Errorf("workflow commit is not reachable from %s in %s", anchor, nwo)
	}
	if v, ok := lock.ParseSemVer(ref); ok && v.IsFull() && !strings.EqualFold(resolved.SHA, sha) {
		return WorkflowPin{}, fmt.Errorf("workflow tag moved: %s@%s", name, ref)
	}
	data, exists, err := a.content(ctx, host, nwo, parts[2], sha)
	if err != nil {
		return WorkflowPin{}, err
	}
	if !exists {
		return WorkflowPin{}, fmt.Errorf("workflow missing at pinned commit: %s", name)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return WorkflowPin{}, err
	}
	if len(doc.Content) == 0 {
		return WorkflowPin{}, fmt.Errorf("empty reusable workflow")
	}
	on := unalias(mapping(doc.Content[0], "on"))
	callable := false
	if on != nil {
		if on.Kind == yaml.ScalarNode {
			callable = on.Value == "workflow_call"
		}
		if on.Kind == yaml.MappingNode {
			callable = mapping(on, "workflow_call") != nil
		}
		if on.Kind == yaml.SequenceNode {
			for _, n := range on.Content {
				if n.Value == "workflow_call" {
					callable = true
				}
			}
		}
	}
	if !callable {
		return WorkflowPin{}, fmt.Errorf("%s does not declare workflow_call", name)
	}
	pin := WorkflowPin{Name: canonicalActionName(name), Ref: ref, Commit: sha, Host: host, OwnerID: repo.Owner.ID, RepoID: repo.ID}
	data, exists, err = a.content(ctx, host, nwo, lock.Path, sha)
	if err != nil {
		return pin, err
	}
	if exists {
		_, e := loadGraph(tree{lock.Path: {Data: data}}, host)
		if e != nil {
			return pin, e
		}
		parsed, e := lock.Parse(data)
		if e != nil {
			return pin, e
		}
		_, pin.CalleeLocked = parsed.Workflows[parts[2]]
	}
	return pin, nil
}

func hasWorkflowSelection(refs []reference, selected string) bool {
	for _, ref := range refs {
		if ref.Workflow && matchesAction(selected, ref) {
			return true
		}
	}
	return false
}
func planWorkflows(ctx context.Context, work, host string, cfg Config, opts Options, r *Report) error {
	files, err := readTree(work, opts.Config)
	if err != nil {
		return err
	}
	refs, err := references(files)
	if err != nil {
		return err
	}
	state, err := loadWorkflowState(files)
	if err != nil {
		return err
	}
	observations, err := loadObservations(files, opts.Now)
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
	releases := opts.Releases
	if releases == nil {
		releases = newAPI(host)
	}
	source := workflowSource(opts, host)
	observed := map[string]time.Time{}
	visited := map[string]bool{}
	for _, ref := range refs {
		if !ref.Workflow || !matchesAction(opts.Action, ref) {
			continue
		}
		key := workflowKey(ref.Name, ref.Ref)
		if visited[key] {
			continue
		}
		visited[key] = true
		old, ok := state.Pins[key]
		if !ok {
			return fmt.Errorf("run sanad sync before updating reusable workflow %s", ref.Name)
		}
		var list []Release
		if opts.To == "" {
			v, ok := lock.ParseSemVer(old.Ref)
			if !ok || !v.IsFull() || !v.IsStable() {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: old.Ref, Status: "frozen", Reason: "choose a full stable version with --action and --to to begin version tracking"})
				continue
			}
			list, err = releases.List(ctx, old.Host, ref.NWO)
			if err != nil {
				return err
			}
		}
		candidates := targets(list, old.Ref, effectivePolicy(cfg, opts, ref.Name, ref.NWO), opts.To)
		if len(candidates) == 0 {
			r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: old.Ref, Status: "unchanged", Reason: "no newer stable release matches policy"})
		}
		for _, target := range candidates {
			if target == old.Ref {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: old.Ref, To: target, Status: "unchanged", Reason: "already tracking this ref"})
				continue
			}
			pin, err := source.Workflow(ctx, old.Host, ref.Name, target, "")
			if err != nil {
				return err
			}
			if pin.Host != old.Host || pin.OwnerID != old.OwnerID || pin.RepoID != old.RepoID {
				return fmt.Errorf("source identity changed for %s", ref.Name)
			}
			if !fullSHA.MatchString(pin.Commit) || canonicalActionName(pin.Name) != canonicalActionName(ref.Name) || pin.Ref != target {
				return fmt.Errorf("invalid workflow resolution for %s", ref.Name)
			}
			current, err := readTree(work, opts.Config)
			if err != nil {
				return err
			}
			before, err := loadPolicyGraph(current, host)
			if err != nil {
				return err
			}
			edited := copyTree(current)
			currentRefs, err := references(edited)
			if err != nil {
				return err
			}
			selected := []reference{}
			for _, c := range currentRefs {
				if c.Workflow && workflowKey(c.Name, c.Ref) == key {
					selected = append(selected, c)
				}
			}
			if err := rewriteRefs(edited, selected, pin.Commit); err != nil {
				return err
			}
			nextState, err := loadWorkflowState(edited)
			if err != nil {
				return err
			}
			delete(nextState.Pins, key)
			nextState.Pins[workflowKey(pin.Name, pin.Commit)] = pin
			if err := saveWorkflowState(edited, nextState); err != nil {
				return err
			}
			after, err := loadPolicyGraph(edited, host)
			if err != nil {
				return err
			}
			if err := validateTrustedCandidate(after, cfg.Trust); err != nil {
				r.Decisions = append(r.Decisions, Decision{Action: ref.Name, From: old.Ref, To: target, Status: "blocked", Reason: err.Error(), Changes: graphDiff(before, after)})
				continue
			}
			ready, until, err := assess(before, after, observations, observed, minimum, opts.Now)
			if err != nil {
				return err
			}
			mature, ageUntil, err := assessReleaseAge(ctx, before, after, age, opts.Now, releases)
			if err != nil {
				return err
			}
			if ageUntil.After(until) {
				until = ageUntil
			}
			decision := Decision{Action: ref.Name, From: old.Ref, To: target, Status: "eligible", Reason: "reusable workflow source and adoption policy verified", Changes: graphDiff(before, after)}
			if !ready || !mature {
				decision.Status = "pending"
				decision.Reason = "waiting for reusable workflow adoption policy until " + until.Format("2006-01-02T15:04:05Z07:00")
				decision.EligibleAt = &until
			}
			r.Decisions = append(r.Decisions, decision)
			if !ready || !mature {
				continue
			}
			if !pin.CalleeLocked {
				r.Findings = append(r.Findings, Finding{Code: "callee-lock-missing", Severity: "warning", File: ref.File, Message: ref.Name + ": callee action dependencies have no structural lockfile coverage at the selected commit"})
			}
			if err := installTree(work, current, edited); err != nil {
				return err
			}
			break
		}
	}
	if minimum > 0 && len(observed) > 0 {
		for k, at := range observed {
			observations.Seen[k] = at
		}
		staged := tree{}
		if err := saveObservations(staged, observations); err != nil {
			return err
		}
		return putFile(work, StatePath, staged[StatePath])
	}
	return nil
}
