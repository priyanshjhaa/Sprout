# Sprout Go Backend Development and Learning Plan

## 1. Purpose

This document is the source of truth for building Sprout's Go backend and for learning the backend concepts exposed by that work.

The goal is not to memorize Go syntax. The goal is to understand how a backend receives work, owns resources, communicates with PostgreSQL and the operating system, handles concurrency, and shuts down safely. Every concept must be learned through a real Sprout capability.

The product path guiding the first backend phases is:

```text
authenticated user
      -> workspace authorization
      -> create or inspect an application
      -> create a deployment
      -> observe progress
      -> reach a healthy application URL
      -> share with an authorized teammate
      -> pause, archive, or retire the application safely
```

The backend must remain useful to more than the web dashboard. Its HTTP API will eventually serve the Next.js frontend, CLI, coding agents, and MCP server.

### Product boundary

Sprout is the runtime and control plane for small, potentially short-lived, agent-generated applications. Application code is an untrusted input, not the product itself. The backend turns that input into an isolated, observable, permissioned workload with an explicit lifecycle.

- Design for many small, low-traffic applications without assuming every application runs permanently.
- Keep deploy, observe, recover, share, pause, archive, and delete behavior explicit in the domain and API.
- Apply workspace and application authorization consistently across the dashboard, CLI, coding-agent, and MCP clients.
- Do not couple the backend to an embedded code-generation harness or automatically expose deployed applications as agent tools.
- Require separate validation and planning before adding agent interoperability, external connectors, or communication-channel integrations.

## 2. Working and teaching method

Every milestone follows the same sequence:

```text
Mental model
    -> systems story
    -> NestJS and Django comparison
    -> request or process trace
    -> smallest useful implementation
    -> automated and manual verification
    -> explain-back checkpoint
    -> dedicated commit
```

Before implementation begins, the concept should be explained in terms of the running system. Syntax is introduced only when it expresses that concept.

The **systems story** explains why the design exists, not how to type it:

1. **Problem:** what goes wrong without this piece, in a real running system.
2. **History:** the incidents, products, or constraints that shaped the usual solution (for example Slowloris for server timeouts, Zip Slip for archive validation, Firecracker for multi-tenant isolation).
3. **Decision in Sprout:** the choice made here, pointing at the actual file.
4. **Trade-off:** what the decision costs and what it deliberately leaves unsolved.
5. **Check:** one question that tests understanding of the system, not the syntax.

The first lesson, covering milestones 0–8, is recorded in [`backend-systems-story.md`](./backend-systems-story.md).

Each milestone is complete only when:

- the intended Sprout behavior works;
- automated checks pass;
- the relevant failure path has been exercised;
- the request, resource, or concurrency lifecycle can be explained in plain language;
- the result is committed separately with the commit named in that milestone.

When pairing on an implementation, explanations should answer:

1. What problem are we solving?
2. What happens inside the Go process?
3. How would NestJS or Django usually hide or model this behavior?
4. Why is this design appropriate for Sprout?
5. What owns this resource, how is it cancelled, and how can it fail?

## 3. Architecture principles

### 3.1 Start as a modular monolith

Sprout begins as one Go API service, one Next.js frontend, and one PostgreSQL database. Keep domain boundaries clear inside the service, but do not introduce network boundaries until independent scaling or isolation is demonstrated as necessary.

Do not add microservices, Kubernetes, Redis, Kafka, a separate job queue, or a custom framework during the initial milestones.

### 3.2 Prefer explicit construction

Dependencies should be created in the program entrypoint and passed to the code that needs them. Avoid a dependency-injection container and global mutable state.

```text
configuration
    -> logger
    -> database pool
    -> repositories
    -> services
    -> HTTP handlers
    -> server
```

Constructors may return interfaces where substitution is valuable, but interfaces should be defined by the consumer and introduced only for a real boundary or test seam.

### 3.3 Use the standard library first

Use Go's standard library for HTTP serving, contexts, JSON, logging, signals, synchronization, process execution, and testing. Add focused libraries where they remove meaningful work:

- Go 1.27.x for the service toolchain;
- `github.com/go-chi/chi/v5` for routing and middleware composition over `net/http`;
- `github.com/jackc/pgx/v5` and `pgxpool` for PostgreSQL;
- `sqlc` for type-safe Go code generated from reviewed SQL queries;
- `log/slog` for structured logging;
- OpenAPI for the external HTTP contract.

Do not add an ORM to the Go service.

### 3.4 Keep one schema owner

The existing Drizzle schema and SQL migrations remain authoritative. Go reads and writes the tables created by those migrations, while `sqlc` generates query code from SQL.

Never describe the same migration in both Drizzle and Go. If migration ownership is reconsidered later, it must be a deliberate repository-wide migration rather than gradual duplication.

### 3.5 Make concurrency bounded and owned

Do not start a goroutine without answering all of these questions:

- Who owns it?
- How does it stop?
- Which `context.Context` controls its lifetime?
- What is its maximum concurrency?
- Where does its error go?
- What happens when its consumer is slower than its producer?

Concurrency is a tool for coordinating independent or waiting work. It does not automatically make CPU work faster, and it must not be used to hide blocking or unreliable operations.

## 4. Initial service boundary

The Go backend will live beside the Next.js application in this repository. Its initial structure should remain small:

```text
backend/
├── cmd/api/             process entrypoint and dependency wiring
├── internal/
│   ├── config/          validated runtime configuration
│   ├── database/        pool setup and generated query access
│   ├── application/     application domain, service, and repository behavior
│   ├── workspace/       membership and authorization behavior
│   └── httpapi/         router, middleware, handlers, DTOs, and errors
├── queries/             reviewed SQL used by sqlc
├── openapi/             versioned external API contract
├── go.mod
└── sqlc.yaml
```

Route handlers remain thin. A handler decodes and validates transport input, calls a service, translates the result into an HTTP response, and does not contain SQL or domain policy.

The initial request path is:

```text
Browser or API client
  -> TCP connection accepted by net/http
  -> chi router and middleware
  -> one request-handling goroutine
  -> authentication identity
  -> workspace authorization
  -> application service
  -> repository
  -> bounded pgx connection pool
  -> PostgreSQL
  -> explicit response DTO
  -> JSON encoding
  -> HTTP response
```

The request context travels through every layer. Client cancellation or server timeout must cancel database work derived from that request.

## 5. API rules

Product endpoints are versioned under `/api/v1`. Operational probes are not product resources and remain outside that prefix:

```text
GET  /health/live
GET  /health/ready

GET    /api/v1/workspaces/{workspaceSlug}/applications
POST   /api/v1/workspaces/{workspaceSlug}/applications
GET    /api/v1/workspaces/{workspaceSlug}/applications/{applicationId}
PATCH  /api/v1/workspaces/{workspaceSlug}/applications/{applicationId}
```

The first vertical slice stops at these application operations. Deployments, logs, resources, access management, and agent runs are added by later milestones.

### Contract conventions

- OpenAPI is the reviewed source of truth for public HTTP request and response shapes.
- Transport DTOs are explicit and do not expose generated query structs or database rows.
- JSON uses stable string identifiers and RFC 3339 timestamps.
- Create operations return `201 Created`; successful reads and updates return `200 OK`.
- Invalid input returns `400`, missing authentication returns `401`, insufficient access returns `403`, missing resources return `404`, and uniqueness conflicts return `409`.
- Unexpected failures return `500` without internal error text, SQL, stack traces, credentials, or provider responses.
- List endpoints use a response object rather than a bare array so pagination can be added compatibly.

Every error uses one envelope:

```json
{
  "error": {
    "code": "application_not_found",
    "message": "The application could not be found.",
    "requestId": "01J..."
  }
}
```

Machine-readable error codes are stable. Messages are safe for users. Detailed causes remain in structured server logs and must be redacted.

## 6. Security and data boundaries

Every workspace or application operation must prove membership through the workspace boundary. Looking up an application by ID alone is not sufficient.

The application database and logs must never contain:

- passwords or password hashes;
- OAuth access or refresh tokens;
- Git credentials or credential-bearing clone URLs;
- environment variable values, API keys, certificates, or private keys;
- raw agent prompts or model transcripts;
- source archives, build artifacts, or container images;
- unrestricted build/runtime logs or raw provider error payloads.

Use stable, allow-listed failure codes at persistence and API boundaries. Treat request bodies, headers, query strings, repository metadata, subprocess output, and Docker responses as potentially sensitive.

Authentication is delegated to an external identity provider. Provider selection and session transport must be designed before the authentication milestone; the database continues to store only provider identity metadata.

## 7. Learning and implementation milestones

### Milestone 0: Toolchain and Go runtime

**Build:** Install a supported Go 1.27.x toolchain, create the backend module, and produce a minimal compiled command.

**Understand:** Compilation, modules, packages, exported identifiers, the runtime, garbage collection, stack growth, native processes, and why a Go deployment usually ships as one binary.

**Compare:** NestJS requires Node.js and resolves much application structure at runtime. Django runs inside Python and commonly behind a WSGI or ASGI server. Go compiles the application and its dependencies, while retaining a runtime for scheduling and memory management.

**Trace:** Source files -> compiler -> executable -> operating-system process -> exit status.

**Verify:** `go version`, `go test ./...`, `go vet ./...`, build the binary, run it, and inspect its exit behavior.

**Explain back:** What remains in a compiled Go program at runtime? What is the difference between the Go runtime and the Node.js or Python runtime?

**Commit:** `chore: initialize Go backend service`

### Milestone 1: HTTP lifecycle

**Build:** Create the `net/http` server with chi routing, `GET /health/live`, JSON helpers, safe server timeouts, and graceful shutdown.

**Understand:** Listeners, TCP connections, HTTP parsing, handlers, middleware, goroutines per active request, response writers, request bodies, contexts, timeouts, and OS signals.

**Compare:** NestJS decorators and Django URL/view declarations register behavior through framework machinery. Go exposes the handler contract directly and composes middleware around it.

**Trace:** Client connection -> server listener -> router -> handler goroutine -> response -> connection reuse or close.

**Verify:** Handler tests with `httptest`, malformed-method behavior, JSON content type, timeout behavior, SIGTERM shutdown, `go test ./...`, and `go vet ./...`.

**Explain back:** Who creates the request goroutine? When can the request context be cancelled? Why must response headers be written before the body?

**Commit:** `feat: add Go HTTP service foundation`

### Milestone 2: Configuration and observability

**Build:** Add validated environment configuration, `slog` JSON logging, request IDs, panic recovery, request summaries, and `GET /health/ready`.

**Understand:** Process environments, startup validation, dependency health, structured events, correlation identifiers, recovery boundaries, and the difference between liveness and readiness.

**Compare:** NestJS and Django provide conventions or plugins for much of this lifecycle. Go keeps construction and middleware order explicit.

**Trace:** Process start -> configuration validation -> dependency construction -> ready state -> request log -> shutdown.

**Verify:** Missing or invalid configuration fails startup, secrets never appear in logs, request IDs cross the response boundary, panics produce a safe `500`, and readiness reflects dependency state.

**Explain back:** Why should invalid configuration fail before serving traffic? Why is recovering at an HTTP boundary different from ignoring an error?

**Commit:** `feat: add backend configuration and observability`

### Milestone 3: PostgreSQL access

**Build:** Connect `pgxpool` to the existing Docker PostgreSQL service, configure `sqlc`, add a database readiness check, and implement the first read-only workspace/application queries.

**Understand:** Connection pools, database sessions, SQL execution, generated query code, query cancellation, pool limits, transactions, isolation, and row scanning.

**Compare:** An ORM in NestJS or Django often combines schema mapping, query construction, and persistence behavior. Sprout keeps reviewed SQL, generated types, and schema migrations as distinct concerns.

**Trace:** Request context -> repository -> pool acquisition -> PostgreSQL session -> SQL -> rows -> domain result -> pool release.

**Verify:** Query tests against the local migrated database, unavailable-database readiness, cancellation and timeout behavior, uniqueness constraints, and proof that pool capacity bounds simultaneous database work.

**Explain back:** Why is a pool not one permanent connection per request? What happens when all connections are busy? How does cancellation reach PostgreSQL?

**Commit:** `feat: connect Go backend to PostgreSQL`

### Milestone 4: First vertical API slice

**Build:** Define OpenAPI operations and implement authenticated-placeholder create, list, view, update, and explicit lifecycle-state application flows. Replace only the corresponding frontend mock calls with HTTP requests.

**Understand:** Transport DTOs, domain models, service boundaries, validation, repository errors, HTTP status translation, idempotency considerations, and contract ownership.

**Compare:** NestJS pipes/classes and Django serializers/model forms bundle common transformations. Go uses explicit decoding, validation, mapping, and error handling so each boundary is visible.

**Trace:** TanStack Query -> HTTP client -> handler -> workspace guard -> service -> repository -> PostgreSQL -> response DTO -> query cache.

**Verify:** Happy paths; valid and invalid lifecycle transitions; invalid names/slugs; unknown workspace/application; duplicate slug conflict; cross-workspace lookup rejection; database failure; frontend loading/error/empty states; frontend typecheck, lint, and build.

**Explain back:** Why should database structs not become API responses? Which layer owns business validation? How does a database uniqueness error become `409` safely?

**Commit:** Use separate commits for the API slice and frontend adapter, keeping both applications runnable after each commit.

### Milestone 5: Authentication and authorization

**Build:** After choosing an identity provider and session model, replace the placeholder identity with verified authentication, external identity mapping, workspace membership checks, application permissions, invitations, access grants, and access removal for secure sharing.

**Understand:** Authentication versus authorization, cookies/tokens, trust boundaries, identity claims, tenant isolation, least privilege, and authorization close to resource access.

**Compare:** NestJS guards and Django authentication middleware provide framework conventions. Go middleware may establish identity, but services and repositories must still enforce resource scope explicitly.

**Trace:** Credential/session -> verification -> request identity -> membership lookup -> authorized service operation -> audit-safe result.

**Verify:** Missing, expired, malformed, and wrong-user sessions; viewer/editor/owner permissions; invitation acceptance and expiry; inherited and application-specific access; access removal; cross-workspace access; deleted membership; and negative tests for every protected operation.

**Explain back:** Why is successful authentication not enough? Why must application queries include the workspace boundary even after middleware runs?

**Commit:** Split identity integration and authorization enforcement into independently verified commits.

### Milestone 6: Concurrency fundamentals

**Build:** Model deployment work with an in-process, bounded worker pool and simulated jobs before invoking real build tools. Capacity must remain explicit when many small applications submit work concurrently.

**Understand:** Goroutines, channels, mutexes, `select`, worker ownership, bounded queues, backpressure, cancellation, error propagation, concurrent versus parallel work, and the race detector.

**Compare:** Node.js commonly coordinates asynchronous I/O through an event loop and promises; Python commonly uses processes, threads, or asyncio. Go schedules many goroutines across OS threads and makes synchronization explicit.

**Trace:** API request -> persisted queued deployment -> bounded channel -> worker -> lifecycle events -> completion or cancellation.

**Verify:** Queue-full behavior, concurrency limits, bounded per-application work, absence of starvation across applications, ordered lifecycle changes, cancellation, worker panic containment, shutdown with work in flight, deterministic tests, and `go test -race ./...`.

**Explain back:** Why is an unbounded goroutine-per-job design unsafe? When is a mutex clearer than a channel? What applies backpressure?

**Commit:** `feat: add bounded deployment worker`

### Milestone 7: Streaming and long-running work

**Build:** Stream typed deployment progress with Server-Sent Events while keeping raw log persistence outside PostgreSQL.

**Understand:** Long-lived HTTP responses, flushing, heartbeats, disconnect detection, context cancellation, fan-out, slow consumers, bounded buffers, and reconnect cursors.

**Compare:** Frameworks offer streaming adapters, but the same ownership and pressure problems remain. Go exposes them through interfaces such as `http.Flusher` and contexts.

**Trace:** Worker event -> subscriber broker -> bounded client channel -> SSE encoder -> browser -> TanStack Query-adjacent stream state.

**Verify:** Ordered events, reconnect behavior, client disconnect cleanup, slow consumers, worker completion, server shutdown, redaction, and race tests.

**Explain back:** What keeps a streaming response alive? How is a disconnected client detected? What prevents one slow browser from blocking deployment work?

**Commit:** `feat: stream deployment progress`

### Milestone 8: Operating-system and Docker interaction

**Build:** Replace simulated work incrementally with controlled source preparation, Docker build/container operations, per-application resource budgets, health checks, cleanup, and stable deployment events. Treat every source tree, build step, artifact, and runtime process as untrusted.

**Understand:** Processes, file descriptors, temporary directories, signals, subprocess cancellation, stdout/stderr pipes, Docker's API boundary, resource limits, isolation, and cleanup after partial failure.

**Compare:** NestJS and Django can invoke the same OS capabilities, but Go's process, I/O, concurrency, and cancellation primitives fit naturally in one compiled service. This is a suitability advantage, not a claim that Go makes unsafe operations safe automatically.

**Trace:** Deployment record -> isolated workspace -> build process/API -> bounded output -> container -> health check -> live or failed status -> cleanup.

**Verify:** Invalid and malicious source, build failure, excessive output, timeout, cancellation, unhealthy container, process crash, network and filesystem boundary violations, orphan cleanup, CPU/memory/process limits, and secret redaction.

**Explain back:** What resources survive if the Go process crashes? Which boundary provides isolation? Why is command construction a security boundary?

**Commit:** Use one commit per independently working system capability; never combine build, runtime, proxy, and cleanup into one unreviewable change.

### Milestone 9: Production hardening

**Build:** Add metrics, tracing, profiling controls, graceful draining, deployment packaging, backup/recovery procedures, production-facing security checks, and the operational application lifecycle: pause, resume, archive, restore, and permanent deletion. Define how idle applications release runtime capacity without losing their persisted configuration or access model.

**Understand:** Service-level indicators, latency distributions, saturation, distributed traces, profiles, deployment health, failure recovery, and operational feedback loops.

**Compare:** Framework ecosystems supply integrations, but observability still requires choosing useful signals and defining failure behavior. Go's runtime and profiling tools expose process behavior directly.

**Trace:** Release artifact -> startup -> readiness -> traffic -> saturation/failure signal -> drain -> shutdown or recovery.

**Verify:** Load behavior across many small applications, pool and worker saturation, shutdown during active requests/jobs, paused-runtime capacity release, archive and restore behavior, deletion cleanup, restoration from backup, safe diagnostics, container scanning, and documented rollback.

**Explain back:** Which signals show user impact? What is drained during shutdown? What can be recovered automatically and what requires operator action?

**Commit:** Split observability, packaging, and recovery into reviewable milestones.

## 8. Testing and verification policy

Run checks in proportion to the boundary being changed:

```bash
go test ./...
go vet ./...
go test -race ./...
```

- Unit-test pure validation, domain rules, error mapping, and handlers.
- Use `httptest` for the HTTP boundary.
- Use the migrated local PostgreSQL instance for repository integration tests; tests must isolate their records and clean up deterministically.
- Test cancellation, timeouts, unavailable dependencies, authorization denials, and conflicts—not only happy paths.
- Run the race detector whenever concurrent code changes.
- When an API contract or frontend adapter changes, also run `npm run typecheck`, `npm run lint`, and `npm run build`.
- Manually trace at least one successful and one failed request for every vertical slice.

Tests must not require real credentials, external repositories, or a publicly reachable service unless that integration is the explicit milestone under review.

## 9. Definition of done for explanations

An explain-back checkpoint is a short discussion, diagram, or repository note—not a syntax quiz. It should demonstrate that the following are understood:

- the complete request or job lifecycle;
- which layer owns each decision;
- resource ownership and cleanup;
- cancellation and timeout propagation;
- concurrency limits and backpressure;
- how errors cross internal and external boundaries;
- the tradeoff against the equivalent NestJS or Django approach.

If the implementation works but these mechanics cannot yet be explained, the milestone is operationally complete but the learning milestone remains open.

## 10. Change discipline

- Follow the milestones in order unless a documented dependency requires otherwise.
- Keep the service runnable after every commit.
- Commit each completed milestone separately; do not mix frontend redesign, schema changes, and backend foundations.
- Do not generate a new abstraction until two concrete uses make it helpful.
- Record significant API, security, persistence, and concurrency decisions in the relevant implementation documentation.
- Revisit this guide when evidence changes the architecture, and document why the principle or milestone changed.

This plan guides development; it is not a promise to build every infrastructure feature before validating the simpler product flow.
