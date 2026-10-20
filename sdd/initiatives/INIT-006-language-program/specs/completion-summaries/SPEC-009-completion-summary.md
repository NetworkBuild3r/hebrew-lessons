# SPEC-009 Completion Summary — Library and flashcard routes

**Traceability:** INIT-006/SPEC-009
**Status:** in_review (independent AC verification PASS; run-trace close belongs to the orchestrator)
**Domain:** api
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/learner/library.js`, `server/learner/cards.js`, `server/api/library-routes.js`
- [x] Mounted on the API router from `server/web.js` (see Deviations)
- [x] Tests: `test/library-routes.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-005): a fixture learner with 5 reviewed words and 2 added words gets 7 library rows with lexicon tags and correct stage per direction.
- [x] ac-2 (REQ-005): `POST /api/me/library` without a valid origin returns 403; with one, it creates two review rows per word and is idempotent on repeat. A key with no lexicon or vocab entry and no `gloss_en` returns 400; with one, it stores the note on `user_words` and the `lexicon_entries` row count is unchanged. After an admin adds a lexicon entry for that key, the list row shows the lexicon gloss and the learner's note as `note`.
- [x] ac-3 (REQ-003): `GET /api/me/cards?deck=weak` returns only weak or very weak words; each card has four `intervals`.
- [x] ac-4: a suspended word appears in the library with `suspended: true` and in no deck.
- [x] ac-5: a signed-out request to any route returns 401; another learner's words never appear (two-user fixture).

## AC Verification (independent)

Verified by a fresh sub-agent that did **not** author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-009-eval-report.md`](SPEC-009-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `PASS` · security_review=`required` `FAIL` (light-tier condition) · domain=`api` `FAIL` (light-tier condition) · autonomy_level=`auto_tests` `FAIL` (light-tier condition) ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: five reviewed words and two added words are seven tagged rows` · `test/library-routes.test.js:103` · `node --test test/library-routes.test.js` exit 0 |
| ac-2 | PASS | `ac-2: origin, idempotent add, personal gloss, then lexicon gloss plus note` · `test/library-routes.test.js:172` · same command, exit 0 |
| ac-3 | PASS | `ac-3: weak deck is only weak or very weak words, each with four intervals` · `test/library-routes.test.js:236` · same command, exit 0 |
| ac-4 | PASS | `ac-4: a suspended word stays in the library and leaves every deck` · `test/library-routes.test.js:297` · same command, exit 0. A no-Origin PATCH assertion was added after the verdict; the file was re-run, 10 pass, exit 0. |
| ac-5 | PASS | `ac-5: signed-out requests are 401 and learners cannot see each other` · `test/library-routes.test.js:339` · same command, exit 0 |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-009`
- **Title:** Learner library, flashcard decks, and signed-in lexicon and grammar reads
- **Files changed:** 7 · new modules and the route test, plus the router mount, cap export, and harness cache list
- **Change status:** proposed
- **Reversibility:** branch

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/library-routes.test.js` | 10 | 10 |

**All tests passing:** the spec file exits 0 (10/10), including a re-run after the PATCH CSRF assertion. `npm test` in this worktree was 756 pass / 1 fail. The failure is `test/personal-plan.test.js` (`findAccount` selects `display_name`), in `server/db/repos/users.js`, which this spec does not edit. An earlier full run also failed `test/learner-mix.test.js` while that file was mid-edit; a later isolated run of that file passed 8/8.
**Coverage (new code):** not measured. This repo has no coverage tool. The spec's pass condition is the named route file exiting 0.

---

## Gates & Controls

None — branch/reversible only. No `gate_action` is configured. No commit, push, or pull request (dispatch said to skip them).

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | — | `node --test test/library-routes.test.js` exit 0 | Rollback is to unmount `libraryRoutes.register` in `server/web.js`. |

---

## Security Review

- **Sensitivity / depth:** required · standard focus (authz, CSRF, input limits) · **method:** security-review subagent
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| none | — | — | — | — |

- **Positive observations:** Every new route takes the user id from the session. Writes reject a body `userId` and any key other than `words` or `suspended`. `library.add` from the HTTP route always uses `source: 'self'` and never inserts a lexicon row. Body size is 16 KiB and the batch cap is 50. Cursors are opaque and checked for a round trip. Learner lexicon reads drop `verified_by` and return `verified` as a boolean. The subagent also confirmed two-user isolation and the CSRF gate in `server/web.js`.

---

## Files Changed

Provenance is `INIT-006/SPEC-009` for every row below (the spec that authored the change).

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/learner/library.js` | Created — list, add, suspend; lexicon is read only | `INIT-006/SPEC-009` |
| `server/learner/cards.js` | Created — decks and FSRS preview intervals | `INIT-006/SPEC-009` |
| `server/api/library-routes.js` | Created — session routes for library, cards, lexicon, concepts | `INIT-006/SPEC-009` |
| `server/web.js` | Modified — `libraryRoutes.register(router)` | `INIT-006/SPEC-009` |
| `shared/player/review.js` | Modified — export existing `DEFAULT_CAPS` | `INIT-006/SPEC-009` |
| `test/library-routes.test.js` | Created — ac-1 through ac-5 plus limits, signals, lexicon reads | `INIT-006/SPEC-009` |
| `test/helpers/signed-in-server.js` | Modified — drop the new modules from the require cache on restart | `INIT-006/SPEC-009` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 / Phase 5 sentinel | `trace_open`, `trace_complete` | Orchestrator owns the timer and the run-trace for a dispatched spec. This subagent did not call `open_trace` / `emit` / `close_trace`. | No events from this process unless the orchestrator writes them. | A Tier-5 check against a missing file stays INCONCLUSIVE or FAIL until the orchestrator closes the trace. This is the ownership rule, not a failed emit. |

---

## Deviations from the Spec

- **Mount point.** The spec says mount in `server/api/router.js`. That module is the generic matcher. Every existing route module is registered from `server/web.js`, which is what this spec does.
- **Graded signals.** The existing route is `POST /api/review/signals` (`server/api/progress-routes.js`). The spec names `POST /api/progress/signals`. Tests cover `grade` and `kind` on the route the client already calls. No second path was added.
- **Persona.** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo, and `scripts/dev/expertise_state.py` is not here, so the dispatch-check was not run. No persona file was invented. The work follows the spec and the existing route modules.
- **Git.** No commit, push, or pull request, per the dispatch. The worktree stays dirty.
- **Full suite.** `npm test` is not green in this shared worktree because `test/personal-plan.test.js` fails on `server/db/repos/users.js`, which this spec does not change. The spec file itself passes.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
