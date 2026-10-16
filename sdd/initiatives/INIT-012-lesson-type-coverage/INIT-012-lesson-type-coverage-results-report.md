# INIT-012 Results Report — Lesson-Type Coverage (`1.2.0`, Exercise, Assignment, Future Tense)

**TL;DR:** `INIT-012` (`INIT-012-lesson-type-coverage`) completed all 9 specs in 129 wall-clock minutes of orchestrated execution (204 session-minutes across 9 sub-agent sessions), delivering `schemaVersion` `1.2.0`, the five-kind `exercise` block (`choose`, `true-false`, `unscramble`, `translate`, `dictation`), the `assignment` block, and `future` tense in `conjugation-table` across schema, validator, lexicon, player, `/editor/`, and the `hebrew-worksheet` skill, with 100% independent AC verification pass rate and 0 unresolved Critical/High security findings (**Confidence: 0.95**).

## Executive Summary

Analysis of two years of the teacher's real Hebrew lesson documents showed that four of the five recurring lesson types already mapped to existing `1.1.0` recipes (`topic-conversation`, `review-worksheet`, `verse-reading`, `verb-workshop`), while Type E (short story with true/false) is a `review-worksheet` variant. What prevented building those lessons end-to-end was missing exercise formats inside the lessons (`choose`, `true-false`, `unscramble`, `translate`, `dictation`), multi-sentence writing assignments with homework checklists (`assignment`), and `future` tense conjugation tables. `INIT-012` specified (`ADR-0014`), validated, rendered, and exposed all of these in `/editor/` and MCP under `schemaVersion` `1.2.0`.

## What the Process Caught

Independent Phase 4.5 verification and the Phase 3.5 Opus security review caught concrete defects before completion and fixed them in-loop: in `SPEC-004` a `var section` loop closure bug in `exercise.js` caused multi-block pages to announce live-region feedback on the last block only; in `SPEC-005` `shuffleWords` could perform a no-op swap between two indices holding identical Hebrew words; in `SPEC-006` clicking an assignment word chip with no audio clip triggered browser TTS instead of staying quiet; in `SPEC-007` teacher activity forms still showed `Id`/`Type` metadata rows; in `SPEC-009` migration `010` lacked a `down()` rollback for `rollout-preflight.test.js`; and in Phase 3.5 migration `010` had a `'hufil'` typo in the rebuilt `binyan` `CHECK` constraint and `validate.js` used a stateful `/g` regex inside `.test()`. Every one of those defects was remediated and re-verified green before closing the initiative.

## Methodology

- **Design-first (`SPEC-001`):** Recorded all item shapes, limits, rule IDs (`N20`–`N23`), and `1.2.0` versioning rules in `ADR-0014` with machine-parseable examples (`ADR-0014-assets/exercise-examples.json`) before touching schema or code.
- **Parallel batch orchestration:** Executed `SPEC-001` -> `SPEC-002` -> parallel `SPEC-003 + SPEC-004` -> parallel `SPEC-005 + SPEC-006` -> parallel `SPEC-007 + SPEC-008` -> `SPEC-009`, enforcing strict pairwise file-disjoint boundaries on every parallel batch.
- **Independent adversarial AC verification (Phase 4.5):** Every spec was verified by a fresh, read-only sub-agent that did not author the implementation, running node tests and real headless Chrome smoke scripts and looping through fix sub-agents until every AC was `PASS`.
- **Thorough security review (Phase 3.5):** Conducted an initiative-wide review on `claude-opus-5-5-high` across all five threat-model areas and remediated both actionable findings (`SEC-012-M1`, `SEC-012-L1`) immediately.

## Spec-by-Spec Results

| Spec | Domain | Title | Time | Loops | Final Verdict |
|------|--------|-------|------|-------|---------------|
| SPEC-001 | docs | ADR-0014: exercise, assignment, and future tense | 6m | 2 | PASS (5/5 ACs) |
| SPEC-002 | data | Schema version 1.2.0 and the `exercise` block | 8m | 2 | PASS (10/10 ACs) |
| SPEC-003 | data | The `assignment` block and future tense, on 1.2.0 | 32m | 2 | PASS (10/10 ACs) |
| SPEC-004 | frontend | The player runs Exercise: choose and true or false | 32m | 3 | PASS (8/8 ACs) |
| SPEC-005 | frontend | The player runs Exercise: unscramble, translate, and dictation | 24m | 3 | PASS (10/10 ACs) |
| SPEC-006 | frontend | The player runs Assignment and the future-tense table | 24m | 2 | PASS (9/9 ACs) |
| SPEC-007 | frontend | `/editor/` adds and edits every new activity | 19m | 3 | PASS (11/11 ACs) |
| SPEC-008 | docs | Skill and recipe cards for the five lesson types | 19m | 1 | PASS (6/6 ACs) |
| SPEC-009 | testing | Five lesson types built, played, and checked in a browser | 40m | 3 | PASS (12/12 ACs) |

## Design References

- Teacher plan and lesson screens follow the existing `INIT-011` Figma frames (`19:2304` and `19:2592` in file `NJ8HfTPCQgrFty2VVQBMhY`) and the token system in `docs/decisions/ADR-0006-assets/tokens.json` (`shared/worksheet.css`, verified with zero raw hex values in new rules and tested at `360px`/`390px` and `1280px` viewports).
- Architecture diagrams: `sdd/initiatives/INIT-012-lesson-type-coverage/design/INIT-012-lesson-type-coverage-architecture.md`.

## Risks & Follow-Ups

1. **Human verification of future verb forms (`O4` / `D13`):** All 9 future forms of `לְדַבֵּר` in `SPEC-009-forms-to-check.md` remain `verified = 0` by design so `N14` warns and the drill does not score them until you and Aaron review the table and mark them verified.
2. **`INIT-010/SPEC-004` (`match` and `type-it`):** `match` remains tracked under `INIT-010/SPEC-004` (`SPEC-009-not-held.md`).
3. **New topic lessons (health, hobbies, phone, calendar, culture/news):** Ready to be authored as content plans using the new `1.2.0` blocks and recipe cards.

## Conclusion

All 81 acceptance criteria across `SPEC-001`–`SPEC-009` passed independent verification, all three Chrome smoke suites (`chrome-exercise-smoke.js`, `chrome-plan-editor-exercise.js`, `chrome-lesson-types-smoke.js`) and `rollout-preflight.test.js` / `restyle-gate.test.js` pass, and the local Docker container `hebrew-hebrew-1` is updated and running.
