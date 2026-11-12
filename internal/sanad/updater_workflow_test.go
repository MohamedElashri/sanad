package sanad

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUpdaterPullRequestLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the updater runs in Bash on Ubuntu")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("the updater requires Python 3")
	}
	_, source, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../.github/workflows/update-pr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	script := ""
	for _, step := range workflow.Jobs["update"].Steps {
		if step.ID == "pr" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("updater PR step is missing")
	}
	for _, tc := range []struct {
		name, state string
		lookupError bool
	}{
		{name: "first update"},
		{name: "existing update", state: "OPEN"},
		{name: "previous update merged", state: "MERGED"},
		{name: "previous update closed", state: "CLOSED"},
		{name: "lookup failure", state: "MERGED", lookupError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			state := filepath.Join(root, "state.json")
			calls := filepath.Join(root, "calls.jsonl")
			output := filepath.Join(root, "output")
			initial, _ := json.Marshal(map[string]any{"state": tc.state, "number": 81, "lookup_error": tc.lookupError})
			for name, content := range map[string][]byte{
				"state.json": initial,
				"git":        []byte("#!/bin/sh\nexit 0\n"),
				"gh":         []byte(updaterGHStub),
				"body.md":    []byte("Dependency update\n"),
			} {
				if err := os.WriteFile(filepath.Join(root, name), content, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"),
				"UPDATER_STATE="+state, "UPDATER_CALLS="+calls,
				"UPDATE_BRANCH=sanad/updates", "BASE_BRANCH=main",
				"GITHUB_REPOSITORY=example/repo", "GITHUB_OUTPUT="+output,
				"GH_TOKEN=test-token", "GITHUB_TOKEN=test-token",
				"CHANGED_FILES=[\".github/workflows/actions.lock\"]", "PR_BODY="+filepath.Join(root, "body.md"),
			)
			result, runErr := cmd.CombinedOutput()
			if tc.lookupError {
				if runErr == nil {
					t.Fatalf("lookup failure was ignored: %s", result)
				}
			} else if runErr != nil {
				t.Fatalf("PR step failed: %v\n%s", runErr, result)
			}
			final, err := os.ReadFile(state)
			if err != nil {
				t.Fatal(err)
			}
			var record struct {
				State  string `json:"state"`
				Number int    `json:"number"`
			}
			if err := json.Unmarshal(final, &record); err != nil {
				t.Fatal(err)
			}
			if tc.lookupError {
				if record.State != tc.state || record.Number != 81 {
					t.Fatalf("lookup failure changed a PR: %s", final)
				}
				return
			}
			wantNumber := 82
			if tc.state == "OPEN" {
				wantNumber = 81
			}
			if record.State != "OPEN" || record.Number != wantNumber {
				t.Fatalf("update has no correct open PR: %s", final)
			}
			outputs, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			wantURL := "url=https://github.com/example/repo/pull/82\n"
			if tc.state == "OPEN" {
				wantURL = "url=https://github.com/example/repo/pull/81\n"
			}
			if string(outputs) != wantURL {
				t.Fatalf("wrong PR output: %q", outputs)
			}
			invocations, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if tc.state == "OPEN" && strings.Contains(string(invocations), `"create"`) {
				t.Fatal("an existing open PR was duplicated")
			}
		})
	}
}

const updaterGHStub = `#!/usr/bin/env python3
import json, os, pathlib, sys
state_path = pathlib.Path(os.environ['UPDATER_STATE'])
state = json.loads(state_path.read_text())
args = sys.argv[1:]
with open(os.environ['UPDATER_CALLS'], 'a') as log:
    log.write(json.dumps(args) + '\n')
command = args[:2]
if command == ['pr', 'list']:
    if state['lookup_error']:
        sys.exit(41)
    if state['state'] == 'OPEN':
        print(state['number'])
elif command == ['pr', 'create']:
    if state['state'] == 'OPEN':
        sys.exit('duplicate open PR')
    state.update(state='OPEN', number=82)
    state_path.write_text(json.dumps(state))
    print('https://github.com/example/repo/pull/82')
elif command == ['pr', 'edit']:
    if not state['state']:
        sys.exit('PR does not exist')
elif command == ['pr', 'view']:
    if state['lookup_error']:
        sys.exit(41)
    if not state['state']:
        sys.exit(1)
    print('url=https://github.com/example/repo/pull/' + str(state['number']))
else:
    sys.exit('unexpected gh command: ' + repr(args))
`
