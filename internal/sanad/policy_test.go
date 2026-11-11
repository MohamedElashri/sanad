package sanad

import (
	"context"
	"testing"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func TestCooldownRequiresOnlineTrustedBase(t *testing.T) {
	root := fixture(t, map[string]lock.Action{"a/root@v1.0.0": action("v1.0.0", "a")}, "")
	if err := putFile(root, ".github/dependabot.yml", file{Data: []byte("version: 2\nupdates:\n - package-ecosystem: github-actions\n   cooldown:\n     default-days: 7\n"), Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	opts := options(root, &fakeBackend{t: t})
	if _, err := Run(context.Background(), "check", opts); err == nil {
		t.Fatal("cooldown check accepted a missing trusted base")
	}
	opts.Offline = true
	r, err := Run(context.Background(), "check", opts)
	if err != nil || !r.Blocked() {
		t.Fatalf("offline check accepted cooldown policy: %+v %v", r, err)
	}
}
