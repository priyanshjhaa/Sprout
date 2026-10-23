# The `sprout` CLI

The dashboard deploys a folder you drop on a page. That works for a person at a browser, but not for the other people Sprout serves: a developer living in a terminal, a CI job, or a coding agent that has just written an app and can run commands but cannot click. This milestone gives them one command, `sprout deploy`, that does what the drop zone does.

Commits: `docs(openapi): describe real deployments with their own schema`, `feat: add the sprout CLI with login, logout and whoami`, `feat: pack a folder into a deployable archive in Go`, `fix: end a build's progress stream when it goes live`, `fix: take the build progress stream off the access-token allowlist`, `feat: add sprout deploy`, `fix: check a token's shape in the CLI and say what is wrong with it`, plus these docs.

## The mental model

The CLI is **just another client** of the same deploy API as the dashboard. The server learns nothing new: same routes, same permission checks, same build pipeline. The CLI does four things on your machine and then hands over:

```
your folder ─► pack (.tar) ─► check with the server's rules ─► upload ─► poll until live ─► print the URL
              └──────────── on your machine ────────────┘   └──────────── on the server ───────────┘
```

```sh
go install ./cmd/sprout          # from backend/; puts sprout in ~/go/bin
sprout login                     # paste a token from the dashboard's Access tokens page
sprout deploy ~/my-site          # or: cd ~/my-site && sprout deploy
sprout whoami
sprout logout
```

`SPROUT_TOKEN` overrides the stored login (for CI and agents), `SPROUT_API_URL` points at another API (default `http://127.0.0.1:8080`), and `--app` / `--workspace` choose where the app goes. Exit codes: 0 worked, 1 failed, 2 usage or login problem, 130 interrupted.

## Story 1: Who are you? (logging in)

**Problem.** A browser proves who you are with a Clerk session. A terminal has no browser, so it needs its own key: the personal access token from PR #12. The question is how to keep that key safe on a laptop.

**History.** Command-line tools learned this the hard way. Secrets typed as arguments end up in shell history. Key files that other users can read get copied, which is why `ssh` refuses to use one. And a key sent to the wrong server is a key given away, which is why `gh` and `aws` store a credential together with the host it belongs to.

**Decision in Sprout** (`cmd/sprout/credentials.go`, `auth.go`). `sprout login` reads the token from a hidden prompt, never from an argument. It checks the token with `GET /me` before saving it, so a typo is caught now, not at the next deploy. It saves the token in `~/.config/sprout/credentials.json` (mode 0600) **together with the API address it was checked against**, and never sends it anywhere else. The typed client (`internal/apiclient`) refuses plain HTTP except to this machine and never follows redirects, for the same reason.

During testing, a token pasted twice into the hidden prompt came back as "refused": the server answers every malformed token with the same 401 as a revoked one. The CLI now checks the token's shape itself (`sprout_pat_` plus 52 base32 characters) and says "pasted twice" or "incomplete" instead.

**Trade-off.** Switching servers means logging in again, and there is no browser login flow yet (it is on the "before anything leaves localhost" list).

## Story 2: Packing, and why the CLI cannot drift from the server

**Problem.** The deploy API accepts exactly one thing: a plain tar archive. Someone has to turn a folder into one, leave out what should never be published (`.env`, `.git`, keys, `node_modules`), and catch problems before a slow upload.

**History.** The browser packer (PR #13) had to *copy* the server's rules into TypeScript, and its own notes admit that the copy must be kept in step by hand. Copies of rules drift; that is how most "it passed locally but failed on the server" bugs are born.

**Decision in Sprout** (`internal/sourcepack`). `Pack` walks the folder through `os.Root`, never follows symlinks (a terminal sees them, a browser does not), skips the same names as the browser without even opening skipped folders, applies the limits and names the offending files, and writes a reproducible USTAR tar (fixed time, mode and owner). Then `Check` **runs the server's own source stage on your machine**: the real extractor, then the static-site or Node.js contract in the same order as `buildjob`. Because the CLI is written in Go, it calls the server's rules instead of copying them. If `Check` fails, `sprout deploy` stops before uploading and explains why in the dashboard's words; a project with a finished site in `dist/` gets pointed at it.

**Trade-off.** The CLI binary includes a few server packages (`source`, `staticsite`, `nodeapp`). They use only the standard library, so the cost is small; the database driver is deliberately not linked (a test keeps the token prefix in step instead of importing `apitoken`).

## Story 3: Polling instead of streaming

**Problem.** After the upload, the build runs for seconds to minutes. The CLI wants to show each stage as it finishes.

**History.** The web began with polling (asking again and again) because servers could not push. Push technologies (long polling, then Server-Sent Events and WebSockets) arrived for pages that many people watch for a long time. Polling is still the norm for one client watching one short task; most deploy CLIs mostly poll.

**Decision in Sprout** (`cmd/sprout/deploy.go`). `sprout deploy` asks `GET …/deployments/{id}` once a second and prints `✓ stage` as each one finishes. Five failed polls in a row (a network blip, an API restart) end the watch; a refusal from the server ends it at once. Ctrl-C stops *watching*, not the build, which carries on on the server. The dashboard keeps its SSE stream, because a dashboard tab is exactly the long-lived watcher push is for.

Choosing this exposed two server bugs. The stream ended on `succeeded`, `failed` and `cancelled` but not on `live`, so a real build's stream never sent `complete` (the dashboard happened to work around it). And the token allowlist allowed `/events`, but the stream handler only accepts browser sessions, so tokens got a 401. The first is fixed; the second is resolved by taking `/events` off the allowlist (now 11 operations).

**Trade-off.** Up to a second of delay and about one small request per second per watching terminal. Revisit when an agent tool or a status page needs instant updates for many watchers.

## Request trace: `sprout deploy ./site`

1. Pack and check `./site` locally. Nothing is sent if the check fails.
2. `GET /api/v1/capabilities` (does this API build?), then `GET /api/v1/me` (which workspace?).
3. `GET …/applications`, find the app by slug, or `POST …/applications` to create it.
4. `POST …/applications/{id}/deployments` with the tar (`application/x-tar`). Authentication, the token allowlist and the editor check all happen here, exactly as for the dashboard.
5. `GET …/deployments/{id}` every second until `live`, `succeeded`, `failed` or `cancelled`.
6. `GET …/applications/{id}` for the `url`, then print it.

## Verification

```sh
cd backend
go test -race ./cmd/sprout ./internal/apiclient ./internal/sourcepack ./internal/httpapi
go vet ./... && go build ./...
```

Unit tests drive the commands against a fake API: login stores a private file and refuses bad, doubled or argument tokens; a stored token is never sent to another address; deploy creates or reuses an app, prints the URL, never uploads `.env`, stops before uploading what the server would refuse, explains a failed health check, and exits 130 on Ctrl-C. Packer tests cover skipped names and symlinks, reproducible output, long USTAR paths, every limit, name rules and the Node.js rules, and fail if the server ever adds a sensitive name the packer would upload.

Then a full manual run against the local API with a real token: login and `whoami`; `portfolio-site`, `messy-site` (its `.env`, `server.pem`, `.git` and `node_modules` answer 404 once live), `node-app-no-packages` and `built-project/dist --app cli-test` all went live; `built-project`, `bad-names` and `no-index` stopped before upload with the right message and nothing reached the server; Ctrl-C mid-build left the build to finish live; two deploys at once gave the in-progress message; a malformed token, a different API address and logout behaved as described.

## Known gaps

No `sprout logs` or `sprout status`, no zip input, no release pipeline (`go install` is the install step), and no browser login flow. The CLI's failure explanations are copied from the dashboard by hand. `go install` does not put `~/go/bin` on your `PATH`; add it in your shell profile to type plain `sprout`.

## Explain-back checkpoint

1. Why does the CLI check a token's shape before sending it, when the server checks it anyway?
2. Why can the Go packer call the server's rules while the browser packer has to copy them?
3. What would go wrong if the stored token were sent to whatever `SPROUT_API_URL` says?
4. Why is polling right for the CLI but streaming right for the dashboard?
5. After Ctrl-C, why does the build keep going, and how would you find out how it ended?
