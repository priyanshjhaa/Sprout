# Sprout Frontend Implementation Plan

## 1. Purpose

This document defines the first implementation phase of Sprout: a polished frontend prototype for an agent-native cloud for small, purpose-built applications.

Sprout's product promise is:

> Take an existing application from code to a secure, shareable URL with almost no infrastructure work, eventually making small software as easy to share as a document.

This phase focuses only on the product experience. It should establish the visual identity, landing-page story, dashboard information architecture, reusable component system, responsive behavior, and realistic mocked application states. It does not include the real deployment platform, runtime orchestration, databases, secrets infrastructure, or agent backend.

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
- Agent page
- Apps index
- Individual application workspace
- Overview, deployments, logs, environment, access, and settings views
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
- A functioning coding agent
- A real REST API or MCP server

Mocked experiences must be visibly credible, but the product must never imply that a mocked deployment is real.

## 5. Application route map

```text
/
  Public scroll-driven landing page

/sign-in
  Authentication entry screen

/workspace/[workspaceSlug]/agent
  Agent conversation and application creation experience

/workspace/[workspaceSlug]/apps
  Application collection

/workspace/[workspaceSlug]/apps/[appId]
  Application overview

/workspace/[workspaceSlug]/apps/[appId]/deployments
  Deployment history

/workspace/[workspaceSlug]/apps/[appId]/deployments/[deploymentId]
  Deployment details and build logs

/workspace/[workspaceSlug]/apps/[appId]/logs
  Runtime logs

/workspace/[workspaceSlug]/apps/[appId]/environment
  Variables and attached resources

/workspace/[workspaceSlug]/apps/[appId]/access
  Members and application permissions

/workspace/[workspaceSlug]/apps/[appId]/settings
  Repository, domain, runtime, and destructive settings

/workspace/[workspaceSlug]/activity
  Workspace-level recent events

/workspace/[workspaceSlug]/team
  Workspace members

/workspace/[workspaceSlug]/settings
  Workspace settings
```

Only Agent and Apps should receive strong emphasis in the global navigation. Activity, Team, and Settings are secondary destinations.

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

#### Scene 5: Managed infrastructure

- Grow Database, Secrets, Storage, and Logs around the running application.
- Keep the application as the visual center rather than making infrastructure the product.

#### Scene 6: Sharing

- Connect workspace members to the application.
- Introduce Owner, Editor, and Viewer roles.
- Communicate document-like sharing.

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

Workspace
  Agent
  Apps
  Activity

Account
  Team
  Settings
```

The sidebar should include the current workspace switcher and compact user menu. It should collapse appropriately on smaller screens.

The dashboard may retain the landing page's atmospheric background, but at much lower contrast. There should be no scroll-driven storytelling inside normal product workflows.

### 7.2 Agent page

The Agent page is the simplest path from an idea to running software.

Required areas:

- Clear prompt composer with suggested starting prompts
- Conversation history
- Compact agent activity timeline
- Expandable technical details
- Application artifact/preview when one exists
- Approval or clarification state
- Success state with live application URL
- Recent agent-created applications

Example compact activity:

```text
Created application
Generated database schema
Built deployment
Health check passed
Application is live
```

Do not expose raw chain-of-thought or overwhelming command output. Show user-relevant actions, decisions, results, and errors.

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
Overview | Deployments | Logs | Environment | Access | Settings
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

#### Logs

- Runtime/build source selector
- Level filter
- Search
- Time-range selector
- Live-stream pause/resume
- Copy/download affordance
- Empty and disconnected states
- Virtualized rendering if the mock dataset is large enough to justify it

#### Environment

Initially combine environment variables and attached resources.

- Mask secret values
- Distinguish user-managed and Sprout-managed values
- Add, edit, and remove variable flows
- Database status
- Object-storage status
- Clear redeployment warning when configuration changes require it

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
- Delete application

Destructive actions must be visually separated and require explicit confirmation.

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
- Agent composer
- Agent activity item
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
AgentConversation
AgentMessage
AgentActivity
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
    agent/
  features/
    auth/
    workspaces/
    apps/
    deployments/
    environment/
    access/
    agent/
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

### Phase 5: Agent and Apps experiences

- Build Agent page states and mocked conversational flow
- Build Apps empty state, app grid/list, search, and filters
- Connect mock create-app and deployment-progress mutations
- Ensure direct URLs and browser navigation restore the correct state

Deliverable: users can simulate creating an application and find it in Apps.

### Phase 6: Application workspace

- Build application header and nested navigation
- Implement Overview
- Implement deployment list and deployment detail/build logs
- Implement runtime log viewer
- Implement Environment
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

### Journey B: Create through the agent

1. User enters the workspace.
2. User describes an invoice approval tool.
3. Agent activity progresses through creation and deployment states.
4. User receives a working-looking application artifact and URL.
5. The new application appears in Apps.

### Journey C: Investigate a failed deployment

1. User sees an application requiring attention.
2. User opens its failed deployment.
3. User identifies the failed stage and relevant build log.
4. User can retry the mocked deployment.
5. The status updates consistently across the app.

### Journey D: Configure and share an application

1. User adds an environment variable.
2. User sees whether redeployment is required.
3. User invites a workspace member.
4. User assigns a role.
5. The Access page reflects the change.

## 13. Acceptance criteria

### Product clarity

- A new visitor can explain Sprout's value after completing the landing story.
- A signed-in user can locate Agent and Apps immediately.
- An application owner can determine app health without opening multiple pages.
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
