package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStartUsesDefaultsWithoutCreatingConfigInPreview(t *testing.T) {
	withTempWorkingDir(t)
	writeApplyWorkflow(t, "jobs:\n  test:\n    steps:\n      - uses: ./.github/actions/local\n")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"start"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("start returned error: %v", err)
	}
	if _, err := os.Stat(".sanad.toml"); !os.IsNotExist(err) {
		t.Fatalf("start created default config, stat error = %v", err)
	}
	if !strings.Contains(out.String(), "Using built-in defaults") {
		t.Fatalf("unexpected start output:\n%s", out.String())
	}
}
