package workflow

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type UseNode struct {
	File          string `json:"file"`
	NodePath      string `json:"node_path"`
	Raw           string `json:"raw"`
	Line          int    `json:"line"`
	Column        int    `json:"column"`
	HeadLine      string `json:"head_line"`
	LineIndex     int    `json:"line_index"`
	InlineComment string `json:"inline_comment,omitempty"`
}

func ExtractUsesFromFiles(files []string) ([]UseNode, error) {
	var uses []UseNode
	for _, file := range files {
		fileUses, err := ExtractUsesFromFile(file)
		if err != nil {
			return nil, err
		}
		uses = append(uses, fileUses...)
	}
	return uses, nil
}

func ExtractUsesFromFile(file string) ([]UseNode, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read workflow %q: %w", file, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse workflow %q: invalid YAML: %w", file, err)
	}

	lines := strings.Split(string(data), "\n")
	var uses []UseNode
	walkUses(file, &doc, nil, lines, &uses)
	return uses, nil
}

func walkUses(file string, node *yaml.Node, path []string, lines []string, uses *[]UseNode) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walkUses(file, child, path, lines, uses)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			nextPath := appendPath(path, key.Value)

			if key.Kind == yaml.ScalarNode && key.Value == "uses" && value.Kind == yaml.ScalarNode {
				*uses = append(*uses, UseNode{
					File:          file,
					NodePath:      formatNodePath(nextPath),
					Raw:           value.Value,
					Line:          value.Line,
					Column:        value.Column,
					HeadLine:      lineAt(lines, value.Line),
					LineIndex:     value.Line - 1,
					InlineComment: value.LineComment,
				})
			}

			walkUses(file, value, nextPath, lines, uses)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			walkUses(file, child, appendPath(path, fmt.Sprintf("[%d]", i)), lines, uses)
		}
	}
}

func appendPath(path []string, segment string) []string {
	next := make([]string, 0, len(path)+1)
	next = append(next, path...)
	next = append(next, segment)
	return next
}

func formatNodePath(path []string) string {
	if len(path) == 0 {
		return "$"
	}

	var b strings.Builder
	for i, segment := range path {
		if strings.HasPrefix(segment, "[") {
			b.WriteString(segment)
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(segment)
	}
	return b.String()
}

func lineAt(lines []string, line int) string {
	if line <= 0 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}
