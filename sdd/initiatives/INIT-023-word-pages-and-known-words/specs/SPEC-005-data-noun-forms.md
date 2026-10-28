# SPEC-005 · data: Store noun and adjective forms

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-023
title: "Store noun and adjective forms"
domain: data
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [backup production database before deploy]
reversibility_class: data
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-006, SPEC-008]
```

## Description

**Context:** Lexemes have `gender`, `plural`, and `irregular_plural`. There is nowhere to keep construct forms, possessive forms, or an adjective's four agreement forms. `verb_forms` (migration 014) is the model to follow.

**Scope (in):**
- Migration `019_noun_forms.sql`, additive:

  ```sql
  CREATE TABLE noun_forms (
      id INTEGER PRIMARY KEY,
      lemma_lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE CASCADE,
      kind TEXT NOT NULL CHECK (kind IN ('singular', 'plural', 'definite', 'construct', 'possessive', 'agreement')),
      number TEXT NOT NULL DEFAULT '' CHECK (number IN ('', 'sg', 'pl')),
      person TEXT NOT NULL DEFAULT '' CHECK (person IN ('', '1', '2', '3')),
      gender TEXT NOT NULL DEFAULT '' CHECK (gender IN ('', 'm', 'f')),
      owner_number TEXT NOT NULL DEFAULT '' CHECK (owner_number IN ('', 'sg', 'pl')),
      form_he TEXT NOT NULL CHECK (length(form_he) BETWEEN 1 AND 200),
      form_translit TEXT NOT NULL DEFAULT '' CHECK (length(form_translit) <= 200),
      form_lexeme_id INTEGER REFERENCES lexemes (id) ON DELETE SET NULL,
      UNIQUE (lemma_lexeme_id, kind, number, person, gender, owner_number)
  );
  ```

  `possessive` uses `person`, `gender`, and `owner_number` for the owner (שֶׁלִּי → 1, '', sg) and `number` for the thing owned. `agreement` is an adjective's m. sg., f. sg., m. pl., and f. pl.
- `db.NounForm`, `UpsertNounForms` (all or nothing, lemma must exist, a repeat key replaces), `NounFormsFor(lemmaID)` in a fixed order: singular, definite, plural, construct sg, construct pl, possessive by owner, agreement.
- `VerbFormsFor` and `NounFormsFor` also return each form's `audio_id` through the lexicon or `audio_assets` by text, so pages can render a clip.

**Scope (out):** Generating forms automatically.

## Acceptance Criteria

- [ ] ac-1: The migration applies to a database at 018 and keeps every row.
- [ ] ac-2: An upsert with one missing lemma writes nothing.
- [ ] ac-3: A repeat of the same key replaces `form_he`.
- [ ] ac-4: `NounFormsFor` returns the fixed order for a fixture with every kind.
- [ ] ac-5: A form whose text has a clip comes back with its audio id. A form with no clip has an empty one.

## Deliverables

- [ ] `migrations/019_noun_forms.sql`
- [ ] `internal/db/nounforms.go` and `nounforms_test.go`
- [ ] `internal/db/verbforms.go` (audio id)
- [ ] `internal/db/migrate_test.go`

## Verification Strategy

- **Claim:** noun and adjective forms have a home and come back in reading order with their audio.
- **Check + executor:** mechanized — `go test ./internal/db`.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-005`
