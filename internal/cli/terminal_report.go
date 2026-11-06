package cli

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/MohamedElashri/sanad/internal/sanad"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Use the terminal's own palette and foreground, without background queries.
const (
	colorRed    = "1"
	colorGreen  = "2"
	colorYellow = "3"
	colorCyan   = "6"
	colorMuted  = "8"
)

type terminalReport struct {
	strings.Builder
	width   int
	profile termenv.Profile
	styles  *lipgloss.Renderer
}

func paint(profile termenv.Profile, text, color string, bold bool) string {
	if text == "" {
		return ""
	}
	style := profile.String(text).Foreground(profile.Color(color))
	if bold {
		style = style.Bold()
	}
	return style.String()
}

func stateColor(state string) string {
	switch state {
	case "error", "blocked", "failed":
		return colorRed
	case "confirmed", "online", "eligible", "succeeded":
		return colorGreen
	case "warning", "unknown", "offline", "pending", "frozen":
		return colorYellow
	case "none", "unchanged":
		return colorMuted
	default:
		return colorCyan
	}
}

// Report fields are data, not Markdown. Keep punctuation literal and make
// control characters visible rather than letting them move the terminal cursor.
func terminalText(text string) string {
	return escapeControls(text, false)
}

func escapeControls(text string, keepTabs bool) string {
	var out strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) && !(keepTabs && r == '\t') {
			quoted := strconv.QuoteRune(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func (t *terminalReport) text(text, color string, bold bool) string {
	return paint(t.profile, terminalText(text), color, bold)
}

func (t *terminalReport) line(text string) {
	t.WriteString(ansi.Wrap(text, t.width, ""))
	t.WriteByte('\n')
}

func (t *terminalReport) heading(title string) {
	t.WriteByte('\n')
	t.line(t.text(title, colorCyan, true))
}

func renderReport(report sanad.Report, width int, profile termenv.Profile) string {
	styles := lipgloss.NewRenderer(io.Discard, termenv.WithProfile(profile))
	styles.SetColorProfile(profile)
	t := terminalReport{width: max(1, width), profile: profile, styles: styles}
	t.line(t.text("Sanad "+report.Command, colorCyan, true))
	result := "failed"
	if report.Success {
		result = "succeeded"
	}
	t.line("Result: " + t.text(result, stateColor(result), true))
	t.line("Native support: " + t.text(report.NativeSupport, stateColor(report.NativeSupport), false) + "; verification: " + t.text(report.Verification, stateColor(report.Verification), false))
	coverageColor := colorYellow
	if report.Coverage.Total > 0 && report.Coverage.Locked == report.Coverage.Total {
		coverageColor = colorGreen
	}
	t.line(t.text(fmt.Sprintf("%d/%d workflows covered", report.Coverage.Locked, report.Coverage.Total), coverageColor, false) + fmt.Sprintf("; %d locked dependencies.", len(report.Dependencies)))
	if len(report.Findings) > 0 {
		t.heading("Findings")
		for _, finding := range report.Findings {
			location := ""
			if finding.File != "" {
				location = t.text(finding.File, colorCyan, false) + ": "
			}
			t.line(t.text(finding.Severity+":", stateColor(finding.Severity), true) + " " + location + t.text(finding.Message, "", false))
		}
	}
	if (report.Command == "status" || report.Command == "doctor") && len(report.Dependencies) > 0 {
		rows := make([][]string, 0, len(report.Dependencies))
		for _, dependency := range report.Dependencies {
			rows = append(rows, []string{dependency.Key, dependency.Commit, dependency.Host})
		}
		t.renderTable("Dependencies", []string{"Dependency", "Commit", "Host"}, rows, -1)
	}
	if len(report.Decisions) > 0 {
		rows := make([][]string, 0, len(report.Decisions))
		for _, decision := range report.Decisions {
			rows = append(rows, []string{decision.Action, decision.From, decision.To, decision.Status, decision.Reason})
		}
		t.renderTable("Update decisions", []string{"Action", "From", "To", "Decision", "Reason"}, rows, 3)
	}
	if len(report.Changes) > 0 {
		rows := make([][]string, 0, len(report.Changes))
		for _, change := range report.Changes {
			before, after := "—", "—"
			var paths []string
			if change.Before != nil {
				before = change.Before.Commit
				paths = append(paths, change.Before.Workflows...)
			}
			if change.After != nil {
				after = change.After.Commit
				paths = append(paths, change.After.Workflows...)
			}
			slices.Sort(paths)
			rows = append(rows, []string{change.Key, before, after, strings.Join(slices.Compact(paths), ", ")})
		}
		t.renderTable("Dependency changes", []string{"Dependency", "Before", "After", "Affected workflows"}, rows, -1)
	}
	if len(report.Files) > 0 {
		title := "Proposed files"
		if report.Changed {
			title = "Written files"
		}
		t.heading(title)
		for _, file := range report.Files {
			t.line(t.text(file.Path, colorCyan, false))
		}
	}
	return t.String()
}

func (t *terminalReport) renderTable(title string, headers []string, rows [][]string, stateColumn int) {
	t.heading(title)
	widths, natural := make([]int, len(headers)), make([]int, len(headers))
	used := len(headers) + 1 // outer borders plus column separators
	for column, header := range headers {
		widths[column] = ansi.StringWidth(header) + 2
		natural[column] = widths[column]
		used += widths[column]
		for _, row := range rows {
			row[column] = terminalText(row[column])
			natural[column] = max(natural[column], ansi.StringWidth(row[column])+2)
		}
	}
	if used > t.width {
		// A stacked layout keeps full labels and values on narrow terminals.
		for i, row := range rows {
			if i > 0 {
				t.WriteByte('\n')
			}
			for column, value := range row {
				color := ""
				if column == stateColumn {
					color = stateColor(value)
				}
				t.line(t.text(headers[column]+":", colorCyan, true) + " " + paint(t.profile, value, color, false))
			}
		}
		return
	}
	// Reserve enough space for every header before distributing extra width.
	// Lip Gloss otherwise truncates headers even when cell wrapping is enabled.
	for used < t.width {
		grew := false
		for column := range widths {
			if widths[column] < natural[column] && used < t.width {
				widths[column]++
				used++
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	border := lipgloss.NormalBorder()
	if t.profile == termenv.Ascii {
		border = lipgloss.ASCIIBorder()
	}
	grid := table.New().Headers(headers...).Rows(rows...).Width(used).Wrap(true).
		Border(border).BorderStyle(t.styles.NewStyle().Foreground(lipgloss.Color(colorMuted))).
		StyleFunc(func(row, column int) lipgloss.Style {
			style := t.styles.NewStyle().Padding(0, 1).Width(widths[column])
			if row == table.HeaderRow {
				return style.Foreground(lipgloss.Color(colorCyan)).Bold(true)
			}
			if column == stateColumn {
				return style.Foreground(lipgloss.Color(stateColor(rows[row][column]))).Bold(true)
			}
			return style
		})
	t.WriteString(grid.String())
	t.WriteByte('\n')
}
