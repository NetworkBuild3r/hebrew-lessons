# SPEC-001 completion summary

**Spec:** SPEC-001 · ADR-0007 single-page app structure  
**Initiative:** INIT-005  
**Completed:** 2026-10-09  
**Status:** complete

## What shipped

- `docs/decisions/ADR-0007-single-page-app-structure.md` (Accepted): one shell document, hand-built History API router, view `mount`/`unmount`, server-embedded `/api/me`-shaped user for first paint, route-change a11y, 401 → login with safe `next`, legacy-lesson rule with SPEC-002 fallback, Option A stack kept.
- ADR-0005 metadata **Superseded by** note and changelog row (page model only; Option A unchanged).

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass — Decisions 2 and 4 |
| ac-2 | Pass — Decisions 3 and 6 |
| ac-3 | Pass — Decision 5 |
| ac-4 | Pass — Decision 7 |
| ac-5 | Pass — ADR-0005 metadata + changelog |

## Follow-ups

- SPEC-002 implements the shell and router and confirms the legacy-lesson fate.
- INIT-004 SPEC-013–016 should be re-read as views before they start (not edited here).

## Provenance

`INIT-005/SPEC-001`
