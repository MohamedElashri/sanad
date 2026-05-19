package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type platformArchive struct {
	system   string
	archive  string
	artifact string
}

var archives = []platformArchive{
	{system: "x86_64-linux", archive: "Linux_x86_64", artifact: `Linux_x86_64\.tar\.gz`},
	{system: "aarch64-linux", archive: "Linux_arm64", artifact: `Linux_arm64\.tar\.gz`},
	{system: "x86_64-darwin", archive: "Darwin_x86_64", artifact: `Darwin_x86_64\.tar\.gz`},
	{system: "aarch64-darwin", archive: "Darwin_arm64", artifact: `Darwin_arm64\.tar\.gz`},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var version string
	var checksumPath string
	var flakePath string
	flag.StringVar(&version, "version", "", "release version without leading v")
	flag.StringVar(&checksumPath, "checksums", "", "GoReleaser checksum file")
	flag.StringVar(&flakePath, "flake", "flake.nix", "flake.nix path")
	flag.Parse()

	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return fmt.Errorf("--version is required")
	}
	if checksumPath == "" {
		checksumPath = fmt.Sprintf("dist/sanad_%s_checksums.txt", version)
	}

	checksums, err := loadChecksums(checksumPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", flakePath, err)
	}

	updated, err := updateFlake(string(data), version, checksums)
	if err != nil {
		return err
	}
	if updated == string(data) {
		return nil
	}
	if err := os.WriteFile(flakePath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", flakePath, err)
	}
	return nil
}

func loadChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open checksums %s: %w", path, err)
	}
	defer file.Close()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		checksums[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums %s: %w", path, err)
	}
	return checksums, nil
}

func updateFlake(flake string, version string, checksums map[string]string) (string, error) {
	versionPattern := regexp.MustCompile(`version = "[^"]+";`)
	if !versionPattern.MatchString(flake) {
		return "", fmt.Errorf("flake version field not found")
	}
	flake = versionPattern.ReplaceAllString(flake, fmt.Sprintf(`version = "%s";`, version))

	for _, archive := range archives {
		filename := fmt.Sprintf("sanad_%s_%s.tar.gz", version, archive.archive)
		hexHash, ok := checksums[filename]
		if !ok {
			return "", fmt.Errorf("checksum for %s not found", filename)
		}
		sriHash, err := sriFromHex(hexHash)
		if err != nil {
			return "", fmt.Errorf("checksum for %s: %w", filename, err)
		}

		pattern := regexp.MustCompile(fmt.Sprintf(`(?s)(%s = \{\s+artifact = "sanad_\$\{version\}_%s";\s+hash = ")sha256-[^"]+(";)`, regexp.QuoteMeta(archive.system), archive.artifact))
		if !pattern.MatchString(flake) {
			return "", fmt.Errorf("flake source block for %s not found", archive.system)
		}
		flake = pattern.ReplaceAllString(flake, "${1}"+sriHash+"${2}")
	}

	return flake, nil
}

func sriFromHex(value string) (string, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return "", err
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(decoded), nil
}
