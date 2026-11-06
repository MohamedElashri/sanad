package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/MohamedElashri/sanad/internal/sanad"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

func writeReport(w io.Writer, report sanad.Report, format string) error {
	if format == "json" {
		return json.NewEncoder(w).Encode(report)
	}
	var output string
	profile := termenv.Ascii
	if format == "table" {
		var width int
		width, profile = terminalOptions(w)
		if profile != termenv.Ascii {
			restore, err := termenv.EnableVirtualTerminalProcessing(termenv.NewOutput(w, termenv.WithProfile(profile)))
			if err != nil {
				profile = termenv.Ascii
			} else {
				defer func() { _ = restore() }()
			}
		}
		output = renderReport(report, width, profile)
	} else {
		output = report.Markdown()
	}
	if _, err := io.WriteString(w, output); err != nil {
		return err
	}
	if format == "table" {
		// Color each line without reflowing or interpreting patch syntax.
		for _, file := range report.Files {
			if _, err := io.WriteString(w, "\n"+renderDiff(file.Diff, profile)); err != nil {
				return err
			}
		}
	}
	return nil
}

func terminalOptions(w io.Writer) (int, termenv.Profile) {
	width, profile := 80, termenv.Ascii
	file, ok := w.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return width, profile
	}
	if columns, _, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
		width = columns
	}
	if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		// Use the actual destination, including descriptor-preserving wrappers.
		// WithTTY is safe here because IsTerminal already checked that descriptor.
		profile = termenv.NewOutput(w, termenv.WithTTY(true)).EnvColorProfile()
	}
	return width, profile
}

func renderDiff(diff string, profile termenv.Profile) string {
	var out strings.Builder
	inHunk := false
	for _, part := range strings.SplitAfter(diff, "\n") {
		line := strings.TrimSuffix(part, "\n")
		color, bold := "", false
		switch {
		case strings.HasPrefix(line, "diff "):
			inHunk = false
			color, bold = colorCyan, true
		case strings.HasPrefix(line, "@@"):
			inHunk = true
			color = colorCyan
		case !inHunk && (strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ")):
			color, bold = colorCyan, true
		case strings.HasPrefix(line, "+"):
			color = colorGreen
		case strings.HasPrefix(line, "-"):
			color = colorRed
		}
		out.WriteString(paint(profile, escapeControls(line, true), color, bold))
		if strings.HasSuffix(part, "\n") {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
