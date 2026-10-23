# INIT-016: Hybrid foundations

**Initiative ID:** INIT-016 · **Status:** complete · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main` (the single line; push publishes, a tag bump in home_k3 deploys)
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md). First of six initiatives (see its Roadmap).

## What you get

- **"You can now say"** on Progress and Today, with how many you added this week.
- **Lessons that react to how you are doing.** Missing more than 3 in 10? The next lesson leans on words you know. Getting more than 9 in 10? It adds more new ones.
- **Roots and verb patterns can be loaded through MCP**, so the class backfill can start.
- **ADR-0019** records the hybrid decisions every later initiative cites.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | docs | ADR-0019: the hybrid model | — | 2h |
| SPEC-002 | data | Roots and verb patterns in the lexicon | SPEC-001 | 3h |
| SPEC-003 | api | MCP: root and binyan on `upsert_lexemes` and `lookup` | SPEC-002 | 2h |
| SPEC-004 | docs | Class backfill runbook through MCP | SPEC-001 | 2h |
| SPEC-005 | service | "Can say" read model | SPEC-001 | 3h |
| SPEC-006 | frontend | "You can now say" on Progress and Today | SPEC-005 | 4h |
| SPEC-007 | service | Success-rate tuning in the brief | SPEC-001 | 5h |

**Total:** 21h. **Longest chain:** SPEC-001 → 005 → 006 (9h). After SPEC-001, three tracks run side by side: roots (002 → 003), can-say (005 → 006), tuning (007).

## Depends on / unblocks

- Depends on nothing.
- Unblocks INIT-017 (needs ADR-0019), INIT-018 (needs roots in the lexicon), INIT-019 (needs the MCP lexicon fields), INIT-021 (needs ADR-0019).

## Done when

- Progress shows "You can now say" and the weekly number matches the learner model.
- With accuracy below 70% over the last 40 answers, the next brief has fewer new words; above 90%, more.
- An MCP client writes a root and a binyan and reads them back.
- The backfill runbook maps all ten class tables to a tool or to the initiative that adds it.

## Notes

- No migrations: the root and binyan columns already exist.
- Operator work alongside: start the backfill (SPEC-004) once SPEC-003 is live.
- Security: SPEC-003 is an admin-only MCP write; SPEC-005/006 read only the signed-in learner's data.
