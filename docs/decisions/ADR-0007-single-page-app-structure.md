# ADR-0007: Single-page app structure

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-09 |
| **Authored by** | `INIT-005/SPEC-001` |
| **Initiative** | INIT-005 · learner-experience-and-identity |
| **Research grounding** | None for the navigation model itself. Route-change accessibility cites WCAG 2.2 via `RSCH-006` / ADR-0006 (title, focus, reduced motion). Learning-outcome evidence for SPA vs multi-page: none found; this is an engineering and feel decision. |
| **Supersedes** | [ADR-0005](ADR-0005-stack-and-architecture.md) in part — the page model only (one HTML document per signed-in screen, each page loading its own script list and remounting the shell). Option A (Node, no framework, no build step, `shared/ui/` functions, tokens in `shared/worksheet.css`) stays. |
| **Superseded by** | — |

## Summary

Signed-in screens share **one shell document**. A hand-built History API router swaps only the main content. The header and nav are painted once from a server-embedded `/api/me`-shaped user object, so a top-nav click does not reload the document and does not flash an empty nav. Login stays a separate full page. No new npm dependency and no build step.

Legacy worksheet lessons may run inside the shell or as full pages with the same header. That choice is confirmed at the INIT-005/SPEC-002 spike and recorded there; this ADR names the rule and the fallback.

---

## Context

### Problem statement

Clicking a top-nav link feels like a full reload. The operator described it as missing a master page and building one-offs instead of a shared layout. Checked in the tree on 2026-10-09:

| Fact | Evidence |
|------|----------|
| Separate HTML per screen | `index.html`, `lessons/index.html`, `review/index.html`, `progress/index.html`, `admin/index.html` (41–54 lines each), each with its own `<head>`, heading block, and script list |
| Nav remounts every time | Each page has `<div id="app-shell"></div>`; `HebrewShell.mount` paints a `…` placeholder, then calls `Api.me()` (`shared/app-shell.js`) |
| Static revalidation | `server/web.js` `sendFile` sets `Cache-Control: no-cache` |
| ADR gap | ADR-0005 scored stacks and chose Option A. It did not decide how pages share a layout |

The operator answered: use a single-page app with a client-side router, and do that work **before** the restyle.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | One shell document for every signed-in screen. Header and nav do not repaint on in-app navigation. |
| R2 | Hand-built History API router. View contract: `mount(container, ctx)` and `unmount()`. No new dependency, no build step. |
| R3 | Signed-in user known on first paint (no `…` flash). |
| R4 | On route change: document title, focus to main heading, scroll reset (back/forward restores), polite live-region announce, short transition that honours `prefers-reduced-motion`. |
| R5 | Refresh and direct URLs load the right screen. Unknown routes show a not-found view inside the shell. |
| R6 | A 401 sends the learner to `/login/` with a safe `next` (`HebrewApi.safeNextPath`). |
| R7 | Open in new tab, middle click, and copy link keep working (real `href`s). |
| R8 | Login remains a separate full page. |
| R9 | Legacy lesson rule and SPEC-002 fallback are named. |
| R10 | What ADR-0005 keeps and what it loses is explicit. |

### Constraints

- Plan only in this ADR. Application code is INIT-005/SPEC-002 onward.
- Option A stays: Node 22, `node:sqlite`, in-process MCP, hand-built `shared/ui/`, tokens in `shared/worksheet.css`, no front-end framework, no build step.
- No secrets, learner names, or transcript text in this document.
- Existing INIT-002 gating, session, CSRF, and visibility rules stay.

---

## Options considered

| Option | Shape | Why considered |
|--------|-------|----------------|
| A — Shared server layout, multi-page | One HTML template for chrome; each route still a full document load; optional View Transitions API | Fixes duplication and can soften the flash; still a document navigation |
| B — Hand-built SPA router | One document; History API; view mount/unmount; no library | Matches the operator's "header never reloads" answer and ADR-0005's no-dependency rule |
| C — SPA with a router library | Same as B plus a small client router package | Less hand code; opens a `dependency_add` gate and fights Option A's "no new direct dependency" |

---

## Decision

### 1. Choice: Option B

The operator chose a single-page app. Option B is that choice under ADR-0005 Option A. Option C is rejected unless a later ADR reopens dependencies. Option A is rejected because it still performs a document navigation on every top-nav click.

**Status:** Accepted — operator approved INIT-005 on 2026-10-09 ("Approve"), including the SPA approach and "do structure first."

### 2. Shell document

- One HTML entry serves every signed-in app path: `/`, `/lessons/`, `/review/`, `/progress/`, `/admin/`, and worksheet paths that the SPEC-002 decision keeps inside the shell.
- The document contains: the shared stylesheet and script list once; a persistent header/nav region; a single main mount point (for example `#app-main`); a polite live region for route announcements.
- Unauthenticated requests to those paths still redirect to `/login/` with a safe `next`, as today.
- `/login/` stays its own full page outside the shell.

### 3. First-paint user

- The server embeds a minimal user object into the shell HTML, shaped exactly like `GET /api/me`: `id`, `username`, `display_name`, `role`, `must_change_password` (see `server/api/auth-routes.js` public shape).
- Values are HTML-escaped (or emitted as a JSON script with the same escape rules the site already uses for data scripts). No password hash, no session token, and no fields beyond that shape.
- `HebrewShell.mount` paints from the embedded object on first load and does not re-fetch `/api/me`, because the embed comes from the same response. It must not clear the nav to a `…` placeholder, and it must not re-render the shell on navigation.
- Cookies remain the session credential. The embed is convenience for first paint, not a second auth mechanism.

### 4. Router and view contract

| Piece | Rule |
|-------|------|
| Module | `shared/router.js` (name may match the implementation; this record freezes the behaviour) |
| API | History API: `pushState` / `replaceState` / `popstate` |
| Clicks | Same-origin links that match the route table are intercepted on left-click without modifier keys. Real `href` attributes stay so open-in-new-tab, middle click, and copy link work |
| Route table | Maps pathname patterns to view modules |
| View contract | `mount(container, ctx)` and `unmount()`. `unmount` removes listeners, timers, and any globals the view set |
| Active nav | Update `aria-current="page"` on the matching shell link without re-rendering the whole shell |
| Unknown route | Not-found view inside the shell; HTTP may still be 200 for the shell document (the client shows not-found). Direct server 404 for truly unknown static files stays as today |

`ctx` carries at least: the matched path, any route params, the current user object, and a navigate helper.

### 5. Accessibility and motion on route change

On every successful in-app navigation:

1. Set `document.title` for the new screen.
2. Move focus to the new main heading (or the main region if no heading exists yet).
3. Reset scroll to top on push navigations; restore scroll on `popstate` when a saved position exists.
4. Announce the new screen name through a polite `aria-live` region.
5. Optional short transition; under `prefers-reduced-motion: reduce`, skip non-essential motion (ADR-0006 rule 18 / WCAG 2.2.2 and 2.3.3 product choices already taken).

### 6. Auth failures

- Any `/api/` response that is 401 during a mounted view sends the browser to `/login/?next=<safe path>`.
- Path safety uses the existing `HebrewApi.safeNextPath` rules (same-origin path only; reject `//`, `\`, and non-path values).
- Forced password change continues to use the existing shell dialog; it is not a separate route.

### 7. Legacy lessons

- The three published worksheets load data via `window.WORKSHEET` script tags today. Two lessons in one document would collide if globals are not reset.
- **Rule:** INIT-005/SPEC-002 mounts `my-morning` inside the shell and records go or fallback in `INIT-005-shell-spike-decision.md`.
- **Fallback (pre-approved):** if that spike cannot unmount cleanly, legacy lessons open as full documents that still show the same header chrome (or a thin full-page shell), until INIT-004 replaces them with API-served plans. The SPA continues for Today, Lessons, Review, Progress, and Admin.
- New plans served by `GET /api/plans/:id/published` (ADR-0003 / ADR-0005) do not use `window.WORKSHEET` and are first-class views.

### 8. Caching

- The shell document for signed-in app routes is `Cache-Control: no-store` (it embeds the user).
- Fingerprinted or versioned static assets (CSS, JS, fonts, icons) may use a longer cache than today's blanket `no-cache`, as long as a content change busts the URL or the validator. Exact headers are an implementation detail of SPEC-002/004; this ADR requires only that a nav click does not depend on re-downloading the shell HTML.

### 9. What ADR-0005 keeps

| Kept | Unchanged |
|------|-----------|
| Option A stack | Node 22, `node:sqlite`, in-process MCP |
| No front-end framework | No Preact, React, Vite, Phoenix, Next |
| No new direct dependency for this decision | Hand-built router |
| `shared/ui/` contract | `(props) => string`, escaped text, `data-*` hooks |
| Tokens in `shared/worksheet.css` | Custom properties; dark mode; reduced motion; logical properties |
| Plan load path | `GET /api/plans/:id/published` → render with `shared/ui/` |
| Gating and sessions | INIT-002 / ADR-0002 rules |

| Superseded (page model only) | Replacement |
|------------------------------|-------------|
| One HTML file per signed-in screen, each remounting the shell | One shell document + router + views |
| Implied full document navigation between Today / Lessons / Review / Progress / Admin | In-app navigation with the behaviours in Decision 5 |

INIT-004 SPEC-013 through SPEC-016 should treat learner and builder screens as **views** registered with this router, not as new top-level HTML files. Those specs are not edited by this ADR; they are re-read before execution.

### 10. Consequences

**Positive.** Top-nav clicks no longer feel like a reload. Header identity (logo, account) can land once. Restyle and feedback work sit on one layout.

**Negative / costs.** About 9.4k lines of page scripts must become mount/unmount views. Accessibility behaviour the browser did on full loads becomes application code and tests. Legacy `window.WORKSHEET` globals need a reset protocol or the fallback. Chrome smokes that assume full page loads keep working (real URLs) but may need waits adjusted if the shell is already present.

**Neutral.** Login, MCP, SQLite, audio paths, and visibility rules are untouched by this record.

---

## Implementation plan (by INIT-005 specs)

| Spec | What it does |
|------|----------------|
| SPEC-001 | This ADR and the ADR-0005 "Superseded in part" note |
| SPEC-002 | Shell, router, Today + Lessons, one legacy lesson trial, operator gate |
| SPEC-003 | Review + Progress; title/focus/scroll/announce |
| SPEC-004 | Admin; legacy fate; 401 and direct-URL tests |
| SPEC-007+ | Restyle and identity land on this structure |

---

## References

- INIT-005 initiative document, operator answers 2026-10-09 (SPA; structure first).
- ADR-0005 Option A and Decision 6 (components, tokens, hooks).
- ADR-0006 rule 18 (motion / reduced motion).
- `RSCH-006` WCAG rows cited by ADR-0006.
- `shared/app-shell.js` `mount`; `server/api/auth-routes.js` `/api/me` shape; `server/web.js` `sendFile` cache header.

## Changelog

| Date | Change |
|------|--------|
| 2026-10-09 | Initial Accepted record — `INIT-005/SPEC-001`. Operator approved INIT-005 the same day. |
| 2026-10-09 | Implemented for all five screens (SPEC-002–004 together, after the operator rejected the Today/Lessons-only spike). Decision 7 resolved as **fallback**: legacy lessons stay full pages. Decision 3: the shell paints from the embed only, with no background `/api/me` call (that call repainted the nav with a stale path). Decision 8: static files use weak ETag + 304. Added `ctx.signal`, `ctx.actions` and `ctx.ready`, `canLeave()` on Back as well as on link clicks, and `#app-main[data-route-ready]` as the ready signal for tests. See `INIT-005-shell-spike-decision.md`. |
