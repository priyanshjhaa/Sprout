# Sprout Repository Guidance

## Source of truth

- Follow `frontend-implementation.md` for the agreed frontend scope, information architecture, and implementation order.
- Keep the product promise focused: Sprout helps people create, deploy, operate, and share small applications without exposing unnecessary infrastructure complexity.

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


<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
