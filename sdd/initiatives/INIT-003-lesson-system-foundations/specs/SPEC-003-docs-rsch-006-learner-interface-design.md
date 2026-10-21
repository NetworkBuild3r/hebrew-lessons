# SPEC-003 · Docs: RSCH-006 — learner-interface design evidence

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-003
title: "RSCH-006: learner-interface design evidence"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed. Executes through the sdd-research skill.
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-006, SPEC-008]
```

---

## Description

**Context:** The operator finds the site "super lame" and wants the look to be "whatever is backed by science to support real learning". Taste is not evidence, and `RSCH-001` to `RSCH-003` do not cover interface design. Today the site is white cards, a blue gradient top bar, and pill badges, with one breakpoint and no dark mode. The design language (ADR-0006) needs rules that are traced to evidence, and an honest label on every rule that is only a product choice.

**Scope (in):** one `sdd-research` session, registered as `RSCH-006`, type `prior_art_review`. It answers:

1. **Cognitive load and multimedia learning.** What do the multimedia-learning principles (coherence, signaling, segmenting, redundancy, split attention, spatial contiguity) say about a lesson screen that shows a word, its meaning, audio, and a picture? How well do they hold for second-language learners, and at what grade?
2. **Reading and typography.** Evidence on type size, line height, line length, and contrast for readability. Specifically for Hebrew with points (nikud): size and spacing so that points stay legible, font choice, and mixed Hebrew and English on one screen (bidirectional text).
3. **Feedback and flow.** Timing and form of feedback in learning software (immediate versus delayed, showing the answer after a miss, retry prompts), as they bear on interface behavior.
4. **Motivation without gamification.** What self-determination research and the learning-app literature say about autonomy, competence, and visible progress. Note again that `RSCH-003` graded gamification as unproven (Low).
5. **Accessibility.** What WCAG 2.2 requires for contrast, target size, focus, motion, and zoom. Reduced-motion and dark-mode evidence, stated honestly if it is mixed.
6. **Mobile and one-handed use.** Evidence on touch target size, thumb reach, and short sessions on phones.
7. **Layout and navigation for learners.** Evidence on navigation depth, progressive disclosure, and "one thing to do now" home screens for novices.

**Scope (out):** choosing a look (SPEC-006 and SPEC-008); course design (SPEC-001); code.

---

## Understood (agent's read of the intent)

- [ ] und-1: Many interface rules have thin or no learner-specific evidence. A rule that rests on general usability practice is labelled "adapted" or "product choice", not "evidence-backed".
- [ ] und-2: The output is a rule table that ADR-0006 adopts, not a mood board.
- [ ] und-3: The ban on points and streaks stays unless this report finds evidence that changes `RSCH-003`'s grade, in which case it states the conflict plainly.

## Acceptance Criteria

- [ ] ac-1 (REQ-008, REQ-009): `RSCH-006` has a report file and an index row. Each of the seven questions has its own section with a short answer, grade, and sources.
- [ ] ac-2 (REQ-009): a "Not answered" list is present.
- [ ] ac-3 (REQ-008): the report ends with a table of interface rules. Each row has the rule, its grade, the source, and whether it is evidence-backed, adapted, or product choice. At least these topics have rows: type size and spacing for pointed Hebrew, contrast and target size, how a word, meaning, audio, and picture are laid out together, feedback after a miss, and progress display.
- [ ] ac-4 (REQ-008): the accessibility section lists the testable WCAG 2.2 criteria that the site should meet (for example contrast ratios, target sizes), so that SPEC-009 can check them mechanically.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | "Backed by science" for interface taste | Rules are graded; where evidence is absent the rule is labelled a product choice, and the operator's pick among mockups settles it | resolved | yes — operator 2026-10-08 |
| aud-2 | Dark mode | In scope as a question; whether to ship it is decided in ADR-0006 | resolved | yes — scope |

---

## Deliverables

- [ ] `sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md`
- [ ] A new row in `sdd/research/README.md`
- [ ] Tests: none (documentation)

---

## Technical Requirements (domain-specific)

- [ ] Follow `.claude/skills/sdd-research/SKILL.md` and use the `RSCH-003` layout and grading.
- [ ] Every rule row has the rule, grade, source, and label (evidence-backed, adapted, product choice).
- [ ] No learner names, transcripts, or secrets.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the research access gate; operator read before SPEC-006 starts.
- **Rollback plan:** delete the report folder and the index row.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** every interface rule in `RSCH-006` traces to a cited source at a stated grade, or is labelled a product choice, and the accessibility criteria are testable.
- **Check + executor:** `human` for source tracing: a reviewer who did not write the report samples five rows from the rule table and tries to reach each from its source. `mechanized` for the accessibility list: each criterion names a measurable threshold (a ratio or a pixel size) that SPEC-009 can test with a script.
- **Pass condition:** the seven headings, the rule table, and the "Not answered" list are present; the five sampled rows are reachable from their sources at the stated grade; every accessibility criterion has a number or a yes/no test.

---

## Integration Points

- Feeds SPEC-006 (mockups follow the rules), SPEC-008 (ADR-0006), and SPEC-009 (accessibility checks in the spike).
- Produces **Change:** one research report and one index row.
- **Provenance tag:** `INIT-003/SPEC-003`
