# ADR-0022: Learner day and outside teaching

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-01 |
| **Authored by** | INIT-026 SPEC-001 |
| **Initiative** | INIT-026 (tutor-data-loop), SPEC-001 |
| **Extends** | [ADR-0018](ADR-0018-v3-adaptive-learning.md) (word × skill states and FSRS cards) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

Each learner's calendar day and review study day use one IANA zone: the learner's `users.time_zone`, or the app zone from `TZ` when the learner has none. UTC is never a fallback. Outside teaching (tutor sessions, Grok conversations, worksheets) enters only as taught words and dated corrections over MCP; it schedules new cards and never grades or moves existing ones. The app does not store session media or tutor session records. A learner may write coach notes about themselves, revising the admin-only rule from INIT-015/SPEC-021.

## Context

Days were computed in three zones. North Star §4.2 makes every answer an event, and the evidence package is the only card writer, but nothing said what outside teaching may do to cards. Coach notes were admin only (INIT-015/SPEC-021). Tutor and worksheet work needed a path into scheduling without becoming a second grader or a store of recordings and transcripts.

## Decision

**D1 · Learner day.** Days are computed in the learner's IANA zone (`users.time_zone`). A learner with no zone uses the app zone from `TZ`. Every MCP answer that names a day returns `zone` and `zoneSource`. UTC is never a fallback. Calendar days start at midnight. The review study day rolls at 4:00. Both use the same zone. Rejected: inventing today in UTC when the learner zone is missing, and rejected: different zones for calendar day and study day.

**D2 · Outside teaching.** Tutor sessions, Grok conversations, and worksheets enter only as taught words and dated corrections, written over MCP. A taught word gets `new` cards due at the next study-day cutoff. Outside teaching never grades a card and never moves an existing card. Undo removes only cards the same taught row created that have no review. Rejected: letting an outside session grade or reschedule cards already under FSRS, and rejected: storing the session itself as evidence.

**D3 · What the app does not store.** The app does not store recordings, transcripts, worksheet files, links to them, tutor names, or session records. The calling LLM keeps them. Rejected: mirroring tutor session media or session metadata into the learner database.

**D4 · Learner coach notes.** A learner may write notes about themselves. They are stored with `author_role = learner`, quoted as learner text in the brief, read only into that learner's brief, and capped at 20 per 24 hours. This revises INIT-015/SPEC-021, which made coach notes admin only. Rejected: keeping `add_coach_note` admin-only, and rejected: mixing learner notes into another learner's brief.

## Consequences

- North Star sections 4.2 and 8 follow this record.
- MCP day-named answers carry `zone` and `zoneSource`; scheduling and digests share one learner (or app) zone.
- Outside teaching tools schedule through taught-word rows; the evidence package remains the only path that grades or moves cards from in-app answers.
- `add_coach_note` accepts learner authorship under the cap and brief-scoping rules above.
- Later code specs (INIT-026) implement the zone helpers, taught-word MCP tools, and the coach-note policy change against this record.
