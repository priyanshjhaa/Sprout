# Redesign plan

The redesign takes its layout language from cofounder.co: a calm off-white page, regular-weight geometric type, small mono labels, translucent cards, dark tactile buttons, and sections that reveal themselves as they enter the viewport. Sprout keeps its own content, its painted landscape (`landscape-full.webp`) as the hero background, and its green. We copy layout and styling only, never their fonts, artwork, logos, or copy.

The work happens in three passes: landing, then sign-in, then the dashboard. Each stage below is one commit and leaves the app runnable.

## Visual system

| Token | Value | Use |
|---|---|---|
| Surface | `#f5f5f2` | Page background, sticky nav |
| Ink | `#171717` | Headings and primary text |
| Ink muted | `rgb(38 35 35 / 0.8)` and `/ 0.5` | Body and labels |
| Glass card | `rgb(255 255 255 / 0.6)`, 1px `rgb(0 0 0 / 0.1)` border, 10–12px radius | Lists, mock product surfaces |
| Dark button | `#4f4f4f → #202020` gradient, inset highlight, 8px radius, 44px tall | Primary action on light surfaces |
| Light button | White, 8px radius | Primary action on the hero |
| Band | Deep Sprout green | The one full-colour section |
| Sans | Geist (free stand-in for a geometric grotesk), weight 400 for headings | Everything |
| Mono | IBM Plex Mono, 9–13px | Chapter numbers, eyebrows, status chips |

## Motion

Motion explains what Sprout does; it is never decoration for its own sake. Every animation is CSS or a small `IntersectionObserver`, has a reduced-motion fallback, and leaves the content readable without JavaScript.

- Sections and cards rise and fade in once as they enter the viewport.
- The hero dissolves into the page through a pixel-step mask, and its headline settles in word by word.
- The nav turns from glass to solid surface after the hero.
- Product mocks play their own small story when visible: a build moves through its stages, a health check turns green, an idle app dims to sleep and wakes on a visit.
- The letter grid highlights hidden app types one after another.

## Landing stages

0. Docs: this plan, and the landing section of `frontend-implementation.md` without scrollytelling.
1. Foundation: landing tokens, fonts, buttons, glass card, mono label, and the reveal-on-scroll primitive, scoped to the landing so the dashboard is unaffected.
2. Nav and hero: landscape background, pixel-dissolve edge, sticky nav. The scrollytelling landing and `growth-system.tsx` are removed.
3. Statement: facts strip, product statement, animated Deploy mock, three feature columns, and a call to action.
4. Chapters: 1.0 Deploy, 2.0 Operate, 3.0 Share, each with four numbered items and an animated mock.
5. Four ways in: Dashboard, CLI, MCP, and coding agent as book-like covers.
6. Band: the green section, "Everything a small app needs".
7. Letter grid and footer.
8. Polish: responsive layouts, reduced motion, keyboard focus, removal of the old story styles, lint, typecheck, build, and browser verification.

## Sign-in stages

Built around Cofounder's login: a landscape banner with a glass brand badge and a typed mono tagline, a Sign In / Sign Up pill switch, stacked dark pill buttons for Google and GitHub, an "or" rule, the email field, and a quiet serif sign-off. Clerk still owns authentication; we restyle its components rather than rebuild the flow.

1. Shared landscape scene: the hero's sky, soil, grass, and pollen become one component the auth banner reuses.
2. Auth shell: banner, badge, typed tagline, mode switch with a sliding thumb, and sign-off, used by sign-in, sign-up, and invitation acceptance.
3. Clerk restyle: dark pill social and primary buttons, the divider, inputs, and hidden duplicate header and footer links.
4. Polish: staggered entrance, button shine, reduced motion, mobile, removal of the old auth landscape styles, and browser verification.

## Dashboard stages

The workspace follows a calm sidebar layout (inspired by the Hearth dashboard concept): a left sidebar with the workspace, search, main navigation, recent apps, settings, and the signed-in user; a slim top bar with a back chip and the Deploy action; a hero card with Sprout's landscape fading in from the right; filter tabs; app cards; and a lifecycle list. Analytics charts and quick stats are left out, as are a theme toggle and notifications, which Sprout does not have. Fonts, colours, and motion follow the landing and sign-in pages.

0. Docs: this section.
1. Shell: sidebar and top bar replace the old top bar for every workspace page.
2. Overview hero: the landscape banner card with its entrance motion.
3. Overview body: filter tabs, app cards with plants, the lifecycle list, and loading, empty, and error states.
4. Apps and app detail: the apps index and an app's pages on the new cards, tabs, and buttons.
5. Deploy, Team, Tokens, and Settings in the same style.
6. Polish: the mobile drawer, reduced motion, removal of the old dashboard styles, build, and verification.
