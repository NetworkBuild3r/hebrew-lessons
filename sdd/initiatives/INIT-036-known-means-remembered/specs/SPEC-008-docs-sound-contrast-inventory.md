# SPEC-008 · docs: the sound contrasts worth training, checked by a native speaker

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-036
title: "The sound contrasts worth training, checked by a native speaker"
domain: docs
status: blocked
primary_prompt: none
complexity: medium
complexity_reason: no code, but it depends on a person outside the repo and the result gates three specs
model: inherit
autonomy_level: plan_only
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-009]
```

## Description

Write `docs/sound-contrasts.md`, the list of contrasts the "hear the difference" drill may train. RSCH-001 line 19 found no published list of English speakers' Hebrew errors, so the list needs a person who speaks Hebrew.

The draft starts from the analyst's working view, which is **not evidence** and must be confirmed or struck:

- Letter pairs that are mostly homophones in modern speech, so a listening drill on them would train spelling, not hearing: ט/ת, כּ/ק, ח/כ (in many speakers), ס/שׂ, א/ע, ב/ו.
- Candidate true perception targets: ח vs ה vs כ/ק, ר, צ vs ס, stress minimal pairs (for example בִּירָה and בִּירָה with the other stress), and prefix and clitic boundaries.

For each contrast the document gives: a slug, what to listen for, why it matters to an English speaker, two or three real lexicon word pairs, and a status of `approved`, `rejected` or `unsure`. Only `approved` rows are served.

A native speaker or the learner's tutor reviews the list and listens to a sample clip for each pair in both voices before it is approved. The document records who checked and on what date.

**Scope (out):** the data table (SPEC-009) and the drill.

## Acceptance criteria

- [ ] `docs/sound-contrasts.md` exists with the fields above for every contrast.
- [ ] The homophone letter pairs are listed as `rejected` for listening, with the reason, unless the reviewer says otherwise.
- [ ] Each `approved` contrast has at least two real lexicon pairs and a recorded listening check in both voices.
- [ ] The document names the reviewer and the date. No row is `approved` without them.
- [ ] It states that the analyst's draft was unverified until the reviewer signed it.

## Verification Strategy

- **Claim:** every contrast the drill may serve was confirmed by a person who speaks Hebrew.
- **Check + executor:** human. The learner's tutor or another native speaker reads the list and listens to each sample pair in both voices.
- **Pass condition:** each `approved` row carries the reviewer's name and date, and a reader who did not write the draft can tell from the file alone which rows are servable.
