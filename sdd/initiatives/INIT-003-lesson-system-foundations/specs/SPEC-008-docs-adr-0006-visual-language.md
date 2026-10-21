# SPEC-008 · Docs: ADR-0006 — visual language and interface rules

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-003
title: "ADR-0006: visual language and interface rules"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-003, SPEC-006]
blocks: [SPEC-009, SPEC-010]
```

---

## Description

**Context:** After the operator picks a look from SPEC-006's mockups, the pick needs to become something a builder can implement without a designer in the room: named tokens, a type scale, spacing, a component list, and the interface rules, each with its evidence label from `RSCH-006`. Without this ADR, the new look would be re-argued in every INIT-004 spec.

**Scope (in):** `docs/decisions/ADR-0006-visual-language-and-interface-rules.md` (number claimed at plan time by directory scan, disclosed). It decides:

1. **The pick.** Which mockup (or blend) was chosen, quoting the operator's words, and what was taken from the others.
2. **Tokens.** The named values for colour (light and dark), type (family, scale, weights, the size and line height for pointed Hebrew), spacing, radius, elevation, and motion. Each has a stated purpose, and each colour pair has its measured contrast.
3. **Components.** The inventory the product needs, each with states: app bar, buttons, cards, word card, picture frame, audio control, progress indicator, feedback message, form fields, lesson-step frame, and the builder's panels.
4. **Interface rules.** A table adopted from `RSCH-006`, each rule with its grade and label (evidence-backed, adapted, product choice), and the surfaces it governs.
5. **Accessibility baseline.** The testable WCAG 2.2 criteria the product commits to, as numbers, and the command that checks them.
6. **Right-to-left and bidirectional rules.** Alignment, mixed Hebrew and English runs, numerals, and icons that flip.
7. **What is excluded.** No points, streaks, or badges; no decorative motion that cannot be turned off.
8. **Class and attribute contracts.** The naming scheme for components, so that tests can depend on stable hooks, and a list of existing class names and `data-*` attributes that tests rely on, with their fate (kept, renamed with the test updated, or retired).

**Scope (out):** building the design system (INIT-004); the stack (ADR-0005), except where a decision here depends on it; research.

---

## Understood (agent's read of the intent)

- [x] und-1: This ADR records a choice the operator makes, so every claim of "the evidence says" quotes `RSCH-006` and its grade.
- [x] und-2: Where the picked look conflicts with a rule from `RSCH-006`, the ADR says so and the operator's pick wins, labelled as a product choice.
- [x] und-3: The existing tests that depend on class names are identified here, so INIT-004 does not break them by surprise.

## Acceptance Criteria

- [x] ac-1 (REQ-008): ADR-0006 has a Decision section for each of the eight items above.
- [x] ac-2 (REQ-008): the tokens table lists every token with its value and purpose, and every foreground and background pair lists its contrast ratio.
- [x] ac-3 (REQ-008): the rules table matches `RSCH-006`'s table row for row, with any difference explained.
- [x] ac-4 (REQ-008): the accessibility baseline gives numbers (ratios, sizes), not adjectives.
- [x] ac-5: the class and attribute contract names every class and `data-*` hook that `app-shell.test.js`, `progress-view.test.js`, `review-page.test.js`, `learner-support.test.js`, `admin-users-view.test.js`, `admin-lesson-editor.test.js`, and the Chrome smokes depend on, and gives each a fate.
- [x] ac-6: the ADR contains no secret values, no learner names, and no transcript text.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | The operator's pick | a-calm-type (best RSCH-006 match; operator go 2026-10-08) | resolved | yes — operator go; pick a-calm-type |
| aud-2 | Ship dark mode | yes | resolved | yes — operator 2026-10-08, ship dark mode |

> **Readiness** = 100. Operator go; pick a-calm-type; ship dark mode.

---

## Deliverables

- [x] `docs/decisions/ADR-0006-visual-language-and-interface-rules.md`
- [x] `docs/decisions/ADR-0006-assets/tokens.json` (the token values as data, for the spike and INIT-004 to read)
- [x] Tests: none (documentation). SPEC-009 reads `tokens.json`.

---

## Technical Requirements (domain-specific)

- [x] Use `.claude/skills/sdd/assets/templates/architecture-decision-record-template.md` section order, matching ADR-0001 and ADR-0002.
- [x] Every proposed number is stated once, in the ADR or in `tokens.json`, and the other cites it.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the operator's written pick (aud-1) and answer on dark mode (aud-2).
- **Rollback plan:** delete ADR-0006 and its assets folder.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** ADR-0006 turns the operator's pick into implementable tokens, components, and rules, with each rule traced to `RSCH-006` or labelled a product choice, and without breaking any named test hook unannounced.
- **Check + executor:** `mechanized` for the tokens (a script recomputes each listed contrast ratio from `tokens.json` and compares it to the ADR's number) and for the hooks (a script searches `test/` for each hook the ADR lists and for any hook it omits); `human` for the pick and the rule-to-source tracing (a reviewer who did not write the ADR samples five rules).
- **Pass condition:** every recomputed ratio matches the ADR's to one decimal place; no test hook found in `test/` is missing from the contract; five sampled rules reach their sources at the stated grade.

---

## Integration Points

- Consumed by SPEC-009 (the spike applies the tokens), SPEC-010 (INIT-004's frontend specs), and the INIT-004 design-system spec.
- Produces **Change:** one ADR and one tokens file.
- **Provenance tag:** `INIT-003/SPEC-008`
