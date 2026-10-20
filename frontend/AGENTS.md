# Frontend (Next.js)

Product, scope and engineering rules for the whole repository are in the root [AGENTS.md](../AGENTS.md). Follow `../frontend-implementation.md` for the agreed frontend scope. This file holds the Next.js-specific rules, which `next dev` manages here because this folder is the Next.js project.

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
