# SPEC-005 · Docs: RSCH-007 motion, sound, icons, picture style

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-005
title: "RSCH-007: motion, sound, icons and picture style in L2 practice"
domain: docs
status: complete
primary_prompt: .claude/agents/researcher/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-007]
```

---

## Description

**Context:** The operator wants satisfying interactions and icons. RSCH-006 found no direct evidence that animation or sound improve second-language vocabulary learning. This research answers four questions with graded evidence before the feedback kit is built.

**Scope (in):** RSCH-007 report and index row. Questions: (1) animated or sound feedback in L2 vocabulary practice; (2) icon+label vs icon alone for adult novices; (3) vocabulary picture style (flat / realistic / cartoon) and recall; (4) a "finished" moment without points and return to study. Allowed / product-choice / not-allowed effect lists.

**Scope (out):** implementing effects (SPEC-009); logo design (SPEC-006).

---

## Understood

- [x] und-1: Honest grades. "We found nothing" is a valid answer. (Q3 and Q4 are mostly "found nothing"; overall confidence Low.)
- [x] und-2: Does not reopen points/streaks. Operator excluded them. (One contest-mechanics RCT is recorded as a counter-signal in RSCH-008 §1.4; the exclusion is not reopened.)

## Acceptance Criteria

- [x] ac-1: report answers all four questions with evidence labels (High / Moderate / Low / product choice / no evidence). (RSCH-008 §1.1–1.4.)
- [x] ac-2: an effects table lists what SPEC-009 may build, what is product choice (off by default or toggle), and what is banned. (RSCH-008 effects table, with a reconciliation against ADR-0010 and the shipped kit.)
- [x] ac-3: `sdd/research/README.md` gains an index row. (Row RSCH-008.)

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Research number | RSCH-007 by scanning the research index | resolved | yes — next free. **Amended at execution:** RSCH-007 was already taken (new-user ease report, cited as such by ADR-0010 and INIT-007). The next free number is RSCH-008. SPEC-007, SPEC-008, SPEC-009 and the initiative text still say "RSCH-007" for this research and need the same change. |

> **Readiness** = 100.

---

## Deliverables

- [x] `sdd/research/RSCH-008-motion-sound-icons/RSCH-008-motion-sound-icons-report.md` (number changed from RSCH-007, see aud-1)
- [x] Index row
- [x] Completion summary (`specs/completion-summaries/SPEC-005-completion-summary.md`)

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** report and index row present; four questions answered.
- **Rollback plan:** delete the report folder and index row.

---

## Security

- **Sensitivity:** light. No learner data.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** four graded answers and an effects table exist.
- **Check + executor:** `manual` — read the report and index.
- **Pass condition:** ac-1 through ac-3 hold.
- **Provenance tag:** `INIT-005/SPEC-005`
