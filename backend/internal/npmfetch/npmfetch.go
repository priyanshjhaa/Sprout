// Package npmfetch downloads the packages an npm lockfile names, so a build can
// install them without any network of its own.
//
// It is trusted Go code that handles untrusted bytes: it never unpacks or runs
// a package. Each tarball comes only from the public npm registry, must match
// the SHA-512 integrity the lockfile records, and is kept in a cache named by
// that digest, so a later build of any app reuses it without downloading
// again. The lockfile itself is checked first by nodeapp.Validate.
package npmfetch

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// MaxPackageBytes bounds one tarball.
	MaxPackageBytes int64 = 64 << 20
	// MaxTotalBytes bounds every tarball one build needs together.
	MaxTotalBytes int64 = 160 << 20
	// registryHost is the only place packages come from.
	registryHost = "registry.npmjs.org"
	// downloads is how many tarballs one build downloads at once.
	downloads = 8
)

var (
	// ErrUnavailable means the registry could not be reached or answered badly.
	ErrUnavailable = errors.New("npm_registry_unavailable")
	// ErrIntegrity means a tarball did not match the lockfile.
	ErrIntegrity = errors.New("npm_package_integrity_mismatch")
	// ErrTooLarge means the packages exceed Sprout's limits.
	ErrTooLarge = errors.New("npm_packages_too_large")
	// ErrNotFound means the registry has no such package version.
	ErrNotFound = errors.New("npm_package_not_found")
	// ErrLockfile means the lockfile names a package Sprout will not fetch.
	ErrLockfile = errors.New("node_contract_lockfile")
	// ErrCache means the local package cache could not be used.
	ErrCache = errors.New("npm_package_cache_unavailable")
)

// Platform is the system the build installs for, in npm's own words.
type Platform struct{ OS, CPU, Libc string }

// BuildPlatform is the Linux build container on this machine's architecture.
func BuildPlatform() Platform {
	cpu := runtime.GOARCH
	if cpu == "amd64" {
		cpu = "x64"
	}
	return Platform{OS: "linux", CPU: cpu, Libc: "glibc"}
}

// Package is one verified tarball in the cache.
type Package struct {
	// Path is the cached file.
	Path string
	// Integrity is the lockfile's "sha512-…" string.
	Integrity string
	Bytes     int64
}

// Fetcher downloads into a cache directory it owns.
type Fetcher struct {
	cache    string
	platform Platform
	client   *http.Client
	// registry replaces https://registry.npmjs.org in tests.
	registry string
}

// New opens a cache in an existing absolute directory, private to this user.
func New(cache string, platform Platform) (*Fetcher, error) {
	if !filepath.IsAbs(cache) {
		return nil, ErrCache
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return nil, ErrCache
	}
	info, err := os.Stat(cache)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, ErrCache
	}
	client := &http.Client{
		Timeout: 2 * time.Minute,
		// Tarballs are served directly; a redirect could lead anywhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Fetcher{cache: cache, platform: platform, client: client, registry: "https://" + registryHost}, nil
}

type lockEntry struct {
	Resolved  string   `json:"resolved"`
	Integrity string   `json:"integrity"`
	Optional  bool     `json:"optional"`
	OS        []string `json:"os"`
	CPU       []string `json:"cpu"`
	Libc      []string `json:"libc"`
}

type lockfile struct {
	Packages map[string]lockEntry `json:"packages"`
}

type wanted struct {
	url, integrity string
	digest         []byte
}

// selectPackages reads a lockfile and returns the tarballs this platform needs. A
// package built for another system (a macOS or Windows binary of esbuild, for
// example) is skipped, as npm itself would skip it.
func (f *Fetcher) selectPackages(lock []byte) ([]wanted, error) {
	var parsed lockfile
	if err := json.Unmarshal(lock, &parsed); err != nil {
		return nil, ErrLockfile
	}
	names := make([]string, 0, len(parsed.Packages))
	for name := range parsed.Packages {
		names = append(names, name)
	}
	slices.Sort(names)
	seen := map[string]bool{}
	var result []wanted
	for _, name := range names {
		entry := parsed.Packages[name]
		if name == "" || !f.platform.matches(entry) {
			continue
		}
		location, err := url.Parse(entry.Resolved)
		if err != nil || location.Scheme != "https" || location.Host != registryHost || location.User != nil ||
			location.RawQuery != "" || location.Fragment != "" || !strings.HasSuffix(location.Path, ".tgz") || strings.Contains(location.Path, "..") {
			return nil, ErrLockfile
		}
		digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(entry.Integrity, "sha512-"))
		if !strings.HasPrefix(entry.Integrity, "sha512-") || err != nil || len(digest) != sha512.Size {
			return nil, ErrLockfile
		}
		if seen[entry.Integrity] {
			continue
		}
		seen[entry.Integrity] = true
		result = append(result, wanted{url: f.registry + location.EscapedPath(), integrity: entry.Integrity, digest: digest})
	}
	return result, nil
}

// matches applies npm's os, cpu and libc fields, including "!name" exclusions.
func (p Platform) matches(entry lockEntry) bool {
	return allows(entry.OS, p.OS) && allows(entry.CPU, p.CPU) && allows(entry.Libc, p.Libc)
}

func allows(list []string, value string) bool {
	if len(list) == 0 {
		return true
	}
	allowed, positive := false, false
	for _, item := range list {
		if excluded, negated := strings.CutPrefix(item, "!"); negated {
			if excluded == value {
				return false
			}
			continue
		}
		positive = true
		if item == value {
			allowed = true
		}
	}
	return allowed || !positive
}

// Fetch returns every tarball the lockfile needs on this platform, downloading
// only what the cache does not already hold. It owns its downloads: all of
// them have finished when it returns, and the first failure cancels the rest.
func (f *Fetcher) Fetch(ctx context.Context, lock []byte) ([]Package, error) {
	needed, err := f.selectPackages(lock)
	if err != nil {
		return nil, err
	}
	packages := make([]Package, len(needed))
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(downloads)
	for i, item := range needed {
		group.Go(func() error {
			fetched, err := f.one(ctx, item)
			packages[i] = fetched
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	var total int64
	for _, p := range packages {
		if total += p.Bytes; total > MaxTotalBytes {
			return nil, ErrTooLarge
		}
	}
	return packages, nil
}

func (f *Fetcher) one(ctx context.Context, item wanted) (Package, error) {
	path := filepath.Join(f.cache, hex.EncodeToString(item.digest)+".tgz")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return Package{Path: path, Integrity: item.integrity, Bytes: info.Size()}, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, item.url, nil)
	if err != nil {
		return Package{}, ErrUnavailable
	}
	request.Header.Set("User-Agent", "sprout-build")
	response, err := f.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Package{}, ctx.Err()
		}
		return Package{}, ErrUnavailable
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return Package{}, ErrNotFound
	case response.StatusCode != http.StatusOK:
		return Package{}, ErrUnavailable
	}
	if response.ContentLength > MaxPackageBytes {
		return Package{}, ErrTooLarge
	}
	return f.store(ctx, response.Body, item, path)
}

// store writes a download beside the cache entry and publishes it by rename
// only once its digest matches, so the cache never holds an unverified file
// and two builds fetching the same package cannot see half of one.
func (f *Fetcher) store(ctx context.Context, body io.Reader, item wanted, path string) (Package, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Package{}, ErrCache
	}
	partial := filepath.Join(f.cache, "."+hex.EncodeToString(random[:])+".partial")
	file, err := os.OpenFile(partial, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Package{}, ErrCache
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(partial)
		}
	}()
	hash := sha512.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(body, MaxPackageBytes+1))
	closeErr := file.Close()
	switch {
	case copyErr != nil && ctx.Err() != nil:
		return Package{}, ctx.Err()
	case copyErr != nil:
		return Package{}, ErrUnavailable
	case written > MaxPackageBytes:
		return Package{}, ErrTooLarge
	case closeErr != nil:
		return Package{}, ErrCache
	case !slices.Equal(hash.Sum(nil), item.digest):
		return Package{}, ErrIntegrity
	}
	if err := os.Rename(partial, path); err != nil {
		return Package{}, ErrCache
	}
	published = true
	return Package{Path: path, Integrity: item.integrity, Bytes: written}, nil
}

// Verify re-reads a cached tarball through its digest while copying it, so a
// file changed on disk after it was cached is caught before a build uses it.
// A file that no longer matches is removed, so the next build downloads it
// again instead of failing forever.
func Verify(p Package, into io.Writer) error {
	digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(p.Integrity, "sha512-"))
	if err != nil {
		return ErrIntegrity
	}
	file, err := os.Open(p.Path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCache, err)
	}
	defer file.Close()
	hash := sha512.New()
	written, err := io.Copy(io.MultiWriter(into, hash), io.LimitReader(file, p.Bytes+1))
	if err != nil {
		return err
	}
	if written != p.Bytes || !slices.Equal(hash.Sum(nil), digest) {
		_ = os.Remove(p.Path)
		return ErrIntegrity
	}
	return nil
}
