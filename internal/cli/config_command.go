package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MohamedElashri/sanad/internal/config"
	"github.com/spf13/cobra"
)

type configShowOptions struct {
	origins bool
}

func newConfigCommand(rootOpts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Validate and inspect effective configuration"}
	cmd.AddCommand(newConfigValidateCommand(rootOpts), newConfigShowCommand(rootOpts))
	return cmd
}

func newConfigValidateCommand(rootOpts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration without scanning workflows",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(rootOpts, func() error {
				cfg, err := loadConfig(rootOpts)
				if err != nil {
					return err
				}
				if cfg.Source == "defaults" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Configuration is valid (built-in defaults).")
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Configuration is valid: %s\n", cfg.Source)
				}
				return nil
			})
		},
	}
}

func newConfigShowCommand(rootOpts *rootOptions) *cobra.Command {
	opts := &configShowOptions{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the fully merged effective configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(rootOpts, func() error {
				cfg, err := loadConfig(rootOpts)
				if err != nil {
					return err
				}
				switch rootOpts.format {
				case "json":
					return printConfigJSON(cmd, cfg, opts.origins)
				case "table":
					return printConfigText(cmd, cfg, opts.origins)
				default:
					return fmt.Errorf("unsupported format %q: expected table or json", rootOpts.format)
				}
			})
		},
	}
	cmd.Flags().BoolVar(&opts.origins, "origins", false, "show configuration source precedence")
	return cmd
}

func printConfigJSON(cmd *cobra.Command, cfg config.Config, origins bool) error {
	view := map[string]any{
		"workflow_paths":  cfg.WorkflowPaths,
		"cooldown":        durationString(cfg.Cooldown),
		"cooldown_source": cfg.CooldownSource,
		"updates": map[string]any{
			"tags": cfg.Updates.Tags, "branches": cfg.Updates.Branches, "unpinned": cfg.Updates.Unpinned, "reusable_workflows": cfg.Updates.ReusableWorkflows,
		},
		"ignore":       map[string]any{"actions": cfg.Ignore.Actions, "files": cfg.Ignore.Files},
		"organization": map[string]any{"policy_files": cfg.Organization.PolicyFiles},
		"comments":     map[string]any{"write": cfg.Comments.Write, "format": cfg.Comments.Format},
		"upgrade": map[string]any{
			"latest_release": cfg.Upgrade.LatestRelease, "level": cfg.Upgrade.Level, "constraint": cfg.Upgrade.Constraint, "selection": cfg.Upgrade.Selection, "actions": cfg.Upgrade.Actions,
		},
		"security": map[string]any{
			"require_full_sha": cfg.Security.RequireFullSHA, "require_commit_in_source_repo": cfg.Security.RequireCommitInSourceRepo,
			"allow_private": cfg.Security.AllowPrivate, "deny_forks": cfg.Security.DenyForks,
		},
	}
	if origins {
		view["origins"] = configOrigins(cfg)
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(view)
}

func printConfigText(cmd *cobra.Command, cfg config.Config, origins bool) error {
	out := cmd.OutOrStdout()
	if origins {
		_, _ = fmt.Fprintln(out, "# Sources, lowest to highest precedence:")
		for _, source := range configOrigins(cfg) {
			_, _ = fmt.Fprintf(out, "# - %s\n", source)
		}
		_, _ = fmt.Fprintln(out, "# Command flags override these values for the current invocation.")
		_, _ = fmt.Fprintln(out)
	}
	_, _ = fmt.Fprintf(out, "workflow_paths = %s\n", stringArray(cfg.WorkflowPaths))
	_, _ = fmt.Fprintf(out, "cooldown = %s\n", strconv.Quote(durationString(cfg.Cooldown)))
	_, _ = fmt.Fprintf(out, "cooldown_source = %s\n\n", strconv.Quote(cfg.CooldownSource))
	_, _ = fmt.Fprintf(out, "[updates]\ntags = %q\nbranches = %q\nunpinned = %q\nreusable_workflows = %t\n\n", cfg.Updates.Tags, cfg.Updates.Branches, cfg.Updates.Unpinned, cfg.Updates.ReusableWorkflows)
	_, _ = fmt.Fprintf(out, "[ignore]\nactions = %s\nfiles = %s\n\n", stringArray(cfg.Ignore.Actions), stringArray(cfg.Ignore.Files))
	_, _ = fmt.Fprintf(out, "[organization]\npolicy_files = %s\n\n", stringArray(cfg.Organization.PolicyFiles))
	_, _ = fmt.Fprintf(out, "[comments]\nwrite = %t\nformat = %q\n\n", cfg.Comments.Write, cfg.Comments.Format)
	_, _ = fmt.Fprintf(out, "[upgrade]\nlatest_release = %q\nlevel = %q\n", cfg.Upgrade.LatestRelease, cfg.Upgrade.Level)
	if cfg.Upgrade.Constraint != "" {
		_, _ = fmt.Fprintf(out, "constraint = %q\n", cfg.Upgrade.Constraint)
	}
	_, _ = fmt.Fprintf(out, "selection = %q\n", cfg.Upgrade.Selection)
	selectors := make([]string, 0, len(cfg.Upgrade.Actions))
	for selector := range cfg.Upgrade.Actions {
		selectors = append(selectors, selector)
	}
	sort.Strings(selectors)
	for _, selector := range selectors {
		policy := cfg.Upgrade.Actions[selector]
		_, _ = fmt.Fprintf(out, "\n[upgrade.actions.%q]\n", selector)
		if policy.Level != "" {
			_, _ = fmt.Fprintf(out, "level = %q\n", policy.Level)
		}
		if policy.Constraint != "" {
			_, _ = fmt.Fprintf(out, "constraint = %q\n", policy.Constraint)
		}
		if policy.Selection != "" {
			_, _ = fmt.Fprintf(out, "selection = %q\n", policy.Selection)
		}
	}
	_, _ = fmt.Fprintf(out, "\n[security]\nrequire_full_sha = %t\nrequire_commit_in_source_repo = %t\nallow_private = %t\ndeny_forks = %t\n", cfg.Security.RequireFullSHA, cfg.Security.RequireCommitInSourceRepo, cfg.Security.AllowPrivate, cfg.Security.DenyForks)
	return nil
}

func configOrigins(cfg config.Config) []string {
	origins := []string{"built-in defaults"}
	for _, source := range cfg.PolicySources {
		origins = append(origins, "organization policy: "+source)
	}
	if cfg.Source != "defaults" {
		origins = append(origins, "repository config: "+cfg.Source)
	}
	return origins
}

func stringArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func durationString(value interface{ String() string }) string {
	text := value.String()
	if strings.HasSuffix(text, "h0m0s") {
		hours := strings.TrimSuffix(text, "h0m0s")
		if hoursInt, err := strconv.Atoi(hours); err == nil && hoursInt%24 == 0 {
			return fmt.Sprintf("%dd", hoursInt/24)
		}
	}
	return text
}
