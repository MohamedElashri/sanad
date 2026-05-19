package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/policy"
	"gopkg.in/yaml.v3"
)

type RewriteOptions struct {
	WriteMetadataComment bool
}

type RewriteChange struct {
	Use      UseNode
	Action   actions.ParsedAction
	Decision policy.Decision
}

type SourceEdit struct {
	Start       int
	End         int
	Replacement []byte
}

func RewriteWorkflowBytes(data []byte, changes []RewriteChange, opts RewriteOptions) ([]byte, error) {
	edits, err := BuildSourceEdits(data, changes, opts)
	if err != nil {
		return nil, err
	}
	rewritten, err := ApplySourceEdits(data, edits)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(rewritten, &doc); err != nil {
		return nil, fmt.Errorf("refusing rewrite: resulting YAML is invalid: %w", err)
	}
	return rewritten, nil
}

func BuildSourceEdits(data []byte, changes []RewriteChange, opts RewriteOptions) ([]SourceEdit, error) {
	lines := indexLines(data)
	edits := make([]SourceEdit, 0, len(changes))
	for _, change := range changes {
		if change.Decision.Kind != policy.DecisionUpdate {
			continue
		}
		edit, err := buildSourceEdit(data, lines, change, opts)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	if err := validateNonOverlappingEdits(edits); err != nil {
		return nil, err
	}
	return edits, nil
}

func ApplySourceEdits(data []byte, edits []SourceEdit) ([]byte, error) {
	if err := validateNonOverlappingEdits(edits); err != nil {
		return nil, err
	}
	ordered := append([]SourceEdit(nil), edits...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Start > ordered[j].Start
	})

	out := append([]byte(nil), data...)
	for _, edit := range ordered {
		if edit.Start < 0 || edit.End < edit.Start || edit.End > len(out) {
			return nil, fmt.Errorf("invalid edit range [%d,%d)", edit.Start, edit.End)
		}
		next := make([]byte, 0, len(out)-(edit.End-edit.Start)+len(edit.Replacement))
		next = append(next, out[:edit.Start]...)
		next = append(next, edit.Replacement...)
		next = append(next, out[edit.End:]...)
		out = next
	}
	return out, nil
}

func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	file, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("write workflow %q: create temp file: %w", path, err)
	}
	tempName := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if err := file.Chmod(perm); err != nil {
		_ = file.Close()
		return fmt.Errorf("write workflow %q: chmod temp file: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write workflow %q: write temp file: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("write workflow %q: sync temp file: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write workflow %q: close temp file: %w", path, err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("write workflow %q: rename temp file: %w", path, err)
	}
	cleanup = false
	return nil
}

func buildSourceEdit(data []byte, lines []lineSpan, change RewriteChange, opts RewriteOptions) (SourceEdit, error) {
	if !actions.IsFullSHA(change.Decision.NewSHA) {
		return SourceEdit{}, fmt.Errorf("%s:%d: update decision must include a full 40-character SHA", change.Use.File, change.Use.Line)
	}
	if change.Decision.LogicalRef == "" && opts.WriteMetadataComment {
		return SourceEdit{}, fmt.Errorf("%s:%d: metadata comment requires a logical ref", change.Use.File, change.Use.Line)
	}
	if change.Use.LineIndex < 0 || change.Use.LineIndex >= len(lines) {
		return SourceEdit{}, fmt.Errorf("%s:%d: line is outside source file", change.Use.File, change.Use.Line)
	}

	line := lines[change.Use.LineIndex]
	lineBytes := data[line.Start:line.End]
	valueStart, valueEnd, err := findValueSpan(lineBytes, change.Use)
	if err != nil {
		return SourceEdit{}, err
	}

	newRaw := rewriteRawReference(change.Action, change.Decision.NewSHA)
	tail := append([]byte(nil), lineBytes[valueEnd:]...)
	if opts.WriteMetadataComment {
		tail = updateMetadataComment(tail, change.Decision.LogicalRef)
	} else {
		tail = removeMetadataComment(tail)
	}

	replacement := make([]byte, 0, len(newRaw)+len(tail))
	replacement = append(replacement, newRaw...)
	replacement = append(replacement, tail...)
	return SourceEdit{
		Start:       line.Start + valueStart,
		End:         line.End,
		Replacement: replacement,
	}, nil
}

func rewriteRawReference(action actions.ParsedAction, sha string) []byte {
	selector := action.Owner + "/" + action.Repo
	if action.Path != "" {
		selector += "/" + action.Path
	}
	return []byte(selector + "@" + sha)
}

func findValueSpan(line []byte, use UseNode) (int, int, error) {
	raw := []byte(use.Raw)
	if len(raw) == 0 {
		return 0, 0, fmt.Errorf("%s:%d: empty uses value cannot be rewritten", use.File, use.Line)
	}

	col := use.Column - 1
	for _, start := range []int{col, col + 1, col - 1} {
		if start >= 0 && start+len(raw) <= len(line) && bytes.Equal(line[start:start+len(raw)], raw) {
			return start, start + len(raw), nil
		}
	}

	key := bytes.Index(line, []byte("uses:"))
	searchFrom := 0
	if key >= 0 {
		searchFrom = key + len("uses:")
	}
	idx := bytes.Index(line[searchFrom:], raw)
	if idx >= 0 {
		start := searchFrom + idx
		return start, start + len(raw), nil
	}
	return 0, 0, fmt.Errorf("%s:%d: could not locate uses value %q in source line", use.File, use.Line, use.Raw)
}

var sanadCommentPattern = regexp.MustCompile(`sanad:\s*ref=("[^"]*"|'[^']*'|\S+)`)

func updateMetadataComment(tail []byte, logicalRef string) []byte {
	commentStart := findInlineCommentStart(tail)
	metadataText := []byte("sanad: ref=" + logicalRef)
	if commentStart < 0 {
		return append(append([]byte(nil), tail...), append([]byte(" # "), metadataText...)...)
	}

	before := append([]byte(nil), tail[:commentStart]...)
	comment := string(tail[commentStart:])
	if sanadCommentPattern.MatchString(comment) {
		return append(before, []byte(sanadCommentPattern.ReplaceAllString(comment, string(metadataText)))...)
	}

	trimmedRight := strings.TrimRight(comment, " \t")
	trailing := comment[len(trimmedRight):]
	separator := "; "
	if strings.TrimSpace(trimmedRight) == "#" {
		separator = " "
	}
	return append(before, []byte(trimmedRight+separator+string(metadataText)+trailing)...)
}

func removeMetadataComment(tail []byte) []byte {
	commentStart := findInlineCommentStart(tail)
	if commentStart < 0 {
		return append([]byte(nil), tail...)
	}

	before := append([]byte(nil), tail[:commentStart]...)
	comment := string(tail[commentStart:])
	trimmedRight := strings.TrimRight(comment, " \t")
	trailing := comment[len(trimmedRight):]

	loc := sanadCommentPattern.FindStringIndex(trimmedRight)
	if loc == nil {
		return append(before, []byte(comment)...)
	}

	left := strings.TrimRight(trimmedRight[:loc[0]], " \t")
	right := strings.TrimLeft(trimmedRight[loc[1]:], " \t")
	if strings.HasSuffix(left, ";") {
		left = strings.TrimRight(strings.TrimSuffix(left, ";"), " \t")
	}
	if strings.HasPrefix(right, ";") {
		right = strings.TrimLeft(strings.TrimPrefix(right, ";"), " \t")
	}

	leftTrimmed := strings.TrimSpace(left)
	rightTrimmed := strings.TrimSpace(right)
	if leftTrimmed == "#" && rightTrimmed == "" {
		return bytes.TrimRight(before, " \t")
	}
	if leftTrimmed == "#" {
		return append(before, []byte(left+" "+right+trailing)...)
	}
	if rightTrimmed == "" {
		return append(before, []byte(left+trailing)...)
	}
	return append(before, []byte(left+"; "+right+trailing)...)
}

func findInlineCommentStart(tail []byte) int {
	inSingle := false
	inDouble := false
	for i, b := range tail {
		switch b {
		case '\'':
			if i == 0 {
				continue
			}
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if i == 0 {
				continue
			}
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || tail[i-1] == ' ' || tail[i-1] == '\t') {
				return i
			}
		}
	}
	return -1
}

func validateNonOverlappingEdits(edits []SourceEdit) error {
	ordered := append([]SourceEdit(nil), edits...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Start < ordered[j].Start
	})
	for i, edit := range ordered {
		if edit.Start < 0 || edit.End < edit.Start {
			return fmt.Errorf("invalid edit range [%d,%d)", edit.Start, edit.End)
		}
		if i > 0 && edit.Start < ordered[i-1].End {
			return fmt.Errorf("overlapping edits [%d,%d) and [%d,%d)", ordered[i-1].Start, ordered[i-1].End, edit.Start, edit.End)
		}
	}
	return nil
}

type lineSpan struct {
	Start int
	End   int
}

func indexLines(data []byte) []lineSpan {
	var lines []lineSpan
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		end := i
		if end > start && data[end-1] == '\r' {
			end--
		}
		lines = append(lines, lineSpan{Start: start, End: end})
		start = i + 1
	}
	if start < len(data) {
		lines = append(lines, lineSpan{Start: start, End: len(data)})
	}
	return lines
}
