# Codebase Assessment: Hebrew web app → overall design and usability

**Assessment ID:** ASMT-002  ·  **Status:** Complete
**Source:** `c:\Users\BrianNelson\Projects\Hebrew` (working tree on 2026-10-17, run live through the repo's own test harness)
**Assessed:** 2026-10-17  ·  **Analyst:** Cursor agent running `sdd-assess`
**Recommendation:** Refactor  ·  **Confidence:** 0.80
**Pipeline mode:** brownfield (targeted in-place changes)

> **How this assessment is adapted.** `sdd-assess` normally compares an outside codebase with a target platform. Here the question was: *how good is the app's overall design and usability?* The **standard** is the project's interface research (RSCH-006 interface rules, RSCH-007 new-user ease, RSCH-008 motion/sound/icons, RSCH-010 program layouts), WCAG 2.2 AA, and Nielsen's ten usability heuristics. The **source** is the running app, not only its code. "Capability" means how well the design helps a learner do the task (hierarchy, wayfinding, copy, empty states, layout). "Readiness" means the accessibility and robustness mechanics (reflow, contrast, focus, targets, direction, dark mode, what content learners can see). "Refactor" means targeted changes inside the existing code: no redesign from scratch and no new framework.

## Bottom Line Up Front (BLUF)

**Recommendation: Refactor. The accessibility mechanics are strong; the usability problems are in hierarchy, wayfinding, and copy, and each one is local to one or two files.** Measured live on phone (320–390px) and desktop (1024–1440px), in light and dark: nothing scrolls sideways, text contrast fails in only one place, keyboard focus was visible on every control in an eight-Tab sample, and a brand-new learner's Today screen is a model of "one next action". The weaker side shows once the learner has started. On desktop the Today grid steps down diagonally and leaves a large empty block. The phone nav packs seven destinations, two with no icon and one truncated. The lesson you are on drops to the bottom of the lesson list. The lesson player swaps the desktop rail for a top bar. Learners also see internal wording such as "Coverage unknown — estimated known words from the review store", and a false "saves in this browser" note. Fixing the top ten issues is roughly two to three weeks of front-end work, with no architectural change.

## Objectivity & Independent Validation

The same agent that ran ASMT-001 on this repo wrote this report, and design judgments are partly taste. The mitigations:

- **Measured, not just read.** The app was started on a throwaway database through `test/helpers/signed-in-server.js` and driven by headless Chrome through `test/helpers/chrome-cdp.js`. Twenty-eight page states were measured (target sizes, text contrast, overflow, headings, primary buttons, nav, focus) and 28 screenshots plus 9 viewport-only probe shots were saved. Scripts and raw output are in `evidence/`, so anyone can re-run them. Nothing was written outside `sdd/assessments/`.
- **Every finding ties to a number or a rule.** Each problem below cites a measured value, a screenshot, and the implementing lines, and is mapped to a research rule, a WCAG criterion, or a Nielsen heuristic. Severity is still judgment; that is called out.
- **Artifacts were caught and dropped.** The first full-page captures appeared to show desktop cards clipped at the right edge. A viewport-only probe with DOM geometry showed every card inside the viewport at 1024, 1280, and 1440px, so that finding was removed (see Evidence & Falsification).
- **Falsification pass:** five ways the headline could be wrong were tested. F1 and F3 lowered a severity, F3 and F4 narrowed a finding's scope, F5 lowered overall confidence, and F2 ruled out a capture artifact.
- **Independent LLM-judge eval:** a fresh read-only sub-agent that did not write this report scored it against the human-authored oracle. The verdict is recorded in `eval/`.
- **Limits:** one evaluator, no real learners, no screen reader run (NVDA or VoiceOver), and contrast was computed from CSS colours (text over background images was skipped). A five-learner phone test is in the spec skeleton for that reason.

## Executive Summary

**We recommend one of the following paths:**

1. **No action.** Accessibility is already at or near WCAG 2.2 AA.
2. **Targeted usability refactor.** Fix about ten specific hierarchy, wayfinding, copy, and empty-state problems in place.
3. **Information-architecture redesign.** Rebuild the navigation model and Today around fewer destinations before fixing details.
4. **Visual redesign from scratch.** New shell, new card system.

**Based on these findings, the recommendation is Option 2,** with the navigation consolidation from Option 3 included as one spec rather than a separate project.

- **Where the app is today:** readiness scores about **4.0 out of 5** and capability about **2.7**.
  - **Readiness strengths, all measured live:** no sideways scrolling on any of the 7 learner routes at 320, 360, 390, 1024, 1280, or 1440px (WCAG 1.4.10). Zero text-contrast failures across 28 page states in light and dark, apart from one heading pair. A 3px focus outline on every control reached in an eight-Tab sample of desktop Today. One `h1` per app route, `lang="he" dir="rtl"`, every image with `alt`, no console errors.
  - **Capability strengths:** the brand-new learner's Today screen shows one card with one primary button ("Start your first lesson") plus a short explanation. The lesson player shows "Step 1 of 8" with a progress bar and a thumb-reach Continue button. The Verbs empty state does all three jobs RSCH-010 asks of an empty state.
- **What it is missing, in priority order:**
  1. **The desktop Today grid staircases.** The cards render in the order Mix, Review, Continue. When the Mix card is empty (it renders nothing, `today-page.js:546`), Review takes row 1 and the primary column starts one row lower (`worksheet.css:6979–6995`, `today-page.js:978–982`).
  2. **Returning learners see "Nothing due" first.** That card has no action, and two equal primary buttons follow it. (Two primaries is a product choice under RSCH-006 rule 16, graded Low; see F1.)
  3. **Seven nav destinations on the phone dock.** Two (Guides, Verbs) have no icon. "Progress" is truncated at 360 and 390px, and "Lessons" at 360px. For a new learner, two of the seven open empty pages.
  4. **The lesson in progress sorts below every new lesson** (status rank 2 against 1, `player/utils.js:355–357`, `lessons-page.js:223–228`).
  5. **The lesson player swaps the 260px desktop rail for a 111px top bar,** so the navigation jumps when a lesson opens.
  6. **On phone, chrome above the lesson title takes 210px of 844** (25%). Inside a step, the step heading starts at 265px (31%) and the fixed Continue bar starts at 784px (`evidence/probe2.json` `phoneStep`).
  7. **Internal or untrue copy reaches learners:** "Coverage unknown — estimated known words from the review store", "Answers save automatically in this browser" (progress actually goes to the server for every signed-in learner), "Practice again" on a lesson never started, and a 3,126px "What your AI sees" panel of "unseen" rows for a new learner.
  8. **Empty states that mislead or stall:** Review tells a learner who has learned nothing that they are "All caught up!", and Words opens with nine filter controls before "No words yet".
  9. **Smaller fixes:** the Match column headings fail contrast at 2.56:1 (WCAG 1.4.3); a colon lands on the wrong side ("No next due yet :Next review"); two Progress controls are 18px tall.
  10. **Content governance:** a fresh or rebuilt environment publishes every lesson folder on disk to everyone. That includes a stray lesson titled "test" and a private lesson whose own description says "hidden unless assigned".
- **Capability versus readiness:** the split matters. The app is *accessible* (readiness 4.0) but not yet *easy* once the first lesson is under way (capability 2.7). None of the capability gaps needs a new framework or a new layout system; ADR-0015's shell, tokens, and breakpoints are sound and most fixes are a CSS rule, a sort key, or a string.
- **Harvestable deltas:** places where the app already does better than its research asked, worth copying across the app:
  1. The first-run Today state (one card, one action, one sentence of "what happens next"). Copy it to Review, Words, and Progress.
  2. The Verbs empty state (status, what will appear, a link to Lessons). Copy it to Guides and Words.
  3. Dark mode with zero contrast failures in the two measured dark states (Today on phone and desktop). The other screens were not measured in dark.
  4. The fixed bottom Continue button in the player, which RSCH-006 §1.6 asks for (main action in the lower reachable region).

## Assessment

- **Domain summary:** a Hebrew-learning single-page web app for adult English speakers, Hebrew-first (`dir="rtl"`) with English glosses, behind a login. Learner routes are Today, Lessons, Guides, Verbs, Review, Words, and Progress; admins also get Editor and Admin. Lessons open in a separate worksheet player page.
- **Tech stack (UI):** vanilla JS page modules mounted by a small router (`shared/app-routes.js`), one stylesheet (`shared/worksheet.css`, about 7,700 non-blank lines), and Light/Dark tokens. A 260px rail appears at ≥1024px and a fixed bottom dock at ≤767px (ADR-0015).
- **Method:**
  - **Live capture:** `evidence/capture.js` signs in as a new learner and visits every learner route at 390×844 and 1440×900, in light and dark. It steps four screens into the "My morning" lesson, checks reflow at 320×640, presses Tab eight times on Today, and opens Admin and Editor.
  - **Targeted probes:** `evidence/probe.js` takes viewport-only screenshots and DOM geometry at 1024, 1280, and 1440px. It also checks the dock at 360 and 390px and compares the player shell with the app shell. `evidence/probe2.js` checks overflow on every learner route at 360, 1024, and 1280px and measures the step-heading position inside a lesson on phone.
  - **Code reading:** every finding was confirmed against its implementing lines.
- **Risk areas:**
  - The desktop Today grid placement.
  - The nav model (seven destinations).
  - Player chrome on phone.
  - Learner-facing copy written for developers.
  - The one-time visibility seed (`server/visibility.js:76–102`).

## Persona-Lens Findings

| Lens | Headline finding | Evidence (Tier A) |
| --- | --- | --- |
| Software Architect | ADR-0015's shell, breakpoints, and token system hold up live: no overflow on any learner route at six widths from 320 to 1440px, and no contrast failures in the measured dark states. The one structural seam that leaks is that lessons run in a separate worksheet page with its own top-bar shell, not inside the SPA shell. | `evidence/probe.json` `playerShell` (1425×111 top bar) vs `appShell` (260×900 rail); `shared/app-routes.js:1–17` (screens belong in the route table) |
| Data Architect | Learner-facing copy contradicts the data model. The player says answers save "in this browser", but every signed-in learner's lesson progress goes through `HebrewSync.saveLessonProgress` to the server, and every learner is signed in. | `shared/player/core.js:639`, `:686`, `:807–817`; `server/web.js:411–417` |
| Security & Compliance | A one-time seed publishes every on-disk lesson to everyone on a fresh database. That covers a private lesson whose description says "hidden unless assigned", plus a stray `test` worksheet. Lessons added later default to hidden. | `server/visibility.js:76–102`; `worksheets/8ee73fd9-…/`, `worksheets/anal-sex-vocab/`; screenshot `probe-desktop1440-lessons.png` |
| Platform / Cloud | Responsive tiers behave as specified. The rail is 260px from 1024px up, the dock appears on phone, and nothing overflows at 320px. One soft spot is 1024px, where Today reports `scrollWidth` 1009, which is the 15px scrollbar and not an overflow. | `evidence/ux-metrics.json` `reflow320`; `probe.json` `clip-today-1024`; `probe2.json` `overflow` |
| Day-2 Operations | The capture and probe scripts are reusable regression checks. The repo's own `test/responsive-layout.test.js` checks geometry but not contrast, target size, or Today grid row placement, which is how the staircase shipped. | `test/responsive-layout.test.js` (static CSS and geometry only); `evidence/capture.js` |
| Capability-Parity / Reuse | Most fixes reuse what exists. The first-run Today card and the Verbs empty state are the in-house patterns to copy. The `bi()` helper already wraps English in `dir="ltr"`; the colon bug is one character outside it. The sprite already has icons for five of seven nav items. | `today-page.js:972–990`; `shared/verbs-page.js:47` empty state (screenshot `phone-verbs.png`); `player/utils.js:81–88`; `app-shell.js:36` |
| Agentic / Judgment | Nothing here is judgment-heavy. Every fix is deterministic UI. The "What your AI sees" panel is an AI-transparency feature (INIT-006/SPEC-017), and the finding concerns its presentation only. | `shared/progress-page.js:5`, `:614–640` |

Conflict surfaced, not averaged: the Security lens wants the private lesson hidden by default, while the visibility seed's comment records "publish what was already on disk" as a deliberate migration choice. That is an operator decision; the spec skeleton makes the default safe for new environments and asks the operator to audit production.

## Capability & Readiness Scoring Matrix

The score is the app against the standard (5 means it fully meets it). "Basis" names the rule the row is scored against and that rule's own evidence grade.

| Dimension | Basis (grade) | App | Notes (Tier-A evidence) |
| --- | --- | :---: | --- |
| *Capability: does the design help the learner?* | | | |
| First-run path | RSCH-006 rule 16; RSCH-007 §2 (Low, product choice) | 5 | New learner's Today: one card, one primary "Start your first lesson", plus a Welcome note (`phone-today.png`; `ux-metrics.json` `phone:today` 1 primary). |
| Returning Today hierarchy | RSCH-006 rule 16 (Low); Nielsen 8 | 2 | The "Nothing due" Review card is first and has no action. Two equal primary buttons, "Continue" and "Next new lesson" (`desktop:today` 2 primaries). The desktop grid staircases (`probe-desktop1440-today.png`; `worksheet.css:6979–6995`; `today-page.js:978–982`). |
| Navigation and findability | RSCH-007 §4 icon + label, 3–5 destinations (Moderate) | 2 | Seven learner destinations at 48–52px each. Guides and Verbs have no icon. "Progress" is truncated at 390px (`phone-today.png`, visible "Progre…") and at 360px, where "Lessons" is truncated too (`probe-phone360-today.png`; `dock360` flags only "Progress"). The probe's `truncated` flag misses the visible ellipsis at 390px (`dock390` reports false), so the screenshots are the evidence. Nav definition: `app-shell.js:20–36`. |
| Lesson list orientation | RSCH-010 (WCAG 2.4.8 AAA, 2.4.5 AA) | 2 | The in-progress lesson sorts after every new one (`player/utils.js:355–357`; `lessons-page.js:223–228`; `probe-desktop1440-lessons.png`). No `aria-current`, no "next" marker, no search (ASMT-001 C13). |
| Lesson player focus | RSCH-006 rules 10, 16; §1.6 thumb reach (Moderate/Low) | 3 | Good: "Step N of 8", a progress bar, a fixed bottom Continue, and the Hebrew word at 40px on phone. Weak: the lesson title starts at 210px of 844 (`probe.json` `phoneLessonChrome`), and inside a step the step heading starts at 265px (`probe2.json` `phoneStep`). A "Round 1 · Next round Sunday 9:25 PM" banner appears on step 1 of a first lesson (`phone-lesson-step2.png`). |
| Copy clarity | Nielsen 2; RSCH-007 §2 copy sweep | 2 | "Coverage unknown — Estimated known words from the review store — no data yet." (`desktop-lesson-step3.png`). "Answers save automatically in this browser" is untrue when signed in (`core.js:639`, `:686`, `:810–817`). "Practice again" shows on a never-started lesson (`core.js:676–684`). |
| Empty states | RSCH-010 [20] (Low) | 3 | Verbs: does all three jobs (`phone-verbs.png`). Review: "All caught up!" for a learner with nothing learned, above 0/0/0 chips and two blank selects (`phone-review.png`). Words: nine filter controls before "No words yet" (`phone-words.png`). Guides: one line, no link. Progress: 3,126px of "unseen" rows (`phone-progress.png`). |
| Visual consistency across screens | Nielsen 4; ADR-0015 REQ-001 | 3 | Cards, buttons, and tokens are consistent. The shell changes from rail to top bar inside lessons (`probe.json` `playerShell` against `appShell`). |
| Desktop use of space | ADR-0015 Today Bento 7fr/5fr (product) | 2 | When the Mix card is empty, the staircase leaves the top of the primary column empty beside the Review card (`probe-desktop1440-today.png`; `today-page.js:546`). Lessons is fine (a three-up grid, `probe-desktop1440-lessons.png`). |
| **Capability (average)** | | **≈ 2.7** | The low rows are hierarchy, wayfinding, and copy. None needs a new layout system. |
| *Readiness: accessibility and robustness* | | | |
| Reflow at 320px (WCAG 1.4.10) | WCAG 2.2 AA (High) | 5 | No horizontal overflow on any of the 7 learner routes at 320×640 (`ux-metrics.json` `reflow320`), at 390 and 1440px (`ux-metrics.json` `overflowX`), or at 360, 1024, and 1280px (`probe2.json` `overflow`). |
| Text contrast (WCAG 1.4.3) | WCAG 2.2 AA (High) | 4 | 0 failures across 28 measured states in light and dark, except the Match `.col-title` headings at 2.56:1 (`#94a3b8`; `worksheet.css:504–510`; `match.js:30`). |
| Focus visible (WCAG 2.4.7) | WCAG 2.2 AA (High) | 5 | Every element reached in the 8-Tab sample of desktop Today shows a 3px solid outline (`ux-metrics.json` `focus`). The third Tab landed on `body` (`focus[2]` is null), a possible focus-order gap worth checking. Sample only; see limits. |
| Target size (WCAG 2.5.8; 44px preference) | WCAG AA (High); RSCH-006 44px (Moderate) | 4 | Dock items are 48–52×44, primary buttons are at least 44 tall, and the login form is clean. Below 24px: the Progress "Weak words" and "Very weak words" controls at 89×18 and 126×18. Above 24px but below RSCH-006's 44px preference: the brand link (366×37 on phone, 227×38 on desktop) and "Show pronunciation" (160×30). Inline tappable Hebrew words are 10–50px wide; WCAG's inline exception covers them, but a single-letter prefix at 10×19 is hard to tap. |
| Language and direction (WCAG 3.1.1, 3.1.2) | WCAG AA (High); RSCH-006 | 4 | `lang="he" dir="rtl"` and English wrapped `lang="en" dir="ltr"` through `bi()` (`utils.js:81–88`). One punctuation bug: the colon falls outside the span, so it renders as "No next due yet :Next review" (`today-page.js:235–236`). |
| Headings and structure (WCAG 1.3.1) | WCAG A (High) | 4 | Exactly one `h1` on every app route. Inside lesson steps the page has no `h1` (`ux-metrics.json` `lesson-step2`–`step4` `h1Count` 0). That is not a failure, but screen-reader users lose the lesson title. |
| Dark mode | ADR-0015 token table (product) | 4 | 0 contrast failures in the two measured dark states (`phone:today-dark`, `desktop:today-dark`). Other screens were not measured in dark, so this is not scored 5. |
| Motion (WCAG 2.2.2) | WCAG A (High); RSCH-008 | 4 | Carried from ASMT-001 C19: three infinite pulses remain; `prefers-reduced-motion` is honoured. |
| What learners can see (content governance) | Product / operator | 2 | A fresh database publishes all on-disk lessons to everyone, including `test` and a lesson marked "hidden unless assigned" (`visibility.js:76–102`). Production was not inspected. |
| **Readiness (average)** | | **≈ 4.0** | 36 over 9 rows. |

## Technical Component Analysis

| Source unit | LOC | Nature | Target home | Existing support | Status | Rationale (cite behaviour) |
| --- | ---: | --- | --- | --- | --- | --- |
| `shared/worksheet.css` Today Bento (`:6977–7000`) | 24 | Presentation | Frontend (CSS) | Grid already defined | Adapt | Add `grid-auto-flow: dense` or pin the primary cards to row 1 so the hero is top-of-page |
| `shared/today-page.js` `renderToday` (`:972–990`) | 1,026 | Presentation | Frontend | First-run state already right | Adapt | Show the Review card only when something is due; one primary per screen, with "Next new lesson" secondary |
| `shared/app-shell.js` `NAV` and dock (`:20–36`) | 578 | Presentation | Frontend | Sprite icons for 5 items | Adapt | Five destinations, each with an icon; fold Guides into Lessons and Verbs into Review as tabs; no truncated labels |
| `shared/lessons-page.js` `sortRows` (`:223–228`) and `player/utils.js` status rank (`:355–357`) | 336 + 342 | Deterministic (ordering) | Frontend | Status model exists | Adapt | In-progress first, then new; add `aria-current` and a "Next" marker |
| Worksheet player shell (`/worksheets/<slug>/`) | n/a | Presentation | Frontend | Rail CSS exists for app routes | Adapt | Same rail on desktop (or a documented focus mode); one-row header on phone during a lesson |
| `shared/player/core.js` path and ceremony copy (`:620–690`) | 859 | Presentation (copy) | Frontend | `U.bi` | Adapt | "Saved to your account"; hide "Practice again" until a round is done; hide the Round banner on the first round |
| Story step coverage note | n/a | Presentation (copy) | Frontend | n/a | Adapt | Hide when there is no data; never show "review store" wording to learners |
| `shared/review-page.js` empty state | 1,091 | Presentation | Frontend | Today Welcome card pattern | Adapt | New learner: "Nothing to review yet — finish a lesson and words appear here", with a link; hide filters at zero |
| `shared/words-page.js` layout | 881 | Presentation | Frontend | Progressive disclosure | Adapt | Empty list first, filters collapsed until there are words |
| `shared/progress-page.js` "What your AI sees" (`:614–640`) | 756 | Presentation | Frontend | A toggle pattern exists ("Show raw", `:723`), but it only reveals the raw JSON; the section itself does not collapse | Adapt | Keep the feature; collapse it by default and hide "unseen" rows for new learners |
| `shared/guides-page.js` empty state (`:94–97`) | 144 | Presentation | Frontend | Verbs empty-state pattern | Adapt | Add "what will appear" and a link |
| `.col-title` (`worksheet.css:504–510`) | 7 | Presentation | Frontend (CSS) | Text tokens | Adapt | Darken to ≥ 4.5:1 |
| `today-page.js:235–236` colon | 2 | Presentation | Frontend | `bi()` | Adapt | Move the colon inside each language span |
| `server/visibility.js` `seedVisibility` (`:76–102`) | 203 | Persistence / access control | Server + operator | ADR-0002 default `hidden` for later lessons | Adapt (operator call) | Exclude `test`-like and private lessons from the seed, or seed `hidden`; remove the stray `8ee73fd9-…` folder; audit production |
| `test/responsive-layout.test.js` | n/a | Test | Test | `evidence/capture.js`, `probe.js` | Adapt | Add contrast, target-size, and Today row-placement checks so these regressions are caught |
| ADR-0015 shell, tokens, breakpoints | n/a | Presentation (system) | Keep | n/a | Provided | Reflow, focus (sampled), and contrast pass live, apart from the one Match heading pair; measured dark states have no failures |

## Recommendation

Refactor, confidence 0.80.

### Recommendation Options Key

- **Option 1: No action.** Not recommended. Accessibility is strong, but a returning learner meets a staircased Today, seven nav items, a lost in-progress lesson, and copy that is untrue or written for developers.
- **Option 2: Targeted usability refactor.** **Recommended.** Each problem is local to one or two files, and the in-house patterns to copy already exist (first-run Today, Verbs empty state).
- **Option 3: Information-architecture redesign.** Not recommended as a separate project. Its one essential piece, consolidating to five destinations, is included in Option 2 as SPEC-005. A broader rebuild would risk the parts that measure well.
- **Option 4: Visual redesign from scratch.** Not recommended. The token system, shell, and breakpoints pass nearly every accessibility check measured (one contrast pair fails), and dark mode had no contrast failures in the two measured dark Today states.

**Why:** the scoring matrix splits cleanly. Readiness (≈ 4.0) shows the design system works. Capability (≈ 2.7) shows how screens are composed and worded once a learner is under way. Every capability gap traces to a specific line: `worksheet.css:6979–6995`, `today-page.js:978–982`, `app-shell.js:20–36`, `utils.js:355–357`, `core.js:639`/`:676–686`, `worksheet.css:504–510`, and `today-page.js:235–236`. Capability and readiness are kept separate: high readiness does not mean the app is easy, and the low capability scores do not mean the design system is wrong.

**Ranked improvements: can we do it better?** Ranked by learner impact, then cost.

1. **Fix the desktop Today grid.** One CSS rule. Highest visibility for the lowest cost.
2. **One clear next step on Today for returning learners.** Lead with Continue as the single primary action; show Review only when due; make "Next new lesson" a secondary link.
3. **Five destinations, each with an icon and a full label.** Guides becomes a tab in Lessons and Verbs a tab in Review (or both hide until they have content).
4. **Keep the learner's place.** Put the in-progress lesson first, mark it with `aria-current`, and name the next lesson.
5. **Learner copy sweep.** Fix the untrue "saves in this browser" note, remove "Practice again" before a first round, remove the "review store" wording, and hide the round banner on round one.
6. **Empty states that tell the truth.** For a new learner, Review should say "nothing yet", not "All caught up!". Words should lead with the empty list. Guides needs a link. Progress should collapse "What your AI sees".
7. **One shell everywhere.** Keep the desktop rail in the lesson player and use a one-row header on phone during a lesson. Recover about 100px of phone screen for the item.
8. **Small accessibility fixes:** Match heading contrast, the misplaced colon, the 18px Progress controls, and an `h1` (or a labelled region) inside lesson steps.
9. **Make new environments safe:** a visibility seed that cannot publish private or test lessons, the stray `test` folder removed, and a production visibility audit (operator).
10. **Guard it.** Fold the capture and probe checks into the responsive test suite, then run a five-learner phone test and one screen-reader pass.

**Open unknowns (confidence < 0.9):**

- **What production learners actually see.** The production visibility table was not read; application data is out of scope. *Spike:* the operator opens Admin → lesson visibility on production and confirms `test` and the private lesson are hidden.
- **How real learners rank these problems.** One evaluator, no users. *Spike:* a five-learner, 20-minute phone session, with tasks "start your first lesson", "find where you left off", and "review today's words".
- **Screen-reader experience.** Not run. *Spike:* one VoiceOver (iOS) and one NVDA (Windows) pass over Today, a lesson, and Review.

## Evidence & Falsification

- **Load-bearing claims:** all Tier A (ledger below). Two methods were used: *measured* (a value read from the live app in Chrome, saved in `evidence/ux-metrics.json`, `evidence/probe.json`, or `evidence/probe2.json`, with a screenshot in `screens/`), and *read* (the implementing lines). Every load-bearing claim has both.
- **Artifact caught:** the full-page captures (`screens/desktop-*.png` from `capture.js`) show the fixed dock mid-page on phone and cards apparently cut off at the right edge on desktop. Both are artifacts of Chrome's beyond-viewport capture. DOM geometry put every card inside the viewport (for example, Lessons cards at x 300–1400 in a 1440 viewport), and the viewport-only `probe-*.png` shots confirm it. The clipping finding was dropped; use the `probe-*` shots for layout.
- **Falsification pass:**

**Hypothesis:** the app's design system is sound, and the usability gaps can be fixed in place, screen by screen, without an IA or visual redesign.

| # | How it could be wrong | Tested against (Tier-A evidence) | Survived? |
| --- | --- | --- | :---: |
| F1 | Two primary buttons on Today is fine: RSCH-007 rated Today as conforming with "one primary button each" card. | RSCH-007 line 28 does accept one primary *per card*; RSCH-006 rule 16 says the home screen has one next action but grades it Low (product choice). | Partly. **Severity lowered:** the two-button point is a product choice, not a rule breach. The finding that remains is the "Nothing due" card ranked first and the desktop staircase, both measured. |
| F2 | The desktop staircase is another capture artifact, like the clipping. | Viewport-only `probe-desktop1440-today.png`; CSS `worksheet.css:6979–6995` has no `grid-auto-flow: dense`; DOM order is Mix, Review, Continue (`today-page.js:978–982`). When Mix is empty (`:546`), Review (column 2) takes row 1 and auto-placement moves Continue (column 1) to row 2. | **Yes.** Mechanism and screenshot agree. The staircase appears whenever no Today's-practice section has items (`today-page.js:367–380`, `:546`), as for a learner with nothing due. |
| F3 | The visibility seed is intentional: lessons on disk were public before visibility existed, so nothing is wrong. | `visibility.js:76–79` comment: a one-time seed, with later lessons defaulting to `hidden`. `worksheets/` holds a `test` lesson and a lesson describing itself as "hidden unless assigned". | Partly. **Scope narrowed:** production that was seeded before those folders existed may be fine. The risk is fresh or rebuilt environments plus the stray folder in the repo. It sits ninth of ten in the ranked improvements, not first, and it remains an operator check. |
| F4 | "What your AI sees" is a deliberate transparency feature (INIT-006/SPEC-017), so calling it clutter is wrong. | `progress-page.js:5`, `:614–640`, and the login page disclosure (`phone-signed-out.png`). | Partly. The feature stays; **the finding is limited to presentation** (collapse it by default, hide "unseen" rows for new learners). |
| F5 | One evaluator with no users: the findings could be taste. | Each finding is tied to a measured value or a rule (matrix "Basis" column); 4 of the top 10 are mechanical defects (grid placement, sort rank, contrast, colon) independent of taste. | Partly. Mechanical findings survive. **Severity ordering of the judgment findings (nav, copy, empty states) is lower confidence;** SPEC-009 adds a learner test. Confidence set at 0.80, not 0.85. |

- **Independent evaluation:** the first judge pass rated the report 3/4 (MITIGATE, faithfulness 0.94). Its 11 follow-ups were applied. Two gaps were closed with new measurements, not softer wording: overflow at 360, 1024, and 1280px on every route, and the in-step heading position (`evidence/probe2.js`). The re-score is **PASS**: judge 4/4 (score 1.0), faithfulness 1.0 (16 of 16 claims checked), 12 of 12 acceptance criteria, and property checks 16 passed with 0 failed. Its four wording nits were also applied. See `eval/ASMT-002-hebrew-web-app-eval-report.md`.

**Result:** the recommendation holds. F1 and F3 lowered two severities, F3 and F4 narrowed two findings' scope, and F5 lowered overall confidence.

### Claims-Evidence Ledger

| # | Claim | Type | Tier | Source(s) | Method | Load-bearing? | Status |
| --- | --- | --- | :---: | --- | --- | :---: | --- |
| C1 | No horizontal overflow on 7 learner routes at 320, 360, 390, 1024, 1280, and 1440px | readiness | A | `ux-metrics.json` `reflow320` and `pages.*.overflowX` (390, 1440); `probe2.json` `overflow` (360, 1024, 1280) | measured | yes | verified |
| C2 | Zero text-contrast failures in 28 states, light and dark, except Match `.col-title` at 2.56:1 | readiness | A | `ux-metrics.json` `lowContrast`; `worksheet.css:504–510`; `match.js:30` | measured and read | yes | verified |
| C3 | Focus is visible (3px solid) on every element in the eight-Tab sample of desktop Today; Tab 3 landed on `body` | readiness | A | `ux-metrics.json` `focus` | measured | yes | verified (sample) |
| C4 | First-run Today has one card and one primary action | capability | A | `phone-today.png`; `ux-metrics.json` `phone:today`; `today-page.js:972–990` | measured and read | yes | verified |
| C5 | Desktop Today staircases when the Mix card is empty, because Review renders before Continue without dense packing | capability | A | `probe-desktop1440-today.png`; `worksheet.css:6979–6995`; `today-page.js:546`, `:978–982` | measured and read | yes | verified |
| C6 | Returning Today shows "Nothing due" first and two primary buttons | capability | A | `ux-metrics.json` `desktop:today`; `today-page.js:232–236` | measured and read | yes | verified |
| C7 | Seven learner nav destinations; Guides and Verbs have no icon; "Progress" truncated at 360 and 390px, "Lessons" at 360px | capability | A | `app-shell.js:20–36`; `probe.json` `dock390`/`dock360` `hasIcon`; `phone-today.png` (390px ellipsis; the probe's `truncated` flag misses it); `probe-phone360-today.png` (`dock360` flags only "Progress") | measured and read | yes | verified |
| C8 | The in-progress lesson sorts after new lessons | capability | A | `player/utils.js:355–357` (rank 2) and `:266–269` (rank 1); `lessons-page.js:223–228`; `probe-desktop1440-lessons.png` | read and measured | yes | verified |
| C9 | Lesson player uses a top bar on desktop, while app routes use the rail | capability | A | `probe.json` `playerShell` (1425×111) and `appShell` (260×900) | measured | yes | verified |
| C10 | Phone lesson title starts at 210px of 844; inside a step the heading starts at 265px | capability | A | `probe.json` `phoneLessonChrome`; `probe2.json` `phoneStep` | measured | no | verified |
| C11 | "Saves in this browser" copy is untrue for signed-in learners, and all learners are signed in | capability | A | `core.js:639`, `:686`, `:807–817`; `web.js:411–417` | read | yes | verified |
| C12 | "Practice again" is always rendered next to Start on the path screen | capability | A | `core.js:676–684`; `phone-lesson-step1.png` | read and measured | no | verified |
| C13 | Story step shows "Coverage unknown … review store" to learners | capability | A | `desktop-lesson-step3.png` | measured | no | verified |
| C14 | Review tells a new learner "All caught up!" above zeroed filters | capability | A | `phone-review.png`; `ux-metrics.json` `phone:review` | measured | no | verified |
| C15 | Progress for a new learner is 3,126px tall, mostly "unseen" rows | capability | A | `ux-metrics.json` `phone:progress` `pageHeight`; `phone-progress.png`; `progress-page.js:264`, `:626` | measured and read | no | verified |
| C16 | Colon renders on the wrong side ("No next due yet :Next review") | readiness | A | `today-page.js:235–236`; `probe-desktop1440-today.png` | read and measured | no | verified |
| C17 | Progress controls are 18px tall | readiness | A | `ux-metrics.json` `phone:progress` `under24` | measured | no | verified |
| C18 | A fresh database publishes every on-disk lesson, including `test` and a lesson marked "hidden unless assigned" | readiness | A | `visibility.js:76–102`; `worksheets/8ee73fd9-…/`, `worksheets/anal-sex-vocab/`; `probe-desktop1440-lessons.png` | read and measured | yes | verified |
| C19 | Lesson steps have no `h1` | readiness | A | `ux-metrics.json` `lesson-step2`–`step4` `h1Count` | measured | no | verified |
| C20 | Desktop clipping in full-page shots is a capture artifact | n/a | A | `probe.json` `clip-today-1440`, `clip-lessons-1440` (`offRight` empty) | measured | no | verified (finding dropped) |
| C21 | Production visibility state | readiness | C | not inspected (application data out of scope) | n/a | no | open (spike) |
| C22 | Real-learner ranking and screen-reader experience | capability | C | not tested | n/a | no | open (spike) |

## Spec Skeleton

Grouped by domain in dependency order. One domain per spec. It overlaps ASMT-001's SPEC-010 (Today, nav, lessons list); if both initiatives run, SPEC-004 to SPEC-006 here replace that one.

### Phase 1: Quick fixes (low risk, high visibility)
- SPEC-001 · Server + operator (`security_review: required`): the visibility seed must not publish lessons whose plan marks them private, or must seed `hidden`. Remove the stray `worksheets/8ee73fd9-…` lesson. The operator audits production visibility.
- SPEC-002 · Frontend (CSS): Today grid row placement (`grid-auto-flow: dense` or explicit rows). Match `.col-title` contrast to ≥ 4.5:1. Progress controls to ≥ 24px. Colon inside the language spans.
- SPEC-003 · Frontend (copy): "Saved to your account"; no "Practice again" before a first round; no round banner on round one; no "review store" or "coverage unknown" wording; one language per label (RSCH-007 §2, INIT-005 REQ-016).

### Phase 2: Hierarchy and wayfinding
- SPEC-004 · Frontend: Today leads with one primary action (Continue, else Start next). Review card only when due; "Next new lesson" secondary.
- SPEC-005 · Frontend: five learner destinations, each with icon and full label (Guides as a Lessons tab, Verbs as a Review tab, or hidden while empty). No truncation at 320px.
- SPEC-006 · Frontend: Lessons list puts in-progress first, adds `aria-current` and a "Next" marker, and adds search (RSCH-010, WCAG 2.4.5 and 2.4.8).

### Phase 3: Player, shell, and empty states
- SPEC-007 · Frontend: lesson player keeps the desktop rail (or ADR-0015 records a deliberate focus mode). Phone header collapses to one row during a lesson, with settings moved into the account menu. Each step keeps the lesson title as `h1` or a labelled region.
- SPEC-008 · Frontend: empty states for Review, Words, Guides, and Progress follow the Verbs pattern: status, what will appear, link. "What your AI sees" collapses by default.

### Phase 4: Guard and validate
- SPEC-009 · Test: add contrast, target-size, and Today row-placement checks to `test/responsive-layout.test.js` (reuse `evidence/capture.js` measures). Run a five-learner phone usability test and one VoiceOver and one NVDA pass, then re-score this matrix.

## Risks

| Risk | Probability | Impact | Mitigation |
| --- | --- | --- | --- |
| Folding Guides and Verbs into tabs hides them from current users | M | L | Tabs rather than removal; keep the old URLs working through the router |
| Seed change accidentally hides lessons on an existing production database | L | M | The seed is a one-time no-op once its meta flag exists (`visibility.js:83–86`); test on a fresh and an already-seeded database |
| Copy changes drift from the Hebrew strings | M | L | Change both halves of each `bi()` pair together; a native-speaker read of changed strings |
| Rail inside the player costs width on 1024px laptops | M | L | A collapsible rail at 1024–1279px; full rail from 1280px |
| Fixes reintroduce problems later (the staircase shipped once already) | M | M | SPEC-009 automated checks in the existing test suite |

## Next Step

Proceed to SDD `create-initiative` with the spec skeleton above as the pre-analysed decomposition, setting `source: ASMT-002`. SPEC-001 needs a security review and an operator decision. If ASMT-001's initiative is created too, merge its SPEC-010 into SPEC-004 to SPEC-006 here.
