# SPEC-013 · Frontend: Server-backed progress client

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-002
title: Server-backed progress client
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-009, SPEC-012]
blocks: [SPEC-014]
```

---

## Description

**Context:** `shared/player/review.js` (`HebrewReview`) keeps the review store in localStorage `hebrew-review:v1`. `shared/player/core.js` saves lesson rounds under `hebrew-worksheet:<slug>`. Activities call `ctx.review.record(...)`, and the review page uses `HebrewReview.load()` and `dueList`. REQ-004 and REQ-005 make the server the store, with each account starting fresh. The legacy keys are neither read nor deleted.

**Scope (in):**
- **`shared/player/sync.js`** (`window.HebrewSync`).
  - `init()` gets `/api/me`, then `GET /api/review/state`, then the current lesson's progress.
  - `queueSignal(signal)` gives each signal an id (`crypto.randomUUID`), keeps it in a per-user pending queue in localStorage (`hebrew-sync:v1:<userId>`), and flushes in batches of up to 200 with retry and backoff.
  - The server's returned records replace local copies.
  - It flushes on `visibilitychange`/`pagehide` (via `fetch` with `keepalive`).
  - It reports `synced`, `pending`, or `error` to the SPEC-012 top-bar slot ("not saved yet — will retry" in Hebrew with English).
- **`shared/player/review.js`**: add a backend adapter.
  - When `HebrewSync` is active, `load()` returns the server state, and `record()` updates memory and calls `queueSignal`.
  - The existing pure functions are unchanged.
  - Neither `hebrew-review:v1` nor the legacy `wordMarks` migration is touched while signed in.
- **`shared/player/core.js`**: load lesson progress from the server at boot, and save through a debounced `PUT` with `rev`.
  - On 409, adopt the server copy and re-render the path screen.
  - `hebrew-worksheet:<slug>` is not read or written while signed in.
- **`shared/worksheet.js`** and **`shared/review-page.js`**: wait for `HebrewSync.init()` before `HebrewPlayer.boot()` or the review session renders. Data globals still load by script tag.

**Sizing (frontend band):**
- Screens: 0 (no new screen).
- Components: 2 (sync module, top-bar indicator).
- States: 5 (loading, synced, pending, error, conflict).

This is below the floor, so it stays one spec.

**Scope (out):** importing old browser progress (operator chose a fresh start); new pages (SPEC-014/015).

---

## Understood (agent's read of the intent)

- [ ] und-1: The browser never computes the authoritative schedule. It shows what the server returns, and it shows due words from the server state using the existing `dueList`.
- [ ] und-2: Two people sharing a browser never see each other's pending data, because the queue key includes the user id.

## Acceptance Criteria

- [ ] ac-1 (REQ-004): marking a word in a lesson on browser profile A, then signing in as the same user on a fresh profile B, shows the word's updated schedule on Review in B.
- [ ] ac-2 (REQ-004): signed in as a fresh account on a browser that has legacy `hebrew-review:v1` and `hebrew-worksheet:<slug>` data, the account shows no progress. Both legacy keys are byte-identical after a session.
- [ ] ac-3 (REQ-005): with the network blocked, answers are queued and the top bar shows the pending state. When the network is restored, the queue flushes once (no duplicates on the server) and the indicator returns to synced.
- [ ] ac-4: user A's pending queue is not sent or shown when user B signs in on the same browser.
- [ ] ac-5: a 409 on lesson progress adopts the server copy without throwing, and the lesson stays playable.
- [ ] ac-6 (REQ-014): no `fetch` is made for lesson or vocab data. Every new `fetch` targets `/api/`, which the smoke test asserts by recording request URLs.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Queue cap | at most 1,000 pending signals per user; oldest dropped beyond that with a console warning | resolved | yes — operator confirmed 2026-10-08 |

---

## Deliverables

- [ ] `shared/player/sync.js`; `shared/player/review.js` (backend adapter); `shared/player/core.js`; `shared/worksheet.js`; `shared/review-page.js`
- [ ] Tests:
  - `test/progress-sync.test.js`: fake `HebrewApi` and fake localStorage, covering queueing, retry, idempotency, per-user keys, and 409.
  - `test/chrome-progress-smoke.js`: two browser profiles, a network block, and a request-URL log.

---

## Technical Requirements (domain-specific)

- [ ] The existing `test/review-scheduler.test.js` and `test/review-store.test.js` keep passing unchanged.
- [ ] Time is injected in the sync tests, and there are no real timers in unit tests.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; `node test/chrome-progress-smoke.js`.
- **Rollback plan:** revert the listed files. The legacy localStorage path is untouched, so it still exists.

---

## Security

- **Sensitivity:** light. Triggers matched: none beyond the per-user cache key (ac-4).
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** progress follows the account across browsers, survives a lost connection without duplicates, never mixes users, and leaves legacy browser data untouched.
- **Check + executor:** `mechanized`. `test/progress-sync.test.js` (unit) and `test/chrome-progress-smoke.js` (headless Chrome, two profiles, real server, temp DB).
- **Pass condition:**
  - Smoke: profile B sees profile A's update; the event count on the server equals the number of answers given while offline; legacy keys are unchanged; and every `fetch` URL starts with `/api/`.
  - Unit tests pass.

---

## Integration Points

- Consumed by SPEC-014 (Today and Lessons read state through `HebrewSync`).
- **Provenance tag:** `INIT-002/SPEC-013`
