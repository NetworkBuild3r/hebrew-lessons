# SPEC-007 Completion Summary — Learner profile contract and text coverage

**Traceability:** INIT-006/SPEC-007
**Status:** complete (orchestrator closed 2026-10-10 — independent AC all-PASS; no Critical/High). Open follow-ups recorded in summary: null FSRS strength, coverage stripped-vowel sentence, recipe counts across personal plans.
**Domain:** service
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/learner/profile.js`, `server/learner/profile-contract.js`, `server/learner/coverage.js`
- [x] Tests: `test/learner-profile.test.js`, `test/learner-coverage.test.js`
- [x] Supporting files, not named in the Deliverables: `server/learner/gate.js` (access, rate, audit for both services), `server/db/repos/coverage.js`, two account lookups in `server/db/repos/users.js`, and explicit columns in `server/db/repos/lesson-progress.js`

## Acceptance Criteria

- [x] ac-1 (REQ-007): with the switch on, the profile JSON contains every ADR-0009 §1 field, and a recursive key and value walk finds no name in `NEVER_RETURNED`, no ISO timestamp string, and no numeric user id.
- [x] ac-2 (REQ-007): switch off yields `{ error: 'planning_off' }` only, and an audit row with `fields: none`.
- [x] ac-3 (REQ-007): the 61st MCP read in a clock hour (profile and coverage combined) returns `rate_limited` and writes an audit row.
- [x] ac-4 (REQ-008): `audience: 'self'` returns a deep-equal object to the `mcp` result (minus `ai_planning`) for the same fixture, and writes no audit row.
- [x] ac-5 (REQ-009): on a fixture text of 20 Hebrew words with 18 known, coverage is 0.9 and `unknown` lists the other two. A known word with a leading ו is in `unknown` and lowers coverage until a `form` entry whose `lemma_key` is the known word exists, after which it is known. The result has no `matched_by` field.

## AC Verification (independent)

Verified by fresh sub-agents that did **not** author the code, not self-reported: [Verify SPEC-007 acceptance criteria](a4eed042-e043-480e-b8ec-1e002d022c70), then [Re-verify SPEC-007 gate after clock fix](4ca1bf47-16fa-46d9-b66a-f09bd355825e) after the security remediation.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 1.
**Full eval report:** [`SPEC-007-eval-report.md`](SPEC-007-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** not classified light by the orchestrator (no light-tier line in the dispatch); security_review=`required` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: the profile holds every ADR-0009 §1 field…` (path set equals `RETURNED_FIELDS` both ways), `ac-1: a key and value walk…`, `ac-1: words[] stops at 500…`. Verifier probe: 38/38 paths, 318 values, no violation. |
| ac-2 | PASS | `ac-2: switch off returns planning_off…` and the coverage switch-off test. Exact deep equality on body and audit detail. |
| ac-3 | PASS | `ac-3: the 61st MCP read in a clock hour…`: 60 mixed reads, then `rate_limited` for profile and coverage, 62 audit rows, reset in the next UTC hour. Re-verified after the clock fix. |
| ac-4 | PASS | `ac-4: self returns the MCP object plus ai_planning…`: strict deep equality with the switch on and off, audit count unchanged, self reads take no MCP read. |
| ac-5 | PASS | `ac-5: twenty Hebrew words with eighteen known…` → `{ coverage: 0.9, known_count: 18, running_words: 20, unknown: [2] }`; `ac-5: a known word with a leading ו…` → 18/21 then 19/21. Verifier counted the fixture token by token. |

The verifier injected 18 defects into a temporary copy; the SPEC-007 tests caught all 18.

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-007`
- **Title:** ADR-0009 §1 learner profile and known-word coverage, gated by the AI-planning switch, limited to 60 MCP reads an hour, and audited
- **Files changed:** 9 (7 new, 2 modified) · additions about +2,090 (1,101 of them tests) · deletions -2
- **Change status:** proposed
- **Reversibility:** branch. Rollback: delete the five new modules and two tests, and remove `findLearnerById` and `findLearnerByUsername` from `users.js`. The `lesson-progress.js` column list can stay, since it returns the same columns. No migration. The only persistent state is `audit_log` rows.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/learner-profile.test.js` | 14 | 14 |
| `test/learner-coverage.test.js` | 11 | 11 |

**All tests passing:** Yes · `node --test test/learner-profile.test.js test/learner-coverage.test.js` → 25/25, exit 0 · `npm test` → 789/789, exit 0, after the last code change. Two later comment-only edits, in `users.js` and the `profile.js` header, were followed by runs of the spec files and of `personal-plan` and `learner-mix` (37/37, exit 0) · `test/learner-mix.test.js` (SPEC-008) passes against the real `checkCoverage`, which `mix.js` now loads instead of its fallback · **Coverage (new code):** no coverage collector is configured in this repo. The verifier's mutation run (18 of 18 injected defects caught) is the evidence that the tests exercise the code.

Quality gates: no lint, type-check, or coverage tool is configured. `node --check` is clean on every changed file. No suppression comment was added and no threshold was moved.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | both spec test files and `npm test` | Both exited 0. No `gate_actions`, no migration, no new npm dependency, no HTTP or MCP route. |

None — branch/reversible only.

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** `security-review` sub-agent [Security Review](1ce9f3aa-c6ed-4003-88cd-1315e34186c4), scoped to this spec's files (the dirty checkout's other changes were excluded by instruction). Inherited model. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 1 (fixed) · Low: 1 (fixed) · Info: 3 (accepted)
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| SEC-M-01: the hourly MCP count and the audit time came from the caller's `now`, so a far-future value reset the count and set the audit time | Medium | API4 resource consumption; audit integrity | `server/learner/gate.js` `admit`, `record` | fixed — server clock in `gate.js`; `now` only sets the strength day; test `a caller-supplied now does not reset the hourly count…`; re-verified 6/6 |
| SEC-L-01: a `now` whose conversion throws used a rate slot and wrote no audit row (found by the re-verifier) | Low | audit completeness | `server/learner/profile.js` `buildProfile` | fixed — `now` is converted before admission; test `a now that cannot be read throws before the call is counted` |
| SEC-I-01: `model.recipeCounts` reads every published snapshot, including other learners' personal plans, and matches by lesson id or `legacyStorageKey`; a shared id can add a recipe count | Info | data minimization | `server/learner/model.js` `recipeCounts` (SPEC-006) | accepted — output is this learner's counts only; follow-up below |
| SEC-I-02: coverage can confirm library membership for words outside the profile's 500-word cap or never graded | Info | inference | `server/learner/coverage.js` | accepted — ADR-0009 §1 ("it is learner data and is gated like the profile"); switch-gated, counted, audited |
| SEC-I-03: the hourly count is in memory and resets on restart | Info | availability | `server/learner/gate.js` | accepted — one replica (ADR-0002) |

- **Positive observations:** the profile is built field by field and its order uses returned values only, so 12 lapses and 9 lapses sort the same. Every refusal is exactly `{ error }`. A `self` read accepts only a numeric user id and throws on a username, so an adapter cannot read another account through a request field. A result is returned only after its audit row is written, and a read that throws is audited as `error`. Audit details hold an outcome, contract field paths, and counts. A missing account is recorded with a null target, so the row does not keep arbitrary text from the caller. Every query on the path names its columns and binds the user id. The test checks this at the SQL wrapper, and the verifier repeated it at the driver.

---

## Files Changed

Provenance is `INIT-006/SPEC-007` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/learner/profile-contract.js` | Created — `RETURNED_FIELDS`, `NEVER_RETURNED`, `SELF_FIELDS`, `COVERAGE_FIELDS`, `LIMITS`, `AUDIT`, `fieldPaths` | `INIT-006/SPEC-007` |
| `server/learner/gate.js` | Created — audience check, learner lookup, switch, shared hourly MCP count on a server clock, audit rows | `INIT-006/SPEC-007` |
| `server/learner/profile.js` | Created — `buildProfile`, explicit projection from `model.wordStates`, `conceptStates`, `journey` | `INIT-006/SPEC-007` |
| `server/learner/coverage.js` | Created — `checkCoverage`, `tokenize` | `INIT-006/SPEC-007` |
| `server/db/repos/coverage.js` | Created — `listLibraryWords` (review rows plus `user_words`, one user-scoped query), `listMatchKeys` | `INIT-006/SPEC-007` |
| `server/db/repos/users.js` | Modified — `findLearnerById`, `findLearnerByUsername` (`id, username, status, ai_planning`) | `INIT-006/SPEC-007` |
| `server/db/repos/lesson-progress.js` | Modified — `find` and `listByUser` name their five columns instead of `SELECT *` | `INIT-006/SPEC-007` |
| `test/learner-profile.test.js` | Created — 14 tests | `INIT-006/SPEC-007` |
| `test/learner-coverage.test.js` | Created — 11 tests | `INIT-006/SPEC-007` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 sentinel, Phase 1 steps, Phase 4.5 dispatches | `trace_open`, `task_step`, `dispatch`, `trace_complete` | The orchestrator owns the timer and trace for a dispatched spec. The trace library (`scripts/dev/lib/sdd_trace`) and both validators (`validate_run_trace_invariants.py`, `validate_model_dispatch_conformance.py`) are not in this repo; `scripts/dev/` does not exist. | No `.sdd-trace/SPEC-007.jsonl` events from this process. The Tier-5 and model-dispatch checks cannot run here. | A Tier-5 reader finds no trace and must not read PASS. Same environment gap as the INIT-004 SPEC-020 summary. |

---

## Deviations from the Spec

- Commit, push, and PR were not done. The dispatch said not to commit.
- `.claude/agents/` is not in this repo, so `principal-backend-developer` was not loaded. The work followed the spec's service rules: data access in repos, user-scoped queries, explicit columns.
- **Extra files.** `gate.js` holds the access rules both services share. `server/db/repos/coverage.js` is a new repo file because `review.js`, `user-words.js`, and `lexicon.js` were open to the SPEC-008 and SPEC-009 agents running in the same checkout. `lesson-progress.js` changed because the profile reads `listByUser`, and the spec forbids `SELECT *`.
- **Arguments.** `buildProfile(learner, { audience, now })` and `checkCoverage(learner, text, { audience })`. For `mcp`, `learner` is the username the tool receives, or a user id. That puts `not_found` refusals under the rate count and the audit row. For `self`, `learner` must be the signed-in user's numeric id, and anything else throws `TypeError`. `checkCoverage` takes no `now`, and `buildProfile`'s `now` only sets the day strength is measured on. The INIT-006 §12 parity table calls these `profile.build` and `coverage.check`; this spec and SPEC-010 use `buildProfile` and `checkCoverage`.
- **Refusal codes.** `rate_limited`, `not_found` (no such account, or a disabled one, per aud-2), and `planning_off`, plus `invalid_text` for coverage text that is not a string of at most 4000 characters. For `mcp` the checks run in that order: rate, input, account, switch. Every `mcp` call counts toward the 60, including refusals, until the hour is full. The hour is the UTC clock hour, as in `server/media/ingest.js`.
- **Readings of the ADR, written down because the ADR does not pin them:**
  - A direction has "a graded review" once it has passed or has a lapse. Every grade of 1 adds a lapse and every grade of 2 or more is a pass, so a library-add row or a `not_yet`-only row is left out.
  - Word order inside a flag group uses the weaker of the word's graded directions. Lapses sort on the capped value. A direction with no strength sorts after `solid`.
  - Audit `fields` is the list of contract paths present in the response. `word_count` counts distinct word ids in the response, `concept_count` counts grammar entries (32), `lesson_count` counts completed slugs, and `unknown_count` counts the tokens listed, at most 100. Every detail also carries `outcome`.
  - "Known" is the learner's library, meaning review-row words plus `user_words` words (ADR-0009 §6). A passed word is always in it, and so is a review-row word that has never been passed.
  - A vowel-stripped form must be unique across the lexicon's lemma and form keys and the learner's words together.
  - Tokens use the Hebrew character set of `HebrewUtils.tokenizeHebrew`, so maqaf, geresh, and gershayim stay in the word. Paseq, sof pasuq, and nun hafukha split. Maqaf and gershayim are trimmed from word edges, and a leading geresh is dropped. Invisible format characters are removed. ASCII `'` and `"` split. Runs of other letters or digits are non-Hebrew and do not count.
  - `coverage` is not rounded, because `known_count` and `running_words` already give it exactly. It is `null` when the text has no Hebrew word.
- **A shared test was edited by another agent.** At 9:17 PM, while SPEC-008 was running, `test/personal-plan.test.js` was changed. It had read `users.js` from `findAccount` to `findByProvider` and failed on this spec's JSDoc, which named the password column. It now stops at `findAccount`'s closing brace, and `findAccount`'s comment was reworded. This spec did not edit that test. Its own JSDoc now uses the same wording, so a source-slice check cannot trip on it again.

### Open items for the operator

1. **`strength` can be `null`.** Under `SRS_SCHEDULER=fsrs`, `model.wordStates` gives no retrievability for a row without stability. That covers rows the ladder wrote after migration 004, or never-passed migrated rows. The profile passes `null` through and sorts it after `solid`. ADR-0009 §1 names only four buckets. Suggested follow-up for SPEC-006: fall back to the ladder stability, as `stageOf` and `progress.cardForRow` already do. Ladder mode, today's default, is not affected. SPEC-010 and SPEC-017 should accept `null`.
2. **ADR-0009 §1 says coverage "never over-reports". The ADR-0001 A1 stripped-form rule it adopts can over-report.** A pointed token with different pointing (סַפָּר) counts as a known word (סֵפֶר) when the lexicon does not hold the other reading. The code follows the rule as written. Either limit the fallback to unpointed tokens (an ADR amendment), or drop that sentence.
3. **SEC-I-01.** Narrowing `recipeCounts` to shared snapshots plus the learner's own personal plans would remove the cross-plan id match. That is a change to SPEC-006 and the INIT-004 store.

### Notes for SPEC-010

- MCP adapters pass the tool's `username` with `audience: 'mcp'`. HTTP adapters pass `ctx.user.id` with `audience: 'self'`. Do not forward a request field as `now`.
- Each `checkCoverage` call reads the learner's words and the lexicon's lemma and form keys once. A caller scoring many texts in one request pays that cost per text, which is milliseconds.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
