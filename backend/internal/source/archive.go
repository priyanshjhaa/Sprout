// Package source prepares untrusted source files, but never executes them.
package source

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"unicode"
)

const (
	MaxArchiveBytes int64 = 16 << 20
	MaxSourceBytes  int64 = 8 << 20
	MaxFileBytes    int64 = 2 << 20
	MaxEntries            = 256
)

var (
	ErrInvalid   = errors.New("source_archive_invalid")
	ErrLimit     = errors.New("source_limit_exceeded")
	ErrSensitive = errors.New("source_sensitive_path")
	ErrStorage   = errors.New("source_storage_unavailable")
	ErrCleanup   = errors.New("source_cleanup_failed")
)

type Summary struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// WithArchive owns a private temporary tree for the duration of consume. The
// callback is trusted Go code, must honor ctx, and must not retain open handles
// or start work that outlives this call. Only regular files and directories in
// plain uncompressed tar archives are accepted. The caller owns input and must give
// blocking network readers their own read deadline; context cannot interrupt an
// arbitrary Reader. This function starts no goroutines and runs no source code.
func WithArchive(ctx context.Context, input io.Reader, consume func(context.Context, fs.FS) error) (summary Summary, err error) {
	if err = ctx.Err(); err != nil {
		return
	}
	if input == nil || consume == nil {
		return summary, ErrInvalid
	}
	directory, err := os.MkdirTemp("", "sprout-source-")
	if err != nil {
		return summary, ErrStorage
	}
	// Only this exact generated directory is owned by this call.
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			err = errors.Join(err, ErrCleanup)
		}
	}()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return summary, ErrStorage
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			err = errors.Join(err, ErrCleanup)
		}
	}()
	limited := &io.LimitedReader{R: contextReader{ctx: ctx, reader: input}, N: MaxArchiveBytes + 1}
	summary, err = extract(ctx, root, limited)
	if ctx.Err() != nil {
		return summary, ctx.Err()
	}
	if limited.N <= 0 {
		return summary, ErrLimit
	}
	if err != nil {
		return summary, err
	}
	if err = ctx.Err(); err != nil {
		return
	}
	err = consume(ctx, root.FS())
	if err == nil {
		err = ctx.Err()
	}
	return
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func extract(ctx context.Context, root *os.Root, input io.Reader) (Summary, error) {
	var result Summary
	reader := tar.NewReader(input)
	seen := map[string]bool{}
	for entries := 0; ; entries++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return result, ErrInvalid
		}
		if entries >= MaxEntries {
			return result, ErrLimit
		}
		// BSD tar's numeric padding can be classified as FormatUnknown by Go.
		// Accept those parsed plain headers, not GNU/PAX metadata or sparse types.
		if (header.Format != tar.FormatUSTAR && header.Format != tar.FormatUnknown) || len(header.PAXRecords) != 0 || len(header.Xattrs) != 0 || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir) || header.Linkname != "" {
			return result, ErrInvalid
		}
		name, err := safeName(header.Name, header.Typeflag == tar.TypeDir)
		if err != nil {
			return result, err
		}
		key := strings.ToLower(name)
		if seen[key] {
			return result, ErrInvalid
		}
		seen[key] = true
		if header.Size < 0 || header.Size > MaxFileBytes || result.Bytes+header.Size > MaxSourceBytes {
			return result, ErrLimit
		}
		if header.Typeflag == tar.TypeDir {
			if header.Size != 0 {
				return result, ErrInvalid
			}
			if err := root.MkdirAll(name, 0700); err != nil {
				return result, ErrStorage
			}
			continue
		}
		if err := root.MkdirAll(path.Dir(name), 0700); err != nil {
			return result, ErrStorage
		}
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return result, ErrStorage
		}
		written, copyErr := io.Copy(file, contextReader{ctx: ctx, reader: reader})
		closeErr := file.Close()
		if copyErr != nil || written != header.Size {
			return result, ErrInvalid
		}
		if closeErr != nil {
			return result, ErrStorage
		}
		result.Files++
		result.Bytes += written
	}
	if result.Files == 0 {
		return result, ErrInvalid
	}
	// tar.Reader stops at the end marker. Reject appended archives/hidden payloads
	// and count ordinary zero padding against the input budget as well.
	buffer := make([]byte, 32*1024)
	for {
		n, err := input.Read(buffer)
		for _, b := range buffer[:n] {
			if b != 0 {
				return result, ErrInvalid
			}
		}
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return result, ErrInvalid
		}
	}
}

func safeName(name string, directory bool) (string, error) {
	if directory {
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || len(name) > 240 || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\:") || strings.ContainsFunc(name, unicode.IsControl) {
		return "", ErrInvalid
	}
	parts := strings.Split(name, "/")
	if len(parts) > 16 {
		return "", ErrLimit
	}
	for _, part := range parts {
		if part == "." || part == ".." || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
			return "", ErrInvalid
		}
		part = strings.ToLower(part)
		switch part {
		case ".git", ".ssh", ".aws", ".azure", ".kube", ".npmrc", ".pypirc", ".netrc", "credentials.json", "id_rsa", "id_ed25519", "id_ecdsa":
			return "", ErrSensitive
		}
		if part == ".env" || strings.HasPrefix(part, ".env.") || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") || strings.HasSuffix(part, ".p12") || strings.HasSuffix(part, ".pfx") {
			return "", ErrSensitive
		}
	}
	return name, nil
}
