package githubsource

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

type item struct {
	name, body string
	kind       byte
}

// tarball builds a gzip tarball shaped like GitHub's: a global header, then
// everything under one top folder.
func tarball(t *testing.T, items ...item) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(zipped)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(writer.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": commit}}))
	must(writer.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "hello-0123456/", Mode: 0o755}))
	for _, it := range items {
		header := &tar.Header{Name: "hello-0123456/" + it.name, Mode: 0o644, Typeflag: it.kind}
		switch it.kind {
		case tar.TypeReg:
			header.Size = int64(len(it.body))
		case tar.TypeSymlink:
			header.Linkname = it.body
		}
		must(writer.WriteHeader(header))
		if it.kind == tar.TypeReg {
			_, err := writer.Write([]byte(it.body))
			must(err)
		}
	}
	must(writer.Close())
	must(zipped.Close())
	return buffer.Bytes()
}

func file(name, body string) item { return item{name: name, body: body, kind: tar.TypeReg} }

// github serves one tarball for octocat/hello at commit and 404s otherwise.
func github(t *testing.T, body []byte) (Fetcher, *[]string) {
	t.Helper()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/octocat/hello/tar.gz/"+commit {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return Fetcher{BaseURL: server.URL}, &paths
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFetchWritesTheCommitAndLeavesOutWhatIsNeverPublished(t *testing.T) {
	fetcher, _ := github(t, tarball(t,
		file("index.html", "<h1>hi</h1>"),
		file("css/site.css", "body{}"),
		file(".env", "SECRET=1"),
		file("node_modules/x/index.js", "x"),
		file("deploy.pem", "key"),
		item{name: "link", body: "/etc/passwd", kind: tar.TypeSymlink},
	))
	dir := t.TempDir()
	result, err := fetcher.Fetch(context.Background(), Repository{"octocat", "hello"}, commit, "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 || read(t, dir, "index.html") != "<h1>hi</h1>" || read(t, dir, "css/site.css") != "body{}" {
		t.Fatalf("result %+v", result)
	}
	if want := []string{".env", "node_modules", "deploy.pem", "link"}; !slices.Equal(result.Skipped, want) {
		t.Fatalf("skipped %v, want %v", result.Skipped, want)
	}
	for _, name := range []string{".env", "node_modules", "deploy.pem", "link"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s was written", name)
		}
	}
}

func TestFetchTakesOnlyTheNamedFolder(t *testing.T) {
	fetcher, _ := github(t, tarball(t, file("README.md", "x"), file("site/index.html", "site"), file("sites/index.html", "other")))
	dir := t.TempDir()
	result, err := fetcher.Fetch(context.Background(), Repository{"octocat", "hello"}, commit, "site", dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || read(t, dir, "index.html") != "site" {
		t.Fatalf("result %+v", result)
	}
	if _, err := fetcher.Fetch(context.Background(), Repository{"octocat", "hello"}, commit, "docs", t.TempDir()); err == nil || !strings.Contains(err.Error(), `no folder "docs"`) {
		t.Fatalf("missing folder: %v", err)
	}
}

func TestFetchExplainsAMissingOrPrivateRepository(t *testing.T) {
	fetcher, paths := github(t, tarball(t, file("index.html", "x")))
	_, err := fetcher.Fetch(context.Background(), Repository{"octocat", "secret"}, commit, "", t.TempDir())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v", err)
	}
	if (*paths)[0] != "/octocat/secret/tar.gz/"+commit {
		t.Fatalf("asked for %v", *paths)
	}
}

func TestFetchRefusesUnsafePaths(t *testing.T) {
	fetcher, _ := github(t, tarball(t, file("../escape.html", "x")))
	dir := t.TempDir()
	if _, err := fetcher.Fetch(context.Background(), Repository{"octocat", "hello"}, commit, "", dir); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a file escaped the folder")
	}
}

func TestFetchDoesNotFollowRedirects(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("followed a redirect")
	}))
	defer elsewhere.Close()
	server := httptest.NewServer(http.RedirectHandler(elsewhere.URL, http.StatusFound))
	defer server.Close()
	_, err := Fetcher{BaseURL: server.URL}.Fetch(context.Background(), Repository{"octocat", "hello"}, commit, "", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("err %v", err)
	}
}

func TestParseInputs(t *testing.T) {
	for input, want := range map[string]string{
		"octocat/hello":                         "octocat/hello",
		"https://github.com/octocat/hello":      "octocat/hello",
		"github.com/octocat/hello.git":          "octocat/hello",
		"https://www.github.com/octocat/hello/": "octocat/hello",
	} {
		repository, err := ParseRepository(input)
		if err != nil || repository.String() != want {
			t.Errorf("%q: %v, %v", input, repository, err)
		}
	}
	for _, input := range []string{"", "octocat", "octocat/hello/tree/main", "-bad/hello", "octocat/..", "https://gitlab.com/a/b"} {
		if _, err := ParseRepository(input); err == nil {
			t.Errorf("%q was accepted", input)
		}
	}
	if got, err := ParseCommit(strings.ToUpper(commit)); err != nil || got != commit {
		t.Errorf("commit %q, %v", got, err)
	}
	for _, input := range []string{"main", "0123456", commit + "0"} {
		if _, err := ParseCommit(input); err == nil {
			t.Errorf("commit %q was accepted", input)
		}
	}
	for input, want := range map[string]string{"": "", "/": "", "site/": "site", "apps/web": "apps/web"} {
		if got, err := ParsePath(input); err != nil || got != want {
			t.Errorf("path %q: %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"..", "../x", "a/../b", `a\b`} {
		if _, err := ParsePath(input); err == nil {
			t.Errorf("path %q was accepted", input)
		}
	}
}
