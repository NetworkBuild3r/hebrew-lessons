# SPEC-001 · docs: ADR for lessons that build on each other

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-030
title: "ADR: lessons that build on each other"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003]
```

## Description

ADR-0026 records the initiative's decisions:

- The code plans the progression and the AI writes it.
- One new pattern and two reused patterns per unit.
- The pace follows accuracy and the learning backlog, with consolidation units.
- About three quarters of a unit is meaning, with the drill cap and the story floor.
- The previous units' words are recycled.
- Mistakes rotate.
- Only untouched units are revised.
- New units wait while too many untouched units exist.

It extends ADR-0023 and ADR-0025 and supersedes nothing. It cites RSCH-004 for the four strands and recycling, and RSCH-003 for spacing. Each threshold is stated as a starting value or a product choice, matching how RSCH-004 grades it.

`docs/north-star.md` section 7.1 (the lesson brief) adds the new brief fields. Section 7.2 (the lesson) adds the shape rules.

## Acceptance criteria

- [ ] `docs/decisions/ADR-0026-lessons-that-build.md` exists, extends ADR-0023 and ADR-0025, and cites RSCH-003 and RSCH-004.
- [ ] Every number in it is labelled a product choice or a starting value, or is given its research grade.
- [ ] North-star 7.1 lists the next pattern, reuse patterns, previous units, pace, and consolidation. Section 7.2 lists the story floor, the drill cap, and recycling.
