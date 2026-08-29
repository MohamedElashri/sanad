package cli

import (
	"encoding/json"
	"testing"

	"github.com/MohamedElashri/sanad/internal/actions"
	"github.com/MohamedElashri/sanad/internal/policy"
)

func TestCheckReportV1JSONContract(t *testing.T) {
	report := checkReport{
		Version: checkReportVersion,
		Passed:  false,
		Summary: checkSummary{Checked: 2, Violations: 1, Updates: 1, Pending: 0, Skipped: 1},
		Violations: []checkViolation{{
			File: ".github/workflows/ci.yml", NodePath: "jobs.test.steps[0].uses", Line: 10, Column: 15,
			Raw: "actions/checkout@v4", Action: "actions/checkout", Decision: policy.DecisionUpdate,
			ReasonCode: "mutable-ref", Reason: "mutable action reference", CandidateSHA: "abc", LogicalRef: "v4",
		}},
	}
	assertJSONContract(t, report, `{"version":1,"passed":false,"summary":{"checked":2,"violations":1,"updates":1,"pending_cooldown":0,"skipped":1},"violations":[{"file":".github/workflows/ci.yml","node_path":"jobs.test.steps[0].uses","line":10,"column":15,"raw":"actions/checkout@v4","action":"actions/checkout","decision":"update","reason_code":"mutable-ref","reason":"mutable action reference","candidate_sha":"abc","logical_ref":"v4"}]}`)
}

func TestAutomationReportsEncodeEmptyCollectionsAsArrays(t *testing.T) {
	assertJSONContract(t, checkReport{Version: checkReportVersion, Passed: true, Violations: []checkViolation{}}, `{"version":1,"passed":true,"summary":{"checked":0,"violations":0,"updates":0,"pending_cooldown":0,"skipped":0},"violations":[]}`)
	assertJSONContract(t, planReport{Version: planReportVersion, Files: []planFile{}}, `{"version":1,"summary":{"actions":0,"already_pinned":0,"updates_available":0,"pending_cooldown":0,"policy_violations":0,"skipped":0},"files":[]}`)
	assertJSONContract(t, upgradeReport{Version: upgradeReportVersion, Actions: []upgradeAction{}}, `{"version":2,"summary":{"matched":0,"updates":0,"pending_cooldown":0,"unchanged":0,"blocked":0},"actions":[]}`)
}

func TestPlanReportV1JSONContract(t *testing.T) {
	report := planReport{
		Version: planReportVersion,
		Summary: planSummary{Actions: 1, UpdatesAvailable: 1},
		Files: []planFile{{Path: ".github/workflows/ci.yml", Actions: []planAction{{
			NodePath: "jobs.test.steps[0].uses", Line: 10, Column: 15, Raw: "actions/checkout@v4",
			Owner: "actions", Repo: "checkout", Kind: actions.KindGitHubAction, LogicalRef: "v4",
			CandidateSHA: "abc", CandidateRefKind: "tag", Decision: policy.DecisionUpdate,
			ReasonCode: "mutable-ref", Reason: "mutable action reference", Age: "10d", AgeSeconds: 864000,
		}}}},
	}
	assertJSONContract(t, report, `{"version":1,"summary":{"actions":1,"already_pinned":0,"updates_available":1,"pending_cooldown":0,"policy_violations":0,"skipped":0},"files":[{"path":".github/workflows/ci.yml","actions":[{"node_path":"jobs.test.steps[0].uses","line":10,"column":15,"raw":"actions/checkout@v4","owner":"actions","repo":"checkout","kind":"github-action","logical_ref":"v4","candidate_sha":"abc","candidate_ref_kind":"tag","decision":"update","reason_code":"mutable-ref","reason":"mutable action reference","age":"10d","age_seconds":864000}]}]}`)
}

func TestUpgradeReportV2JSONContract(t *testing.T) {
	report := upgradeReport{
		Version: upgradeReportVersion,
		Summary: upgradeSummary{Matched: 1, Updates: 1},
		Actions: []upgradeAction{{
			File: ".github/workflows/ci.yml", NodePath: "jobs.test.steps[0].uses", Line: 10,
			Action: "actions/checkout", CurrentLogicalRef: "v4", TargetLogicalRef: "v5",
			CurrentSHA: "old", CandidateSHA: "new", Decision: policy.DecisionUpdate,
			ReasonCode: "upgrade", Reason: "new release", Age: "10d", Level: "major",
			Selection: "latest-eligible", SelectedRelease: "v5.0.0",
			Candidates: []upgradeCandidate{{LogicalRef: "v5.0.0", Version: "5.0.0", SHA: "new", Decision: policy.DecisionUpdate, Reason: "eligible", Age: "10d"}},
		}},
	}
	assertJSONContract(t, report, `{"version":2,"summary":{"matched":1,"updates":1,"pending_cooldown":0,"unchanged":0,"blocked":0},"actions":[{"file":".github/workflows/ci.yml","node_path":"jobs.test.steps[0].uses","line":10,"action":"actions/checkout","current_logical_ref":"v4","target_logical_ref":"v5","current_sha":"old","candidate_sha":"new","decision":"update","reason_code":"upgrade","reason":"new release","age":"10d","level":"major","selection":"latest-eligible","selected_release":"v5.0.0","candidates":[{"logical_ref":"v5.0.0","version":"5.0.0","sha":"new","decision":"update","reason":"eligible","age":"10d"}]}]}`)
}

func assertJSONContract(t *testing.T, value any, want string) {
	t.Helper()
	got, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if string(got) != want {
		t.Fatalf("JSON contract changed\n got: %s\nwant: %s", got, want)
	}
}
