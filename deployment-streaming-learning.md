# Live simulation progress

This implements learning milestone 7: backend SSE and its dashboard integration. The deployment history, detail, start, and cancel controls now use real simulation APIs. This does not build or deploy real application code and does not stream raw build logs.

## Mental model

Previously, a client could ask “how is my job doing?” by repeatedly sending GET requests. Now it can open one HTTP connection and listen while Go sends updated snapshots down that connection. This is **server-sent events (SSE)**: ordinary HTTP, with several messages instead of one JSON response. It is one-way; creating or cancelling jobs still uses separate POST requests.

Think of the subscription as a doorbell, not a storage box. The worker saves a change in PostgreSQL, then rings the bell. The stream handler checks permissions and reads the latest saved state. If the bell rings five times while the browser is slow, one pending notification is enough. The browser needs the current state, not five copies of old state.

In NestJS, an SSE endpoint often returns an Observable. In Django, streaming responses need an appropriate streaming server/runtime. Here, Go's HTTP handler waits on channels and request cancellation, writes an SSE frame, and flushes it to the client. The HTTP server owns the handler goroutine; we do not launch another goroutine for every notification.

## Request trace

1. A client opens `GET /api/v1/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations/{deploymentId}/events`, using its Clerk bearer token.
2. Existing authentication resolves the local user. Subscription checks current application access before registering a listener.
3. The listener is registered **before** the first database snapshot is read, so an intervening worker update cannot be missed.
4. The handler sends the current job as a `progress` event. Its `id` is a hash of that snapshot.
5. Worker updates notify listeners only after persistence. Each listener reads a fresh, authorized job snapshot. A heartbeat also rechecks access every ten seconds, even when the worker is idle.
6. Terminal snapshots are followed by `complete`. Revoked access or a failed read sends a generic `unavailable` event and closes the stream without exposing internal errors.
7. Closing the browser, expiring the session, reaching the connection lifetime, or shutting down the API releases the subscription. It does **not** cancel the job.

## Capacity, cancellation, and security

- Each subscriber has one buffered notification, containing no job data. Sending a notification never waits for the browser to consume it.
- Limits: four streams per local user and 128 streams per API process. Capacity rejection is HTTP 503 with `Retry-After: 3`.
- One connection lasts at most 25 seconds, or until the verified session's expiry, whichever comes first. A client must reconnect with a fresh token. Each database read is bounded to five seconds; each write/flush has a five-second deadline.
- The API's process context closes streams during graceful shutdown, before the workers and database are torn down.
- Membership is rechecked on each snapshot read, including heartbeats. Revocation is not instantaneous: an idle stream may take up to the heartbeat interval plus the bounded read time to notice.
- Only typed simulation fields are emitted. No commands, secrets, raw agent prompts, raw provider responses, artifacts, or build output are persisted or streamed.
- A cursor deduplicates identical snapshots; it is **not** a durable sequence number or replay log. Reconnect sends the latest database state, so intermediate states may be skipped. A terminal reconnect still sends `complete` even if its cursor matches.
- The broker is process-local, consistent with the existing single-worker-process ownership guard. Distributed delivery and durable event history are not implemented.

## Client contract

Use streaming `fetch`, not native `EventSource`, because the request needs an Authorization header. Keep the token in memory, never in the URL. Keep the latest event id for `Last-Event-ID` on reconnect; CORS allows that header.

On `progress`, replace the cached simulation snapshot. On `complete`, stop reconnecting. On `unavailable`, stop displaying stale progress and re-fetch with a fresh token to determine whether access or service availability changed. On ordinary EOF, reconnect with bounded backoff and a fresh token. Abort the connection when leaving the page; do not POST cancellation unless the user explicitly asks to cancel the job.

## Verification

Prerequisites: local PostgreSQL running with the existing migrations applied; no schema changes in this milestone. From `backend/`, load the ignored local database environment without printing it, then run:

```sh
set -a
source ../.env
set +a
go test -race ./...
go vet ./...
```

Automated tests cover ordered snapshots, terminal completion, cursor deduplication, heartbeat/lifetime cleanup, disconnect and shutdown cleanup, reauthorization errors, expired authentication, malformed cursors, hidden jobs, stream capacity, concurrent subscription release, and notification coalescing. Existing PostgreSQL integration tests cover the scoped repository reads used by the stream. HTTP tests inject explicit test claims; they do not establish a live Clerk session.

Manual verification with an authorized development account remains required:

1. Start the API and create a simulation using the existing authenticated POST endpoint.
2. Open its `/events` URL with an Authorization header using a local HTTP client. Keep credentials out of saved files, shell history, screenshots, and logs. Confirm `text/event-stream` and `Cache-Control: no-store`.
3. Confirm queued/building snapshots, ordered stage changes, then a terminal snapshot and `complete`. Fast changes may coalesce.
4. Reconnect to the finished job with its last event id: expect `complete`, without a duplicate snapshot.
5. Disconnect during a job and GET its state separately: the job should continue. Revoke access while listening: no further authorized snapshots should be delivered after the next access check.
6. Open four streams with one account; the fifth should receive 503. Close one and confirm capacity is reusable. Stop the API and verify listeners exit promptly.

## Explain-back checkpoint

1. Why does closing a progress page stop the listener but not the deployment?
2. Why can the worker safely discard a second pending notification?
3. Why must reconnect check permissions again, even if the client has a valid event id?

Commit boundaries: `feat: stream deployment progress` for the backend; `feat: connect dashboard to deployment simulations` for the frontend. Verify the full Clerk-authenticated journey before proceeding to isolated build execution.

## Dashboard integration

The history page reads the latest 50 simulations. It refreshes every five seconds only while its list includes active work, and offers a manual refresh for jobs started elsewhere. Running a simulation sends one POST and opens its detail page. A failed POST is never automatically retried, because the server may already have accepted the work.

The detail page first fetches an authorized snapshot, then subscribes while the job is active. It refreshes the Clerk token on every reconnect and keeps the cursor only in memory. The stream has a 35-second client deadline and up to five consecutive reconnect delays (1, 2, 4, 8, and 16 seconds), after which the user can retry. A permission/unavailable event hides the old snapshot and offers a fresh authorized lookup. Leaving the page or changing the signed-in identity aborts the reader and pending retry timer. A terminal snapshot cannot be replaced by an older active frame after cancellation.

In plain language: TanStack Query is the dashboard's notebook. An ordinary GET writes the first entry; SSE replaces it as progress arrives. The browser observes the worker, but does not own its lifetime. This client behavior is the same whether the API is written in Go, NestJS, or Django; Go owns the worker and HTTP stream on the server side.

Frontend verification (Node 22.18+ or Node 24+ for native TypeScript test loading):

```sh
npm run test:stream
npm run typecheck
npm run lint
npm run build
```

Stream tests cover split frames, CRLF boundaries, multiline payloads, size limits, fresh-token reconnects, cursor forwarding, terminal completion, unavailable/access errors, bounded retries, abort cleanup, and cross-application response rejection. They use test-only responses, not real Clerk credentials.

Manual dashboard checklist:

1. Sign in as an application editor or owner; open an active application's Deployments tab.
2. Run a simulation. Confirm navigation to its detail, three source/build/package steps, and a final Succeeded state without a live URL or invented build logs.
3. Return to history and start another; immediately cancel. Confirm Cancelled persists after refreshing the page.
4. Start another and leave the detail page. Return later and confirm completion: navigation must not cancel work.
5. Stop the API while observing an active job. Confirm reconnect feedback followed by a retry action, not a frozen page claiming live progress. Restart the API and retry; unfinished work is recovered as process-interrupted failure.
6. As a view-only member, confirm history remains readable and start/cancel are rejected by Go. The UI describes the edit-access requirement; the server remains authoritative.
7. Verify at a narrow viewport and with keyboard navigation. Check that simulation labels, status, retry, and cancellation remain available.

Live Clerk-authenticated browser verification requires an existing signed-in development session; automated stream tests do not replace that final check.
