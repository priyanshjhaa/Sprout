package staticsite

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

func tree(files map[string]string) fstest.MapFS {
	tree := fstest.MapFS{}
	for name, body := range files {
		tree[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return tree
}

func TestValidateContract(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		files map[string]string
		ok    bool
	}{
		"minimal":             {map[string]string{"index.html": "<h1>hi</h1>"}, true},
		"nested assets":       {map[string]string{"index.html": "x", "css/site.css": "a{}", "img/logo-2.png": "p", "docs/index.html": "d"}, true},
		"no index":            {map[string]string{"about.html": "x"}, false},
		"index in subfolder":  {map[string]string{"site/index.html": "x"}, false},
		"node app not static": {map[string]string{"index.html": "x", "package.json": "{}"}, false},
		"dotfile":             {map[string]string{"index.html": "x", ".htaccess": "x"}, false},
		"space in name":       {map[string]string{"index.html": "x", "my file.png": "x"}, false},
		"unicode name":        {map[string]string{"index.html": "x", "café.html": "x"}, false},
		"percent in name":     {map[string]string{"index.html": "x", "a%2e.html": "x"}, false},
		"backslash":           {map[string]string{"index.html": "x", `a\b.html`: "x"}, false},
		"overlong segment":    {map[string]string{"index.html": "x", strings.Repeat("a", 129) + ".html": "x"}, false},
		"too deep":            {map[string]string{"index.html": "x", strings.Repeat("d/", 17) + "f.html": "x"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate(ctx, tree(tc.files))
			if tc.ok && err != nil || !tc.ok && !errors.Is(err, ErrContract) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
	if err := Validate(ctx, nil); !errors.Is(err, ErrContract) {
		t.Fatalf("nil tree: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := Validate(cancelled, tree(map[string]string{"index.html": "x"})); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestPackThenLoadRoundTrip(t *testing.T) {
	archive, err := Pack(context.Background(), tree(map[string]string{"index.html": "<p>home</p>", "a/b.css": "b{}", "a/index.html": "<p>a</p>"}))
	if err != nil {
		t.Fatal(err)
	}
	site, err := Load(archive)
	if err != nil || site.Size() != 3 {
		t.Fatalf("load: %v size=%d", err, site.Size())
	}
	for path, want := range map[string]struct {
		body, kind string
		ok         bool
	}{
		"/":             {"<p>home</p>", "text/html; charset=utf-8", true},
		"/index.html":   {"<p>home</p>", "text/html; charset=utf-8", true},
		"/a/":           {"<p>a</p>", "text/html; charset=utf-8", true},
		"/a/b.css":      {"b{}", "text/css; charset=utf-8", true},
		"/a":            {"", "", false}, // no redirect magic: a directory needs its slash
		"/missing":      {"", "", false},
		"/a/../a/b.css": {"", "", false},
		"//a/b.css":     {"", "", false},
		"/a/./b.css":    {"", "", false},
		"/a\\b.css":     {"", "", false},
		"a/b.css":       {"", "", false},
		"/a/b.css\x00":  {"", "", false},
	} {
		file, ok := site.Lookup(path)
		if ok != want.ok || ok && (string(file.Data) != want.body || file.ContentType != want.kind) {
			t.Errorf("Lookup(%q) = %q %q %v", path, file.Data, file.ContentType, ok)
		}
	}
	if file, _ := site.Lookup("/a/b.css"); file.ContentType == "" {
		t.Fatal("content type missing")
	}
}

func TestContentTypesAreFixedAndUnknownIsOpaque(t *testing.T) {
	if contentType("x.JS") != "text/javascript; charset=utf-8" || contentType("x.svg") != "image/svg+xml" {
		t.Fatal("known types wrong")
	}
	for _, name := range []string{"x.php", "x.exe", "x", "x.unknown"} {
		if contentType(name) != "application/octet-stream" {
			t.Errorf("%s served as %s", name, contentType(name))
		}
	}
}

// Load distrusts the artifact: a tampered store must not yield traversal,
// links, duplicates or oversized allocations.
func TestLoadRejectsMaliciousArchives(t *testing.T) {
	build := func(entries ...tar.Header) []byte {
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)
		for _, header := range entries {
			header := header
			if header.Typeflag == 0 {
				header.Typeflag = tar.TypeReg
			}
			if err := writer.WriteHeader(&header); err != nil {
				t.Fatal(err)
			}
			_, _ = writer.Write(bytes.Repeat([]byte("x"), int(header.Size)))
		}
		_ = writer.Close()
		return buffer.Bytes()
	}
	index := tar.Header{Name: "index.html", Size: 1, Mode: 0600}
	for name, archive := range map[string][]byte{
		"traversal": build(index, tar.Header{Name: "../evil.html", Size: 1}),
		"absolute":  build(index, tar.Header{Name: "/etc/passwd", Size: 1}),
		"symlink":   build(index, tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}),
		"hardlink":  build(index, tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "index.html"}),
		"device":    build(index, tar.Header{Name: "dev", Typeflag: tar.TypeChar}),
		"duplicate": build(index, index),
		"no index":  build(tar.Header{Name: "other.html", Size: 1}),
		"dotfile":   build(index, tar.Header{Name: ".env", Size: 1}),
		"huge file": build(index, tar.Header{Name: "big.bin", Size: source.MaxFileBytes + 1}),
		"garbage":   []byte(strings.Repeat("not a tar", 100)),
	} {
		if _, err := Load(archive); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// The total size is bounded across files, not only per file.
	var many []tar.Header
	many = append(many, index)
	for i := 0; i < 5; i++ {
		many = append(many, tar.Header{Name: "f" + string(rune('a'+i)) + ".bin", Size: source.MaxFileBytes})
	}
	if _, err := Load(build(many...)); !errors.Is(err, source.ErrLimit) {
		t.Errorf("total size not bounded: %v", err)
	}
}

// testdata/folder-packed.tar is what the dashboard's browser-side packer produces
// for a dropped folder (tests/folder-archive.test.mjs regenerates and checks it).
// The real extractor and the static contract must accept exactly those bytes, or
// "drop a folder" would fail at the server for every user.
func TestBrowserPackedFolderIsAcceptedAndServable(t *testing.T) {
	data, err := os.ReadFile("testdata/folder-packed.tar")
	if err != nil {
		t.Fatal(err)
	}
	var packed []byte
	summary, err := source.WithArchive(context.Background(), bytes.NewReader(data), func(ctx context.Context, files fs.FS) error {
		if err := Validate(ctx, files); err != nil {
			return err
		}
		packed, err = Pack(ctx, files)
		return err
	})
	if err != nil {
		t.Fatalf("the server rejected the browser's archive: %v", err)
	}
	if summary.Files != 5 {
		t.Fatalf("files = %d, want 5 (the .DS_Store must never have been packed)", summary.Files)
	}
	site, err := Load(packed)
	if err != nil {
		t.Fatal(err)
	}
	long := "/assets/" + strings.Repeat("a", 40) + "/" + strings.Repeat("b", 40) + "/" + strings.Repeat("c", 30) + ".txt"
	for path, want := range map[string]string{"/": "<h1>Home</h1>", "/about/": "<h1>About</h1>", "/css/site.css": "h1{color:green}", "/404.html": "<h1>Missing</h1>", long: "deep file"} {
		file, ok := site.Lookup(path)
		if !ok || string(file.Data) != want {
			t.Errorf("Lookup(%q) = %q %v, want %q", path, file.Data, ok, want)
		}
	}
	if _, ok := site.Lookup("/.DS_Store"); ok {
		t.Error("junk file was served")
	}
}
