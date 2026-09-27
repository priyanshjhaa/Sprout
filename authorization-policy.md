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

## Request trace and learning checkpoint

Browser session → Clerk verifies identity → Go resolves the local user → SQL checks workspace membership and application scope → service applies the operation rule → SQL performs the read or write → response. In NestJS this might be split between guards and services; in Django between middleware, permissions, and views. In Go the checks are explicit dependencies, and a middleware identity alone is never enough.

Explain back: Why must a restricted application's query include both its workspace and the requesting user? Why should removing workspace membership override an old application grant?
