# SPEC-006 Completion Summary — Learner model

**Traceability:** INIT-006/SPEC-006
**Status:** complete (orchestrator closed 2026-10-10 — `node --test test/learner-model.test.js` → 22/22)
**Domain:** service
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/learner/model.js`, `server/learner/thresholds.js`
- [x] One call from `server/progress.js` `applySignals` (form `pass` / `miss` only)
- [x] Tests: `test/learner-model.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-006): fixture rows with chosen lapses, difficulty, and due dates produce the `weak` / `very_weak` flags the ADR-0009 §3 table predicts, one test per row of that table.
- [x] ac-2 (REQ-006): a `form` signal on a tagged past-tense form updates exactly the four concept rows; on an untagged form, none. A word-card `pass` or `miss` on the same lemma updates none. A concept with 1 to 4 attempts has status `emerging`.
- [x] ac-3 (REQ-006): `rebuildConceptMastery` after a sequence of signals equals the incremental result.
- [x] ac-4 (REQ-006): journey stage boundaries: one fixture per boundary in ADR-0009 §5 (49 and 50, 149 and 150, 499 and 500, 1499 and 1500 words long-term in `he_en`).

## AC Verification (independent)

Independent verification was **not** dispatched. This run is a subagent and did not spawn a second verifier. The table below is the implementer's test evidence, not an independent verdict. Spec status stays `in_review` until a fresh verifier records all-PASS.
Overall: **UNVERIFIED** · remediation loops: 0.
**Full eval report:** not written — no independent verifier ran.

**Verification tier:** `full` · **pre-flight:** not classified light by the orchestrator (no light-tier line in the dispatch) · **checks skipped:** `None` (tests were run by the implementer; the independent re-run was not)

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | UNVERIFIED (implementer run passed) | Tests `ac-1 W1` through `ac-1 W6` in `test/learner-model.test.js`. W2 is long-term with 3 lapses and is not flagged. W5 is weak only while `SRS_SCHEDULER=fsrs`. `node --test test/learner-model.test.js` → 22 pass, 0 fail, exit 0. |
| ac-2 | UNVERIFIED (implementer run passed) | Test `ac-2: a tagged form miss updates four concepts…` and `ac-2: a form binyan overrides the lemma, and 1 to 4 attempts stay emerging`. Word-card, omitted kind, `not_yet`, untagged, and a missing lexicon key leave mastery empty. Four attempts stay `emerging`. Five passes become `strong`. Judged edges 0.49 through 0.90 are in `ac-2: five attempts leave emerging…`. |
| ac-3 | UNVERIFIED (implementer run passed) | Test `ac-3: rebuildConceptMastery matches the incremental fold, and retags from the lexicon`. Stored `ewma` matches α from `thresholds.js`. Clearing tags and rebuilding drops the rows; restoring tags restores them. Another learner's event is not folded into this learner. |
| ac-4 | UNVERIFIED (implementer run passed) | Eight tests `ac-4: N he_en long-term words is <stage>` for 49 `script`, 50 and 149 `foundations`, 150 and 499 `building`, 500 and 1499 `expanding`, 1500 `independent`. The 49 fixture also has one `en_he` long-term word and stays `script`. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-006`
- **Title:** Derive word strength, weak flags, concept mastery, and journey from the learner's own rows
- **Files changed:** 8 · the model, the threshold module, the `applySignals` call, four repository reads, and the test
- **Change status:** proposed
- **Reversibility:** branch. Rollback is to remove the `applySignals` call. The model does not migrate the database. `concept_mastery` rows written after this call can be deleted per user.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/learner-model.test.js` | 22 | 22 |

**All tests passing:** Yes · `node --test test/learner-model.test.js` → 22/22, exit 0 (re-run after the recipe-slug guard) · `node --test test/learner-model.test.js test/progress-grades.test.js test/progress-service.test.js` → 52/52, exit 0 · `npm test` → 737/737, exit 0, before the recipe-slug guard (that guard is covered by the journey test in the 22/22 re-run) · **Coverage (new code):** no coverage collector is configured in this repo

Ladder dues in `test/progress-service.test.js` were not edited. The default scheduler is still ladder.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | `node --test test/learner-model.test.js` and `npm test` | Both exited 0. No `gate_actions`. No migration. No new npm dependency. |

None — branch/reversible only.

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** inline review by the implementer (`self_reviewed: true`). A separate security-review subagent was not spawned. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** Learner reads and writes bind `user_id` (`listForLearner`, `listByUser`, `listFormOutcomes`, `listByKeys`, `deleteByUser`, mastery upsert). Lexicon `IN` lists use placeholders only. `recordConceptEvent` returns immediately unless `kind` is `form` and the result is `pass` or `miss`, and `applySignals` calls it only on that path, after the duplicate-id check. Concept keys that are not in `grammar_concepts` are dropped, so a bad tag cannot fail the practice-event transaction through the foreign key. Recipe ids copied from a published snapshot must match `^[a-z0-9-]{1,64}$` before they become keys. The model does not select account secrets, and it does not return a numeric user id.

---

## Files Changed

Provenance is `INIT-006/SPEC-006` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/learner/thresholds.js` | Created — ADR-0009 §2–§5 numbers in one module | `INIT-006/SPEC-006` |
| `server/learner/model.js` | Created — `wordStates`, `recordConceptEvent`, `conceptStates`, `journey`, `rebuildConceptMastery` | `INIT-006/SPEC-006` |
| `server/progress.js` | Modified — `applySignals` calls `recordConceptEvent` for a form pass or miss | `INIT-006/SPEC-006` |
| `server/db/repos/review.js` | Modified — `listForLearner` (one query, pass-event flag, user scoped) | `INIT-006/SPEC-006` |
| `server/db/repos/events.js` | Modified — `listFormOutcomes` for rebuild | `INIT-006/SPEC-006` |
| `server/db/repos/lexicon.js` | Modified — `listByKeys` batched `IN` | `INIT-006/SPEC-006` |
| `server/db/repos/concept-mastery.js` | Modified — `listByKeys`, `deleteByUser` | `INIT-006/SPEC-006` |
| `test/learner-model.test.js` | Created — one test per weak-flag row and per journey boundary | `INIT-006/SPEC-006` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 sentinel | `trace_open`, `trace_complete` | Orchestrator owns the timer and the run-trace for a dispatched spec. This subagent did not call `open_trace` / `emit` / `close_trace`. | No events from this process in `.sdd-trace/SPEC-006.jsonl` unless the orchestrator writes them. | A Tier-5 check against an empty or missing file stays INCONCLUSIVE or FAIL until the orchestrator closes the trace. This is the ownership rule, not a failed emit. |

---

## Deviations from the Spec

- Commit, push, and PR were not done. The dispatch said not to commit.
- `.claude/agents/` is not in this repo, so `principal-backend-developer` was not loaded (`agents_dir_absent`). The work followed the spec's service rules: user-scoped queries, batched lexicon lookups, thresholds in one module.
- Repository helpers (`listForLearner`, `listFormOutcomes`, `listByKeys`, `deleteByUser`) are not named in the deliverables. They keep the SQL in the data layer and avoid a query per word.
- `journey(userId, now)` accepts `now` and does not read it. Stage is step or stability, which do not move with the clock. Strength and `due_in_days` do use `now`, inside `wordStates`.
- Ladder remains the default. `test/progress-service.test.js` was not edited.
- Plan schema and validator were not touched. No HTTP or MCP routes.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
