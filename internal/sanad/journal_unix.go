//go:build !windows

package sanad

import (
	"os"
	"syscall"
)

func openJournal(path string, create bool) (*os.File, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	return os.OpenFile(path, flags, 0o600)
}

func lockJournal(record *os.File) error {
	return syscall.Flock(int(record.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
