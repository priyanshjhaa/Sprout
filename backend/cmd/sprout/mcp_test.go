package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// agent is a test MCP client connected to `sprout mcp` for one project.
type agent struct {
	t        *testing.T
	session  *mcp.ClientSession
	mu       sync.Mutex
	progress []string
}

func connectAgent(t *testing.T, project string, h *harness) *agent {
	t.Helper()
	ctx := context.Background()
	serverSide, clientSide := mcp.NewInMemoryTransports()
	if _, err := newMCPServer(project, h.env).Connect(ctx, serverSide, nil); err != nil {
		t.Fatal(err)
	}
	a := &agent{t: t}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, request *mcp.ProgressNotificationClientRequest) {
			a.mu.Lock()
			a.progress = append(a.progress, request.Params.Message)
			a.mu.Unlock()
		},
	})
	session, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	a.session = session
	return a
}

// deploy calls the tool and returns its text and structured output.
func (a *agent) deploy(arguments map[string]any) (text string, output map[string]any, isError bool) {
	a.t.Helper()
	params := &mcp.CallToolParams{Name: "deploy", Arguments: arguments}
	params.SetProgressToken("deploy-1")
	result, err := a.session.CallTool(context.Background(), params)
	if err != nil {
		a.t.Fatalf("protocol error: %v", err)
	}
	for _, content := range result.Content {
		if part, ok := content.(*mcp.TextContent); ok {
			text += part.Text
		}
	}
	if result.StructuredContent != nil {
		raw, _ := json.Marshal(result.StructuredContent)
		_ = json.Unmarshal(raw, &output)
	}
	return text, output, result.IsError
}

// heard waits briefly for a progress message, which arrives asynchronously.
func (a *agent) heard(message string) bool {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		a.mu.Lock()
		found := slices.Contains(a.progress, message)
		a.mu.Unlock()
		if found {
			return true
		}
	}
	return false
}

// project makes a project folder holding the given files.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	return site(t, "project", files)
}

func TestMCPListsTheDeployTool(t *testing.T) {
	_, api := newFakeSprout(t)
	a := connectAgent(t, project(t, map[string]string{"site/index.html": "x"}), deployHarness(t, api))
	tools, err := a.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "deploy" {
		t.Fatalf("tools %+v", tools.Tools)
	}
	schema, _ := json.Marshal(tools.Tools[0].InputSchema)
	for _, field := range []string{`"folder"`, `"repository"`, `"commit"`, `"path"`} {
		if !strings.Contains(string(schema), field) {
			t.Fatalf("no %s input: %s", field, schema)
		}
	}
	if strings.Contains(string(schema), `"required"`) {
		t.Fatalf("folder or repository, so neither is required on its own: %s", schema)
	}
	if !strings.Contains(a.session.InitializeResult().Instructions, "public") {
		t.Fatal("the instructions must tell the agent that apps are public")
	}
}

func TestMCPDeploysAFolderInTheProject(t *testing.T) {
	fake, api := newFakeSprout(t)
	h := deployHarness(t, api)
	a := connectAgent(t, project(t, map[string]string{"site/index.html": "<h1>hi</h1>", "site/.env": "SECRET=1"}), h)

	text, output, isError := a.deploy(map[string]any{"folder": "site"})
	if isError {
		t.Fatalf("deploy failed: %s", text)
	}
	if text != "Live at http://site.ada-team.localhost:8081" || output["url"] != "http://site.ada-team.localhost:8081" ||
		output["status"] != "live" || output["app"] != "site" || output["deploymentId"] != "d1" {
		t.Fatalf("text %q, output %v", text, output)
	}
	if leftOut, _ := output["leftOut"].([]any); len(leftOut) != 1 || leftOut[0] != ".env" {
		t.Fatalf("leftOut %v", output["leftOut"])
	}
	if names := uploadedNames(t, fake.uploads[0]); strings.Join(names, ",") != "index.html" {
		t.Fatalf("uploaded %v", names)
	}
	for _, message := range []string{"website, 1 files, 1 KiB; left out (never uploaded): .env", `Created the app "site" in ada-team`, "health_check done"} {
		if !a.heard(message) {
			t.Errorf("no progress message %q (heard %v)", message, a.progress)
		}
	}
	if h.stdout.Len() != 0 {
		t.Fatalf("sprout mcp wrote to stdout, which belongs to the protocol: %q", h.stdout)
	}
}

func TestMCPExplainsARefusalSoTheAgentCanFixIt(t *testing.T) {
	fake, api := newFakeSprout(t)
	a := connectAgent(t, project(t, map[string]string{
		"shop/package.json":    `{"name":"shop","private":true,"dependencies":{"express":"^5.0.0"}}`,
		"shop/dist/index.html": "x",
	}), deployHarness(t, api))

	text, _, isError := a.deploy(map[string]any{"folder": "shop"})
	if !isError || !strings.Contains(text, "npm packages") || !strings.Contains(text, "deploy shop/dist instead") {
		t.Fatalf("isError %v, text %q", isError, text)
	}
	if len(fake.uploads) != 0 {
		t.Fatal("a refused folder was uploaded")
	}
}

func TestMCPOnlyDeploysInsideTheProject(t *testing.T) {
	fake, api := newFakeSprout(t)
	dir := project(t, map[string]string{"site/index.html": "x"})
	// A real folder right next to the project.
	outside := filepath.Join(filepath.Dir(dir), "private")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "index.html"), []byte("diary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sneaky")); err != nil {
		t.Fatal(err)
	}
	a := connectAgent(t, dir, deployHarness(t, api))

	for _, folder := range []string{outside, "../private", "sneaky", "site/../.."} {
		text, _, isError := a.deploy(map[string]any{"folder": folder})
		if !isError || !strings.Contains(text, "outside this project") {
			t.Errorf("%s: isError %v, text %q", folder, isError, text)
		}
	}
	if text, _, isError := a.deploy(map[string]any{"folder": "missing"}); !isError || !strings.Contains(text, "not a folder in this project") {
		t.Errorf("missing: isError %v, text %q", isError, text)
	}
	if len(fake.uploads) != 0 {
		t.Fatal("something outside the project was uploaded")
	}
}

func TestMCPRefusesToTreatHomeAsAProject(t *testing.T) {
	_, api := newFakeSprout(t)
	home := project(t, map[string]string{"site/index.html": "x"})
	t.Setenv("HOME", home)
	a := connectAgent(t, home, deployHarness(t, api))
	if text, _, isError := a.deploy(map[string]any{"folder": "site"}); !isError || !strings.Contains(text, "not a project folder") {
		t.Fatalf("isError %v, text %q", isError, text)
	}
}

func TestMCPAsksTheUserToLogIn(t *testing.T) {
	_, api := newFakeSprout(t)
	h := deployHarness(t, api)
	delete(h.vars, "SPROUT_TOKEN")
	a := connectAgent(t, project(t, map[string]string{"site/index.html": "x"}), h)
	if text, _, isError := a.deploy(map[string]any{"folder": "site"}); !isError || !strings.Contains(text, "run `sprout login` in a terminal") {
		t.Fatalf("isError %v, text %q", isError, text)
	}
}

const testCommit = "0123456789abcdef0123456789abcdef01234567"

// fakeGitHub serves octocat/hello at testCommit as a tarball shaped like
// GitHub's, and 404s for anything else.
func fakeGitHub(t *testing.T, files map[string]string) {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(zipped)
	for name, body := range files {
		if err := writer.WriteHeader(&tar.Header{Name: "hello-0123456/" + name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		writer.Write([]byte(body))
	}
	writer.Close()
	zipped.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/octocat/hello/tar.gz/"+testCommit {
			http.NotFound(w, r)
			return
		}
		w.Write(buffer.Bytes())
	}))
	t.Cleanup(server.Close)
	githubBaseURL = server.URL
	t.Cleanup(func() { githubBaseURL = "https://codeload.github.com" })
}

func TestMCPDeploysAGitHubRepositoryAtACommit(t *testing.T) {
	fake, api := newFakeSprout(t)
	fakeGitHub(t, map[string]string{"README.md": "x", "site/index.html": "<h1>hi</h1>", "site/.env": "SECRET=1"})
	a := connectAgent(t, project(t, map[string]string{"notes.txt": "x"}), deployHarness(t, api))

	text, output, isError := a.deploy(map[string]any{"repository": "https://github.com/octocat/hello", "commit": testCommit, "path": "site"})
	if isError {
		t.Fatalf("deploy failed: %s", text)
	}
	if output["app"] != "hello" || output["status"] != "live" {
		t.Fatalf("text %q, output %v", text, output)
	}
	if leftOut, _ := output["leftOut"].([]any); len(leftOut) != 1 || leftOut[0] != ".env" {
		t.Fatalf("leftOut %v", output["leftOut"])
	}
	if names := uploadedNames(t, fake.uploads[0]); strings.Join(names, ",") != "index.html" {
		t.Fatalf("uploaded %v", names)
	}
	if !a.heard("Downloading octocat/hello/site at 0123456 from GitHub") {
		t.Errorf("no download progress (heard %v)", a.progress)
	}
}

func TestMCPExplainsAMissingRepository(t *testing.T) {
	fake, api := newFakeSprout(t)
	fakeGitHub(t, map[string]string{"index.html": "x"})
	a := connectAgent(t, project(t, map[string]string{"notes.txt": "x"}), deployHarness(t, api))
	text, _, isError := a.deploy(map[string]any{"repository": "octocat/private", "commit": testCommit})
	if !isError || !strings.Contains(text, "only deploy public repositories") {
		t.Fatalf("isError %v, text %q", isError, text)
	}
	if len(fake.uploads) != 0 {
		t.Fatal("something was uploaded")
	}
}

func TestMCPHintsAtTheBuiltFolderInARepository(t *testing.T) {
	_, api := newFakeSprout(t)
	fakeGitHub(t, map[string]string{
		"web/package.json":    `{"name":"web","private":true,"dependencies":{"express":"^5.0.0"}}`,
		"web/dist/index.html": "x",
	})
	a := connectAgent(t, project(t, map[string]string{"notes.txt": "x"}), deployHarness(t, api))
	text, _, isError := a.deploy(map[string]any{"repository": "octocat/hello", "commit": testCommit, "path": "web"})
	if !isError || !strings.Contains(text, `with path "web/dist" instead`) {
		t.Fatalf("isError %v, text %q", isError, text)
	}
}

func TestMCPChecksWhatToDeploy(t *testing.T) {
	_, api := newFakeSprout(t)
	a := connectAgent(t, project(t, map[string]string{"site/index.html": "x"}), deployHarness(t, api))
	for _, c := range []struct {
		arguments map[string]any
		want      string
	}{
		{map[string]any{}, "say what to deploy"},
		{map[string]any{"folder": "site", "repository": "octocat/hello"}, "not both"},
		{map[string]any{"folder": "site", "commit": testCommit}, "only go with repository"},
		{map[string]any{"repository": "octocat/hello"}, "40-character SHA"},
		{map[string]any{"repository": "octocat/hello", "commit": "main"}, "not a full commit SHA"},
		{map[string]any{"repository": "octocat", "commit": testCommit}, "owner/name"},
		{map[string]any{"repository": "octocat/hello", "commit": testCommit, "path": "../x"}, "not a folder path"},
	} {
		if text, _, isError := a.deploy(c.arguments); !isError || !strings.Contains(text, c.want) {
			t.Errorf("%v: isError %v, text %q", c.arguments, isError, text)
		}
	}
}
