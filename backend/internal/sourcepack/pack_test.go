package sourcepack

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

// write creates files under dir; a name ending in "/" makes a folder.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// entries lists what an archive holds, as the server would see it.
func entries(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	found := map[string]string{}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return found
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg || header.Format != tar.FormatUSTAR || header.Mode != 0o644 || header.ModTime.Unix() != 0 {
			t.Fatalf("%s: unexpected header %+v", header.Name, header)
		}
		body, _ := io.ReadAll(reader)
		found[header.Name] = string(body)
	}
}

func problem(t *testing.T, err error, contains string) {
	t.Helper()
	if !IsProblem(err) || !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected a problem mentioning %q, got %v", contains, err)
	}
}

func TestPacksSiteAndLeavesOutWhatShouldNeverBePublished(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{
		"index.html":              "<h1>hi</h1>",
		"css/site.css":            "body{}",
		".env":                    "SECRET=1",
		".git/config":             "[core]",
		".DS_Store":               "junk",
		"node_modules/x/index.js": "x",
		"id_rsa":                  "key",
		"certs/server.pem":        "key",
	})
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")
	if err := os.WriteFile(outside, []byte("not yours"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.txt")); err != nil {
		t.Fatal(err)
	}

	archive, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := entries(t, archive.Tar)
	want := map[string]string{"index.html": "<h1>hi</h1>", "css/site.css": "body{}"}
	if len(got) != len(want) || got["index.html"] != want["index.html"] || got["css/site.css"] != want["css/site.css"] {
		t.Fatalf("archive holds %v, want %v", got, want)
	}
	if archive.Kind != KindStatic || archive.Files != 2 || archive.Bytes != int64(len("<h1>hi</h1>")+len("body{}")) {
		t.Fatalf("unexpected summary %+v", archive)
	}
	skipped := map[string]Reason{}
	for _, skip := range archive.Skipped {
		skipped[skip.Path] = skip.Reason
	}
	for path, reason := range map[string]Reason{
		".env": ReasonHidden, ".git": ReasonHidden, ".DS_Store": ReasonHidden, "node_modules": ReasonDependencies,
		"id_rsa": ReasonPrivate, "certs/server.pem": ReasonPrivate, "linked.txt": ReasonLink,
	} {
		if skipped[path] != reason {
			t.Errorf("%s: skipped as %q, want %q (all: %v)", path, skipped[path], reason, archive.Skipped)
		}
	}
	if err := Check(context.Background(), archive); err != nil {
		t.Fatalf("the server's source stage refused the archive: %v", err)
	}
}

func TestPackingIsReproducible(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"index.html": "a", "b/c.txt": "c", "b/a.txt": "a"})
	first, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// A file touched in between must not change the archive.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "index.html"), later, later); err != nil {
		t.Fatal(err)
	}
	second, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Tar, second.Tar) {
		t.Fatal("packing the same folder twice gave different bytes")
	}
}

// Every name the server refuses as sensitive must be left out before upload,
// or a deploy of an ordinary project folder would fail.
func TestEverySensitiveNameIsSkipped(t *testing.T) {
	for _, name := range []string{
		".git", ".ssh", ".aws", ".azure", ".kube", ".npmrc", ".pypirc", ".netrc", ".env", ".env.local",
		"credentials.json", "ID_RSA", "id_ed25519", "id_ecdsa", "tls.pem", "server.key", "cert.p12", "cert.pfx",
	} {
		if SkipReason(name) == "" {
			t.Errorf("%s is refused by the server but would be uploaded", name)
		}
		if !errors.Is(source.CheckName(name), source.ErrSensitive) {
			t.Errorf("%s: the server no longer treats it as sensitive; update this list", name)
		}
	}
}

func TestLongPathsUseTheUSTARPrefix(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("folder-name/", 9) + "a-file-with-a-long-name.txt" // over 100 bytes
	write(t, dir, map[string]string{"index.html": "x", long: "deep"})
	archive, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if entries(t, archive.Tar)[long] != "deep" {
		t.Fatal("the long path did not survive packing")
	}
	if err := Check(context.Background(), archive); err != nil {
		t.Fatalf("the server's source stage refused a long path: %v", err)
	}
}

func TestLimitsNameTheProblem(t *testing.T) {
	t.Run("too many files", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string]string{"index.html": "x"}
		for i := range source.MaxEntries {
			files[fmt.Sprintf("many/f%03d.txt", i)] = "x"
		}
		write(t, dir, files)
		_, err := Pack(context.Background(), dir)
		problem(t, err, "more than 256 files")
	})
	t.Run("file too big", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, map[string]string{"index.html": "x", "video.mp4": strings.Repeat("v", int(source.MaxFileBytes)+1)})
		_, err := Pack(context.Background(), dir)
		problem(t, err, `"video.mp4"`)
	})
	t.Run("folder too big", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string]string{"index.html": "x"}
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			files[name+".bin"] = strings.Repeat("z", int(source.MaxFileBytes)-1)
		}
		write(t, dir, files)
		_, err := Pack(context.Background(), dir)
		problem(t, err, "Sprout takes up to 8.0 MiB")
	})
	t.Run("bad name", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, map[string]string{"index.html": "x", "notes.": "x"})
		_, err := Pack(context.Background(), dir)
		problem(t, err, `"notes."`)
	})
}

func TestFolderWithoutAnEntryPoint(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"README.md": "x", "dist/index.html": "x"})
	_, err := Pack(context.Background(), dir)
	problem(t, err, `deploy that folder instead`)

	dir = t.TempDir()
	write(t, dir, map[string]string{"Index.HTML": "x"})
	_, err = Pack(context.Background(), dir)
	problem(t, err, `Rename "Index.HTML"`)
}

func TestEmptyAndAllHiddenFolders(t *testing.T) {
	_, err := Pack(context.Background(), t.TempDir())
	problem(t, err, "empty")

	dir := t.TempDir()
	write(t, dir, map[string]string{".env": "x", ".git/HEAD": "x"})
	_, err = Pack(context.Background(), dir)
	problem(t, err, "nothing to deploy")
}

func TestCheckReportsTheServersNodeRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"package.json": `{"name":"app","private":true,"dependencies":{"express":"^5.0.0"}}`, "server.js": "x"})
	archive, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if archive.Kind != KindNode {
		t.Fatalf("kind %q, want node", archive.Kind)
	}
	if err := Check(context.Background(), archive); !errors.Is(err, nodeapp.ErrDependencies) {
		t.Fatalf("expected the server's packages rule, got %v", err)
	}

	dir = t.TempDir()
	write(t, dir, map[string]string{"package.json": `{"name":"app","private":true}`, "server.js": "x"})
	archive, err = Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var contract *nodeapp.ContractError
	if err := Check(context.Background(), archive); !errors.As(err, &contract) || !strings.HasPrefix(contract.Code, "node_contract_") {
		t.Fatalf("expected a named Node contract rule, got %v", err)
	}
}

func TestSkipReasonMatchesTheBrowser(t *testing.T) {
	// The same table as skipReason in frontend/src/lib/folder-archive.ts.
	cases := map[string]Reason{
		"._icon": ReasonJunk, ".env": ReasonHidden, "node_modules": ReasonDependencies, "Thumbs.db": ReasonJunk,
		"__MACOSX": ReasonJunk, "desktop.ini": ReasonJunk, "credentials.json": ReasonPrivate, "key.PEM": ReasonPrivate,
		"index.html": "", "src": "",
	}
	for name, want := range cases {
		if got := SkipReason(name); got != want {
			t.Errorf("SkipReason(%q) = %q, want %q", name, got, want)
		}
	}
	if !slices.Contains(junkNames, "thumbs.db") {
		t.Fatal("junk names must be lower case")
	}
}

func TestWebsiteNamesAreNamed(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"index.html": "x", "images/my photo.png": "x"})
	_, err := Pack(context.Background(), dir)
	problem(t, err, `"images/my photo.png"`)

	// A Node app is not served file by file, so the stricter rule does not apply.
	dir = t.TempDir()
	write(t, dir, map[string]string{"package.json": "{}", "notes/my notes.txt": "x"})
	if _, err := Pack(context.Background(), dir); err != nil {
		t.Fatalf("a Node app was held to the website name rule: %v", err)
	}
}

func TestMissingEntryPointSaysWhatItFound(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"README.md": "x", "src/app.js": "x", "src/util.js": "x"})
	_, err := Pack(context.Background(), dir)
	problem(t, err, `Found: "README.md", "src"`)
}

func TestBuiltFolderIsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"package.json": "{}", "dist/index.html": "x"})
	archive, err := Pack(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if archive.BuiltFolder != "dist" {
		t.Fatalf("BuiltFolder %q, want dist", archive.BuiltFolder)
	}
}
