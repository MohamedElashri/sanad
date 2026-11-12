package sanad

import (
	"context"
	"errors"
	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
	"testing"
)

type cancelAfterVendorVerification struct {
	Backend
	cancel        context.CancelFunc
	verifications int
}

func (b *cancelAfterVendorVerification) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	result, err := b.Backend.Run(ctx, root, host, repo, args...)
	if err == nil && len(args) > 0 && args[0] == "--verify" {
		b.verifications++
		if b.verifications == 2 {
			b.cancel()
		}
	}
	return result, err
}
func TestVendorCancellationBeforeCommitLeavesRepositoryUnchanged(t *testing.T) {
	root, opts := vendorOptions(t)
	opts.Write = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts.Backend = &cancelAfterVendorVerification{Backend: opts.Backend, cancel: cancel}
	before, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(ctx, "vendor", opts)
	if !errors.Is(err, context.Canceled) || r.Changed {
		t.Fatalf("canceled vendor operation committed: success=%v changed=%v error=%v", r.Success, r.Changed, err)
	}
	after, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if files := changedFiles(before, after); len(files) != 0 {
		t.Fatalf("cancellation changed managed files: %v", files)
	}
}

type cancelAfterSyncGeneration struct {
	Backend
	cancel context.CancelFunc
}

func (b cancelAfterSyncGeneration) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	result, err := b.Backend.Run(ctx, root, host, repo, args...)
	if err == nil && len(args) > 0 && args[0] == "--migrate-local-actions" {
		b.cancel()
	}
	return result, err
}
func TestCanceledPendingSyncDoesNotWriteObservations(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "7d")
	if err := putFile(root, ".github/workflows/ci.yml", workflow("a/root@v1.1.0")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := options(root, cancelAfterSyncGeneration{Backend: &fakeBackend{t: t, definitions: map[string]lock.Action{"a/root@v1.1.0": action("v1.1.0", "b")}}, cancel: cancel})
	opts.Write = true
	before, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(ctx, "sync", opts)
	if !errors.Is(err, context.Canceled) || r.Changed {
		t.Fatalf("canceled pending sync wrote observations: %+v %v", r, err)
	}
	after, err := readTree(root, ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if files := changedFiles(before, after); len(files) != 0 {
		t.Fatalf("cancellation changed files: %v", files)
	}
}
