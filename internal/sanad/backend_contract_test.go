package sanad

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBackendContract(t *testing.T) {
	official := os.Getenv("SANAD_TEST_BACKEND")
	if official == "" {
		t.Skip("set SANAD_TEST_BACKEND to exercise pinned binary installation")
	}
	data, err := os.ReadFile(official)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backend")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SANAD_BACKEND", path)
	backend, err := EnsureBackend(context.Background(), false)
	if err != nil || backend.Path != path {
		t.Fatalf("official offline backend rejected: %+v %v", backend, err)
	}
	data[len(data)-1] ^= 1
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, download := range []bool{false, true} {
		if _, err := EnsureBackend(context.Background(), download); err == nil {
			t.Fatal("tampered backend was accepted")
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileSHA(current) != fileSHA(data) {
		t.Fatal("failed verification replaced the user's explicit backend")
	}
	t.Setenv("SANAD_BACKEND", filepath.Join(t.TempDir(), "missing"))
	for _, download := range []bool{false, true} {
		if _, err := EnsureBackend(context.Background(), download); err == nil {
			t.Fatal("explicit missing backend silently downloaded another executable")
		}
	}
}
