# Bounded deployment simulations

This implements backend learning milestone 6 on `deployment-jobs`. It deliberately does **not** build, execute or deploy an application. SSE is now covered in [the streaming milestone](./deployment-streaming-learning.md); real Docker operations remain future work.

## The idea in plain language

A deployment takes longer than an ordinary API request. Keeping it inside the request would tie the job's lifetime to that browser connection. Starting a new goroutine for every submission would allow an unlimited amount of work to accumulate.

Instead, Sprout has two workers and a waiting room with eight places. A worker picks the next accepted job, processes it, records its result, then picks another. When the waiting room is full, the API asks the caller to try later. Each application can have only one accepted simulation at a time.

The **goroutines** are the two workers. The **buffered channel** is the waiting room. The **mutex** protects admission, the map of applications with work, and the shutdown flag. PostgreSQL stores job state; the channel is not the permanent record.

In NestJS you might use a background-job library; in Django you might reach for Celery. We do not need those extra services yet. This in-process implementation makes ownership, capacity and cancellation visible. It is not a durable distributed job queue.

## Request and job trace

1. An authenticated client POSTs an empty object to an application's `deployment-simulations` endpoint.
2. Go checks current workspace membership and application edit permission. Viewers cannot submit/cancel unless they have creator or explicit editor privileges.
3. Admission checks process availability, per-app ownership and queue capacity.
4. A transaction creates a `queued` deployment with `simulated=true` and three pending stage records. A partial unique index independently prevents two active simulations for the same application.
5. After commit, the manager puts the job on its bounded channel and the API returns `201` with a `Location` header. Request failure before commit creates no accepted job; an ambiguous response after commit should be followed by a list request before retrying.
6. A worker rechecks the submitter's permission and the application's active lifecycle, then records `building`.
7. Source, build and package stages run in order. Each simulated stage waits one second, checking cancellation. No command, repository checkout, raw log or container is involved.
8. The job becomes `succeeded`, `failed` or `cancelled`. A successful simulation never becomes `live`, never changes application lifecycle and never produces a usable app URL.

## Ownership and limits

| Resource | Owner | Bound / end condition |
| --- | --- | --- |
| HTTP request | HTTP server | Database-facing handler deadline: 5 seconds |
| Worker goroutines | Manager constructed in `cmd/api` | Exactly 2; joined by `Close` before the DB pool closes |
| Pending channel | Manager | 8 entries; full queue returns `503 deployment_queue_full` with `Retry-After: 3` |
| Accepted work per app | Manager + PostgreSQL unique index | 1 queued/running job; duplicates return `409` |
| Running job | Worker context | 15-second deadline, explicit cancellation or manager shutdown |
| Final persistence | Worker | Independent 5-second context, so cancelled work can still record its terminal state |
| Process ownership | Dedicated PostgreSQL connection | Session advisory lock per schema; second simulation-owning API process fails startup |

The ownership connection is checked before worker writes and by readiness. Losing it fails closed; it is never silently replaced by another pooled connection. This startup guard is **not** a distributed lease/fencing protocol or high-availability design. Run one API process per schema.

FIFO admission plus one accepted job per application prevents one app from repeatedly overtaking other accepted apps. Each running simulation has a deadline. This does not promise fair admission across users or tenants under hostile load; rate limits and tenant scheduling belong to later hardening.

The simulated runner honours context cancellation. Any future runner must do the same; Go cannot forcibly stop an arbitrary function that ignores its context. Real process cancellation and cleanup must be designed before replacing the simulation.

## Cancellation, errors and restart

- Closing the browser after acceptance does **not** cancel a job. Its context belongs to the manager, not the request.
- An authorized cancellation first commits the terminal database state, then signals the in-memory job. A late worker completion cannot overwrite it. The per-app memory slot remains occupied until the worker observes cancellation; a cancelled queued item is drained normally.
- Only allowlisted failure codes are persisted. Runner error strings, panic values and raw outputs never reach the database, response or logs.
- A runner panic becomes `worker_panic`; the worker continues serving later jobs. A timeout becomes `job_timeout`.
- Failure to persist a terminal result marks the manager unhealthy, cancels its jobs and rejects new submissions. Read/list endpoints remain useful for diagnosis. Restart is required after resolving the database problem.
- An unexpected persistence failure during admission also stops new work: a lost commit acknowledgement may have left a queued row without an in-memory task. Restart recovery resolves that ambiguity rather than silently leaving the row stuck.
- During graceful shutdown the HTTP server stops/drains requests first. The manager then stops admission, cancels active work, drains waiting jobs into cancelled states and joins its workers. The database remains open during cleanup.
- After an abrupt process exit the channel is lost. On next startup, **after acquiring exclusive process ownership**, unfinished simulations become failed with `process_interrupted`. They are not replayed automatically. Real deployments (`simulated=false`) are untouched.

## API

All paths below start with:

```text
/api/v1/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations
```

| Method | Suffix | Result |
| --- | --- | --- |
| POST | empty | Submit `{}`; receive persisted queued simulation |
| GET | empty | Latest 50 visible simulations, newest first |
| GET | `/{deploymentId}` | Current state with ordered stage summaries |
| POST | `/{deploymentId}/cancel` | Submit `{}` to cancel queued/running work |

Every operation requires a valid Clerk session. Read permission follows existing application visibility. Submit/cancel require application edit permission. Cross-workspace or hidden resources return `404`; a visible but forbidden operation returns `403`. Inactive apps, duplicate active work and cancelling a terminal job return `409`. Responses use `Cache-Control: no-store`.

The existing dashboard deployment tab still uses its previous adapter. No simulation is silently substituted for a real deployment. Use the explicit API for this milestone; UI wiring and progress streaming are separate work.

## Verification

From the repository root:

```bash
npm run db:up
npm run db:migrate
npm run db:check
npm run typecheck
npm run lint
npm run build
```

From `backend/`, load your existing ignored local environment without printing it:

```bash
set -a
source ../.env
set +a
GOCACHE=/private/tmp/sprout-go-cache go test ./...
GOCACHE=/private/tmp/sprout-go-cache go test -race ./...
GOCACHE=/private/tmp/sprout-go-cache go vet ./...
GOCACHE=/private/tmp/sprout-go-cache go run ./cmd/api
```

Do not run a second API process alongside one already owning the simulation lock. Stop the existing local API before starting this version.

Tests cover capacity, bounded parallelism, FIFO order, one-job-per-app admission, detached request lifetime, queued/active cancellation, timeouts, panic containment, shutdown, terminal persistence failure and startup ownership. Repository tests use a uniquely named temporary PostgreSQL schema with copies of the migrated table definitions and indexes; it is removed afterward. They do not recover or alter your real deployment records. The test copies do not reproduce foreign keys; existing migrated database tests cover the shared schema separately.

Repository tests also cover viewer/outsider rejection, restricted-app grants, revoked membership before execution, inactive applications, atomic stage transitions, cancellation winning over completion, and recovery leaving non-simulated deployments unchanged. HTTP tests trace authenticated scope into submission and verify the error envelope, rejection of arbitrary input, authentication requirements and queue backpressure responses. Integration tests explicitly skip if `DATABASE_URL` is absent.

### Manual API check

Use a trusted local REST client configured with your own Clerk session bearer token. Do not paste it into chat, commit it, or put it in URLs/screenshots.

1. Pick an active application you can edit. POST `{}` to the explicit simulation path. Expect `201`, `simulated: true`, `status: queued`, and three pending stages.
2. GET the returned `Location`. Observe ordered progress and eventually `succeeded`, never `live`.
3. Submit another and immediately cancel it. Expect `cancelled`; later GETs must not revert to success.
4. Try submission as an ordinary viewer without creator/editor grants: expect `403`. Try a different workspace with the same app ID: expect `404`.
5. Submit twice rapidly for the same app: expect one accepted job and a `409` conflict. If both finish sequentially, both may legitimately succeed—automated tests hold the runner to test this deterministically.
6. Stop the API during accepted work and restart it. Graceful shutdown should cancel unfinished work; abrupt termination is recovered as `process_interrupted` on restart.

Do not manually flood the server to test saturation; the deterministic worker tests cover queue limits without generating unnecessary historical jobs.

### Recorded verification

The local migration, Drizzle check, frontend type/lint/build checks, full Go test suite with PostgreSQL enabled, race detector and `go vet` passed. Deployment race/integration tests were repeated. OpenAPI local references were checked. A temporary API on port 8081 returned ready/200, rejected unauthenticated simulation submission with 401, and handled shutdown; it was then stopped, leaving the existing server on port 8080 untouched. Successful authenticated submission/error traces are covered by HTTP tests and real database tests; a live Clerk-authenticated manual simulation remains to be tried using the flow above.

## Explain-back checkpoint

1. Why should a submitted deployment survive its browser request ending?
2. What happens when both workers are busy and all eight waiting places are occupied?
3. Why do we save the deployment in PostgreSQL before placing it on the channel?
4. Why does recording cancellation need a fresh short-lived context after the job context is cancelled?

Commit boundary: `feat: add bounded deployment worker`. Next: typed progress streaming (milestone 7), before any real untrusted build execution.
