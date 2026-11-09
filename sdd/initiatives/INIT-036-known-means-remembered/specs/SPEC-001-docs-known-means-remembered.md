# SPEC-001 · docs: known means remembered on a later day

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-036
title: "ADR: known means remembered on a later day"
domain: docs
status: complete
primary_prompt: none
complexity: low
complexity_reason: one record, no code, decisions already made by the operator
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003, SPEC-004]
```

## Description

Write `docs/decisions/ADR-0031-known-means-remembered.md`. ADR-0030 is the highest file today and ADR-0029 is expected to go to INIT-032, so confirm 0031 is free before writing it.

The record says:

- A skill is known only when its checked right answers fall on at least two different study days in the learner's zone. Lesson and review answers both count.
- Checked means not `peeked` and not from the say self-check. Typed spelling's implied credit still counts (ADR-0030). Other implied credits do not.
- The same rule feeds the headline "Words you know", the comfortable-text tier, the validator's coverage check, `get_context.knownLexemes` and the suggested course start. ADR-0027 D1 kept the read-Good set for the last two; this record replaces that set with the strict one.
- A self-reported say check writes no attempt, review or card. It supersedes INIT-020 SPEC-002 line 31 (it fed the scheduler). It returns when speech can be checked.
- Words that pass the old rule but have only one study day show as "learned today, check tomorrow", a count beside "Words you know".
- Rule version 4 rebuilds history. `review_log` is never edited.

It cites RSCH-003 lines 18 and 173, and ADR-0024 D3. It names ASMT-003 falsifier 1 and says plainly that how often the old rule inflated "known" in real data was never measured.

It extends ADR-0024, ADR-0027 and ADR-0030. It changes ADR-0027 D1's choice of set for `knownLexemes`.

## Acceptance criteria

- [x] `docs/decisions/ADR-0031-known-means-remembered.md` exists and states the five rules above.
- [x] It records the operator's answers: self-report counts for nothing, the later-day rule follows the research, and the headline follows the research.
- [x] It names the consumers that change: headline, tier, coverage check, `knownLexemes`, suggested start.
- [x] It links ADR-0024, ADR-0027, ADR-0030, INIT-020 SPEC-002 and ASMT-003.
- [x] It states the one thing not measured: how often the old rule called a word known on day one.
