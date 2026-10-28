# SPEC-009 · content: Forms and audio for the words already taught

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-023
title: "Forms and audio for the words already taught"
domain: content
status: blocked
primary_prompt: none
model: inherit
autonomy_level: manual_review
gate_actions: [Brian checks the pointed forms before audio is generated]
reversibility_class: data
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-006, SPEC-007]
blocks: []
```

## Description

**Context:** The word page is only as full as the forms stored. Brian named five verbs. Learner 1 has 35 words with evidence, among them עוֹבֵד, לוֹמֵד, שׁוֹמֵעַ, בּוֹחֵר, and מַסְכִּים.

**Scope (in):**
- Look up each lemma with `lookup`, and add any missing lemma with `upsert_lexemes` (pos, root, binyan, translit).
- Verbs: present (4), past (10), future (10), imperative (up to 5), and infinitive for לְהַחְלִיט (ח.ל.ט, hif'il), לִבְחוֹר (ב.ח.ר, pa'al), לְהַאֲמִין (א.מ.נ, hif'il), לְנַצֵּחַ (נ.צ.ח, pi'el), and לְלַמֵּד (ל.מ.ד, pi'el). Then every other verb learner 1 has seen.
- Present forms for the five, as a check on pointing:

  | Verb | M. sg. | F. sg. | M. pl. | F. pl. |
  | --- | --- | --- | --- | --- |
  | לְהַחְלִיט | מַחְלִיט | מַחְלִיטָה | מַחְלִיטִים | מַחְלִיטוֹת |
  | לִבְחוֹר | בּוֹחֵר | בּוֹחֶרֶת | בּוֹחֲרִים | בּוֹחֲרוֹת |
  | לְהַאֲמִין | מַאֲמִין | מַאֲמִינָה | מַאֲמִינִים | מַאֲמִינוֹת |
  | לְנַצֵּחַ | מְנַצֵּחַ | מְנַצַּחַת | מְנַצְּחִים | מְנַצְּחוֹת |
  | לְלַמֵּד | מְלַמֵּד | מְלַמֶּדֶת | מְלַמְּדִים | מְלַמְּדוֹת |

- Nouns and adjectives learner 1 has seen: plural, definite, construct singular and plural, and possessive singular for the common ones.
- `generate_audio` with each lemma id. Male voice from Vault, as for all clips.
- The forms are standard grammar written fresh. Nothing is copied from class worksheets.

**Scope (out):** Words learner 1 has not met.

## Acceptance Criteria

- [ ] ac-1: `lookup` returns the full present tense and the infinitive for all five verbs, with `hasAudio` true on each.
- [ ] ac-2: Every verb in learner 1's model with any evidence has at least present and infinitive forms.
- [ ] ac-3: Every noun in learner 1's model with any evidence has a plural.
- [ ] ac-4: Brian has checked the pointing in the table above before audio was generated.

## Deliverables

- [ ] MCP writes only: `upsert_lexemes`, `upsert_verb_forms`, `upsert_noun_forms`, `generate_audio`
- [x] A short list in this spec of the lemmas filled, added when done

## Lemmas filled (2026-10-28)

Verb forms are on the live server. 185 forms, 170 new clips, 5 already present and skipped. Live `lookup` does not return the form table until the newer MCP build is deployed, so these ids are the write result.

| id | Lemma | Sets |
| --- | --- | --- |
| 748 | לְהַחְלִיט | present, past, future, imperative, infinitive |
| 99 | לִבְחוֹר | present, past, future, imperative, infinitive |
| 93 | לְהַאֲמִין | present, past, future, imperative, infinitive |
| 749 | לְנַצֵּחַ | present, past, future, imperative, infinitive |
| 259 | לְלַמֵּד | present, past, future, imperative, infinitive |
| 153 | לַעֲבוֹד | present, infinitive |
| 31 | לִלְמוֹד | present, infinitive |
| 129 | לִשְׁמוֹעַ | present, infinitive |
| 128 | לְהַסְכִּים | present, infinitive |
| 112 | לָגוּר | present, infinitive |
| 677 | לָקוּם | present, infinitive |
| 30 | לֶאֱכוֹל | present, infinitive |
| 505 | לְבַקֵּשׁ | present, infinitive |

Nouns and adjectives not written, because `upsert_noun_forms` is not on the live server: עִבְרִית, מִשְׁפָּט, שֶׁלֶג, סֵפֶר, מָרָק, טוֹב, רַע, לָבָן, נוֹחַ, מְיוּחָד, יָשָׁר, הַכּוֹל.

## Verification Strategy

- **Claim:** the words already taught have full, voiced forms.
- **Check + executor:** manual — `lookup` per lemma, and `/words/{id}` in production for לִבְחוֹר and one noun.
- **Pass condition:** ac-1 to ac-4 hold.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-009`
