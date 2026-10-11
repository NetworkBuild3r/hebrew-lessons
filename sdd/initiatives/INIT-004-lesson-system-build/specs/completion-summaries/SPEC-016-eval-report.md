# SPEC-016 AC Verification Eval Report — Builder media library, preview, and validation

**Traceability:** INIT-004/SPEC-016 \
**Verifier:** independent sub-agent · model `composer-2.5-fast` · agent `f2d999a3-f3f9-4257-a884-03dd5a6658de` · temperature 0 \
**Evaluated at:** 2026-10-09T21:10:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

Cursor Task has no native `readonly` flag. The prompt forbade edits. The verifier did not author the code.

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Choosing a picture on a graded-story block is not offered. On an introduction word, save sends set_field for mediaId and alt. | PASS | Test `ac-1: a graded-story block offers no picture; an introduction word saves mediaId and alt`. No `[data-plan-pictures]` or attach control on the story block. Introduction word PATCH fields are `mediaId` then `alt` on `units/unit-1/lessons/cafe-order/blocks/introduction/words/1`. `paintPictures` runs only for `introduction` (`shared/admin/plan-builder.js`). `node --test test/plan-builder-media.test.js` 7 pass, exit 0. |
| ac-2 | Generate stays disabled until a dry-run response is shown and confirm is checked. No generate call before that. | PASS | Test `ac-2: generate stays disabled until the dry-run figure is shown and confirmed`. Zero `POST /api/admin/images` after a bare click and after confirm without a dry-run. After `GET /api/admin/images/image-plan`, the estimate text is `7.25` and generate stays disabled until confirm. One POST then, with `confirm: true` and `estimate: 7.25`. Source does not contain `7.25`. `syncGenerate` / `generateImage` in `shared/admin/plan-builder.js`. |
| ac-3 | A publish response that includes an N1 warning shows the id N1. A block failure shows the snapshot was not published. | PASS | Test `ac-3: an N1 warning is shown, and a block failure says the snapshot was not published`. `[data-plan-finding="N1"]` text matches `N1` and `server said nine`. `[data-plan-publish-refusal]` matches `snapshot was not published` and `validation`. Status stays `draft`. `schema` and `recall-day` are labeled from the same response. Builder source has no `maxNewWords`. |
| ac-4 | The approval button is disabled when the alt field is empty. | PASS | Test `ac-4: the approval button is disabled while alt text is empty`. Disabled on an empty field and on whitespace. No `/approval` call until alt is set. Then one `POST /api/admin/media/coffee-cup/approval` with `{ altText: 'a cup of coffee' }`. |

## Definition of done

- [x] Media, preview, and validation regions on the builder view — `shared/admin/plan-builder.js`
- [x] Tests `test/plan-builder-media.test.js` — 7 pass, exit 0
- [x] Upload is `POST /api/media` through the same-origin admin fetch, not a form action
- [x] Preview is `HebrewPlanRender.renderBlock` output. The test's `<b>` meaning is escaped. Source has no `eval(`

## Tests run by the verifier

```text
node --test test/plan-builder-media.test.js
→ 7 passed / 7 total ; exit 0
```

The implementer's `npm test` (553 pass, 0 fail, exit 0) was not re-run by the verifier.

---

## Remediation log (feedback loop)

No verifier FAIL. The first independent pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-###/SPEC-NNN · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- No AC was passed by weakening a test or the criterion.
- A word that has no `image` object also sends `set_field` `image` before `mediaId` and `alt`, so the server can nest those two names. The ac-1 fixture word already has an image, and that save is only `mediaId` and `alt`.
