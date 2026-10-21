# SPEC-004 · Service: FSRS scheduler and signal mapping

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-006
title: "Pure FSRS scheduler shared by browser and server; grade mapping in applySignals"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-002]
blocks: [SPEC-006, SPEC-009]
```

---

## Description

**Context:** `shared/player/review.js` is a UMD module used by the browser and required by `server/progress.js:14`. Its `nextState(record, signal, now)` is the ladder (`:74`–`:114`). `applySignals` validates `pass` / `miss` / `not_yet` and applies them server-side with server `now` (`server/progress.js:97`–`:120`). ADR-0009 §2 is the contract.

**Scope (in):**

- `shared/player/fsrs.js`: a UMD module in the same IIFE style as `review.js`, no dependency. Exports `initCard(now)`, `nextCard(card, grade, now, opts)` returning the new `{stability, difficulty, reps, state, due, lastGrade, lapses}`, `retrievability(card, now)`, and `previewIntervals(card, now)` (days for each of the four grades, for the button labels). Parameters and desired retention from ADR-0009 §2, as one frozen constant with a comment naming the version and URL.
- Same-day rule: a grade of Good or Easy on a card already reviewed on the same local day does not lengthen the interval beyond what a same-day review allows in FSRS short-term handling, and a lesson `pass` on the same day is recorded but does not advance (ADR-0009 §2; matches `review.js:98`–`:103`).
- `server/progress.js`: `applySignals` accepts an optional integer `grade` 1–4 on a signal. Mapping when absent: `pass`→3, `miss`→1, `not_yet`→no schedule change. `kind: 'form'` signals are logged with `lemma` and **do not** create or change a review row (forms feed SPEC-006 only). `suspended` rows are not scheduled.
- `SRS_SCHEDULER` (`fsrs` | `ladder`, default `ladder` until SPEC-020): `ladder` keeps today's `nextState` path for the schedule while still writing `grade`; `fsrs` uses `fsrs.js`. Read once at process start.
- `stageFor` reads stability when `SRS_SCHEDULER=fsrs` per ADR-0009 §2; ladder behavior unchanged otherwise.
- Header comment in `fsrs.js` telling a future editor: pure functions only, server `now` is authoritative, do not import from the DOM.

**Scope (out):** card selection by deck (SPEC-009); flashcard UI (SPEC-012); parameter optimization (out of initiative).

---

## Understood (agent's read of the intent)

- [x] und-1: Existing lesson activities keep sending `pass` / `miss` / `not_yet`. Nothing in `shared/activities/` changes.
- [x] und-2: The browser may call `previewIntervals` for labels; the server recomputes and is the only writer.

## Acceptance Criteria

- [x] ac-1 (REQ-003): golden tests: for a fixed `now` and a sequence of grades, `nextCard` produces the stability, difficulty, and interval values listed in a fixture computed from the pinned FSRS formulas (the fixture records the formula source).
- [x] ac-2 (REQ-003): a signal with `result: 'pass'` and no grade schedules as Good; `miss` as Again and increments lapses; `not_yet` leaves `due`, `stability`, `difficulty` unchanged.
- [x] ac-3 (REQ-003): two `pass` signals on the same local day advance once.
- [x] ac-4 (REQ-011): a `kind: 'form'` signal inserts a `practice_events` row with `kind='form'` and `lemma`, and creates no `review_items` row.
- [x] ac-5 (REQ-016): with `SRS_SCHEDULER=ladder`, every existing `test/*review*` and progress test passes unchanged.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Retire `review.js` | No. It keeps `keyFor`, `dueList`, and the ladder for the fallback. `fsrs.js` is beside it. | resolved | yes — fallback is REQ-016 |
| aud-2 | Grade from lesson activities | Activities never send Hard or Easy. Only flashcards do. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/player/fsrs.js`
- [x] Changes to `server/progress.js`
- [x] Tests: `test/fsrs.test.js`, `test/progress-grades.test.js`

---

## Technical Requirements (domain-specific)

- [x] No `Date.now()` inside pure functions; `now` is a parameter.
- [x] Floating point compared with a tolerance in tests.
- [x] `fsrs.js` added to the static allowlist the shell uses for `shared/player/*.js`, if that list is explicit. (Not explicit: `server/web.js` serves all of `shared/` by prefix, so nothing needed adding.)

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/fsrs.test.js test/progress-grades.test.js` and `npm test`.
- **Rollback plan:** `SRS_SCHEDULER=ladder`; revert the module.

---

## Security

- **Sensitivity:** standard. Input validation on `grade` (integer 1–4, else the signal is rejected and counted in `rejected`).
- **Review depth:** standard · **focus areas:** signal validation.

---

## Verification Strategy

- **Claim:** FSRS math matches the pinned formulas, lesson signals map as ADR-0009 says, and the ladder fallback is intact.
- **Check + executor:** `mechanized` — the two test files and the full suite under both scheduler values.
- **Pass condition:** exit 0 for `SRS_SCHEDULER=ladder npm test` and `SRS_SCHEDULER=fsrs node --test test/fsrs.test.js test/progress-grades.test.js`.
- **Test levels:** unit (pure module), integration (temporary database).

---

## Integration Points

- SPEC-006 calls `retrievability`. SPEC-009 returns `previewIntervals` with each card. SPEC-012 shows them.
- Produces **Change:** scheduler.
- **Provenance tag:** `INIT-006/SPEC-004`
