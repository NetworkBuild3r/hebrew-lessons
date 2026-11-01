# SPEC-001 · docs: ADR-0022, each learner's day, and outside teaching that schedules but never grades

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-026
title: "ADR-0022: each learner's day, and outside teaching that schedules but never grades"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-005]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:** Days are computed in three zones (initiative "What we found"). North Star §4.2 makes every answer an event, and the evidence package (`internal/evidence/doc.go`) is the only card writer. Nothing says what outside teaching may do. INIT-015/SPEC-021 made coach notes admin only (`internal/mcp/policy.go:20`).

**Scope (in):**
- `docs/decisions/ADR-0022-learner-day-and-outside-teaching.md`, laid out like ADR-0021, with four decisions:
  1. **Learner day.** A learner's days are computed in their IANA zone (`users.time_zone`). A learner with no zone uses the app zone from `TZ`. Every MCP answer that names a day returns `zone` and `zoneSource`. UTC is never a fallback. Calendar days start at midnight. The review study day rolls at 4:00. Both use the same zone.
  2. **Outside teaching.** Tutor sessions, Grok conversations, and worksheets enter only as taught words and dated corrections, written over MCP.
     - A taught word gets `new` cards due at the next study-day cutoff.
     - Nothing outside the app grades a card or moves an existing card.
     - Undo removes only cards the same taught row created that have no review.
  3. **What the app does not store.** Recordings, transcripts, worksheet files, links to them, tutor names, and session records. The calling LLM keeps them.
  4. **Learner coach notes.** A learner may write notes about themselves. They are stored with `author_role = learner`, quoted as learner text in the brief, read only into that learner's brief, and capped at 20 per 24 hours. This revises INIT-015/SPEC-021.
- `docs/north-star.md`:
  - §4.2 gets one paragraph on outside teaching (decision 2).
  - §8 lists `set_time_zone`, `get_study_digest`, `record_taught_words`, `remove_taught_words`, and the `hebrew://tutor` resource, with one line each.

**Scope (out):** Code. The `evidence/doc.go` comment (SPEC-005). The `policy.go` comment (SPEC-008).

## Acceptance Criteria

- [ ] ac-1: ADR-0022 exists with Status Accepted, Initiative INIT-026 SPEC-001, Extends ADR-0018, and the four decisions above.
- [ ] ac-2: Decision 2 says in one sentence that outside teaching never grades a card and never moves an existing card.
- [ ] ac-3: Decision 3 lists recordings, transcripts, worksheet files, links, tutor names, and session records as not stored.
- [ ] ac-4: North Star §4.2 and §8 carry the paragraph and the new tool rows, and §8's `add_coach_note` row no longer implies admin only.

## Deliverables

- [ ] `docs/decisions/ADR-0022-learner-day-and-outside-teaching.md`
- [ ] `docs/north-star.md` (§4.2, §8)

## Verification Strategy

- **Claim:** the two rules the code specs implement are recorded where the next author looks first.
- **Check + executor:** human. A reviewer who did not write the ADR reads it against this spec's four decisions. Mechanized: `rg -n 'ADR-0022' docs/decisions docs/north-star.md` and `rg -n 'never grades' docs/decisions/ADR-0022-*.md`.
- **Pass condition:** both `rg` calls match, and the reviewer can point to each of ac-1 to ac-4 in the text. A missing decision fails.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-001`
- SPEC-005 and SPEC-008 cite ADR-0022 in their code comments.
