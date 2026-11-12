package sanad

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

const VendorStatePath = ".github/sanad/vendor.json"
const VendorRoot = ".github/sanad/vendor/"
const vendorWorkflow = ".github/workflows/vendor-source.yml"

type ArchiveSource interface {
	Archive(context.Context, Dependency) ([]byte, error)
}
type vendorFile struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}
type vendorEntry struct {
	Action string     `json:"action"`
	Source Dependency `json:"source"`
	// Keep the producer's exact graph for frozen source checks, including its
	// SHA-ref normalization and transitive resolution rules.
	Lockfile string                `json:"upstream_lockfile"`
	Files    map[string]vendorFile `json:"files"`
}
type vendorState struct {
	Version      int                    `json:"version"`
	Repositories map[string]vendorEntry `json:"repositories"`
}

func loadVendors(files tree) (vendorState, error) {
	state := vendorState{Version: 1, Repositories: map[string]vendorEntry{}}
	if f, ok := files[VendorStatePath]; ok {
		dec := json.NewDecoder(bytes.NewReader(f.Data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&state); err != nil {
			return state, err
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return state, fmt.Errorf("invalid trailing vendor state")
		}
		if state.Version != 1 || state.Repositories == nil {
			return state, fmt.Errorf("unsupported vendor state")
		}
		for nwo, entry := range state.Repositories {
			parts := strings.Split(nwo, "/")
			if len(parts) != 2 || !validSegment(parts[0]) || !validSegment(parts[1]) || nwo != strings.ToLower(nwo) || repositoryName(entry.Source.Key) != nwo || lock.ParseActionRef(entry.Action+"@v1") == nil || repositoryName(entry.Action) != nwo || entry.Source.Host != "github.com" && !validTenant(entry.Source.Host) || entry.Source.OwnerID <= 0 || entry.Source.RepoID <= 0 || !fullSHA.MatchString(strings.TrimPrefix(entry.Source.Commit, "sha1-")) || entry.Lockfile == "" || len(entry.Files) == 0 {
				return state, fmt.Errorf("invalid vendor source %s", nwo)
			}
			sourceGraph, err := loadGraph(tree{lock.Path: {Data: []byte(entry.Lockfile)}}, entry.Source.Host)
			if err != nil {
				return state, err
			}
			source, ok := sourceGraph[entry.Source.Key]
			if !ok || source.Commit != entry.Source.Commit || source.OwnerID != entry.Source.OwnerID || source.RepoID != entry.Source.RepoID || source.Host != entry.Source.Host {
				return state, fmt.Errorf("vendor source and upstream lock disagree for %s", nwo)
			}
			for name, f := range entry.Files {
				if !strings.HasPrefix(name, VendorRoot+nwo+"/") || !validDigest(f.SHA256) || f.Mode != 0o644 && f.Mode != 0o755 {
					return state, fmt.Errorf("invalid vendor file %s", name)
				}
				if err := validArchivePath(name); err != nil {
					return state, err
				}
			}
		}
	}
	return state, nil
}
func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func fileSHA(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func validArchivePath(name string) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00") || filepath.ToSlash(filepath.Clean(name)) != name {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	for _, p := range strings.Split(name, "/") {
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return fmt.Errorf("nonportable archive path %q", name)
		}
		if p == ".." || strings.EqualFold(p, ".git") || strings.EqualFold(p, ".gitmodules") || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") {
			return fmt.Errorf("unsupported archive path %q", name)
		}
	}
	return nil
}
func unpackArchive(data []byte, nwo string) (tree, error) {
	if len(data) > 64<<20 {
		return nil, fmt.Errorf("source archive exceeds 64 MiB")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := tree{}
	folded := map[string]bool{}
	var total int64
	root := ""
	entries := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries++
		if entries > 20000 || len(h.Name) > 4096 {
			return nil, fmt.Errorf("source archive has too many entries or an oversized path")
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimSuffix(h.Name, "/")
		if err := validArchivePath(name); err != nil {
			return nil, err
		}
		prefix, relative, hasPath := strings.Cut(name, "/")
		if root == "" {
			root = prefix
		}
		if prefix != root {
			return nil, fmt.Errorf("archive has multiple roots")
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if !hasPath || h.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("source archives with links or special files are unsupported: %s", name)
		}
		if h.Size < 0 || h.Size > 64<<20 || total+h.Size > 64<<20 || len(out) >= 10000 {
			return nil, fmt.Errorf("unpacked source exceeds 64 MiB or 10000 files")
		}
		total += h.Size
		target := VendorRoot + nwo + "/" + relative
		if err := validArchivePath(target); err != nil {
			return nil, err
		}
		fold := strings.ToLower(target)
		if folded[fold] {
			return nil, fmt.Errorf("duplicate or case-colliding archive path %s", target)
		}
		folded[fold] = true
		b, err := io.ReadAll(io.LimitReader(tr, h.Size+1))
		if err != nil {
			return nil, err
		}
		if int64(len(b)) != h.Size {
			return nil, fmt.Errorf("truncated source file %s", name)
		}
		mode := os.FileMode(0o644)
		if h.Mode&0o111 != 0 {
			mode = 0o755
		}
		out[target] = file{Data: b, Mode: mode}
	}
	trailing, err := io.Copy(io.Discard, io.LimitReader(gz, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if trailing > 1<<20 {
		return nil, fmt.Errorf("excess trailing archive data")
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("source archive is empty")
	}
	return out, nil
}
func vendorChecks(files tree, state vendorState) error {
	expected := map[string]vendorFile{}
	for _, entry := range state.Repositories {
		for name, record := range entry.Files {
			expected[name] = record
			f, ok := files[name]
			if !ok || fileSHA(f.Data) != record.SHA256 || uint32(f.Mode) != record.Mode {
				return fmt.Errorf("vendored file changed or missing: %s; refusing to overwrite local changes", name)
			}
		}
	}
	for name := range files {
		if strings.HasPrefix(name, VendorRoot) {
			if _, ok := expected[name]; !ok {
				return fmt.Errorf("unexpected file in managed vendor snapshot: %s", name)
			}
		}
	}
	return nil
}
func (a *githubAPI) Archive(ctx context.Context, d Dependency) ([]byte, error) {
	sha := strings.TrimPrefix(d.Commit, "sha1-")
	if !fullSHA.MatchString(sha) {
		return nil, fmt.Errorf("invalid archive commit")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api."+d.Host+"/repos/"+repositoryName(d.Key)+"/tarball/"+sha, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sanad")
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := a.token(ctx, d.Host); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("source archive HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (64<<20)+1))
	if len(b) > 64<<20 {
		return nil, fmt.Errorf("source archive exceeds 64 MiB")
	}
	return b, err
}
func archiveSource(opts Options, host string) ArchiveSource {
	if opts.Archives != nil {
		return opts.Archives
	}
	return newAPI(host)
}
func vendorFixture(entry vendorEntry) tree {
	_, ref, _ := strings.Cut(entry.Source.Key, "@")
	return tree{vendorWorkflow: {Data: []byte("on: push\njobs:\n  source:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: " + entry.Action + "@" + ref + "\n"), Mode: 0o644}, lock.Path: {Data: []byte(entry.Lockfile), Mode: 0o644}}
}
func verifyVendors(ctx context.Context, files tree, host, repo string, opts Options, backend Backend, r *Report) error {
	state, err := loadVendors(files)
	if err != nil {
		return err
	}
	if err := vendorChecks(files, state); err != nil {
		return err
	}
	if opts.Offline {
		return nil
	}
	source := archiveSource(opts, host)
	names := []string{}
	for name := range state.Repositories {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := state.Repositories[name]
		dir, err := snapshot(vendorFixture(entry))
		if err != nil {
			return err
		}
		native, err := backend.Run(ctx, dir, host, repo, "--verify")
		os.RemoveAll(dir)
		if err != nil {
			return err
		}
		r.native(native)
		if r.Blocked() {
			return nil
		}
		archive, err := source.Archive(ctx, entry.Source)
		if err != nil {
			return err
		}
		expected, err := unpackArchive(archive, name)
		if err != nil {
			return err
		}
		if len(expected) != len(entry.Files) {
			return fmt.Errorf("vendored snapshot differs from upstream commit for %s", name)
		}
		for path, f := range expected {
			record, ok := entry.Files[path]
			if !ok || record.SHA256 != fileSHA(f.Data) || record.Mode != uint32(f.Mode) {
				return fmt.Errorf("vendored snapshot differs from upstream commit: %s", path)
			}
		}
	}
	return nil
}
func saveVendors(files tree, state vendorState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	files[VendorStatePath] = file{Data: append(b, '\n'), Mode: 0o644}
	return nil
}

func runVendor(ctx context.Context, root, host, repo string, cfg Config, opts Options, backend Backend, input tree, r *Report) (Report, error) {
	parts := strings.Split(opts.Action, "/")
	if len(parts) != 2 || !validSegment(parts[0]) || !validSegment(parts[1]) {
		return *r, fmt.Errorf("vendor --action requires owner/repo; the repository snapshot includes every subaction")
	}
	nwo := strings.ToLower(opts.Action)
	if opts.To != "" && (lock.ParseActionRef(nwo+"@"+opts.To) == nil || strings.ContainsAny(opts.To, "'\" #\t\n\r")) {
		return *r, fmt.Errorf("invalid target reference")
	}
	state, err := loadVendors(input)
	if err != nil {
		return *r, err
	}
	previous, owned := state.Repositories[nwo]
	nativeGraph, err := loadGraph(input, host)
	if err != nil {
		return *r, err
	}
	before, err := loadPolicyGraph(input, host)
	if err != nil {
		return *r, err
	}
	refs, err := references(input)
	if err != nil {
		return *r, err
	}
	selected := []reference{}
	installed := map[string]bool{}
	actionName := ""
	for _, ref := range refs {
		if !ref.Workflow && strings.EqualFold(ref.NWO, nwo) && !strings.HasPrefix(ref.File, VendorRoot) {
			selected = append(selected, ref)
			installed[ref.Ref] = true
			if actionName == "" {
				actionName = ref.Name
			}
		}
	}
	target := opts.To
	if owned {
		actionName = previous.Action
		if target == "" {
			_, target, _ = strings.Cut(previous.Source.Key, "@")
		}
	}
	if actionName == "" {
		return *r, fmt.Errorf("no action call or vendor snapshot matches %s", nwo)
	}
	if target == "" {
		if len(installed) != 1 {
			return *r, fmt.Errorf("multiple installed versions; select --to explicitly")
		}
		for ref := range installed {
			target = ref
		}
	}
	fixture := tree{vendorWorkflow: {Data: []byte("on: push\njobs:\n  source:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: " + actionName + "@" + target + "\n"), Mode: 0o644}}
	frozen := false
	if owned && opts.To == "" {
		fixture = vendorFixture(previous)
		frozen = true
	} else if opts.To == "" {
		parsed, err := lock.Parse(input[lock.Path].Data)
		if err != nil {
			return *r, err
		}
		key := nwo + "@" + target
		if _, ok := nativeGraph[key]; !ok {
			return *r, fmt.Errorf("action is not natively locked: %s", key)
		}
		keys := map[string]bool{}
		closure(nativeGraph, key, keys)
		deps := map[string]lock.Action{}
		list := []string{}
		for k := range keys {
			deps[k] = parsed.Dependencies[k]
			list = append(list, k)
		}
		sort.Strings(list)
		b, err := marshalVendorLock(deps, list)
		if err != nil {
			return *r, err
		}
		fixture[lock.Path] = file{Data: b, Mode: 0o644}
		frozen = true
	}
	dir, err := snapshot(fixture)
	if err != nil {
		return *r, err
	}
	defer os.RemoveAll(dir)
	args := []string{"--no-narrow"}
	if frozen {
		args = []string{"--verify"}
	}
	result, err := backend.Run(ctx, dir, host, repo, args...)
	if err != nil {
		return *r, err
	}
	r.native(result)
	if r.Blocked() {
		return *r, nil
	}
	resolved, err := readTree(dir, ConfigPath)
	if err != nil {
		return *r, err
	}
	graph, err := loadGraph(resolved, host)
	if err != nil {
		return *r, err
	}
	resolvedRefs, err := references(resolved)
	if err != nil {
		return *r, err
	}
	if len(resolvedRefs) != 1 {
		return *r, fmt.Errorf("ambiguous vendor source resolution")
	}
	source, ok := graph[nwo+"@"+resolvedRefs[0].Ref]
	if !ok {
		return *r, fmt.Errorf("resolved vendor source missing from native graph")
	}
	source.Workflows = []string{}
	for _, d := range before {
		if repositoryName(d.Key) == nwo {
			source.Workflows = append(source.Workflows, d.Workflows...)
		}
	}
	sort.Strings(source.Workflows)
	source.Workflows = unique(source.Workflows)
	archive, err := archiveSource(opts, host).Archive(ctx, source)
	if err != nil {
		return *r, err
	}
	copied, err := unpackArchive(archive, nwo)
	if err != nil {
		return *r, err
	}
	metadata := VendorRoot + nwo + "/"
	sub := strings.SplitN(actionName, "/", 3)
	if len(sub) == 3 {
		metadata += sub[2] + "/"
	}
	if _, ok := copied[metadata+"action.yml"]; !ok {
		if _, ok := copied[metadata+"action.yaml"]; !ok {
			return *r, fmt.Errorf("source snapshot has no action metadata for %s", actionName)
		}
	}
	if !owned {
		path, err := safePath(root, strings.TrimSuffix(VendorRoot+nwo, "/"))
		if err != nil {
			return *r, err
		}
		if _, err := os.Lstat(path); err == nil {
			return *r, fmt.Errorf("refusing to adopt an existing unmanaged vendor directory")
		} else if !os.IsNotExist(err) {
			return *r, err
		}
	}
	edited := copyTree(input)
	for name := range previous.Files {
		delete(edited, name)
	}
	entry := vendorEntry{Action: actionName, Source: source, Lockfile: string(resolved[lock.Path].Data), Files: map[string]vendorFile{}}
	for name, f := range copied {
		edited[name] = f
		entry.Files[name] = vendorFile{SHA256: fileSHA(f.Data), Mode: uint32(f.Mode)}
	}
	state.Repositories[nwo] = entry
	if err := saveVendors(edited, state); err != nil {
		return *r, err
	}
	if err := rewriteUses(edited, selected, func(ref reference) string {
		suffix := strings.TrimPrefix(ref.Name, ref.NWO)
		return "$/" + strings.TrimSuffix(VendorRoot, "/") + "/" + nwo + suffix
	}); err != nil {
		return *r, err
	}
	work, err := snapshot(edited)
	if err != nil {
		return *r, err
	}
	defer os.RemoveAll(work)
	result, err = backend.Run(ctx, work, host, repo, "--no-narrow")
	if err != nil {
		return *r, err
	}
	r.generated(result)
	if r.Blocked() {
		return *r, nil
	}
	final, err := readTree(work, opts.Config)
	if err != nil {
		return *r, err
	}
	if err := vendorChecks(final, state); err != nil {
		return *r, fmt.Errorf("producer changed the immutable vendor snapshot: %w", err)
	}
	if err := checkPublisher(final, cfg, r); err != nil {
		return *r, err
	}
	if r.Blocked() {
		return *r, nil
	}
	after, err := loadPolicyGraph(final, host)
	if err != nil {
		return *r, err
	}
	r.Findings = append(r.Findings, trustFindings(after, cfg.Trust)...)
	if r.Blocked() {
		return *r, nil
	}
	if err := validateTransitivePolicies(before, after, source.Key, cfg, opts); err != nil {
		return *r, err
	}
	observations, err := loadObservations(input, opts.Now)
	if err != nil {
		return *r, err
	}
	observed := map[string]time.Time{}
	minimum, err := duration(cfg.Observation.MinimumAge)
	if err != nil {
		return *r, err
	}
	ready, until, err := assess(before, after, observations, observed, minimum, opts.Now)
	if err != nil {
		return *r, err
	}
	age, err := releaseCooldown(root)
	if err != nil {
		return *r, err
	}
	releases := opts.Releases
	if releases == nil {
		releases = newAPI(host)
	}
	mature, ageUntil, err := assessReleaseAge(ctx, before, after, age, opts.Now, releases)
	if err != nil {
		return *r, err
	}
	if ageUntil.After(until) {
		until = ageUntil
	}
	if !ready || !mature {
		r.Decisions = append(r.Decisions, Decision{Action: nwo, To: target, Status: "pending", Reason: "vendor update is waiting for adoption policy", EligibleAt: &until, Changes: graphDiff(before, after)})
		pending := copyTree(input)
		for k, at := range observed {
			observations.Seen[k] = at
		}
		if minimum > 0 {
			if err := saveObservations(pending, observations); err != nil {
				return *r, err
			}
		}
		return finish(ctx, root, opts.Config, input, pending, opts.Write, r)
	}
	result, err = backend.Run(ctx, work, host, repo, "--verify")
	if err != nil {
		return *r, err
	}
	r.native(result)
	r.Verification = "online"
	if r.Blocked() {
		return *r, nil
	}
	if err := reconcileWorkflows(ctx, final, host, opts, true, r); err != nil {
		return *r, err
	}
	if r.Blocked() {
		return *r, nil
	}
	r.Dependencies = after.list()
	r.Changes = graphDiff(before, after)
	r.Coverage = coverage(final)
	r.Protection, err = protections(final, host)
	if err != nil {
		return *r, err
	}
	r.Decisions = append(r.Decisions, Decision{Action: nwo, To: target, Status: "eligible", Reason: "verified complete source snapshot; review the copied code and retained licenses", Changes: r.Changes})
	return finish(ctx, root, opts.Config, input, final, opts.Write, r)
}

func marshalVendorLock(deps map[string]lock.Action, keys []string) ([]byte, error) {
	return yaml.Marshal(lock.File{Version: lock.Version, Dependencies: deps, Workflows: map[string][]string{vendorWorkflow: keys}})
}
