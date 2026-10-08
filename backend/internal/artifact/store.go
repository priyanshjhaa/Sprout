// Package artifact stores bounded, opaque byte blobs as local files: build
// outputs, and the uploaded source archives they are built from. It does not
// authorize users, interpret application code, or expose files over HTTP.
package artifact

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxBytes = 16 << 20

var (
	ErrInvalid = errors.New("artifact_invalid")
	ErrStorage = errors.New("artifact_storage_unavailable")
	ErrMissing = errors.New("artifact_missing")
	ErrCorrupt = errors.New("artifact_corrupt")
)

type Descriptor struct {
	ID     string `json:"id"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Store struct {
	root *os.Root
}

// Open requires an existing, absolute, private directory. The caller owns the
// returned store and must close it. No directory is created implicitly.
func Open(directory string) (*Store, error) {
	if !filepath.IsAbs(directory) {
		return nil, ErrInvalid
	}
	before, err := os.Lstat(directory)
	if err != nil || !before.IsDir() || before.Mode().Perm()&0077 != 0 {
		return nil, ErrInvalid
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrStorage
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		return nil, ErrInvalid
	}
	return &Store{root: root}, nil
}

func (s *Store) Close() error {
	if s == nil || s.root == nil {
		return ErrInvalid
	}
	if err := s.root.Close(); err != nil {
		return ErrStorage
	}
	return nil
}

// Save atomically publishes one opaque artifact. The caller must supply a
// validated build output; this store only enforces size, integrity, and local
// filesystem ownership. No source contents or filesystem paths are returned.
func (s *Store) Save(ctx context.Context, data []byte) (Descriptor, error) {
	if s == nil || s.root == nil {
		return Descriptor{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Descriptor{}, err
	}
	if len(data) == 0 || len(data) > MaxBytes {
		return Descriptor{}, ErrInvalid
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Descriptor{}, ErrStorage
	}
	id := hex.EncodeToString(random[:])
	temporary := "." + id + ".partial"
	filename := id + ".tar"
	file, err := s.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Descriptor{}, ErrStorage
	}
	defer s.root.Remove(temporary)
	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		_ = file.Close()
		return Descriptor{}, ErrStorage
	}
	if err := ctx.Err(); err != nil {
		_ = file.Close()
		return Descriptor{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Descriptor{}, ErrStorage
	}
	if err := file.Close(); err != nil {
		return Descriptor{}, ErrStorage
	}
	if err := ctx.Err(); err != nil {
		return Descriptor{}, err
	}
	if err := s.root.Rename(temporary, filename); err != nil {
		return Descriptor{}, ErrStorage
	}
	if err := syncDirectory(s.root); err != nil {
		_ = s.root.Remove(filename)
		return Descriptor{}, ErrStorage
	}
	digest := sha256.Sum256(data)
	return Descriptor{ID: id, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}, nil
}

// Load verifies size and digest before returning bytes. A descriptor is an
// internal storage reference, not an authorization token.
func (s *Store) Load(ctx context.Context, descriptor Descriptor) ([]byte, error) {
	if s == nil || s.root == nil || !valid(descriptor) {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := s.root.Open(descriptor.ID + ".tar")
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, ErrStorage
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != descriptor.Size {
		return nil, ErrCorrupt
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return nil, ErrStorage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	if int64(len(data)) != descriptor.Size || hex.EncodeToString(digest[:]) != descriptor.SHA256 {
		return nil, ErrCorrupt
	}
	return data, nil
}

// Delete removes only the artifact named by a validated internal descriptor.
// Authorization and deployment ownership belong to the caller.
func (s *Store) Delete(ctx context.Context, descriptor Descriptor) error {
	if s == nil || s.root == nil || !valid(descriptor) {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.root.Remove(descriptor.ID + ".tar"); errors.Is(err, os.ErrNotExist) {
		return ErrMissing
	} else if err != nil {
		return ErrStorage
	}
	return syncDirectory(s.root)
}

// Sweep removes partial writes left by an interrupted Save and every stored
// blob whose ID keep rejects. It runs at startup, once nothing else can be
// writing, to reclaim files a crashed process never released. It touches only
// names this store creates; anything else in the directory is left alone.
func (s *Store) Sweep(ctx context.Context, keep func(id string) bool) (int, error) {
	if s == nil || s.root == nil || keep == nil {
		return 0, ErrInvalid
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return 0, ErrStorage
	}
	names, err := directory.Readdirnames(-1)
	_ = directory.Close()
	if err != nil {
		return 0, ErrStorage
	}
	removed := 0
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		id, partial := storedID(name)
		if id == "" || (!partial && keep(id)) {
			continue
		}
		if err := s.root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, ErrStorage
		}
		removed++
	}
	if removed > 0 {
		return removed, syncDirectory(s.root)
	}
	return removed, nil
}

// storedID recognises "<id>.tar" and ".<id>.partial" and returns the ID.
func storedID(name string) (string, bool) {
	partial := strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".partial")
	id := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".partial")
	if !partial {
		if !strings.HasSuffix(name, ".tar") {
			return "", false
		}
		id = strings.TrimSuffix(name, ".tar")
	}
	if len(id) != 32 {
		return "", false
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", false
	}
	return id, partial
}

func valid(descriptor Descriptor) bool {
	if len(descriptor.ID) != 32 || len(descriptor.SHA256) != 64 || descriptor.Size < 1 || descriptor.Size > MaxBytes {
		return false
	}
	if _, err := hex.DecodeString(descriptor.ID); err != nil {
		return false
	}
	_, err := hex.DecodeString(descriptor.SHA256)
	return err == nil
}

func syncDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return ErrStorage
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return ErrStorage
	}
	return nil
}
