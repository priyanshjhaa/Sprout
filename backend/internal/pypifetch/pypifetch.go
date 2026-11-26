// Package pypifetch downloads the wheels a hashed requirements.txt names, so a
// Python build can install them without any network of its own.
//
// It is trusted Go code that handles untrusted bytes: it never unpacks or runs
// a package. requirements.txt names each package by exact version and lists
// the SHA-256 of every file it accepts, but not where the files are, so Sprout
// asks PyPI's JSON API which files that version has, keeps the wheels whose
// hash is listed and that suit the build container, and downloads one from
// files.pythonhosted.org. Only wheels are fetched: a wheel installs by copying
// files, while a source package runs its own setup code during installation.
package pypifetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/priyanshjhaa/Sprout/backend/internal/pkgcache"
)

const (
	// MaxPackageBytes bounds one wheel.
	MaxPackageBytes int64 = 64 << 20
	// MaxTotalBytes bounds every wheel one build needs together.
	MaxTotalBytes int64 = 160 << 20
	// MaxRequirementsBytes bounds requirements.txt itself.
	MaxRequirementsBytes = 256 << 10
	maxRequirements      = 500
	maxHashes            = 256
	maxMetadataBytes     = 8 << 20
	// metadataHost answers which files a version has; filesHost serves them.
	metadataHost = "pypi.org"
	filesHost    = "files.pythonhosted.org"
	downloads    = 8
)

var (
	// ErrUnavailable means PyPI could not be reached or answered badly.
	ErrUnavailable = errors.New("python_registry_unavailable")
	// ErrIntegrity means a wheel did not match its hash.
	ErrIntegrity = errors.New("python_package_integrity_mismatch")
	// ErrTooLarge means the wheels exceed Sprout's limits.
	ErrTooLarge = errors.New("python_packages_too_large")
	// ErrNotFound means PyPI has no such package version.
	ErrNotFound = errors.New("python_package_not_found")
	// ErrNoWheel means no listed file is a wheel for the build container.
	ErrNoWheel = errors.New("python_package_no_wheel")
	// ErrRequirements means requirements.txt breaks Sprout's rules.
	ErrRequirements = errors.New("python_contract_requirements")
	// ErrCache means the local package cache could not be used.
	ErrCache = errors.New("python_package_cache_unavailable")
)

// Platform is what the build container's Python accepts.
type Platform struct {
	PythonMinor int    // 13 for Python 3.13
	Arch        string // aarch64 or x86_64
	GlibcMajor  int
	GlibcMinor  int
}

// BuildPlatform is the pinned Python 3.13 Bookworm image (glibc 2.36) on this
// machine's architecture.
func BuildPlatform() Platform {
	arch := runtime.GOARCH
	switch arch {
	case "arm64":
		arch = "aarch64"
	case "amd64":
		arch = "x86_64"
	}
	return Platform{PythonMinor: 13, Arch: arch, GlibcMajor: 2, GlibcMinor: 36}
}

// Requirement is one pinned line of requirements.txt.
type Requirement struct {
	Name    string // normalized, as PyPI's URLs use it
	Version string
	Hashes  []string // lowercase hex SHA-256
	// Marker is true when the line applies only on some systems
	// ("; python_version < '3.11'"). pip decides; Sprout fetches a wheel when one
	// suits the container and otherwise leaves the line to pip.
	Marker bool
}

// Package is one verified wheel in the cache.
type Package struct {
	Path     string
	Filename string // the wheel's own name, which pip reads its tags from
	SHA256   string
	Bytes    int64
}

var (
	specPattern  = regexp.MustCompile(`^([A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?)\s*(\[[A-Za-z0-9._,\s-]*\])?\s*==\s*([A-Za-z0-9.+!_-]+)$`)
	hashPattern  = regexp.MustCompile(`^--hash=sha256:([0-9a-f]{64})$`)
	optionStart  = regexp.MustCompile(`\s--`)
	comment      = regexp.MustCompile(`(^|\s)#.*$`)
	separators   = regexp.MustCompile(`[-_.]+`)
	wheelPattern = regexp.MustCompile(`^[A-Za-z0-9_.+!-]+\.whl$`)
)

// ParseRequirements reads a fully pinned, hashed requirements.txt: every line
// is name==version followed by one or more --hash=sha256:… options, as written
// by `uv pip compile --generate-hashes`, `uv export` or `pip-compile
// --generate-hashes`. Anything that could send pip elsewhere (other indexes,
// URLs, includes, editable installs) is refused.
func ParseRequirements(data []byte) ([]Requirement, error) {
	if len(data) > MaxRequirementsBytes || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, ErrRequirements
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\\\n", " ")
	var result []Requirement
	unconditional := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(comment.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			return nil, ErrRequirements // -r, -e, --index-url and friends
		}
		requirement, options := line, ""
		if at := optionStart.FindStringIndex(line); at != nil {
			requirement, options = line[:at[0]], line[at[0]:]
		}
		spec, marker, hasMarker := strings.Cut(requirement, ";")
		match := specPattern.FindStringSubmatch(strings.TrimSpace(spec))
		if match == nil || strings.Contains(match[3], "*") || (hasMarker && strings.TrimSpace(marker) == "") {
			return nil, ErrRequirements
		}
		entry := Requirement{Name: separators.ReplaceAllString(strings.ToLower(match[1]), "-"), Version: match[3], Marker: hasMarker}
		seen := map[string]bool{}
		for _, option := range strings.Fields(options) {
			hash := hashPattern.FindStringSubmatch(option)
			if hash == nil {
				return nil, ErrRequirements
			}
			if !seen[hash[1]] {
				seen[hash[1]] = true
				entry.Hashes = append(entry.Hashes, hash[1])
			}
		}
		if len(entry.Hashes) == 0 || len(entry.Hashes) > maxHashes {
			return nil, ErrRequirements
		}
		// The same package may appear once per system, each with a marker.
		if !entry.Marker {
			if unconditional[entry.Name] {
				return nil, ErrRequirements
			}
			unconditional[entry.Name] = true
		}
		if result = append(result, entry); len(result) > maxRequirements {
			return nil, ErrRequirements
		}
	}
	return result, nil
}

// Fetcher downloads into a cache directory it owns.
type Fetcher struct {
	cache    *pkgcache.Cache
	platform Platform
	client   *http.Client
	// metadata and files replace https://pypi.org and
	// https://files.pythonhosted.org in tests.
	metadata, files string
}

// New opens a cache in an absolute directory, private to this user.
func New(cache string, platform Platform) (*Fetcher, error) {
	opened, err := pkgcache.Open(cache)
	if err != nil {
		return nil, ErrCache
	}
	return &Fetcher{cache: opened, platform: platform, client: pkgcache.NewClient(),
		metadata: "https://" + metadataHost, files: "https://" + filesHost}, nil
}

// Fetch returns a wheel for every requirement that applies, downloading only
// what the cache does not already hold. It owns its downloads: all of them
// have finished when it returns, and the first failure cancels the rest.
func (f *Fetcher) Fetch(ctx context.Context, requirements []byte) ([]Package, error) {
	parsed, err := ParseRequirements(requirements)
	if err != nil {
		return nil, err
	}
	found := make([]*Package, len(parsed))
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(downloads)
	for i, requirement := range parsed {
		group.Go(func() error {
			p, err := f.one(ctx, requirement)
			found[i] = p
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	var packages []Package
	var total int64
	seen := map[string]bool{}
	for _, p := range found {
		if p == nil || seen[p.SHA256] {
			continue
		}
		seen[p.SHA256] = true
		if total += p.Bytes; total > MaxTotalBytes {
			return nil, ErrTooLarge
		}
		packages = append(packages, *p)
	}
	return packages, nil
}

// one finds a wheel for a requirement: from the cache if any of its hashes is
// already there, otherwise by asking PyPI. It returns nil for a line with a
// marker that has no wheel for this container: pip will skip that line if the
// marker is false, and fail the install if it is not.
func (f *Fetcher) one(ctx context.Context, requirement Requirement) (*Package, error) {
	for _, hash := range requirement.Hashes {
		matches, _ := filepath.Glob(f.cache.Path(hash + "-*.whl"))
		for _, match := range matches {
			name := filepath.Base(match)
			filename := strings.TrimPrefix(name, hash+"-")
			if size, ok := f.cache.Cached(name); ok && wheelPattern.MatchString(filename) && f.platform.accepts(filename) {
				return &Package{Path: match, Filename: filename, SHA256: hash, Bytes: size}, nil
			}
		}
	}
	file, err := f.choose(ctx, requirement)
	if err != nil || file == nil {
		return nil, err
	}
	digest, _ := hex.DecodeString(file.hash)
	name := file.hash + "-" + file.filename
	size, err := f.cache.Download(ctx, f.client, file.url, name, sha256.New, digest, MaxPackageBytes)
	if err != nil {
		return nil, translate(err)
	}
	return &Package{Path: f.cache.Path(name), Filename: file.filename, SHA256: file.hash, Bytes: size}, nil
}

type release struct {
	URLs []struct {
		Filename    string            `json:"filename"`
		URL         string            `json:"url"`
		PackageType string            `json:"packagetype"`
		Digests     map[string]string `json:"digests"`
	} `json:"urls"`
}

type wheel struct{ filename, url, hash string }

// choose asks PyPI for the version's files and picks a wheel whose hash
// requirements.txt lists and that suits the container.
func (f *Fetcher) choose(ctx context.Context, requirement Requirement) (*wheel, error) {
	address := f.metadata + "/pypi/" + url.PathEscape(requirement.Name) + "/" + url.PathEscape(requirement.Version) + "/json"
	body, err := pkgcache.Get(ctx, f.client, address, maxMetadataBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %s==%s", translate(err), requirement.Name, requirement.Version)
	}
	defer body.Close()
	var answer release
	if err := json.NewDecoder(io.LimitReader(body, maxMetadataBytes)).Decode(&answer); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	listed := map[string]bool{}
	for _, hash := range requirement.Hashes {
		listed[hash] = true
	}
	for _, file := range answer.URLs {
		hash := file.Digests["sha256"]
		if file.PackageType != "bdist_wheel" || !listed[hash] || !wheelPattern.MatchString(file.Filename) || len(file.Filename) > 200 || !f.platform.accepts(file.Filename) {
			continue
		}
		location, err := url.Parse(file.URL)
		if err != nil || location.Scheme != "https" || location.Host != filesHost || location.User != nil || location.RawQuery != "" ||
			location.Fragment != "" || strings.Contains(location.Path, "..") || !strings.HasSuffix(location.Path, "/"+file.Filename) {
			return nil, ErrUnavailable
		}
		return &wheel{filename: file.Filename, url: f.files + location.EscapedPath(), hash: hash}, nil
	}
	if requirement.Marker {
		return nil, nil
	}
	return nil, fmt.Errorf("%w: %s==%s", ErrNoWheel, requirement.Name, requirement.Version)
}

// accepts reads a wheel's tags (name-version[-build]-python-abi-platform.whl)
// and reports whether the container's Python can install it.
func (p Platform) accepts(filename string) bool {
	parts := strings.Split(strings.TrimSuffix(filename, ".whl"), "-")
	if len(parts) != 5 && len(parts) != 6 {
		return false
	}
	pythons, abis, platforms := parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1]
	for _, python := range strings.Split(pythons, ".") {
		for _, abi := range strings.Split(abis, ".") {
			if !p.acceptsPython(python, abi) {
				continue
			}
			for _, platform := range strings.Split(platforms, ".") {
				if p.acceptsPlatform(platform) {
					return true
				}
			}
		}
	}
	return false
}

func (p Platform) acceptsPython(python, abi string) bool {
	cpython := fmt.Sprintf("cp3%d", p.PythonMinor)
	switch abi {
	case "none":
		return python == "py3" || python == fmt.Sprintf("py3%d", p.PythonMinor) || python == cpython
	case cpython:
		return python == cpython
	case "abi3": // the stable ABI: built for 3.x, works on every later 3.y
		minor, err := strconv.Atoi(strings.TrimPrefix(python, "cp3"))
		return strings.HasPrefix(python, "cp3") && err == nil && minor >= 2 && minor <= p.PythonMinor
	}
	return false
}

func (p Platform) acceptsPlatform(platform string) bool {
	if platform == "any" {
		return true
	}
	for legacy, glibc := range map[string][2]int{"manylinux1_": {2, 5}, "manylinux2010_": {2, 12}, "manylinux2014_": {2, 17}} {
		if arch, ok := strings.CutPrefix(platform, legacy); ok {
			return arch == p.Arch && p.hasGlibc(glibc[0], glibc[1])
		}
	}
	rest, ok := strings.CutPrefix(platform, "manylinux_")
	if !ok {
		return false // musllinux, macOS and Windows wheels, and unportable linux_* builds
	}
	fields := strings.SplitN(rest, "_", 3)
	if len(fields) != 3 {
		return false
	}
	major, err1 := strconv.Atoi(fields[0])
	minor, err2 := strconv.Atoi(fields[1])
	return err1 == nil && err2 == nil && fields[2] == p.Arch && p.hasGlibc(major, minor)
}

func (p Platform) hasGlibc(major, minor int) bool {
	return major < p.GlibcMajor || major == p.GlibcMajor && minor <= p.GlibcMinor
}

// translate gives the cache's errors Python's own codes.
func translate(err error) error {
	for from, to := range map[error]error{
		pkgcache.ErrUnavailable: ErrUnavailable, pkgcache.ErrIntegrity: ErrIntegrity, pkgcache.ErrTooLarge: ErrTooLarge,
		pkgcache.ErrNotFound: ErrNotFound, pkgcache.ErrCache: ErrCache,
	} {
		if errors.Is(err, from) {
			return to
		}
	}
	return err
}

// Verify re-reads a cached wheel through its hash while copying it, so a file
// changed on disk after it was cached is caught before a build uses it.
func Verify(p Package, into io.Writer) error {
	digest, err := hex.DecodeString(p.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return ErrIntegrity
	}
	return translate(pkgcache.Verify(p.Path, sha256.New, digest, p.Bytes, into))
}
