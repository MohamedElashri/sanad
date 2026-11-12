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

const BackendVersion = "v0.4.1"

var backendDigests = map[string]string{
	"darwin-amd64":      "4ef64599440398999f184321d5237df131721434901ecaaf3da6aa018bdd1f8f",
	"darwin-arm64":      "f205ea5dbbda67165e08aab1fb6395290ca62f0a038bf27d4b684abd0816635c",
	"linux-amd64":       "39deb2fe714dbf74b073d8245fc7f3d50cd0eee18d5c0900e9e849ae933ae17e",
	"linux-arm64":       "aee61c993da36a5f09be2c77e6e4a1a22542a1b30ded4996bf8301c15dcf2b39",
	"windows-amd64.exe": "d3582b58f907c145d673f5f31ebfa21a6570df04f3aca9cce667d129a3c607dc",
	"windows-arm64.exe": "6e769101eb05342b1e29585f39aa14b8a0be8cac8fe8cb9fd0b21177abe7294f",
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

// The pinned producer reports its post-fix state. Frozen verification still
// checks the complete candidate before any changes are installed.
func (r *Report) generated(result NativeResult) { r.native(result) }

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
