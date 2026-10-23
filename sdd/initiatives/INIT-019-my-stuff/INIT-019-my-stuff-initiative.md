# INIT-019: My stuff — mistakes, verb tables, and section navigation

**Initiative ID:** INIT-019 · **Status:** complete · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md), ADR-0019 (INIT-016/SPEC-001).

## What you get

- **Mistakes you keep making**, private to you: what you said, what is right, how often. A tutor's agent records them over MCP, and your next lessons practise the right form (up to 3 per lesson).
- **"Learners often say"** on explainer pages: a general list, never anyone's actual answers.
- **Verb tables** for the verbs you know, with the forms you know marked.
- **A word bank** of every word you have met, by state.
- **Navigation by section** (Path, Learn, Roots, Isaiah, My stuff), showing only sections that have content.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | data | Mistakes and verb forms (migration) | INIT-017/SPEC-001 | 5h |
| SPEC-002 | api | MCP: mistakes and verb forms; brief and learner model read personal mistakes | INIT-016/SPEC-003, SPEC-001 | 6h |
| SPEC-003 | frontend | My stuff: mistakes, verb tables, word bank | SPEC-002 | 6h |
| SPEC-004 | frontend | Navigation around the sections | INIT-017/SPEC-003, INIT-018/SPEC-002, SPEC-003 | 4h |

**Total:** 21h. **Longest chain:** SPEC-001 → 002 → 003 → 004 (21h).

## Depends on / unblocks

- Depends on INIT-016 (MCP lexicon fields), INIT-017 (patterns; Learn pages for navigation), INIT-018 (roots hub for navigation).
- Unblocks nothing; it closes the section structure.

## Done when

- A tutor's agent records a mistake for one learner, it shows in that learner's My stuff, the next brief practises it, and no other learner can see it.
- Each known verb has a table with exactly the known forms marked.
- Learn, Roots and Isaiah appear in navigation only when they have content.

## Notes

- One migration (next free number when it lands; 014 today).
- Security: **required** for SPEC-002 — private learner data over MCP. The learner id comes from the token; personal and general lists are separate tables with no copy path between them.
- Content depends on the backfill. Pages show plain empty states until data arrives.
