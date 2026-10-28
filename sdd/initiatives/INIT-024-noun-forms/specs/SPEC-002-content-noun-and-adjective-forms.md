# SPEC-002 · content: Store the forms for the words already met

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-024
title: "Store the forms for the words already met"
domain: content
status: planned
primary_prompt: none
model: inherit
autonomy_level: manual_review
gate_actions: [Brian accepts the pointing table before any write]
reversibility_class: data
security_review: none
estimated_effort: 2 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003]
```

## Description

**Context:** INIT-023/SPEC-009 wrote verb forms and left twelve words unwritten. Live `lookup` on 2026-10-28 classifies them as follows. Forms are standard grammar written fresh.

**Scope (in):**
- After SPEC-001, call `upsert_noun_forms` for the nouns and adjectives in the tables below.
- Keep a plural that is already on the lexeme (`מִשְׁפָּטִים`, `שְׁלָגִים`). Add it as a form row so the word page and `lookup` agree.
- Leave construct and possessive off a word when the table has a dash. The word page omits an empty row.

**Scope (out):** הַכּוֹל (pronoun, "everything") and יָשָׁר (adverb, "straight ahead"). No paradigm. Words the learner has not met. Verb forms already stored. Anything copied from a class worksheet.

### Nouns

Gender and gloss are the live lexicon. Construct and possessive are included only where the form is ordinary.

| Lemma | Gloss | Singular | Plural | Definite sg | Definite pl | Construct sg | Construct pl |
| --- | --- | --- | --- | --- | --- | --- | --- |
| עִבְרִית | Hebrew (the language), f | עִבְרִית | — | הָעִבְרִית | — | — | — |
| מִשְׁפָּט | sentence, m | מִשְׁפָּט | מִשְׁפָּטִים | הַמִּשְׁפָּט | הַמִּשְׁפָּטִים | מִשְׁפַּט | מִשְׁפְּטֵי |
| שֶׁלֶג | snow, m | שֶׁלֶג | שְׁלָגִים | הַשֶּׁלֶג | הַשְּׁלָגִים | — | — |
| סֵפֶר | book, m | סֵפֶר | סְפָרִים | הַסֵּפֶר | הַסְּפָרִים | סֵפֶר | סִפְרֵי |
| מָרָק | soup, m | מָרָק | מְרָקִים | הַמָּרָק | הַמְּרָקִים | — | — |

Possessive, singular owned, for מִשְׁפָּט and סֵפֶר only:

| Owner | מִשְׁפָּט | סֵפֶר |
| --- | --- | --- |
| my | מִשְׁפָּטִי | סִפְרִי |
| your (m) | מִשְׁפָּטְךָ | סִפְרְךָ |
| your (f) | מִשְׁפָּטֵךְ | סִפְרֵךְ |
| his | מִשְׁפָּטוֹ | סִפְרוֹ |
| her | מִשְׁפָּטָהּ | סִפְרָהּ |
| our | מִשְׁפָּטֵנוּ | סִפְרֵנוּ |
| your (m pl) | מִשְׁפַּטְכֶם | סִפְרְכֶם |
| your (f pl) | מִשְׁפַּטְכֶן | סִפְרְכֶן |
| their (m) | מִשְׁפָּטָם | סִפְרָם |
| their (f) | מִשְׁפָּטָן | סִפְרָן |

### Adjectives

Four agreement cells. No construct and no possessive.

| Lemma | Gloss | M sg | F sg | M pl | F pl |
| --- | --- | --- | --- | --- | --- |
| טוֹב | good | טוֹב | טוֹבָה | טוֹבִים | טוֹבוֹת |
| רַע | bad | רַע | רָעָה | רָעִים | רָעוֹת |
| לָבָן | white | לָבָן | לְבָנָה | לְבָנִים | לְבָנוֹת |
| נוֹחַ | comfortable | נוֹחַ | נוֹחָה | נוֹחִים | נוֹחוֹת |
| מְיוּחָד | special | מְיוּחָד | מְיוּחֶדֶת | מְיוּחָדִים | מְיוּחָדוֹת |

## Acceptance Criteria

- [ ] ac-1: Brian has accepted the two tables before any `upsert_noun_forms` call.
- [ ] ac-2: `lookup` returns a plural for מִשְׁפָּט, שֶׁלֶג, סֵפֶר, and מָרָק, and no plural for עִבְרִית.
- [ ] ac-3: `lookup` returns four agreement forms for טוֹב, רַע, לָבָן, נוֹחַ, and מְיוּחָד.
- [ ] ac-4: `lookup` of הַכּוֹל and יָשָׁר returns no forms.

## Deliverables

- [ ] MCP writes only: `upsert_noun_forms`
- [ ] Lemma ids recorded in this spec after the write

## Verification Strategy

- `lookup` each of the twelve words and compare the form list to the tables.

## Integration Points

- `INIT-024/SPEC-002`
