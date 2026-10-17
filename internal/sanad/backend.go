package sanad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const BackendVersion = "v0.2.0"

var backendDigests = map[string]string{
	"darwin-amd64":      "23832d88761af62021b75b12a97663483d753a90c4951550874bcb2ce3b7d1c6",
	"darwin-arm64":      "eef4de166006eeec7d2d1bbb780371f6f34a397a18f0b3638547eb7c8432b512",
	"linux-amd64":       "79ec84c191667fd935a277e6fd3662cf5d3669cccc9e99998731d8f306f373b6",
	"linux-arm64":       "c79a279d6b0b635dc89ab6d38d0ecf98ff83abcee7bb483cd53353b74cd4f6df",
	"windows-amd64.exe": "b25318292a6f2a07103067fa3edfe7ec8b6f779e8f4bc4ffe89aa96a91800e38",
	"windows-arm64.exe": "56a10c1909426e786dfe9cdd1a8247ae3462828d87ed2ab4aa8c39fc1cbe5b0f",
}

func BackendTargets() []string {
	keys := make([]string, 0, len(backendDigests))
	for key := range backendDigests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// FetchBackend also serves release packaging, using the same pinned checksums as
// runtime setup for every supported target.
func FetchBackend(ctx context.Context, asset, destination string) error {
	want, ok := backendDigests[asset]
	if !ok {
		return fmt.Errorf("unsupported backend asset %s", asset)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://github.com/github/gh-actions-lock/releases/download/"+BackendVersion+"/"+asset, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("backend download HTTP %d", res.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), "download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(res.Body, 64<<20))
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("backend checksum mismatch for %s %s", BackendVersion, asset)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), destination)
}

type NativeResult struct {
	ExitCode int    `json:"-"`
	CLI      string `json:"cli_version"`
	Schema   string `json:"lockfile_version"`
	Valid    *bool  `json:"valid"`
	Findings []struct {
		Category    string `json:"category"`
		Severity    string `json:"severity"`
		Detail      string `json:"detail"`
		Remediation string `json:"remediation"`
		Workflow    string `json:"workflow"`
	} `json:"findings"`
}

// Backend is the supported producer interface. Tests inject a backend, never a
// process-wide environment override that could bypass binary verification.
type Backend interface {
	Run(context.Context, string, string, string, ...string) (NativeResult, error)
}

type NativeBackend struct{ Path string }

func BackendPath() (string, error) {
	asset := runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	if _, ok := backendDigests[asset]; !ok {
		return "", fmt.Errorf("unsupported backend platform %s", asset)
	}
	if path := os.Getenv("SANAD_BACKEND"); path != "" {
		return filepath.Abs(path)
	}
	if exe, err := os.Executable(); err == nil {
		name := "gh-actions-lock"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "sanad", BackendVersion, asset), nil
}

func verifyBackend(path string) error {
	asset := runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	want, ok := backendDigests[asset]
	if !ok {
		return fmt.Errorf("unsupported platform %s", asset)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return fmt.Errorf("backend checksum mismatch: expected official %s %s", BackendVersion, asset)
	}
	return nil
}

// EnsureBackend downloads only the exact official release with a pinned digest.
// Offline verification calls it with download=false.
func EnsureBackend(ctx context.Context, download bool) (NativeBackend, error) {
	path, err := BackendPath()
	if err != nil {
		return NativeBackend{}, err
	}
	err = verifyBackend(path)
	if err == nil {
		return NativeBackend{Path: path}, nil
	}
	if !os.IsNotExist(err) {
		return NativeBackend{}, err
	}
	if !download || os.Getenv("SANAD_BACKEND") != "" {
		return NativeBackend{}, fmt.Errorf("backend missing: run sanad setup online, or preseed SANAD_BACKEND with the official %s binary", BackendVersion)
	}
	if err := FetchBackend(ctx, filepath.Base(path), path); err != nil {
		return NativeBackend{}, err
	}
	return NativeBackend{Path: path}, nil
}

func backendEnv(root, host, repo string) []string {
	env := []string{}
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(key, "GH_ACTIONS_LOCK_") || key == "GH_HOST" || key == "GH_REPO" || strings.HasPrefix(key, "GIT_") {
			continue
		}
		env = append(env, pair)
	}
	// User-global upstream cooldowns must not change repository policy.
	return append(env, "GH_HOST="+host, "GH_REPO="+host+"/"+repo, "GH_ACTIONS_LOCK_REPO_ROOT="+root, "GH_ACTIONS_LOCK_CONFIG="+os.DevNull, "NO_COLOR=1")
}

func (b NativeBackend) Run(ctx context.Context, root, host, repo string, args ...string) (NativeResult, error) {
	var result NativeResult
	if err := verifyBackend(b.Path); err != nil {
		return result, err
	}
	args = append(args, "--hostname", host, "--no-interactive", "--json=valid,findings,workflows,dependencies,pins")
	cmd := exec.CommandContext(ctx, b.Path, args...)
	cmd.Dir = root
	cmd.Env = backendEnv(root, host, repo)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			return result, fmt.Errorf("native backend: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		result.ExitCode = 1
	}
	if decodeErr := json.Unmarshal(out, &result); decodeErr != nil {
		return result, fmt.Errorf("invalid native backend JSON: %w", decodeErr)
	}
	if result.CLI != BackendVersion || result.Valid == nil || result.Schema != "v0.0.3" {
		return result, fmt.Errorf("unsupported native backend response (%s, schema %s)", result.CLI, result.Schema)
	}
	return result, nil
}

// Successful producer output contains the pre-fix diagnosis. Its valid field can
// therefore be false even after a successful write. A separate frozen check of
// the candidate is the authority for success.
func (r *Report) generated(result NativeResult) {
	if result.ExitCode != 0 {
		r.native(result)
		return
	}
	for _, f := range result.Findings {
		if f.Severity == "warning" {
			r.Findings = append(r.Findings, Finding{Code: f.Category, Severity: "warning", Message: strings.TrimSpace(f.Detail + " " + f.Remediation), File: f.Workflow})
		}
	}
}

func (r *Report) native(result NativeResult) {
	for _, f := range result.Findings {
		severity := f.Severity
		if f.Category == "reachability-unknown" {
			severity = "error"
		}
		if severity != "error" && severity != "warning" {
			continue
		}
		r.Findings = append(r.Findings, Finding{Code: f.Category, Severity: severity, Message: strings.TrimSpace(f.Detail + " " + f.Remediation), File: f.Workflow})
	}
	if result.Valid != nil && !*result.Valid && !r.Blocked() {
		r.finding("native-invalid", "error", "native verification failed")
	}
}
