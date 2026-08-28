package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDoctorReportsLockAndPolicyHealth(t *testing.T) {
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: ./.github/actions/local\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	for _, want := range []string{"Lockfile:", "All workflow dependencies comply"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorJSONIsOneDocument(t *testing.T) {
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: ./.github/actions/local\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"doctor", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	var report doctorReport
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("doctor output is not JSON: %v\n%s", err, out.String())
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("doctor output contains trailing data: %v\n%s", err, out.String())
	}
	if report.Version != 1 || !report.Policy.Passed || report.Lock.Version != 1 {
		t.Fatalf("unexpected doctor report: %#v", report)
	}
}
