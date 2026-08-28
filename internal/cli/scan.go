package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/policy"
	"github.com/MohamedElashri/sanad/internal/workflow"
	"github.com/spf13/cobra"
)

type scanOptions struct {
	workflowPaths []string
}

type scanEntry struct {
	File       string             `json:"file"`
	NodePath   string             `json:"node_path"`
	Raw        string             `json:"raw"`
	Line       int                `json:"line"`
	Column     int                `json:"column"`
	HeadLine   string             `json:"head_line"`
	LineIndex  int                `json:"line_index"`
	Owner      string             `json:"owner,omitempty"`
	Repo       string             `json:"repo,omitempty"`
	Path       string             `json:"path,omitempty"`
	Ref        string             `json:"ref,omitempty"`
	Kind       actions.ActionKind `json:"kind"`
	Pinned     bool               `json:"pinned"`
	Valid      bool               `json:"valid"`
	Ignored    bool               `json:"ignored"`
	IgnoreBy   string             `json:"ignore_by,omitempty"`
	IgnoreRule string             `json:"ignore_rule,omitempty"`
	Error      string             `json:"error,omitempty"`
}

func newScanCommand(opts *rootOptions) *cobra.Command {
	scanOpts := &scanOptions{}

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover GitHub Actions workflow dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withRepositoryRoot(opts, func() error { return runScan(cmd, opts, scanOpts) })
		},
	}

	cmd.Flags().StringSliceVar(&scanOpts.workflowPaths, "workflows", nil, "workflow file or directory paths to scan")

	return cmd
}

func runScan(cmd *cobra.Command, opts *rootOptions, scanOpts *scanOptions) error {
	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}

	paths := cfg.WorkflowPaths
	if len(scanOpts.workflowPaths) > 0 {
		paths = scanOpts.workflowPaths
	}

	files, err := workflow.DiscoverWorkflowFiles(paths)
	if err != nil {
		return err
	}

	uses, err := workflow.ExtractUsesFromFiles(files)
	if err != nil {
		return err
	}

	entries, err := classifyUses(uses, policyOptionsFromConfig(cfg, time.Time{}))
	if err != nil {
		return configError{err: err}
	}

	switch opts.format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	case "table":
		return printScanTable(cmd, entries)
	default:
		return fmt.Errorf("unsupported format %q: expected table or json", opts.format)
	}
}

func classifyUses(uses []workflow.UseNode, opts policy.Options) ([]scanEntry, error) {
	entries := make([]scanEntry, 0, len(uses))
	for _, use := range uses {
		parsed := actions.Parse(use.Raw)
		ignore, err := policy.MatchIgnore(parsed, use.File, opts)
		if err != nil {
			return nil, err
		}
		entries = append(entries, scanEntry{
			File:       use.File,
			NodePath:   use.NodePath,
			Raw:        use.Raw,
			Line:       use.Line,
			Column:     use.Column,
			HeadLine:   use.HeadLine,
			LineIndex:  use.LineIndex,
			Owner:      parsed.Owner,
			Repo:       parsed.Repo,
			Path:       parsed.Path,
			Ref:        parsed.Ref,
			Kind:       parsed.Kind,
			Pinned:     parsed.Pinned,
			Valid:      parsed.Valid,
			Ignored:    ignore.Ignored,
			IgnoreBy:   ignore.Kind,
			IgnoreRule: ignore.Pattern,
			Error:      parsed.Error,
		})
	}
	return entries, nil
}

func printScanTable(cmd *cobra.Command, entries []scanEntry) error {
	rows := make([]styledTableRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, styledTableRow{
			{Text: entry.File, Role: colorFile},
			{Text: fmt.Sprintf("%d", entry.Line), Role: colorLine},
			{Text: entry.actionName()},
			{Text: emptyDash(entry.Ref), Role: scanRefColorRole(entry)},
			{Text: string(entry.Kind), Role: scanKindColorRole(entry)},
			{Text: yesNo(entry.Pinned), Role: boolColorRole(entry.Pinned)},
			{Text: yesNo(entry.Valid), Role: boolColorRole(entry.Valid)},
			{Text: yesNo(entry.Ignored), Role: ignoredColorRole(entry.Ignored)},
			{Text: emptyDash(entry.IgnoreRule), Role: mutedIfDash(entry.IgnoreRule)},
			{Text: emptyDash(entry.Error), Role: scanErrorColorRole(entry)},
		})
	}
	return printStyledTable(
		cmd.OutOrStdout(),
		styleForCommand(cmd),
		[]string{"FILE", "LINE", "ACTION", "REF", "KIND", "PINNED", "VALID", "IGNORED", "IGNORE RULE", "ERROR"},
		rows,
	)
}

func scanRefColorRole(entry scanEntry) colorRole {
	if entry.Ref == "" {
		return colorMuted
	}
	if entry.Valid && entry.Pinned {
		return colorSuccess
	}
	if !entry.Valid {
		return colorDanger
	}
	return colorWarning
}

func scanKindColorRole(entry scanEntry) colorRole {
	switch entry.Kind {
	case actions.KindLocalAction, actions.KindDockerAction:
		return colorMuted
	default:
		return colorInfo
	}
}

func scanErrorColorRole(entry scanEntry) colorRole {
	if entry.Error != "" {
		return colorDanger
	}
	return colorMuted
}

func boolColorRole(value bool) colorRole {
	if value {
		return colorSuccess
	}
	return colorDanger
}

func ignoredColorRole(value bool) colorRole {
	if value {
		return colorMuted
	}
	return colorNone
}

func mutedIfDash(value string) colorRole {
	if value == "" {
		return colorMuted
	}
	return colorNone
}

func (e scanEntry) actionName() string {
	return actionName(e.Owner, e.Repo, e.Path, e.Raw)
}

func actionName(owner string, repo string, path string, fallback string) string {
	switch {
	case owner != "" && repo != "" && path != "":
		return owner + "/" + repo + "/" + path
	case owner != "" && repo != "":
		return owner + "/" + repo
	case path != "":
		return path
	default:
		return fallback
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
