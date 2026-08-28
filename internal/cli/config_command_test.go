package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestConfigValidateUsesBuiltInDefaults(t *testing.T) {
	withTempWorkingDir(t)
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"config", "validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config validate returned error: %v", err)
	}
	if !strings.Contains(out.String(), "built-in defaults") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestConfigShowJSONIncludesOrigins(t *testing.T) {
	withTempWorkingDir(t)
	configData := "cooldown = \"2d\"\n[upgrade.actions.\"actions/checkout\"]\nlevel = \"minor\"\n"
	if err := os.WriteFile(".sanad.toml", []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"config", "show", "--origins", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["cooldown"] != "2d" {
		t.Fatalf("cooldown = %#v, want 2d", got["cooldown"])
	}
	if origins, ok := got["origins"].([]any); !ok || len(origins) != 2 {
		t.Fatalf("origins = %#v, want defaults and repository config", got["origins"])
	}
	upgrade, ok := got["upgrade"].(map[string]any)
	if !ok || upgrade["latest_release"] != "github-release" {
		t.Fatalf("upgrade = %#v, want complete effective upgrade config", got["upgrade"])
	}
	actions, ok := upgrade["actions"].(map[string]any)
	policy, policyOK := actions["actions/checkout"].(map[string]any)
	if !ok || !policyOK || policy["level"] != "minor" {
		t.Fatalf("upgrade actions use unexpected JSON shape: %#v", upgrade["actions"])
	}
	if _, exists := policy["Level"]; exists {
		t.Fatalf("upgrade action policy leaked Go field names: %#v", policy)
	}
}
