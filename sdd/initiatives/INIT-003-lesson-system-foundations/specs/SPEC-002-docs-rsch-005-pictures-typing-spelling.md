# SPEC-002 · Docs: RSCH-005 — pictures, typing, and spelling in beginner vocabulary learning

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-003
title: "RSCH-005: pictures, typing, and spelling in beginner vocabulary learning"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed. Executes through the sdd-research skill.
supplement_prompts: []
model: opus
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: []
blocks: [SPEC-004, SPEC-005]
```

---

## Description

**Context:** The operator wants lessons that carry uploaded and AI-generated pictures, and a way to help a learner who cannot yet spell or type Hebrew words. `RSCH-003` says nothing about pictures as a vocabulary aid beyond one line naming "visual support" as a moderator, and nothing about keyboard or spelling practice. `RSCH-001` leaves the pointed-to-unpointed question untested. Without this report, ADR-0003 would add an image block and a typing block with no evidence, and ADR-0004 would not know what risks AI-made pictures carry.

**Scope (in):** one `sdd-research` session, registered as `RSCH-005`, type `prior_art_review`. It answers:

1. **Pictures and vocabulary.** Does pairing a word with a picture help adult second-language vocabulary learning? What do dual-coding, picture-gloss, and keyword-mnemonic studies show? Does it differ for concrete versus abstract words? Can pictures hurt (distraction, wrong association, split attention)? When should a picture appear: at introduction, at recall, or both? Should the picture be shown at recall time, or withheld?
2. **Generated images.** What is known about AI-generated images as teaching material: accuracy for everyday objects and actions, cultural fit, text baked into images, and the cost of a wrong picture. Where research is thin, say so, and list the practical risks a human approval step must catch.
3. **Typing and spelling.** Does copying, typing, or spelling a new word help learn it? Is there evidence on productive spelling practice for words a learner can barely recognize? What does evidence say about copy-then-recall ladders, first-letter cues, and on-screen keyboards?
4. **New-script keyboard learning.** How do adults learn a new keyboard layout or script input? Is there evidence for touch-typing practice in a new script, for on-screen layout maps, and for how much it helps vocabulary? State if none exists.
5. **Hebrew script for English speakers.** What is known about learning to read Hebrew letters and points, letter confusion (look-alike letters), right-to-left reading, and transliteration help versus harm for a beginner?

**Scope (out):** course sequencing (SPEC-001); interface design (SPEC-003); ADR text; code.

---

## Understood (agent's read of the intent)

- [x] und-1: `RSCH-003` found recall-versus-recognition evidence in conflict (Low, leaning recall). This report adds the typing and spelling angle and does not re-open the general question.
- [x] und-2: The operator constraint "no forced vowel fading" stands. This report may inform an optional learner choice, never a forced one.
- [x] und-3: A finding that pictures do not help is as useful as one that they do. The spec does not assume the answer.

## Acceptance Criteria

- [x] ac-1 (REQ-004, REQ-009): `RSCH-005` has a report file and an index row. Each of the five questions has its own section with a short answer, grade, and sources.
- [x] ac-2 (REQ-009): a "Not answered" list names every question the evidence does not settle (for example keyboard learning for adults, if nothing exists).
- [x] ac-3 (REQ-004): the report gives usage rules for pictures that a builder can check or a human can follow (for example "when to show, when to hide, alt text always"), each marked evidence-backed, adapted, or product choice.
- [x] ac-4 (REQ-004): the report lists the concrete failure modes of AI-generated vocabulary pictures and the review step each one needs.
- [x] ac-5 (REQ-007): the report names which lesson or activity types (keyboard practice, copy-then-recall ladder, picture recognition, others) the evidence supports for a beginner, and which it does not.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Whether to keep the existing type-it "hint ladder" | The report may recommend changes to it; this spec does not decide | resolved | yes — scope (out) |
| aud-2 | Depth | A focused session like `RSCH-003` | resolved | yes — operator 2026-10-08 ("go"); plan default |

> **Readiness** = 100. All ambiguous rows resolved by operator go (2026-10-08).

---

## Deliverables

- [x] `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md`
- [x] A new row in `sdd/research/README.md`
- [x] Tests: none (documentation)

---

## Technical Requirements (domain-specific)

- [x] Follow `.claude/skills/sdd-research/SKILL.md` and use the `RSCH-003` layout and grading.
- [x] Every claim row has the claim, grade, source, and design consequence.
- [x] No learner names, transcripts, or secrets.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the research access gate; operator read before SPEC-004 and SPEC-005 start.
- **Rollback plan:** delete the report folder and the index row.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** every picture, typing, and spelling recommendation traces to a cited source at a stated grade, and the report separates evidence from product choice.
- **Check + executor:** `human`. A reviewer who did not write the report samples five rules from the usage-rule list and tries to reach each from its cited source. A `mechanized` aid: a search for the five question headings and the "Not answered" heading.
- **Pass condition:** all five headings and the "Not answered" list are present; the five sampled rules are reachable from their sources at the stated grade; every rule is labelled evidence-backed, adapted, or product choice.

---

## Integration Points

- Feeds SPEC-004 (image block, typing blocks, and the spelling ladder in the lesson-plan language), SPEC-005 (generated-image risks and the media rules), and later the INIT-004 media pipeline.
- Produces **Change:** one research report and one index row.
- **Provenance tag:** `INIT-003/SPEC-002`
