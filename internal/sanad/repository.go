package sanad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const transactionPath = ".github/sanad/transaction.json"

type file struct {
	Data []byte      `json:"data"`
	Mode fs.FileMode `json:"mode"`
}
type tree map[string]file

func Root(ctx context.Context, requested string) (string, error) {
	if requested != "" {
		root, err := filepath.Abs(requested)
		if err != nil {
			return "", err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(root)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("repository root must be a directory")
		}
		return root, nil
	}
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return os.Getwd()
	}
	return filepath.EvalSymlinks(strings.TrimSpace(string(out)))
}

func repository(ctx context.Context, root, requested string) (string, string, error) {
	value := requested
	if value == "" {
		value = os.Getenv("GH_REPO")
	}
	if value == "" {
		cmd := exec.CommandContext(ctx, "git", "-C", root, "remote", "get-url", "origin")
		b, err := cmd.Output()
		if err != nil {
			return "", "", fmt.Errorf("repository identity missing; set --repo HOST/OWNER/REPO or add an origin remote")
		}
		value = strings.TrimSpace(string(b))
	}
	host := strings.ToLower(os.Getenv("GH_HOST"))
	if host == "" {
		host = "github.com"
	}
	if strings.HasPrefix(value, "git@") {
		value = strings.Replace(strings.TrimPrefix(value, "git@"), ":", "/", 1)
	}
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", "", err
		}
		value = u.Hostname() + u.Path
	}
	value = strings.TrimSuffix(strings.TrimSuffix(value, "/"), ".git")
	parts := strings.Split(value, "/")
	if len(parts) == 3 {
		host = strings.ToLower(parts[0])
		parts = parts[1:]
	}
	if len(parts) != 2 || !validSegment(parts[0]) || !validSegment(parts[1]) {
		return "", "", fmt.Errorf("invalid repository %q", value)
	}
	if host != "github.com" && !validTenant(host) {
		return "", "", fmt.Errorf("host %s is unsupported by native lockfiles", host)
	}
	return host, parts[0] + "/" + parts[1], nil
}

func validSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
func validTenant(host string) bool {
	name, ok := strings.CutSuffix(host, ".ghe.com")
	if !ok || name == "" || len(name) > 63 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func safePath(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || filepath.ToSlash(filepath.Clean(name)) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	full := filepath.Join(root, filepath.FromSlash(name))
	current := root
	for _, part := range strings.Split(name, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is not allowed in managed path %s", name)
		}
	}
	return full, nil
}

func relevant(name, config string) bool {
	base := filepath.Base(name)
	return name == config || name == StatePath || name == WorkflowStatePath || name == ".github/workflows/actions.lock" || name == ".github/dependabot.yml" || name == ".github/dependabot.yaml" || base == "action.yml" || base == "action.yaml" || strings.HasPrefix(name, ".github/workflows/") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"))
}

func readTree(root, config string) (tree, error) {
	out := tree{}
	var total int64
	load := func(name string) (bool, error) {
		if _, ok := out[name]; ok {
			return true, nil
		}
		for _, part := range strings.Split(name, "/") {
			if strings.EqualFold(part, ".git") {
				return false, fmt.Errorf("git metadata cannot be managed: %s", name)
			}
		}
		path, err := safePath(root, name)
		if err != nil {
			return false, err
		}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("managed file %s is not regular", name)
		}
		total += info.Size()
		if total > 64<<20 {
			return false, fmt.Errorf("workflow and action metadata exceed 64 MiB")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		out[name] = file{Data: b, Mode: info.Mode().Perm()}
		return true, nil
	}
	for _, name := range []string{config, StatePath, WorkflowStatePath, ".github/workflows/actions.lock", ".github/dependabot.yml", ".github/dependabot.yaml"} {
		if _, err := load(name); err != nil {
			return nil, err
		}
	}
	dir, err := safePath(root, ".github/workflows")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}
		if _, err := load(".github/workflows/" + entry.Name()); err != nil {
			return nil, err
		}
	}
	cfg, err := loadConfig(root, config)
	if err != nil {
		return nil, err
	}
	for _, name := range cfg.Publisher.Actions {
		exists, err := load(name)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("publisher action missing: %s", name)
		}
	}
	_, err = collectMetadata(out, load)
	return out, err
}

func snapshot(input tree) (string, error) {
	dir, err := os.MkdirTemp("", "sanad-*")
	if err != nil {
		return "", err
	}
	for name, f := range input {
		if err := putFile(dir, name, f); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
}

func putFile(root, name string, f file) error {
	path, err := safePath(root, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, f.Data, f.Mode)
}

func sameFile(a file, aok bool, b file, bok bool) bool {
	return aok == bok && (!aok || a.Mode == b.Mode && bytes.Equal(a.Data, b.Data))
}

func changedFiles(before, after tree) []string {
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	out := []string{}
	for n := range names {
		a, aok := before[n]
		b, bok := after[n]
		if !sameFile(a, aok, b, bok) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func fileDiff(name string, a, b file) string {
	var out strings.Builder
	old := strings.SplitAfter(string(a.Data), "\n")
	next := strings.SplitAfter(string(b.Data), "\n")
	if len(a.Data) == 0 {
		old = nil
	} else if old[len(old)-1] == "" {
		old = old[:len(old)-1]
	}
	if len(b.Data) == 0 {
		next = nil
	} else if next[len(next)-1] == "" {
		next = next[:len(next)-1]
	}
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", name, name, len(old), len(next))
	for _, group := range []struct {
		prefix string
		lines  []string
	}{{"-", old}, {"+", next}} {
		for _, line := range group.lines {
			out.WriteString(group.prefix + line)
			if !strings.HasSuffix(line, "\n") {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return out.String()
}

type journal struct {
	Before tree     `json:"before"`
	After  tree     `json:"after"`
	Paths  []string `json:"paths"`
}

func atomicFile(root, name string, f file, exists bool) error {
	path, err := safePath(root, name)
	if err != nil {
		return err
	}
	if !exists {
		err = os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sanad-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(f.Mode); err != nil {
		_ = tmp.Close()
		return err
	}
	_, err = tmp.Write(f.Data)
	if err == nil {
		err = tmp.Sync()
	}
	err = errors.Join(err, tmp.Close())
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func applyTree(root, config string, before, after tree) error {
	paths := changedFiles(before, after)
	if len(paths) == 0 {
		return nil
	}
	path, err := safePath(root, transactionPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	record, err := openJournal(path, true)
	if err != nil {
		return fmt.Errorf("another write or interrupted transaction exists; run sanad recover --write: %w", err)
	}
	defer record.Close()
	if err := lockJournal(record); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("transaction is active: %w", err)
	}
	current, err := readTree(root, config)
	if err != nil || len(changedFiles(before, current)) != 0 {
		_ = os.Remove(path)
		if err != nil {
			return err
		}
		return fmt.Errorf("repository changed during planning; rerun the command")
	}
	j := journal{Before: before, After: after, Paths: paths}
	data, err := json.Marshal(j)
	if err == nil {
		_, err = record.Write(data)
	}
	if err == nil {
		err = record.Sync()
	}
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	for _, name := range paths {
		f, ok := after[name]
		if err := atomicFile(root, name, f, ok); err != nil {
			restoreErr := recoverRecord(root, config, record)
			return errors.Join(err, restoreErr)
		}
	}
	return os.Remove(path)
}

func recoverTransaction(root, config string) error {
	path, err := safePath(root, transactionPath)
	if err != nil {
		return err
	}
	record, err := openJournal(path, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer record.Close()
	if err := lockJournal(record); err != nil {
		return fmt.Errorf("transaction is active; wait for the writer to finish: %w", err)
	}
	return recoverRecord(root, config, record)
}

func recoverRecord(root, config string, record *os.File) error {
	if _, err := record.Seek(0, io.SeekStart); err != nil {
		return err
	}
	b, err := io.ReadAll(record)
	if err != nil {
		return err
	}
	var j journal
	if err := json.Unmarshal(b, &j); err != nil || len(j.Paths) == 0 {
		return fmt.Errorf("invalid recovery record; inspect %s manually", transactionPath)
	}
	if err := validateJournal(root, config, j); err != nil {
		return err
	}
	// Check all files before restoring any, so recovery never erases later edits.
	for _, name := range j.Paths {
		p, err := safePath(root, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		old, oldOK := j.Before[name]
		next, nextOK := j.After[name]
		if !(exists == oldOK && bytes.Equal(data, old.Data)) && !(exists == nextOK && bytes.Equal(data, next.Data)) {
			return fmt.Errorf("%s changed after the transaction; recovery refused to overwrite it", name)
		}
	}
	for _, name := range j.Paths {
		f, ok := j.Before[name]
		if err := atomicFile(root, name, f, ok); err != nil {
			return err
		}
	}
	return os.Remove(record.Name())
}

func validateJournal(root, config string, j journal) error {
	for _, files := range []tree{j.Before, j.After} {
		for name, f := range files {
			if _, err := safePath(root, name); err != nil {
				return err
			}
			for _, part := range strings.Split(name, "/") {
				if strings.EqualFold(part, ".git") {
					return fmt.Errorf("recovery cannot modify Git metadata: %s", name)
				}
			}
			if !relevant(name, config) || name == transactionPath || f.Mode & ^fs.FileMode(0o777) != 0 {
				return fmt.Errorf("invalid managed file in recovery record: %s", name)
			}
		}
	}
	want := changedFiles(j.Before, j.After)
	got := append([]string(nil), j.Paths...)
	sort.Strings(got)
	if len(want) != len(got) {
		return fmt.Errorf("recovery paths do not match recorded changes")
	}
	for i := range want {
		if want[i] != got[i] {
			return fmt.Errorf("recovery paths do not match recorded changes")
		}
	}
	return nil
}
