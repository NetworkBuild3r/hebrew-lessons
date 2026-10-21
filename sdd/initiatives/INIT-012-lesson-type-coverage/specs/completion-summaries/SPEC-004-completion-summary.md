# SPEC-004 Completion Summary

**Spec:** SPEC-004 · A learner can answer choose and true-or-false exercises in a published plan
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** frontend · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-004`

## Change Produced

Added `shared/ui/blocks/compare.js` (Hebrew vowel-point stripping, final-letter normalization, and `stepAttempt` retry-before-reveal helper) and `shared/ui/blocks/exercise.js` (`HebrewBlockExercise` rendering and binding `choose` and `true-false` exercises with retry-before-reveal, keyboard `Enter`/`Space` support, `aria-live` announcements with icon + words, and full HTML escaping), registered `exercise` in `shared/ui/plan-render.js`, wired browser script loads in `shared/plan/index.html` and `shared/app-routes.js`, added token-only responsive styles in `shared/worksheet.css`, and added unit + headless Chrome 360×640 tests in `test/plan-render-exercise.test.js`.

| File | Change | Provenance |
|------|--------|------------|
| `shared/ui/blocks/compare.js` | New shared Hebrew compare + retry-before-reveal helper | `INIT-012/SPEC-004` |
| `shared/ui/blocks/exercise.js` | New `exercise` block player (`choose`, `true-false`) | `INIT-012/SPEC-004` |
| `shared/ui/plan-render.js` | Registered `exercise` in `REGISTRY` and `bind()` | `INIT-012/SPEC-004` |
| `shared/plan/index.html`, `shared/app-routes.js` | Loaded `compare.js` and `exercise.js` | `INIT-012/SPEC-004` |
| `shared/worksheet.css` | Added `.plan-exercise*` responsive styles using CSS custom properties only | `INIT-012/SPEC-004` |
| `test/plan-render-exercise.test.js`, `test/plan-render.test.js` | New test suite (including headless Chrome 360×640 layout/keyboard/focus-ring test; 37/37 pass) | `INIT-012/SPEC-004` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 3 after two remediation loops (Loop 1 added `keydown` Enter/Space handling and `true-false` XSS tests; Loop 2 added a real headless Chrome 360×640 viewport test and fixed a multi-block `bind()` closure bug). Full report: [SPEC-004-eval-report.md](SPEC-004-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-exercise.test.js:261-290` (`shared/ui/blocks/exercise.js:75-79`) |
| ac-2 | PASS | `test/plan-render-exercise.test.js:291-314` (`shared/ui/blocks/exercise.js:274-314`) |
| ac-3 | PASS | `test/plan-render-exercise.test.js:317-329` (`shared/ui/blocks/exercise.js:111-151`) |
| ac-4 | PASS | `test/plan-render-exercise.test.js:332-377` (XSS in all choose + true-false fields escaped) |
| ac-5 | PASS | `test/plan-render-exercise.test.js:375-426,589-598` (buttons, focus-visible ring, keyboard-only completion) |
| ac-6 | PASS | `test/plan-render-exercise.test.js:429-438,600-620` (`aria-live` region + icon and words) |
| ac-7 | PASS | `test/plan-render-exercise.test.js:441-469` (zero `fetch`/beacon/XHR calls) |
| ac-8 | PASS | `test/plan-render-exercise.test.js:472-632` (360×640 headless Chrome `scrollWidth === 360`, sections `312 <= 312`) |

## Security Review

All authored strings pass through `escapeHtml` (`text()` and `attr()`), `assertCleanTags` confirms no inline `on*` event attributes, and no learner answer is sent over the network.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` in parallel with `SPEC-003` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-004.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- `INIT-010/SPEC-004` had not landed yet, so `SPEC-004` lifted the shared Hebrew compare and retry-before-reveal helper into `shared/ui/blocks/compare.js` as specified in Scope (in).
