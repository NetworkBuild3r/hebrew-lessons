# SPEC-007 · frontend: Word page with the verb view

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-023
title: "Word page with the verb view"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-008, SPEC-009, SPEC-010]
```

## Description

**Context:** A verb's forms appear only as a flat list on `/me/verbs`, with no audio and no tense grouping. A word card shows one form. Brian wants to see every form of a verb in a conjugation table, hear each one, see the tense, binyan, and root, and see the verb used.

**Scope (in):**
- `GET /words/{id}` for a signed-in learner. An unknown id is 404. The page picks its view from `pos`: verb, noun, adjective, or a plain view for anything else.
- **Header:** the lemma (infinitive for a verb) large, with its clip; transliteration; meaning; then chips for part of speech, binyan, and root. The root chip links to `/roots/{root}`. The learner's state per skill sits under the header.
- **Present** (labelled "Present tense · הוֹוֶה"): a four-column grid, Singular M, Singular F, Plural M, Plural F.
- **Past** ("Past tense · עָבָר") and **Future** ("Future tense · עָתִיד"): rows for I, you m., you f., he, she, we, you m. pl., you f. pl., they m., they f., with the pronoun in Hebrew beside the English. The cells follow the rows. A tense with no stored forms is left out.
- **Imperative** and **Infinitive** below.
- Every cell is a `clip` with its own audio when a clip exists. A form the learner knows (its form lexeme is known or solid in any skill) gets a mark and an accessible label "you know this form".
- **In your lessons:** up to 8 lines from published units the learner can see whose text contains any form of the word, each with its English and a link to the lesson. The matched form is highlighted.
- Links in: word card "Word details" gets "All forms", plus each `/me/words` row, each `/me/verbs` table, and each Roots word.
- On mobile the grid scrolls sideways inside its own box, and the header stays readable at 320px.

**Scope (out):** Noun and adjective layouts (SPEC-008). Showing another learner's lessons.

## Acceptance Criteria

- [ ] ac-1: `/words/{id}` for a fixture לִבְחוֹר with present forms renders the four-cell grid in the order m. sg., f. sg., m. pl., f. pl., each cell with `<audio>` when the fixture has a clip.
- [ ] ac-2: Past forms render under "Past tense" with the person rows in the fixed order. A tense with no rows renders nothing.
- [ ] ac-3: A known form carries the known mark and label. An unknown form does not.
- [ ] ac-4: "In your lessons" lists only units the learner may read, and never more than 8.
- [ ] ac-5: The root chip links to the Roots page for that root.
- [ ] ac-6: A word card's "All forms" link and the `/me/verbs` table link go to `/words/{id}`.
- [ ] ac-7: The browser check taps a present-tense cell and the orb reaches `playing`. At 360px wide the page has no horizontal scroll outside the grid.

## Deliverables

- [ ] `internal/web/word_page.go` and `word_page_test.go`
- [ ] `internal/web/shell.go` (route)
- [ ] `internal/web/views/word.templ`, `word_templ.go`, and `word.go`
- [ ] `internal/web/static/css/word.css`
- [ ] `internal/web/views/player.go` (All forms link in Word details)
- [ ] `internal/web/views/me.templ`, `roots.templ` (links)
- [ ] `internal/web/testdata/word-page-browser.mjs`

## Verification Strategy

- **Claim:** one page shows every stored form of a verb, playable, by tense.
- **Check + executor:** mechanized — `go test ./internal/web ./internal/web/views`, then the browser script under Node with Playwright.
- **Pass condition:** ac-1 to ac-7 pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-007`
