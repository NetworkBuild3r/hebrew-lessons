# ADR-0019: The hybrid learning model

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-23 |
| **Authored by** | INIT-016 SPEC-001, recording the decisions in the design doc |
| **Initiative** | INIT-016 (hybrid-foundations), SPEC-001 |
| **Extends** | [ADR-0018](ADR-0018-v3-adaptive-learning.md) (the learner model, the brief, and MCP authoring) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A lesson is decided by three sources working together. Research rules live in the code and apply to everyone. The learner's own record tunes the mix. A tutor, when there is one, adds class material through MCP. ADR-0018 built the record and the brief. This record says how the course is shaped around them. It changes no code.

The design this records is [the hybrid model](../design/learner-without-a-tutor.md).

## Decisions

Each decision names the option chosen and one option rejected.

**D1 · Three sources decide a lesson.** The code holds the research rules, the learner's record tunes the lesson, and a tutor contributes through MCP when there is a tutor. Rejected: one fixed course that is identical for every learner.

**D2 · Class data arrives through MCP and is not loaded yet.** The spreadsheet stays on the operator's machine. It is loaded through MCP once the backfill is under way, and it is never committed to the repo. Rejected: committing the spreadsheet, and rejected: loading the four-class sample before the backfill has started.

**D3 · Roots and verb patterns are written through the lexicon.** `upsert_lexemes` gains `root` and `binyan`, and `lookup` returns them. How many roots the live word list already has is a question the database answers, not an assumption. Rejected: a separate roots table beside the lexicon.

**D4 · Explainers are keyed to grammar patterns.** A lesson links to its explainer because the pattern says so. Rejected: pages linked by hand.

**D5 · A root page appears only when the root has at least two words or a verse.** Rejected: a page for every root from the start, which would be mostly empty.

**D6 · The default path starts from class order and is reordered by prerequisites.** No pattern is taught before what it depends on. Rejected: copying the class order exactly.

**D7 · Two mistakes lists.** The shared course gets a general list that says what learners often do and never quotes anyone's answer. Each learner also has a personal list, private to them, that the tuned path reads. Rejected: one shared list built from real answers.

**D8 · The content is original.** Lessons follow the structure and order of the class material. The class sheets are not republished. Rejected: copying the sheets into the course.

**D9 · "You can now say" is computed.** The recall record already shows which words a learner can produce, so the count and the weekly change come from it. Rejected: a list the learner maintains by hand.

**D10 · The brief tunes its mix to recent accuracy.** Over the last 40 checked attempts, below 70% right leans on what is known, above 90% adds new material, and the band between stays as it is. The mix changes only after the rate clears the line by 3 points, and fewer than 15 attempts always stays steady. Rejected: the same number of new words for every learner regardless of how they are doing.

## Consequences

- INIT-016 makes roots writable, computes what a learner can say, and tunes the brief. It writes the runbook for the backfill and does not run it.
- INIT-017 through INIT-021 build the patterns, the roots hub, the personal pages, the speaking step, and the Isaiah reader on top of these decisions.
- The voice service is low on credits and has no second voice, so anything that needs new audio waits on that. This record does not change the voice service.

## Specs

INIT-016 SPEC-001 (this record), SPEC-002 (roots and verb patterns), SPEC-003 (MCP), SPEC-004 (backfill runbook), SPEC-005 (what you can say), SPEC-006 (the page), SPEC-007 (accuracy tuning).
