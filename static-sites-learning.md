# Static sites

This continues backend milestone 8, after the local runtime. A static site is the simplest thing Sprout can host: files and a rule for serving them. It has no process, so it has nothing to start, health-check, sleep or wake. The work is therefore a strict contract and a careful serving path, not a runtime.

Commits: `feat: record how each release is served`, `feat: accept static sites as a source type`, `feat: serve static sites from the artifact`, plus dashboard copy and these docs.

## The mental model

A release is either **run** (a Node app in a container) or **read** (files returned from the stored artifact). Both are immutable and both are reached through the same public proxy, which already knows how to check an application's lifecycle. A static release just takes a shorter road after the lifecycle check.

## Slice 1: say how a release is served

**Problem.** The proxy must know, per live release, whether to start a container or read files. Guessing from the artifact's contents on every request would be slow and fragile.

**Decision.** `deployments.runtime` (`node` or `static`, default `node`, checked by the database) is set by the runner as soon as it has inspected the source, through `Steps.SetRuntime`. It can change only while the deployment is still building. Routing and the reconciler read it with the live release.

## Slice 2: the contract

A static site is a plain `.tar` with:

- `index.html` at the root,
- no `package.json` (its presence makes it a Node app),
- directories and regular files only, with names made of letters, digits and `._@+-`, no leading dot, at most 128 characters per segment and 16 levels deep,
- at most 256 entries, 2 MiB per file and 8 MiB in total (the shared source limits).

There is no build step and nothing executes. The source preparer has already rejected symlinks, special files and sensitive paths such as `.env`. The runner packs the tree into a normalized tar (platform-owned modes, no uploaded timestamps or owners) and stores it as the artifact.

**Stages.** A static deployment still walks the six stages, because the stage rows are created before the source is known. `build`, `provision` and `release` complete immediately (nothing to build, provision or start), and `health_check` proves the stored artifact can be indexed and contains `index.html`. The runtime driver is never called, so a static release has no container, volume or Docker dependency.

**Why names are restricted.** File names become URL paths. A conservative alphabet removes whole classes of problems (encoded separators, control characters, dotfiles, confusable Unicode) before they can reach the serving code.

## Slice 3: serving

**Where.** The proxy looks up the application, checks its lifecycle (paused is 503, archived is 410, exactly as for Node apps), then dispatches on `runtime`. Static requests are answered from an in-memory index of the artifact.

**The cache.** A release never changes, so a loaded site is cached by deployment ID and never invalidated, only evicted. At most eight sites are held (about 64 MiB at the limits). One load at a time runs per release, a failed load is not cached, and the artifact's digest is verified when it is read.

**Serving rules.** The site is untrusted content, so the rules are narrow:

- Only `GET` and `HEAD` (everything else is 405 with `Allow`).
- A path must already be canonical. `/a/../b`, `//a`, `\`, NUL and encoded traversal are 404, not "cleaned" into something that works.
- A directory needs its trailing slash and serves that directory's `index.html`.
- Content types come from a fixed table in Sprout, not from the upload or the host's mime database. Unknown extensions download as `application/octet-stream`.
- Every response carries `X-Content-Type-Options: nosniff`.
- `Cache-Control: no-cache` with a content-derived `ETag`, so a redeploy shows up immediately and unchanged files cost a 304. `http.ServeContent` provides range and conditional handling.
- A site may supply `404.html`; it is returned with status 404.

**Trade-off.** Sprout's own process now returns untrusted bytes. Nothing is executed, and each app has its own host so cookies are not shared, but a site's scripts do run in visitors' browsers on that host. That is inherent to hosting a website; abuse limits and takedown remain on the "before anything leaves localhost" list.

## Request trace

Deploy: upload → source prepared (safe extraction) → no `package.json` → static contract validated → `runtime = static` → packed artifact stored → live (one transaction demotes the previous release).

Visit: `shop.acme.localhost:8081/about/` → strict host parse → live route (lifecycle, `runtime = static`, artifact) → cached site (or load and verify the artifact) → canonical path lookup → `ServeContent` with fixed headers.

When a static release replaces a running Node release, the reconcile loop removes the old container within one interval, as for any superseded release.

## Verification

```sh
cd backend && set -a && . ../.env && set +a && go test -p 1 ./... -race
npm run test:stream && ./node_modules/.bin/tsc --noEmit && npm run lint
```

Tests cover the contract (missing index, nested index, `package.json`, dotfiles, spaces, Unicode, encodings, overlong and too-deep names, cancellation), malicious artifacts (traversal, absolute paths, symlinks, hard links, devices, duplicates, oversized files and totals, garbage), content types, conditional, `HEAD` and range requests, write methods, traversal attempts, lifecycle responses, cache bounds and failure healing, and that a static deployment never calls the runtime driver. A temporary live static deployment was also served through the running API and checked with `curl` (200, 304, 405, 404, then 503 when paused and 410 when archived).

## Known gaps

No build step, so a framework that needs `npm run build` is not yet a static site (it waits for controlled `npm` installs). No clean URLs without the trailing slash, no redirects, and no custom headers. No abuse limits, bandwidth accounting or takedown. Old static artifacts are kept, like all artifacts.

## Explain-back checkpoint

1. Why is `runtime` stored on the deployment instead of detected when a request arrives?
2. Why is a static release immune to the container problems of a Node release, and what risk replaces them?
3. Why does `Lookup` reject `/a/../b` instead of cleaning it?
4. Why can the cache never serve a stale site, and what bounds its memory?
5. A static release goes live over a running Node release. What removes the old container, and when?
