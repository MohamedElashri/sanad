package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsCommandErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"upgrade", "--action", "actions/checkout", "--all"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "pass exactly one of --action or --all") {
		t.Fatalf("stderr = %q, want upgrade validation error", got)
	}
}

func TestRunReturnsSuccessWithoutErrorOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "sanad") {
		t.Fatalf("stdout = %q, want version output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
