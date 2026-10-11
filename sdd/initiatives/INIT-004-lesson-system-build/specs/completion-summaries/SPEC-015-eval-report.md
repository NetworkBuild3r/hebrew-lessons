# SPEC-015 AC Verification Eval Report — Builder outline and block editors

**Traceability:** INIT-004/SPEC-015 \
**Verifier:** independent readonly sub-agent · model `composer-2.5-fast` · temperature 0 \
**Evaluated at:** 2026-10-09T18:55:44Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Insert, move, remove, field save, and publish call the SPEC-004 routes and do not patch the plan document locally | PASS | Test `ac-1: insert, move, remove, field save, and publish call the plan routes`. Writes are `PATCH /api/plans/beginner-unit-1/fields`, `POST …/blocks/move`, `DELETE …/parts`, `POST …/blocks`, `POST …/publish`. Bodies have no `plan` or `units`. `moveDestination` and the move body send index `1` for the first block moving down. `shared/admin/plan-builder.js` publish, move, remove, insert, and saveField. `node --test test/plan-builder.test.js` 7 pass, exit 0. |
| ac-2 | A 409 leaves the draft in the field and shows the stored revision | PASS | Test `ac-2: a 409 keeps the textarea and names revision_conflict plus the stored revision`. Textarea stays `ADMIN DRAFT`. Conflict text matches `revision_conflict` and `9`. No extra GET after the PATCH. A later publish still sends `expectedRevision: 3`, not the DOM text `999` and not `9`. `showConflict` in `shared/admin/plan-builder.js`. |
| ac-3 | A non-admin session does not receive the builder markup | PASS | Test `ac-3: a non-admin session does not get the builder from the admin shell`. `plan-builder.js` is only on the `adminOnly` route. `spa.renderShell` for role `user` has no builder markup. Router start as `{ role: 'user' }` on `/admin/` sets `No access` and does not mount the shell. `server/web.js` still 403s `/admin/` when `auth.user.role !== 'admin'`. |
| ac-4 | The legacy `data-lesson-save` hook still exists | PASS | Test `keeps the legacy save hook and does not post to upsert_lesson`. `shared/admin/lesson-editor.js` still contains `data-lesson-save`. `plan-builder.js` does not, and it does not mention `upsert_lesson` or `/api/admin/lessons`. |

## Definition of done

- [x] Admin builder view for outline and fields — `HebrewPlanBuilder.mount` / `mountPicker`, admin hash `#plans` and `#plan/<slug>`
- [x] Tests `test/plan-builder.test.js` — 7 pass, exit 0
- [x] Preview uses `HebrewPlanRender.renderBlock` on the lesson part from `GET /api/plans/:id/parts`
- [x] Legacy worksheet editor remains; this page does not post a worksheet save

## Tests run by the verifier

```text
node --test test/plan-builder.test.js
→ 7 passed / 7 total ; exit 0
```

The implementer's `npm test` (495 pass, 0 fail, 2 todo, exit 0) was not re-run by the verifier, so the two processes would not share the suite's HTTP servers.

---

## Remediation log (feedback loop)

No loop. The first independent pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-015 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- Verifier: [Verify SPEC-015 criteria](22e87cbe-29d2-464a-ae69-44178c7d6c90). Cursor Task has no native readonly flag. The prompt forbade edits.
- The conflict alert shows the server revision. The toolbar revision line stays on the last successful response until reload. The verifier recorded that and did not treat it as a fail.
- No AC was passed by weakening a test or the criterion.
