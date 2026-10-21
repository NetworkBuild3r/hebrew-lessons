# SPEC-006 eval report

**Spec:** `INIT-012/SPEC-006` (`SPEC-006-frontend-assignment-and-future-table.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test run (blocking checks)

```text
node --test test/plan-render-assignment.test.js test/plan-render.test.js test/render-conjugation.test.js
-> tests 46 | pass 46 | fail 0 | exit 0
```

## Per-AC verdicts (Loop 2 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-assignment.test.js:211-235` — `shows prompt, minimum line, word chips, and labeled textarea` (`shared/ui/blocks/assignment.js:95-136`). |
| ac-2 | PASS | `test/plan-render-assignment.test.js:238-252` — `updates count and toggles Done when crossing the minimum` (`shared/ui/blocks/assignment.js:146-155`). |
| ac-3 | PASS | `test/plan-render-assignment.test.js:255-292` — `calls playWord with clip and word, or quietly when no clip exists` (`shared/ui/blocks/assignment.js:157-172`). |
| ac-4 | PASS | `test/plan-render-assignment.test.js:295-315,355-372,488-515` — homework checkboxes toggle with keyboard (`Space`), reload clears ticks (`hwChecked: false`), and zero `fetch`/`localStorage` calls. |
| ac-5 | PASS | `test/plan-render-assignment.test.js:317-352` — escapes `<img src=x onerror=alert(1)>` in `prompt`, `useWords`, and `homework`; learner textarea read as text only (`shared/ui/blocks/assignment.js:19-34,153`). |
| ac-6 | PASS | `test/plan-render-assignment.test.js:328-350` and `test/render-conjugation.test.js:144-165` — `future` table renders nine rows in past-tense order with the same pronoun labels (`shared/ui/blocks/conjugation-table.js:25-35,149,160-164`). |
| ac-7 | PASS | `test/plan-render-assignment.test.js:352-375` and `test/render-conjugation.test.js:167-182` — drill queues verified future forms only; unverified future forms show `not yet checked` / `data-verified="0"` and are not scored (`shared/ui/blocks/conjugation-table.js:128-134,247`). |
| ac-8 | PASS | `test/plan-render-assignment.test.js:377-395` and `test/render-conjugation.test.js` — present and past study/drill tables render unchanged. |
| ac-9 | PASS | `test/plan-render-assignment.test.js:398-520` — visible `<label for=...>`, `lang="he" dir="rtl"`, and headless Chrome `360 -> 1280` resize keeps learner text while `Page.reload()` clears both text and homework ticks. |

## Remediation-loop log

- **Loop 1:** `ac-1`, `ac-2`, `ac-4`–`ac-9` PASS; `ac-3` FAIL because `playAssignmentWord` called `playWord(null, word)` when a chip had no clip, triggering TTS fallback instead of doing nothing quietly.
- **Remediation 1:** Fix sub-agent updated `playAssignmentWord` in `shared/ui/blocks/assignment.js:157-172` to return early when no clip exists, and added tests for missing clip, keyboard checkbox toggle, Chrome reload clearing homework ticks, and future study `not yet checked` markup.
- **Loop 2 (fresh independent verifier):** all-PASS (46/46 tests pass, exit 0).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
