# Sprout

Sprout is an agent-native cloud concept for small, purpose-built applications.

It explores a simple product question:

> What should the cloud look like when software is small, temporary, highly customized, and increasingly written by coding agents?

The long-term goal is to make deploying and securely sharing a small application feel as straightforward as sharing a document. A developer or coding agent provides the application; Sprout handles the path from source code to a healthy URL, along with identity, configuration, logs, data, and access.

This repository currently contains the first frontend prototype, the PostgreSQL schema/local development setup, and the initial Go backend executable. Infrastructure and agent operations are represented with typed mock data and are clearly identified as demonstrations. The Go backend is being built incrementally using the repository's backend development and learning guide.

## Product direction

Sprout begins where code generation ends:

```text
Application code
      ↓
Build and isolate
      ↓
Health check
      ↓
Managed URL
      ↓
Database · Secrets · Storage · Logs
      ↓
Share with coworkers
```

It is not intended to be another prompt-to-React generator. The interesting product and engineering work is creating a safe, calm environment where agent-built software can run and be shared with real people.

## Current frontend

The prototype includes:

- A scroll-driven landing page that explains the application lifecycle inside one persistent visual environment
- A simplified semantic mobile landing experience
- Authentication entry screen
- Responsive workspace shell and navigation
- Agent workspace with a mocked application-creation journey
- Searchable application library
- Application health overview
- Deployment history and deployment-level build logs
- Searchable and filterable runtime logs
- Environment variables and attached resources
- Workspace-inherited access and application roles
- Application settings and guarded destructive actions
- Loading, empty, failure, and responsive states

The dashboard deliberately keeps only **Agent** and **Apps** prominent. Operational concepts such as deployments, logs, environment, and permissions remain inside the selected application instead of becoming global cloud-console navigation.

## Technology

- [Next.js](https://nextjs.org/) App Router
- React and TypeScript
- Tailwind CSS
- [TanStack Query](https://tanstack.com/query/latest) for server-state-shaped data
- Lucide icons
- A typed HTTP adapter for application data, with focused mocks for unfinished capabilities

The frontend uses an explicit API boundary. Components consume typed TanStack Query hooks rather than importing fixtures directly. The application list and detail views now read from the Go API; unfinished deployment, log, environment, access, and agent capabilities remain behind the same boundary as mocks.

## Getting started

Requirements:

- Node.js 20 or newer
- npm
- Go 1.27.x for backend development
- `sqlc` 1.31.x for generating typed Go queries

Install dependencies:

```bash
npm install
```

Start the development server:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

Start and migrate PostgreSQL, add the safe local demo records, then load the untracked local environment into your current shell:

```bash
npm run db:up
npm run db:migrate
npm run db:seed
set -a
source .env
set +a
```

Only source an environment file you trust. Then generate, verify, and run the backend:

```bash
cd backend
sqlc generate
sqlc vet
go test ./...
go vet ./...
go run ./cmd/api
```

The API listens on `http://127.0.0.1:8080`. Keep it running while using the dashboard and start `npm run dev` from the repository root in another terminal. Verify its liveness endpoint with:

```bash
curl http://127.0.0.1:8080/health/live
```

Set `SPROUT_API_ADDRESS` to override the default address. `/health/live` reports that the process is alive, while `/health/ready` now checks the PostgreSQL pool before reporting that the API is ready for traffic.

The first versioned API slice supports listing, creating, viewing, and updating workspace applications under `/api/v1/workspaces/{workspaceSlug}/applications`. During local development these routes require an `X-Sprout-User-ID` header containing the UUID of an existing workspace member. This placeholder is not production authentication and will be replaced during the authentication milestone. The reviewed contract is in [`backend/openapi/openapi.yaml`](./backend/openapi/openapi.yaml).

Press `Ctrl+C` in the server terminal to perform a graceful shutdown.

For database work, follow the [local PostgreSQL setup](./local-postgres.md) to start the localhost-only Docker service and apply the Drizzle migration.

Useful demo routes:

```text
/                                      Landing experience
/sign-in                               Authentication entry
/workspace/acme/agent                  Agent workspace
/workspace/acme/apps                   Application library
/workspace/acme/apps/{application-id}  Application overview
```

## Project commands

```bash
npm run dev        # Start the development server
npm run build      # Create a production build
npm run start      # Run the production build
npm run lint       # Run ESLint
npm run typecheck  # Run TypeScript without emitting files
npm run db:up      # Start the local PostgreSQL container
npm run db:status  # Check PostgreSQL container health
npm run db:migrate # Apply pending Drizzle migrations
npm run db:seed    # Add idempotent local demo records
npm run db:down    # Stop PostgreSQL without deleting its data
```

The production build currently uses Next.js with webpack because the restricted development environment used for this prototype does not permit one of Turbopack's internal CSS worker operations.

## Source structure

```text
src/
├── app/                  Next.js routes and layouts
├── components/
│   ├── access/           Sharing and roles
│   ├── agent/            Agent workspace
│   ├── apps/             App library, shell, and overview
│   ├── dashboard/        Workspace navigation
│   ├── deployments/      Deployment history and details
│   ├── environment/      Variables and resources
│   ├── logs/             Runtime log viewer
│   ├── marketing/        Scroll-driven landing experience
│   └── settings/         Application configuration
├── lib/
│   ├── api/              HTTP adapter and remaining focused mocks
│   └── query/            Query keys and typed hooks
└── types/                Frontend domain types
```

The Go backend lives in `backend/`. Its `cmd/api` package is the executable entrypoint. Drizzle remains the only schema and migration owner; reviewed SQL in `backend/queries` is converted by `sqlc` into typed query methods used through the bounded `pgx` connection pool.

## Design principles

- Use one visual language across marketing and product surfaces.
- Keep the landing page expressive and the dashboard quiet.
- Show application health and required actions before infrastructure details.
- Introduce complexity through progressive disclosure.
- Use motion to explain state, not as decoration.
- Preserve semantic HTML, keyboard access, and reduced-motion behavior.
- Avoid premature infrastructure and speculative abstractions.

## Current boundary

The following capabilities are mocked and are not connected to production infrastructure:

- GitHub authentication and repository access
- Coding-agent execution
- Application builds and containers
- DNS and TLS provisioning
- Live log streaming
- Database and object-storage provisioning
- Secret encryption
- Invitations and permissions persistence
- Rollbacks and destructive operations

The UI should be treated as a product and interaction prototype, not a hosting service.

## Roadmap

The next implementation phases are intentionally incremental:

1. Add automated component and critical-journey browser tests.
2. Define the versioned backend API from the existing frontend domain model.
3. Replace demo authentication with GitHub OAuth and real workspaces.
4. Implement the smallest deployment loop: repository, Docker build, container, proxy, and URL.
5. Connect deployment state and log streaming to the existing UI.
6. Add database provisioning, encrypted secrets, resource limits, and rollback.
7. Expose the same operations through a CLI and agent-facing API.
8. Add an MCP server after the underlying API is stable.

Sprout should remain deployable on a deliberately small initial architecture—one server, PostgreSQL, Docker, a reverse proxy, and only the supporting services justified by real product needs.

## Project documentation

- [Frontend implementation plan](./frontend-implementation.md)
- [Go backend development and learning plan](./backend-development-learning-plan.md)
- [Database design](./database-design.md)
- [Local PostgreSQL setup](./local-postgres.md)
- [Repository implementation guidance](./AGENTS.md)
