package main

import (
	"context"
	"encoding/json"
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
	if !strings.Contains(string(schema), `"required":["folder"]`) {
		t.Fatalf("folder should be the one required input: %s", schema)
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
