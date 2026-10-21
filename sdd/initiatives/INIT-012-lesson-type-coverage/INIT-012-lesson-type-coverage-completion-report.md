# INIT-012 Completion Report — A plan can hold every kind of lesson the teacher actually teaches

**Initiative:** INIT-012 · `INIT-012-lesson-type-coverage`
**Specs closed:** 2026-10-15 (all 9 specs at `complete`)
**Mode E closed:** 2026-10-15
**Branch:** `lesson-type-coverage` (not created; work was done in place on the main checkout because uncommitted `INIT-009` to `INIT-011` work lived in the working tree; nothing was committed).

## Outcome

All **9** specs (`SPEC-001` through `SPEC-009`) are `complete`, with completion summaries and independent AC-verifier eval reports under `specs/completion-summaries/` (verified by `initiative_inventory_check.py`: 9/9 specs, 9/9 completion summaries, 9/9 timer entries).

What shipped:
1. **`ADR-0014` (`INIT-012/SPEC-001`):** `docs/decisions/ADR-0014-lesson-type-coverage.md` and `docs/decisions/ADR-0014-assets/exercise-examples.json`, defining the five-lesson-type table (Types A–E), decisions D2–D13, the `exercise` and `assignment` block contracts, `tense: "future"`, validator rules `N20`–`N23`, `schemaVersion` `1.2.0` auto-raise and floor rules, and answer disclosure (`D6`).
2. **Plan language `1.2.0` + `exercise` and `assignment` + `future` tense (`INIT-012/SPEC-002`, `INIT-012/SPEC-003`):** `lesson-plan.schema.json` (`1.2.0`, `v120BlockType`, `v120Only`, `acceptString`, `exerciseBlock` with 5 per-kind item definitions, `assignmentBlock`, `futureCell`), `server/plans/validate.js` (`V120`, `N20`–`N23`, `N13`/`N14`/`N16`, `usesV120`), `server/plans/ops.js` (auto-raise to `1.2.0` in the same revision and restore floor), `server/lexicon/` (`CELLS.future`, `listGrammarConcepts`, migration `010-lexicon-future-cells.js` with `up()` and `down()`), and `shared/admin/activity-map.js` + `plan-hints.js`.
3. **Learner plan player (`INIT-012/SPEC-004`, `INIT-012/SPEC-005`, `INIT-012/SPEC-006`):** `shared/ui/blocks/compare.js` (vowel-point stripping, final-letter and geresh/gershayim normalization, `sameHebrew` with `accept[]`, `stepAttempt`, deterministic distinct-aware `shuffleWords`), `shared/ui/blocks/exercise.js` (`choose`, `true-false`, `unscramble`, `translate`, `dictation` with gesture-only audio playback and load-failure skip, first-try summary, keyboard `Enter`/`Space`, `aria-live` region, full HTML escaping), `shared/ui/blocks/assignment.js` (`prompt`, `minSentences` live counter, word chips calling `playWord` when a clip exists, ephemeral homework checkboxes), and `future` tense in `shared/ui/blocks/conjugation-table.js`.
4. **Teacher plan editor `/editor/` (`INIT-012/SPEC-007`):** `exercise-items`, `assignment-words`, `assignment-homework`, and 9-row `future` `table-cells` widgets in `shared/admin/plan-widgets.js` and `plan-form.js`, with kind-change double confirmation (`aud-1`), single-language chrome (`ac-8`), hidden `id`/`type`/JSON metadata (`ac-6`), and `<900px` responsive stacking.
5. **Worksheet skill and recipe cards (`INIT-012/SPEC-008`):** `.cursor/skills/hebrew-worksheet/recipes.md`, `SKILL.md`, and `schema.md` updated for all five lesson types, the five exercise kinds, assignment, future tense, and the class-docs originality and dictionary-pointing rule, guarded by `test/skill-recipes.test.js`.
6. **End-to-end five-lesson-type verification (`INIT-012/SPEC-009`):** five original sample lesson fixtures (`test/fixtures/lesson-types/`), matrix suite `test/plan-lesson-types-matrix.test.js`, MCP build suite `test/plan-lesson-types-mcp.test.js`, headless Chrome smoke `test/chrome-lesson-types-smoke.js`, `SPEC-009-not-held.md`, and `SPEC-009-forms-to-check.md`.

## Session & Timer Metrics (Phase 2)

From `sdd/initiatives/INIT-012-lesson-type-coverage/specs/.sdd-timer.json`:

| Spec | Domain | Sessions | Elapsed (min) | Status |
|------|--------|----------|---------------|--------|
| SPEC-001 | docs | 1 | 6 | completed |
| SPEC-002 | data | 1 | 8 | completed |
| SPEC-003 | data | 1 | 32 | completed |
| SPEC-004 | frontend | 1 | 32 | completed |
| SPEC-005 | frontend | 1 | 24 | completed |
| SPEC-006 | frontend | 1 | 24 | completed |
| SPEC-007 | frontend | 1 | 19 | completed |
| SPEC-008 | docs | 1 | 19 | completed |
| SPEC-009 | testing | 1 | 40 | completed |

- **Total specs:** 9/9 completed (0 active sessions, 0 timer discrepancies)
- **Sum of per-spec sessions:** 204 minutes across 9 sessions (min 6m, median 24m, mean 22.7m, max 40m)
- **Wall-clock span (with parallel batches SPEC-003+004, SPEC-005+006, SPEC-007+008):** 129 minutes of active agent execution time.

## Requirements Traceability

| REQ | Requirement | Evidence | Status |
|-----|-------------|----------|--------|
| REQ-001 | `exercise` block (`choose`, `true-false`, `unscramble`, `translate`, `dictation`) in schema, validator, player | `test/plan-exercise-schema.test.js`, `test/plan-render-exercise.test.js`, `test/plan-render-exercise-typed.test.js` | Met |
| REQ-002 | Choose: 2–5 choices, one answer, optional `why` shown after | `test/plan-exercise-schema.test.js` (`N20`), `test/plan-render-exercise.test.js` (`ac-1`, `ac-2`) | Met |
| REQ-003 | True/false: statement, boolean answer, optional `why` | `test/plan-render-exercise.test.js` (`ac-3`) | Met |
| REQ-004 | Unscramble: 3–10 words, deterministic shuffle, `alt` orders | `test/plan-exercise-schema.test.js` (`N21`), `test/plan-render-exercise-typed.test.js` (`ac-1`, `ac-2`, `ac-9`) | Met |
| REQ-005 | Translate: English cue, typed Hebrew answer, point-insensitive + `accept[]` | `test/plan-render-compare.test.js`, `test/plan-render-exercise-typed.test.js` (`ac-3`, `ac-4`) | Met |
| REQ-006 | Dictation: audio on tap, load-failure skip, same scoring as translate | `test/plan-render-exercise-typed.test.js` (`ac-6`, `ac-7`), `test/chrome-exercise-smoke.js` | Met |
| REQ-007 | One retry before reveal on wrong answer | `shared/ui/blocks/compare.js` `stepAttempt`, verified across all 5 kinds | Met |
| REQ-008 | `assignment` block (`prompt`, `minSentences`, `useWords`, `homework`), stores no answer | `test/plan-assignment-schema.test.js`, `test/plan-render-assignment.test.js` | Met |
| REQ-009 | `conjugation-table` accepts `tense: future`, `N13`/`N14`, 9 rows, verified-only drill | `test/plan-future-tense.test.js`, `test/render-conjugation.test.js`, `test/plan-render-assignment.test.js` | Met |
| REQ-010 | `/editor/` adds and edits every new activity without JSON, id, or path | `test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, `test/chrome-plan-editor-exercise.js` | Met |
| REQ-011 | MCP builds the same blocks through existing operations | `test/plan-lesson-types-mcp.test.js` | Met |
| REQ-012 | `hebrew-worksheet` skill and recipes updated for all 5 lesson types | `test/skill-recipes.test.js` (8/8 pass) | Met |
| REQ-013 | One original sample lesson per type validates and plays in Chrome | `test/plan-lesson-types-matrix.test.js`, `test/chrome-lesson-types-smoke.js` | Met |
| REQ-014 | Authored text escaped on every new surface | `test/plan-lesson-types-matrix.test.js` (`ac-8`), `test/plan-render-exercise.test.js`, `test/plan-editor-exercise.test.js` | Met |
| REQ-015 | `1.2.0` version guard and same-revision auto-raise | `test/plan-schema-version-1-2.test.js`, `test/plan-future-tense.test.js` (`ac-9`) | Met |
| REQ-016 | Future forms listed for human check; none verified by tool or test | `SPEC-009-forms-to-check.md`, `test/plan-lesson-types-matrix.test.js` (`ac-12`) | Met |

## Security Review (Phase 3.5 — `INIT-012/SECURITY-REVIEW`, `SECSCAN-012`)

Full report: [INIT-012-security-review.md](INIT-012-security-review.md) (`SECSCAN-012`, executed on `claude-opus-5-5-high` at `thorough` depth).

- **Severity totals:** Critical: **0** · High: **0** · Medium: **0** (1 found and remediated before completion) · Low: **1** (pre-existing absence of HTTP CSP header; 1 regex `/g` finding remediated before completion) · Informational: **4**.
- **Remediations applied before Phase 5:**
  1. **`SEC-012-M1` (Medium — remediated):** `server/db/migrations/010-lexicon-future-cells.js` line 130 had a typo `'hufil'` instead of `'hufal'` in the rebuilt `binyan` `CHECK` constraint. Fixed to `'hufal'` with a self-healing guard in `up()`.
  2. **`SEC-012-L1` (Low — remediated):** `server/plans/validate.js` line 1190 used stateful `/g` `POINTS_AND_ACCENTS.test(alt)` inside `checkExerciseBlocks`. Replaced with non-global `HEBREW_POINT.test(alt)`.
- **Positive observations:** All authored strings are escaped via `escapeHtml` in learner blocks and set via `textContent`/`.value` in admin widgets (`0` `innerHTML` uses in `plan-widgets.js`); `audioPath` is validated by regex at schema, validator, and player levels with `encodeURIComponent` per segment; three independent layers prevent MCP from setting `verified = 1`; learner answers never leave the browser.

## Architecture Diagrams (Phase 3.6 — `INIT-012/DESIGN-DIAGRAMS`)

Full diagrams: [design/INIT-012-lesson-type-coverage-architecture.md](design/INIT-012-lesson-type-coverage-architecture.md) (Component architecture, Authoring & Learner playback sequence diagram, and Lesson-Type A–E to Block mapping).
