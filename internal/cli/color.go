package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

type colorRole string

const (
	colorNone    colorRole = ""
	colorHeader  colorRole = "header"
	colorSuccess colorRole = "success"
	colorWarning colorRole = "warning"
	colorDanger  colorRole = "danger"
	colorInfo    colorRole = "info"
	colorMuted   colorRole = "muted"
	colorFile    colorRole = "file"
	colorLine    colorRole = "line"
	colorReason  colorRole = "reason"
	colorAdd     colorRole = "add"
	colorDelete  colorRole = "delete"
	colorHunk    colorRole = "hunk"
)

type terminalTheme string

const (
	terminalThemeDark  terminalTheme = "dark"
	terminalThemeLight terminalTheme = "light"
)

type terminalStyle struct {
	enabled bool
	codes   map[colorRole]string
}

type colorSettings struct {
	mode  string
	theme terminalTheme
}

type styledTableCell struct {
	Text string
	Role colorRole
}

type styledTableRow []styledTableCell

func colorSettingsForCommand(cmd *cobra.Command) (colorSettings, error) {
	mode, err := effectiveColorMode(cmd)
	if err != nil {
		return colorSettings{}, err
	}
	theme, err := effectiveColorTheme()
	if err != nil {
		return colorSettings{}, err
	}
	return colorSettings{mode: mode, theme: theme}, nil
}

func styleForCommand(cmd *cobra.Command) terminalStyle {
	settings, err := colorSettingsForCommand(cmd)
	if err != nil || settings.mode == colorModeNever {
		return terminalStyle{}
	}
	if settings.mode == colorModeAuto && !outputSupportsColor(cmd.OutOrStdout()) {
		return terminalStyle{}
	}
	return newTerminalStyle(settings.theme)
}

func newTerminalStyle(theme terminalTheme) terminalStyle {
	codes := map[colorRole]string{
		colorHeader:  "1",
		colorSuccess: "32",
		colorDanger:  "31",
		colorInfo:    "36",
		colorMuted:   "90",
		colorFile:    "36",
		colorLine:    "90",
		colorAdd:     "32",
		colorDelete:  "31",
		colorHunk:    "36",
	}
	if theme == terminalThemeLight {
		codes[colorWarning] = "35"
		codes[colorReason] = "34"
	} else {
		codes[colorWarning] = "33"
		codes[colorReason] = "35"
	}
	return terminalStyle{enabled: true, codes: codes}
}

func (s terminalStyle) Wrap(role colorRole, text string) string {
	if !s.enabled || role == colorNone || text == "" {
		return text
	}
	code := s.codes[role]
	if code == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s terminalStyle) Wrapf(role colorRole, format string, args ...any) string {
	return s.Wrap(role, fmt.Sprintf(format, args...))
}

func printStyledTable(out io.Writer, style terminalStyle, header []string, rows []styledTableRow) error {
	widths := make([]int, len(header))
	for i, value := range header {
		widths[i] = len(value)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			if len(cell.Text) > widths[i] {
				widths[i] = len(cell.Text)
			}
		}
	}

	for i, value := range header {
		printStyledTableCell(out, style, styledTableCell{Text: value, Role: colorHeader}, widths[i], i == len(header)-1)
	}
	_, _ = fmt.Fprintln(out)
	for _, row := range rows {
		for i := range header {
			cell := styledTableCell{}
			if i < len(row) {
				cell = row[i]
			}
			printStyledTableCell(out, style, cell, widths[i], i == len(header)-1)
		}
		_, _ = fmt.Fprintln(out)
	}
	return nil
}

func printStyledTableCell(out io.Writer, style terminalStyle, cell styledTableCell, width int, last bool) {
	text := cell.Text
	if !last {
		text += strings.Repeat(" ", width-len(cell.Text)+2)
	}
	_, _ = fmt.Fprint(out, style.Wrap(cell.Role, text))
}

func printStyledLine(out io.Writer, style terminalStyle, role colorRole, line string) {
	suffix := ""
	if strings.HasSuffix(line, "\n") {
		line = strings.TrimSuffix(line, "\n")
		suffix = "\n"
	}
	_, _ = fmt.Fprint(out, style.Wrap(role, line), suffix)
}

func effectiveColorMode(cmd *cobra.Command) (string, error) {
	flag := cmd.Root().PersistentFlags().Lookup("color")
	if flag != nil && flag.Changed {
		return normalizeColorMode(flag.Value.String())
	}
	if value := strings.TrimSpace(os.Getenv("SANAD_COLOR")); value != "" {
		return normalizeColorMode(value)
	}
	return colorModeFromEnvironment(), nil
}

func normalizeColorMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", colorModeAuto:
		return colorModeAuto, nil
	case colorModeAlways, "1", "true", "yes", "on":
		return colorModeAlways, nil
	case colorModeNever, "0", "false", "no", "off":
		return colorModeNever, nil
	default:
		return "", fmt.Errorf("unsupported color mode %q: expected auto, always, or never", value)
	}
}

func colorModeFromEnvironment() string {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return colorModeNever
	}
	if force := strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")); force != "" && force != "0" {
		return colorModeAlways
	}
	if strings.TrimSpace(os.Getenv("CLICOLOR")) == "0" {
		return colorModeNever
	}
	return colorModeAuto
}

func effectiveColorTheme() (terminalTheme, error) {
	if value := strings.TrimSpace(os.Getenv("SANAD_COLOR_THEME")); value != "" {
		switch strings.ToLower(value) {
		case string(terminalThemeDark):
			return terminalThemeDark, nil
		case string(terminalThemeLight):
			return terminalThemeLight, nil
		case colorModeAuto:
			return detectedColorTheme(), nil
		default:
			return "", fmt.Errorf("unsupported color theme %q: expected auto, dark, or light", value)
		}
	}
	return detectedColorTheme(), nil
}

func detectedColorTheme() terminalTheme {
	fields := strings.Split(strings.TrimSpace(os.Getenv("COLORFGBG")), ";")
	if len(fields) > 0 {
		last := strings.TrimSpace(fields[len(fields)-1])
		if value, err := strconv.Atoi(last); err == nil {
			if value == 7 || value >= 10 {
				return terminalThemeLight
			}
			if value >= 0 {
				return terminalThemeDark
			}
		}
	}
	return terminalThemeDark
}

func outputSupportsColor(out io.Writer) bool {
	if !isTerminalWriter(out) {
		return false
	}
	term := strings.TrimSpace(os.Getenv("TERM"))
	return term != "" && term != "dumb"
}

func isTerminalWriter(out io.Writer) bool {
	return isTerminalFile(out)
}

func isTerminalFile(value any) bool {
	file, ok := value.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
