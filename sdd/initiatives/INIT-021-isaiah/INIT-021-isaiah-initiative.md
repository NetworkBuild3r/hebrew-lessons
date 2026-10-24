# INIT-021: Isaiah, verse by verse

**Initiative ID:** INIT-021 · **Status:** complete · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md), ADR-0019 (INIT-016/SPEC-001).

## What you get

- **An Isaiah reader**, one verse at a time: the pointed text large; tap a word for its gloss and root; then the layers — roots, "the manner of the Jews" (parallel lines, wordplay and sound play, idioms, how Jewish tradition reads the verse), and a bridge to modern Hebrew. The English is behind a toggle.
- **A human check before anything shows.** An agent drafts each verse over MCP; only a signed-in admin pressing "Checked" on a web page publishes it. No MCP tool can do that.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | data | Isaiah verse notes (migration) | INIT-016/SPEC-001 | 3h |
| SPEC-002 | api | MCP: verse notes, drafted and checked | SPEC-001 | 3h |
| SPEC-003 | frontend | Isaiah reader | INIT-018/SPEC-001, SPEC-002 | 6h |

**Total:** 12h, one chain.

## Depends on / unblocks

- Depends on INIT-016/SPEC-001 (ADR-0019), INIT-017/SPEC-002 (the restricted text format, reused for notes), INIT-018/SPEC-001 (root links). The Tanakh text itself is already imported (`tanakh_verses`).
- Ongoing work outside the code: the Isaiah agent drafts, and a person who reads Hebrew checks, one verse at a time.

## Done when

- A draft note never appears to a learner; a checked one does.
- Each checked word gloss is tappable and links to its root page when that page exists.

## Notes

- Migration 017 is in. Isaiah stays hidden until a verse is checked. No class notes were loaded.
- Security: **required** for SPEC-002 — agent-written content reaches learners only through the human check, which needs an admin web session and CSRF. No security-review registry is in this repo. The check route refuses a non-admin, and the session middleware requires CSRF.
