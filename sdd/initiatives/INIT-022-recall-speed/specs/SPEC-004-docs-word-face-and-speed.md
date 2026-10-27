# SPEC-004 · docs: Record the word face and the speed rule

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-022
title: "Record the word face and the speed rule"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: []
```

## Description

**Context:** The player shows pointed Hebrew on the face of a lesson word and grades a slow right answer as Hard. The North Star still said the Latin reading is on every taught word, and that any fast right answer is Easy.

**Scope (in):** ADR-0020. North Star sections 2, 4.2, and 6.1 point at it. ADR-0018 is not edited.

**Scope (out):** Turning on fade-by-mastery, unpointed reading, or the Isaiah reader.

## Acceptance Criteria

- [x] ac-1: ADR-0020 records the Hebrew face, the speed bands, the one-minute ceiling, and holding Easy until review.
- [x] ac-2: North Star sections 2, 4.2, and 6.1 cite that record.
- [x] ac-3: ADR-0018 is unchanged by this spec.

## Deliverables

- [ ] `docs/decisions/ADR-0020-lesson-word-face-and-speed.md`
- [ ] `docs/north-star.md`

## Verification Strategy

- **Claim:** the written rule matches the player.
- **Check + executor:** read the two documents against the code.
- **Pass condition:** ac-1 to ac-3 hold on reading.

## Integration Points

- **Provenance tag:** `INIT-022/SPEC-004`
