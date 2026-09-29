# Sprout authorization policy

This is the permission contract for the first team-sharing implementation. A valid Clerk session identifies a user; it does **not** grant access to a workspace or application. Every operation must check current PostgreSQL membership and scope its query to that workspace.

## Roles

| Action | Workspace owner | Workspace editor | Workspace viewer |
| --- | --- | --- | --- |
| View a workspace-wide application | Yes | Yes | Yes |
| Create an application | Yes | Yes | No |
| Edit a workspace-wide application | Yes | Yes | No |
| Manage workspace members | Yes | No | No |
| Manage application access | Yes | Creator only | No |
| Permanently delete an application | Yes | No | No |

An application is **workspace-wide** by default, preserving the current behavior. An owner or the application creator may make it **restricted**. A restricted application is visible only to workspace owners, its creator, and workspace members with an explicit application grant. Workspace editors do not automatically see restricted applications created by someone else.

Application grants are `editor` or `viewer`. A grant can only be issued to an existing member of the same workspace. A grant may elevate a workspace viewer to editor for one application; it cannot reduce an owner's access. The creator retains editor-level access while they remain a workspace member. Only the workspace owner or creator can manage grants or switch access mode. Only the workspace owner can permanently delete an application.

If membership is removed, all access ends immediately, even if an application grant remains in the database. Missing or cross-workspace applications return `404`; an authenticated member lacking permission to an otherwise visible operation receives `403`. The UI may hide unavailable actions, but the Go service and SQL must enforce the policy.

This policy governs Sprout's control-plane API. Authentication and authorization for a deployed application's own URL are separate concerns and are not implied by these grants.

## Invitations and membership management

- Only workspace owners create/list/revoke invitations or change/remove members. Nonmembers receive `404`; non-owner members receive `403`.
- Invitations grant only `editor` or `viewer`. Owner promotion, ownership transfer, owner removal and owner demotion are deliberately unsupported.
- Invitation tokens contain 32 cryptographically random bytes. PostgreSQL stores only a SHA-256 digest, email, role, creator, expiry and consumption/revocation timestamps. Raw tokens are returned once, never listed or logged.
- Links expire after seven days and can be accepted once. The accepting Clerk subject must map to the authenticated local user, and a fresh Clerk lookup must supply the matching verified primary email. Cached profile email is insufficient.
- The invitation token travels in a browser URL fragment, then in a POST body. The browser temporarily keeps it in per-tab session storage across OAuth and removes it after acceptance. Never include tokens in request paths, query strings, analytics, screenshots or logs.
- Workspace mutation transactions acquire the same workspace row lock. Consuming an invitation and creating membership commit together; retries cannot alter an existing member's role. Concurrent acceptance has one winner. Invitation responses use `Cache-Control: no-store`.
- Duplicate pending email invitations return `409`; revoke the old invitation before replacing it. Expired invitations may be replaced. At most 100 unexpired pending invitations are allowed per workspace.
- Removal deletes membership and explicit application grants, and revokes pending invitations addressed to the member's recorded email, in one transaction. Grant insertion locks the relevant memberships so a simultaneous removal cannot leave a newly inserted grant behind.
- Downgrading workspace role does not erase explicit application grants or creator privileges. Review application access separately. A removed creator has no access; if later re-invited, creator privileges apply again.
- No email delivery or deployed-application URL authorization is included. See [verification and learning notes](./workspace-invitations-learning.md).

## Request trace and learning checkpoint

Browser session → Clerk verifies identity → Go resolves the local user → SQL checks workspace membership and application scope → service applies the operation rule → SQL performs the read or write → response. In NestJS this might be split between guards and services; in Django between middleware, permissions, and views. In Go the checks are explicit dependencies, and a middleware identity alone is never enough.

Explain back: Why must a restricted application's query include both its workspace and the requesting user? Why should removing workspace membership override an old application grant?
