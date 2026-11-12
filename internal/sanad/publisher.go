package sanad

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var fullSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// Exported action metadata is a contract with callers who may have no lockfile.
// Refuse an entire transaction if the producer would weaken that contract.
func checkPublisher(files tree, cfg Config, r *Report) error {
	active := map[string]bool{}
	for _, name := range cfg.Publisher.Actions {
		f, ok := files[name]
		if !ok {
			return fmt.Errorf("publisher action missing: %s", name)
		}
		active[name] = true
		var doc yaml.Node
		if err := yaml.Unmarshal(f.Data, &doc); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		sites, err := metadataUses(&doc, false)
		if err != nil {
			return err
		}
		for _, site := range sites {
			if strings.HasPrefix(site.node.Value, "$/") {
				r.Findings = append(r.Findings, Finding{Code: "publisher-self-reference", Severity: "error", File: name, Message: "exported actions cannot use $/ references resolved in a caller's repository"})
			}
		}
	}
	refs, err := referencesIn(files, active)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if !fullSHA.MatchString(ref.Ref) {
			r.Findings = append(r.Findings, Finding{Code: "publisher-unpinned", Severity: "error", File: ref.File, Message: ref.Name + ": exported actions require an inline full commit SHA for callers without lockfiles; native normalization was refused"})
		}
	}
	return nil
}
