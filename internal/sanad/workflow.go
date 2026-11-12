package sanad

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

type reference struct {
	File, Name, NWO, Ref string
	Line, Column         int
	Workflow             bool
}

func mapping(n *yaml.Node, key string) *yaml.Node {
	n = unalias(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func unalias(n *yaml.Node) *yaml.Node {
	seen := map[*yaml.Node]bool{}
	for n != nil && n.Kind == yaml.AliasNode {
		if seen[n] {
			return nil
		}
		seen[n] = true
		n = n.Alias
	}
	return n
}

func isWorkflow(name string) bool {
	return strings.HasPrefix(name, ".github/workflows/") && !strings.Contains(strings.TrimPrefix(name, ".github/workflows/"), "/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
}

type useSite struct {
	node     *yaml.Node
	reusable bool
}

func stepNodes(n *yaml.Node, parallel bool) ([]*yaml.Node, error) {
	n = unalias(n)
	if n == nil {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("steps must be a sequence")
	}
	out := []*yaml.Node{}
	for _, step := range n.Content {
		step = unalias(step)
		if step == nil || step.Kind != yaml.MappingNode {
			continue
		}
		out = append(out, step)
		if parallel {
			inner, err := stepNodes(mapping(step, "parallel"), false)
			if err != nil {
				return nil, err
			}
			out = append(out, inner...)
		}
	}
	return out, nil
}

// Match the producer's syntax: job steps can have one flat parallel group;
// composite action steps cannot. Input/env keys named uses are ordinary data.
func metadataUses(doc *yaml.Node, workflow bool) ([]useSite, error) {
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	out := []useSite{}
	addSteps := func(n *yaml.Node, parallel bool) error {
		steps, err := stepNodes(n, parallel)
		if err != nil {
			return err
		}
		for _, step := range steps {
			if u := unalias(mapping(step, "uses")); u != nil {
				out = append(out, useSite{u, false})
			}
		}
		return nil
	}
	if workflow {
		jobs := unalias(mapping(root, "jobs"))
		if jobs == nil {
			return out, nil
		}
		if jobs.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("jobs must be a mapping")
		}
		for i := 1; i < len(jobs.Content); i += 2 {
			job := jobs.Content[i]
			if u := unalias(mapping(job, "uses")); u != nil {
				out = append(out, useSite{u, true})
			}
			if err := addSteps(mapping(job, "steps"), true); err != nil {
				return nil, err
			}
		}
	} else {
		if err := addSteps(mapping(mapping(root, "runs"), "steps"), false); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func references(files tree) ([]reference, error) {
	active, err := activeMetadata(files)
	if err != nil {
		return nil, err
	}
	return referencesIn(files, active)
}

func referencesIn(files tree, active map[string]bool) ([]reference, error) {
	out := []reference{}
	seen := map[reference]bool{}
	for name, f := range files {
		if !active[name] {
			continue
		}
		workflow := isWorkflow(name)
		if !workflow && name != "action.yml" && name != "action.yaml" && !strings.HasSuffix(name, "/action.yml") && !strings.HasSuffix(name, "/action.yaml") {
			continue
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(f.Data, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if len(doc.Content) == 0 {
			continue
		}
		add := func(n *yaml.Node, reusable bool) error {
			n = unalias(n)
			if n == nil {
				return nil
			}
			if n.Kind != yaml.ScalarNode || n.Style == yaml.LiteralStyle || n.Style == yaml.FoldedStyle {
				return fmt.Errorf("%s: uses must be a single-line scalar", name)
			}
			ref := reference{File: name, Line: n.Line, Column: n.Column, Workflow: reusable}
			if a := lock.ParseActionRef(n.Value); a != nil {
				ref.Name = a.FullName()
				ref.NWO = a.NWO()
				ref.Ref = a.Ref
			} else if w := lock.ParseReusableWorkflowRef(n.Value); w != nil {
				ref.Name = w.FullName()
				ref.NWO = w.NWO()
				ref.Ref = w.Ref
			} else {
				return nil
			}
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
			return nil
		}
		sites, err := metadataUses(&doc, workflow)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, site := range sites {
			if err := add(site.node, site.reusable); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File == out[j].File {
			if out[i].Line == out[j].Line {
				return out[i].Column < out[j].Column
			}
			return out[i].Line < out[j].Line
		}
		return out[i].File < out[j].File
	})
	return out, nil
}

// Only inspect metadata reachable from this repository's actual workflows.
func activeMetadata(files tree) (map[string]bool, error) {
	return collectMetadata(files, nil)
}

func collectMetadata(files tree, load func(string) (bool, error)) (map[string]bool, error) {
	active := map[string]bool{}
	queue := []string{}
	var loadErr error
	enqueue := func(name string) bool {
		_, exists := files[name]
		if !exists && load != nil && loadErr == nil {
			exists, loadErr = load(name)
		}
		if exists {
			queue = append(queue, name)
		}
		return exists
	}
	for name := range files {
		if strings.HasPrefix(name, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) && !strings.Contains(strings.TrimPrefix(name, ".github/workflows/"), "/") {
			queue = append(queue, name)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if active[name] {
			continue
		}
		active[name] = true
		f, ok := files[name]
		if !ok {
			continue
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(f.Data, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		sites, err := metadataUses(&doc, isWorkflow(name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, site := range sites {
			value := site.node.Value
			path := ""
			if strings.HasPrefix(value, "$/") {
				path = strings.TrimPrefix(value, "$/")
			} else if strings.HasPrefix(value, "./") {
				path = strings.TrimPrefix(value, "./")
			} else {
				continue
			}
			if strings.Contains(path, "@") || strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
				continue
			}
			if site.reusable {
				enqueue(path)
				continue
			}
			path = strings.TrimSuffix(path, "/")
			if path == "." {
				path = ""
			}
			if path != "" {
				path += "/"
			}
			for _, suffix := range []string{"action.yml", "action.yaml"} {
				if enqueue(path + suffix) {
					break
				}
			}
		}
		if loadErr != nil {
			return nil, loadErr
		}
	}
	return active, nil
}

func rewriteRefs(files tree, refs []reference, target string) error {
	return rewriteUses(files, refs, func(r reference) string { return r.Name + "@" + target })
}

func rewriteUses(files tree, refs []reference, value func(reference) string) error {
	// Reverse source order makes multiple edits on one line stable.
	for i := len(refs) - 1; i >= 0; i-- {
		r := refs[i]
		f := files[r.File]
		lines := strings.Split(string(f.Data), "\n")
		if r.Line <= 0 || r.Line > len(lines) {
			return fmt.Errorf("invalid source location")
		}
		chars := []rune(string(f.Data))
		index := r.Column - 1
		for _, line := range lines[:r.Line-1] {
			index += len([]rune(line)) + 1
		}
		index = scalarStart(chars, index)
		if index < 0 || index >= len(chars) {
			return fmt.Errorf("invalid source column")
		}
		tail := string(chars[index:])
		old := r.Name + "@" + r.Ref
		if strings.HasPrefix(tail, "'") || strings.HasPrefix(tail, "\"") {
			index++
			tail = string(chars[index:])
		}
		if !strings.HasPrefix(tail, old) {
			return fmt.Errorf("%s:%d: cannot preserve uses formatting", r.File, r.Line)
		}
		f.Data = []byte(string(chars[:index]) + value(r) + strings.TrimPrefix(tail, old))
		files[r.File] = f
	}
	return nil
}

// YAML source positions include node properties such as &anchors and !!tags.
func scalarStart(chars []rune, index int) int {
	for index >= 0 && index < len(chars) {
		if unicode.IsSpace(chars[index]) {
			index++
			continue
		}
		if chars[index] != '&' && chars[index] != '!' {
			break
		}
		for index < len(chars) && !unicode.IsSpace(chars[index]) {
			index++
		}
	}
	return index
}

func legacyCommentFile(files tree) (string, error) {
	active, err := activeMetadata(files)
	if err != nil {
		return "", err
	}
	names := []string{}
	for name := range active {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var doc yaml.Node
		if err := yaml.Unmarshal(files[name].Data, &doc); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		var hasLegacy func(*yaml.Node) bool
		hasLegacy = func(n *yaml.Node) bool {
			for _, comment := range []string{n.HeadComment, n.LineComment, n.FootComment} {
				for _, line := range strings.Split(comment, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "# sanad:") {
						return true
					}
				}
			}
			for _, child := range n.Content {
				if hasLegacy(child) {
					return true
				}
			}
			return false
		}
		if hasLegacy(&doc) {
			return name, nil
		}
	}
	return "", nil
}
