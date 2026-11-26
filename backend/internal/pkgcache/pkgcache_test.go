package pkgcache

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadPublishesOnlyAMatchingFile(t *testing.T) {
	body := []byte("wheel bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good", "/tampered":
			if r.URL.Path == "/tampered" {
				w.Write([]byte("other bytes"))
				return
			}
			w.Write(body)
		case "/big":
			w.Write(make([]byte, 64))
		case "/moved":
			http.Redirect(w, r, "/good", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cache, err := Open(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	client := NewClient()
	ctx := context.Background()

	if size, err := cache.Download(ctx, client, server.URL+"/good", "good", sha256.New, sum[:], 1024); err != nil || size != int64(len(body)) {
		t.Fatalf("good: %d %v", size, err)
	}
	if _, ok := cache.Cached("good"); !ok {
		t.Fatal("a verified file was not published")
	}
	for path, want := range map[string]error{"/tampered": ErrIntegrity, "/big": ErrTooLarge, "/moved": ErrUnavailable, "/missing": ErrNotFound} {
		if _, err := cache.Download(ctx, client, server.URL+path, "x", sha256.New, sum[:], 32); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", path, err, want)
		}
	}
	entries, _ := os.ReadDir(cache.Dir())
	if len(entries) != 1 {
		t.Fatalf("only the verified file may stay in the cache: %v", entries)
	}
}

func TestOpenWantsAPrivateAbsoluteDirectory(t *testing.T) {
	if _, err := Open("relative"); !errors.Is(err, ErrCache) {
		t.Fatalf("relative: %v", err)
	}
	open := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(open); !errors.Is(err, ErrCache) {
		t.Fatalf("world-readable: %v", err)
	}
}
