# ADR-0026: Lessons that build on each other

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-05 |
| **Authored by** | INIT-030 SPEC-001 |
| **Initiative** | INIT-030 (lessons-that-build), SPEC-001 |
| **Extends** | [ADR-0023](ADR-0023-review-that-moves-on.md) (review accuracy bands), [ADR-0025](ADR-0025-practice-that-teaches.md) (practice that teaches) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

The lesson brief plans the next grammar step, the pace, and what to recycle; the AI turns that plan into a story. One new pattern per unit, two earlier ones reused. New words follow accuracy and the learning backlog, with consolidation when the backlog is large. About three quarters of a unit is meaning-focused work: a story floor by tier, a 40% cap on single-word drills, and recycling of previous units' words. Mistakes rotate. Only untouched units are revised, and new briefs wait while three or more untouched units already sit on Today.

## Context

Learner units share one skeleton: a short story, the same three steps per new word, one pattern that never returns, and the same three fix-the-mistake pairs. The brief is a word list. It does not name a grammar step, the previous unit's story, or whether the pace should slow. Writers repeat the shape they used last time.

[ADR-0023](ADR-0023-review-that-moves-on.md) already bands new-card intake by the last fifty reviews. [ADR-0025](ADR-0025-practice-that-teaches.md) adds sentence build and later-day results by kind of practice. Neither record plans progression across units.

[RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) supports four strands, meaning-focused work on known words, studied language coming back in later meaning-focused work, and a fluency pass with no new language. It grades the exact three-quarters / one-quarter share **Low**. [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) supports spacing (**High**) and gives no fixed count of encounters. The pattern order and every numeric threshold below are product choices or starting values unless a research grade is named.

## Decision

**D1 · The code plans the progression. The AI writes it.** Choosing the next pattern, the pace, and what to recycle is counting, so it lives in the brief and is testable. The writer turns that plan into a story. This follows north-star principle 1. Rejected: leaving progression choices to the writer's prompt.

**D2 · One new pattern per unit, two old ones reused.** The next pattern is the first, in a fixed order, whose prerequisites have all been taught in this learner's earlier units and which has not been taught yet. The two most recent patterns are named for reuse in the story. [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) says studied language should come back in later meaning-focused work. The one-new / two-reused counts and the fixed order are product choices. No trial ranks grammar orders ([RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md), **Low**). Rejected: one pattern with no reuse, and rejected: inventing the next pattern in the prompt.

**D3 · Pace from accuracy and backlog, with consolidation.** New words per unit use the same accuracy bands as review ([ADR-0023](ADR-0023-review-that-moves-on.md)): 2 below 80% on the last 50 reviews, 5 from 80% to 90%, and the tier's count above 90%. A consolidation unit is asked for when learning words exceed 40 or accuracy is under 75%: no new words, a longer story of words the learner is learning, and checked questions on them. The accuracy bands match ADR-0023. The consolidation thresholds of 40 learning words and 75% accuracy are starting values, not findings. Rejected: a fixed new-word count from the tier alone.

**D4 · About three quarters meaning.** At most 40% of a unit's activities may be single-word drills (a bare-word `shadow`, `listen_pick`, `recall`, or `produce`). Each lesson needs a story or dialogue of at least the brief's `minStoryTokens`: 12 Hebrew words for beginner, 30 for developing, and 50 for established, capped by `maxTextTokens`. [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) gives the three-quarters split as a planning rule, **Low** for the exact share, so the 40% drill cap is a product choice. The story floors of 12, 30, and 50 are product choices. Spacing of recycled words remains [RSCH-003](../../sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) (**High**); there is no fixed encounter count. Rejected: lessons that are mostly bare-word drill, and rejected: a three-word story as the main text.

**D5 · Recycle the previous units' words.** At least four of the previous two units' new words appear in this unit's stories, dialogues, sentence questions, or `order` steps. This is the validator's `brief.recycle_previous`. The floor of four is a product choice. It follows studied language coming back in later meaning-focused work in [RSCH-004](../../sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md). Rejected: a unit that never uses the previous units' new words.

**D6 · Mistakes rotate.** A mistake pair that appeared in each of the last two units is skipped for one unit. A pair answered right three times since it was recorded is retired. The brief still caps `fixMistakes` at three. The look-back of two units, the skip of one unit, the retirement after three right answers, and the cap of three are product choices. Rejected: the same three pairs in every unit.

**D7 · Only untouched units are revised.** A unit with any `lesson_runs` row for that learner is started, and revision refuses it. That keeps every attempt pointing at the lesson it was made on. An untouched unit has no attempts, so a new revision loses nothing. The old revision stays in `unit_revisions` and can be republished. Rejected: rewriting a unit the learner has started or finished.

**D8 · Revise before you add.** `get_lesson_brief` for a new unit refuses when three or more untouched units exist. The agent revises the earliest one instead. The cap of three is a product choice. Rejected: piling new drafts on top of an untouched backlog.

## Consequences

- North Star section 7.1 adds the next pattern, reuse patterns, previous units, pace, and consolidation to the lesson brief. Section 7.2 adds the story floor, the drill cap, and recycling.
- [ADR-0023](ADR-0023-review-that-moves-on.md) and [ADR-0025](ADR-0025-practice-that-teaches.md) are unchanged: accuracy bands still drive intake, and practice layouts stay as decided there.
- Later INIT-030 specs implement unit progress, the progression brief, validator shape rules, revision over MCP, and revising learner 1's untouched units against this record.
