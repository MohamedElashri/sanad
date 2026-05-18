package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	changelogPath    = "CHANGELOG.md"
	releaseNotesPath = "docs/content/release-notes.md"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	changelog, err := os.ReadFile(changelogPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", changelogPath, err)
	}

	content := buildReleaseNotes(changelog)
	if err := os.MkdirAll(filepath.Dir(releaseNotesPath), 0o755); err != nil {
		return fmt.Errorf("create release notes directory: %w", err)
	}
	if err := os.WriteFile(releaseNotesPath, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", releaseNotesPath, err)
	}

	fmt.Printf("generated %s from %s\n", releaseNotesPath, changelogPath)
	return nil
}

func buildReleaseNotes(changelog []byte) []byte {
	body := strings.TrimSpace(string(changelog))
	body = strings.TrimPrefix(body, "# Changelog")
	body = strings.TrimSpace(body)
	body = strings.TrimPrefix(body, "All notable changes to Sanad are documented here.")
	body = strings.TrimSpace(body)

	var out bytes.Buffer
	out.WriteString("+++\n")
	out.WriteString("title = \"Release Notes\"\n")
	out.WriteString("description = \"Release notes generated from the Sanad changelog.\"\n")
	out.WriteString("weight = 40\n")
	out.WriteString("template = \"page\"\n")
	out.WriteString("+++\n\n")
	out.WriteString("This page is generated from [CHANGELOG.md](https://github.com/MohamedElashri/sanad/blob/main/CHANGELOG.md).\n\n")
	out.WriteString(body)
	out.WriteByte('\n')
	return out.Bytes()
}
