# SPEC-016 Completion Summary — Verse-reading, letter-intro, and the Guides shelf

**Traceability:** INIT-006/SPEC-016
**Status:** complete (orchestrator closed 2026-10-10 — 833 suite pass; guides chrome smokes pass)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/ui/blocks/verse-reading.js` — reference line, new words, pointed verse with tap-to-gloss, vowels control, questions, then SPEC-014's personal-question component. No scripture fetch. No stored translation.
- [x] `shared/ui/blocks/letter-intro.js` — letter, name, vowel sounds, tap-to-choose over `findIn`, paper-practice prompt. No picture. Not scored.
- [x] `shared/guides-page.js` and route `{ id: 'guides', path: '/guides/' }`. Nav label מדריכים (Guides). Published `guide` lessons grouped by topic, opened in `/shared/plan/?id=&lesson=`.
- [x] Recipe chips on the plan lesson shelf: Conversation, Review, Verse, Verbs, Letters, Guide.
- [x] Tests: `test/render-verse-letter.test.js`, `test/chrome-guides-smoke.js`, plus `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js`.

## Acceptance Criteria

- [x] ac-1 (REQ-010): hostile strings in both blocks render as text. Hebrew carries `dir="rtl"`.
- [x] ac-2 (REQ-010): Chrome opens `/guides/`, sees `data-route-ready="guides"`, opens a fixture guide, and sees its explainer section.
- [x] ac-3: the fixture verse-reading shows new words before the verse and the personal question last.
- [x] ac-4: `test/spa-shell.test.js` and `test/chrome-navigation-smoke.js` cover `guides`.

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-016-eval-report.md`](SPEC-016-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`standard` `fail` (light-tier condition) · domain=`frontend` `fail` (light-tier condition) · autonomy_level=`auto_tests` `fail` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `node --test test/render-verse-letter.test.js` exit 0. Hostile `<script>alert(1)</script>` and `<img src=x onerror=alert(1)>` are absent as markup and present escaped. No `<img>`, no `<script`, no `javascript:`, no `evil.example`. Hebrew nodes include `lang="he" dir="rtl"`. A string `contains: "true"` is `data-contains="no"`. Letter HTML has the paper prompt and no `<form>`. |
| ac-2 | PASS (in-session) | `node test/chrome-guides-smoke.js` exit 0 (`chrome-guides-smoke: PASS`). `/guides/` reaches `data-route-ready="guides"`. Topics Verbs and Reading are listed. `ZZ-HIDDEN-GUIDE` is not. Clicking the decoding link shows `[data-section-id="roots"]` with "Three letters". The plan root has no `img`. |
| ac-3 | PASS (in-session) | Same unit file. Fixture `test/fixtures/plans-1.1/verse-reading.plan.json`: `data-verse-part="new-words"` precedes `data-verse-part="verse"`, which precedes `data-block-type="personal-question"`, both in `verse.render` and in `renderScreens`. |
| ac-4 | PASS (in-session) | `node --test test/spa-shell.test.js` exit 0 (guides is an app path, the route module mounts, signed-in `/guides/` returns the shell). `node test/chrome-navigation-smoke.js` exit 0. The guides screen stays in the same document, path `/guides/`, `aria-current` on `data-nav="guides"`, one heading. |

---

## Change Produced

- **Title:** Verse readings, letter introductions, and a Guides shelf.
- **Files changed:** renderers, the Guides route, a read-only catalog, tests, this summary.
- **Authored by:** INIT-006/SPEC-016
- **Summary:** A verse-reading shows the reference, then new words, then the pointed verse. A known lexicon gloss is a tap target. The vowels control rewrites text with `textContent`. Questions keep optional English under the Hebrew. The personal question is SPEC-014's component and comes last. A letter-intro shows the letter, its name, and each vowel's pointed form and sound, with audio only when the path is a safe `.mp3`. Choosing a `findIn` word writes feedback with `textContent` and posts nothing. "Write the letter" is a paper prompt. There is no picture. Guides lists published `guide` lessons by topic and opens them in the plan view. The plan overview shows a recipe chip on each plan lesson.

## Verification

| Command | Result |
| --- | --- |
| `node --test test/render-verse-letter.test.js test/render-explainer.test.js test/plan-render.test.js test/spa-shell.test.js test/app-shell.test.js` | 59 pass, exit 0 |
| `node test/chrome-guides-smoke.js` | exit 0 (`chrome-guides-smoke: PASS`) |
| `node test/chrome-navigation-smoke.js` | exit 0 (`chrome-navigation-smoke: PASS`), including `guides` |
| `npm test` | 833 pass, 0 fail, exit 0 |

**Coverage:** this repo has no coverage tool. The spec's pass condition is the renderer tests and the Chrome smoke exiting 0.

## Gates & Controls

| Gate | Reversibility | Approval | Blocking checks | Outcome |
| --- | --- | --- | --- | --- |
| — | branch | n/a | tests and `npm test` | None. `gate_actions` is empty. No new npm dependency, no migration, no commit. Rollback: remove the `guides` route and unregister the two renderers. |

## Security Review

- **Depth:** standard, focus DOM insertion. **Reviewer:** this session (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **SAST / secret scan:** not run — none configured for this repo.
- **Findings:** none. Critical 0, High 0, Medium 0, Low 0.
- Plan strings go through `HebrewUtils.escapeHtml` when that helper is loaded, otherwise the same four replacements. `dir` is only `rtl` or `ltr` from a Hebrew-letter test. Chapter and verse numbers are integers in range or omitted. A recipe chip's `data-recipe` is one of the six recipe ids.
- An audio path is accepted only when it matches the existing `.mp3` path pattern. The path is not copied into the page. `javascript:` does not become a request.
- `letter-intro` ignores `image` and `picture`. Hostile tests reject `<img`, a raw script, and `evil.example`.
- Tap-to-gloss and find-in feedback assign `textContent`. The vowels control assigns `textContent` from the `data-vowels` attribute. Find-in `contains` is yes only when the value is boolean `true`. Nothing in that click is posted.
- The verse does not render a `translation` field. The personal question is the existing component in `speak` mode, so this block adds no answer field.
- `GET /api/plans/catalog` is session-gated and reuses `mayReadPublished`. A hidden lesson and another learner's personal plan are omitted. The payload is plan id, unit id, topic, lesson id, title, and recipe. No block bodies. Ids must be slugs. Titles are clipped to 120 characters. The shelf drops any id that is not a slug before building `/shared/plan/?id=&lesson=`, then escapes the href.
- The Guides page paints with `innerHTML` of that escaped string, the same paint the lessons shelf already uses. Click handlers do not.

## Files Changed

| File | Change | Provenance |
| --- | --- | --- |
| `shared/ui/blocks/verse-reading.js` | Created — verse renderer and vowels/gloss bind | `INIT-006/SPEC-016` |
| `shared/ui/blocks/letter-intro.js` | Created — letter renderer and find-in feedback | `INIT-006/SPEC-016` |
| `shared/guides-page.js` | Created — Guides shelf | `INIT-006/SPEC-016` |
| `shared/ui/plan-render.js` | Registered both blocks, recipe chips, gloss load | `INIT-006/SPEC-016` |
| `shared/plan/index.html` | Loads the two block scripts | `INIT-006/SPEC-016` |
| `shared/app-routes.js` | Route `guides` | `INIT-006/SPEC-016` |
| `shared/app-shell.js` | Nav link מדריכים (Guides) | `INIT-006/SPEC-016` |
| `server/spa.js` | `/guides/` is an app path | `INIT-006/SPEC-016` |
| `server/api/plan-routes.js` | `GET /api/plans/catalog` | `INIT-006/SPEC-016` |
| `shared/worksheet.css` | Letter, chip, and tap-target styles from ADR-0006 tokens | `INIT-006/SPEC-016` |
| `test/render-verse-letter.test.js` | Created — hostile render, order, chips, catalog | `INIT-006/SPEC-016` |
| `test/chrome-guides-smoke.js` | Created — Guides shelf smoke | `INIT-006/SPEC-016` |
| `test/render-explainer.test.js` | Verse and letter now render | `INIT-006/SPEC-016` |
| `test/plan-render.test.js` | Paint-line match includes the gloss map | `INIT-006/SPEC-016` |
| `test/spa-shell.test.js` | `/guides/` in the shell path lists | `INIT-006/SPEC-016` |
| `test/chrome-navigation-smoke.js` | Nav screen `guides` | `INIT-006/SPEC-016` |
| `test/web-gating.test.js` | Signed-out `/guides/` stays gated | `INIT-006/SPEC-016` |

## Deviations

- **Persona.** `primary_prompt` `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. The spec already names the degradation path. No persona was manufactured. The work follows the existing block-renderer and route-table patterns.
- **Lessons nav page.** `test/rollout-preflight.test.js` forbids `/shared/plan` inside `shared/lessons-page.js` until the written deploy gate. Recipe chips are on the plan overview (each plan lesson) and on the Guides shelf. The Lessons route still opens worksheets through `lessonHref`.
- **Catalog route.** The shelf needs a list of published lessons. `GET /api/plans/catalog` is a session read on `server/api/plan-routes.js`. `mcp/tools.js`, `mcp/http.js`, `mcp/store.js`, `server/api/learner-routes.js`, and `server/api/admin-routes.js` were not edited.
- **Timer, trace, commit.** This spec was dispatched to a subagent. The orchestrator owns the timer and the run-trace sentinels. No commit, push, or PR (dispatch instruction).

## Trace Emission Degradation

| Site | Event | Reason | Lost | Impact |
| --- | --- | --- | --- | --- |
| Phase 0 `open_trace` / Phase 5 `close_trace` | `trace_open`, `trace_complete` | Orchestrator owns the trace for a dispatched spec. This subagent did not emit. | This session's events, until the orchestrator writes them on return. | The live `.sdd-trace/SPEC-016.jsonl` is the orchestrator's file. A Tier-5 check has to run there after the orchestrator closes it. |

## Harness Adaptation

| Site | Capability | Tier | What was done |
| --- | --- | --- | --- |
| Phase 4 security review | `subagent_dispatch` | T3 (nested dispatch skipped) | Inline review, `self_reviewed: true`. |
| Phase 4.5 AC verifier | `subagent_dispatch` | T3 (nested dispatch skipped) | In-session evidence. Spec stays `in_review`, not `complete`. |

## Follow-ups

- Orchestrator: independent AC verification, then status `complete` if all-PASS.
- `letter-intro` still has no picture match (ADR-0009 §7, Pictures).
