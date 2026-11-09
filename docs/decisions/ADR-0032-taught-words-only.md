# ADR-0032: Taught words only

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-08 |
| **Authored by** | INIT-037/SPEC-002 |
| **Initiative** | INIT-037 (taught-words-only), SPEC-002 |
| **Extends** | [ADR-0026](ADR-0026-lessons-that-build.md) (lessons that build) |
| **Amends** | [ADR-0026](ADR-0026-lessons-that-build.md) D3 (consolidation when the backlog is large); restores [INIT-030 SPEC-004](../../sdd/initiatives/INIT-030-lessons-that-build/specs/SPEC-004-content-shape-rules.md) line 31 (`unit.same_shape` at ≥80% similarity) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A lesson asks only about words the learner was taught. Answer buttons come from words with a card, new words this unit introduces before the ask, or words in a unit the learner finished—not from an earlier published unit they never opened, and not from the wrong side of a tutor-mistake contrast. Learner-facing English carries no authoring notes. Consolidation is at most one unit ahead, then review; ADR-0026 D4's meaning mix stays. A unit that copies the previous one's activity-type sequence warns; a copied story blocks. Old units are never edited in place: the reviser rebuilds the next one or two and leaves the rest as drafts.

## Context

Brian, on 2026-11-08, failed a step at https://hebrew.ibhacked.us/lesson/12/1 and wrote: "I didn't even have some of the option i just failed on." [ASMT-004](../../sdd/assessments/ASMT-004-hebrew-web-app/ASMT-004-hebrew-web-app-assessment-report.md) confirmed the pool: cloze wrong answers come from the unit's own word list first, then from earlier published units, with no check of what the learner was taught. That list includes both sides of a "right — wrong" contrast. The same assessment found 23 units (12–34) built from one structure—zero new words, cloned shapes, authoring notes in the English—and recommended taking them off Today, fixing the pipeline, then rebuilding forward. Units 12–34 were unpublished on 2026-11-08 (INIT-037 SPEC-001).

[ADR-0026](ADR-0026-lessons-that-build.md) D3 already asks for consolidation when more than 40 words are learning or accuracy is under 75% with reviews. Without a queue, that switch produced a long run of consolidation clones. D4's 40% drill cap and story floor are not the fault. INIT-030 SPEC-004 line 31 required `unit.same_shape` to warn when the activity-type sequence is at least 80% the same as the previous unit; the live check warned the other way.

## Decision

**D1 · Taught.** A word is taught for a learner when any of these holds: they have a card for it (`LearnerCardLexemeIDs`, `internal/db/cards_new.go`); it is a new word this unit introduces before the ask; it is in a unit they finished (`UnitProgress`, `internal/db/scope.go`). An earlier published unit they never opened does not count. A word counts as taught only when the app can show the learner met it: a card, a new word introduced before the ask, or a finished unit. Lesson 12's buttons included words learner 1 had no card for, because the pool treated the unit list and earlier published units as known. This governs button pools in `internal/web/lesson.go`. Rejected: treating every earlier published unit as taught.

**D2 · A wrong form is never a unit word.** On a pattern line, the text after the first " — " is the wrong side: shown, never tokenized into `lesson_items`. The stored form is "right — wrong" in one pattern `text` (`internal/units/doc.go` `Activity.Texts`, `internal/db/put_unit.go` `deriveLessonItems`). This initiative does not add a structured `wrong` field: no spec here owns a migration, and every unit on the path already uses the dash form. A later initiative may add a field; none of SPEC-003 through SPEC-012 does. This governs `internal/db/put_unit.go`. Rejected: a new JSON `wrong` field in this initiative.

**D3 · Learner English carries no authoring notes.** A translation or prompt says what the Hebrew means, not how the author built the step (the note "Correct form first; incorrect form second." is the example that shipped). That sentence tells the learner how the step was built. Hiding it in the player would leave it in the stored lesson, so the next copy would ship it again. The check belongs on the text. Rejected: keeping author notes in the English and hiding them in the player.

**D4 · Consolidation is review first.** This amends [ADR-0026](ADR-0026-lessons-that-build.md) D3: the same starting thresholds still ask for consolidation (more than 40 words learning, or accuracy under 75% with reviews), but at most one consolidation unit is queued ahead. Beyond that the brief returns no unit and points to review. [ADR-0026](ADR-0026-lessons-that-build.md) D4 (the 40% drill cap and the story floor) stays in force and is not amended; "pace" here is not a change to the meaning mix. A tutor-mistake pair is not reused within the next three units (`internal/brief/progression.go`). Today's `skipRightForms` only skips a right form that sits in both of the previous two units' final checkpoints, which is how the same pairs cycled. This governs `internal/brief/build.go` `PaceFor` and `internal/brief/progression.go`. Rejected: a consolidation unit whenever the backlog is large, and rejected: cycling the same two mistake pairs.

**D5 · A copy warns and a copied story blocks.** `unit.same_shape` warns when the activity-type sequence is at least 80% the same as the previous unit, which is what [INIT-030 SPEC-004](../../sdd/initiatives/INIT-030-lessons-that-build/specs/SPEC-004-content-shape-rules.md) line 31 specified. The code in `internal/units/shape_rules.go` `sameShapeIssues` currently warns when similarity is under 80%; this decision restores the spec. A story that matches an earlier unit's story after normalizing blocks. This governs `internal/units/shape_rules.go`. Rejected: warning only when a unit varies.

**D6 · Rebuild forward, one or two units at a time.** Old units are never edited in place. Every old revision is kept. The reviser rebuilds only the next one or two units and leaves the rest as drafts. This governs `.cursor/skills/hebrew-lesson-reviser/SKILL.md`. Rejected: rewriting the whole untouched backlog from one snapshot, and rejected: editing a published unit in place.

## Consequences

- INIT-037 SPEC-003 through SPEC-012 implement these decisions.
- SPEC-007 ships before INIT-036 SPEC-003, because that spec raises the Learning count `PaceFor` reads.
- Below three taught buttons, the existing reveal stands (`internal/web/lesson.go` already continues when a cloze has fewer than three choices).
- [ADR-0026](ADR-0026-lessons-that-build.md) D4 stays in force for the drill cap and story floor. D3's consolidation thresholds stay; only the queue ahead and the mistake rotation change.
- [ASMT-004](../../sdd/assessments/ASMT-004-hebrew-web-app/ASMT-004-hebrew-web-app-assessment-report.md) remains the evidence for the untaught-button fault, the inverted same-shape check, and the consolidation clone run that produced units 12–34.
