# SPEC-004 AC Verification Eval Report — Plan HTTP routes

**Traceability:** INIT-004/SPEC-004 \
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) · [Verify SPEC-004 criteria](391df52d-43c1-450b-a55d-4451159515d8) \
**Evaluated at:** 2026-10-09 \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Each write route calls the matching function and no other edit implementation. A test spies on the export. | PASS | Handlers call `ops.replacePart`, `ops.insertBlock`, `ops.moveBlock`, `ops.removeBlock`, `ops.setField`, and `publish.publishPlan` by property (`server/api/plan-routes.js`). Test `ac-1: each write route calls only its function`: `calls` equals `[route.fn]` and revision becomes 2. Test `ac-1: GET parts calls readPart and does not write`. Verifier: `node --test test/plan-routes.test.js` → 9 pass, 0 fail, exit 0. |
| ac-2 | A stale revision is HTTP 409 with `revision_conflict` and the stored revision, and the stored document is unchanged. | PASS | `sendMapped` writes `{ error: 'revision_conflict', revision }` at 409 (`server/api/plan-routes.js`). Test `ac-2: a stale revision is 409 revision_conflict and the document is unchanged`: body `{ error: 'revision_conflict', revision: 1 }`, document, revision, and `updatedAt` unchanged, snapshot still null. |
| ac-3 | A signed-in user who is not `assigneeId` receives 404 for a personal plan and for a `selected` lesson they are not granted. The assignee receives the snapshot. A signed-out caller receives 401. | PASS | `mayReadPublished` allows a personal snapshot only when `assigneeId === user.username`, and a shared snapshot only when every lesson is `everyone` or `selected` with a grant (`server/api/plan-routes.js`). Tests `ac-3: personal snapshot, selected grant, and signed-out 401` and `ac-3: draft, hidden, and a later edit stay off the learner read`. 404 bodies omit `newWords` and `assigneeId`. Assignee and a fully granted learner receive the snapshot. Signed-out GET is 401 `no_session`. |
| ac-4 | A non-admin session receives 403 on every write route. Existing worksheet routes still behave (`npm test`). | PASS | Every write is `requireSession(requireAdmin(...))`. Test `ac-4: a non-admin session is 403 on every write route and worksheet admin routes still gate`: six writes return 403 `forbidden` and the spy list stays empty. `GET /api/admin/users` is still 403 for a learner and 200 for an admin. Isolated `npm test` → 476 tests, 474 pass, 0 fail, 2 todo, exit 0, including `web-gating.test.js` and `http-security.test.js`. |
| sec-1 | Every write checks the admin role on the server. | PASS | Same `addAdmin` wrapper and ac-4 test. |
| sec-2 | Personal, hidden, draft, and ungranted reads are 404 and do not echo the plan. | PASS | Denials use `sendJsonError(404, 'Not found', 'not_found')`. Tests assert the body does not contain `newWords`, `assigneeId`, or lesson Hebrew. A missing id and a hidden plan return the same text. |
| sec-3 | CSRF and Origin checks match the existing mutating `/api/` routes. | PASS | `server/web.js` `handleApi` rejects a mutating method without JSON content-type or an allowed Origin before `router.handle`. `plan-routes.js` does not parse cookies or reimplement that check. Test `security: a write without Origin is the existing CSRF 403 and does not call ops`. |

## Definition of done

- [x] `node --test test/plan-routes.test.js` exits 0 — verifier re-ran it: 9 pass, 0 fail.
- [x] Spy assertions name each route's function — ac-1.
- [x] 404 bodies do not contain `newWords` or `assigneeId` — ac-3.

## Tests run by the verifier

```text
node --test test/plan-routes.test.js
→ tests 9, pass 9, fail 0, exit 0

npm test
→ tests 476, pass 474, fail 0, todo 2, exit 0
```

The implementer re-ran `npm test` alone after a concurrent run collided with another spec's in-progress import test. The isolated run matched the verifier: 474 pass, 0 fail, 2 todo, exit 0.

---

## Remediation log (feedback loop)

None. First verifier pass was all-PASS. Loops used: 0 of 3.
