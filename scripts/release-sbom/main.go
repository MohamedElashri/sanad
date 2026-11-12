// release-sbom inventories the Go binaries actually present in each release
// archive. It does not infer executable dependencies from the source go.mod.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"checksumValue"`
}
type pkg struct {
	ID               string     `json:"SPDXID"`
	Name             string     `json:"name"`
	Version          string     `json:"versionInfo,omitempty"`
	Download         string     `json:"downloadLocation"`
	FilesAnalyzed    bool       `json:"filesAnalyzed"`
	LicenseConcluded string     `json:"licenseConcluded"`
	LicenseDeclared  string     `json:"licenseDeclared"`
	Copyright        string     `json:"copyrightText"`
	Checksums        []checksum `json:"checksums,omitempty"`
}
type relationship struct {
	From string `json:"spdxElementId"`
	Type string `json:"relationshipType"`
	To   string `json:"relatedSpdxElement"`
}
type document struct {
	Version   string `json:"spdxVersion"`
	License   string `json:"dataLicense"`
	ID        string `json:"SPDXID"`
	Name      string `json:"name"`
	Namespace string `json:"documentNamespace"`
	Creation  struct {
		Creators []string `json:"creators"`
		Created  string   `json:"created"`
	} `json:"creationInfo"`
	Packages      []pkg          `json:"packages"`
	Relationships []relationship `json:"relationships"`
}

func digest(b []byte) string                { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func packageID(name, version string) string { return "SPDXRef-" + digest([]byte(name+"@"+version)) }
func packageFor(name, version string) pkg {
	return pkg{ID: packageID(name, version), Name: name, Version: version, Download: "NOASSERTION", LicenseConcluded: "NOASSERTION", LicenseDeclared: "NOASSERTION", Copyright: "NOASSERTION"}
}
func archiveBinaries(path string) (map[string][]byte, error) {
	result := map[string][]byte{}
	read := func(name string, r io.Reader) error {
		base := filepath.Base(name)
		if base != "sanad" && base != "sanad.exe" && base != "gh-actions-lock" && base != "gh-actions-lock.exe" {
			return nil
		}
		if _, exists := result[base]; exists {
			return fmt.Errorf("duplicate binary %s", base)
		}
		b, err := io.ReadAll(io.LimitReader(r, (64<<20)+1))
		if err != nil {
			return err
		}
		if len(b) > 64<<20 {
			return fmt.Errorf("binary too large")
		}
		result[base] = b
		return nil
	}
	if strings.HasSuffix(path, ".zip") {
		archive, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		for _, f := range archive.File {
			reader, err := f.Open()
			if err != nil {
				return nil, err
			}
			err = read(f.Name, reader)
			reader.Close()
			if err != nil {
				return nil, err
			}
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if h.Typeflag == tar.TypeReg {
				if err := read(h.Name, tr); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(result) != 2 {
		return nil, fmt.Errorf("expected both release executables in %s", path)
	}
	return result, nil
}
func generate(path string, at time.Time) (document, error) {
	doc := document{Version: "SPDX-2.3", License: "CC0-1.0", ID: "SPDXRef-DOCUMENT", Name: filepath.Base(path)}
	raw, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	sha := digest(raw)
	doc.Namespace = "https://github.com/MohamedElashri/sanad/sbom/" + sha
	doc.Creation.Creators = []string{"Tool: sanad-release-sbom"}
	doc.Creation.Created = at.UTC().Format(time.RFC3339)
	root := packageFor(filepath.Base(path), "")
	root.Checksums = []checksum{{"SHA256", sha}}
	packages := map[string]pkg{root.ID: root}
	doc.Relationships = append(doc.Relationships, relationship{doc.ID, "DESCRIBES", root.ID})
	binaries, err := archiveBinaries(path)
	if err != nil {
		return doc, err
	}
	names := []string{}
	for name := range binaries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := binaries[name]
		info, err := buildinfo.Read(bytes.NewReader(data))
		if err != nil {
			return doc, fmt.Errorf("read build metadata in %s: %w", name, err)
		}
		binary := packageFor(name, info.Main.Version)
		binary.Checksums = []checksum{{"SHA256", digest(data)}}
		packages[binary.ID] = binary
		doc.Relationships = append(doc.Relationships, relationship{root.ID, "CONTAINS", binary.ID})
		// Include the Go toolchain as well as module versions linked into this binary.
		toolchain := packageFor("go", info.GoVersion)
		packages[toolchain.ID] = toolchain
		doc.Relationships = append(doc.Relationships, relationship{toolchain.ID, "BUILD_TOOL_OF", binary.ID})
		for _, module := range append(info.Deps, &info.Main) {
			if module.Replace != nil {
				module = module.Replace
			}
			if module.Path == "" {
				continue
			}
			dep := packageFor(module.Path, module.Version)
			packages[dep.ID] = dep
			doc.Relationships = append(doc.Relationships, relationship{binary.ID, "DEPENDS_ON", dep.ID})
		}
	}
	ids := []string{}
	for id := range packages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		doc.Packages = append(doc.Packages, packages[id])
	}
	return doc, nil
}
func run(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	count := 0
	at := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "sanad_") || !strings.HasSuffix(entry.Name(), ".tar.gz") && !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		doc, err := generate(path, at)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+".spdx.json", append(b, '\n'), 0o644); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no release archives in %s", dir)
	}
	return nil
}
func main() {
	dir := flag.String("dir", "dist", "release archive directory")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
