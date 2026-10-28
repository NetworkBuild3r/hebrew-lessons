# SPEC-003 · content: Voice the new forms and check the word page

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-024
title: "Voice the new forms and check the word page"
domain: content
status: planned
primary_prompt: none
model: inherit
autonomy_level: manual_review
gate_actions: [SPEC-002 table accepted, and the forms already stored]
reversibility_class: data
security_review: none
estimated_effort: 1 hour
blocked_by: [SPEC-002]
blocks: []
```

## Description

**Context:** Clips use the male voice. On the live server that is a `generate_audio` line with `speaker` `b`. After SPEC-001, `lemmaId` generates every stored form of one lemma and skips a form that already has a clip. הַכּוֹל and יָשָׁר have no forms. Each needs one clip of the word itself. Live `lookup` on 2026-10-28 reports no clip for either.

**Scope (in):**
- `generate_audio` with `lemmaId` for each noun and adjective stored in SPEC-002.
- One clip each for הַכּוֹל and יָשָׁר, male voice.
- Open `/words/{id}` on the live site for סֵפֶר and for טוֹב. Play one cell on each.

**Scope (out):** Re-generating clips that already exist. Verb audio. A paradigm for הַכּוֹל or יָשָׁר.

## Acceptance Criteria

- [ ] ac-1: `lookup` reports `hasAudio` true on every form stored in SPEC-002.
- [ ] ac-2: `lookup` reports `audioPresent` true for הַכּוֹל and יָשָׁר.
- [ ] ac-3: The live word page for סֵפֶר shows singular, plural, definite, construct, and the possessive rows, and one cell plays.
- [ ] ac-4: The live word page for טוֹב shows four agreement cells, and one cell plays.

## Deliverables

- [ ] Clips for the stored forms, plus the two single-word clips
- [ ] A count of clips generated and clips skipped, recorded in this spec

## Verification Strategy

- `lookup` after generation.
- Signed-in browser check of `/words/{id}` for סֵפֶר and טוֹב on the live site. Tap one cell on each page.

## Integration Points

- `INIT-024/SPEC-003`
