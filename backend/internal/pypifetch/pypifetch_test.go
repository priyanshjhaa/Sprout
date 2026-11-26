package pypifetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func sum(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// file is one file of a fake PyPI release.
type file struct {
	name, kind string
	data       []byte
	url        string // overrides the usual files.pythonhosted.org URL
}

// pypi is a fake PyPI: a JSON API listing files and a file server.
type pypi struct {
	mu       sync.Mutex
	releases map[string][]file // by "name/version"
	requests map[string]int
}

func newPyPI(t *testing.T, releases map[string][]file) (*pypi, *httptest.Server) {
	p := &pypi{releases: releases, requests: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.requests[r.URL.Path]++
		p.mu.Unlock()
		if release, ok := strings.CutPrefix(r.URL.Path, "/pypi/"); ok {
			files, found := p.releases[strings.TrimSuffix(release, "/json")]
			if !found {
				http.NotFound(w, r)
				return
			}
			var answer struct {
				URLs []map[string]any `json:"urls"`
			}
			for _, f := range files {
				address := "https://files.pythonhosted.org/packages/ab/cd/" + f.name
				if f.url != "" {
					address = f.url
				}
				answer.URLs = append(answer.URLs, map[string]any{"filename": f.name, "url": address, "packagetype": f.kind,
					"digests": map[string]string{"sha256": sum(f.data)}})
			}
			json.NewEncoder(w).Encode(answer)
			return
		}
		for _, files := range p.releases {
			for _, f := range files {
				if r.URL.Path == "/packages/ab/cd/"+f.name {
					w.Write(f.data)
					return
				}
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return p, server
}

var linuxARM = Platform{PythonMinor: 13, Arch: "aarch64", GlibcMajor: 2, GlibcMinor: 36}

func fetcher(t *testing.T, server *httptest.Server) *Fetcher {
	t.Helper()
	f, err := New(filepath.Join(t.TempDir(), "cache"), linuxARM)
	if err != nil {
		t.Fatal(err)
	}
	f.metadata, f.files = server.URL, server.URL
	return f
}

func TestFetchPicksTheWheelForThisContainerAndCachesIt(t *testing.T) {
	flask := []byte("flask wheel")
	armWheel, macWheel, sdist := []byte("arm markupsafe"), []byte("mac markupsafe"), []byte("markupsafe sdist")
	registry, server := newPyPI(t, map[string][]file{
		"flask/3.1.3": {{name: "flask-3.1.3-py3-none-any.whl", kind: "bdist_wheel", data: flask}},
		"markupsafe/3.0.4": {
			{name: "markupsafe-3.0.4.tar.gz", kind: "sdist", data: sdist},
			{name: "markupsafe-3.0.4-cp313-cp313-macosx_11_0_arm64.whl", kind: "bdist_wheel", data: macWheel},
			{name: "markupsafe-3.0.4-cp313-cp313-manylinux2014_aarch64.manylinux_2_17_aarch64.whl", kind: "bdist_wheel", data: armWheel},
		},
	})
	f := fetcher(t, server)
	requirements := []byte("# generated\nFlask==3.1.3 \\\n    --hash=sha256:" + sum(flask) + "\n" +
		"MarkupSafe==3.0.4 \\\n    --hash=sha256:" + sum(sdist) + " \\\n    --hash=sha256:" + sum(macWheel) + " \\\n    --hash=sha256:" + sum(armWheel) + "\n    # via jinja2\n")

	packages, err := f.Fetch(context.Background(), requirements)
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
		if p.Filename == "markupsafe-3.0.4-cp313-cp313-manylinux2014_aarch64.manylinux_2_17_aarch64.whl" && !bytes.Equal(copied.Bytes(), armWheel) {
			t.Fatalf("wrong markupsafe wheel %q", copied.String())
		}
	}
	// A second build needs no network at all: the cache names each wheel by
	// its hash and keeps its filename.
	before := len(registry.requests)
	total := 0
	for _, n := range registry.requests {
		total += n
	}
	if _, err := f.Fetch(context.Background(), requirements); err != nil {
		t.Fatal(err)
	}
	after := 0
	for _, n := range registry.requests {
		after += n
	}
	if after != total || len(registry.requests) != before {
		t.Fatalf("a cached build made requests: %v", registry.requests)
	}
}

func TestFetchRefusesATamperedWheel(t *testing.T) {
	_, server := newPyPI(t, map[string][]file{
		"flask/3.1.3": {{name: "flask-3.1.3-py3-none-any.whl", kind: "bdist_wheel", data: []byte("evil")}},
	})
	f := fetcher(t, server)
	// The registry claims the hash of "evil", but requirements.txt lists another.
	requirements := []byte("flask==3.1.3 --hash=sha256:" + sum([]byte("evil")) + "\n")
	server.Config.Handler = tamper(server.Config.Handler)
	if _, err := f.Fetch(context.Background(), requirements); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("err %v", err)
	}
	entries, _ := os.ReadDir(f.cache.Dir())
	if len(entries) != 0 {
		t.Fatalf("an unverified wheel stayed in the cache: %v", entries)
	}
}

// tamper serves different bytes for every file than the metadata promised.
func tamper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/packages/") {
			w.Write([]byte("something else"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestFetchExplainsMissingWheels(t *testing.T) {
	sdist, mac := []byte("sdist"), []byte("mac only")
	_, server := newPyPI(t, map[string][]file{
		"oldlib/1.0":  {{name: "oldlib-1.0.tar.gz", kind: "sdist", data: sdist}},
		"maconly/1.0": {{name: "maconly-1.0-cp313-cp313-macosx_11_0_arm64.whl", kind: "bdist_wheel", data: mac}},
	})
	f := fetcher(t, server)
	if _, err := f.Fetch(context.Background(), []byte("oldlib==1.0 --hash=sha256:"+sum(sdist)+"\n")); !errors.Is(err, ErrNoWheel) {
		t.Fatalf("source only: %v", err)
	}
	if _, err := f.Fetch(context.Background(), []byte("ghost==1.0 --hash=sha256:"+sum(sdist)+"\n")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// With a marker the line may not apply here, so pip decides.
	packages, err := f.Fetch(context.Background(), []byte(`maconly==1.0 ; sys_platform == "darwin" --hash=sha256:`+sum(mac)+"\n"))
	if err != nil || len(packages) != 0 {
		t.Fatalf("marker: %+v %v", packages, err)
	}
}

func TestFetchRefusesFilesFromAnywhereElse(t *testing.T) {
	data := []byte("wheel")
	_, server := newPyPI(t, map[string][]file{
		"flask/3.1.3": {{name: "flask-3.1.3-py3-none-any.whl", kind: "bdist_wheel", data: data, url: "https://evil.example/flask-3.1.3-py3-none-any.whl"}},
	})
	f := fetcher(t, server)
	if _, err := f.Fetch(context.Background(), []byte("flask==3.1.3 --hash=sha256:"+sum(data)+"\n")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err %v", err)
	}
}

func TestParseRequirements(t *testing.T) {
	hash := strings.Repeat("a", 64)
	good := "Django==5.2 --hash=sha256:" + hash + "\nzope.interface[test] == 7.0 \\\n  --hash=sha256:" + hash + "\n" +
		"tomli==2.0 ; python_version < \"3.11\" --hash=sha256:" + hash + "\n\n# comment\n"
	parsed, err := ParseRequirements([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 3 || parsed[0].Name != "django" || parsed[1].Name != "zope-interface" || parsed[1].Version != "7.0" || !parsed[2].Marker {
		t.Fatalf("parsed %+v", parsed)
	}
	for _, bad := range []string{
		"flask==3.1.3\n",                                // no hash
		"flask>=3 --hash=sha256:" + hash,                // not pinned
		"flask==3.* --hash=sha256:" + hash,              // wildcard
		"flask==3.1 --hash=md5:abc",                     // weak hash
		"flask==3.1 --hash=sha256:" + hash + " --pre",   // other option
		"-r other.txt",                                  // include
		"--index-url https://evil.example/simple",       // another index
		"--extra-index-url https://evil.example/simple", // dependency confusion
		"-e .", // editable
		"flask @ https://evil.example/flask.whl --hash=sha256:" + hash,
		"flask==3.1 --hash=sha256:" + hash + "\nflask==3.1 --hash=sha256:" + hash, // twice
	} {
		if _, err := ParseRequirements([]byte(bad)); !errors.Is(err, ErrRequirements) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestPlatformAcceptsWheels(t *testing.T) {
	for name, want := range map[string]bool{
		"flask-3.1.3-py3-none-any.whl":                               true,
		"six-1.17.0-py2.py3-none-any.whl":                            true,
		"markupsafe-3.0.4-cp313-cp313-manylinux_2_17_aarch64.whl":    true,
		"cryptography-46.0-cp311-abi3-manylinux_2_34_aarch64.whl":    true,
		"cryptography-46.0-cp311-abi3-manylinux_2_39_aarch64.whl":    false, // needs a newer glibc
		"markupsafe-3.0.4-cp313-cp313-manylinux_2_17_x86_64.whl":     false, // other architecture
		"markupsafe-3.0.4-cp313-cp313-musllinux_1_2_aarch64.whl":     false, // Alpine
		"markupsafe-3.0.4-cp312-cp312-manylinux_2_17_aarch64.whl":    false, // other Python
		"markupsafe-3.0.4-cp313-cp313t-manylinux_2_17_aarch64.whl":   false, // free-threaded build
		"pydantic_core-2.41.5-cp313-cp313-manylinux2014_aarch64.whl": true,
		"numpy-2.3.0-1-cp313-cp313-manylinux_2_28_aarch64.whl":       true,  // with a build tag
		"numpy-2.3.0-cp313-cp313-linux_aarch64.whl":                  false, // not portable
		"numpy-2.3.0-cp313-cp313-win_amd64.whl":                      false,
		"thing-1.0-py314-none-any.whl":                               false,
		"bad-name.whl":                                               false,
	} {
		if got := linuxARM.accepts(name); got != want {
			t.Errorf("accepts(%s) = %v", name, got)
		}
	}
}
