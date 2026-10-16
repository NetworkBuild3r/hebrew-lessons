# SPEC-011 AC Verification Eval Report — Media, generation, and approval routes

**Traceability:** INIT-004/SPEC-011 \
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-09T20:20:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Upload, fetch, and approval match the signed-out / user / admin matrix. A user fetch of an unreferenced id is 404. | PASS | `addAdmin` on `POST /api/media` (`server/api/media-routes.js`). GET checks `userMayFetch` before `sendStoredImage`. Tests `ac-1: signed-out is 401…`, `ac-1: admin upload is pending…`, `ac-1: a user fetch is 200 only when…`, `ac-1: a personal plan image…`. `node --test test/media-routes.test.js` → 7 pass, 0 fail, exit 0. |
| ac-2 | `image-plan` returns the dollar estimate and the fake transport records zero calls. Generate without confirm records zero calls. | PASS | `planImage` does not call the transport (`server/media/openai-images.js`). Test `ac-2: image-plan returns the estimate…`: `estimatedDollars` 0.011 and 0.022, `transportCalls.length` 0 until `confirm: true`, then exactly one call, `httpsCalls` 0. |
| ac-3 | Publishing to `everyone` while an introduction word references a pending media id is refused. The same reference on a hidden lesson is allowed. | PASS | `referencesPendingMedia` skips `hidden` (`server/plans/publish.js`). Test `ac-3: everyone and selected refuse a pending image; hidden allows it`: `pending_media` for everyone and selected, 200 for hidden, snapshot stays null on refusal. |
| sec-1 | Every matrix row is tested, including 401 and 403. | PASS | The ac-1 cell loop covers upload, fetch, approval, image-plan, generate, and delete. MCP bearer on approval is 403 `forbidden`. |
| sec-2 | Generation responses and error bodies do not contain `OPENAI_API_KEY`. | PASS | `scrub` / `sendData` (`server/api/media-routes.js`). `assertNoSecret` on the matrix, the dry-run, and the prompt that contains the key (`prompt_refused`). |
| sec-3 | CSRF on every mutation. | PASS | `server/web.js` requires Origin plus JSON, or multipart only for `POST /api/media`. Tests reject a missing Origin on upload, approval, generate, and delete. |

## Definition of done

- [x] `node --test test/media-routes.test.js` exits 0 — verifier re-ran it: 7 pass, 0 fail. The author re-ran it after adding the delete CSRF assertion: 7 pass, 0 fail.
- [x] Each matrix cell has an assertion.
- [x] The transport counter stays 0 on dry-run and on missing confirm.

## Tests run by the verifier

```text
node --test test/media-routes.test.js
→ tests 7, pass 7, fail 0, todo 0, exit 0
```

---

## Remediation log (feedback loop)

None. The first pass was all-PASS.

The verifier noted that delete's missing-Origin case was not asserted. The middleware already applied. The author added that assertion after the verdict. It did not change a criterion or weaken a test. The scoped file was re-run and stayed green. No second verifier pass was required.

---

## Notes

- A kept media row whose live file was archived answers 404. `sendStoredImage` returns false when the file is gone (`server/media/ingest.js`). That matches the SPEC-008 adapter note.
- `PATCH /api/admin/users/:id/ai-planning` is not on this router.
- HTTP routes do not load `mcp/tools.js`. Approval returns 403 when the request presents the MCP bearer.
- No AC was passed by weakening a test or the criterion.
