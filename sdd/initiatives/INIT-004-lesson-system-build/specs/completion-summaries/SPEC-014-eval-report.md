# SPEC-014 AC Verification Eval Report — Today, plan, and six block renderers

**Traceability:** INIT-004/SPEC-014 \
**Verifier:** independent readonly sub-agent · model `composer-2.5-fast` (`ac_eval.verifier_model` on Cursor) · [Verify SPEC-014 criteria](e136026e-a24b-42ef-9398-218510058ec9) · temperature 0 \
**Evaluated at:** 2026-10-09T18:20:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Fixture for `beginner-unit-1` renders Today, the unit overview, an introduction, recall, and the checkpoint. The checkpoint lists each recycled word with `fromLessonId`. | PASS | Test `renders Today, the overview, an introduction, recall, and the checkpoint` passed. `renderScreens` emits those screens (`shared/ui/plan-render.js`). Checkpoint asserts 13 `data-from-lesson` values from the sample recycles and rejects `שָׁלוֹם`. `node --test test/plan-render.test.js` → 14 pass, 0 fail, exit 0. |
| ac-2 | Each of the six `type` values has a renderer test. Five types and the checkpoint emit no `img`. | PASS | Test `renders each type, and only an introduction with alt may emit an img` passed. `renderBlock` dispatches all six types. Decoy `image` fields on the non-introduction blocks produce no `<img>`. |
| ac-3 | A plan string `<script>` and an image with empty alt produce no extra script, no event-handler attribute, and no `img`. Text is escaped. | PASS | Tests `escapes a script payload, refuses an empty alt, and keeps one script src` and `does not turn a handler string into an attribute` passed. One `<script src="/shared/ui/plan-render.js">`. Empty alt and a URL `mediaId` emit no `<img>` and no `evil.example`. `picture()` builds `src` only as `/api/media/` plus a slug. |
| ac-4 | New rules use token variables. Text contrast meets 4.5:1 in light and dark. Primary controls are at least 44px. Other pointer targets are at least 24px. | PASS | Test `uses token variables, meets 4.5:1, and sizes the controls` passed. The SPEC-014 block in `shared/worksheet.css` uses `var(--color-*)`, `var(--size-target)` (24px), and `var(--size-target-primary)` (44px). Token text pairs recompute at or above 4.5:1. No `direction` property in that block. Contrast is the token pairs, not a browser computed-style audit. |
| ac-5 | The sign-in page and the account menu still contain the admin-visibility sentence. | PASS | Test `leaves the sign-in page and the account menu sentence in place` passed. The sentence is still in `shared/app-shell.js`, `shared/login.js`, and `login/index.html`. This spec did not edit those files. |

## Definition of done

- [x] Six block renderers, Today, overview, and the lesson step — `renderScreens` / `renderBlock` in `shared/ui/plan-render.js`, covered by the tests above.
- [x] Hostile text cannot inject a tag — hostile suite, exit 0.
- [x] Admin-visibility sentence still present — ac-5 test, exit 0.
- [x] `node --test test/plan-render.test.js` exits 0 — verifier re-ran it: 14 pass, 0 fail.

## Tests run by the verifier

```text
node --test test/plan-render.test.js
→ 14 pass / 14 total ; exit 0
```

The implementer also ran `npm test` → 488 pass, 0 fail, 2 todo, exit 0. The verifier did not re-run `npm test`.

---

## Remediation log (feedback loop)

No remediation loop. The first independent pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-014 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- A full Chrome click of recall was not run. The spec allows that when the driver is absent. Recall is covered by `decideRecall` and by the hidden correction markup.
- Contrast is checked on the ADR-0006 token pairs the new rules reference, not on a live computed style of the fixture DOM.
- No AC was passed by weakening a test or the criterion.
