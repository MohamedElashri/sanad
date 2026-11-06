package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MohamedElashri/sanad/internal/sanad"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestReportTablesWrapWithoutLosingCells(t *testing.T) {
	before := sanad.Dependency{Key: "actions/checkout@v7.0.1", Commit: "sha1-0123456789abcdef0123456789abcdef01234567", Host: "github.com", Workflows: []string{".github/workflows/continuous-integration.yml"}}
	after := before
	after.Commit = "sha1-abcdef0123456789abcdef0123456789abcdef0123"
	for _, tc := range []struct {
		name    string
		report  sanad.Report
		columns []string
	}{
		{"dependencies", sanad.Report{Command: "status", Dependencies: []sanad.Dependency{before}}, []string{before.Key, before.Commit, before.Host}},
		{"decisions", sanad.Report{Command: "plan", Decisions: []sanad.Decision{{Action: "actions/checkout", From: "v7.0.1", To: "v7.1.0", Status: "pending", Reason: "waiting for the minimum observation age before updating"}}}, []string{"actions/checkout", "v7.0.1", "v7.1.0", "pending", "waiting for the minimum observation age before updating"}},
		{"changes", sanad.Report{Command: "update", Changes: []sanad.Change{{Key: before.Key, Before: &before, After: &after}}}, []string{before.Key, before.Commit, after.Commit, before.Workflows[0]}},
	} {
		for _, width := range []int{60, 80, 120} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, width), func(t *testing.T) {
				output := renderReport(tc.report, width, termenv.Ascii)
				if strings.Contains(output, "\x1b") || strings.Contains(output, "| --- |") {
					t.Fatalf("expected a plain rendered table:\n%s", output)
				}
				columns := make([]string, len(tc.columns))
				for _, line := range strings.Split(output, "\n") {
					if ansi.StringWidth(line) > width {
						t.Errorf("line exceeds width %d: %q", width, line)
					}
					if !strings.HasPrefix(line, "|") {
						continue
					}
					cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|"), "|")
					if len(cells) != len(columns) {
						continue
					}
					for i, cell := range cells {
						columns[i] += strings.Join(strings.Fields(cell), "")
					}
				}
				for i, value := range tc.columns {
					if !strings.Contains(columns[i], strings.Join(strings.Fields(value), "")) {
						t.Errorf("column %d lost %q:\n%s", i, value, output)
					}
				}
			})
		}
	}
}

func TestReportPreservesPunctuation(t *testing.T) {
	report := sanad.NewReport("plan")
	report.Decisions = []sanad.Decision{{Action: "a/root", Reason: "require `<v2 | v3>` & approval"}}
	report.Findings = []sanad.Finding{{Severity: "warning", Message: "check <policy> & `refs`"}}
	output := renderReport(report, 160, termenv.Ascii)
	for _, want := range []string{"require `<v2 | v3>` & approval", "check <policy> & `refs`"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing literal content %q:\n%s", want, output)
		}
	}
}

func TestReportFormats(t *testing.T) {
	report := sanad.NewReport("status")
	report.Dependencies = []sanad.Dependency{{Key: "a/root@v1", Commit: "sha1-" + strings.Repeat("a", 40), Host: "github.com"}}
	report.Files = []sanad.FileChange{{Path: ".github/workflows/ci.yml", Diff: "--- a/ci.yml\n+++ b/ci.yml\n@@ -1 +1 @@\n-  uses: a/root@v1\n+  uses: a/root@v2\n"}}
	for _, format := range []string{"table", "markdown", "json"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeReport(&out, report, format); err != nil {
				t.Fatal(err)
			}
			output := out.String()
			switch format {
			case "table":
				if strings.Contains(output, "| --- |") || strings.Contains(output, "\x1b") || !strings.Contains(output, "Dependency") {
					t.Fatalf("expected rendered table without ANSI:\n%s", output)
				}
				if !strings.HasSuffix(output, "\n"+report.Files[0].Diff) {
					t.Fatalf("literal diff changed:\n%s", output)
				}
			case "markdown":
				if output != report.Markdown() {
					t.Fatalf("raw Markdown changed:\n%s", output)
				}
			case "json":
				want, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				if output != string(want)+"\n" {
					t.Fatalf("JSON changed:\n%s", output)
				}
			}
		})
	}
}

func TestReportColorProfiles(t *testing.T) {
	report := sanad.NewReport("plan")
	report.Decisions = []sanad.Decision{{Action: "a/root", Status: "pending"}}
	for _, profile := range []termenv.Profile{termenv.Ascii, termenv.ANSI256} {
		output := renderReport(report, 80, profile)
		if got := strings.Contains(output, "\x1b["); got != (profile != termenv.Ascii) {
			t.Fatalf("unexpected styling for profile %v: %q", profile, output)
		}
		if !strings.Contains(ansi.Strip(output), "pending") {
			t.Fatalf("missing table content: %q", output)
		}
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestReportWriteErrors(t *testing.T) {
	want := errors.New("output closed")
	for _, format := range []string{"table", "markdown", "json"} {
		if err := writeReport(failingWriter{want}, sanad.NewReport("status"), format); !errors.Is(err, want) {
			t.Fatalf("%s: got %v, want %v", format, err, want)
		}
	}
}

func TestActualReportHasColorBeyondTables(t *testing.T) {
	report := sanad.NewReport("status")
	report.Success = true
	report.NativeSupport = "confirmed"
	report.Verification = "online"
	report.Coverage = sanad.Coverage{Locked: 1, Total: 1}
	report.Findings = []sanad.Finding{{Severity: "warning", Message: "review the observation policy"}}
	output := renderReport(report, 80, termenv.ANSI256)
	for _, want := range []string{
		"\x1b[36;1mSanad status\x1b[0m",
		"\x1b[32;1msucceeded\x1b[0m",
		"\x1b[32mconfirmed\x1b[0m",
		"\x1b[32monline\x1b[0m",
		"\x1b[32m1/1 workflows covered\x1b[0m",
		"\x1b[33;1mwarning:\x1b[0m",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("report missing semantic style %q:\n%q", want, output)
		}
	}
}

func TestReportKeepsLiteralValues(t *testing.T) {
	report := sanad.NewReport("plan")
	report.Findings = []sanad.Finding{{Severity: "warning", Message: "review __policy__ and [target](https://example.com) in a\\_path"}}
	output := renderReport(report, 160, termenv.Ascii)
	if !strings.Contains(output, report.Findings[0].Message) {
		t.Fatalf("report data was interpreted as Markdown:\n%s", output)
	}
}

func TestLongFindingsFitTerminal(t *testing.T) {
	report := sanad.NewReport("status")
	report.Findings = []sanad.Finding{{Severity: "warning", Message: strings.Repeat("long-path/", 20)}}
	for _, width := range []int{10, 40, 60} {
		for _, profile := range []termenv.Profile{termenv.Ascii, termenv.ANSI256} {
			output := renderReport(report, width, profile)
			for _, line := range strings.Split(output, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("finding exceeds width %d: %q", width, line)
				}
			}
		}
	}
}

func TestNarrowReportKeepsLabelsAndValues(t *testing.T) {
	report := sanad.NewReport("plan")
	report.Decisions = []sanad.Decision{{Action: "actions/checkout", From: "v7.0.1", To: "v7.1.0", Status: "pending", Reason: "wait for observation"}}
	for _, width := range []int{1, 10, 20, 40} {
		for _, profile := range []termenv.Profile{termenv.Ascii, termenv.ANSI256} {
			output := renderReport(report, width, profile)
			for _, line := range strings.Split(output, "\n") {
				if ansi.StringWidth(line) > width {
					t.Errorf("line exceeds width %d: %q", width, line)
				}
			}
			compact := strings.Join(strings.Fields(ansi.Strip(output)), "")
			for _, want := range []string{"Action:actions/checkout", "From:v7.0.1", "To:v7.1.0", "Decision:pending", "Reason:waitforobservation"} {
				if !strings.Contains(compact, want) {
					t.Errorf("stacked report lost %q:\n%s", want, output)
				}
			}
		}
	}
}

func TestDecisionAndFailureColors(t *testing.T) {
	report := sanad.NewReport("plan")
	report.Findings = []sanad.Finding{{Severity: "error", Message: "policy rejected this update", File: ".sanad.toml"}}
	report.Decisions = []sanad.Decision{{Action: "a/root", Status: "pending"}, {Action: "b/root", Status: "eligible"}, {Action: "c/root", Status: "frozen"}, {Action: "d/root", Status: "unchanged"}}
	for _, width := range []int{20, 100} {
		output := renderReport(report, width, termenv.ANSI)
		for _, want := range []string{"\x1b[31;1merror:\x1b[0m", "\x1b[31;1mfailed\x1b[0m", ".sanad.toml"} {
			if !strings.Contains(output, want) {
				t.Errorf("missing error context %q:\n%q", want, output)
			}
		}
		for status, code := range map[string]string{"pending": "33", "eligible": "32", "frozen": "33", "unchanged": "90"} {
			pattern := "\x1b\\[([0-9;]+)m[ \\t]*" + status
			match := regexp.MustCompile(pattern).FindStringSubmatch(output)
			if len(match) != 2 || !slices.Contains(strings.Split(match[1], ";"), code) {
				t.Errorf("%s should use color %s:\n%q", status, code, output)
			}
		}
	}
}

func TestDiffColorsPreservePatchText(t *testing.T) {
	for _, diff := range []string{
		"", "-old\n+new", "--- a/ci.yml\n+++ b/ci.yml\n@@ -1 +1 @@\n-\told  \n+\tnew  \n unchanged\n\\ No newline at end of file\n",
	} {
		for _, profile := range []termenv.Profile{termenv.Ascii, termenv.ANSI, termenv.ANSI256, termenv.TrueColor} {
			output := renderDiff(diff, profile)
			if ansi.Strip(output) != diff {
				t.Errorf("patch text changed: got %q, want %q", output, diff)
			}
			if profile == termenv.Ascii && output != diff {
				t.Errorf("unstyled patch changed: %q", output)
			}
		}
	}
	output := renderDiff("--- a/file\n+++ b/file\n@@ hunk @@\n-old\n+new\n", termenv.ANSI)
	for _, want := range []string{"\x1b[36;1m--- a/file", "\x1b[36;1m+++ b/file", "\x1b[36m@@ hunk @@", "\x1b[31m-old", "\x1b[32m+new"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing diff color %q: %q", want, output)
		}
	}
	output = renderDiff("@@ -1 +1 @@\n--- looks like a header\n+++ looks like a header\n", termenv.ANSI)
	for _, want := range []string{"\x1b[31m--- looks like a header", "\x1b[32m+++ looks like a header"} {
		if !strings.Contains(output, want) {
			t.Errorf("patch content was mistaken for a file header: %q", output)
		}
	}
}

func TestControlCharactersAreVisible(t *testing.T) {
	report := sanad.NewReport("status")
	report.Dependencies = []sanad.Dependency{{Key: "a/\x1b[2Jroot", Commit: "sha1-123", Host: "github.com"}}
	report.Findings = []sanad.Finding{{Severity: "warning", Message: "review\a\b\x1b]0;title\a", File: "a\tfile\nname"}}
	for _, profile := range []termenv.Profile{termenv.Ascii, termenv.ANSI} {
		output := ansi.Strip(renderReport(report, 160, profile))
		for _, want := range []string{"a/\\x1b[2Jroot", "review\\a\\b\\x1b]0;title\\a", "a\\tfile\\nname"} {
			if !strings.Contains(output, want) {
				t.Errorf("control characters not visible: want %q in %q", want, output)
			}
		}
		if strings.ContainsAny(output, "\x1b\a\b\t") {
			t.Errorf("report contains terminal control characters: %q", output)
		}
	}
	diff := "-old\x1b[2J\n+new\a\n"
	if output := ansi.Strip(renderDiff(diff, termenv.ANSI)); output != "-old\\x1b[2J\n+new\\a\n" {
		t.Errorf("diff control characters not visible: %q", output)
	}
}

func TestRedirectedOutputStaysPlainEvenWithForcedColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	report := sanad.NewReport("plan")
	report.Decisions = []sanad.Decision{{Action: "a/root", Status: "pending"}}
	var buffer bytes.Buffer
	file, err := os.CreateTemp(t.TempDir(), "report")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, output := range []io.Writer{&buffer, file} {
		width, profile := terminalOptions(output)
		if width != 80 || profile != termenv.Ascii {
			t.Errorf("redirected output got width %d and profile %v", width, profile)
		}
		if err := writeReport(output, report, "table"); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(buffer.String(), "\x1b") {
		t.Fatalf("forced color leaked into redirected report: %q", buffer.String())
	}
}

func TestReportWrapsUnicodeWithoutLosingText(t *testing.T) {
	report := sanad.NewReport("plan")
	message := "review 日本語 حزمة cafe\u0301 👩‍💻 before updating"
	report.Decisions = []sanad.Decision{{Action: "a/root", Status: "pending", Reason: message}}
	for _, width := range []int{10, 40, 80} {
		output := renderReport(report, width, termenv.ANSI256)
		for _, line := range strings.Split(output, "\n") {
			if ansi.StringWidth(line) > width {
				t.Errorf("Unicode line exceeds width %d: %q", width, line)
			}
		}
		plain := strings.Map(func(r rune) rune {
			if strings.ContainsRune("│─┼┌┐└┘┬┴", r) {
				return -1
			}
			return r
		}, ansi.Strip(output))
		compact := strings.Join(strings.Fields(plain), "")
		if !strings.Contains(compact, strings.Join(strings.Fields(message), "")) {
			t.Errorf("Unicode table cell lost text:\n%s", output)
		}
	}
}
