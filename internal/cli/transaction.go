package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MohamedElashri/sanad/internal/metadata"
)

type transactionFile struct {
	path string
	data []byte
	perm os.FileMode
}

type stagedTransactionFile struct {
	transactionFile
	tempPath   string
	backupPath string
	existed    bool
}

func writeWorkflowAndLockfile(rewrites []workflowRewrite, entries []metadata.LockfileEntry) error {
	existing, _, err := metadata.LoadLockfile(metadata.DefaultLockfilePath)
	if err != nil {
		return categorizedError{code: exitConfig, err: err}
	}
	entries = mergeActiveLockEntries(existing.Entries, entries)
	updated, err := metadata.UpdateLockfile(existing, entries)
	if err != nil {
		return categorizedError{code: exitInternal, err: err}
	}
	lockData, err := metadata.MarshalLockfile(updated)
	if err != nil {
		return categorizedError{code: exitInternal, err: err}
	}

	files := make([]transactionFile, 0, len(rewrites)+1)
	for _, rewrite := range rewrites {
		files = append(files, transactionFile{path: rewrite.Path, data: rewrite.New, perm: rewrite.Perm})
	}
	files = append(files, transactionFile{path: metadata.DefaultLockfilePath, data: lockData, perm: 0o600})
	if err := commitFileTransaction(files); err != nil {
		return categorizedError{code: exitFileSystem, err: err}
	}
	return nil
}

func commitFileTransaction(files []transactionFile) error {
	staged := make([]stagedTransactionFile, 0, len(files))
	cleanup := func() {
		for _, file := range staged {
			if file.tempPath != "" {
				_ = os.Remove(file.tempPath)
			}
		}
	}

	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			cleanup()
			return fmt.Errorf("prepare transaction directory for %q: %w", file.path, err)
		}
		temp, err := os.CreateTemp(filepath.Dir(file.path), ".sanad-stage-*")
		if err != nil {
			cleanup()
			return fmt.Errorf("stage %q: %w", file.path, err)
		}
		tempPath := temp.Name()
		stageErr := func() error {
			if err := temp.Chmod(file.perm); err != nil {
				_ = temp.Close()
				return err
			}
			if _, err := temp.Write(file.data); err != nil {
				_ = temp.Close()
				return err
			}
			if err := temp.Sync(); err != nil {
				_ = temp.Close()
				return err
			}
			return temp.Close()
		}()
		if stageErr != nil {
			_ = os.Remove(tempPath)
			cleanup()
			return fmt.Errorf("stage %q: %w", file.path, stageErr)
		}
		staged = append(staged, stagedTransactionFile{transactionFile: file, tempPath: tempPath})
	}

	committed := 0
	for i := range staged {
		file := &staged[i]
		if info, err := os.Stat(file.path); err == nil {
			backup, createErr := os.CreateTemp(filepath.Dir(file.path), ".sanad-backup-*")
			if createErr != nil {
				rollbackErr := rollbackFileTransaction(staged[:committed])
				cleanup()
				return fmt.Errorf("back up %q: %w", file.path, errors.Join(createErr, rollbackErr))
			}
			file.backupPath = backup.Name()
			backupErr := func() error {
				if !info.Mode().IsRegular() {
					_ = backup.Close()
					return fmt.Errorf("target is not a regular file")
				}
				data, err := os.ReadFile(file.path)
				if err != nil {
					_ = backup.Close()
					return err
				}
				if err := backup.Chmod(info.Mode().Perm()); err != nil {
					_ = backup.Close()
					return err
				}
				if _, err := backup.Write(data); err != nil {
					_ = backup.Close()
					return err
				}
				if err := backup.Sync(); err != nil {
					_ = backup.Close()
					return err
				}
				return backup.Close()
			}()
			if backupErr != nil {
				_ = os.Remove(file.backupPath)
				rollbackErr := rollbackFileTransaction(staged[:committed])
				cleanup()
				return fmt.Errorf("back up %q: %w", file.path, errors.Join(backupErr, rollbackErr))
			}
			file.existed = true
		} else if !os.IsNotExist(err) {
			rollbackErr := rollbackFileTransaction(staged[:committed])
			cleanup()
			return fmt.Errorf("inspect %q: %w", file.path, errors.Join(err, rollbackErr))
		}
		if err := os.Rename(file.tempPath, file.path); err != nil {
			if file.backupPath != "" {
				_ = os.Remove(file.backupPath)
			}
			rollbackErr := rollbackFileTransaction(staged[:committed])
			cleanup()
			return fmt.Errorf("commit %q: %w", file.path, errors.Join(err, rollbackErr))
		}
		file.tempPath = ""
		committed++
	}

	for _, file := range staged {
		if file.backupPath != "" {
			_ = os.Remove(file.backupPath)
		}
	}
	return nil
}

func rollbackFileTransaction(files []stagedTransactionFile) error {
	var rollbackErrs []error
	for i := len(files) - 1; i >= 0; i-- {
		file := files[i]
		if file.existed && file.backupPath != "" {
			if err := os.Rename(file.backupPath, file.path); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("restore %q: %w", file.path, err))
			}
		} else {
			if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("remove new file %q: %w", file.path, err))
			}
		}
	}
	return errors.Join(rollbackErrs...)
}
