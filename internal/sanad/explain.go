package sanad

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Explanation struct {
	Dependency Dependency `json:"dependency"`
	Paths      [][]string `json:"paths"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason"`
	SourceURL  string     `json:"source_url"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
	EligibleAt *time.Time `json:"eligible_at,omitempty"`
}

func repositoryName(key string) string {
	name, _, _ := strings.Cut(key, "@")
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		name = strings.Join(parts[:2], "/")
	}
	return strings.ToLower(name)
}
func sourceURL(d Dependency) string {
	return "https://" + d.Host + "/" + repositoryName(d.Key) + "/tree/" + strings.TrimPrefix(d.Commit, "sha1-")
}
func reviewChange(c *Change, before graph) {
	if c.After == nil {
		return
	}
	var previous *Dependency
	if c.Before != nil {
		previous = c.Before
	} else {
		candidates := []Dependency{}
		for _, old := range before {
			if repositoryName(old.Key) == repositoryName(c.After.Key) && old.Host == c.After.Host {
				candidates = append(candidates, old)
			}
		}
		if len(candidates) == 1 {
			previous = &candidates[0]
		}
		c.NewSource = len(candidates) == 0
	}
	if previous == nil {
		return
	}
	c.IdentityChanged = previous.Host != c.After.Host || previous.OwnerID != c.After.OwnerID || previous.RepoID != c.After.RepoID
	if !c.IdentityChanged && previous.Commit != c.After.Commit {
		c.CompareURL = "https://" + c.After.Host + "/" + repositoryName(c.After.Key) + "/compare/" + strings.TrimPrefix(previous.Commit, "sha1-") + "..." + strings.TrimPrefix(c.After.Commit, "sha1-")
	}
}
func explanationMatches(selected string, d Dependency) bool {
	if selected == "" {
		return true
	}
	if canonicalActionName(selected) == canonicalActionName(d.Key) {
		return true
	}
	name, _, _ := strings.Cut(d.Key, "@")
	return canonicalActionName(selected) == canonicalActionName(name) || canonicalActionName(selected) == repositoryName(d.Key)
}
func metadataEdges(files tree, name string) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(files[name].Data, &doc); err != nil {
		return nil, err
	}
	targets := map[string]bool{}
	seen := map[*yaml.Node]bool{}
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		n = unalias(n)
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		if n.Kind == yaml.MappingNode {
			if u := unalias(mapping(n, "uses")); u != nil {
				path := ""
				if strings.HasPrefix(u.Value, "$/") {
					path = strings.TrimPrefix(u.Value, "$/")
				}
				if strings.HasPrefix(u.Value, "./") {
					path = strings.TrimPrefix(u.Value, "./")
				}
				if path != "" && !strings.Contains(path, "..") && !strings.Contains(path, "@") {
					if _, ok := files[path]; ok {
						targets[path] = true
					} else {
						path = strings.TrimSuffix(path, "/")
						if path == "." {
							path = ""
						}
						if path != "" {
							path += "/"
						}
						for _, suffix := range []string{"action.yml", "action.yaml"} {
							if _, ok := files[path+suffix]; ok {
								targets[path+suffix] = true
								break
							}
						}
					}
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&doc)
	out := []string{}
	for t := range targets {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}
func metadataChains(files tree) (map[string][][]string, error) {
	chains := map[string][][]string{}
	edges := map[string][]string{}
	active, err := activeMetadata(files)
	if err != nil {
		return nil, err
	}
	for name := range active {
		list, err := metadataEdges(files, name)
		if err != nil {
			return nil, err
		}
		edges[name] = list
	}
	count := 0
	var visit func(string, []string) error
	visit = func(name string, path []string) error {
		for _, p := range path {
			if p == name {
				return nil
			}
		}
		if len(path) > 64 || count > 10000 {
			return fmt.Errorf("metadata graph has too many explanation paths")
		}
		count++
		path = append(append([]string{}, path...), name)
		chains[name] = append(chains[name], path)
		for _, child := range edges[name] {
			if err := visit(child, path); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range sortedTreeNames(files) {
		if strings.HasPrefix(name, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			if err := visit(name, nil); err != nil {
				return nil, err
			}
		}
	}
	return chains, nil
}
func sortedTreeNames(files tree) []string {
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func explain(files tree, g graph, cfg Config, opts Options, r *Report) error {
	chains, err := metadataChains(files)
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
	paths := map[string][][]string{}
	var visit func(string, []string) error
	count := 0
	visit = func(key string, path []string) error {
		for _, p := range path {
			if p == key {
				return nil
			}
		}
		if len(path) > 128 || count > 20000 {
			return fmt.Errorf("dependency graph has too many explanation paths")
		}
		count++
		path = append(append([]string{}, path...), key)
		paths[key] = append(paths[key], path)
		for _, child := range g[key].Uses {
			if err := visit(child, path); err != nil {
				return err
			}
		}
		return nil
	}
	for _, ref := range refs {
		key := strings.ToLower(ref.NWO) + "@" + ref.Ref
		if ref.Workflow {
			pin, ok := state.Pins[workflowKey(ref.Name, ref.Ref)]
			if !ok {
				continue
			}
			key = workflowDependency(pin).Key
		}
		if _, ok := g[key]; !ok {
			continue
		}
		for _, chain := range chains[ref.File] {
			if err := visit(key, chain); err != nil {
				return err
			}
		}
	}
	for _, d := range g.list() {
		if !explanationMatches(opts.Action, d) {
			continue
		}
		p := paths[d.Key]
		if p == nil {
			p = [][]string{}
		}
		e := Explanation{Dependency: d, Paths: p, SourceURL: sourceURL(d), Status: "installed", Reason: "recorded execution pin; run an online check to verify source trust"}
		if at, ok := observations.Seen[observationKey(d)]; ok {
			e.ObservedAt = &at
		}
		r.Explanations = append(r.Explanations, e)
	}
	keys := []string{}
	for key := range observations.Seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts, err := observationParts(key, false)
		if err != nil {
			return err
		}
		if installed, ok := g[parts[3]]; ok && observationKey(installed) == key {
			continue
		}
		d := Dependency{Host: parts[0], Key: parts[3], Commit: parts[4]}
		// Identities were validated while loading the observations.
		fmt.Sscan(parts[1], &d.OwnerID)
		fmt.Sscan(parts[2], &d.RepoID)
		if !explanationMatches(opts.Action, d) {
			continue
		}
		at := observations.Seen[key]
		until := at.Add(minimum)
		e := Explanation{Dependency: d, Paths: [][]string{}, SourceURL: sourceURL(d), Status: "observed", Reason: "stored observation matured; run sanad plan for current release and graph policy", ObservedAt: &at, EligibleAt: &until}
		if until.After(opts.Now) {
			e.Status = "pending"
			e.Reason = "first-seen waiting period has not elapsed"
		}
		r.Explanations = append(r.Explanations, e)
	}
	if opts.Action != "" && len(r.Explanations) == 0 {
		return fmt.Errorf("no recorded dependency or observation matches %s", opts.Action)
	}
	return nil
}
