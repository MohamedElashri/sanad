package sanad

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

type Dependency struct {
	Key       string   `json:"key"`
	Host      string   `json:"host"`
	Commit    string   `json:"commit"`
	OwnerID   int64    `json:"owner_id"`
	RepoID    int64    `json:"repo_id"`
	Uses      []string `json:"uses"`
	Workflows []string `json:"workflows"`
}

type Change struct {
	Key    string      `json:"key"`
	Before *Dependency `json:"before,omitempty"`
	After  *Dependency `json:"after,omitempty"`
}

type Decision struct {
	Action     string     `json:"action"`
	From       string     `json:"from,omitempty"`
	To         string     `json:"to,omitempty"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason"`
	EligibleAt *time.Time `json:"eligible_at,omitempty"`
	Changes    []Change   `json:"changes,omitempty"`
}

type FileChange struct {
	Path string `json:"path"`
	Diff string `json:"diff"`
}

type Report struct {
	Version         int          `json:"version"`
	Command         string       `json:"command"`
	Success         bool         `json:"success"`
	BackendVersion  string       `json:"backend_version"`
	ParserVersion   string       `json:"parser_version"`
	LockfileVersion string       `json:"lockfile_version"`
	Verification    string       `json:"verification"`
	NativeSupport   string       `json:"native_support"`
	Coverage        Coverage     `json:"coverage"`
	Findings        []Finding    `json:"findings"`
	Dependencies    []Dependency `json:"dependencies"`
	Changes         []Change     `json:"changes"`
	Decisions       []Decision   `json:"decisions"`
	Files           []FileChange `json:"files"`
	Changed         bool         `json:"changed"`
}

type Coverage struct {
	Total     int      `json:"total"`
	Locked    int      `json:"locked"`
	Uncovered []string `json:"uncovered"`
}

func NewReport(command string) Report {
	return Report{Version: 1, Command: command, BackendVersion: BackendVersion, ParserVersion: "v0.0.6", LockfileVersion: "v0.0.3", Verification: "none", NativeSupport: "unknown", Coverage: Coverage{Uncovered: []string{}}, Findings: []Finding{}, Dependencies: []Dependency{}, Changes: []Change{}, Decisions: []Decision{}, Files: []FileChange{}}
}

func (r *Report) finding(code, severity, message string) {
	r.Findings = append(r.Findings, Finding{Code: code, Severity: severity, Message: message})
}
func (r Report) Blocked() bool {
	for _, f := range r.Findings {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "`", "&#96;", "\n", " ", "\r", " ").Replace(s)
}

func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Sanad %s: native support %s; verification %s.\n", escape(r.Command), escape(r.NativeSupport), escape(r.Verification))
	fmt.Fprintf(&b, "%d/%d workflows covered; %d locked dependencies.\n", r.Coverage.Locked, r.Coverage.Total, len(r.Dependencies))
	if (r.Command == "status" || r.Command == "doctor") && len(r.Dependencies) > 0 {
		b.WriteString("\n| Dependency | Commit | Host |\n| --- | --- | --- |\n")
		for _, d := range r.Dependencies {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", escape(d.Key), escape(d.Commit), escape(d.Host))
		}
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "\n- %s: %s\n", escape(f.Severity), escape(f.Message))
	}
	if len(r.Decisions) > 0 {
		b.WriteString("\n| Action | From | To | Decision | Reason |\n| --- | --- | --- | --- | --- |\n")
		for _, d := range r.Decisions {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", escape(d.Action), escape(d.From), escape(d.To), escape(d.Status), escape(d.Reason))
		}
	}
	if len(r.Changes) > 0 {
		b.WriteString("\n| Dependency | Before | After | Affected workflows |\n| --- | --- | --- | --- |\n")
		for _, c := range r.Changes {
			old, next := "—", "—"
			paths := []string{}
			if c.Before != nil {
				old = c.Before.Commit
				paths = append(paths, c.Before.Workflows...)
			}
			if c.After != nil {
				next = c.After.Commit
				paths = append(paths, c.After.Workflows...)
			}
			sort.Strings(paths)
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", escape(c.Key), escape(old), escape(next), escape(strings.Join(unique(paths), ", ")))
		}
	}
	if len(r.Files) > 0 {
		b.WriteString("\nFiles:\n")
		for _, f := range r.Files {
			fmt.Fprintf(&b, "\n- %s\n", escape(f.Path))
		}
	}
	return b.String()
}

func unique(items []string) []string {
	out := []string{}
	for _, s := range items {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
