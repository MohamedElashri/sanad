package cli

import (
	"encoding/json"
	"io"
	"sort"
)

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID               string            `json:"id"`
	Name             string            `json:"name,omitempty"`
	ShortDescription sarifMessage      `json:"shortDescription"`
	FullDescription  sarifMessage      `json:"fullDescription,omitempty"`
	Properties       sarifRuleProperty `json:"properties,omitempty"`
}

type sarifRuleProperty struct {
	ProblemSeverity string `json:"problem.severity,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

func printCheckSARIF(out io.Writer, report checkReport) error {
	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "sanad",
					InformationURI: "https://github.com/MohamedElashri/sanad",
					Rules:          sarifRules(report.Violations),
				},
			},
			Results: sarifResults(report.Violations),
		}},
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(log)
}

func sarifRules(violations []checkViolation) []sarifRule {
	seen := make(map[string]checkViolation)
	for _, violation := range violations {
		if _, ok := seen[violation.ReasonCode]; !ok {
			seen[violation.ReasonCode] = violation
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rules := make([]sarifRule, 0, len(ids))
	for _, id := range ids {
		violation := seen[id]
		description := violation.Reason
		if description == "" {
			description = string(violation.Decision)
		}
		rules = append(rules, sarifRule{
			ID:   id,
			Name: string(violation.Decision),
			ShortDescription: sarifMessage{
				Text: description,
			},
			FullDescription: sarifMessage{
				Text: description,
			},
			Properties: sarifRuleProperty{
				ProblemSeverity: "error",
			},
		})
	}
	return rules
}

func sarifResults(violations []checkViolation) []sarifResult {
	results := make([]sarifResult, 0, len(violations))
	for _, violation := range violations {
		message := violation.Reason
		if message == "" {
			message = string(violation.Decision)
		}
		if violation.Action != "" {
			message = violation.Action + ": " + message
		}
		results = append(results, sarifResult{
			RuleID:  violation.ReasonCode,
			Level:   "error",
			Message: sarifMessage{Text: message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: violation.File},
					Region: sarifRegion{
						StartLine:   violation.Line,
						StartColumn: violation.Column,
					},
				},
			}},
		})
	}
	return results
}
