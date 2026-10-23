# SPEC-001 · data: Grammar patterns, prerequisites and explainers (migration 013)

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-017
title: "Grammar patterns, prerequisites and explainers"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: remote
security_review: standard
estimated_effort: 5 hours
blocked_by: [INIT-016/SPEC-001]
blocks: [SPEC-002, INIT-019/SPEC-001]
```

## Description

**Context:** A unit has one pattern (its `concept`, free text today). Patterns have prerequisites in the class data. Explainers are keyed to patterns (D6). Nothing stores either.

**Scope (in):** `migrations/013_patterns.sql` and `internal/db` store methods:

- `grammar_patterns`: `slug` (primary key, `[a-z0-9-]`, ≤ 60), `title_en`, `summary_en` (≤ 300), `example_he` (pointed, optional), `created_at`, `updated_at`.
- `pattern_prereqs`: (`pattern_slug`, `requires_slug`), both referencing `grammar_patterns`, no self-reference; cycles refused by the store.
- `explainers`: `pattern_slug` (primary key, references `grammar_patterns`), `title_en`, `body` (restricted markdown, ≤ 20 000 bytes), `status` (`draft` | `published`), `revision`, `updated_at`.
- `units.pattern_slug`: nullable column referencing `grammar_patterns`.
- Store: `UpsertPatterns`, `PatternBySlug`, `Patterns`, `Prereqs`, `UpsertExplainer`, `ExplainerFor(slug, publishedOnly)`, `SetUnitPattern`.

**Scope (out):** Tools (SPEC-002), rendering (SPEC-003), the body grammar check (SPEC-002).

## Acceptance Criteria

- [ ] ac-1: The migration applies to a copy of the current schema and leaves existing rows unchanged. (REQ-003)
- [ ] ac-2: Adding a prerequisite that makes a cycle is refused and writes nothing. (REQ-003)
- [ ] ac-3: `ExplainerFor(slug, true)` returns only a published explainer. (REQ-005)

## Deliverables

- [ ] `migrations/013_patterns.sql`
- [ ] `internal/db/patterns.go`, `explainers.go` and tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** `db_migration` — additive only; back up the production SQLite file before the deploy that runs it. **Rollback:** revert the app; the tables stay unused.

## Security

- **Sensitivity:** standard — schema for content shown to all learners. Length caps live in `CHECK` constraints as well as code.

## Verification Strategy

- **Claim:** patterns, prerequisites and explainers can be stored and read with the rules above.
- **Check + executor:** mechanized — `go test ./internal/db/...`, including the migration test against a seeded older database.
- **Pass condition:** the migration test, the cycle test and the published-only test pass.

## Integration Points

- SPEC-002 exposes them. INIT-019/SPEC-001 references pattern slugs from mistakes.
- **Provenance tag:** `INIT-017/SPEC-001`
