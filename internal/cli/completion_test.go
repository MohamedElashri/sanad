package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionCommandGeneratesScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{shell: "bash", want: "__start_sanad"},
		{shell: "zsh", want: "#compdef sanad"},
		{shell: "fish", want: "complete -c sanad"},
		{shell: "powershell", want: "Register-ArgumentCompleter"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var out bytes.Buffer
			cmd := NewRootCommand()
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"completion", tt.shell})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("completion output for %s does not contain %q:\n%s", tt.shell, tt.want, out.String())
			}
		})
	}
}

func TestCompletionInstallDetectsFish(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, "config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("SHELL", "/usr/bin/fish")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"completion", "install"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	path := filepath.Join(configHome, "fish", "completions", "sanad.fish")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("completion file was not written: %v", err)
	}
	if !strings.Contains(string(data), "complete -c sanad") {
		t.Fatalf("fish completion file missing completion content:\n%s", string(data))
	}
	if !strings.Contains(out.String(), "Installed fish completion") {
		t.Fatalf("install output = %q, want success message", out.String())
	}
}

func TestCompletionInstallBashUpdatesProfileOnce(t *testing.T) {
	home := t.TempDir()
	dataHome := filepath.Join(home, "data")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("SHELL", "/bin/bash")

	for range 2 {
		var out bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"completion", "install", "bash"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
	}

	completionPath := filepath.Join(dataHome, "bash-completion", "completions", "sanad")
	data, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatalf("bash completion file was not written: %v", err)
	}
	if !strings.Contains(string(data), "__start_sanad") {
		t.Fatalf("bash completion file missing start function:\n%s", string(data))
	}

	profilePath := filepath.Join(home, ".bashrc")
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("bash profile was not written: %v", err)
	}
	if count := strings.Count(string(profile), "# sanad completion start"); count != 1 {
		t.Fatalf("profile contains %d sanad completion blocks, want 1:\n%s", count, string(profile))
	}
	if !strings.Contains(string(profile), filepath.ToSlash(completionPath)) {
		t.Fatalf("profile does not source completion path %q:\n%s", completionPath, string(profile))
	}
}

func TestCompletionInstallZshUpdatesProfileOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	for range 2 {
		var out bytes.Buffer
		cmd := NewRootCommand()
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"completion", "install", "zsh"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
	}

	completionPath := filepath.Join(home, ".zsh", "completions", "_sanad")
	data, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatalf("zsh completion file was not written: %v", err)
	}
	if !strings.Contains(string(data), "#compdef sanad") {
		t.Fatalf("zsh completion file missing compdef:\n%s", string(data))
	}

	profilePath := filepath.Join(home, ".zshrc")
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("zsh profile was not written: %v", err)
	}
	if count := strings.Count(string(profile), "# sanad completion start"); count != 1 {
		t.Fatalf("profile contains %d sanad completion blocks, want 1:\n%s", count, string(profile))
	}
}
