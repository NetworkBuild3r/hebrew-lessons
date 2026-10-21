# SPEC-002 · frontend: Today grid placement and three small fixes

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-014
title: "Today grid placement and three small fixes"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 3 hours
blocked_by: [None]
blocks: [SPEC-005, SPEC-009]
```

## Description

**Context:** ASMT-002 measured three mechanical defects. REQ-003, REQ-010.

1. Desktop Today staircases. `.today-list` at `min-width: 1024px` is `grid-template-columns: minmax(0, 7fr) minmax(0, 5fr)` (`shared/worksheet.css` around 6979–6995) with no dense packing. Cards render Mix, then Review, then Continue (`shared/today-page.js` around 978–982). When Mix returns nothing (`today-page.js` around 546), Review takes row 1 in column 2 and Continue drops to row 2. Screenshot: `sdd/assessments/ASMT-002-hebrew-web-app/screens/probe-desktop1440-today.png`.
2. Match column headings are `#94a3b8` at 0.72rem (`.col-title`, `shared/worksheet.css` around 504–510), measured at 2.56:1. WCAG 1.4.3 needs 4.5:1.
3. Progress "Weak words" and "Very weak words" controls measured 89×18 and 126×18, under the 24px WCAG 2.5.8 floor.
4. The due line is built as the English words, then a colon outside the language spans, so it renders "No next due yet :Next review" (`shared/today-page.js` around 235–236).

**Scope (in):** the grid placement, `.col-title` contrast, the two Progress control sizes, and the colon. Light and dark themes.

**Scope (out):** Which Today cards exist, and which button is primary (SPEC-004). Nav (SPEC-005). Copy other than this colon (SPEC-003).

## Understood (agent's read of the intent)

- [ ] und-1: The Continue card should sit at the top of its column, level with Review, including when Mix is empty.
- [ ] und-2: The three small fixes ship with the grid change because they are the same kind of defect and SPEC-002 is the first editor of `worksheet.css`.

## Acceptance Criteria

- [ ] ac-1: At 1440×900, with Mix empty and both Review and Continue visible, the top of `#today-continue-card` is not below the top of `#today-review-card`. — covered: no
- [ ] ac-2: `.col-title` text against its card background is at least 4.5:1 in light and in dark. — covered: no
- [ ] ac-3: The "Weak words" and "Very weak words" controls are at least 24px tall. — covered: no
- [ ] ac-4: The due line does not render a colon on the wrong side of the English phrase. The colon is inside each language's own span. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | How to stop the staircase | `grid-auto-flow: dense`, or an explicit row, whichever keeps Continue at row 1 without reordering the DOM. DOM order stays Mix, Review, Continue so SPEC-004 can still change what is rendered. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/worksheet.css` — grid placement, `.col-title`, Progress control min size
- [ ] `shared/today-page.js` — colon only, around line 235
- [ ] Tests: extend `test/responsive-layout.test.js` or add a focused test for ac-1 through ac-4

## Technical Requirements (domain-specific)

- [ ] Do not edit the ADR-0006 token comment block that `test/design-tokens.test.js` compares byte for byte.
- [ ] Use existing color tokens if one already meets 4.5:1. Do not add a new hex if a token works.
- [ ] Provenance comment: `INIT-014/SPEC-002`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new or extended test passes, and existing responsive tests still pass.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none (presentation only).
- **Security acceptance criteria:**
  - [ ] No new HTML is built from unsanitized data. The colon change stays inside the existing `bi()` helper.
- **Review depth:** quick · **focus areas:** none beyond not introducing an injection in the due line.

## Verification Strategy

- **Claim:** Continue aligns to row 1 beside Review when Mix is empty, Match headings meet 4.5:1, the two Progress controls are at least 24px, and the colon sits inside each language.
- **Check + executor:** mechanized — `node --test` on the responsive test file. ac-1 and ac-3 use the repo's Chrome helper (`test/helpers/chrome-cdp.js`) at 1440 and 390. ac-2 may be a computed-style check in that same run, or a contrast calculation from the declared colors if the test names the foreground and background it used.
- **Pass condition:** an independent run prints pass for ac-1 through ac-4. The contrast assertion prints the ratio it measured, and that ratio is >= 4.5.

## Integration Points

- SPEC-004 later changes which Today cards render. It must not remove the grid rule from this spec.
- SPEC-005 and SPEC-007 edit `shared/worksheet.css` after this spec.
- Produces **Change:** Today no longer staircases, and three measured accessibility defects are closed.
- **Provenance tag:** `INIT-014/SPEC-002`
