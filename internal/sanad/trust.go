package sanad

import (
	"fmt"
	"strings"
)

type SourceIdentity struct {
	OwnerID int64 `toml:"owner_id" json:"owner_id"`
	RepoID  int64 `toml:"repo_id" json:"repo_id"`
}
type TrustPolicy struct {
	RequireApproval bool                      `toml:"require_approval" json:"require_approval"`
	Repositories    map[string]SourceIdentity `toml:"repositories" json:"repositories,omitempty"`
}

func validateTrust(p *TrustPolicy) error {
	normalized := map[string]SourceIdentity{}
	for source, identity := range p.Repositories {
		parts := strings.Split(source, "/")
		if len(parts) != 3 || strings.ToLower(parts[0]) != "github.com" && !validTenant(strings.ToLower(parts[0])) || !validSegment(parts[1]) || !validSegment(parts[2]) || identity.OwnerID <= 0 || identity.RepoID <= 0 {
			return fmt.Errorf("invalid trusted repository %q; use host/owner/repo and positive numeric IDs", source)
		}
		key := strings.ToLower(source)
		if _, ok := normalized[key]; ok {
			return fmt.Errorf("duplicate trusted repository %s", source)
		}
		normalized[key] = identity
	}
	p.Repositories = normalized
	return nil
}
func trustFindings(g graph, policy TrustPolicy) []Finding {
	out := []Finding{}
	for _, d := range g.list() {
		source := strings.ToLower(d.Host) + "/" + repositoryName(d.Key)
		expected, ok := policy.Repositories[source]
		if !ok {
			if policy.RequireApproval {
				out = append(out, Finding{Code: "source-unapproved", Severity: "error", Message: fmt.Sprintf("%s introduces unapproved source %s (owner %d, repository %d); review and approve its identity in trusted policy", d.Key, source, d.OwnerID, d.RepoID)})
			}
			continue
		}
		if expected.OwnerID != d.OwnerID || expected.RepoID != d.RepoID {
			out = append(out, Finding{Code: "source-identity-changed", Severity: "error", Message: fmt.Sprintf("%s identity differs from approved %s (expected owner %d, repository %d; got owner %d, repository %d)", d.Key, source, expected.OwnerID, expected.RepoID, d.OwnerID, d.RepoID)})
		}
	}
	return out
}
func validateTrustedCandidate(g graph, policy TrustPolicy) error {
	findings := trustFindings(g, policy)
	if len(findings) > 0 {
		return fmt.Errorf("%s", findings[0].Message)
	}
	return nil
}
