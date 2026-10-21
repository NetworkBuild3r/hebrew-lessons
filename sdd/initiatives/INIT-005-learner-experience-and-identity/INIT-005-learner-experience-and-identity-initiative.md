# Initiative: Learner experience and identity

**Initiative ID:** INIT-005
**Status:** executed 2026-10-12. SPEC-001, 003 to 006 and 008 to 013 complete; SPEC-007 superseded (look already chosen by INIT-007 / ADR-0010); SPEC-002 stays `in_review` until the operator clicks through the shell and says go (ac-5). Mode E (completion report) not yet run.
**Slug:** INIT-005-learner-experience-and-identity
**Created:** 2026-10-09
**Updated:** 2026-10-09 (added the app-structure work after the navigation complaint)
**Owner:** Brian Nelson
**Branch:** not created. Planned on `lesson-system-foundations`.
**Source:** operator, 2026-10-09: "The UI really sucks. so plan. Make it feel funnier." and "When I click on a button on the top it feels like the page is reloading... we don't have a master page... we are creating one offs."
**Relates to:** ADR-0005 (stack), ADR-0006 (visual language), RSCH-006 (interface rules), INIT-004 SPEC-013 to SPEC-016 (tokens, renderers, builder).

---

## 1. What the operator asked for, and what was decided

### Part A: feel and identity

| Question | Answer | What it means for this plan |
|----------|--------|-----------------------------|
| What does "fun" mean? | Satisfying interactions; game mechanics; plus "Icons, logos, etc." | Motion, sound and tactile feedback; real identity (logo, icon set, illustration style). |
| Points and streaks? | **None** | No points, streaks, levels or leaderboards. This agrees with ADR-0006 and RSCH-006 rule 14. |
| Humour? | "Skip jokes" | No jokes, no mascot with lines, no funny error copy. |
| Which screens? | All of it | The live site and the new design. |

**One conflict, resolved by assumption (A1).** "Game mechanics" was ticked, and "none" was chosen in the next question. I took the more specific answer. The plan keeps a "words you can now use" count and a finished-lesson moment (RSCH-006 rule 14: show what the learner can now do). That is not a score. Say so if you want it removed, or want real points.

### Part B: page structure and navigation

| Question | Answer | What it means |
|----------|--------|---------------|
| How to fix the reload feel? | **One app with a client-side router.** The header stays; only the content swaps. | New spec group SPEC-001 to SPEC-004. |
| When? | **First**, before the restyle. | The restyle is built on the new structure. |

---

## 2. What is wrong today (observed in the code and screenshots)

### Structure (why a click feels like a reload)

The operator's reading is right. Checked in the repo:

1. **Every nav click is a full document load.** `index.html`, `lessons/`, `review/`, `progress/` and `admin/` are five separate HTML files of 41 to 54 lines each (`login/index.html` is 250 lines). Each one loads about nine scripts and starts from nothing.
2. **The nav is empty on arrival, then pops in.** Each page has `<div id="app-shell"></div>`. `HebrewShell.mount` first paints a `…` placeholder, then calls `Api.me()` and paints the real nav (`shared/app-shell.js`, `mount`). That is a visible flash on every click.
3. **Static files are sent with `Cache-Control: no-cache`** (`server/web.js`, `sendFile`). The browser revalidates every script and the stylesheet on every page load.
4. **There is no page template.** Each page repeats its own `<head>`, its own `topbar page-heading` block, its own script list and its own `mount` call. A change to the header means editing every page. This is what the operator calls "one-offs".
5. **ADR-0005 never decided page structure.** It scored stacks (Node, Preact, Vite, Phoenix) and chose to keep Node with a hand-built design system. It did not say how pages share a layout. That gap is mine, from INIT-003, and this initiative closes it.

### Look and feel

From `spikes/skeleton/shots/` and `spikes/look/shots/`:

1. **No identity.** No logo, no icon set. Every control is text, and many are small grey links.
2. **A grey box where the picture goes.** The first thing a learner sees in an introduction is blank.
3. **Flat and timid.** a-calm-type passes the contrast checks and has no energy. It was the operator's pick and is now rejected, so this initiative supersedes the **look part** of ADR-0006. The accessibility numbers, the 18 interface rules and the test-hook contract stay.
4. **Mixed-language labels.** English words such as "introduction" sit beside Hebrew headings.
5. **Dead interactions.** Nothing happens when you answer right or wrong, or when you finish a lesson.

---

## 3. Decision this plan makes, and what it costs

**One page, a hand-built router, no framework.** The operator chose a single-page app. ADR-0005 Option A said no front-end framework and no build step. A router written by hand (History API, a route table, a mount and unmount contract) keeps both rules. It still **supersedes ADR-0005's page model** (one HTML file per screen, one `<script>` list per page). SPEC-001 records that as ADR-0007.

What a single-page app costs here, so the operator is not surprised later:

- **About 9.4k lines of page scripts are global IIFEs** that assume they own the document (`window.WORKSHEET`, `window.HebrewPlayer`, listeners on `document`). Each must become a view that mounts into a container and cleans up on exit. That is the bulk of the work.
- **The three published lessons load their data by script tag** (`window.WORKSHEET`). Two lessons in one session would collide. The router must reset those globals on exit. INIT-004 removes the problem for new plans (ADR-0003: the API serves them). SPEC-002 is a spike that proves whether the legacy lessons can run inside the shell. **If they cannot, legacy lessons open as full pages and only the shell, Today, Lessons, Review, Progress and Admin are single-page. That is the fallback, decided at that spike.**
- **Accessibility work a browser normally does for free.** On every route change: set the document title, move focus to the new main heading, restore or reset scroll, announce the change, keep back and forward correct, and support open-in-new-tab. WCAG 2.2 criteria already in ADR-0006 apply, and SPEC-004's tests cover them.
- **Sign-in stays a separate page.** `login/` is outside the shell on purpose.
- **Auth and security.** The router must treat a 401 as "go to login with a safe `next`", and the existing security headers and public/gated rules in `server/web.js` stay. The server returns the same shell HTML for any app route.

---

## 4. Evidence position (kept honest)

- **Feedback content.** Elaborated feedback on the same turn beats right/wrong only (Moderate, RSCH-006 section 1.3). Satisfying feedback should carry the correct form.
- **Motion.** WCAG 2.2.2 and 2.3.3 constrain it (High for 2.2.2; reduced motion is a product choice already taken in ADR-0006 rule 18). A smooth route change is motion too, and honours reduced motion.
- **Do animation, sound and icons help a second-language learner?** RSCH-006 found nothing direct, and one experiment found that moving animation did not help local comprehension beyond still pictures. **There is no evidence that "satisfying" effects improve learning. They are a product choice,** and SPEC-005 asks the question properly before effects are built.
- **Single-page versus multi-page for learning outcomes.** No research bears on it. This is an engineering and feel decision, not an evidence-backed one.
- **Points and streaks.** Low evidence, not harm. Excluded by the operator's answer.

Rule: every effect either serves a learning purpose (shows the correct form, confirms an audio play, marks where you are) or is labelled a product choice and can be turned off.

---

## 5. Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | One shell document serves every signed-in screen. The header and nav are rendered once and do not repaint on navigation. The active link updates with `aria-current`. | Must |
| REQ-002 | A hand-built router (History API) with a route table and a view contract: `mount(container, ctx)` and `unmount()`. No new dependency, no build step. | Must |
| REQ-003 | The nav shows the signed-in user on first paint: the user is known before the first route renders, with no `…` placeholder and no flash. | Must |
| REQ-004 | Route change: title updates, focus moves to the main heading, scroll resets (back/forward restores), and a polite live region announces the new screen. Links work with open in new tab, middle click and copy link. | Must |
| REQ-005 | Route transitions are short and smooth, and honour `prefers-reduced-motion`. | Must |
| REQ-006 | A refresh or direct URL (for example `/review/`) loads the right screen. Unknown routes show a clear "not found" view inside the shell. | Must |
| REQ-007 | A 401 from any call sends the learner to `/login/` with a safe `next`. The unsafe-`next` rules in `HebrewApi.safeNextPath` stay. | Must |
| REQ-008 | Each existing screen (Today, Lessons, Review, Progress, Admin) becomes a view with no regression in behaviour. Test hooks (`data-*` and classes) stay unless the same change updates the test. | Must |
| REQ-009 | The three legacy lessons run inside the shell, or (fallback chosen at SPEC-002) open as full pages with the same header. The choice is recorded. | Must |
| REQ-010 | A logo, wordmark, favicon, and an SVG icon set (about 20 icons) with a usage guide. Icon-only controls keep an accessible name; primary actions keep a visible word. | Must |
| REQ-011 | An illustration style guide for per-word pictures, so the AI image prompts in INIT-004 SPEC-009 produce one family of pictures. | Must |
| REQ-012 | The new look is chosen from live, tap-through prototypes, not static screenshots. | Must |
| REQ-013 | A feedback kit: tap press-state, correct and wrong feedback that includes the correct form, an audio-playing indicator, a lesson-complete moment. All honour reduced motion. Nothing runs by itself for more than 5 seconds without a way to pause it. | Must |
| REQ-014 | UI sound effects are **off by default**, have one visible mute control, and never overlap Hebrew word audio. | Must |
| REQ-015 | No points, streaks, levels, badges or leaderboards, and no jokes or characters (operator answers). A "words you can now use" count is allowed (A1). | Must |
| REQ-016 | Interface wording is one language per label. English appears only as the gloss. | Should |
| REQ-017 | New tokens replace the look in `ADR-0006-assets/tokens.json`. Contrast stays at 4.5:1 for text and 3:1 for interface parts. Targets stay 24 px minimum and 44 px for primary actions. Dark mode ships. | Must |
| REQ-018 | Fonts, icons and the logo are self-hosted. No external request. | Must |
| REQ-019 | Tests: route change behaviour (title, focus, history, 401), reduced motion per route, screenshots at 360 and 1280 px, and the hook contract. | Must |

---

## 6. Spec breakdown

Each spec is one domain, 2 to 6 hours. Sizes are planning estimates, not promises. Total about **58 h**. Specs are written as files after approval.

### Group 1: structure (first)

| Spec | Domain | Title | Blocked by | Effort | Gate |
|------|--------|-------|-----------|--------|------|
| SPEC-001 | docs | ADR-0007: single-page app structure (hand-built router; supersedes ADR-0005's page model in part) | none | 3h | none |
| SPEC-002 | frontend | Shell and router spike: one entry document, route table, view contract, Today and Lessons ported, one legacy lesson inside it | 001 | 6h | **operator tries it; go or fallback for legacy lessons** |
| SPEC-003 | frontend | Port Review and Progress to views; accessibility behaviours (title, focus, scroll, announce) | 002 | 6h | none |
| SPEC-004 | frontend | Port Admin; legacy lessons per the SPEC-002 decision; route tests; 401 and direct-URL handling | 003 | 6h | none |

### Group 2: identity, look and feel (can start in parallel with Group 1)

| Spec | Domain | Title | Blocked by | Effort | Gate |
|------|--------|-------|-----------|--------|------|
| SPEC-005 | docs | Research RSCH-008: motion, sound, icons and picture style in second-language practice | none | 3h | none |
| SPEC-006 | frontend | Identity: logo, wordmark, favicon, icon set, illustration style guide | none | 5h | **operator picks the logo** |
| SPEC-007 | frontend | Three tap-through prototypes of one lesson step, inside the SPEC-002 shell | 002, 005, 006 | 5h | **operator picks one look** |
| SPEC-008 | docs | ADR-0008: new look, tokens and feedback rules (supersedes ADR-0006 look part) | 007 | 3h | none |
| SPEC-009 | frontend | Feedback kit: motion tokens, route transition, correct/wrong, audio indicator, complete moment, sound toggle | 008, 003 | 4h | none |
| SPEC-010 | frontend | Icon sprite, logo and favicon in the shell and every view | 006, 008 | 3h | none |
| SPEC-011 | frontend | Restyle learner views: Today, Lessons, player, Review, Progress | 009, 010, 004 | 5h | none |
| SPEC-012 | frontend | Restyle login and Admin | 009, 010, 004 | 4h | none |
| SPEC-013 | frontend | Accessibility, screenshot and hook-contract tests for the restyle | 011, 012 | 3h | none |

```
SPEC-001 -- SPEC-002 --[operator tries shell]-- SPEC-003 -- SPEC-004 ------------------+
                |                                                                       |
SPEC-005 -------+                                                                       |
SPEC-006 -------+-- SPEC-007 --[operator picks look]-- SPEC-008 -- SPEC-009 --+--------+-- SPEC-011 --\
   |                                                              |           |                        >-- SPEC-013
   +--[operator picks logo]                                       +-- SPEC-010 +-- SPEC-012 ---------/
```

SPEC-005 and SPEC-006 can run while the structure work is in progress. The restyle (SPEC-011 and SPEC-012) waits for the structure to be done, as the operator asked.

### Spec summaries

**SPEC-001, ADR-0007.** Records one shell document, the hand-built router, the view contract, what the server does for app routes, the 401 rule, and the fallback for legacy lessons. Marks ADR-0005's page model "Superseded in part". Names what does not change: Node, `node:sqlite`, the in-process MCP, no framework, no build step.

**SPEC-002, shell spike.** One HTML entry with the header rendered once, the user known before first paint (embedded by the server from the session, with `/api/me` as a check), a route table, Today and Lessons as views, and one legacy lesson mounted and unmounted cleanly. The operator clicks through it. Output: go, or fallback for legacy lessons.

**SPEC-003 and SPEC-004, ports.** Convert each page script from a global IIFE to a view with `mount` and `unmount`, listeners removed on exit. Then the route behaviours, the 401 path and the direct-URL path are tested.

**SPEC-005, research (RSCH-008).** Four questions, graded with the project's evidence labels: (1) does animated or sound feedback help or distract second-language vocabulary practice; (2) icon plus label versus icon alone for adult novices; (3) does the style of vocabulary pictures change recall; (4) does a "finished" moment affect return to study without points.

**SPEC-006, identity.** Three logo options in SVG, a wordmark in the Hebrew type we ship, favicon set, about 20 icons on one grid, and a one-page illustration guide (palette, line weight, background, and the sentence added to every image-generation prompt).

**SPEC-007, prototypes.** Three working looks of one lesson step: introduction, correct answer, wrong answer with elaboration, lesson complete. Real icons, tap states and motion, dark and light, phone and desktop, each more energetic than a-calm-type within the contrast and target rules.

**SPEC-008 to SPEC-013.** Record the pick, build the feedback kit and icon sprite, restyle every view in `shared/worksheet.css` with no rename of any queried class or `data-*`, fix stray English labels, then test.

---

## 7. Relationship to INIT-004

- INIT-004 SPEC-013 (tokens), SPEC-014 (plan renderers), SPEC-015 and SPEC-016 (builder) assume pages and `shared/ui/` functions. After ADR-0007 they become **views in the router**. They should be re-read against ADR-0007 and ADR-0008 before they start. I have not edited them.
- INIT-004 SPEC-013 should not start before SPEC-008, or it encodes a look the operator has rejected.
- INIT-004 SPEC-009 (image generation) gains the illustration sentence from SPEC-006.
- INIT-004 SPEC-004 serves plans by API, which removes the script-tag globals for new plans (ADR-0003). That makes the legacy-lesson question in SPEC-002 a temporary problem.

---

## 8. Assumptions and open questions

| Id | Item | Status |
|----|------|--------|
| A1 | "Words you can now use" count and a finished-lesson moment are allowed; no points or streaks. | Assumed. Tell me if wrong. |
| A2 | No mascot or character. | Assumed. |
| A3 | Sound effects are a product choice with no learning evidence; off by default. | Assumed. |
| A4 | The router is hand-written, with no library. It is a small amount of code and keeps ADR-0005's "no dependency, no build step". | Assumed. Say so if you would rather add a library. |
| A5 | The server embeds the signed-in user in the shell document so the nav is right on first paint. | Assumed. |
| Q1 | Colours, a name or a logo idea you already like? | Open. SPEC-006 offers three if not. |
| Q2 | If the legacy lessons cannot run inside the shell, is it acceptable that they open as full pages with the same header until INIT-004 replaces them? | Open. Decided at SPEC-002. |

---

## 9. Risks

- **The port breaks things that work.** 9.4k lines of global-style scripts, 44 test files and 10 Chrome smokes depend on them. Mitigation: port one screen at a time, keep the hooks, and run the whole suite at each spec.
- **Router accessibility regressions.** Mitigation: REQ-004 and REQ-019 make the behaviours testable, and SPEC-002 is a spike with an operator gate.
- **Legacy lesson globals collide.** Mitigation: the fallback in section 3.
- **Looks pass the checks but still feel flat.** Mitigation: SPEC-007 uses tap-through prototypes, and the operator chooses before production work.
- **Effects distract from the word.** Mitigation: SPEC-005, rule 18, and every effect can be turned off.
- **Scope.** 58 h here and 88 h in INIT-004. Gates after SPEC-002 and SPEC-007 let the operator stop or change direction.

---

## 10. Approval

- [x] Operator approves INIT-005 (2026-10-09, "Approve"). Q1 and Q2 stay open; defaults apply until answered.
- [x] Spec files created on approval (`specs/SPEC-001` … `SPEC-013`).
- [ ] No commit is made until the operator asks.
