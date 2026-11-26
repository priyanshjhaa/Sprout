// Package pkgcache keeps downloaded packages in a directory named by their
// digest. It is the one place Sprout turns untrusted bytes from a package
// registry into a file it trusts: a download is hashed while it streams to
// disk and is published, by an atomic rename, only if the digest matches.
// Because a file's name is its digest, a cached file can be shared by every
// later build of any app. npmfetch and pypifetch decide what to download;
// this package decides when a download may be believed.
package pkgcache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

var (
	// ErrUnavailable means the registry could not be reached or answered badly.
	ErrUnavailable = errors.New("package registry unavailable")
	// ErrIntegrity means a file did not match the digest it was expected to have.
	ErrIntegrity = errors.New("package integrity mismatch")
	// ErrTooLarge means a download exceeded its limit.
	ErrTooLarge = errors.New("package too large")
	// ErrNotFound means the registry has no such file.
	ErrNotFound = errors.New("package not found")
	// ErrCache means the cache directory could not be used.
	ErrCache = errors.New("package cache unavailable")
)

// Cache is a private directory of verified files.
type Cache struct {
	dir string
}

// Open uses an absolute directory private to this user, creating it if needed.
func Open(dir string) (*Cache, error) {
	if !filepath.IsAbs(dir) {
		return nil, ErrCache
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, ErrCache
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, ErrCache
	}
	return &Cache{dir: dir}, nil
}

// Dir is the cache directory.
func (c *Cache) Dir() string { return c.dir }

// Path is where a file named name lives in the cache. name must already be
// safe: the callers build it from a hex digest.
func (c *Cache) Path(name string) string { return filepath.Join(c.dir, name) }

// Cached reports the size of a cached regular file, if it exists.
func (c *Cache) Cached(name string) (int64, bool) {
	info, err := os.Lstat(c.Path(name))
	if err != nil || !info.Mode().IsRegular() {
		return 0, false
	}
	return info.Size(), true
}

// NewClient is an HTTP client for registries. It follows no redirects: package
// files are served directly, and a redirect could lead anywhere.
func NewClient() *http.Client {
	return &http.Client{
		Timeout:       2 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Get requests url and returns the body of a 200 answer, refusing one that
// declares more than max bytes. The caller closes the body.
func Get(ctx context.Context, client *http.Client, url string, max int64) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("User-Agent", "sprout-build")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	switch {
	case response.StatusCode == http.StatusNotFound:
		response.Body.Close()
		return nil, ErrNotFound
	case response.StatusCode != http.StatusOK:
		response.Body.Close()
		return nil, ErrUnavailable
	case response.ContentLength > max:
		response.Body.Close()
		return nil, ErrTooLarge
	}
	return response.Body, nil
}

// Download fetches url into the cache as name, publishing it only when its
// digest (computed with newHash) equals digest. It returns the file's size.
func (c *Cache) Download(ctx context.Context, client *http.Client, url, name string, newHash func() hash.Hash, digest []byte, max int64) (int64, error) {
	body, err := Get(ctx, client, url, max)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	return c.store(ctx, body, name, newHash, digest, max)
}

// store writes beside the cache entry and renames only once the digest
// matches, so the cache never holds an unverified file and two builds fetching
// the same package cannot see half of one.
func (c *Cache) store(ctx context.Context, body io.Reader, name string, newHash func() hash.Hash, digest []byte, max int64) (int64, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return 0, ErrCache
	}
	partial := filepath.Join(c.dir, "."+hex.EncodeToString(random[:])+".partial")
	file, err := os.OpenFile(partial, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, ErrCache
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(partial)
		}
	}()
	sum := newHash()
	written, copyErr := io.Copy(io.MultiWriter(file, sum), io.LimitReader(body, max+1))
	closeErr := file.Close()
	switch {
	case copyErr != nil && ctx.Err() != nil:
		return 0, ctx.Err()
	case copyErr != nil:
		return 0, ErrUnavailable
	case written > max:
		return 0, ErrTooLarge
	case closeErr != nil:
		return 0, ErrCache
	case !slices.Equal(sum.Sum(nil), digest):
		return 0, ErrIntegrity
	}
	if err := os.Rename(partial, c.Path(name)); err != nil {
		return 0, ErrCache
	}
	published = true
	return written, nil
}

// Verify re-reads a cached file through its digest while copying it into w,
// so a file changed on disk after it was cached is caught before a build uses
// it. A file that no longer matches is removed, so the next build downloads it
// again instead of failing forever.
func Verify(path string, newHash func() hash.Hash, digest []byte, size int64, into io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCache, err)
	}
	defer file.Close()
	sum := newHash()
	written, err := io.Copy(io.MultiWriter(into, sum), io.LimitReader(file, size+1))
	if err != nil {
		return err
	}
	if written != size || !slices.Equal(sum.Sum(nil), digest) {
		_ = os.Remove(path)
		return ErrIntegrity
	}
	return nil
}
