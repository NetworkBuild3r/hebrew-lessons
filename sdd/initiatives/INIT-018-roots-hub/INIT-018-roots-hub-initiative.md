# INIT-018: Roots hub

**Initiative ID:** INIT-018 · **Status:** complete · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md), ADR-0019 (INIT-016/SPEC-001).

## What you get

- **A page per root** (for example שׁ־מ־ע, "hear"): its words as cards with your state on each, its verb forms when loaded, the lessons that use it, and verses that contain it, Isaiah first.
- **A root link on the back of each word card**, when the root has a page.
- A root page shows only when it has at least two words or a verse, so the hub never looks empty.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | service | Root family read model | INIT-016/SPEC-002 | 4h |
| SPEC-002 | frontend | Roots hub and the word-card link | SPEC-001 | 5h |

**Total:** 9h, one chain.

## Depends on / unblocks

- Depends on INIT-016/SPEC-002 (roots in the lexicon). Useful only once the backfill has loaded roots.
- Unblocks INIT-019/SPEC-004 (navigation) and INIT-021/SPEC-003 (the Isaiah reader links words to roots).

## Done when

- Every root with two or more words, or a word and a verse, has a page; no other root does.
- A word card links to its root only when that page exists.

## Notes

- No migration. Verse lookup reuses `VersesContaining`, capped at 10 verses.
- Security: read-only pages; the root path segment is validated as Hebrew letters before lookup.
