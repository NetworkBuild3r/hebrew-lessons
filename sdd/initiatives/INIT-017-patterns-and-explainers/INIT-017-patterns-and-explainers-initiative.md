# INIT-017: Grammar patterns and explainers

**Initiative ID:** INIT-017 · **Status:** planned · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md), ADR-0019 (INIT-016/SPEC-001).

## What you get

- **Explainer pages**, one idea each, in plain English with pointed examples. The lesson shows a quiet "How this works" link on the step where the pattern first appears. Five to start: present-tense agreement, past with ־תִּי, "going to" + verb, אֶפְשָׁר + verb, prepositions with endings.
- **Grammar patterns with prerequisites** as data, and a check that the default path never teaches a pattern before what it depends on.
- **Worksheet-type templates** (topic conversation, review sheet, verb workshop, short story with true/false, foundations and reading) that an authoring agent fetches over MCP.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | data | Grammar patterns, prerequisites and explainers (migration) | INIT-016/SPEC-001 | 5h |
| SPEC-002 | api | MCP: patterns, explainers, unit pattern, path-order check | SPEC-001 | 6h |
| SPEC-003 | frontend | Learn pages and the lesson link | SPEC-002 | 6h |
| SPEC-004 | api | Worksheet-type templates and the unit's type | SPEC-002 | 4h |

**Total:** 21h. **Longest chain:** SPEC-001 → 002 → 003 (17h).

## Depends on / unblocks

- Depends on INIT-016/SPEC-001 (ADR-0019).
- Unblocks INIT-019 (mistakes reference patterns; navigation links Learn), INIT-020 (the say step extends the unit schema next to `pattern`), INIT-021 (verse notes reuse the restricted text format).

## Done when

- Every published unit with a pattern that has an explainer links to it from the first step, and nowhere else.
- `check_path_order` reports every violation in the shared path with a proposed order.
- An explainer body with any raw HTML, image or outside link is refused.

## Notes

- One migration (the first free number when it lands; 013 today).
- Security: **required** for SPEC-002 and SPEC-003. Authored text reaches every learner, so it is parsed into an allow-listed node tree at write time and escaped at render.
