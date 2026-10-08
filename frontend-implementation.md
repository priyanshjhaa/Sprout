# Sprout Frontend Implementation Plan

## 1. Purpose

This document defines the first implementation phase of Sprout: a polished frontend prototype for a deployment platform for small, purpose-built applications.

Sprout's product promise is:

> Take an existing application from code to a healthy public URL with almost no infrastructure work, and make small software as easy to put online as sharing a link.

### 1.1 Product boundary and differentiation

Deployment is Sprout's core scope and differentiator. Sprout is the runtime and control plane for small, potentially short-lived applications, many of them written by coding agents. Application code is the input; an isolated, observable, permissioned application with a healthy URL is the output.

- Sprout starts where code generation ends. It does not generate code and is not a prompt-to-application builder.
- The primary product is the application lifecycle: deploy, inspect health, configure, observe, recover, pause, archive, and delete. Live apps are public by default; the team that operates an app is managed in Sprout. See `deployment-strategy.md`.
- Source code and running applications must be presented as crossing an explicit trust boundary through build, isolation, resource limits, and health checks.
- The product should remain calm and understandable for people who do not want to operate traditional cloud infrastructure.
- The Deploy page is the dashboard's entry path. Coding agents, the CLI, and MCP clients use the same deploy API rather than a built-in agent experience.
- Agent-tool exposure, Slack operation, and similar integrations are not assumed consequences of deployment; each requires separate product validation.

This phase focuses only on the product experience. It should establish the visual identity, landing-page story, dashboard information architecture, reusable component system, responsive behavior, and realistic mocked application states. It does not include the real deployment platform, runtime orchestration, databases, or secrets infrastructure.

## 2. Confirmed technology choices

- Framework: Next.js with the App Router
- Language: TypeScript
- Styling: Tailwind CSS
- Server-state management: TanStack Query
- Accessible UI primitives: Radix UI, preferably through selectively adopted shadcn/ui components
- Forms: React Hook Form
- Validation: Zod
- Tables: TanStack Table when a real data table is required
- Icons: Lucide
- Unit and component testing: Vitest and React Testing Library
- End-to-end testing: Playwright

The frontend must communicate through an explicit API/client boundary. Core Sprout behavior must not become coupled to Next.js Server Actions because the same future API will serve the web dashboard, CLI, coding agents, and MCP server.

For this frontend-only phase, TanStack Query will consume mock API functions with realistic latency and error states. The mock implementation must be replaceable with HTTP calls without requiring page-level rewrites.

## 3. Product and design principles

### 3.1 One visual world

The public landing page and authenticated dashboard should feel like two modes of the same product.

- The landing page is cinematic and expressive.
- The dashboard is quiet, practical, and focused.
- Both share typography, colors, surfaces, spacing, application-node imagery, status language, and motion principles.

### 3.2 Progressive disclosure

Show users the condition of their application first. Reveal infrastructure details only when they ask for them.

- Do not expose every infrastructure feature as a top-level route.
- Do not fill application cards with low-priority metrics.
- Keep advanced settings inside the relevant application.
- Prefer clear defaults and contextual actions over permanent controls.

### 3.3 Cozy rather than sterile

The interface should feel calm, warm, and comfortable without becoming playful or imprecise.

- Use a graphite or warm-neutral base.
- Use a restrained sprout-green accent.
- Use muted borders, soft depth, generous spacing, and rounded surfaces.
- Reserve saturated color for status, feedback, and primary actions.
- Avoid dense enterprise-cloud-console styling.
- Avoid excessive glassmorphism, gradients, and decorative cards.

### 3.4 Motion must communicate state

Motion should explain creation, deployment, connection, and progress.

- Landing-page motion may be expressive and scroll-driven.
- Dashboard motion should be subtle and functional.
- Avoid continuous background movement that competes with content.
- Every essential interaction must work with reduced motion enabled.

## 4. Frontend scope

### Included

- Public scroll-driven landing page
- Authentication screens or authentication entry state
- Workspace shell and navigation
- Deploy page
- Apps index
- Individual application workspace
- Overview, deployments, access, and settings views
- Empty, loading, success, warning, failure, and offline states
- Responsive desktop, tablet, and mobile behavior
- Accessible keyboard interaction and reduced-motion behavior
- Mock API layer and seeded demo data
- Component and end-to-end tests for critical journeys

### Not included in this phase

- Real GitHub OAuth
- Real source repository connection
- Docker builds or deployments
- Live containers
- DNS or HTTPS provisioning
- Real log streaming
- Real databases or object storage
- Secret encryption
- Billing
- Kubernetes or multi-region infrastructure
- Code generation of any kind
- A real REST API or MCP server

Mocked experiences must be visibly credible, but the product must never imply that a mocked deployment is real.

## 5. Application route map

```text
/
  Public scroll-driven landing page

/sign-in
  Authentication entry screen

/workspace/[workspaceSlug]/deploy
  Deploy entry flow: name an application, choose its source, and run a deployment

/workspace/[workspaceSlug]/apps
  Application collection

/workspace/[workspaceSlug]/apps/[appId]
  Application overview

/workspace/[workspaceSlug]/apps/[appId]/deployments
  Deployment history

/workspace/[workspaceSlug]/apps/[appId]/deployments/[deploymentId]
  Deployment details and build logs

/workspace/[workspaceSlug]/apps/[appId]/access
  Members and application permissions

/workspace/[workspaceSlug]/apps/[appId]/settings
  Name, description, and lifecycle (pause, resume, archive, restore)

/workspace/[workspaceSlug]/team
  Workspace members and invitations
```

The global navigation is Deploy, Apps, and Team. Workspace activity and workspace settings are not part of the current scope.

## 6. Landing-page experience

### 6.1 Interaction model

The landing page must not be a conventional stack of hero, feature, testimonial, and CTA sections.

It should use a long semantic document containing a sticky, viewport-sized visual stage. Scroll progress advances a single continuous product story inside that stage.

```text
Long semantic document
        |
        +-- Sticky 100vh visual stage
        +-- Story chapters
        +-- Scroll progress mapped to scene state
```

The background is a persistent Sprout cloud environment: a subtle grid or spatial canvas containing application nodes, fine connection lines, restrained glow, and integrated terminal/browser surfaces.

### 6.2 Story sequence

#### Scene 1: The idea

- Begin with a quiet environment and a single application seed/node.
- Introduce the tension: small software is easy to create but still hard to deploy and share.

#### Scene 2: The application

- Introduce a repository or source-code object.
- Example application: `invoice-approval`.
- Show the project as ready to deploy.

#### Scene 3: Deployment

- Move the repository through Build, Container, and Health Check states.
- Show short, readable build output.
- Motion must reinforce forward progress.

#### Scene 4: Live application

- Transform the result into an integrated browser preview.
- Reveal a Sprout application URL.
- Communicate the repository-to-production outcome.

#### Scene 5: Quiet when idle

- Show the app sleeping when nobody uses it and waking on the next visit.
- Communicate that many small apps stay cheap to keep online.

#### Scene 6: Online and in good hands

- Show the app reachable at a public URL.
- Connect workspace members who operate it, with Owner, Editor, and Viewer roles.
- Communicate that anyone can open it while the team stays in control.

#### Scene 7: The workspace

- Pull back to reveal multiple small applications living in the workspace.
- Resolve into primary and secondary calls to action.

### 6.3 Suggested scroll ranges

```text
0.00-0.15  Introduction
0.15-0.32  Repository appears
0.32-0.53  Deployment pipeline
0.53-0.68  Live application
0.68-0.82  Managed infrastructure
0.82-0.93  Sharing
0.93-1.00  Workspace and CTA
```

These ranges are starting points, not hard-coded design constraints. The final timing should be tuned through browser testing.

### 6.4 Landing-page accessibility and fallback

- Story text must remain real semantic HTML.
- Essential content must not exist only inside a canvas or animation.
- With `prefers-reduced-motion`, replace transforms with restrained fades or a readable staged layout.
- On narrow mobile screens, prefer a simplified sticky experience or a sequential story rather than shrinking the desktop scene.
- Keyboard and assistive-technology users must be able to understand the story in document order.
- The page must remain meaningful if JavaScript fails.

## 7. Authenticated dashboard

### 7.1 Workspace shell

Use a compact global sidebar and a wide, comfortable content area.

```text
Sprout

Deploy
Apps
Team
```

The sidebar should include the current workspace switcher and compact user menu. It should collapse appropriately on smaller screens.

The dashboard may retain the landing page's atmospheric background, but at much lower contrast. There should be no scroll-driven storytelling inside normal product workflows.

### 7.2 Deploy page

The Deploy page is the shortest path from existing application code to a healthy, shareable URL.

Required areas:

- Name the application (creates it through the API)
- Choose a source: archive upload, `sprout deploy` from the CLI, or a connected repository
- A plain-language summary of the supported application contract (for example, Node.js 24 with an npm lockfile)
- Start a deployment and follow it through Queued, Building, Isolated, Health Check, and Live
- Clear failure states that name the failed stage and the next action
- Recent deployments across the workspace

Sources that are not implemented yet must be visibly marked as unavailable, never faked. Do not show raw build output by default; show stage results and errors that help the user act.

### 7.3 Apps index

Each application card should initially show only:

- Name
- Status
- URL
- Latest deployment time
- Preview or restrained identifying visual
- Attention state when action is required

Do not show CPU, memory, region, Git hash, request counts, and framework badges by default.

Required collection states:

- No applications yet
- A small collection
- Enough applications to activate search and filtering
- Application building
- Application failed
- Application paused or unavailable

### 7.4 Application workspace

Keep the global sidebar. Add app-level navigation in the page header:

```text
Overview | Deployments | Access | Settings
```

The header must contain:

- Back link to Apps
- Application name
- Current status
- Public URL
- Open application action

#### Overview

Answer these questions immediately:

1. Is the application running?
2. Where can the user open it?
3. What was most recently deployed?
4. Does anything need attention?

Show the latest deployment and attached resources without turning the page into a metrics wall.

#### Deployments

- Chronological deployment history
- Status, branch, commit identifier, initiator, and timestamp
- Clear current deployment
- Deployment detail progression: Queued, Building, Starting, Health Check, Live
- Build logs inside the selected deployment
- Mock rollback action with a confirmation flow

#### Access

- Member list
- Owner, Editor, and Viewer roles
- Invite flow
- Remove-access flow
- Workspace-wide access toggle
- Clear explanation of inherited workspace identity

#### Settings

- Application name
- Repository connection
- Custom domain
- Runtime configuration
- Pause and resume application
- Archive application
- Delete application permanently

Lifecycle actions must explain their runtime and access effects. Destructive actions must be visually separated and require explicit confirmation.

## 8. Shared component system

Build components from reusable primitives rather than designing each page independently.

### Foundations

- Color tokens
- Typography scale
- Spacing scale
- Radius and shadow tokens
- Motion duration and easing tokens
- Focus styles
- Responsive breakpoints

### Core primitives

- Button
- Icon button
- Link
- Input
- Textarea
- Select/combobox
- Checkbox and switch
- Dialog and alert dialog
- Popover and tooltip
- Tabs
- Badge
- Avatar
- Skeleton
- Toast
- Dropdown menu

### Product components

- Workspace sidebar
- Workspace switcher
- Page header
- Application card
- Application status indicator
- Deployment timeline
- Deployment row
- Log viewer
- Environment-variable row
- Resource connection card
- Member/permission row
- Deploy source picker
- Deployment stage item
- Empty state
- Attention banner
- Browser preview frame
- Terminal surface

Status colors and wording must be consistent across cards, headers, timelines, logs, and notifications.

## 9. Frontend data model

Define stable frontend types early:

```text
User
Workspace
WorkspaceMember
Application
ApplicationAccess
Deployment
DeploymentStage
LogEntry
EnvironmentVariable
ManagedResource
ActivityEvent
```

TanStack Query keys should follow a predictable hierarchy:

```text
['workspaces']
['workspace', workspaceSlug]
['workspace', workspaceSlug, 'apps', filters]
['workspace', workspaceSlug, 'app', appId]
['workspace', workspaceSlug, 'app', appId, 'deployments']
['workspace', workspaceSlug, 'app', appId, 'deployment', deploymentId]
['workspace', workspaceSlug, 'app', appId, 'logs', filters]
['workspace', workspaceSlug, 'app', appId, 'environment']
['workspace', workspaceSlug, 'app', appId, 'access']
```

Components should not import mock fixtures directly. They should call typed query and mutation hooks backed by an API interface.

## 10. Proposed source organization

```text
src/
  app/
    (marketing)/
    (auth)/
    workspace/[workspaceSlug]/
  components/
    ui/
    marketing/
    dashboard/
    apps/
    deployments/
    logs/
    deploy/
  features/
    auth/
    workspaces/
    apps/
    deployments/
    environment/
    access/
    deploy/
  lib/
    api/
    query/
    validation/
    motion/
    utils/
  mocks/
    fixtures/
    handlers/
  styles/
  types/
```

Keep route files thin. Domain-specific query hooks, mutations, schemas, and view components should live in feature modules.

## 11. Implementation phases

### Phase 0: Project foundation

- Initialize Next.js, TypeScript, Tailwind, linting, and formatting
- Configure fonts and global styles
- Add TanStack Query provider and development tools
- Establish directory structure and import aliases
- Configure Vitest, Testing Library, and Playwright
- Create initial design tokens

Deliverable: a clean application shell with automated checks passing.

### Phase 1: Visual system and product primitives

- Build foundational UI components
- Define light/dark decision and final color system
- Build status, terminal, browser-frame, and application-node primitives
- Create representative component states in an internal showcase route or Storybook only if it improves development speed

Deliverable: a coherent reusable component language for both marketing and dashboard surfaces.

### Phase 2: Landing-page static composition

- Build semantic story chapters
- Build the persistent visual stage
- Compose all seven scenes without scroll animation
- Complete desktop and simplified mobile compositions

Deliverable: the entire landing story is understandable before animation is introduced.

### Phase 3: Landing-page motion

- Add normalized scroll-progress tracking
- Implement explicit scene states and transitions
- Tune transforms, opacity, focus, and camera-like depth
- Add reduced-motion and non-JavaScript fallbacks
- Profile loading, animation smoothness, and layout stability

Deliverable: a performant, accessible scroll-driven landing experience.

### Phase 4: Dashboard shell and mocked API

- Build sign-in entry state
- Build workspace shell, sidebar, header, responsive navigation, and workspace switcher
- Define frontend domain types
- Create typed mock API and TanStack Query hooks
- Seed realistic data, delays, and deterministic errors

Deliverable: navigable authenticated shell backed by replaceable mock services.

### Phase 5: Deploy and Apps experiences

- Build the Deploy entry flow
- Build Apps empty state, app grid/list, search, and filters
- Connect mock create-app and deployment-progress mutations
- Ensure direct URLs and browser navigation restore the correct state

Deliverable: users can create an application, start a deployment, and find it in Apps.

### Phase 6: Application workspace

- Build application header and nested navigation
- Implement Overview
- Implement deployment list and deployment detail/build logs
- Implement Access
- Implement Settings and confirmations

Deliverable: a complete frontend representation of an application's lifecycle and configuration.

### Phase 7: Quality and product polish

- Complete keyboard and screen-reader review
- Complete reduced-motion review
- Verify responsive layouts
- Add loading, empty, failure, retry, and offline states
- Test critical journeys with Playwright
- Measure and improve Core Web Vitals
- Remove nonessential controls and visual clutter

Deliverable: a portfolio-quality frontend prototype ready to connect to the future backend.

## 12. Critical user journeys

The frontend phase is complete when these mocked journeys work coherently:

### Journey A: Understand Sprout

1. Visitor opens the landing page.
2. Scrolling explains code, deployment, infrastructure, and sharing.
3. Visitor reaches a clear call to action.

### Journey B: Deploy an application

1. User opens Deploy in the workspace.
2. User names an invoice approval application and chooses its source.
3. The deployment progresses through build, isolation, and health-check stages.
4. User receives the application's URL, or a clear failure with its stage.
5. The new application appears in Apps.

### Journey C: Investigate a failed deployment

1. User sees an application requiring attention.
2. User opens its failed deployment.
3. User identifies the failed stage and relevant build log.
4. User can retry the mocked deployment.
5. The status updates consistently across the app.

### Journey D: Bring in the team

1. User invites a workspace member.
2. User assigns a role.
3. The Access page reflects who can manage the app.

## 13. Acceptance criteria

### Product clarity

- A new visitor can explain Sprout's value after completing the landing story.
- A new visitor understands that application code is Sprout's input and that a safe, public, running app is its primary value.
- The landing story communicates that untrusted code passes through build, isolation, resource limits, and health checks before receiving a URL.
- A signed-in user can locate Deploy and Apps immediately.
- The product reads as a deployment platform, not a code-generation experience.
- An application owner can determine app health without opening multiple pages.
- An application owner can distinguish running, paused, archived, and failed states and understand the permanent effect of deletion.
- The team that operates an app (roles, invitations) is presented as a first-class workflow rather than a secondary infrastructure setting.
- Infrastructure terminology is introduced contextually and explained where necessary.

### Visual quality

- Landing and dashboard visibly belong to the same design system.
- The landing experience does not resemble a stack of generic marketing sections.
- The dashboard remains calm and usable with realistic data.
- Empty and error states feel intentionally designed.

### Responsiveness

- All critical journeys work at desktop, tablet, and narrow mobile widths.
- The landing narrative remains understandable on devices where the full desktop motion system is inappropriate.
- No essential controls require hover.

### Accessibility

- All interactive elements are keyboard reachable.
- Focus order and focus indicators are clear.
- Dialogs trap and restore focus correctly.
- Text and status indicators meet contrast requirements.
- Status is never communicated by color alone.
- Reduced-motion mode avoids large scroll-linked transforms.

### Performance

- The initial landing experience does not require downloading dashboard code.
- Heavy animation or visual libraries are lazy-loaded where practical.
- Scroll interaction does not trigger avoidable React rerenders on every frame.
- Layout shifts are minimized.
- Long log collections remain responsive.

### Maintainability

- Pages consume typed query/mutation hooks rather than fixtures.
- Route files remain thin.
- Shared status terminology and components are used consistently.
- Replacing the mock API with HTTP does not require redesigning components.

## 14. Decisions to make during implementation

These decisions should be resolved with small prototypes rather than prolonged upfront debate:

- Final typeface pairing
- Final graphite/warm-light base palette
- Whether the landing page uses DOM/SVG only or a small canvas layer
- Motion library selection
- Exact sidebar collapsed behavior
- Apps grid versus user-selectable grid/list presentation
- Whether an internal component showcase is worth maintaining

The default should be the smallest solution that achieves the intended experience and remains accessible.

## 15. Frontend completion boundary

This frontend milestone is complete when the landing page and all dashboard journeys are visually polished, responsive, accessible, and backed by a typed mock API.

At that point, backend implementation can begin against the frontend's documented domain types and API needs. Backend work should replace mock adapters incrementally rather than forcing a second frontend rewrite.
