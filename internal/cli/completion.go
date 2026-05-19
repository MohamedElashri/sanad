package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	completionShellBash       = "bash"
	completionShellZsh        = "zsh"
	completionShellFish       = "fish"
	completionShellPowerShell = "powershell"
)

var completionShells = []string{
	completionShellBash,
	completionShellZsh,
	completionShellFish,
	completionShellPowerShell,
}

type completionInstallOptions struct {
	dryRun    bool
	noProfile bool
}

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate or install shell completion scripts",
		Long: strings.TrimSpace(`Generate shell completion scripts or install them for the current user.

Package managers such as Homebrew and Nix install completions automatically.
Use "sanad completion install" for Go installs, manual archives, or other installs
that only place the sanad binary on PATH.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	for _, shell := range completionShells {
		cmd.AddCommand(newCompletionGenerateCommand(shell))
	}
	cmd.AddCommand(newCompletionInstallCommand())

	return cmd
}

func newCompletionGenerateCommand(shell string) *cobra.Command {
	return &cobra.Command{
		Use:   shell,
		Short: fmt.Sprintf("Generate %s completion script", shell),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(cmd.Root(), shell, cmd.OutOrStdout())
		},
	}
}

func newCompletionInstallCommand() *cobra.Command {
	opts := &completionInstallOptions{}

	cmd := &cobra.Command{
		Use:   "install [bash|zsh|fish|powershell]",
		Short: "Install shell completion for the current user",
		Long: strings.TrimSpace(`Install sanad shell completion for the current user.

With no shell argument, sanad detects the shell from the environment. Completion
files are written to user-level locations. Bash, zsh, and PowerShell profile files
are updated with a marked block when activation is needed; pass --no-profile to
write only the completion file.`),
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		ValidArgs: completionShells,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := ""
			if len(args) == 1 {
				shell = args[0]
			}
			return runCompletionInstall(cmd, shell, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print the files that would be written")
	cmd.Flags().BoolVar(&opts.noProfile, "no-profile", false, "do not update shell profile files")

	return cmd
}

func runCompletionInstall(cmd *cobra.Command, shell string, opts *completionInstallOptions) error {
	if shell == "" {
		detected, err := detectCompletionShell()
		if err != nil {
			return err
		}
		shell = detected
	}
	if !validCompletionShell(shell) {
		return fmt.Errorf("unsupported shell %q: expected bash, zsh, fish, or powershell", shell)
	}

	script, err := completionScript(cmd.Root(), shell)
	if err != nil {
		return err
	}
	target, err := completionInstallTarget(shell)
	if err != nil {
		return err
	}

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would install %s completion to %s\n", shell, target)
		if profile := completionProfileUpdate(shell); profile.Path != "" && !opts.noProfile {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would update %s\n", profile.Path)
		}
		return nil
	}

	if err := writeCompletionFile(target, script); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed %s completion to %s\n", shell, target)

	if !opts.noProfile {
		if profile := completionProfileUpdate(shell); profile.Path != "" {
			if err := ensureProfileBlock(profile); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", profile.Path)
		}
	}

	return nil
}

func completionScript(root *cobra.Command, shell string) ([]byte, error) {
	var out bytes.Buffer
	if err := generateCompletion(root, shell, &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func generateCompletion(root *cobra.Command, shell string, out io.Writer) error {
	switch shell {
	case completionShellBash:
		return root.GenBashCompletionV2(out, true)
	case completionShellZsh:
		return root.GenZshCompletion(out)
	case completionShellFish:
		return root.GenFishCompletion(out, true)
	case completionShellPowerShell:
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell %q: expected bash, zsh, fish, or powershell", shell)
	}
}

func detectCompletionShell() (string, error) {
	if shell := shellFromName(os.Getenv("SHELL")); shell != "" {
		return shell, nil
	}
	if runtime.GOOS == "windows" {
		if os.Getenv("PSModulePath") != "" {
			return completionShellPowerShell, nil
		}
		if shell := shellFromName(os.Getenv("COMSPEC")); shell != "" {
			return shell, nil
		}
	}
	return "", fmt.Errorf("could not detect shell; pass one of bash, zsh, fish, or powershell")
}

func shellFromName(name string) string {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	base = strings.TrimSuffix(base, ".exe")
	switch {
	case strings.Contains(base, "bash"):
		return completionShellBash
	case strings.Contains(base, "zsh"):
		return completionShellZsh
	case strings.Contains(base, "fish"):
		return completionShellFish
	case strings.Contains(base, "pwsh"), strings.Contains(base, "powershell"):
		return completionShellPowerShell
	default:
		return ""
	}
}

func validCompletionShell(shell string) bool {
	for _, candidate := range completionShells {
		if shell == candidate {
			return true
		}
	}
	return false
}

func completionInstallTarget(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch shell {
	case completionShellBash:
		return filepath.Join(xdgDataHome(home), "bash-completion", "completions", "sanad"), nil
	case completionShellZsh:
		return filepath.Join(home, ".zsh", "completions", "_sanad"), nil
	case completionShellFish:
		return filepath.Join(xdgConfigHome(home), "fish", "completions", "sanad.fish"), nil
	case completionShellPowerShell:
		return filepath.Join(powerShellDataHome(home), "sanad", "completions", "sanad.ps1"), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func xdgDataHome(home string) string {
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return value
	}
	return filepath.Join(home, ".local", "share")
}

func xdgConfigHome(home string) string {
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		return value
	}
	return filepath.Join(home, ".config")
}

func powerShellDataHome(home string) string {
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return value
		}
	}
	return xdgDataHome(home)
}

func writeCompletionFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write completion file: %w", err)
	}
	return nil
}

type profileUpdate struct {
	Path  string
	Block string
}

func completionProfileUpdate(shell string) profileUpdate {
	home, err := os.UserHomeDir()
	if err != nil {
		return profileUpdate{}
	}

	switch shell {
	case completionShellBash:
		target, err := completionInstallTarget(shell)
		if err != nil {
			return profileUpdate{}
		}
		return profileUpdate{
			Path: filepath.Join(home, ".bashrc"),
			Block: strings.Join([]string{
				"# sanad completion start",
				fmt.Sprintf(`if [ -r "%s" ]; then`, filepath.ToSlash(target)),
				fmt.Sprintf(`  source "%s"`, filepath.ToSlash(target)),
				"fi",
				"# sanad completion end",
				"",
			}, "\n"),
		}
	case completionShellZsh:
		return profileUpdate{
			Path: filepath.Join(home, ".zshrc"),
			Block: strings.Join([]string{
				"# sanad completion start",
				`fpath=("$HOME/.zsh/completions" $fpath)`,
				"autoload -Uz compinit",
				"compinit",
				"# sanad completion end",
				"",
			}, "\n"),
		}
	case completionShellPowerShell:
		target, err := completionInstallTarget(shell)
		if err != nil {
			return profileUpdate{}
		}
		return profileUpdate{
			Path: powerShellProfilePath(home),
			Block: strings.Join([]string{
				"# sanad completion start",
				fmt.Sprintf(`. "%s"`, filepath.ToSlash(target)),
				"# sanad completion end",
				"",
			}, "\n"),
		}
	default:
		return profileUpdate{}
	}
}

func powerShellProfilePath(home string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	return filepath.Join(xdgConfigHome(home), "powershell", "Microsoft.PowerShell_profile.ps1")
}

func ensureProfileBlock(profile profileUpdate) error {
	if err := os.MkdirAll(filepath.Dir(profile.Path), 0o755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}

	data, err := os.ReadFile(profile.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read profile: %w", err)
	}
	if strings.Contains(string(data), "# sanad completion start") {
		return nil
	}

	prefix := ""
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = "\n"
	}
	if err := os.WriteFile(profile.Path, append(data, []byte(prefix+profile.Block)...), 0o644); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return nil
}
