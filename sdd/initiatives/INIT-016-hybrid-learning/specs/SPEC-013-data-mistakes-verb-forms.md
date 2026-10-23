# SPEC-013 · data: Mistakes and verb forms (migration 014)

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-016
title: "Mistakes and verb forms"
domain: data
status: planned
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: remote
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-007]
blocks: [SPEC-014]
```

## Description

**Context:** Two mistakes lists (D5) and verb tables need storage. Confusions already exist per learner (`confusion_misses`), but they record mixed-up words, not a tutor's "you said X, it is Y".

**Scope (in):** `migrations/014_mistakes_verbs.sql` and store methods:

- `learner_mistakes`: `id`, `learner_id` (cascade on user delete), `wrong_he`, `right_he` (pointed, ≤ 200 each), `note_en` (≤ 300), `pattern_slug` (nullable, references `grammar_patterns`), `lexeme_id` (nullable), `times` (≥ 1), `first_at`, `last_at`, `source` (`tutor` | `app`). Unique on (`learner_id`, `wrong_he`, `right_he`); a repeat increments `times` and `last_at`.
- `common_mistakes`: `id`, `pattern_slug` (nullable), `wrong_he`, `right_he`, `explain_en`, `status` (`draft` | `published`). No learner column, by design.
- `verb_forms`: `lemma_lexeme_id` (references `lexemes`), `tense` (`present` | `past` | `future` | `imperative` | `infinitive`), `person` (`1` | `2` | `3` | empty), `number` (`sg` | `pl` | empty), `gender` (`m` | `f` | empty), `form_he` (pointed), `form_translit`, `form_lexeme_id` (nullable). Unique on (lemma, tense, person, number, gender).
- Store: `RecordMistakes(learnerID, ...)`, `MistakesFor(learnerID, limit)`, `UpsertCommonMistakes`, `CommonMistakesFor(patternSlug)`, `UpsertVerbForms`, `VerbFormsFor(lemmaID)`.

**Scope (out):** Tools (SPEC-014), pages (SPEC-015).

## Acceptance Criteria

- [ ] ac-1: The migration applies to a copy of the current schema and changes no existing rows.
- [ ] ac-2: Recording the same mistake twice for a learner yields one row with `times = 2`; another learner's identical mistake is a separate row. (REQ-008)
- [ ] ac-3: `common_mistakes` has no learner column, and no store method copies a `learner_mistakes` row into it. (REQ-009)
- [ ] ac-4: Verb forms upsert by their unique key and read back in tense and person order. (REQ-010)

## Deliverables

- [ ] `migrations/014_mistakes_verbs.sql`
- [ ] `internal/db/mistakes.go`, `verbforms.go` and tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** `db_migration` — additive; back up before deploy. **Rollback:** revert the app; tables stay unused.

## Security

- **Sensitivity:** standard — private learner data. Every personal read and write takes the learner id as a parameter from the caller's verified identity; no method lists mistakes across learners.

## Verification Strategy

- **Claim:** mistakes and verb forms are stored with the privacy split intact.
- **Check + executor:** mechanized — `go test ./internal/db/...` with the migration test.
- **Pass condition:** ac-1 to ac-4 tests pass.

## Integration Points

- SPEC-014 exposes them; SPEC-010 reads verb forms into root pages.
- **Provenance tag:** `INIT-016/SPEC-013`
