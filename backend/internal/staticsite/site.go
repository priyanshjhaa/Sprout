// Package staticsite validates, packs and indexes static sites: plain files
// with an index.html at the root, served as bytes and never executed. Source
// files are untrusted, so names are restricted to a conservative alphabet and
// the whole site must fit the source limits.
package staticsite

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

var ErrContract = errors.New("static_contract_invalid")

const (
	maxDepth   = 16
	maxSegment = 128
	indexName  = "index.html"
)

// A segment starts with a letter, digit or one of _@+- and continues with the
// same plus dots: no dotfiles, spaces, control characters or encodings to confuse URLs.
var segment = regexp.MustCompile(`^[A-Za-z0-9_@+-][A-Za-z0-9._@+-]*$`)

func validName(name string) bool {
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > maxDepth {
		return false
	}
	for _, part := range parts {
		if len(part) > maxSegment || !segment.MatchString(part) {
			return false
		}
	}
	return true
}

// Validate checks a prepared source tree against the static contract: an
// index.html at the root, no package.json (that would be a Node app), regular
// files and directories only, and safe names. It reads no file contents.
func Validate(ctx context.Context, files fs.FS) error {
	if files == nil {
		return ErrContract
	}
	if info, err := fs.Stat(files, indexName); err != nil || !info.Mode().IsRegular() {
		return ErrContract
	}
	if _, err := fs.Stat(files, "package.json"); err == nil {
		return ErrContract
	}
	return fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return ErrContract
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if !validName(name) || !(entry.IsDir() || entry.Type().IsRegular()) {
			return ErrContract
		}
		return nil
	})
}

// Pack returns a normalized tar of a validated tree: platform-owned metadata,
// no timestamps or owners from the upload.
func Pack(ctx context.Context, files fs.FS) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrContract
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if entry.IsDir() {
			return writer.WriteHeader(&tar.Header{Name: name + "/", Mode: 0700, Typeflag: tar.TypeDir})
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return ErrContract
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			return ErrContract
		}
		if _, err := writer.Write(content); err != nil {
			return ErrContract
		}
		if int64(buffer.Len()) > source.MaxSourceBytes+2*source.MaxEntries*512 {
			return source.ErrLimit
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if writer.Close() != nil {
		return nil, ErrContract
	}
	return buffer.Bytes(), nil
}

// File is one servable file.
type File struct {
	Data        []byte
	ContentType string
}

// Site is a static site held in memory, indexed by relative path. It is
// bounded by the source limits, so a Site never exceeds a few MiB.
type Site struct{ files map[string]File }

// Load indexes a packed site. It re-validates everything: the artifact came
// from Sprout, but a corrupt or tampered store must not become a path
// traversal or an oversized allocation.
func Load(archive []byte) (*Site, error) {
	reader := tar.NewReader(bytes.NewReader(archive))
	site := &Site{files: make(map[string]File)}
	var total int64
	entries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrContract
		}
		if entries++; entries > source.MaxEntries*2 {
			return nil, source.ErrLimit
		}
		name := strings.TrimSuffix(header.Name, "/")
		if !validName(name) {
			return nil, ErrContract
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, ErrContract
		}
		if header.Size < 0 || header.Size > source.MaxFileBytes {
			return nil, source.ErrLimit
		}
		if total += header.Size; total > source.MaxSourceBytes {
			return nil, source.ErrLimit
		}
		data, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
		if err != nil || int64(len(data)) != header.Size {
			return nil, ErrContract
		}
		if _, duplicate := site.files[name]; duplicate {
			return nil, ErrContract
		}
		site.files[name] = File{Data: data, ContentType: contentType(name)}
	}
	if _, ok := site.files[indexName]; !ok {
		return nil, ErrContract
	}
	return site, nil
}

// Size is the number of files in the site.
func (s *Site) Size() int { return len(s.files) }

// Lookup resolves a request path ("/", "/about/", "/app.js") to a file. A path
// naming a directory serves that directory's index.html; anything else must
// match a file exactly.
func (s *Site) Lookup(requestPath string) (File, bool) {
	if !strings.HasPrefix(requestPath, "/") || strings.ContainsAny(requestPath, "\x00\\") {
		return File{}, false
	}
	name := strings.TrimPrefix(requestPath, "/")
	switch {
	case name == "":
		name = indexName
	case strings.HasSuffix(name, "/"):
		name += indexName
	}
	// Reject anything that is not already in canonical form instead of cleaning
	// it: "/a/../b" and "//a" are errors, not aliases.
	if path.Clean(name) != name {
		return File{}, false
	}
	file, ok := s.files[name]
	return file, ok
}

// Content types are chosen by Sprout from a fixed table, never by the upload
// and never by the host's mime database, so a site is served the same
// everywhere. Unknown extensions download as opaque bytes.
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
	".json": "application/json", ".map": "application/json", ".webmanifest": "application/manifest+json",
	".txt": "text/plain; charset=utf-8", ".md": "text/plain; charset=utf-8", ".xml": "application/xml", ".csv": "text/csv; charset=utf-8",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
	".mp4": "video/mp4", ".webm": "video/webm", ".mp3": "audio/mpeg", ".wav": "audio/wav",
	".pdf": "application/pdf", ".wasm": "application/wasm",
}

func contentType(name string) string {
	if value, ok := contentTypes[strings.ToLower(path.Ext(name))]; ok {
		return value
	}
	return "application/octet-stream"
}
