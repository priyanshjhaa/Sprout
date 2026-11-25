// Package githubsource downloads one commit of a public GitHub repository into
// a local folder, so the MCP deploy tool can deploy it exactly like a folder on
// this machine: the folder is then packed, checked and uploaded by the same
// flow, and the deploy API still receives only a source archive.
//
// The download is untrusted input. Nothing in it is executed; every file is
// written through os.Root, links are never created, and the compressed size,
// the uncompressed stream, the number of entries and the bytes written are all
// bounded. Hidden files, secrets and dependencies are never written to disk.
package githubsource

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

const (
	// MaxDownloadBytes bounds the compressed tarball.
	MaxDownloadBytes int64 = 32 << 20
	// maxStreamBytes bounds everything decompressed, including entries that are
	// skipped, so a small download cannot expand without limit.
	maxStreamBytes int64 = 256 << 20
	// maxWrittenBytes bounds what is written to disk. Packing applies Sprout's
	// much smaller source limits afterwards, with a clearer message.
	maxWrittenBytes int64 = 64 << 20
	maxEntries            = 20000
)

// DefaultBaseURL serves a repository's tarball at {base}/{owner}/{repo}/tar.gz/{commit}.
const DefaultBaseURL = "https://codeload.github.com"

// ErrNotFound means GitHub has no such public repository and commit.
var ErrNotFound = errors.New("GitHub has no public repository with that commit. It may be misspelled, or private; Sprout can only deploy public repositories for now")

// Problem is a request or a download that cannot be deployed. Its message is
// written for the person (or agent) who asked.
type Problem struct{ Message string }

func (p *Problem) Error() string { return p.Message }

func problemf(format string, args ...any) error {
	return &Problem{Message: fmt.Sprintf(format, args...)}
}

var (
	ownerPattern  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	repoPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// Repository names a GitHub repository.
type Repository struct{ Owner, Name string }

func (r Repository) String() string { return r.Owner + "/" + r.Name }

// ParseRepository accepts "owner/name" or a github.com URL for it.
func ParseRepository(value string) (Repository, error) {
	trimmed := strings.TrimSpace(value)
	for _, prefix := range []string{"https://", "http://"} {
		trimmed = strings.TrimPrefix(trimmed, prefix)
	}
	trimmed = strings.TrimPrefix(trimmed, "www.")
	trimmed = strings.TrimPrefix(trimmed, "github.com/")
	trimmed = strings.TrimSuffix(strings.TrimSuffix(trimmed, "/"), ".git")
	owner, name, ok := strings.Cut(trimmed, "/")
	if !ok || !ownerPattern.MatchString(owner) || !repoPattern.MatchString(name) || name == "." || name == ".." {
		return Repository{}, problemf("%q is not a GitHub repository; give it as owner/name, like octocat/hello-world", value)
	}
	return Repository{Owner: owner, Name: name}, nil
}

// ParseCommit accepts a full 40-character commit SHA. A branch or tag would
// move, so the deployment would not say exactly which code it is.
func ParseCommit(value string) (string, error) {
	commit := strings.ToLower(strings.TrimSpace(value))
	if !commitPattern.MatchString(commit) {
		return "", problemf("%q is not a full commit SHA; give all 40 characters (for example from `git rev-parse HEAD` after pushing)", value)
	}
	return commit, nil
}

// ParsePath accepts a folder inside the repository, or "" for its top.
func ParsePath(value string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" || trimmed == "." {
		return "", nil
	}
	if path.Clean(trimmed) != trimmed || strings.Contains(trimmed, `\`) || strings.HasPrefix(trimmed, "../") || trimmed == ".." {
		return "", problemf("%q is not a folder path inside the repository; give it like site or apps/web", value)
	}
	return trimmed, nil
}

// Fetcher downloads tarballs. The zero value uses GitHub and a two-minute limit.
type Fetcher struct {
	BaseURL   string
	Client    *http.Client
	UserAgent string
}

// Result describes what was written.
type Result struct {
	Files int
	Bytes int64
	// Skipped names the left-out files and folders, relative to the folder.
	Skipped []string
}

// Fetch downloads repository at commit and writes the files under folder (a
// path from ParsePath) into dir, which must be an existing, empty directory.
func (f Fetcher) Fetch(ctx context.Context, repository Repository, commit, folder, dir string) (Result, error) {
	base := f.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	// A redirect could lead anywhere; codeload answers directly.
	noRedirects := *client
	noRedirects.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	url := fmt.Sprintf("%s/%s/%s/tar.gz/%s", strings.TrimSuffix(base, "/"), repository.Owner, repository.Name, commit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	if f.UserAgent != "" {
		request.Header.Set("User-Agent", f.UserAgent)
	}
	response, err := noRedirects.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, problemf("could not reach GitHub: %v", err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return Result{}, ErrNotFound
	case response.StatusCode != http.StatusOK:
		return Result{}, problemf("GitHub answered %s; try again in a minute", response.Status)
	}

	download := &io.LimitedReader{R: response.Body, N: MaxDownloadBytes + 1}
	result, err := extract(ctx, download, folder, dir)
	if download.N <= 0 {
		return Result{}, problemf("this repository's download is larger than %d MiB; Sprout can't deploy it", MaxDownloadBytes>>20)
	}
	if err != nil {
		return Result{}, err
	}
	if result.Files == 0 {
		if folder != "" {
			return Result{}, problemf("the repository has no folder %q at that commit (or nothing in it can be deployed)", folder)
		}
		return Result{}, problemf("the repository has nothing that can be deployed at that commit")
	}
	return result, nil
}

func extract(ctx context.Context, input io.Reader, folder, dir string) (Result, error) {
	var result Result
	unzipped, err := gzip.NewReader(input)
	if err != nil {
		return result, problemf("GitHub's download is not a readable archive")
	}
	defer unzipped.Close()
	stream := &io.LimitedReader{R: unzipped, N: maxStreamBytes + 1}
	reader := tar.NewReader(stream)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return result, err
	}
	defer root.Close()

	skipped := map[string]bool{}
	skip := func(name string) {
		if !skipped[name] {
			skipped[name] = true
			result.Skipped = append(result.Skipped, name)
		}
	}
	tooLarge := problemf("this repository is too large to deploy; name the folder that holds the app")

	for entries := 0; ; entries++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			if stream.N <= 0 {
				return result, tooLarge
			}
			return result, problemf("GitHub's download is not a readable archive")
		}
		if entries >= maxEntries {
			return result, tooLarge
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		// Every entry sits under one top folder, "<repo>-<commit>/".
		_, name, _ := strings.Cut(strings.TrimSuffix(header.Name, "/"), "/")
		if folder != "" {
			rest, inside := strings.CutPrefix(name, folder+"/")
			if !inside {
				continue
			}
			name = rest
		}
		if name == "" {
			continue
		}
		if path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
			return result, problemf("GitHub's download holds an unsafe path")
		}
		parts := strings.Split(name, "/")
		left := false
		for i, part := range parts {
			if sourcepack.SkipReason(part) != "" {
				skip(strings.Join(parts[:i+1], "/"))
				left = true
				break
			}
		}
		if left {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o700); err != nil {
				return result, err
			}
		case tar.TypeReg:
			if header.Size < 0 || result.Bytes+header.Size > maxWrittenBytes {
				return result, tooLarge
			}
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return result, err
			}
			file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return result, problemf("GitHub's download holds %q twice or in an unusable form", name)
			}
			written, copyErr := io.Copy(file, io.LimitReader(reader, header.Size))
			closeErr := file.Close()
			if copyErr != nil || written != header.Size {
				if stream.N <= 0 {
					return result, tooLarge
				}
				return result, problemf("GitHub's download ended early; try again")
			}
			if closeErr != nil {
				return result, closeErr
			}
			result.Files++
			result.Bytes += written
		default:
			// Links could point anywhere, so they are never created.
			skip(name)
		}
	}
}
