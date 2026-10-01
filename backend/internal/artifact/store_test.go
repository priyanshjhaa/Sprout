package artifact

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLifecycle(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("artifact")
	descriptor, err := store.Save(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(directory, descriptor.ID+".tar"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("artifact file permissions: %v (%v)", info, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loaded, err := store.Load(context.Background(), descriptor)
	if err != nil || !bytes.Equal(loaded, content) {
		t.Fatalf("saved artifact did not survive reopen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, descriptor.ID+".tar"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), descriptor); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("tampered artifact accepted: %v", err)
	}
	if err := store.Delete(context.Background(), descriptor); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), descriptor); !errors.Is(err, ErrMissing) {
		t.Fatalf("deleted artifact still readable: %v", err)
	}
}

func TestStoreRejectsUnsafeInputs(t *testing.T) {
	directory := t.TempDir()
	if _, err := Open("relative-artifacts"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(alias); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Save(context.Background(), nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := store.Save(context.Background(), make([]byte, MaxBytes+1)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	bad := Descriptor{ID: "../another-file", Size: 1, SHA256: "bad"}
	if _, err := store.Load(context.Background(), bad); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), bad); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Save(ctx, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected inputs left files behind: %v (%v)", entries, err)
	}
}
