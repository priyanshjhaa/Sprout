# Local runtime

This continues backend milestone 8. The build half of the pipeline already produced a stored artifact. This milestone starts that artifact as a running application, proves it is healthy, and records it as the application's **live** deployment. It ships in two parts:

1. **Live releases** (this part): the runtime driver, the local Docker driver, the release and health-check stages, and the `live` status. A live app is reachable only on a loopback port for now.
2. **Public routing** (next part): the `{app}.{workspace}.localhost` proxy, wake on request, idle sleep, and real pause, resume and archive.

Commits: `feat: package the start manifest with build output`, `feat: allow one live deployment per application`, `feat: add the runtime driver and local Docker driver`, `feat: release and health-check builds into live deployments`.

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

## Request and job trace

Deployment record → source loaded → validated → sealed build → `dist` + `package.json` → artifact stored → release volume prepared → container started on a loopback port → `/health` answers 200 → one transaction demotes the old live release and records this one → older runtime resources removed.

## Verification

```sh
cd backend
set -a; . ../.env; set +a
go test -p 1 ./...
SPROUT_DOCKER_TEST=1 go test -p 1 ./internal/runtime ./internal/buildjob ./internal/nodeapp ./cmd/sourcecheck -count=1
```

Run Docker tests with `-p 1`: the packages share one Docker daemon, and the leftover-container checks see each other's containers when run in parallel. The tests cover the container flags, an invalid or unpinned input, redirect refusal, timeouts, each failure code, cleanup after failure, retirement of only the same application's old releases, the demote-then-promote transaction, and a real example app going live and being fully removed.

## Known gaps

No public URL yet; build outputs are never deleted; apps do not restart after an API restart until wake-on-request exists; no per-app logs; abuse limits for public URLs; the container boundary shares the host kernel and needs a security review before anything leaves localhost.

## Explain-back checkpoint

1. Why must the old release keep serving until the new one passes its health check?
2. Which component enforces "one live release", and why not Go code?
3. What would be lost if `Provisioned` were set after `Prepare` returned?
4. Why does the helper that unpacks the release run as root, and what limits it?
5. What survives if the Go process crashes mid-deploy, and who removes it?
