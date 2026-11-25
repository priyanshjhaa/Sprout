package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priyanshjhaa/Sprout/backend/internal/apiclient"
	"github.com/priyanshjhaa/Sprout/backend/internal/githubsource"
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
Use the deploy tool when the user asks to deploy, publish or ship a folder of this project, or a public GitHub repository at a commit. Every deployed app is public: anyone with its URL can open it, so deploy only what the user asked to publish.`

// githubBaseURL is where tarballs are downloaded from; tests point it at a fake.
var githubBaseURL = githubsource.DefaultBaseURL

type deployInput struct {
	Folder     string `json:"folder,omitempty" jsonschema:"a folder of this project to deploy; a relative path starts at the project folder. Give this or repository, not both"`
	Repository string `json:"repository,omitempty" jsonschema:"a public GitHub repository to deploy, as owner/name. Give this or folder, not both"`
	Commit     string `json:"commit,omitempty" jsonschema:"with repository: the full 40-character SHA of the commit to deploy; it must already be pushed to GitHub"`
	Path       string `json:"path,omitempty" jsonschema:"with repository: the folder inside the repository that holds the app, like site or apps/web; defaults to the top of the repository"`
	App        string `json:"app,omitempty" jsonschema:"the app's name: lowercase letters, numbers and single dashes, like my-site. Defaults to the folder's or repository's name. Deploying to an existing app replaces its live version"`
	Workspace  string `json:"workspace,omitempty" jsonschema:"the workspace slug; defaults to the user's home workspace"`
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
	tool := deployTool{project: project, env: env, github: githubsource.Fetcher{BaseURL: githubBaseURL, UserAgent: userAgent}}
	mcp.AddTool(server, &mcp.Tool{
		Name:  "deploy",
		Title: "Deploy to Sprout",
		Description: "Deploy a folder in this project, or a public GitHub repository at a commit, to Sprout and return its public URL. " +
			"The folder (or the repository's path) must hold an index.html (a website) or a package.json (a Node.js app that uses no npm packages). " +
			"Hidden files, secrets and node_modules are never uploaded. If Sprout would refuse the folder, nothing is uploaded and the reason is returned so you can fix it.",
	}, tool.deploy)
	return server
}

type deployTool struct {
	project string
	env     environment
	github  githubsource.Fetcher
}

func (t deployTool) deploy(ctx context.Context, request *mcp.CallToolRequest, input deployInput) (*mcp.CallToolResult, deployOutput, error) {
	source, err := t.source(input)
	if err != nil {
		return nil, deployOutput{}, err
	}
	// Log-in problems come first, before anything is downloaded.
	current, err := resolveSession(t.env.getenv, t.env.configDir)
	if err != nil {
		return nil, deployOutput{}, forTheUser(err)
	}
	client, err := apiclient.New(current.apiURL, current.token, userAgent)
	if err != nil {
		return nil, deployOutput{}, forTheUser(err)
	}
	reporter := &progressReporter{ctx: ctx, session: request.Session, token: request.Params.GetProgressToken()}
	folder := source.folder
	if source.repository != nil {
		downloaded, cleanup, err := t.download(ctx, source, reporter)
		if err != nil {
			return nil, deployOutput{}, err
		}
		defer cleanup()
		folder = downloaded
	}
	checked, err := newDeployRequest(folder, input.App, input.Workspace)
	if err != nil {
		return nil, deployOutput{}, err
	}
	result, err := deployFolder(ctx, client, checked, reporter)
	if err != nil {
		return nil, deployOutput{}, forTheAgent(err, source.retry)
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

// deploySource is a checked choice of what to deploy: a folder of the project,
// or a repository at a commit.
type deploySource struct {
	folder     string
	repository *githubsource.Repository
	commit     string
	path       string
	// retry words "deploy this built folder instead" for this kind of source.
	retry func(built string) string
}

func (t deployTool) source(input deployInput) (deploySource, error) {
	switch {
	case input.Folder != "" && input.Repository != "":
		return deploySource{}, invalidRequest{"give a folder or a repository to deploy, not both"}
	case input.Repository != "":
		repository, err := githubsource.ParseRepository(input.Repository)
		if err != nil {
			return deploySource{}, invalidRequest{err.Error()}
		}
		if input.Commit == "" {
			return deploySource{}, invalidRequest{"give the full 40-character SHA of the commit to deploy; it must already be pushed to GitHub"}
		}
		commit, err := githubsource.ParseCommit(input.Commit)
		if err != nil {
			return deploySource{}, invalidRequest{err.Error()}
		}
		inside, err := githubsource.ParsePath(input.Path)
		if err != nil {
			return deploySource{}, invalidRequest{err.Error()}
		}
		retry := func(built string) string {
			return fmt.Sprintf("If the finished website is in %q, deploy the same commit with path %q instead.", built, path.Join(inside, built))
		}
		return deploySource{repository: &repository, commit: commit, path: inside, retry: retry}, nil
	case input.Commit != "" || input.Path != "":
		return deploySource{}, invalidRequest{"commit and path only go with repository; to deploy a folder of this project, give just the folder"}
	case strings.TrimSpace(input.Folder) == "":
		return deploySource{}, invalidRequest{"say what to deploy: a folder of this project, or a public GitHub repository and commit"}
	}
	folder, err := insideProject(t.project, input.Folder)
	if err != nil {
		return deploySource{}, err
	}
	retry := func(built string) string {
		return fmt.Sprintf("If the finished website is in %q, deploy %s instead.", built, filepath.Join(input.Folder, built))
	}
	return deploySource{folder: folder, retry: retry}, nil
}

// download writes the commit into a private temporary folder named after the
// repository (so the app is named after it too) and returns a cleanup.
func (t deployTool) download(ctx context.Context, source deploySource, reporter *progressReporter) (string, func(), error) {
	where := source.repository.String()
	if source.path != "" {
		where += "/" + source.path
	}
	reporter.send(fmt.Sprintf("Downloading %s at %s from GitHub", where, source.commit[:7]))
	temporary, err := os.MkdirTemp("", "sprout-github-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(temporary) }
	folder := filepath.Join(temporary, source.repository.Name)
	if err := os.Mkdir(folder, 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	fetched, err := t.github.Fetch(ctx, *source.repository, source.commit, source.path, folder)
	if err != nil {
		cleanup()
		if ctx.Err() != nil {
			return "", nil, interrupted{"stopped while downloading from GitHub; nothing was deployed"}
		}
		return "", nil, err
	}
	reporter.leftOut = append(reporter.leftOut, fetched.Skipped...)
	return folder, cleanup, nil
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
func forTheAgent(err error, retry func(built string) string) error {
	var refused refusedError
	if errors.As(err, &refused) && refused.builtFolder != "" {
		return fmt.Errorf("%s %s", refused.Error(), retry(refused.builtFolder))
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
