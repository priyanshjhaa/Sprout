# Local runtime

This continues backend milestone 8. The build half of the pipeline already produced a stored artifact. This milestone starts that artifact as a running application, proves it is healthy, and records it as the application's **live** deployment. It ships in two parts:

1. **Live releases** (part 1): the runtime driver, the local Docker driver, the release and health-check stages, and the `live` status.
2. **Public routing** (part 2): the `{app}.{workspace}.localhost` proxy, wake on request, idle sleep, and real pause, resume and archive.

Commits: `feat: package the start manifest with build output`, `feat: allow one live deployment per application`, `feat: add the runtime driver and local Docker driver`, `feat: release and health-check builds into live deployments`. Part 2: `feat: serve live apps at their public localhost URL`, `feat: sleep idle apps and enforce pause and archive`, `feat: show live URLs and runtime stages in the dashboard`.

## The mental model

A **release** is an immutable, self-contained thing you can start: the built files plus the manifest that says how. Starting it does not change it. Deploying is "build a new release, start it, and only when it answers `/health` make it the live one". Anything that goes wrong along the way must leave the previous release serving.

## Slice 1: a self-contained artifact

**Problem.** The artifact held only `dist/`, but `npm start` reads `package.json`. A runtime could not start what the build produced.

**Decision.** The build container now emits `dist/` and `package.json`, and Go refuses output that lacks either. The start command stays platform-owned: `npm --ignore-scripts start`.

## Slice 2: live is a fact about the application

**Problem.** "Which release is serving?" must have exactly one answer per application, even if two workers race.

**History.** Blue/green and rolling deploys exist because a deploy has a moment where old and new coexist. The safe pattern is to keep serving the old one until the new one is proven.

**Decision.** `deployments.status` gains real use of `live`, with a partial unique index `deployments_one_live_per_app`. Promotion runs in one transaction: demote the previous live row to `succeeded`, then record the new one as `live`. The database, not Go code, refuses a second live release. Routing and the reconciler read `GetLiveRelease` and `ListLiveReleases`.

**Trade-off.** The old release's artifact is kept, so rollback stays possible, at the cost of disk until retention exists.

## Slice 3: the runtime driver

**Problem.** Where an app runs will change (local Docker now; Fly Machines or Cloud Run after the hosting trial). The product must not.

**Decision.** `internal/runtime.Driver` has five operations: `Prepare`, `Start`, `Stop`, `Destroy`, `List`. The local Docker driver drives the Docker CLI exactly as the build runner does: named resources, `io.sprout.*` labels, a pinned image, validated UUIDs in every resource name.

A running app gets: non-root user, read-only root, no capabilities, no-new-privileges, 0.5 CPU, 256 MiB with no extra swap, 64 processes, the release mounted read-only, a port published on `127.0.0.1` only, and a dedicated network with container-to-container traffic disabled. Outbound traffic is allowed because real apps call other services. `Prepare` is the only step that runs as root, only to write a fresh volume, with no network and no capabilities.

**Why the flags are tested.** Command construction is a security boundary. Unit tests assert the required flags and the absence of `--privileged`, host namespaces, the Docker socket, bind mounts and added capabilities.

**Comparison.** In NestJS or Django this would be a deployment service class calling a cloud SDK. Here the interface is the seam, and the Docker CLI is one implementation behind it.

## Slice 4: release and health check

Real builds now have six stages: `source`, `build`, `package`, `provision`, `release`, `health_check`. `provision` stores the release files; `release` starts the container; `health_check` polls `GET /health` every 500 ms for up to 30 seconds. The probe never follows redirects (they could point at other local services) and reads at most 4 KiB.

New failure codes: `provision_failed`, `release_failed`, `health_check_failed`, `runtime_unavailable`. Cancelling during the health check is a cancellation, not a health failure.

**Ownership of cleanup.** The runner marks `Provisioned` before its first driver call, so `Release` removes this deployment's container and volume however the job ends, unless it went live. When it did, `Release` retires the same application's older releases. Startup removes every app container and every release volume that no live deployment owns; shutdown stops running containers.

## Part 2, slice 5: the public URL

**Problem.** A live app was reachable only on a random loopback port. The product promise is a public URL that works whenever someone opens it, even if the app is asleep.

**History.** Heroku's router, and later every platform with scale-to-zero (Cloud Run, Fly), put a router in front of apps. The router owns the hostname, finds the app, wakes it if needed, and forwards the request. That keeps apps simple and cheap.

**Decision.** A separate listener (`SPROUT_APPS_ADDRESS`, loopback only, default `127.0.0.1:8081`) serves `{application}.{workspace}.{SPROUT_APPS_DOMAIN}`. The `Host` header is parsed strictly (exactly two slug labels under the domain) and looked up with `GetLiveRelease`, with no permission check, because live apps are public. Lifecycle maps to status codes: unknown or not deployed is 404, paused is 503, archived is 410. Those pages are Sprout-owned and small.

**The supervisor.** `runtime.Supervisor.Ensure` returns an endpoint, starting the release first if it is asleep. It guarantees one shared start per release however many visitors arrive, a cap of ten running apps, and re-preparing release files from the stored artifact if they are gone. A visitor who disconnects does not cancel the start, because the start runs on a goroutine the supervisor owns and joins on `Close`. It adopts a container a deploy already started instead of restarting it. If a proxied request fails, the supervisor forgets the instance so the next request starts it again.

**Trade-offs.** The proxy trusts nothing from the app: it never forwards client-supplied `X-Forwarded-*` or `X-Sprout-*` headers, caps request bodies at 10 MiB, and sets conservative server timeouts. A cold start costs one container start (about a second for the example) added to the first visitor's request.

## Part 2, slice 6: reconciliation

**Problem.** Many facts change outside the request path: an app goes idle, an owner pauses or archives it, a deploy is superseded, a process crashes. Something must make reality match the database.

**History.** Kubernetes controllers popularised the reconcile loop: observe the actual state, compare it with the desired state, take the smallest action that closes the gap, and do it again later. It is robust because a missed event only delays the fix to the next tick.

**Decision.** The supervisor runs one loop (every 15 seconds, or sooner when kicked). Idle apps (10 minutes) are stopped and keep their files, so waking is fast. Paused apps are stopped and keep their files. Archived apps lose container and files. Resources no live release owns are removed. Pausing or archiving through the API kicks the loop immediately; the proxy already answers 503 or 410 from the database without waiting.

**The dangerous part.** Deleting the wrong thing is worse than deleting nothing. The loop reads Docker's inventory first and the database second, so anything it sees has a database row at least as old; deployments still building are explicitly protected, and so is any start in flight. A container it has never seen (started by a deploy) is adopted on first sight, so its idle clock starts then instead of reading as "idle for ever".

**Concurrency contract.** One goroutine owned by the supervisor; cancelled by the server context; joined by `Close`; each reconcile bounded by 60 seconds; errors logged and retried next tick.

## Part 2, slice 7: the dashboard

The API reports `url` (null until live). The pipeline shows five steps: Source, Build, Package, Release (provision and start), and Live (the health check). The status pill, run page, overview and settings copy describe the real effects: pausing takes the URL offline, archiving also removes the running copy. The stream parser accepts the `live` status and all six stage names, and still rejects anything else.

## Request and job trace

Deployment: record → source loaded → validated → sealed build → `dist` + `package.json` → artifact stored → release volume prepared → container started on a loopback port → `/health` answers 200 → one transaction demotes the old live release and records this one → older runtime resources removed.

Visit: `http://shop.acme.localhost:8081/` → strict Host parse → live release lookup → lifecycle check → supervisor (running? adopt? start? re-prepare files?) → reverse proxy with header hygiene → the app's response.

Sleep: the reconcile loop sees no traffic for 10 minutes → stops the container, keeps the files → the next visit starts it again.

## Verification

```sh
cd backend
set -a; . ../.env; set +a
go test -p 1 ./...
SPROUT_DOCKER_TEST=1 go test -p 1 ./internal/runtime ./internal/buildjob ./internal/nodeapp ./cmd/sourcecheck -count=1
npm run test:stream && npx tsc --noEmit && npm run lint
```

Run Docker tests with `-p 1`: the packages share one Docker daemon, and the leftover-container checks see each other's containers when run in parallel. The tests cover the container flags, an invalid or unpinned input, redirect refusal, timeouts, each failure code, cleanup after failure, retirement of only the same application's old releases, the demote-then-promote transaction, and a real example app going live and being fully removed.

To try it by hand, start the API with local builds on and open an app's URL after a deploy (local development only; uploaded code runs in Docker on this machine):

```sh
mkdir -p -m 700 ~/.sprout-local/sources ~/.sprout-local/artifacts
cd backend && set -a && . ../.env && set +a
SPROUT_ENABLE_LOCAL_BUILDS=1 SPROUT_API_ADDRESS=127.0.0.1:8080 \
  SPROUT_SOURCE_DIR=$HOME/.sprout-local/sources SPROUT_ARTIFACT_DIR=$HOME/.sprout-local/artifacts \
  go run ./cmd/api
# optional: SPROUT_APPS_ADDRESS (default 127.0.0.1:8081, loopback only), SPROUT_APPS_DOMAIN (default localhost)
```

## Known gaps

Build outputs are never deleted, and superseded releases keep their artifact. Apps share the parent `.localhost` cookie scope, so production needs a domain on the public-suffix list. There are no per-app logs. There are no abuse limits (rate limits, takedown) for public URLs. Idle detection counts requests, not long-lived connections. The container boundary shares the host kernel and needs a security review before anything leaves localhost.

## Explain-back checkpoint

1. Why must the old release keep serving until the new one passes its health check?
2. Which component enforces "one live release", and why not Go code?
3. What would be lost if `Provisioned` were set after `Prepare` returned?
4. Why does the helper that unpacks the release run as root, and what limits it?
5. What survives if the Go process crashes mid-deploy, and who removes it?
6. Why does the reconcile loop read Docker before the database, and what would go wrong the other way round?
7. What happens to a visitor who arrives while an app is cold, and to one who leaves during the start?
8. Why does pausing answer 503 immediately even though the container is stopped a moment later?
