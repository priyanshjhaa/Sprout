# Access tokens (CLI authentication, part 1)

The deploy API accepts a source archive from any front door, and the dashboard is only one of them. The CLI, CI and coding agents (through MCP) need to call the same API from a terminal or a server, where there is no browser session. This milestone gives them a credential. The CLI itself is the next milestone.

Commits: `feat: store personal access tokens as hashes`, `feat: authenticate with access tokens on an allowlist of deploy routes`, `feat: create and revoke access tokens from the dashboard`, plus these docs.

## The mental model

A **personal access token** is a password you can verify but never read back, that acts as you, but only for a narrow set of things, and that you can revoke at any moment. Each clause is a design decision below.

## Slice 1: a secret you can verify but not read back

**Problem.** A credential for scripts must be long-lived (nobody can click through a login in CI), which makes it dangerous to lose. If the database leaks, it must not hand out working credentials.

**History.** Unix stored password hashes instead of passwords for exactly this reason. GitHub's personal access tokens and Stripe's API keys carry a visible prefix (`ghp_`, `sk_live_`) so secret scanners and humans recognise a leak, and show the secret exactly once.

**Decision.** A token is `sprout_pat_` plus 256 random bits (52 base32 characters). Only its SHA-256 is stored, with the last four characters as a hint so people can tell tokens apart. A fast hash is correct here: password hashes (bcrypt, argon2) are slow on purpose because human passwords are guessable, but a 256-bit random secret cannot be guessed, so slowing the check only adds latency to every request. Lookup is by hash through a unique index, so no secret is ever compared character by character.

**Lifecycle rules.** Every token expires (default 90 days, 1 hour to 365 days). A user may hold at most 20 active tokens. Revocation is a timestamp, and takes effect on the next request because every request looks the token up. "Last used" is written at most once a minute and its failure never fails the request.

**Trade-off.** Anyone holding the secret is you until it expires or is revoked. The remedies are short default lifetimes, visible last-used times, one-click revocation, and the prefix that makes leaks detectable.

## Slice 2: acting as you, but not as everything you can do

**Problem.** A leaked CI token should be able to deploy, not take over the workspace.

**Decision.** The API accepts two credentials on one header. A bearer value starting with `sprout_pat_` is a token; anything else is a Clerk session. A token resolves to its owner, so existing permission checks (workspace roles, per-app grants) apply unchanged, and then passes through an **allowlist** of 12 operations: read the account, list and create applications, and create, read, cancel and follow deployments. It is deny-by-default, so a route added later is closed to tokens until someone opens it on purpose. Team, invitations, access grants, lifecycle changes and token management itself all answer `403 token_not_permitted`.

Token management gets a second lock: its handlers also demand a session. A token can never mint more tokens, list them, or extend its own life, even if the allowlist were widened by mistake.

**Details that matter.**
- An invalid token is always `401`, even on a route tokens may not use, so the allowlist is not revealed to someone without a working token. A storage failure is `503`, never "bad credentials".
- The allowlist matches the escaped URL path, the form the router routes on, so an encoded slash cannot be a separator to one and not to the other.
- Only the exact `Bearer sprout_pat_` prefix selects token authentication. Anything else falls to the session check and is rejected there.
- The secret appears once, in the create response, which is marked `Cache-Control: no-store`. In the dashboard it lives in component memory, never in the query cache or storage.

**Comparison.** In Django REST Framework this is an authentication class plus a permission class, with tokens in a table. In NestJS it is a guard. The allowlist and the "management needs a session" rule are the parts that are easy to forget in either.

## Slice 3: the dashboard

`/workspace/{slug}/tokens` (linked from the Deploy page and the command palette, not global navigation) creates a token with a name and an expiry, shows the secret once with a copy button, and lists tokens with their hint, expiry, last use and a confirmed revoke. Expired tokens stay listed so they can be removed.

## Request trace

`Authorization: Bearer sprout_pat_…` → prefix check → well-formed check (no database for garbage) → SHA-256 → lookup of an unrevoked, unexpired row → owner's user ID → allowlist check → identity loaded as for a session → the normal handler and its permission checks → last-used stamped (throttled).

## Verification

```sh
cd backend && set -a && . ../.env && set +a && go test -p 1 ./... -race
(cd frontend && npm test && npm run typecheck && npm run lint)
```

Tests cover token shape and entropy, that only a hash is stored and listings never contain the secret, name and lifetime validation, the active-token cap and what frees a slot, authentication of valid, malformed, unknown, revoked and expired tokens (and that malformed ones never reach the database), owner-only revocation, the SQL lifecycle against PostgreSQL, the 12 allowed operations, about 25 refused ones including traversal and segment tricks, the 401-before-403 ordering, the exact-prefix rule, infrastructure failures, and the session-only token endpoints. The running API was also exercised with real database rows: a valid token read the account but received `403` for token management, members and invitations, while revoked, expired and garbage tokens received `401`.

## Known gaps

No browser login flow yet (`sprout login` will ask for a pasted token first). Tokens are not scoped to one workspace or app; they inherit their owner's access within the allowlist. No audit log of token use beyond last-used. No rate limiting on authentication. Tokens in transit rely on HTTPS, which local development does not have.

## Explain-back checkpoint

1. Why is SHA-256 enough for a token when bcrypt is right for a password?
2. What can a leaked token do, and what can it not do, and which two mechanisms stop it from minting more tokens?
3. Why does an invalid token get `401` on every route, including ones tokens may not call?
4. Why is the allowlist matched against the escaped path rather than the decoded one?
5. What does the database hold after a token is created, and what is shown to the user afterwards?
