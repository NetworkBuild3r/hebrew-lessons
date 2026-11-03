# SPEC-001 · docs: ADR and an independent check of RSCH-011

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-029
title: "ADR and an independent check of RSCH-011"
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
blocks: [SPEC-002, SPEC-005, SPEC-007]
```

## Description

A second reader checks the RSCH-011 findings this initiative relies on: the multiple-choice and mixed-format testing effect (Adesope et al.), drag and typing compared with clicking for word order (Heift), and the small effect of self-paced reading while listening. A finding that does not survive the check is removed from the ADR, and the matching spec is reconsidered.

ADR-0025 records the decisions in the initiative: sentence build as a checked lesson step built on known words, pattern drills as a checked pick, the later-day measure, and the layouts left out on purpose. It extends ADR-0024 and supersedes nothing. `docs/north-star.md` gets an `order` row in the activity table.

## Acceptance criteria

- [ ] RSCH-011 has a verification note that marks each finding used here as confirmed, weakened, or dropped, with the source read.
- [ ] `docs/decisions/ADR-0025-practice-that-teaches.md` exists, cites RSCH-011, RSCH-003, RSCH-004, RSCH-005, and RSCH-006, and extends ADR-0024.
- [ ] The north-star activity table lists `order` and what it practices.
- [ ] The ADR names the left-out layouts and the reason for each.
