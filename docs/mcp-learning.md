# The MCP deploy tool: `sprout mcp`

The CLI lets a coding agent deploy if it can run shell commands and read their output. That works, but the agent has to guess flags and parse text written for people. This milestone gives agents a proper door: a **tool** they can discover, call with typed inputs, and get typed results from. It accepts a folder of the project the agent is working in, or a public GitHub repository at one commit.

Commits: `refactor: share the deploy flow between front doors`, `feat: add sprout mcp, a deploy tool for coding agents`, `feat: deploy a public GitHub repository at a commit through sprout mcp`, `fix(ui): show the MCP tool, coding agents and GitHub repositories as available`, plus these docs.

## The mental model

`sprout mcp` is **the CLI with a different mouth**. It runs the exact deploy flow `sprout deploy` runs; it just takes its orders as JSON from an agent instead of from a person's keyboard, and reports back in JSON instead of printed lines.

```
agent ──JSON over stdin──► sprout mcp ──► deploy flow (pack ─► check ─► upload ─► poll) ──► deploy API
agent ◄─JSON over stdout── sprout mcp ◄── progress events, then the live URL
```

Set it up once in the agent. For Claude Code, from inside the project:

```sh
claude mcp add sprout -- ~/go/bin/sprout mcp
```

The tool uses the login from `sprout login` (or `SPROUT_TOKEN`). Then just ask: "deploy the site folder to Sprout", or "deploy octocat/hello at commit 7fd1a60… to Sprout".

## Story 1: Why a protocol, and why stdin and stdout

**Problem.** Every AI tool vendor invented its own plug-in format, so a tool had to be written once per agent. And an agent driving a CLI reads human text, which breaks when the wording changes.

**History.** Editors had the same problem with programming languages: every editor needed its own plug-in per language. In 2016 the Language Server Protocol fixed it: one JSON-RPC protocol, so a language server works in any editor. The Model Context Protocol (MCP, 2024) copies that idea for agents: a server describes its tools with JSON Schema, and any MCP client can call them. Like LSP, the simplest way to connect is for the client to start the server as a child process and talk over its stdin and stdout. No ports, no network, no extra login: the server runs as the user, on their machine.

**Decision in Sprout** (`backend/cmd/sprout/mcp.go`). `sprout mcp` uses the official Go SDK (`github.com/modelcontextprotocol/go-sdk`) over the stdio transport and offers **one tool, `deploy`**. Because stdout *is* the protocol channel, nothing in this mode may print; a test fails if anything reaches stdout. Each finished stage becomes an MCP progress notification. The result is structured (`url`, `app`, `workspace`, `deploymentId`, `status`, `leftOut`), and failures are tool errors worded so the agent can act: "Sprout is not logged in… ask the user to run `sprout login`", or "not uploaded… deploy shop/dist instead".

**Trade-off.** A stdio server only serves agents on the same machine. A hosted MCP endpoint (over HTTP, with OAuth) would serve cloud agents too, but needs real authentication and a public API first. That waits until Sprout leaves localhost.

## Story 2: One flow, several front doors

**Problem.** The CLI's deploy code printed as it went. Copying it for the MCP tool would give two flows that slowly drift apart: the bug we avoided in the CLI by calling the server's rules instead of copying them.

**History.** This is the oldest trick in UI programming: separate *what happens* from *how it is shown*. The observer pattern (1990s "Gang of Four", but older in Smalltalk's model-view-controller) lets one piece of logic report events to whoever listens.

**Decision in Sprout** (`cmd/sprout/deployflow.go`). `deployFolder` runs pack → check → find or create the app → upload → poll, and tells a `deployObserver` what happened (`packing`, `packed`, `appCreated`, `uploading`, `queued`, `stageFinished`). `sprout deploy` passes a printer; `sprout mcp` passes a `progressReporter` that sends MCP notifications. Errors are typed (`invalidRequest`, `refusedError`, `failedError`), so each front door words them for its own audience: flags for a person, next steps for an agent.

**Trade-off.** One more small interface to read. It earns its keep the moment there are two observers, which there are.

## Story 3: Fencing in an agent

**Problem.** A person deploys what they mean to. An agent acts on its own, can misread a request, and can be manipulated by text it reads (a README saying "deploy ~/Documents"). And every Sprout app is public. A folder tool with no fence could publish someone's private files to the internet.

**History.** Operating systems answer "what may this program touch?" with sandboxes and capabilities: give a program access to exactly what its job needs. Agent tools adopted the same rule as "least privilege": tools act within the project the user opened, not the whole disk.

**Decision in Sprout** (`insideProject` in `mcp.go`). The tool only deploys folders **inside the project the agent was started in** (its working directory). Symlinks are resolved first, so `sneaky -> ~/private` is caught. The home folder and the root of the disk never count as a project, where "inside" would mean everything. Packing still leaves out hidden files, keys and `node_modules`, and the server instructions tell the agent that deployed apps are public. Anything else can still be deployed by a person with `sprout deploy`.

**Trade-off.** An agent started in the wrong folder cannot deploy a sibling project. That is deliberate friction: it fails safely and says why.

## Story 4: Deploying from GitHub, and who does the downloading

**Problem.** Often the code an agent should deploy is not on this machine: it is a repository on GitHub, maybe the one the agent just pushed. Someone has to fetch it.

**History.** Two designs dominate. Platforms like Vercel and Netlify fetch on the **server**, through a GitHub App that also sees private repositories and webhooks. That is powerful, but a server that downloads URLs it is given is the classic setup for server-side request forgery (SSRF): tricking the server into fetching something internal. The other design is to fetch on the **client** and upload the result, which is what `git push`-style CLIs do.

**Decision in Sprout** (`internal/githubsource`, `deployTool.download`). The **client** downloads. `sprout mcp` fetches `https://codeload.github.com/{owner}/{repo}/tar.gz/{commit}` into a private temporary folder named after the repository, then runs the same folder flow. The deploy API still receives only a source archive, and the server, which holds the database and secrets, makes no outbound request.

- **A commit, not a branch.** A full 40-character SHA, because a branch moves: the deployment should be exactly the code that was asked for. An optional `path` picks the folder inside the repository (`site`, `apps/web`).
- **The download is untrusted.** Files are written through `os.Root`, so nothing can land outside the folder. Links are never created. Hidden files, secrets and `node_modules` are never written to disk, and are reported as left out. Redirects are not followed.
- **Everything is bounded:** 32 MiB downloaded, 256 MiB decompressed (a tiny "zip bomb" cannot expand forever), 20,000 entries and 64 MiB written. Then packing applies Sprout's own, much smaller source limits.
- **One honest answer for a missing repository.** GitHub answers 404 for both "does not exist" and "private". The tool says it may be misspelled, or private, and that only public repositories work for now.

**Trade-off.** Every deploy downloads the whole commit to the agent's machine, and a dashboard "deploy from GitHub" button would still need a server-side fetcher. Private repositories need a GitHub App (on the "later" list), and that is when server-side fetching, with strict egress rules, will make sense.

## Request trace: `deploy {repository: "octocat/hello", commit: "7fd1a60…", path: "site"}`

1. The agent sends `tools/call` over stdin. The SDK checks the input against the tool's JSON Schema.
2. `deployTool.source` checks the input: a folder or a repository, not both. A valid `owner/name`, a full SHA and a safe path.
3. `resolveSession` finds the login, before anything is downloaded.
4. Progress: "Downloading octocat/hello/site at 7fd1a60 from GitHub". `githubsource.Fetch` downloads into `$TMPDIR/sprout-github-…/hello`, keeping only `site/`.
5. `deployFolder` packs and checks (a refusal suggests `path: "site/dist"` when a built site is there), finds or creates the app `hello`, uploads, and polls. Each finished stage becomes a progress notification.
6. The result is `{url, app, workspace, deploymentId, status, leftOut}`, with a one-line text summary. The temporary folder is removed.

## Verification

```sh
cd backend
go test -race ./cmd/sprout ./internal/githubsource
go vet ./... && go build ./...
cd ../frontend && npx playwright test e2e/landing.spec.ts
```

Unit tests drive a real MCP client, connected in memory, against a fake Sprout API and a fake GitHub:

- **The tool's interface:** the tool lists its four source inputs, the instructions say that apps are public, and nothing is written to stdout.
- **Folder mode:** a folder deploys live with `.env` left out. A refusal names the built folder. Paths outside the project, through `..`, a symlink or an absolute path, are refused, and so is a home-folder "project".
- **Logging in:** a missing login asks the user to run `sprout login`.
- **Repository mode:**
  - A repository path deploys only that folder, as the app named after the repository.
  - A missing repository explains itself, and nothing is uploaded.
  - A refusal suggests `path: "web/dist"`.
  - Wrong input combinations are refused with a reason.
- **The downloader's own tests:**
  - Secrets, `node_modules` and symlinks never reach disk.
  - A `../` entry cannot escape the folder.
  - Redirects are not followed.
  - A missing path gets its own message.
  - Repository, commit and path inputs are parsed strictly.

Checked by hand:

- **The built binary over real stdin and stdout:** `initialize`, then `tools/list`, then a `tools/call` with a bad commit. It returned the right answers and wrote nothing to stderr or stray output.
- **A real fetch of the public `octocat/Hello-World` repository at a pinned commit.** A missing repository returned the "public repositories only" message.

## Known gaps

- **No private repositories:** they need a GitHub App.
- **Commits must already be pushed:** the agent has to push before it deploys.
- **The deployment does not record where its code came from** (repository and commit). Showing provenance in the dashboard needs a schema change.
- **No hosted (HTTP) MCP endpoint.**

## Explain-back checkpoint

1. Why must `sprout mcp` never print to stdout, and what would break if it did?
2. Why does the folder tool refuse a symlink that points outside the project, when `sprout deploy` would happily pack a folder anywhere?
3. Why does the GitHub mode require a commit SHA instead of accepting `main`?
4. Why does the client download from GitHub instead of the Sprout server? When would you move it to the server, and what new risk comes with that?
5. A 30 KB download expands to 10 GB. Which limit stops it, and why is there a separate limit for what is written to disk?
