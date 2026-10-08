# Sprout Repository Guidance

## Source of truth

- Follow `frontend-implementation.md` for the agreed frontend scope, information architecture, and implementation order.
- Follow `backend-development-learning-plan.md` for Go backend architecture, teaching cadence, security rules, verification requirements, and milestone order.
- Keep the product promise focused: Sprout helps people create, deploy, operate, and share small applications without exposing unnecessary infrastructure complexity.

## Product boundary

Sprout deploys untrusted small applications safely to a public URL, then lets teams operate and retire them. Deployment is the product's core scope: it accepts application code as input and turns it into an isolated, observable application with a healthy URL. Decisions and their reasons live in `deployment-strategy.md`. Sprout does not generate code, and it is not a general-purpose cloud console.

- Treat source code, build input, build output, and running applications as untrusted.
- Optimize product and infrastructure decisions for many small, low-traffic applications rather than a few permanently provisioned services.
- Keep the complete application lifecycle visible and understandable: deploy, observe, recover, pause, archive, and delete.
- Live applications are public by default. Workspace roles control who can deploy, change, or delete an app, not who can visit it. Private app URLs are a later option.
- Every deployment starts from a source archive sent to one deploy API; the CLI, the MCP tool (folder or GitHub repo), and the dashboard are its front doors.
- Applications run behind a runtime driver boundary. Untrusted code, builds or running apps, never runs on the machine that holds Sprout's database and secrets.
- Preserve one typed API for the dashboard, CLI, coding agents, and MCP clients.
- Coding agents, the CLI, and the dashboard are clients of the same deploy API. Do not build an embedded code-generation or app-builder experience.
- Do not automatically expose deployed applications as agent tools unless that capability is separately validated and added to the product plan.

## Implementation principles

- Do not over-engineer features or introduce infrastructure before the current product flow requires it.
- Prefer the smallest maintainable implementation that satisfies the current acceptance criteria.
- Keep route files thin and move reusable UI or domain behavior into focused components and feature modules.
- Use an explicit typed API boundary. UI components must not import mock fixtures directly.
- Use TanStack Query for server-state-shaped data, even while the frontend is backed by mocks.
- Do not place core product behavior in Next.js Server Actions; the future API must also support the CLI, coding agents, and MCP.
- Add a dependency only when it removes meaningful complexity or materially improves accessibility.
- Reuse shared primitives and design tokens instead of creating one-off visual patterns.
- Use progressive disclosure. Do not promote logs, deployments, resources, or permissions to global navigation.
- Keep the landing page expressive and the dashboard calm, fast, and task-oriented.

## Quality bar

- Preserve keyboard access, visible focus, semantic HTML, useful loading/error/empty states, and reduced-motion behavior.
- Test behavior in proportion to risk. Critical navigation and product journeys deserve automated coverage.
- Keep the application runnable after each implementation phase.
- Avoid speculative abstractions, premature optimization, and backend simulations that pretend to be production infrastructure.

## Go backend learning workflow

- Teach backend mechanics through the Sprout capability being implemented; do not turn milestones into standalone syntax lessons.
- Teach each concept as a systems story: the problem, the history that shaped the usual solution, the decision in Sprout (pointing at the real code), and its trade-off. See `backend-systems-story.md`.
- Use the sequence: mental model, systems story, NestJS/Django comparison, request trace, implementation, verification, explain-back checkpoint, and commit.
- Keep Drizzle as the only schema and migration owner. The Go service may use `pgx` and `sqlc` against the migrated schema but must not duplicate migrations.
- Do not start concurrent work without defined ownership, cancellation, capacity, error propagation, and shutdown behavior.
- Keep each backend milestone independently runnable, verified, and committed.


<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
