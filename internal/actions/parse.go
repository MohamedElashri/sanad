package actions

import (
	"fmt"
	"path"
	"strings"
)

type ActionKind string

const (
	KindGitHubAction     ActionKind = "github-action"
	KindReusableWorkflow ActionKind = "reusable-workflow"
	KindLocalAction      ActionKind = "local-action"
	KindDockerAction     ActionKind = "docker-action"
	KindInvalid          ActionKind = "invalid"
)

type ParsedAction struct {
	Raw    string     `json:"raw"`
	Owner  string     `json:"owner,omitempty"`
	Repo   string     `json:"repo,omitempty"`
	Path   string     `json:"path,omitempty"`
	Ref    string     `json:"ref,omitempty"`
	Kind   ActionKind `json:"kind"`
	Pinned bool       `json:"pinned"`
	Valid  bool       `json:"valid"`
	Error  string     `json:"error,omitempty"`
}

func Parse(raw string) ParsedAction {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return invalid(raw, "empty action reference")
	}

	switch {
	case isLocalAction(raw):
		return ParsedAction{
			Raw:   raw,
			Path:  raw,
			Kind:  KindLocalAction,
			Valid: true,
		}
	case strings.HasPrefix(raw, "docker://"):
		image := strings.TrimPrefix(raw, "docker://")
		if image == "" {
			return invalid(raw, "docker action reference is missing an image")
		}
		return ParsedAction{
			Raw:   raw,
			Path:  image,
			Kind:  KindDockerAction,
			Valid: true,
		}
	case strings.Contains(raw, "://"):
		return invalid(raw, "unsupported action reference scheme")
	}

	selector, ref, hasRef := splitSelectorRef(raw)
	parts := strings.Split(selector, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return invalid(raw, "GitHub action reference must include owner and repo")
	}
	if !validNameSegment(parts[0]) || !validNameSegment(parts[1]) {
		return invalid(raw, "GitHub action owner or repo contains invalid characters")
	}

	actionPath := ""
	if len(parts) > 2 {
		if !hasRef {
			return invalid(raw, "GitHub action path references must include @ref")
		}
		if hasEmptySegment(parts[2:]) {
			return invalid(raw, "GitHub action path contains an empty segment")
		}
		actionPath = strings.Join(parts[2:], "/")
	}

	if hasRef && ref == "" {
		return invalid(raw, "GitHub action reference has empty ref")
	}
	if !hasRef && len(parts) != 2 {
		return invalid(raw, "GitHub action reference is missing @ref")
	}

	kind := KindGitHubAction
	if isReusableWorkflowPath(actionPath) {
		kind = KindReusableWorkflow
	}

	if hasRef && IsShortSHA(ref) {
		return ParsedAction{
			Raw:   raw,
			Owner: parts[0],
			Repo:  parts[1],
			Path:  actionPath,
			Ref:   ref,
			Kind:  KindInvalid,
			Valid: false,
			Error: "short SHA refs are not accepted",
		}
	}

	return ParsedAction{
		Raw:    raw,
		Owner:  parts[0],
		Repo:   parts[1],
		Path:   actionPath,
		Ref:    ref,
		Kind:   kind,
		Pinned: IsFullSHA(ref),
		Valid:  true,
	}
}

func splitSelectorRef(raw string) (string, string, bool) {
	at := strings.LastIndexByte(raw, '@')
	if at < 0 {
		return raw, "", false
	}
	return raw[:at], raw[at+1:], true
}

func isLocalAction(raw string) bool {
	return raw == "." || strings.HasPrefix(raw, "./")
}

func validNameSegment(segment string) bool {
	for _, r := range segment {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func hasEmptySegment(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			return true
		}
	}
	return false
}

func isReusableWorkflowPath(actionPath string) bool {
	if !strings.HasPrefix(actionPath, ".github/workflows/") {
		return false
	}
	ext := strings.ToLower(path.Ext(actionPath))
	return ext == ".yml" || ext == ".yaml"
}

func IsFullSHA(ref string) bool {
	return len(ref) == 40 && isHex(ref)
}

func IsShortSHA(ref string) bool {
	return len(ref) >= 7 && len(ref) < 40 && isHex(ref)
}

func isHex(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return value != ""
}

func invalid(raw string, format string, args ...any) ParsedAction {
	return ParsedAction{
		Raw:   raw,
		Kind:  KindInvalid,
		Valid: false,
		Error: fmt.Sprintf(format, args...),
	}
}
