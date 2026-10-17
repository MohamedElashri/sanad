package sanad

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

type graph map[string]Dependency

func coverage(files tree) Coverage {
	c := Coverage{Uncovered: []string{}}
	var parsed lock.File
	if f, ok := files[lock.Path]; ok {
		parsed, _ = lock.Parse(f.Data)
	}
	for name := range files {
		if strings.HasPrefix(name, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) && !strings.Contains(strings.TrimPrefix(name, ".github/workflows/"), "/") {
			c.Total++
			if _, ok := parsed.Workflows[name]; ok {
				c.Locked++
			} else {
				c.Uncovered = append(c.Uncovered, name)
			}
		}
	}
	sort.Strings(c.Uncovered)
	return c
}

func loadGraph(files tree, home string) (graph, error) {
	out := graph{}
	f, exists := files[lock.Path]
	if !exists {
		return out, nil
	}
	parsed, err := lock.Parse(f.Data)
	if err != nil {
		return nil, err
	}
	// The native parser and producer can accept dangling pin references, while
	// GitHub rejects them before scheduling a job. Check the graph explicitly.
	for workflow, keys := range parsed.Workflows {
		selected := make(map[string]bool, len(keys))
		for _, key := range keys {
			if _, ok := parsed.Dependencies[key]; !ok {
				return nil, fmt.Errorf("%s: %s references missing dependency %s", lock.Path, workflow, key)
			}
			selected[key] = true
		}
		for _, key := range keys {
			for _, child := range parsed.Dependencies[key].Uses {
				if !selected[child] {
					return nil, fmt.Errorf("%s: %s omits transitive dependency %s used by %s", lock.Path, workflow, child, key)
				}
			}
		}
	}
	for key, a := range parsed.Dependencies {
		for _, child := range a.Uses {
			if _, ok := parsed.Dependencies[child]; !ok {
				return nil, fmt.Errorf("%s: %s uses missing dependency %s", lock.Path, key, child)
			}
		}
		host := a.Hostname
		if host == "" {
			host = home
		}
		if home == "github.com" && host != "github.com" {
			return nil, fmt.Errorf("non-github.com dependency on github.com")
		}
		if home != "github.com" && host != "github.com" && host != home {
			return nil, fmt.Errorf("dependency is bound to a different tenant")
		}
		uses := append([]string{}, a.Uses...)
		sort.Strings(uses)
		d := Dependency{Key: key, Host: host, Commit: a.Commit, OwnerID: a.OwnerID, RepoID: a.RepoID, Uses: uses, Workflows: []string{}}
		for path, keys := range parsed.Workflows {
			for _, k := range keys {
				if k == key {
					d.Workflows = append(d.Workflows, path)
					break
				}
			}
		}
		sort.Strings(d.Workflows)
		out[key] = d
	}
	return out, nil
}

func (g graph) list() []Dependency {
	out := make([]Dependency, 0, len(g))
	for _, d := range g {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func graphDiff(before, after graph) []Change {
	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	names := []string{}
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	out := []Change{}
	for _, key := range names {
		a, aok := before[key]
		b, bok := after[key]
		if aok && bok && reflect.DeepEqual(a, b) {
			continue
		}
		c := Change{Key: key}
		if aok {
			c.Before = &a
		}
		if bok {
			c.After = &b
		}
		out = append(out, c)
	}
	return out
}

func observationKey(d Dependency) string {
	return fmt.Sprintf("%s|%d|%d|%s|%s", d.Host, d.OwnerID, d.RepoID, d.Key, d.Commit)
}

type observations struct {
	Version int                  `json:"version"`
	Seen    map[string]time.Time `json:"seen"`
}

func loadObservations(files tree, now time.Time) (observations, error) {
	s := observations{Version: 1, Seen: map[string]time.Time{}}
	if f, ok := files[StatePath]; ok {
		decoder := json.NewDecoder(strings.NewReader(string(f.Data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&s); err != nil {
			return s, fmt.Errorf("%s: %w", StatePath, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return s, fmt.Errorf("invalid trailing observation data")
		}
		if s.Version != 1 || s.Seen == nil {
			return s, fmt.Errorf("unsupported observation state")
		}
		for key, at := range s.Seen {
			if len(strings.Split(key, "|")) != 5 || at.IsZero() || at.After(now) {
				return s, fmt.Errorf("invalid observation %q", key)
			}
		}
	}
	return s, nil
}

func saveObservations(files tree, s observations) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	files[StatePath] = file{Data: b, Mode: 0o644}
	return nil
}

// assess evaluates every introduced commit, not only the action the user chose.
// observed accumulates candidates separately; state contains trusted prior runs.
func assess(before, after graph, state observations, observed map[string]time.Time, minimum time.Duration, now time.Time) (bool, time.Time, error) {
	ready := true
	var eligible time.Time
	for _, change := range graphDiff(before, after) {
		if change.After == nil {
			continue
		}
		d := *change.After
		nwo, _, _ := strings.Cut(d.Key, "@")
		for _, old := range before {
			oldNWO, _, _ := strings.Cut(old.Key, "@")
			if nwo == oldNWO && (d.Host != old.Host || d.OwnerID != old.OwnerID || d.RepoID != old.RepoID) {
				return false, eligible, fmt.Errorf("source identity changed for %s", d.Key)
			}
		}
		if change.Before != nil && change.Before.Commit == d.Commit && change.Before.Host == d.Host && change.Before.RepoID == d.RepoID && change.Before.OwnerID == d.OwnerID {
			continue
		}
		if minimum == 0 {
			continue
		}
		key := observationKey(d)
		at, ok := state.Seen[key]
		if !ok {
			at = now
			observed[key] = now
		}
		until := at.Add(minimum)
		if until.After(now) {
			ready = false
			if until.After(eligible) {
				eligible = until
			}
		}
	}
	return ready, eligible, nil
}

func pendingObservations(before, after graph, state observations, minimum time.Duration, now time.Time) []string {
	keys := []string{}
	for _, c := range graphDiff(before, after) {
		if c.After == nil || c.Before != nil && c.Before.Commit == c.After.Commit {
			continue
		}
		at, ok := state.Seen[observationKey(*c.After)]
		if !ok || at.Add(minimum).After(now) {
			keys = append(keys, c.Key)
		}
	}
	return keys
}
