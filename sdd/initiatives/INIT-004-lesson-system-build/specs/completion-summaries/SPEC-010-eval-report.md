# SPEC-010 AC Verification Eval Report — Personal-plan assignee checks

**Traceability:** INIT-004/SPEC-010 \
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-09T18:15:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Audience and assigneeId go through `ops.setField` or `replacePart`. A spy shows `personal.js` wrote no plan SQL. | PASS | `assign` calls `ops.replacePart` once (`server/plans/personal.js:74-78`). Test `ac-1: audience and assigneeId go through replacePart, and personal.js writes no plan SQL`: stubbed `replacePart` sees 1 call and `db.run` count 0; the live path has `setField` count 0 and one `plans` write from ops. `node --test test/personal-plan.test.js` → 4 pass, 0 fail, exit 0. |
| ac-2 | Disabled or unknown assignee is `assignee_refused`; revision and document stay put. | PASS | `activeAccount` returns null unless `status === 'active'` (`server/plans/personal.js:19-26`); refusal returns before `replacePart` (`68-69`). Test `ac-2: a disabled or unknown assignee is refused and the plan is unchanged`: `{ error: 'assignee_refused', revision: 1 }` for `missing01` and `disabled1`; stored revision, document, and `updatedAt` match the seed. |
| ac-3 | `shared` leaves `assigneeId` null and the plan validates. | PASS | Shared branch sets both fields then one `replacePart` (`server/plans/personal.js:64-66`). Test `ac-3: shared sets assigneeId null in one operation and validates`: spy value is `audience: 'shared'`, `assigneeId: null`; `validatePlan(shared.plan).ok === true`; stored assignee is null. |
| sec-1 | No assignment to a disabled or missing account. | PASS | Same path and test as ac-2. |
| sec-2 | No plan SQL in this module, so revision checks are not bypassed. | PASS | No plan SQL in `personal.js`. `expectedRevision` is passed to `replacePart`. Test `a stale revision does not assign, and progress is not read`: `expectedRevision: 0` → `{ error: 'revision_conflict', revision: 1 }`, document unchanged. |
| und-1 | No learner-progress reads. | PASS | Source has none of `review_items`, `practice_events`, `lesson_progress`. Same test's static scan. |
| und-2 | No AI provider call. | PASS | Source has no `openai` or `fetch(`. Same static scan. |
| tech | Account read does not select `password_hash` or `display_name`. | PASS | `findAccount` is `SELECT id, username, status FROM users WHERE username = ?` (`server/db/repos/users.js:57-59`). Test parses that function body. |

## Definition of done

- [x] `node --test test/personal-plan.test.js` exits 0 — verifier re-ran it: 4 pass, 0 fail.
- [x] Spy records zero direct plan writes from `personal.js` — stubbed `replacePart` leaves `db.run` at 0.
- [x] Refused assignee leaves the revision unchanged — ac-2 compares the stored row to the seed.

## Tests run by the verifier

```text
node --test test/personal-plan.test.js
→ tests 4, pass 4, fail 0, todo 0, exit 0
```

---

## Remediation log (feedback loop)

None. The first pass was all-PASS.

---

## Notes

- `assign` uses `replacePart` only. The criterion allows `setField` or `replacePart`. One call sets both fields, which is what "the same operation" requires for `shared`.
- No AC was passed by weakening a test or the criterion.
