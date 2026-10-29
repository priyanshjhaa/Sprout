package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/sourcepack"
)

// `sprout mcp` lets a coding agent deploy through Sprout. The agent starts it
// as a background process and speaks the Model Context Protocol over stdin and
// stdout, so nothing in this mode may print to stdout: that channel belongs to
// the protocol. It offers one tool, deploy, which runs the same flow as
// `sprout deploy` (deployflow.go).
//
// An agent acts on its own and Sprout apps are public, so the tool only
// deploys folders inside the project the agent was started in: the working
// directory of this process.

const mcpInstructions = `Sprout deploys small apps (a static website, or a Node.js app with no npm packages) to a public URL.
Use the deploy tool when the user asks to deploy, publish or ship a folder. Every deployed app is public: anyone with its URL can open it, so deploy only what the user asked to publish.`

type deployInput struct {
	Folder    string `json:"folder" jsonschema:"the folder to deploy; it must be inside the current project, and a relative path starts at the project folder"`
	App       string `json:"app,omitempty" jsonschema:"the app's name: lowercase letters, numbers and single dashes, like my-site. Defaults to the folder's name. Deploying to an existing app replaces its live version"`
	Workspace string `json:"workspace,omitempty" jsonschema:"the workspace slug; defaults to the user's home workspace"`
}

type deployOutput struct {
	URL          string   `json:"url,omitempty" jsonschema:"the public address of the live app"`
	App          string   `json:"app" jsonschema:"the app's name"`
	Workspace    string   `json:"workspace" jsonschema:"the workspace the app is in"`
	DeploymentID string   `json:"deploymentId" jsonschema:"the deployment's id"`
	Status       string   `json:"status" jsonschema:"live, or succeeded when the server stores builds without running them"`
	LeftOut      []string `json:"leftOut,omitempty" jsonschema:"files and folders that were never uploaded (hidden files, secrets, dependencies, links)"`
}

// mcpServe runs the MCP server on stdin and stdout until the agent hangs up.
func mcpServe(ctx context.Context, args []string, env environment) error {
	if len(args) != 0 {
		return usageError{"usage: sprout mcp (started by a coding agent, not by hand)"}
	}
	project, err := os.Getwd()
	if err != nil {
		return err
	}
	return newMCPServer(project, env).Run(ctx, &mcp.StdioTransport{})
}

// newMCPServer builds the server for a project folder.
func newMCPServer(project string, env environment) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "sprout", Version: "dev"}, &mcp.ServerOptions{Instructions: mcpInstructions})
	tool := deployTool{project: project, env: env}
	mcp.AddTool(server, &mcp.Tool{
		Name:  "deploy",
		Title: "Deploy to Sprout",
		Description: "Deploy a folder in this project to Sprout and return its public URL. " +
			"The folder must hold an index.html (a website) or a package.json (a Node.js app that uses no npm packages). " +
			"Hidden files, secrets and node_modules are never uploaded. If Sprout would refuse the folder, nothing is uploaded and the reason is returned so you can fix it.",
	}, tool.deploy)
	return server
}

type deployTool struct {
	project string
	env     environment
}

func (t deployTool) deploy(ctx context.Context, request *mcp.CallToolRequest, input deployInput) (*mcp.CallToolResult, deployOutput, error) {
	folder, err := insideProject(t.project, input.Folder)
	if err != nil {
		return nil, deployOutput{}, err
	}
	checked, err := newDeployRequest(folder, input.App, input.Workspace)
	if err != nil {
		return nil, deployOutput{}, err
	}
	current, err := resolveSession(t.env.getenv, t.env.configDir)
	if err != nil {
		return nil, deployOutput{}, forTheUser(err)
	}
	client, err := apiclient.New(current.apiURL, current.token, userAgent)
	if err != nil {
		return nil, deployOutput{}, forTheUser(err)
	}
	reporter := &progressReporter{ctx: ctx, session: request.Session, token: request.Params.GetProgressToken()}
	result, err := deployFolder(ctx, client, checked, reporter)
	if err != nil {
		return nil, deployOutput{}, forTheAgent(err, input.Folder)
	}
	output := deployOutput{
		URL: result.url, App: result.app.Slug, Workspace: result.workspace,
		DeploymentID: result.deployment.ID, Status: result.deployment.Status, LeftOut: reporter.leftOut,
	}
	summary := fmt.Sprintf("Live at %s", result.url)
	switch {
	case result.deployment.Status == "succeeded":
		summary = "Built and stored, but this Sprout server does not run apps, so there is no address."
	case result.url == "":
		summary = "Live. Open the app in the Sprout dashboard for its address."
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}, output, nil
}

// insideProject resolves the folder an agent asked for and refuses anything
// outside the project, including through a symlink. The project itself must be
// a real project, not the home folder or the root of the disk, where "inside"
// would mean almost everything.
func insideProject(project, folder string) (string, error) {
	if strings.TrimSpace(folder) == "" {
		return "", invalidRequest{"name the folder to deploy, relative to the project folder"}
	}
	root, err := filepath.EvalSymlinks(project)
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		if resolved, err := filepath.EvalSymlinks(home); err == nil {
			home = resolved
		}
	}
	if root == filepath.Dir(root) || root == home {
		return "", invalidRequest{fmt.Sprintf("Sprout's tool was started in %s, which is not a project folder, so it deploys nothing. Ask the user to start the agent inside the project they want to deploy.", root)}
	}
	path := folder
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", invalidRequest{fmt.Sprintf("%s is not a folder in this project", folder)}
	}
	if relative, err := filepath.Rel(root, resolved); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", invalidRequest{fmt.Sprintf("%s is outside this project (%s). The deploy tool only deploys folders inside the project; the user can deploy anything else with `sprout deploy` in a terminal.", folder, root)}
	}
	return resolved, nil
}

// forTheUser words a problem only the person can fix, such as logging in.
func forTheUser(err error) error {
	if errors.Is(err, errNotLoggedIn) || apiclient.HasCode(err, "authentication_required") {
		return errors.New("Sprout is not logged in on this computer (or the token has expired). Ask the user to run `sprout login` in a terminal, then try again.")
	}
	return errors.New(describe(err))
}

// forTheAgent words a failed deploy so the agent can act on it.
func forTheAgent(err error, folder string) error {
	var refused refusedError
	if errors.As(err, &refused) && refused.builtFolder != "" {
		return fmt.Errorf("%s If the finished website is in %q, deploy %s instead.", refused.Error(), refused.builtFolder, filepath.Join(folder, refused.builtFolder))
	}
	var stopped interrupted
	if errors.As(err, &stopped) {
		return errors.New(stopped.message)
	}
	if errors.Is(err, errNotLoggedIn) || apiclient.HasCode(err, "authentication_required") {
		return forTheUser(err)
	}
	return errors.New(describe(err))
}

// progressReporter is the agent's deployObserver: each event becomes an MCP
// progress notification, when the agent asked for progress.
type progressReporter struct {
	ctx     context.Context
	session *mcp.ServerSession
	token   any
	step    float64
	leftOut []string
}

func (p *progressReporter) send(message string) {
	if p.token == nil {
		return
	}
	p.step++
	// Progress is best effort: a lost notification must not fail the deploy.
	_ = p.session.NotifyProgress(p.ctx, &mcp.ProgressNotificationParams{ProgressToken: p.token, Progress: p.step, Message: message})
}

func (p *progressReporter) packing(folder string) { p.send("Packing " + folder) }

func (p *progressReporter) packed(archive sourcepack.Archive) {
	for _, skip := range archive.Skipped {
		p.leftOut = append(p.leftOut, skip.Path)
	}
	message := fmt.Sprintf("%s, %d files, %s", describeKind(archive.Kind), archive.Files, formatSize(archive.Bytes))
	if len(archive.Skipped) > 0 {
		message += "; left out (never uploaded): " + skippedList(archive.Skipped)
	}
	p.send(message)
}

func (p *progressReporter) appCreated(workspace, slug string) {
	p.send(fmt.Sprintf("Created the app %q in %s", slug, workspace))
}

func (p *progressReporter) uploading(workspace, slug string) {
	p.send(fmt.Sprintf("Uploading to %s/%s", workspace, slug))
}

func (p *progressReporter) queued() { p.send("Waiting for a build slot") }

func (p *progressReporter) stageFinished(stage string, succeeded bool) {
	if succeeded {
		p.send(stage + " done")
	} else {
		p.send(stage + " failed")
	}
}
