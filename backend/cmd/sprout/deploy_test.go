package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSprout is just enough of the API for a deploy: a workspace, its apps,
// and a build that advances one stage each time it is polled.
type fakeSprout struct {
	t           *testing.T
	mu          sync.Mutex
	localBuilds bool
	apps        []map[string]any
	uploads     [][]byte
	// script is the sequence of deployments returned by successive polls.
	script []map[string]any
	polls  int
	// onPoll runs on every poll, before answering.
	onPoll func()
}

func newFakeSprout(t *testing.T) (*fakeSprout, *httptest.Server) {
	f := &fakeSprout{t: t, localBuilds: true, script: liveScript()}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	return f, server
}

func stages(statuses ...string) []map[string]any {
	names := []string{"source", "build", "package", "provision", "release", "health_check"}
	out := make([]map[string]any, len(names))
	for i, name := range names {
		status := "pending"
		if i < len(statuses) {
			status = statuses[i]
		}
		out[i] = map[string]any{"name": name, "status": status, "failureCode": nil}
	}
	return out
}

func liveScript() []map[string]any {
	return []map[string]any{
		{"id": "d1", "status": "queued", "failureCode": nil, "stages": stages()},
		{"id": "d1", "status": "building", "failureCode": nil, "stages": stages("succeeded", "running")},
		{"id": "d1", "status": "live", "failureCode": nil, "stages": stages("succeeded", "succeeded", "succeeded", "succeeded", "succeeded", "succeeded")},
	}
}

func (f *fakeSprout) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+goodToken {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"authentication_required","message":"no","requestId":"r"}}`))
		return
	}
	reply := func(status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	const apps = "/api/v1/workspaces/ada-team/applications"
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/capabilities":
		reply(200, map[string]bool{"localBuilds": f.localBuilds})
	case r.Method == "GET" && r.URL.Path == "/api/v1/me":
		reply(200, map[string]any{"id": "u1", "email": "ada@example.com", "displayName": "Ada", "workspace": map[string]string{"id": "w1", "slug": "ada-team", "name": "Ada's team"}})
	case r.Method == "GET" && r.URL.Path == apps:
		reply(200, map[string]any{"applications": f.apps})
	case r.Method == "POST" && r.URL.Path == apps:
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		app := map[string]any{"id": "app-" + body["slug"], "name": body["name"], "slug": body["slug"], "lifecycle": "active", "url": nil}
		f.apps = append(f.apps, app)
		reply(201, app)
	case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/deployments"):
		if r.Header.Get("Content-Type") != "application/x-tar" {
			reply(415, map[string]any{"error": map[string]string{"code": "unsupported_media_type", "message": "tar only", "requestId": "r"}})
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.uploads = append(f.uploads, body)
		reply(201, f.script[0])
	case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/deployments/d1"):
		if f.onPoll != nil {
			f.onPoll()
		}
		f.polls++
		reply(200, f.script[min(f.polls, len(f.script)-1)])
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, apps+"/"):
		id := strings.TrimPrefix(r.URL.Path, apps+"/")
		for _, app := range f.apps {
			if app["id"] == id {
				app["url"] = "http://" + app["slug"].(string) + ".ada-team.localhost:8081"
				reply(200, app)
				return
			}
		}
		reply(404, map[string]any{"error": map[string]string{"code": "application_not_found", "message": "no", "requestId": "r"}})
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

func deployHarness(t *testing.T, api *httptest.Server) *harness {
	t.Helper()
	pollInterval = time.Millisecond
	t.Cleanup(func() { pollInterval = time.Second })
	h := newHarness(t, "")
	h.vars["SPROUT_API_URL"] = api.URL
	h.vars["SPROUT_TOKEN"] = goodToken
	return h
}

func site(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	for path, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func uploadedNames(t *testing.T, archive []byte) []string {
	t.Helper()
	var names []string
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
	}
}

func TestDeployCreatesTheAppAndPrintsItsAddress(t *testing.T) {
	fake, api := newFakeSprout(t)
	h := deployHarness(t, api)
	dir := site(t, "My Site", map[string]string{"index.html": "<h1>hi</h1>", ".env": "SECRET=1"})

	if code := h.run("deploy", dir); code != 0 {
		t.Fatalf("deploy exited %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	for _, want := range []string{
		"website, 1 files", "left out (never uploaded): .env", `Created the app "my-site" in ada-team`,
		"✓ source", "✓ health_check", "http://my-site.ada-team.localhost:8081",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(fake.uploads) != 1 || strings.Join(uploadedNames(t, fake.uploads[0]), ",") != "index.html" {
		t.Fatalf("uploaded %d archives; names %v", len(fake.uploads), uploadedNames(t, fake.uploads[0]))
	}
}

func TestDeployReusesAnExistingApp(t *testing.T) {
	fake, api := newFakeSprout(t)
	fake.apps = []map[string]any{{"id": "app-blog", "name": "Blog", "slug": "blog", "lifecycle": "active", "url": nil}}
	h := deployHarness(t, api)
	dir := site(t, "anything", map[string]string{"index.html": "x"})

	if code := h.run("deploy", "--app", "blog", dir); code != 0 {
		t.Fatalf("deploy exited %d: %s", code, h.stderr)
	}
	if strings.Contains(h.stdout.String(), "Created") || len(fake.apps) != 1 {
		t.Fatalf("an app was created although blog exists:\n%s", h.stdout)
	}
}

func TestDeployStopsBeforeUploadingWhatTheServerWouldRefuse(t *testing.T) {
	fake, api := newFakeSprout(t)
	h := deployHarness(t, api)
	dir := site(t, "shop", map[string]string{
		"package.json":    `{"name":"shop","private":true,"dependencies":{"express":"^5.0.0"}}`,
		"dist/index.html": "x",
	})

	if code := h.run("deploy", dir); code != 1 {
		t.Fatalf("deploy exited %d, want 1: %s", code, h.stderr)
	}
	if len(fake.uploads) != 0 {
		t.Fatal("an archive the server would refuse was uploaded")
	}
	for _, want := range []string{"not uploaded", "npm packages", "node_build_dependencies_unsupported", "sprout deploy " + filepath.Join(dir, "dist")} {
		if !strings.Contains(h.stderr.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, h.stderr)
		}
	}
}

func TestDeployExplainsAFailedBuild(t *testing.T) {
	fake, api := newFakeSprout(t)
	code := "health_check_failed"
	failed := stages("succeeded", "succeeded", "succeeded", "succeeded", "succeeded", "failed")
	failed[5]["failureCode"] = code
	fake.script = []map[string]any{
		{"id": "d1", "status": "queued", "failureCode": nil, "stages": stages()},
		{"id": "d1", "status": "failed", "failureCode": "release_failed", "stages": failed},
	}
	h := deployHarness(t, api)
	dir := site(t, "app", map[string]string{"package.json": `{"name":"app","version":"1.0.0","private":true,"engines":{"node":"24.x"},"scripts":{"build":"node build.js","start":"node server.js"}}`, "package-lock.json": `{"name":"app","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"app","version":"1.0.0"}}}`, "server.js": "x", "build.js": "x"})

	exit := h.run("deploy", dir)
	if exit != 1 {
		t.Fatalf("deploy exited %d, want 1: %s%s", exit, h.stdout, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "✗ health_check") || !strings.Contains(h.stderr.String(), "did not answer GET /health") || !strings.Contains(h.stderr.String(), "("+code+")") {
		t.Fatalf("stdout %q\nstderr %q", h.stdout, h.stderr)
	}
}

func TestDeployWhenTheServerDoesNotBuild(t *testing.T) {
	fake, api := newFakeSprout(t)
	fake.localBuilds = false
	h := deployHarness(t, api)
	dir := site(t, "s", map[string]string{"index.html": "x"})
	if code := h.run("deploy", dir); code != 1 || !strings.Contains(h.stderr.String(), "SPROUT_ENABLE_LOCAL_BUILDS=1") {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestInterruptStopsWatchingButNotTheBuild(t *testing.T) {
	fake, api := newFakeSprout(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake.script = fake.script[:2] // never finishes
	fake.onPoll = cancel          // Ctrl-C while watching
	h := deployHarness(t, api)
	dir := site(t, "s", map[string]string{"index.html": "x"})

	h.stdout.Reset()
	h.stderr.Reset()
	if code := run(ctx, []string{"deploy", dir}, h.env); code != 130 {
		t.Fatalf("exit %d, want 130: %s", code, h.stderr)
	}
	if !strings.Contains(h.stderr.String(), "carries on in Sprout") {
		t.Fatalf("stderr %q", h.stderr)
	}
}

func TestDeployArguments(t *testing.T) {
	_, api := newFakeSprout(t)
	h := deployHarness(t, api)
	for _, args := range [][]string{
		{"deploy", "--app", "Bad Slug"},
		{"deploy", "one", "two"},
		{"deploy", filepath.Join(t.TempDir(), "missing")},
	} {
		if code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, h.stderr)
		}
	}
}

func TestSlugify(t *testing.T) {
	for name, want := range map[string]string{
		"My Site!": "my-site", "portfolio-site": "portfolio-site", "__x__": "x", "Été 2026": "t-2026", "!!!": "",
		strings.Repeat("a", 70): strings.Repeat("a", 63),
	} {
		if got := slugify(name); got != want {
			t.Errorf("slugify(%q) = %q, want %q", name, got, want)
		}
	}
}
