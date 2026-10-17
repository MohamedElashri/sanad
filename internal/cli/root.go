package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MohamedElashri/sanad/internal/sanad"
	"github.com/spf13/cobra"
)

var version = "devel"
var commit = ""
var date = ""

type commandError struct {
	code int
	err  error
}

func (e commandError) Error() string { return e.err.Error() }
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := err.(commandError); ok {
		return e.code
	}
	return 2
}

func NewRootCommand() *cobra.Command {
	var opts sanad.Options
	format := "table"
	root := &cobra.Command{Use: "sanad", Short: "Adopt, review, and update native GitHub Actions lockfiles", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	root.PersistentFlags().StringVar(&opts.Root, "root", "", "repository directory")
	root.PersistentFlags().StringVar(&opts.Repo, "repo", "", "repository identity: [HOST/]OWNER/REPO")
	root.PersistentFlags().StringVar(&opts.Config, "config", sanad.ConfigPath, "versioned policy file")
	root.PersistentFlags().StringVar(&format, "format", "table", "table, json, or markdown")
	run := func(name string) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" && format != "markdown" {
				return fmt.Errorf("unsupported format %q", format)
			}
			r, err := sanad.Run(cmd.Context(), name, opts)
			if format == "json" {
				if e := json.NewEncoder(cmd.OutOrStdout()).Encode(r); e != nil {
					return e
				}
			} else {
				fmt.Fprint(cmd.OutOrStdout(), r.Markdown())
				if format == "table" {
					for _, f := range r.Files {
						fmt.Fprint(cmd.OutOrStdout(), "\n"+f.Diff)
					}
				}
			}
			if err != nil {
				return commandError{2, err}
			}
			if r.Blocked() {
				return commandError{1, fmt.Errorf("blocking Sanad findings")}
			}
			return nil
		}
	}
	root.RunE = run("status")
	actionRef := ""
	if date != "" && strings.HasPrefix(version, "1.") && !strings.Contains(version, "-") {
		actionRef = commit
	}
	for _, name := range []string{"status", "doctor", "init", "sync", "plan", "update", "check", "recover"} {
		cmd := &cobra.Command{Use: name, Short: descriptions[name], Args: cobra.NoArgs, RunE: run(name)}
		if name == "init" || name == "sync" || name == "update" || name == "recover" {
			cmd.Flags().BoolVar(&opts.Write, "write", false, "apply the proposed local changes")
		}
		if name == "init" {
			cmd.Flags().BoolVar(&opts.ConfirmNative, "confirm-native", false, "confirm this repository supports native lockfile enforcement")
			cmd.Flags().BoolVar(&opts.Automation, "automation", false, "generate verification and scheduled-update workflows")
			cmd.Flags().StringVar(&opts.ActionRef, "action-ref", actionRef, "released Sanad action commit for generated workflows")
		}
		if name == "check" {
			cmd.Flags().BoolVar(&opts.Offline, "offline", false, "check local coverage without network access")
			cmd.Flags().StringVar(&opts.Base, "base", "", "trusted base commit for adoption-policy checks")
		}
		if name == "plan" || name == "update" {
			cmd.Flags().StringVar(&opts.Action, "action", "", "select owner/repo[/path]")
			cmd.Flags().StringVar(&opts.To, "to", "", "explicit target ref (requires --action)")
			cmd.Flags().StringVar(&opts.Level, "level", "", "patch, minor, or major updates")
			cmd.Flags().StringVar(&opts.Constraint, "constraint", "", "SemVer constraint")
		}
		root.AddCommand(cmd)
	}
	root.AddCommand(&cobra.Command{Use: "version", Short: "Show Sanad and native tool versions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if format == "json" {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"version": version, "commit": commit, "date": date, "backend_version": sanad.BackendVersion, "parser_version": "v0.0.6", "lockfile_versions": []string{"v0.0.1", "v0.0.2", "v0.0.3"}, "report_version": 1})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "sanad %s\nnative backend %s; parser v0.0.6; lockfile v0.0.1–v0.0.3\n", version, sanad.BackendVersion)
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "setup", Short: "Install and verify the pinned native backend", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		backend, err := sanad.EnsureBackend(cmd.Context(), true)
		if err != nil {
			return err
		}
		if format == "json" {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"backend_version": sanad.BackendVersion, "path": backend.Path})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Native backend %s ready: %s\n", sanad.BackendVersion, backend.Path)
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "config", Short: "Show effective policy", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return sanad.PrintConfig(cmd.Context(), opts, cmd.OutOrStdout())
	}})
	root.AddCommand(&cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Print shell completion", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
		case "zsh":
			return root.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unknown shell %q", args[0])
		}
	}})
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "Show help", RunE: func(cmd *cobra.Command, args []string) error {
		target, _, err := root.Find(args)
		if err != nil {
			return err
		}
		return target.Help()
	}})
	// Keep paths independent of the calling directory; never change process cwd.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if strings.Contains(opts.Config, "\\") || filepath.IsAbs(opts.Config) {
			return fmt.Errorf("--config must be a repository-relative path")
		}
		return nil
	}
	return root
}

var descriptions = map[string]string{"status": "Show native lockfile readiness and dependencies", "doctor": "Diagnose tools, authentication, and state", "init": "Preview native onboarding and accept an initial baseline", "sync": "Reconcile workflow edits while preserving existing pins", "plan": "Explain eligible dependency updates", "update": "Apply eligible dependency updates", "check": "Verify frozen native pins and adoption policy", "recover": "Restore an interrupted write transaction"}
