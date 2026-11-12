package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInventoriesActualArchiveBinaries(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "sanad_1.2.3_Linux_x86_64.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tr := tar.NewWriter(gz)
	for _, name := range []string{"sanad", "gh-actions-lock"} {
		if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(binary))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write(binary); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	doc, err := generate(archive, time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	packages := map[string]pkg{}
	for _, p := range doc.Packages {
		packages[p.ID] = p
	}
	for _, name := range []string{"sanad", "gh-actions-lock"} {
		found := false
		for _, p := range doc.Packages {
			if p.Name == name {
				found = true
				if len(p.Checksums) != 1 || p.Checksums[0].Value != digest(binary) {
					t.Fatal("binary checksum mismatch")
				}
			}
		}
		if !found {
			t.Fatalf("missing %s", name)
		}
	}
	for _, r := range doc.Relationships {
		if r.From != doc.ID {
			if _, ok := packages[r.From]; !ok {
				t.Fatal("dangling relationship source")
			}
		}
		if _, ok := packages[r.To]; !ok {
			t.Fatal("dangling relationship target")
		}
	}
}
