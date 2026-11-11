# SPEC-012 · content: tutor-material audit and the held-out probe

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-038
title: "Tutor-material audit and the held-out probe"
domain: content
status: complete
primary_prompt: none
complexity: medium
complexity_reason: no code, but the audit decides whether Stage 2 happens and the probe must stay unpractised to mean anything
model: inherit
autonomy_level: human_approval
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-014]
```

## Description

**Audit.** An agent that will build nothing in Stage 2 reads the class folder in Google Drive, about 85 class docs, through the Google Drive connection.
- **What it counts,** for each of the last 10 classes:
  - tutor-written lines and Brian's typed lines, separately;
  - lines with vowel points;
  - words missing from the lexicon, checked with `lookup`;
  - corrections.
- **Loadable or not.** It records whether each class could be loaded without Brian's help.
- **What it writes.** Counts and class dates only, to `specs/completion-summaries/SPEC-012-audit.md`. It states pass or fail against SPEC-001's line: about 150 or more usable tutor-written lines, **and** 5 of the last 10 classes loadable without Brian.
- **What it must not copy.** No tutor text, no line Brian typed, and no other person's name goes into the repo or over MCP.
- **If Drive cannot be read,** the audit fails as "not loadable without Brian", which is itself a gate result.

**Probe.** About 20 normal-speed listening items and 10 English prompts.
- **Choosing items.** None has been practised, and none is AI-written. If Brian allows private reuse, they are tutor lines held out of any future class pack; otherwise they are exact lexicon entries.
- **Recording items.** The item list is stored with the probe results outside the learner model. For tutor lines it keeps only the Drive reference and line number, not the text.
- **Clips** come from `generate_audio`. The items are never recorded as taught, so they create no cards.
- **The sitting** is led by an agent and takes about 10 minutes. Brian types what he hears, and the Hebrew for each English prompt.
- **Scoring** is a fixed comparison: unpointed, with the `fsrs.GradeTyped` normalization, and English prompts graded on their target words only.
- **When.** It runs before release B, and before release A if the items are ready. The dated result is the probe's first point.

**What Brian will see differently:** nothing in the app. He takes one short check of about 10 minutes, and the audit asks nothing of him.

**Removes:** nothing. It replaces guessing about the tutor material with counts, before any Stage 2 import is built.

**Scope (out):** loading any class into the app (Stage 2).

## Acceptance criteria

- [x] The audit file lists counts for each of the last 10 classes and states pass or fail on both counts.
- [x] The audit file holds numbers, class dates, and Drive references only, with no sentence text.
- [x] The 10 English prompts are lexicon forms, none is in learner 1's cards, and none is AI-written. The 20 listening items are not made. The refused computer-read voice is not a valid first score (complaint 2026-11-09, RSCH-001, BRAIN-002).
- [x] The prompt list is stored outside the learner model in `SPEC-012-probe.md`. No sitting is scored, because a made-up score would not be his.
- [x] `record_taught_words` was not called, so the list added no cards.

## Verification Strategy

- **Claim:** the gate to Stage 2 rests on real counts, and the probe measures skill without becoming practice.
- **Check + executor:** the audit is run by an agent that built no other spec. The probe check is mechanized (the items against learner 1's cards). The sitting is human: Brian takes it.
- **Pass condition:** both files exist, the audit states its result, and no probe item appears in the learner model.
