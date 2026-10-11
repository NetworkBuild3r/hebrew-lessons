# SPEC-015 Completion Summary — Today daily mix

**Traceability:** INIT-006/SPEC-015
**Status:** complete (orchestrator closed 2026-10-10 — suite 840/840; today-mix chrome smokes pass)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/today-page.js` — Today's practice card from `GET /api/me/mix`. Review and Weak open in place. Drill uses `HebrewBlockConjugation` when that module is loaded. Reading shows sentences with a first-language gloss on tap.
- [x] Styles appended in `shared/worksheet.css` (ADR-0006 tokens). Guides and the Progress AI panel were not edited.
- [x] Tests: `test/chrome-today-mix-smoke.js`, `test/today-mix.test.js`. Today's route also loads the conjugation drill module. `test/review-page.test.js` records that script list.

## Acceptance Criteria

- [x] ac-1 (REQ-012): Chrome smoke seeds one due card and one weak card, shows both sections with counts, opens Review in place on `/`, and returns to the mix list.
- [x] ac-2 (REQ-012): that seed has no weak concept and no qualifying sentence. The mix JSON and the page both omit drill and reading.
- [x] ac-3: `test/chrome-today-lessons-smoke.js` still passes, including the existing `/review/` start link.

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-015-eval-report.md`](SPEC-015-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`light` `fail` (light-tier condition) · domain=`frontend` `fail` (light-tier condition) · autonomy_level=`auto_tests` `fail` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `node test/chrome-today-mix-smoke.js` exit 0. Review count `1`, weak count `2`, `path` `/`, session opened with no answer node, Back restored `#today-mix-list`. Existing `#today-review-start` stayed `href="/review/"`. The mix card is above the older Today cards. |
| ac-2 | PASS (in-session) | Same smoke. `GET /api/me/mix` had no `drill` and no `reading` section, and the page had neither `#today-mix-drill` nor `#today-mix-reading`. `node --test test/today-mix.test.js` also drops a drill section when `HebrewBlockConjugation` is absent, and drops a reading section the payload does not contain. |
| ac-3 | PASS (in-session) | `node test/chrome-today-lessons-smoke.js` exit 0. Due count `3`, start href `/review/`, continue slug `my-morning`. Fetch log contains only `/api/` URLs, including `/api/me/mix`. |

---

## Change Produced

- **Title:** Today shows and runs the daily mix in place.
- **Files changed:** 3 product files, 3 tests, this summary, the eval note, and the spec status.
- **Authored by:** INIT-006/SPEC-015
- **Summary:** A Today's practice card lists the mix sections the server returned, in order, with counts. Empty sections are omitted. Review and Weak open a recall-before-reveal session on the same page and return to Today. Drill is shown only when the conjugation module is loaded, and checks go through that module. Reading shows the sentence, a first-language gloss on tap, and a play button only for a same-origin worksheet clip. Done for the day is session memory. The older Today cards stay below.

## Verification

| Command | Result |
| --- | --- |
| `node --test test/today-mix.test.js` | 6 pass, exit 0 |
| `node test/chrome-today-mix-smoke.js` | exit 0 (`review` 1, `weak` 2, `path` `/`) |
| `node test/chrome-today-lessons-smoke.js` | exit 0 |
| `npm test` | 840 pass, 0 fail, exit 0 |

**Coverage:** this repo has no coverage tool. The spec's pass condition is `node test/chrome-today-mix-smoke.js` exiting 0. The other commands are the blocking checks and the unit checks for section order, escaping, clip URLs, and `ctx.signal`.

## Gates & Controls

| Gate | Reversibility | Approval | Blocking checks | Outcome |
| --- | --- | --- | --- | --- |
| — | branch | n/a | Chrome smokes and `npm test` | None. `gate_actions` is empty. No new npm dependency, no migration. Rollback: remove the practice card. |

## Security Review

- **Depth:** light (quick). Own data only. **Reviewer:** this session (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **SAST / secret scan:** not run — none configured for this repo.
- **Findings:** none. Critical 0, High 0, Medium 0, Low 0.
- The practice list is HTML only after `escapeHtml`. A unit test renders a drill label `<img src=x onerror=alert(1)>` and a prompt `<script>alert(1)</script>` and finds no raw tag. The gloss `secret-gloss` is not in the list HTML.
- The in-place session, the reading gloss, the drill feedback, and the revealed form are assigned with `textContent`. The answer node is created only after Show answer. The form key is written only after the second miss.
- Clip URLs must be same-origin `http(s)` paths under `/worksheets/<slug>/…mp3`. `javascript:`, another origin, `..`, and a relative `audio/…` path are rejected.
- A drill signal is `HebrewBlockConjugation.formSignal`. The typed string is not a field. The input keeps the typed text.
- Done state in `sessionStorage` (`hebrew-today-mix-done:v1`) is the local date plus booleans for the four section ids. It does not store words, glosses, or answers.
- `GET /api/me/mix` and the reading library fetch receive `ctx.signal`. After that signal aborts, a late mix body does not paint.

## Files Changed

| File | Change | Provenance |
| --- | --- | --- |
| `shared/today-page.js` | Today's practice card, in-place sections, abort on `ctx.signal` | INIT-006/SPEC-015 |
| `shared/app-routes.js` | Today loads the conjugation drill module before the page | INIT-006/SPEC-015 |
| `shared/worksheet.css` | Appended mix rules using ADR-0006 tokens | INIT-006/SPEC-015 |
| `test/chrome-today-mix-smoke.js` | Created — ac-1 and ac-2 | INIT-006/SPEC-015 |
| `test/today-mix.test.js` | Created — section order, escaping, clip URL, `ctx.signal` | INIT-006/SPEC-015 |
| `test/review-page.test.js` | Today script list includes the drill module | INIT-006/SPEC-015 |

## Deviations

- **Persona.** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed on the spec. No AGENT.md was invented. The screen follows ADR-0006 tokens, bilingual labels, and escaped text.
- **Timer and run-trace.** This spec was dispatched to a subagent. The orchestrator owns Start/Complete Timer and `open_trace` / `close_trace`. Those steps were not run here.
- **Commit / push / PR.** Skipped. The dispatch said not to commit.
- **Independent verifier.** Not nested from this subagent. Mechanized results are recorded above; the spec stays `in_review`.
- **In-place cards.** Review and Weak practice the mix items on Today (prompt, then the answer). The older Today review card still links to `/review/` for the four-grade screen. Today does not schedule those cards.
- **Drill.** The mix items are not a conjugation-table block, so Today calls `decideAttempt` and `formSignal` instead of mounting a plan drill station. If that module is missing, the drill section is hidden.
- **Done mark.** Session memory for the local date (`sessionStorage`, with an in-memory fallback). No lesson blob and no new table.
- **Reading audio.** A play button is shown only when `audio` resolves to a same-origin worksheet `.mp3`. The mix payload does not include a lesson slug, so a relative `audio/…` path stays silent.
- **Guides and Progress.** Not edited.
- **No new npm dependency.**

## Trace Emission Degradation

Skipped — the orchestrator owns `open_trace` / `emit` / `close_trace` for a dispatched spec. No events were written by this session. Impact: the Tier-5 invariants check and the model-dispatch check have no live trace from this subagent until the orchestrator opens and closes one.

## Harness Adaptation

Cursor session, already running as a subagent. Phase 4 security review and Phase 4.5 AC verification were done in this session (`self_reviewed: true` for security; AC verdicts are mechanized but not from a fresh verifier). Not a T4 escalation.

## Follow-ups

- SPEC-016 owns verse reading and the Guides shelf. This spec did not change them.
- SPEC-017 owns the Progress AI panel. This spec did not change it.
- SPEC-020 is the rollout that this spec blocks.
