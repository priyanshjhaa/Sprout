package npmfetch

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func integrity(data []byte) string {
	sum := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// registry is a fake npm registry serving tarballs by path.
type registry struct {
	mu       sync.Mutex
	files    map[string][]byte
	requests map[string]int
}

func newRegistry(t *testing.T, files map[string][]byte) (*registry, *httptest.Server) {
	r := &registry{files: files, requests: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests[req.URL.Path]++
		body, ok := r.files[req.URL.Path]
		r.mu.Unlock()
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return r, server
}

func fetcher(t *testing.T, server *httptest.Server, platform Platform) *Fetcher {
	t.Helper()
	f, err := New(filepath.Join(t.TempDir(), "cache"), platform)
	if err != nil {
		t.Fatal(err)
	}
	f.registry = server.URL
	return f
}

type entry map[string]any

func lock(t *testing.T, packages map[string]entry) []byte {
	t.Helper()
	all := map[string]any{"": map[string]any{"name": "app", "version": "1.0.0"}}
	for name, e := range packages {
		all[name] = e
	}
	data, err := json.Marshal(map[string]any{"lockfileVersion": 3, "packages": all})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

var linuxARM = Platform{OS: "linux", CPU: "arm64", Libc: "glibc"}

func TestFetchVerifiesAndCaches(t *testing.T) {
	ms, express := []byte("ms tarball"), []byte("express tarball")
	reg, server := newRegistry(t, map[string][]byte{"/ms/-/ms-2.1.3.tgz": ms, "/express/-/express-5.1.0.tgz": express})
	f := fetcher(t, server, linuxARM)
	data := lock(t, map[string]entry{
		"node_modules/ms":      {"version": "2.1.3", "resolved": "https://registry.npmjs.org/ms/-/ms-2.1.3.tgz", "integrity": integrity(ms)},
		"node_modules/express": {"version": "5.1.0", "resolved": "https://registry.npmjs.org/express/-/express-5.1.0.tgz", "integrity": integrity(express)},
	})

	packages, err := f.Fetch(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("packages %+v", packages)
	}
	for _, p := range packages {
		var copied bytes.Buffer
		if err := Verify(p, &copied); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(copied.Bytes(), ms) && !bytes.Equal(copied.Bytes(), express) {
			t.Fatalf("cached %q", copied.String())
		}
		if info, _ := os.Stat(p.Path); info.Mode().Perm() != 0o600 {
			t.Fatalf("cache file mode %v", info.Mode())
		}
	}
	// A second build downloads nothing.
	if _, err := f.Fetch(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if reg.requests["/ms/-/ms-2.1.3.tgz"] != 1 || reg.requests["/express/-/express-5.1.0.tgz"] != 1 {
		t.Fatalf("requests %v", reg.requests)
	}
}

func TestFetchRefusesATamperedTarball(t *testing.T) {
	_, server := newRegistry(t, map[string][]byte{"/ms/-/ms-2.1.3.tgz": []byte("something else")})
	f := fetcher(t, server, linuxARM)
	data := lock(t, map[string]entry{
		"node_modules/ms": {"version": "2.1.3", "resolved": "https://registry.npmjs.org/ms/-/ms-2.1.3.tgz", "integrity": integrity([]byte("ms tarball"))},
	})
	if _, err := f.Fetch(context.Background(), data); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("err %v", err)
	}
	entries, _ := os.ReadDir(f.cache)
	if len(entries) != 0 {
		t.Fatalf("an unverified file stayed in the cache: %v", entries)
	}
}

func TestVerifyCatchesAChangedCacheFileAndForgetsIt(t *testing.T) {
	ms := []byte("ms tarball")
	_, server := newRegistry(t, map[string][]byte{"/ms/-/ms-2.1.3.tgz": ms})
	f := fetcher(t, server, linuxARM)
	packages, err := f.Fetch(context.Background(), lock(t, map[string]entry{
		"node_modules/ms": {"version": "2.1.3", "resolved": "https://registry.npmjs.org/ms/-/ms-2.1.3.tgz", "integrity": integrity(ms)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packages[0].Path, []byte("ms tarbalX"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(packages[0], &bytes.Buffer{}); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(packages[0].Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a changed cache file was kept")
	}
}

func TestFetchSkipsPackagesForOtherSystems(t *testing.T) {
	linux, mac := []byte("linux binary"), []byte("mac binary")
	reg, server := newRegistry(t, map[string][]byte{
		"/@esbuild/linux-arm64/-/linux-arm64-0.25.0.tgz":   linux,
		"/@esbuild/darwin-arm64/-/darwin-arm64-0.25.0.tgz": mac,
	})
	f := fetcher(t, server, linuxARM)
	packages, err := f.Fetch(context.Background(), lock(t, map[string]entry{
		"node_modules/@esbuild/linux-arm64": {"version": "0.25.0", "optional": true, "os": []string{"linux"}, "cpu": []string{"arm64"},
			"resolved": "https://registry.npmjs.org/@esbuild/linux-arm64/-/linux-arm64-0.25.0.tgz", "integrity": integrity(linux)},
		"node_modules/@esbuild/darwin-arm64": {"version": "0.25.0", "optional": true, "os": []string{"darwin"}, "cpu": []string{"arm64"},
			"resolved": "https://registry.npmjs.org/@esbuild/darwin-arm64/-/darwin-arm64-0.25.0.tgz", "integrity": integrity(mac)},
		"node_modules/@rollup/rollup-linux-arm64-musl": {"version": "4.0.0", "optional": true, "os": []string{"linux"}, "cpu": []string{"arm64"}, "libc": []string{"musl"},
			"resolved": "https://registry.npmjs.org/@rollup/rollup-linux-arm64-musl/-/rollup-linux-arm64-musl-4.0.0.tgz", "integrity": integrity([]byte("musl"))},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || len(reg.requests) != 1 || reg.requests["/@esbuild/linux-arm64/-/linux-arm64-0.25.0.tgz"] != 1 {
		t.Fatalf("packages %+v, requests %v", packages, reg.requests)
	}
}

func TestPlatformMatching(t *testing.T) {
	for _, c := range []struct {
		list  []string
		value string
		want  bool
	}{
		{nil, "linux", true},
		{[]string{"linux"}, "linux", true},
		{[]string{"darwin", "win32"}, "linux", false},
		{[]string{"!win32"}, "linux", true},
		{[]string{"!linux"}, "linux", false},
	} {
		if got := allows(c.list, c.value); got != c.want {
			t.Errorf("allows(%v, %q) = %v", c.list, c.value, got)
		}
	}
}

func TestFetchExplainsRegistryProblems(t *testing.T) {
	_, server := newRegistry(t, map[string][]byte{})
	f := fetcher(t, server, linuxARM)
	data := lock(t, map[string]entry{
		"node_modules/ghost": {"version": "1.0.0", "resolved": "https://registry.npmjs.org/ghost/-/ghost-1.0.0.tgz", "integrity": integrity([]byte("x"))},
	})
	if _, err := f.Fetch(context.Background(), data); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v", err)
	}
	for _, resolved := range []string{
		"https://evil.example/ghost/-/ghost-1.0.0.tgz",
		"http://registry.npmjs.org/ghost/-/ghost-1.0.0.tgz",
		"https://registry.npmjs.org/ghost/-/ghost-1.0.0.tgz?x=1",
		"https://registry.npmjs.org/../ghost-1.0.0.tgz",
	} {
		bad := lock(t, map[string]entry{"node_modules/ghost": {"version": "1.0.0", "resolved": resolved, "integrity": integrity([]byte("x"))}})
		if _, err := f.Fetch(context.Background(), bad); !errors.Is(err, ErrLockfile) {
			t.Errorf("%s: err %v", resolved, err)
		}
	}
}

func TestNewWantsAPrivateAbsoluteCache(t *testing.T) {
	if _, err := New("relative/cache", linuxARM); !errors.Is(err, ErrCache) {
		t.Fatalf("relative: %v", err)
	}
	open := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(open, linuxARM); !errors.Is(err, ErrCache) {
		t.Fatalf("world-readable: %v", err)
	}
}
