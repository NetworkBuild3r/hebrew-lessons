# SPEC-008 · frontend: Noun and adjective views

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-023
title: "Noun and adjective views"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 5 hours
blocked_by: [SPEC-005, SPEC-007]
blocks: [SPEC-009]
```

## Description

**Context:** A noun does not conjugate. What a learner needs is its gender, how it pluralizes, how it joins another noun, and how it takes an owner. An adjective agrees in gender and number with its noun.

**Scope (in):**
- **Noun header:** the word, its clip, the meaning, and a gender chip (masculine, feminine, or both). An irregular plural gets a chip "irregular plural".
- **Forms block:** a two-column grid, Singular and Plural. Rows: the word, with הַ (definite), and construct ("… of"). Every cell plays.
- **Possessive block:** rows my, your m., your f., his, her, our, your m. pl., your f. pl., their m., their f. The Hebrew pronoun shows beside the English. One column for a singular thing owned and one for plural. When there are no suffix forms, the block shows the שֶׁל phrase for each owner from stored `possessive` rows.
- **Adjective view:** the same four-column grid as the present tense (Singular M, Singular F, Plural M, Plural F), with a line under it showing the adjective after a noun of each gender when the unit lines have one.
- "In your lessons" and the known marks work as in SPEC-007.
- Lexeme `plural` fills the plural cell when `noun_forms` has no plural row.

**Scope (out):** Forms for prepositions with suffixes (לִי, לְךָ). They can use the possessive layout later.

## Acceptance Criteria

- [ ] ac-1: A fixture noun with every kind renders the forms grid and the possessive grid in the fixed row order.
- [ ] ac-2: A noun with only `lexemes.plural` set shows singular and plural, and no empty construct or possessive block.
- [ ] ac-3: The gender chip and the irregular-plural chip follow the lexeme.
- [ ] ac-4: An adjective renders four agreement cells, each with its clip when one exists.
- [ ] ac-5: The browser check taps a possessive cell and the orb reaches `playing`.

## Deliverables

- [ ] `internal/web/word_page.go` (noun and adjective builders) and tests
- [ ] `internal/web/views/word.templ`, `word_templ.go`
- [ ] `internal/web/static/css/word.css`
- [ ] `internal/web/testdata/word-page-browser.mjs`

## Verification Strategy

- **Claim:** a noun and an adjective each have a view that fits how they change.
- **Check + executor:** mechanized — `go test ./internal/web ./internal/web/views` and the browser script.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-008`
