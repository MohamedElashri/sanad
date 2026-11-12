package sanad

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"gopkg.in/yaml.v3"
)

type Protection struct {
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

func protections(files tree, host string) ([]Protection, error) {
	out := []Protection{}
	active, err := activeMetadata(files)
	if err != nil {
		return nil, err
	}
	refs, err := references(files)
	if err != nil {
		return nil, err
	}
	state, err := loadWorkflowState(files)
	if err != nil {
		return nil, err
	}
	var parsed lock.File
	if f, ok := files[lock.Path]; ok {
		parsed, err = lock.Parse(f.Data)
		if err != nil {
			return nil, err
		}
	}
	for _, ref := range refs {
		p := Protection{Kind: "action", File: ref.File, Line: ref.Line, Subject: ref.Name + "@" + ref.Ref, Status: "uncovered", Reason: "no native pin recorded for this call"}
		key := strings.ToLower(ref.NWO) + "@" + ref.Ref
		if _, ok := parsed.Dependencies[key]; ok {
			covered := false
			for wf, keys := range parsed.Workflows {
				if strings.HasPrefix(ref.File, ".github/workflows/") && wf != ref.File {
					continue
				}
				for _, k := range keys {
					if k == key {
						covered = true
					}
				}
			}
			if covered {
				p.Status = "native-lock"
				p.Reason = "native pin recorded; online check verifies identity and reachability"
			}
		}
		if ref.Workflow {
			p.Kind = "reusable-workflow"
			p.Status = "mutable"
			p.Reason = "caller lockfiles do not pin reusable workflow calls"
			if fullSHA.MatchString(ref.Ref) {
				p.Status = "inline-sha"
				p.Reason = "workflow code pinned; callee action dependencies use the callee repository's lockfile"
			}
			if pin, ok := state.Pins[workflowKey(ref.Name, ref.Ref)]; ok {
				if pin.CalleeLocked {
					p.Reason += "; callee structural coverage was recorded at this commit (not an online verification of its dependencies)"
				} else {
					p.Reason += "; no callee structural lockfile coverage recorded"
				}
			}
		}
		out = append(out, p)
	}
	for name := range active {
		f := files[name]
		var doc yaml.Node
		if err := yaml.Unmarshal(f.Data, &doc); err != nil {
			return nil, err
		}
		seen := map[*yaml.Node]bool{}
		var walk func(*yaml.Node)
		image := func(n *yaml.Node, label string) {
			n = unalias(n)
			if n == nil {
				return
			}
			if n.Kind == yaml.MappingNode {
				n = unalias(mapping(n, "image"))
				if n == nil {
					return
				}
			}
			status, reason := "mutable", "image contents are outside native action lockfiles"
			if imageDigest(n.Value) {
				status = "digest"
				reason = "image content digest recorded; runtime downloads remain outside coverage"
			} else if strings.Contains(n.Value, "${{") {
				status = "unknown"
				reason = "image selected by an expression"
			}
			out = append(out, Protection{Kind: "container", File: name, Line: n.Line, Subject: label + n.Value, Status: status, Reason: reason})
		}
		walk = func(n *yaml.Node) {
			n = unalias(n)
			if n == nil || seen[n] {
				return
			}
			seen[n] = true
			if n.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(n.Content); i += 2 {
					key := n.Content[i].Value
					value := unalias(n.Content[i+1])
					if value == nil {
						continue
					}
					switch key {
					case "runs-on":
						out = append(out, Protection{Kind: "runner", File: name, Line: value.Line, Subject: "runs-on", Status: "unknown", Reason: "runner labels and groups do not pin the VM or host image contents"})
					case "run":
						out = append(out, Protection{Kind: "runtime", File: name, Line: value.Line, Subject: "shell command", Status: "uninspected", Reason: "shell downloads and runtime dependencies are outside native action lockfiles"})
					case "uses":
						if strings.HasPrefix(value.Value, "docker://") {
							image(value, "uses: ")
						}
						if strings.HasPrefix(value.Value, "./") {
							out = append(out, Protection{Kind: "local-action", File: name, Line: value.Line, Subject: value.Value, Status: "workspace", Reason: "runs files from the checked-out workspace, which can differ from the workflow commit"})
						}
					case "container":
						image(value, "")
					case "services":
						if value.Kind == yaml.MappingNode {
							for j := 1; j < len(value.Content); j += 2 {
								image(value.Content[j], "service: ")
							}
						}
					}
				}
				if u := unalias(mapping(n, "using")); u != nil && u.Value == "docker" {
					image(mapping(n, "image"), "Docker action: ")
				}
				if u := unalias(mapping(n, "uses")); u != nil {
					versions := map[string]string{"actions/setup-node": "node-version", "actions/setup-go": "go-version", "actions/setup-python": "python-version", "actions/setup-java": "java-version", "ruby/setup-ruby": "ruby-version", "actions/setup-dotnet": "dotnet-version"}
					for action, key := range versions {
						if strings.HasPrefix(strings.ToLower(u.Value), action+"@") {
							subject := action
							if v := unalias(mapping(mapping(n, "with"), key)); v != nil {
								subject += " " + v.Value
							}
							out = append(out, Protection{Kind: "runtime-tool", File: name, Line: u.Line, Subject: subject, Status: "uninspected", Reason: "the action commit does not pin the downloaded tool's artifact digest"})
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
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Subject < b.Subject
	})
	return out, nil
}
func imageDigest(value string) bool {
	_, digest, ok := strings.Cut(value, "@sha256:")
	if !ok || len(digest) != 64 {
		return false
	}
	for _, r := range digest {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// Import the stable SARIF interchange format rather than depending on a linter's
// private JSON schema. The user runs zizmor; Sanad never installs or executes it.
func importZizmor(root, name string, r *Report) error {
	path, err := safePath(root, name)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > 16<<20 {
		return fmt.Errorf("zizmor SARIF report exceeds 16 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var sarif struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name string `json:"name"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				Rule    string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					Physical struct {
						Artifact struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							Line int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &sarif); err != nil {
		return err
	}
	if sarif.Version != "2.1.0" || len(sarif.Runs) == 0 {
		return fmt.Errorf("expected a SARIF 2.1.0 report")
	}
	for _, run := range sarif.Runs {
		if !strings.EqualFold(run.Tool.Driver.Name, "zizmor") {
			return fmt.Errorf("SARIF report must come from zizmor")
		}
		for _, result := range run.Results {
			if result.Level == "none" || result.Level == "note" {
				continue
			}
			severity := result.Level
			if severity == "" {
				severity = "warning"
			}
			if severity != "error" && severity != "warning" {
				return fmt.Errorf("invalid SARIF severity")
			}
			name := ""
			if len(result.Locations) > 0 {
				name = result.Locations[0].Physical.Artifact.URI
				if _, err := safePath(root, name); err != nil {
					return err
				}
			}
			r.Findings = append(r.Findings, Finding{Code: "zizmor-" + result.Rule, Severity: severity, File: name, Message: result.Message.Text})
		}
	}
	return nil
}
