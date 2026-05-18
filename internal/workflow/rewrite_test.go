package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/policy"
)

const (
	rewriteOldSHA   = "1111111111111111111111111111111111111111"
	rewriteNewSHA   = "2222222222222222222222222222222222222222"
	rewriteThirdSHA = "3333333333333333333333333333333333333333"
)

func TestRewriteWorkflowBytesPinsMutableTagAndAddsMetadata(t *testing.T) {
	input := strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      - uses: actions/checkout@v4",
		"      - run: echo ok",
		"",
	}, "\n")
	want := strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      - uses: actions/checkout@" + rewriteNewSHA + " # sanad: ref=v4",
		"      - run: echo ok",
		"",
	}, "\n")

	got := rewriteOne(t, input, 0, rewriteNewSHA, "v4")
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesUpdatesManagedPinnedSHA(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteOldSHA + " # sanad: ref=v4\n"
	want := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteNewSHA + " # sanad: ref=v4\n"

	got := rewriteOne(t, input, 0, rewriteNewSHA, "v4")
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesPreservesQuotesAndUnrelatedComments(t *testing.T) {
	input := strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      # keep this comment",
		"      - uses: \"actions/checkout@v4\" # important",
		"",
	}, "\n")
	want := strings.Join([]string{
		"jobs:",
		"  test:",
		"    steps:",
		"      # keep this comment",
		"      - uses: \"actions/checkout@" + rewriteNewSHA + "\" # important; sanad: ref=v4",
		"",
	}, "\n")

	got := rewriteOne(t, input, 0, rewriteNewSHA, "v4")
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesUpdatesExistingMetadataComment(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteOldSHA + " # keep; sanad: ref=v3\n"
	want := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteNewSHA + " # keep; sanad: ref=v4\n"

	got := rewriteOne(t, input, 0, rewriteNewSHA, "v4")
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesPreservesCRLF(t *testing.T) {
	input := "jobs:\r\n  test:\r\n    steps:\r\n      - uses: actions/checkout@v4\r\n"
	want := "jobs:\r\n  test:\r\n    steps:\r\n      - uses: actions/checkout@" + rewriteNewSHA + " # sanad: ref=v4\r\n"

	got := rewriteOne(t, input, 0, rewriteNewSHA, "v4")
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%q\ngot:\n%q", want, string(got))
	}
}

func TestRewriteWorkflowBytesOmitsMetadataCommentWhenDisabled(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"
	want := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteNewSHA + "\n"

	uses := extractUsesFromBytesForRewriteTest(t, input)
	change := rewriteChangeForUse(uses[0], rewriteNewSHA, "v4")
	got, err := RewriteWorkflowBytes([]byte(input), []RewriteChange{change}, RewriteOptions{WriteMetadataComment: false})
	if err != nil {
		t.Fatalf("RewriteWorkflowBytes returned error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesRemovesExistingMetadataCommentWhenDisabled(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteOldSHA + " # keep; sanad: ref=v3\n"
	want := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteNewSHA + " # keep\n"

	uses := extractUsesFromBytesForRewriteTest(t, input)
	change := rewriteChangeForUse(uses[0], rewriteNewSHA, "v4")
	got, err := RewriteWorkflowBytes([]byte(input), []RewriteChange{change}, RewriteOptions{WriteMetadataComment: false})
	if err != nil {
		t.Fatalf("RewriteWorkflowBytes returned error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestRewriteWorkflowBytesRemovesOnlyMetadataCommentWhenDisabled(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteOldSHA + " # sanad: ref=v3\n"
	want := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@" + rewriteNewSHA + "\n"

	uses := extractUsesFromBytesForRewriteTest(t, input)
	change := rewriteChangeForUse(uses[0], rewriteNewSHA, "v4")
	got, err := RewriteWorkflowBytes([]byte(input), []RewriteChange{change}, RewriteOptions{WriteMetadataComment: false})
	if err != nil {
		t.Fatalf("RewriteWorkflowBytes returned error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("rewritten workflow mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}

func TestApplySourceEditsRefusesOverlaps(t *testing.T) {
	_, err := ApplySourceEdits([]byte("abcdef"), []SourceEdit{
		{Start: 1, End: 4, Replacement: []byte("x")},
		{Start: 3, End: 5, Replacement: []byte("y")},
	})
	if err == nil {
		t.Fatal("ApplySourceEdits returned nil error, want overlap error")
	}
	if !strings.Contains(err.Error(), "overlapping edits") {
		t.Fatalf("error = %q, want overlapping edits", err)
	}
}

func TestRewriteWorkflowBytesRefusesInvalidOutputYAML(t *testing.T) {
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"
	uses := extractUsesFromBytesForRewriteTest(t, input)
	change := RewriteChange{
		Use: uses[0],
		Action: actions.ParsedAction{
			Raw:   uses[0].Raw,
			Owner: "bad: [",
			Repo:  "repo",
			Kind:  actions.KindGitHubAction,
			Valid: true,
		},
		Decision: policy.Decision{
			Kind:       policy.DecisionUpdate,
			NewSHA:     rewriteNewSHA,
			LogicalRef: "v4",
		},
	}

	_, err := RewriteWorkflowBytes([]byte(input), []RewriteChange{change}, RewriteOptions{WriteMetadataComment: true})
	if err == nil {
		t.Fatal("RewriteWorkflowBytes returned nil error, want invalid YAML error")
	}
	if !strings.Contains(err.Error(), "resulting YAML is invalid") {
		t.Fatalf("error = %q, want invalid YAML", err)
	}
}

func TestRewriteWorkflowFileWritesAtomicallyAndPreservesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ci.yml")
	input := "jobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n"
	if err := os.WriteFile(path, []byte(input), 0o640); err != nil {
		t.Fatal(err)
	}

	uses, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	change := rewriteChangeForUse(uses[0], rewriteNewSHA, "v4")
	if err := RewriteWorkflowFile(path, []RewriteChange{change}, RewriteOptions{WriteMetadataComment: true}); err != nil {
		t.Fatalf("RewriteWorkflowFile returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "actions/checkout@"+rewriteNewSHA+" # sanad: ref=v4") {
		t.Fatalf("workflow was not rewritten as expected:\n%s", string(got))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions = %v, want 0640", info.Mode().Perm())
	}
}

func TestRewriteWorkflowBytesGolden(t *testing.T) {
	tests := []struct {
		name    string
		changes func(t *testing.T, input []byte) []RewriteChange
	}{
		{
			name: "mutable-tag",
			changes: func(t *testing.T, input []byte) []RewriteChange {
				t.Helper()
				uses := extractUsesFromGoldenBytes(t, input)
				return []RewriteChange{
					rewriteChangeForUse(uses[0], rewriteNewSHA, "v4"),
					rewriteChangeForUse(uses[1], rewriteThirdSHA, "v5"),
				}
			},
		},
		{
			name: "managed-pin",
			changes: func(t *testing.T, input []byte) []RewriteChange {
				t.Helper()
				uses := extractUsesFromGoldenBytes(t, input)
				return []RewriteChange{rewriteChangeForUse(uses[0], rewriteNewSHA, "v4")}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := readTestdata(t, "rewrite", tt.name+".in.yml")
			want := readTestdata(t, "rewrite", tt.name+".golden.yml")
			got, err := RewriteWorkflowBytes(input, tt.changes(t, input), RewriteOptions{WriteMetadataComment: true})
			if err != nil {
				t.Fatalf("RewriteWorkflowBytes returned error: %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("golden rewrite mismatch\nwant:\n%s\ngot:\n%s", string(want), string(got))
			}
		})
	}
}

func rewriteOne(t *testing.T, input string, useIndex int, newSHA string, logicalRef string) []byte {
	t.Helper()

	uses := extractUsesFromBytesForRewriteTest(t, input)
	if useIndex >= len(uses) {
		t.Fatalf("use index %d outside %d uses", useIndex, len(uses))
	}
	change := rewriteChangeForUse(uses[useIndex], newSHA, logicalRef)
	got, err := RewriteWorkflowBytes([]byte(input), []RewriteChange{change}, RewriteOptions{WriteMetadataComment: true})
	if err != nil {
		t.Fatalf("RewriteWorkflowBytes returned error: %v", err)
	}
	return got
}

func extractUsesFromGoldenBytes(t *testing.T, input []byte) []UseNode {
	t.Helper()

	path := filepath.Join(t.TempDir(), "golden.yml")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	uses, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return uses
}

func readTestdata(t *testing.T, parts ...string) []byte {
	t.Helper()

	path := filepath.Join(append([]string{"testdata"}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rewriteChangeForUse(use UseNode, newSHA string, logicalRef string) RewriteChange {
	return RewriteChange{
		Use:    use,
		Action: actions.Parse(use.Raw),
		Decision: policy.Decision{
			Kind:       policy.DecisionUpdate,
			NewSHA:     newSHA,
			LogicalRef: logicalRef,
		},
	}
}

func extractUsesFromBytesForRewriteTest(t *testing.T, content string) []UseNode {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sanad-rewrite-test.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	uses, err := ExtractUsesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return uses
}
