# SPEC-004 eval report

**Spec:** `INIT-012/SPEC-004` (`SPEC-004-frontend-choose-and-true-false.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test run (blocking checks)

```text
node --test test/plan-render-exercise.test.js test/plan-render.test.js
-> tests 37 | pass 37 | fail 0 | exit 0
```

## Per-AC verdicts (Loop 3 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-exercise.test.js:261-290` — `renders each choice as a button in authored order` (`shared/ui/blocks/exercise.js:75-79`, `shared/ui/plan-render.js:95-96`). |
| ac-2 | PASS | `test/plan-render-exercise.test.js:291-314` — `shows try again on first miss and answer on second` (`shared/ui/blocks/exercise.js:274-314`, `shared/ui/blocks/compare.js:71-77`). |
| ac-3 | PASS | `test/plan-render-exercise.test.js:317-329` — `uses the same retry rule` (`shared/ui/blocks/exercise.js:111-151,266-316`). |
| ac-4 | PASS | `test/plan-render-exercise.test.js:332-377` — escapes `<img src=x onerror=alert(1)>` in choose (`id`, `heading`, `prompt`, `choices`, `answer`, `why`) and true-false (`id`, `heading`, `statement`, `why`); no `<img` element or `on*` attribute (`shared/ui/blocks/exercise.js:37-46`). |
| ac-5 | PASS | `test/plan-render-exercise.test.js:375-426,589-598` — `<button type="button">` choices, `.plan-exercise-choice:focus-visible` (`shared/worksheet.css:2368-2371`), keyboard-only completion (`Enter`/`Space` at `shared/ui/blocks/exercise.js:341-354`), and headless Chrome computed focus ring (`3px solid rgb(30, 58, 47)`). |
| ac-6 | PASS | `test/plan-render-exercise.test.js:429-438,600-620` — `aria-live="polite" aria-atomic="true"` live region (`shared/ui/blocks/exercise.js:106,149,194-209`) announces icon + words (`✓ נכון — Correct`, retry, reveal). |
| ac-7 | PASS | `test/plan-render-exercise.test.js:441-469` — source scan and runtime `fetch` spy confirm zero `fetch`/beacon/XHR calls. |
| ac-8 | PASS | `test/plan-render-exercise.test.js:472-632` — CSS rules (`shared/worksheet.css:2315-2417`) plus real headless Chrome at `360x640` confirming `documentElement.scrollWidth === 360` and `section.scrollWidth === 312 <= clientWidth` before and after keyboard completion. |

## Remediation-loop log

- **Loop 1:** `ac-1`–`ac-4`, `ac-6`–`ac-8` PASS; `ac-5` FAIL (missing `keydown` Enter/Space handler and keyboard-only completion test) and Security AC FAIL (missing `true-false` `statement` XSS test).
- **Remediation 1:** Fix sub-agent added `keydown` (`Enter`/`Space` with `preventDefault`) in `shared/ui/blocks/exercise.js:341-352`, keyboard-only multi-item completion test, and `true-false` XSS test in `test/plan-render-exercise.test.js`.
- **Loop 2:** `ac-1`–`ac-7` PASS; `ac-8` FAIL because the 360px check only inspected CSS text without a runtime 360px viewport measurement in Chrome.
- **Remediation 2:** Fix sub-agent added a real headless Chrome 360×640 viewport test in `test/plan-render-exercise.test.js:472-632` (measuring `scrollWidth <= clientWidth`, computed `:focus-visible` outline, and keyboard completion + live region) and fixed a `var section` loop closure bug in `shared/ui/blocks/exercise.js:332-354`.
- **Loop 3 (fresh independent verifier):** all-PASS (37/37 tests pass, exit 0).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
