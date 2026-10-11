# SPEC-017 Completion Summary — Progress AI-profile panel and admin switch

**Traceability:** INIT-006/SPEC-017
**Status:** complete (orchestrator closed 2026-10-10 — chrome-ai-profile + admin-users-view PASS)
**Domain:** frontend
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/progress-page.js` — "What your AI sees" from `GET /api/me/profile` (the response body, not a recomputed profile)
- [x] `shared/admin/users.js` — AI planning control per user, confirm dialog, state taken from the 200 body
- [x] `shared/login.js`, `login/index.html`, `shared/app-shell.js` — disclosure sentence beside the existing admin-visibility sentence
- [x] `shared/worksheet.css` — panel, raw JSON, and root-class swatch styles
- [x] `shared/api.js` — optional `raw: true` so the profile body string is kept
- [x] `server/api/admin-routes.js` — admin user list includes `ai_planning` so the control can show the current flag
- [x] Tests: `test/chrome-ai-profile-smoke.js`; `test/admin-users-view.test.js` updated

## Acceptance Criteria

- [x] ac-1 (REQ-008): the wire text equals the `/api/me/profile` body; the visible raw block is that JSON pretty-printed. Smoke compares both.
- [x] ac-2 (REQ-008): the ADR-0009 disclosure sentence is on Progress, in the admin confirm, on the sign-in page, and in the account menu. The sign-in page and the account menu still contain the admin-visibility sentence.
- [x] ac-3: a learner sees no toggle. `/admin/` for that learner is the server's plain Forbidden page. `PATCH` as a learner returns 403.
- [x] ac-4: the row changes only when the response is 200 and `ai_planning` is a boolean. A failure leaves the old flag and shows the error.

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-017-eval-report.md`](SPEC-017-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`required` `fail` (light-tier condition) · domain=`frontend` `fail` (light-tier condition) · autonomy_level=`auto_tests` `fail` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `node test/chrome-ai-profile-smoke.js` exit 0. `[data-ai-profile-wire]` text equals the HTTP body. `[data-ai-profile-shown]` equals the pretty-print of that body. |
| ac-2 | PASS (in-session) | Same smoke: progress disclosure, account-menu notes, sign-in page, and the open confirm dialog contain the ADR-0009 English sentence. The menu and the sign-in page still contain the admin-visibility sentence. |
| ac-3 | PASS (in-session) | Same smoke: no toggle on Progress; learner PATCH returns 403; `/admin/` as the learner is "Forbidden" with no toggle. |
| ac-4 | PASS (in-session) | `node --test test/admin-users-view.test.js` exit 0. Failure keeps `aria-pressed="false"` and shows the server message. Success paints `ai_planning` from the body. A 200 without that boolean does not flip the flag. The smoke keeps `aria-pressed` false while the dialog is open, leaves it false after Cancel, and sets it true only after Confirm's 200. |

---

## Change Produced

- **Title:** The learner can read the same profile the AI would get, and an admin turns planning on only after confirming.
- **Files changed:** 8 code files, 2 tests, this summary, the eval note, and the spec status.
- **Authored by:** INIT-006/SPEC-017
- **Summary:** Progress loads `GET /api/me/profile` and paints that object: journey stage and counts, grammar grouped by binyan, tense, person cell, and root class (root class carries the traffic-light colour and the colour's name), weak and very weak words linked into Words, and a Show raw block. A null strength is shown as "no strength", not as a missing field and not as the word "null". The switch line is "AI planning: on/off — an admin changes this". The admin users table has a control that opens a confirm dialog with the disclosure sentence and then `PATCH /api/admin/users/:id/ai-planning`. The row updates from `ai_planning` on the 200. The Hebrew disclosure line is a draft for the operator.

## Verification

| Command | Result |
| --- | --- |
| `node --test test/admin-users-view.test.js test/progress-view.test.js test/app-shell.test.js test/design-tokens.test.js` | pass, exit 0 |
| `node test/chrome-ai-profile-smoke.js` | exit 0 (`chrome-ai-profile-smoke: PASS`) |
| `node test/chrome-progress-smoke.js` | exit 0 |
| `npm test` | 840 pass, 0 fail, exit 0 |

**Coverage:** this repo has no coverage tool. The spec's pass condition is the smoke and the admin users unit test exiting 0, including the byte comparison.

## Gates & Controls

| Gate | Reversibility | Approval | Blocking checks | Outcome |
| --- | --- | --- | --- | --- |
| — | branch | n/a | smoke, unit test, `npm test` | None. `gate_actions` is empty. No new npm dependency, no migration. Rollback: remove the Progress section and the admin control. |

## Security Review

- **Depth:** required. Focus: confirm flow, admin gating, profile text in the page. **Reviewer:** this session (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **SAST / secret scan:** not run — none configured for this repo.
- **Findings:** none unresolved. Critical 0, High 0, Medium 0, Low 0.
- The profile section does not put profile fields through `innerHTML`. Labels and values are `textContent` or `setAttribute`. Show raw assigns the response text with `textContent`. A fixture word id of `<img src=x onerror=alert(1)>` produced no `img` element.
- Word links are only `/words/?weak=weak` or `/words/?weak=very_weak`, plus `q=` via `encodeURIComponent`. The weak flag is one of those two constants.
- The control is a button. It does not change `aria-pressed` when the dialog opens. The PATCH runs on confirm. The row is rewritten only when `res.ok` and `data.ai_planning` is a boolean. Any other result shows the error and repaints the previous flag.
- A learner has no control on Progress. `GET /admin/` for a non-admin is the existing plain 403. `PATCH /api/admin/users/:id/ai-planning` as a learner returns 403. The new list field is on that same admin-only users route.
- The confirm dialog quotes the static disclosure sentence (`escapeHtml`). The account name in the dialog is `textContent`.
- `GET /api/admin/users` now includes `ai_planning` as a boolean read from `users.getAiPlanning`. It does not add password hashes, tokens, or another learner's profile.

## Files Changed

| File | Change | Provenance |
| --- | --- | --- |
| `shared/progress-page.js` | Profile section from the profile response body; null strength; Show raw | INIT-006/SPEC-017 |
| `shared/admin/users.js` | Per-user control, confirm dialog, apply only on a boolean 200 | INIT-006/SPEC-017 |
| `shared/app-shell.js` | Disclosure paragraph beside the admin-visibility note | INIT-006/SPEC-017 |
| `shared/login.js` | Same disclosure, set on `[data-ai-disclosure]` | INIT-006/SPEC-017 |
| `login/index.html` | Disclosure beside both copies of the admin-visibility sentence; inline script sets it | INIT-006/SPEC-017 |
| `shared/worksheet.css` | Profile layout, raw block, root-class swatches | INIT-006/SPEC-017 |
| `shared/api.js` | `raw: true` keeps `rawText`; default requests unchanged | INIT-006/SPEC-017 |
| `server/api/admin-routes.js` | Admin user rows include `ai_planning` | INIT-006/SPEC-017 |
| `test/chrome-ai-profile-smoke.js` | Created — byte compare, disclosure, non-admin, confirm | INIT-006/SPEC-017 |
| `test/admin-users-view.test.js` | Failure leaves the flag; success uses the response boolean | INIT-006/SPEC-017 |

## Deviations

- **Persona.** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed on the spec. No persona file was invented. The screen follows the existing bilingual shell, escaped text, and ADR-0006 tokens.
- **Timer and run-trace.** This spec was dispatched to a subagent. The orchestrator owns Start/Complete Timer and `open_trace` / `close_trace`. Those steps were not run here.
- **Commit / push / PR.** Skipped. The dispatch said not to commit.
- **Independent verifier.** Not nested from this subagent. Mechanized results are recorded above; the spec stays `in_review`.
- **Hebrew disclosure.** The English sentence is the ADR-0009 contract. The Hebrew line is a draft until the operator approves it. It sits beside the admin-visibility sentence and does not replace it.
- **Where the sentence is stored.** Progress reads `HebrewShell.AI_DISCLOSURE` at render time. The app shell script loads before the progress view. Putting the sentence in `progress-page.js` would trip that file's existing ban on the word "points" (`vowel points` is not a score). The smoke checks the rendered sentence on the Progress page.
- **Admin list field.** `GET /api/admin/users` did not return the switch. The control cannot show a current state without it, and guessing "off" would be wrong. The list row now includes the boolean. No new route.
- **Root-class colour.** The profile object has no colour field. The swatch is the registry colour for that concept key (green, yellow, red), with the colour name in text. Green uses `--color-control`. Yellow and red are dark swatches, not yellow or red text.
- **hebrew-worksheet skill.** Not edited (SPEC-019).
- **No new npm dependency.**

## Trace Emission Degradation

Skipped — the orchestrator owns `open_trace` / `emit` / `close_trace` for a dispatched spec. No events were written by this session. Impact: the Tier-5 invariants check and the model-dispatch check have no live trace from this subagent until the orchestrator opens and closes one.

## Harness Adaptation

Cursor session, already running as a subagent. Phase 4 security review and Phase 4.5 AC verification were done in this session (`self_reviewed: true` for security; AC verdicts are mechanized but not from a fresh verifier). Not a T4 escalation.

## Follow-ups

- The operator still has to approve the Hebrew disclosure line before it ships.
- SPEC-019 owns the worksheet skill and the operator guide. This spec did not edit them.
- SPEC-020 owns rollout. This spec did not change deploy steps.
