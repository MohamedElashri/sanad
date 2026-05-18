package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
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
			return runScan(cmd, opts, scanOpts)
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
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "FILE\tLINE\tACTION\tREF\tKIND\tPINNED\tVALID\tIGNORED\tIGNORE RULE\tERROR")
	for _, entry := range entries {
		_, _ = fmt.Fprintf(
			writer,
			"%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.File,
			entry.Line,
			entry.actionName(),
			emptyDash(entry.Ref),
			entry.Kind,
			yesNo(entry.Pinned),
			yesNo(entry.Valid),
			yesNo(entry.Ignored),
			emptyDash(entry.IgnoreRule),
			emptyDash(entry.Error),
		)
	}
	return writer.Flush()
}

func (e scanEntry) actionName() string {
	switch {
	case e.Owner != "" && e.Repo != "" && e.Path != "":
		return e.Owner + "/" + e.Repo + "/" + e.Path
	case e.Owner != "" && e.Repo != "":
		return e.Owner + "/" + e.Repo
	case e.Path != "":
		return e.Path
	default:
		return e.Raw
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
