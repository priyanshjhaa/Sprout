package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

func siteArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	tree := fstest.MapFS{}
	for name, body := range files {
		tree[name] = &fstest.MapFile{Data: []byte(body)}
	}
	archive, err := staticsite.Pack(context.Background(), tree)
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

type countingLoader struct {
	archive []byte
	loads   atomic.Int32
	failFor atomic.Int32 // fail this many loads first
}

func (l *countingLoader) Load(context.Context, artifact.Descriptor) ([]byte, error) {
	l.loads.Add(1)
	if l.failFor.Add(-1) >= 0 {
		return nil, artifact.ErrStorage
	}
	return l.archive, nil
}

func staticProxy(t *testing.T, loader ArtifactLoader, lifecycle string) (*Proxy, *scriptedDriver) {
	t.Helper()
	driver := newScripted("127.0.0.1:1")
	s := newSupervisor(t, driver, staticArtifacts{}, 3)
	route := Route{Release: testRelease, Lifecycle: lifecycle, Live: true, Runtime: RuntimeStatic}
	p, err := NewProxy("localhost", func(context.Context, string, string) (Route, error) { return route, nil }, s, loader, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	return p, driver
}

func request(p *Proxy, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Host = "shop.acme.localhost"
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	return w
}

func TestStaticSiteServedFromTheArtifact(t *testing.T) {
	loader := &countingLoader{archive: siteArchive(t, map[string]string{
		"index.html": "<h1>home</h1>", "app.js": "console.log(1)", "docs/index.html": "<p>docs</p>", "blob.bin": "0123456789", "404.html": "<p>custom missing</p>",
	})}
	p, driver := staticProxy(t, loader, "active")

	home := request(p, http.MethodGet, "/", nil)
	if home.Code != 200 || home.Body.String() != "<h1>home</h1>" || home.Header().Get("Content-Type") != "text/html; charset=utf-8" ||
		home.Header().Get("X-Content-Type-Options") != "nosniff" || home.Header().Get("ETag") == "" {
		t.Fatalf("home: %d %q %v", home.Code, home.Body.String(), home.Header())
	}
	if js := request(p, http.MethodGet, "/app.js", nil); js.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("js type %q", js.Header().Get("Content-Type"))
	}
	if bin := request(p, http.MethodGet, "/blob.bin", nil); bin.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("unknown type %q", bin.Header().Get("Content-Type"))
	}
	if docs := request(p, http.MethodGet, "/docs/", nil); docs.Body.String() != "<p>docs</p>" {
		t.Fatalf("docs: %q", docs.Body.String())
	}

	// Conditional, HEAD and range requests behave like a real file server.
	etag := home.Header().Get("ETag")
	if cached := request(p, http.MethodGet, "/", map[string]string{"If-None-Match": etag}); cached.Code != http.StatusNotModified {
		t.Fatalf("conditional: %d", cached.Code)
	}
	if head := request(p, http.MethodHead, "/", nil); head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Type") == "" {
		t.Fatalf("head: %d %q", head.Code, head.Body.String())
	}
	if part := request(p, http.MethodGet, "/blob.bin", map[string]string{"Range": "bytes=2-4"}); part.Code != http.StatusPartialContent || part.Body.String() != "234" {
		t.Fatalf("range: %d %q", part.Code, part.Body.String())
	}

	// The site's own 404 page is used, with a 404 status and the same safety headers.
	missing := request(p, http.MethodGet, "/nope", nil)
	if missing.Code != 404 || missing.Body.String() != "<p>custom missing</p>" || missing.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("404: %d %q", missing.Code, missing.Body.String())
	}
	if driver.starts.Load() != 0 || driver.prepares.Load() != 0 {
		t.Fatal("a static site started or prepared a container")
	}
	if loader.loads.Load() != 1 {
		t.Fatalf("artifact loaded %d times for many requests, want 1", loader.loads.Load())
	}
}

func TestStaticRefusesWritesAndTraversal(t *testing.T) {
	p, _ := staticProxy(t, &countingLoader{archive: siteArchive(t, map[string]string{"index.html": "home", "inner/page.html": "page"})}, "active")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if w := request(p, method, "/", nil); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s: %d", method, w.Code)
		}
	}
	for _, target := range []string{"/%2e%2e/etc/passwd", "/..%2f..%2fetc/passwd", "/a/../index.html", "//index.html", "/index.html/", "/%00", "/index.html%00.png", "/.env", "/%5cindex.html"} {
		if w := request(p, http.MethodGet, target, nil); w.Code == 200 {
			t.Errorf("%s served: %q", target, w.Body.String())
		}
	}
}

func TestStaticRespectsLifecycle(t *testing.T) {
	archive := siteArchive(t, map[string]string{"index.html": "home"})
	for lifecycle, want := range map[string]int{"active": 200, "paused": 503, "archived": 410} {
		p, _ := staticProxy(t, &countingLoader{archive: archive}, lifecycle)
		if w := request(p, http.MethodGet, "/", nil); w.Code != want {
			t.Errorf("%s: %d, want %d", lifecycle, w.Code, want)
		}
	}
}

func TestStaticCacheHealsFromFailuresAndIsBounded(t *testing.T) {
	loader := &countingLoader{archive: siteArchive(t, map[string]string{"index.html": "home"})}
	loader.failFor.Store(1)
	p, _ := staticProxy(t, loader, "active")
	if w := request(p, http.MethodGet, "/", nil); w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "storage") {
		t.Fatalf("storage failure: %d %q", w.Code, w.Body.String())
	}
	if w := request(p, http.MethodGet, "/", nil); w.Code != 200 {
		t.Fatalf("a transient failure was cached: %d", w.Code)
	}

	cache := newSiteCache(loader)
	for i := 0; i < maxCachedSites*3; i++ {
		route := Route{Release: Release{ApplicationID: testRelease.ApplicationID, DeploymentID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i)}}
		if _, err := cache.get(context.Background(), route); err != nil {
			t.Fatal(err)
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) > maxCachedSites || len(cache.order) > maxCachedSites {
		t.Fatalf("cache holds %d entries, want at most %d", len(cache.entries), maxCachedSites)
	}
}

func TestStaticCorruptArtifactIsNotServed(t *testing.T) {
	p, _ := staticProxy(t, &countingLoader{archive: []byte(strings.Repeat("garbage", 200))}, "active")
	if w := request(p, http.MethodGet, "/", nil); w.Code != http.StatusBadGateway {
		t.Fatalf("corrupt artifact: %d", w.Code)
	}
}
