# SPEC-008 Completion Summary — Daily-mix composer

**Traceability:** INIT-006/SPEC-008
**Status:** complete (orchestrator closed 2026-10-10 — re-ran learner-mix tests)
**Domain:** service
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/learner/mix.js`
- [x] Tests: `test/learner-mix.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-012): a fixture with 40 due cards returns 30 in `review`; every `review` and `weak` word is in the learner's review rows or library, every `drill` form has a lemma in the library, and a `reading` sentence has at most 5% unknown running words.
- [x] ac-2 (REQ-012): `drill` contains only `verified` forms and spans two lemmas when two qualify.
- [x] ac-3 (REQ-012): every `reading` sentence has coverage ≥ 0.95 for the fixture learner; with none qualifying, the section is absent.
- [x] ac-4: same inputs and `now` produce the same output. The same local date does too (ADR-0009 §8 seeds the shuffle by user id and local date).

## AC Verification (independent)

Independent verification was **not** dispatched. This run is a subagent and did not spawn a second verifier. The table below is the implementer's test evidence, not an independent verdict. Spec status stays `in_review` until a fresh verifier records all-PASS.
Overall: **UNVERIFIED** · remediation loops: 0.
**Full eval report:** not written — no independent verifier ran.

**Verification tier:** `full` · **pre-flight:** not classified light by the orchestrator (no light-tier line in the dispatch) · **checks skipped:** `None` (tests were run by the implementer; the independent re-run was not)

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | UNVERIFIED (implementer run passed) | `ac-1: 40 due cards return the due cap, and every item stays in this library` in `test/learner-mix.test.js`. Review length equals `Review.DEFAULT_CAPS.due` (30). The oldest due card is included and the 31st is not. A suspended card and another learner's due card and weak word are absent. Each weak word is emitted in `he_en` and `en_he` and is not already in review. Drill lemmas are in this learner's library. Each reading sentence has an unknown share ≤ 0.05. `node --test test/learner-mix.test.js` → 8 pass, 0 fail, exit 0. |
| ac-2 | UNVERIFIED (implementer run passed) | `ac-2: drill uses only verified forms and two lemmas when two qualify`. Concept is `tense:past` (a weaker concept with no verified library form, and an emerging concept with only 4 attempts, are not chosen). Six items, both lemmas, no lemma twice in a row. Unverified `חשבנו`, a form whose lemma is outside the library, and a present-tense form are absent. Accepted spellings include the form key and `חשבתא`. With the lexicon and library cleared, the drill section is absent. |
| ac-3 | UNVERIFIED (implementer run passed) | `ac-3: reading requires coverage of 0.95 and is omitted when none qualify`. A 19-known + 1-unknown sentence is the only reading item and its coverage is ≥ 0.95. An 18-known + 2-unknown sentence is absent. When that low sentence is the only candidate, the reading section is absent and the mix is `review` then `weak`. |
| ac-4 | UNVERIFIED (implementer run passed) | `ac-4: the same now, and the same local date, return the same mix`. Two calls at 20:00 on 2026-10-10 are deep-equal, and a call at 08:00 the same local date matches them. Reading returns 3 of 4 qualifying sentences. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-008`
- **Title:** Compose a learner's daily mix from due cards, weak words, one concept drill, and one known-word reading
- **Files changed:** 2 · additions: `server/learner/mix.js`, `test/learner-mix.test.js` · deletions: none
- **Change status:** proposed
- **Reversibility:** branch. Rollback is to delete the module and the test. Nothing is stored. No migration. No new npm dependency.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/learner-mix.test.js` | 8 | 8 |

**All tests passing:** Scoped yes · `node --test test/learner-mix.test.js` → 8/8, exit 0. `npm test` → 756 pass, 1 fail, exit 1. The failure is `test/personal-plan.test.js` “a stale revision does not assign, and progress is not read”: the source slice from `findAccount` to `findByProvider` now includes INIT-006/SPEC-007 comments that name `password_hash`. This spec did not edit `server/db/repos/users.js` or `server/plans/personal.js`. **Coverage (new code):** no coverage collector is configured in this repo.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | `node --test test/learner-mix.test.js` exit 0. `npm test` exit 1 on an unrelated personal-plan source assertion | No `gate_actions`. No migration. No new npm dependency. No commit. |

None — branch/reversible only.

---

## Security Review

- **Sensitivity / depth:** standard · quick · **method:** inline review by the implementer (`self_reviewed: true`). A separate security-review subagent was not spawned. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** `compose` returns before any query when the user id is not a positive integer. Review, library, lesson-progress, and mastery reads all bind that id. Another learner's due card and weak word are absent in ac-1. Suspended rows are left out of review and weak. Drill forms are included only when the lemma is in this learner's library, and only when `verified` is 1. The mix is not stored. `opts.lessonsDir` is a server test option; the HTTP route is SPEC-010 and must not forward request fields into `opts`. Shared lesson sentences are the only place an unknown word can appear, and only under the 0.95 coverage rule.

---

## Files Changed

Provenance is `INIT-006/SPEC-008` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/learner/mix.js` | Created — `compose(userId, now, opts)` | `INIT-006/SPEC-008` |
| `test/learner-mix.test.js` | Created — temporary database and lessons folder | `INIT-006/SPEC-008` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-008-service-daily-mix.md` | Status set to `in_review` | `INIT-006/SPEC-008` |

`shared/player/review.js` already exported `DEFAULT_CAPS`. This spec imports it and does not restate 30 or 10.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 sentinel | `trace_open`, `trace_complete` | Orchestrator owns the timer and the run-trace for a dispatched spec. This subagent did not call `open_trace` / `emit` / `close_trace`. | No events from this process in `.sdd-trace/SPEC-008.jsonl` unless the orchestrator writes them. | A Tier-5 check against an empty or missing file stays INCONCLUSIVE or FAIL until the orchestrator closes the trace. This is the ownership rule, not a failed emit. |

---

## Harness Adaptation

Dispatched subagent on Cursor. Independent AC verifier and security-review subagent were not spawned (this agent was instructed not to spawn further subagents, and the persona file is absent). AC evidence is implementer-run. Security review is `self_reviewed: true`.

---

## Deviations from the Spec

- Commit, push, and PR were not done. The dispatch said not to commit.
- `.claude/agents/` has no `AGENT.md` in this repo, so `principal-backend-developer` was not loaded. The work followed the service rules in the spec: user-scoped reads, caps imported from `shared/player/review.js`, seeded shuffle, no new dependency.
- `server/learner/coverage.js` (INIT-006/SPEC-007) is not on disk. `compose` calls `checkCoverage(userId, text, { audience: 'self' })` when that module exports it. Until then, reading coverage uses the ADR-0009 §1 rule in `mix.js` (exact key, unique vowel-stripped form, or a lexicon form whose lemma is in the library; no prefix stripping). A missing module is cached as absent for the process; a missing dependency inside `coverage.js` is rethrown.
- SPEC-009 HTTP and SPEC-011 were not edited.
- Plan stories are not reading sources yet (assumption aud-1). Sentences come from `server/vocab-index.js`.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 4 hours
