# Workspace invitations: implementation and learning

This completes the next part of the authentication/authorization milestone. The backend learning plan remains authoritative; this note records the specific feature and its checks.

## Mental model

Clerk answers **“Who are you?”** PostgreSQL answers **“Which workspace are you allowed into?”** An invitation is a temporary offer to create that membership, not a replacement for signing in.

Think of the transaction as a single sealed envelope containing two changes:

1. Mark the invitation used.
2. Add the person to the workspace.

Commit means both changes become permanent. If either fails, rollback removes both changes. We never leave a person with a used invitation but no membership because the second database statement failed.

## NestJS / Django comparison

In Django you may use `transaction.atomic()`. In NestJS an ORM usually exposes a transaction callback. Here `pgx` explicitly starts the transaction; `sqlc` supplies typed query methods bound to that transaction. `defer rollback(...)` handles early returns and is harmless after a successful commit. A short independent cleanup context releases a connection even if the browser request was cancelled.

Two Go request handlers can run at the same time. A PostgreSQL row lock makes team changes for the same workspace take turns. This works even across multiple Go processes; a Go mutex would protect only one process. There are no background workers or new production goroutines in this milestone.

## Request trace

1. Owner submits an email and viewer/editor role from Team.
2. Clerk middleware verifies the session; Go resolves its local user ID.
3. The service locks the workspace and verifies current owner membership.
4. Go generates a random token, stores only its digest, commits, and returns the raw token once.
5. Owner privately shares the link. Sprout sends no email.
6. Recipient opens `/invitations/accept#TOKEN`. The fragment never reaches the HTTP server. The browser keeps it in session storage for same-tab sign-in and clears the address-bar fragment.
7. Sign-in/sign-up uses a fixed, allowlisted return route: `/invitations/accept`. Regular sign-in still returns to `/start`.
8. Recipient explicitly clicks Join workspace. The authenticated POST sends the invitation token in its JSON body, not in the URL.
9. Go freshly fetches the Clerk-verified primary email, checks the identity mapping, locks the workspace, and consumes the unexpired invitation for that email.
10. Membership insertion and consumption commit together. Existing membership is never overwritten. The browser clears the temporary token and navigates to the joined workspace; the workspace picker supports later return visits.

## Acceptance checks

- Owners can create, list and revoke invitations, change non-owner roles and remove non-owner members.
- Non-owners cannot manage the team, even if they manually send requests. Owners cannot remove/demote themselves or another owner.
- Invalid/expired/revoked/used/wrong-email invitations produce the same safe failure and cannot create membership.
- Two concurrent acceptance requests produce one success. Existing member roles are preserved.
- Removing membership deletes explicit application grants. A grant write locks actor/target memberships to coordinate with removal.
- An invitation can never name the owner role, and a token hash is never exposed by list responses.
- Request contexts propagate into database and identity-provider calls. Internal errors, request bodies and credentials are not logged.

## Local verification

Run from the repository root:

```bash
npm run db:up
npm run db:migrate
npm run db:check
npm run typecheck
npm run lint
npm run build
```

Then from `backend/` (use your existing ignored local environment; never paste its values into chat):

```bash
set -a
source ../.env
set +a
GOCACHE=/private/tmp/sprout-go-cache go test ./...
GOCACHE=/private/tmp/sprout-go-cache go test -race ./...
GOCACHE=/private/tmp/sprout-go-cache go vet ./...
GOCACHE=/private/tmp/sprout-go-cache go run ./cmd/api
```

`DATABASE_URL` is required for integration tests; otherwise they explicitly skip. Invitation tests use uniquely named local fixtures and delete only their own records afterward. The race test runs two bounded requests and waits for both before cleanup. Existing authentication middleware tests exercise rejection separately; invitation handler tests trace authenticated actor and Clerk subject through the route.

### Manual two-account check

1. Start the frontend with `npm run dev`; use the origin configured in `SPROUT_WEB_ORIGIN` (normally `http://localhost:3000`).
2. As owner, open Team and invite the second account's verified primary email as Viewer. Copy the link before navigating away; it is not recoverable from storage.
3. Open it in a separate browser profile. Sign in/up and confirm you return to the invitation page. Click Join workspace; confirm you arrive in the shared workspace, not the personal one.
4. Use the workspace picker to move between personal and shared workspaces.
5. Repeat opening the old link: it must fail. Try a new invitation with the wrong account: it must fail without consuming it. Switch accounts and accept with the correct one.
6. As owner, change the member to Editor, then back to Viewer. Explicit app grants and creator privileges are independent of the workspace role.
7. Remove the member with the confirmation control. Refresh the removed account's workspace: access must be denied. Old links must not restore access.
8. Create a fresh invitation, revoke it, and verify its link cannot join. Refresh Team: pending entries and member roles must persist.

No automated test substitutes for the real two-account Clerk OAuth handoff. Do not weaken authentication or manufacture production sessions to perform this check.

### Verification recorded for this milestone

The migration was applied to local PostgreSQL. Go tests (including real database fixtures and concurrent acceptance), race tests, `go vet`, frontend lint/type checks, Drizzle checks and the production build passed. A database failure after invitation consumption was also tested: rollback leaves the invitation unused. Browser inspection confirmed the missing-link state, fragment handling, invitation-aware sign-in and the direct sign-up page. The complete OAuth return, authenticated Team UI and two-account join/removal journey still require the manual check above.

## Explain-back checkpoint

1. Why do we check Clerk's current verified email instead of trusting the email already in our `users` table?
2. What could go wrong if “mark invitation used” and “add membership” were separate commits?
3. Why does the Go API still check membership after Clerk says the session is valid?

## Deliberate boundaries

No email provider, queue, ownership transfer, deployed-site access gate, invitation-history UI or scheduled metadata-retention worker has been added. Raw link recovery is impossible by design: revoke and create a new invitation if the link is lost. Browser session storage is temporary, tab-scoped storage accessible to same-origin JavaScript, not a secret vault; XSS prevention remains essential. Fresh provider lookup can fail; the API then fails closed with a safe retryable response.

Commit boundary: verified backend/schema/contract changes, then the frontend invitation journey and these learning notes. The next deployment milestone belongs on a separate branch after this authorization work is reviewed and merged.
