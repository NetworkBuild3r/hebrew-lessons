# SPEC-001 · docs: ADR-0018, the v3 line and the north-star decisions

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-015
title: "ADR-0018: the v3 line and the north-star decisions"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-002]
```

## Description

**Context:** `docs/north-star.md` section 12 records five operator decisions (2026-10-19). The v2 line is set by ADR-R1, which is cited in the v2 README: greenfield, never merges into `main`, publishes only `ghcr.io/networkbuild3r/hebrew`. v3 continues that line on a new branch. ADR-0017 is the highest ADR in `docs/decisions/`, and `scripts/dev/next_adr_number.py` is not in the repo, so this ADR is ADR-0018.

**Scope (in):** `docs/decisions/ADR-0018-v3-adaptive-learning.md`. It records:

- D1–D5 with the evidence lines from north-star section 12.
- D6, the `v3` branch cut from `origin/v2`, with v2 frozen once v3 publishes.
- D7, the production data assumption.
- The word × skill model (listen, read, recall, spell). Unpointed reading is deferred.
- Lamplight as the one design system, with Figma page `139:2` as its source.

**Scope (out):** Code. The north-star doc itself, which is already updated.

## Understood (agent's read of the intent)

- [x] und-1: The ADR is the one place a later agent reads to learn why placement shows no right/wrong and why lessons auto-publish.
- [x] und-2: D6 and D7 are assumptions. The ADR says so and names the spec that checks each one.

## Acceptance Criteria

- [x] ac-1: ADR-0018 exists with Status "Accepted", the date 2026-10-19, and one section each for D1–D7. — covered: no
- [x] ac-2: Each of D1–D4 cites the research report and finding number that north-star section 12 cites. — covered: no
- [x] ac-3: The ADR restates the release rules: never merge into `main`, never publish `hebrew-lessons`, ARC `k8-build` only, and Hebrew PR Reviewer merges. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | ADR-R1 is not in this repo | Cite it as "ADR-R1 (v2 README)". Do not invent its text. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `docs/decisions/ADR-0018-v3-adaptive-learning.md`

## Technical Requirements (domain-specific)

- [ ] Follow the section layout of ADR-0017.
- [ ] Short plain sentences, matching `docs/north-star.md`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** none beyond the review.
- **Rollback plan:** delete the file.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick · **focus areas:** none.

## Verification Strategy

- **Claim:** ADR-0018 records D1–D7, with evidence for D1–D4 and the release rules.
- **Check + executor:** inspection — reviewer reads the ADR against north-star section 12.
- **Pass condition:** every decision row in the initiative table appears in the ADR with the same choice, and each of D1–D4 names a research finding.

## Integration Points

- SPEC-002 cites ADR-0018 in the CI header comment.
- **Provenance tag:** `INIT-015/SPEC-001`
