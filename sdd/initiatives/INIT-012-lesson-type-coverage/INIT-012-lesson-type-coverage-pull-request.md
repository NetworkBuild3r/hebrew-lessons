# Pull Request: INIT-012 — Lesson-type coverage (`schemaVersion` 1.2.0, `exercise`, `assignment`, and `future` tense)

## Summary

Adds `schemaVersion` `1.2.0` support for the five lesson types in the teacher's curriculum (Topic conversation, Review worksheet, Bible verse, Verb workshop, Short story with true/false) by introducing the `exercise` block (`choose`, `true-false`, `unscramble`, `translate`, `dictation`), the `assignment` block (writing task with sentence count, word bank chips, and homework lines), and `future` tense in `conjugation-table`, across the JSON schema, validator (`N20`–`N23`), lexicon, learner player, `/editor/` teacher UI, and `hebrew-worksheet` skill.

## Work Performed (by Spec)

- **`INIT-012/SPEC-001` (docs):** Authored `docs/decisions/ADR-0014-lesson-type-coverage.md` and `docs/decisions/ADR-0014-assets/exercise-examples.json`.
- **`INIT-012/SPEC-002` (data):** Added `schemaVersion` `1.2.0`, `v120BlockType`/`v120Only`, `exerciseBlock` (5 kinds), validator rules `N20`–`N23`, and `usesV120` auto-raise and restore floor in `server/plans/`.
- **`INIT-012/SPEC-003` (data):** Added `assignmentBlock`, `tense: "future"` (`futureCell`), `CELLS.future` and `listGrammarConcepts` future cells in `server/lexicon/`, and migration `010-lexicon-future-cells.js` (with `up()` and `down()`).
- **`INIT-012/SPEC-004` (frontend):** Added `shared/ui/blocks/compare.js` and `shared/ui/blocks/exercise.js` (`choose` and `true-false` with retry-before-reveal, keyboard `Enter`/`Space`, `aria-live`, and 360px layout).
- **`INIT-012/SPEC-005` (frontend):** Added `unscramble`, `translate`, and `dictation` to `exercise.js` and `compare.js` (`shuffleWords`, `sameHebrew` with `accept[]`, gesture-only audio playback, load-failure skip, first-try summary).
- **`INIT-012/SPEC-006` (frontend):** Added `shared/ui/blocks/assignment.js` and `future` tense rendering/drill in `shared/ui/blocks/conjugation-table.js`.
- **`INIT-012/SPEC-007` (frontend):** Added `exercise-items`, `assignment-words`, `assignment-homework`, and 9-row `future` `table-cells` widgets in `/editor/` (`shared/admin/plan-widgets.js`, `plan-form.js`, `plan-hints.js`), with single-language chrome and hidden `id`/`type`/JSON metadata.
- **`INIT-012/SPEC-008` (docs):** Updated `.cursor/skills/hebrew-worksheet/recipes.md`, `SKILL.md`, and `schema.md` for all five lesson types and added `test/skill-recipes.test.js`.
- **`INIT-012/SPEC-009` (testing):** Added five original lesson-type fixtures (`test/fixtures/lesson-types/`), `test/plan-lesson-types-matrix.test.js`, `test/plan-lesson-types-mcp.test.js`, `test/chrome-lesson-types-smoke.js`, `SPEC-009-not-held.md`, and `SPEC-009-forms-to-check.md`.

## Tests & Smoke Verification

- **Node test suites added/updated:** `test/plan-exercise-schema.test.js`, `test/plan-schema-version-1-2.test.js`, `test/plan-assignment-schema.test.js`, `test/plan-future-tense.test.js`, `test/plan-render-compare.test.js`, `test/plan-render-exercise.test.js`, `test/plan-render-exercise-typed.test.js`, `test/plan-render-assignment.test.js`, `test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, `test/skill-recipes.test.js`, `test/plan-lesson-types-matrix.test.js`, `test/plan-lesson-types-mcp.test.js`, `test/rollout-preflight.test.js`, `test/restyle-gate.test.js`, `test/plan-revisions-migration.test.js` — **all pass (0 failures)**.
- **Real headless Chrome smoke scripts executed:**
  - `node test/chrome-exercise-smoke.js` -> `PASS`
  - `node test/chrome-plan-editor-exercise.js` -> `PASS`
  - `node test/chrome-lesson-types-smoke.js` -> `PASS` (390px and 1280px, English and Hebrew, `/editor/` hand build + learner playback)
- **Local Docker container (`hebrew-hebrew-1`):** restarted and verified `Up` on ports `8080`/`8787`.

## Eval & Security Reports

- Per-spec independent AC verification eval reports (`SPEC-001-eval-report.md` through `SPEC-009-eval-report.md`): **all-PASS (9/9)**.
- Initiative-wide Security Review (`INIT-012-security-review.md`, `INIT-012/SECURITY-REVIEW`): **PASS** (0 Critical, 0 High; 1 Medium `'hufil'`->`'hufal'` typo in migration 010 and 1 Low `/g` regex flag in `validate.js` remediated and verified).
- Architecture Diagrams: `sdd/initiatives/INIT-012-lesson-type-coverage/design/INIT-012-lesson-type-coverage-architecture.md` (`INIT-012/DESIGN-DIAGRAMS`).
