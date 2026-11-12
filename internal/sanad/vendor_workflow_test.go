package sanad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVendoringVerifiesExistingReusableWorkflowPins(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(map[bool]string{false: "untracked-call", true: "changed-source-identity"}[tracked], func(t *testing.T) {
			root, opts := vendorOptions(t)
			opts.Write = true
			pin := WorkflowPin{Name: "org/ci/.github/workflows/build.yml", Ref: "v1.0.0", Commit: strings.Repeat("c", 40), Host: "github.com", OwnerID: 3, RepoID: 4, CalleeLocked: true}
			ref := pin.Ref
			if tracked {
				ref = pin.Commit
			}
			name := filepath.Join(root, ".github/workflows/ci.yml")
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, []byte("  reuse:\n    uses: "+pin.Name+"@"+ref+"\n")...)
			if err := os.WriteFile(name, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if tracked {
				state := tree{}
				if err := saveWorkflowState(state, workflowState{Version: 1, Pins: map[string]WorkflowPin{workflowKey(pin.Name, pin.Commit): pin}}); err != nil {
					t.Fatal(err)
				}
				if err := putFile(root, WorkflowStatePath, state[WorkflowStatePath]); err != nil {
					t.Fatal(err)
				}
			}
			changed := pin
			changed.OwnerID = 99
			opts.Workflows = fakeWorkflows{pins: map[string]WorkflowPin{pin.Ref: changed}}
			before, err := readTree(root, ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Run(context.Background(), "vendor", opts)
			if r.Success || r.Changed || err == nil && !r.Blocked() {
				t.Fatalf("vendoring bypassed workflow verification: %+v %v", r, err)
			}
			after, err := readTree(root, ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			if files := changedFiles(before, after); len(files) != 0 {
				t.Fatalf("failed workflow verification wrote files: %v", files)
			}
		})
	}
}
