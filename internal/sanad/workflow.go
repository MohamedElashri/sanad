package sanad

import (
	"fmt"
	"sort"
	"strings"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

type reference struct {
	File, Name, NWO, Ref string
	Line, Column         int
	Workflow             bool
}

func mapping(n *yaml.Node, key string) *yaml.Node {
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

func references(files tree) ([]reference, error) {
	out := []reference{}
	active, err := activeMetadata(files)
	if err != nil {
		return nil, err
	}
	for name, f := range files {
		if !active[name] {
			continue
		}
		workflow := strings.HasPrefix(name, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
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
		root := doc.Content[0]
		add := func(n *yaml.Node, reusable bool) error {
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
			out = append(out, ref)
			return nil
		}
		steps := func(n *yaml.Node) error {
			if n == nil {
				return nil
			}
			if n.Kind != yaml.SequenceNode {
				return fmt.Errorf("%s: steps must be a sequence", name)
			}
			for _, step := range n.Content {
				if err := add(mapping(step, "uses"), false); err != nil {
					return err
				}
			}
			return nil
		}
		if workflow {
			jobs := mapping(root, "jobs")
			if jobs != nil {
				for i := 1; i < len(jobs.Content); i += 2 {
					job := jobs.Content[i]
					if err := add(mapping(job, "uses"), true); err != nil {
						return nil, err
					}
					if err := steps(mapping(job, "steps")); err != nil {
						return nil, err
					}
				}
			}
		} else {
			if err := steps(mapping(mapping(root, "runs"), "steps")); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File == out[j].File {
			return out[i].Line < out[j].Line
		}
		return out[i].File < out[j].File
	})
	return out, nil
}

// Only edit metadata reachable from this repository's actual workflows. Action
// fixtures and unused action definitions are copied for the producer, not updated.
func activeMetadata(files tree) (map[string]bool, error) {
	active := map[string]bool{}
	queue := []string{}
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
		var walk func(*yaml.Node)
		walk = func(n *yaml.Node) {
			if n.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(n.Content); i += 2 {
					if n.Content[i].Value != "uses" || n.Content[i+1].Kind != yaml.ScalarNode {
						continue
					}
					value := n.Content[i+1].Value
					var path string
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
					if strings.HasPrefix(path, ".github/workflows/") {
						queue = append(queue, path)
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
						if _, ok := files[path+suffix]; ok {
							queue = append(queue, path+suffix)
							break
						}
					}
				}
			}
			for _, child := range n.Content {
				walk(child)
			}
		}
		walk(&doc)
	}
	return active, nil
}

func rewriteRefs(files tree, refs []reference, target string) error {
	// Reverse source order makes multiple edits on one line stable.
	for i := len(refs) - 1; i >= 0; i-- {
		r := refs[i]
		f := files[r.File]
		lines := strings.Split(string(f.Data), "\n")
		if r.Line <= 0 || r.Line > len(lines) {
			return fmt.Errorf("invalid source location")
		}
		line := lines[r.Line-1]
		chars := []rune(line)
		index := r.Column - 1
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
		lines[r.Line-1] = string(chars[:index]) + r.Name + "@" + target + strings.TrimPrefix(tail, old)
		f.Data = []byte(strings.Join(lines, "\n"))
		files[r.File] = f
	}
	return nil
}
