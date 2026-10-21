# SPEC-004 · frontend: Today shows one next action

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-014
title: "Today shows one next action"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-002]
blocks: [SPEC-009]
```

## Description

**Context:** A brand-new learner's Today screen already has one card and one primary button (ASMT-002, `phone-today.png`). After the learner starts a lesson, Today shows the Review card first ("Nothing due", no action) and then two primary buttons, "Continue" and "Next new lesson" (`shared/today-page.js` `renderToday`, around 972–990). RSCH-006 rule 16 asks the home screen for one next action. That rule is graded Low (a product choice). ASMT-002 still ranks this as the second fix, and F1 recorded the two-button point as a product choice rather than a standards breach. REQ-004.

**Scope (in):** `renderToday` and the card renderers it calls, in `shared/today-page.js` only.

- The first card with a primary button is the lesson to continue, or the next lesson when nothing is in progress.
- The Review card is rendered only when at least one item is due.
- "Next new lesson" is a secondary button or a text link when Continue is the primary. It is the primary only when there is nothing to continue.
- The first-run state (one "Start your first lesson" card plus the short welcome) stays as it is.

**Scope (out):** Grid row placement (SPEC-002). Nav (SPEC-005). The words inside unrelated cards.

## Understood (agent's read of the intent)

- [ ] und-1: One primary button on the screen, not one per card.
- [ ] und-2: An empty Review card is noise. Hide it until it has work.
- [ ] und-3: Do not regress the first-run screen, which already scored 5 in ASMT-002.

## Acceptance Criteria

- [ ] ac-1: With a lesson in progress and nothing due, Today shows one primary button, on the Continue card, and does not show the "Nothing due" Review card. "Next new lesson" is not `btn-primary`. — covered: no
- [ ] ac-2: With items due, the Review card is shown and its action is not a second `btn-primary` beside Continue. Continue stays the primary. — covered: no
- [ ] ac-3: With no progress and no due items, Today still shows one primary, "Start your first lesson", and the welcome note. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | What "secondary" means | The existing `btn` style, not `btn-primary`. No new button variant. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/today-page.js`
- [ ] Tests: a node test that renders Today for the three states in ac-1, ac-2, and ac-3 and counts `.btn-primary`

## Technical Requirements (domain-specific)

- [ ] Do not change the grid CSS. SPEC-002 owns that.
- [ ] Keep `bi()` pairs and the existing card ids (`#today-continue-card`, `#today-review-card`, `#today-next-card`) so SPEC-002's geometry check still finds them.
- [ ] Provenance comment: `INIT-014/SPEC-004`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new Today-state test passes, and existing today-page tests still pass.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none.
- **Security acceptance criteria:**
  - [ ] Card HTML is still built through the existing escape path. No new raw interpolation of lesson titles.
- **Review depth:** quick · **focus areas:** output encoding.

## Verification Strategy

- **Claim:** each of the three learner states has exactly one primary button, and the Review card is absent when nothing is due.
- **Check + executor:** mechanized — `node --test` on the new Today render test.
- **Pass condition:** the test counts `.btn-primary` as 1 in all three states, asserts the Review card's absence in ac-1 and ac-3, and asserts its presence in ac-2.

## Integration Points

- SPEC-002 places `#today-continue-card` and `#today-review-card`. This spec must keep those ids.
- Produces **Change:** returning learners get one obvious next step.
- **Provenance tag:** `INIT-014/SPEC-004`
