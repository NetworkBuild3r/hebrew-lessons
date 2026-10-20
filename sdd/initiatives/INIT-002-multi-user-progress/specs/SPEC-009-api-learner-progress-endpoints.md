# SPEC-009 · API: Learner progress and metrics endpoints

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-002
title: Learner progress and metrics endpoints
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3.5 hours
blocked_by: [SPEC-005, SPEC-008]
blocks: [SPEC-013, SPEC-015]
```

---

## Description

**Context:** the browser needs to read and write the signed-in learner's own progress (REQ-004/005) and read their metrics (REQ-007). Every route acts on the session's user only. There is no user id in the URL, so one learner can't address another's data.

**Scope (in):**
- `server/api/progress-routes.js`, registered on SPEC-008's router with `requireSession`:
  - `GET /api/review/state` returns `progress.reviewState(me)` (ADR-0001 store shape), limited to words whose sources include at least one lesson the user can see.
  - `POST /api/review/signals` takes `{ signals: [...] }` (≤ 200 items) and returns the updated records plus a rejected count. Each signal's `slug` must be visible to the user, or that signal is rejected.
  - `GET /api/lessons/:slug/progress` and `PUT /api/lessons/:slug/progress` with `{ state, rev }`. An invisible slug gets 404. A stale `rev` gets 409 with the stored state.
  - `GET /api/me/metrics` returns `progress.metrics(me)`.

**Scope (out):** admin views of other users (SPEC-010); browser client (SPEC-013).

---

## Understood (agent's read of the intent)

- [ ] und-1: Learner routes take no user id parameter at all.
- [ ] und-2: A lesson hidden from the user is treated as not existing, on reads and on writes.

## Acceptance Criteria

- [ ] ac-1 (REQ-004): user A's session can't read or change user B's review state, lesson progress, or metrics through any route or body field. A test tries injecting `userId` in the body and query.
- [ ] ac-2 (REQ-005): posting the same signal batch twice leaves the state as after the first post, and the response reports zero applied the second time.
- [ ] ac-3 (REQ-010): signals naming a slug the user can't see are rejected. `GET`/`PUT` on that slug's progress returns 404.
- [ ] ac-4: a batch of 201 signals or a body over 64 KB is refused with 413 and changes nothing.
- [ ] ac-5: a stale `rev` on `PUT` returns 409 with the stored state; a fresh `rev` succeeds and increments it.
- [ ] ac-6 (REQ-007): `GET /api/me/metrics` returns the stage and accuracy shape from SPEC-005, with `pct: null` under 10 attempts.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Review state for words whose only source became hidden | omitted from state and due list; kept in the database (restored if shared again) | resolved | yes — plan default |

---

## Deliverables

- [ ] `server/api/progress-routes.js` (registered in `server/web.js`)
- [ ] Tests: `test/api-progress.test.js` (live server, two users, one restricted lesson)

---

## Technical Requirements (domain-specific)

- [ ] Use SPEC-008's body limit, JSON errors, and `no-store`.
- [ ] Use SPEC-006's `canSeeLesson` for every slug.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** standard. Triggers matched: per-user data access (IDOR surface), external input.
- **Review depth:** standard. Security acceptance: ac-1, ac-3, and ac-4.

---

## Verification Strategy

- **Claim:** a learner can read and write only their own progress, only for lessons they can see. Writes are idempotent and bounded.
- **Check + executor:** `mechanized`. `node --test test/api-progress.test.js` runs against a live server.
- **Pass condition:** all tests pass. The two-user isolation test and the hidden-lesson tests each fail if `requireSession` or `canSeeLesson` is removed from the route; the test file includes that negative check by asserting the guarded status codes.

---

## Integration Points

- Consumed by SPEC-013 (progress client) and SPEC-015 (Progress page).
- **Provenance tag:** `INIT-002/SPEC-009`
