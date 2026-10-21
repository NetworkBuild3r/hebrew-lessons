# SPEC-015 · Frontend: Progress page — memory stages and direction accuracy

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-002
title: "Progress page: memory stages and direction accuracy"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-009, SPEC-012]
blocks: []
```

---

## Description

**Context:** the operator chose two learner measures (REQ-007), both grounded in `RSCH-003`:
- **Words by memory stage.** It shows the spacing schedule doing its job; spacing across days is rated **High**.
- **Recall accuracy by direction.** Hebrew→English and English→Hebrew build different knowledge (**Moderate**).

The page has no streaks, points, or badges, because `RSCH-003` rates gamification as unproven on its own (**Low**, a "don't").

**Scope (in):** `progress/index.html` + `shared/progress-page.js`, reading `GET /api/me/metrics`:
- **Memory stages.** For each direction, a stacked bar with counts of new / learning / long-term. The labels say what each means in plain words: "long-term = next review 2+ weeks away".
- **Recall accuracy, last 30 days.** For each direction, the percentage and the number of attempts. Below 10 attempts it shows "not enough answers yet" instead of a percentage.
- **One "why this matters" line per measure**, in Hebrew with English. For example: "Words move to long-term after you recall them on separate days."
- A reusable component (`HebrewProgressView.render(el, metrics)`), so SPEC-016's admin learner view shows the same charts.

**Sizing (frontend band):**
- Screens: 1.
- Components: 2 (stage bars, accuracy panel), wrapped in one reusable view.
- States: 5 (loading, error, no reviews yet, not enough data, ready).

This is below the floor, so it stays one spec.

**Scope (out):** practice time, calendar, coverage, and trouble words for learners (out of scope by operator choice).

---

## Understood (agent's read of the intent)

- [ ] und-1: Nothing on this page rewards or punishes a single missed day.
- [ ] und-2: The numbers come from the server's definitions (ADR-0002 item 10); the page does not recompute them.

## Acceptance Criteria

- [ ] ac-1 (REQ-007): given a metrics fixture, the stage bars show the exact new, learning, and long-term counts for each direction, with accessible text equivalents (no information only in color).
- [ ] ac-2 (REQ-007): accuracy shows `pct` and the attempt count when attempts are 10 or more, and "not enough answers yet" when `pct` is null.
- [ ] ac-3 (REQ-007): a new account with no reviews sees an empty state pointing to Today or Lessons.
- [ ] ac-4 (REQ-007): the page contains no streak, points, badge, or leaderboard element. A test asserts that the absence of those words and classes is part of the contract.
- [ ] ac-5 (REQ-014): labels show Hebrew with English.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Chart drawing | plain HTML/CSS bars, no chart library | resolved | yes — no new dependencies |

---

## Deliverables

- [ ] `progress/index.html`, `shared/progress-page.js` (exports `HebrewProgressView`), `shared/worksheet.css`
- [ ] Tests: `test/progress-view.test.js` (DOM render from fixtures for each state); extend the SPEC-012 sign-in smoke test to open `/progress/`

---

## Technical Requirements (domain-specific)

- [ ] No new dependencies; plain browser JS in the IIFE style.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; the sign-in smoke test reaches `/progress/` and renders the ready state.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** light. Triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** the page shows the two chosen measures exactly as the server reports them, handles the empty and low-data cases honestly, and contains no gamification.
- **Check + executor:** `mechanized`. `test/progress-view.test.js` renders each state from fixtures, and the smoke test opens the live page.
- **Pass condition:** each state's DOM matches the fixture numbers, the gamification-absence test passes, and the smoke test finds the ready state.

---

## Integration Points

- `HebrewProgressView` is reused by SPEC-016.
- **Provenance tag:** `INIT-002/SPEC-015`
