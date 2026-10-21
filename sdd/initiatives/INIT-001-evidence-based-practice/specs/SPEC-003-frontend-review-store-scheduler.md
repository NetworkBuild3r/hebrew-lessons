# SPEC-003 · Frontend: Review store and scheduler

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-001
title: Review store and scheduler
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4.5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005, SPEC-006, SPEC-007]
```

---

## Description

**Context:** progress today is per lesson (`hebrew-worksheet:<slug>` with `wordMarks`, `missedLast`, rounds). Spacing needs per-word, cross-lesson state (`RSCH-003`, High). This spec adds the browser-side store and the pure scheduling logic; it has no screen of its own.

**Scope (in):**
- `shared/player/review.js` exposing `window.HebrewReview` (and `module.exports` when loaded in Node for tests):
  - pure: `keyFor(word)`, `nextState(record, signal, now)`, `isDue(record, now)`, `dueList(records, now, caps)`;
  - store: `load()`, `record(word, direction, signal, meta)`, `get(word)`, `all()`, `migrateLessonMarks(lessonState, ws)`; storage key `hebrew-review:v1`; fail-soft on quota or parse errors.
- Load it from `shared/worksheet.js` before activities, and expose `ctx.review` to activities.
- Wire the existing `words` self-marks (`knew` / `missed`) to `record()` in the `he→en` direction.
- One-time migration of existing per-lesson `wordMarks` (`missed` → due now; `knew` → step 1).

**Scope (out):** review page UI (SPEC-004), new recall modes (SPEC-005), feedback changes (SPEC-006), glosses (SPEC-007).

**Sizing (frontend band):** 0 screens, 1 new module + 2 touched (`worksheet.js` loader, `words.js`), states: loaded / empty / storage-unavailable — below the floor; one spec.

---

## Understood (agent's read of the intent)

- [x] und-1: Same-day repeats never advance a word (REQ-001).
- [x] und-2: The per-lesson round system keeps working unchanged; the store is additive.

## Acceptance Criteria

- [x] ac-1 (REQ-001/002): `nextState` follows the ADR-0001 ladder: pass advances one step, miss resets to step 1, a second pass on the same calendar day does not advance — covered: yes
- [x] ac-2: `dueList` returns overdue items oldest-first, respects the due and new-word caps, and interleaves lessons (no more than 2 consecutive items from one lesson when others are due) — covered: yes
- [x] ac-3: marking "Knew it" / "Missed it" in the `words` activity creates or updates the word's `he→en` record — covered: yes
- [x] ac-4: existing `wordMarks` in localStorage migrate once; re-running migration is a no-op — covered: yes
- [x] ac-5 (REQ-009): with localStorage throwing on `setItem`, lessons still run and no uncaught error reaches the console — covered: yes
- [x] ac-6 (REQ-009): no lesson `index.html` is modified; no network request is added — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Interleave rule | at most 2 consecutive items from one lesson when other lessons have due items | resolved | yes — orchestrator 2026-10-08 (mode-e run; matches ADR-0001 §4) |

---

## Deliverables

- [x] `shared/player/review.js`
- [x] `shared/worksheet.js` (load order), `shared/player/core.js` (`ctx.review`), `shared/activities/words.js` (record self-marks)
- [x] Tests: `test/review-scheduler.test.js` (pure functions with a fake clock), `test/review-store.test.js` (fake localStorage, quota error, migration)

---

## Technical Requirements (domain-specific)

- [x] Plain browser JS in the existing IIFE style; no build step, no dependencies.
- [x] Time injected (`now` parameter) for testability; calendar-day comparison in local time.
- [x] NFC normalization matches `speech.js`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; `node test/chrome-audio-smoke.js` still passes.
- **Rollback plan:** revert the listed files; the `hebrew-review:v1` key is ignored by older code.

---

## Security

- **Sensitivity:** light — triggers matched: none (local, single-user browser storage).
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** the scheduler implements ADR-0001 exactly, and the store never breaks a lesson when storage misbehaves.
- **Check + executor:** `mechanized` — `npm test` runs the two new test files; `node test/chrome-audio-smoke.js` loads `my-morning`, marks a word, and reads `hebrew-review:v1` back.
- **Pass condition:** all ladder, same-day, cap, interleave, migration, and quota tests pass; the smoke check finds the marked word's record with the expected due date; `npm test` exits 0.

---

## Integration Points

- Consumed by SPEC-004 (due list, recording results), SPEC-005/006/007 (recording signals).
- Produces **Change:** new review module and the `words` self-mark hook.
- **Provenance tag:** `INIT-001/SPEC-003`
