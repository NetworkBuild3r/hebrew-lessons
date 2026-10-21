# SPEC-005 · Service: Server-side progress and metrics

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-002
title: Server-side progress and metrics
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-003]
blocks: [SPEC-009, SPEC-010]
```

---

## Description

**Context:** ADR-0001's scheduler (`shared/player/review.js`: `keyFor`, `nextState`, `isDue`, `dueList`) runs in the browser against localStorage. REQ-004 and REQ-005 move the state to the server per user, with the server as the authority. `RSCH-003` rates spacing across days High. Recall direction is Moderate, and the operator chose memory stages and direction accuracy as the learner metrics (REQ-007), plus trouble words and the full answer history for admins (REQ-009, operator decision: admins have a full view).

**Scope (in):**
- `server/progress.js`:
  - **`applySignals(userId, signals, now)`.**
    - Each signal is `{ id, word, direction, result, at, slug, activity }`.
    - The function de-duplicates by (`user_id`, `id`) and resolves the word key with `HebrewReview.keyFor`.
    - It applies `nextState` using **server** `now`, not the client's `at`, and upserts `review_items`.
    - It appends a `practice_events` row (`at` is kept, but only for the log) and adds `slug` to the word's sources.
    - The whole batch is one transaction. It returns the updated records.
  - **`reviewState(userId)`** returns the user's records in the ADR-0001 store shape (`{ v:1, words:{…} }`), so the browser's existing `dueList` and page code work unchanged.
  - **`getLessonProgress(userId, slug)`** and **`putLessonProgress(userId, slug, state, rev)`**.
    - The state is the existing `hebrew-worksheet:<slug>` shape.
    - It is validated against an allowlist of keys and capped at 64 KB.
    - It has optimistic revision checks; a stale `rev` is refused with the current copy.
  - **`metrics(userId, now)`** returns `{ stages: { he_en: {new, learning, longTerm}, en_he: {…} }, accuracy: { he_en: {pass, miss, pct|null}, en_he: {…} } }`, using ADR-0002 item 10 definitions.
  - **`troubleWords(userId, now, limit=10)`**, for the admin view.
  - **`answerHistory(userId, { before, limit })`**, for the admin full view (REQ-009). It returns the user's `practice_events`, newest first, with the word's meaning and lesson title looked up from lesson data, paged by an opaque cursor (`before`) with a limit of 1 to 200 (default 50).
- Load `shared/player/review.js` in Node through its existing `module.exports` path. Do not copy the ladder.

**Scope (out):** HTTP routes (SPEC-009/010); lesson visibility checks (callers pass only visible slugs; SPEC-009 enforces); browser changes (SPEC-013).

---

## Understood (agent's read of the intent)

- [ ] und-1: There is one ladder implementation, the one in `shared/player/review.js`. The server calls it; it does not reimplement it.
- [ ] und-2: Server time decides `due`, so a wrong or tampered client clock can't jump a word forward.

## Acceptance Criteria

- [ ] ac-1 (REQ-005): a pass then a same-day pass advances one step only. A miss resets to step 0, due in one day. This is the same result as calling `HebrewReview.nextState` directly; the test compares both.
- [ ] ac-2 (REQ-005): submitting the same batch twice (same signal ids) changes nothing the second time, so the event count and step stay the same.
- [ ] ac-3 (REQ-004): user A's signals never change user B's records, events, or metrics.
- [ ] ac-4 (REQ-007): with a fixture of known steps, `metrics` returns the expected new, learning, and long-term counts per direction. Accuracy excludes `not_yet` and returns `pct: null` below 10 attempts.
- [ ] ac-5 (REQ-009): `troubleWords` orders by misses in the last 30 days, then by lapses, limited to 10.
- [ ] ac-5b (REQ-009): `answerHistory` returns only that user's events, newest first. Paging with the returned cursor yields every event exactly once with no overlap. A limit above 200 is clamped.
- [ ] ac-6: `putLessonProgress` rejects unknown keys and bodies over 64 KB. A stale `rev` returns a conflict with the stored state.
- [ ] ac-7: a signal with an unknown `direction` or `result`, or an empty `word`, is rejected without aborting the valid rest of the batch, and the rejection count is returned.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Bad signals in a batch | skip and count them; apply the valid ones | resolved | yes — plan default |
| aud-2 | Stage of a word never passed in a direction | `new` for that direction | resolved | yes — per ADR-0002 item 10 |

---

## Deliverables

- [ ] `server/progress.js`
- [ ] Tests: `test/progress-service.test.js`, `test/progress-metrics.test.js` (temp `DATA_DIR`, injected `now`)

---

## Technical Requirements (domain-specific)

- [ ] Reuse `shared/player/review.js` via `require`. If it needs a small export tweak to load in Node without `window`, keep its browser behavior identical and covered by the existing `review-*` tests.
- [ ] Metrics queries use the (`user_id`, `at`) and (`user_id`, `due`) indexes from SPEC-003.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`, including the existing `review-scheduler` and `review-store` tests unchanged.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** standard. Triggers matched: per-user data isolation, external input (signal batches).
- **Review depth:** standard. Security acceptance: ac-3 and ac-7.

---

## Verification Strategy

- **Claim:** server-side scheduling is identical to ADR-0001, idempotent, isolated per user, and the metrics match their ADR-0002 definitions.
- **Check + executor:** `mechanized`. The two new test files, plus the existing `test/review-*.test.js`.
- **Pass condition:** all tests pass, and the equivalence test (server result equal to the direct `nextState` result) passes for the pass, miss, same-day, and `not_yet` cases.

---

## Integration Points

- Consumed by SPEC-009 (learner endpoints) and SPEC-010 (admin progress view).
- **Provenance tag:** `INIT-002/SPEC-005`
