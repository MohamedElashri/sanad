package policy

import "time"

type DecisionKind string

const (
	DecisionUnchanged         DecisionKind = "unchanged"
	DecisionUpdate            DecisionKind = "update"
	DecisionPending           DecisionKind = "pending-cooldown"
	DecisionSkip              DecisionKind = "skip"
	DecisionError             DecisionKind = "error"
	DecisionSkipLocalAction   DecisionKind = "skip-local-action"
	DecisionSkipDockerAction  DecisionKind = "skip-docker-action"
	DecisionSkipIgnored       DecisionKind = "skip-ignored"
	DecisionErrorInvalid      DecisionKind = "error-invalid"
	DecisionErrorUnpinned     DecisionKind = "error-unpinned"
	DecisionErrorShortSHA     DecisionKind = "error-short-sha"
	DecisionErrorTagDenied    DecisionKind = "error-tag-denied"
	DecisionErrorBranchDenied DecisionKind = "error-branch-denied"
	DecisionErrorReusable     DecisionKind = "error-reusable-workflow-denied"
	DecisionErrorUnresolved   DecisionKind = "error-unresolved"
	DecisionErrorUnsupported  DecisionKind = "error-unsupported-policy"
)

type Decision struct {
	Kind       DecisionKind  `json:"kind"`
	Reason     string        `json:"reason,omitempty"`
	CurrentSHA string        `json:"current_sha,omitempty"`
	NewSHA     string        `json:"new_sha,omitempty"`
	LogicalRef string        `json:"logical_ref,omitempty"`
	Age        time.Duration `json:"age,omitempty"`
}
