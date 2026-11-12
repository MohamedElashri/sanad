package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsInventoryBatchAndInvalidArchives(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		names         []string
		invalidBinary bool
		wantError     bool
	}{
		{"complete", []string{"sanad.exe", "gh-actions-lock.exe", "README.md"}, false, false},
		{"missing-backend", []string{"sanad.exe"}, false, true},
		{"duplicate-binary", []string{"sanad.exe", "nested/sanad.exe", "gh-actions-lock.exe"}, false, true},
		{"invalid-binary", []string{"sanad.exe", "gh-actions-lock.exe"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sanad_1.0.3_Windows_x86_64.zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			archive := zip.NewWriter(file)
			for _, name := range tc.names {
				w, err := archive.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				data := binary
				if tc.invalidBinary || name == "README.md" {
					data = []byte("fixture")
				}
				if _, err := w.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			err = run(dir)
			if (err != nil) != tc.wantError {
				t.Fatalf("inventory error=%v, wantError=%v", err, tc.wantError)
			}
			if tc.wantError {
				return
			}
			data, err := os.ReadFile(path + ".spdx.json")
			if err != nil {
				t.Fatal(err)
			}
			var doc document
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Version != "SPDX-2.3" || len(doc.Packages) < 3 {
				t.Fatalf("incomplete ZIP inventory: %+v", doc)
			}
		})
	}
	if err := run(t.TempDir()); err == nil {
		t.Fatal("an empty release directory produced a successful inventory")
	}
}
