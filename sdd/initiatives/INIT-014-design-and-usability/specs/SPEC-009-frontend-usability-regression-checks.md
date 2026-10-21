# SPEC-009 · frontend: Usability regression checks

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-014
title: "Usability regression checks"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002, SPEC-003, SPEC-004, SPEC-006, SPEC-007, SPEC-008]
blocks: [None]
```

## Description

**Context:** ASMT-002's defects shipped because `test/responsive-layout.test.js` checks geometry and not contrast, target size, or which Today row a card lands on. REQ-011. Earlier specs add their own tests. This spec gathers the ones that need a live browser into one Chrome run so a later change cannot bring the staircase, the contrast failure, or the clipped label back.

**Scope (in):** one Chrome test, using `test/helpers/signed-in-server.js` and `test/helpers/chrome-cdp.js`, that signs in as the harness learner and checks:

- At 1440px, with nothing due, `#today-continue-card` is not below `#today-review-card` when Review is present, and exactly one `.btn-primary` is visible.
- At 320px, no learner nav label is clipped.
- `.col-title` contrast is at least 4.5:1.
- The two Progress controls are at least 24px tall.
- At 390px, on lesson step 1, the step heading top is ≤ 160px.

Do not duplicate assertions that SPEC-002 through SPEC-008 already run in `node --test` without Chrome. Call those out in this spec's test only when they need a real viewport.

**Scope (out):** New product behavior. The five-learner study and the screen-reader pass are human checks below, not code.

## Understood (agent's read of the intent)

- [ ] und-1: This spec locks the measured fixes. It does not redesign anything.
- [ ] und-2: It runs last, against the code the other specs left behind.
- [ ] und-3: A human study is recorded, not faked. If it has not been run, the summary says so.

## Acceptance Criteria

- [ ] ac-1: The Chrome test covers the five measurements listed in Scope, and it passes on a temporary database with the real lesson fixtures. — covered: no
- [ ] ac-2: The test fails if `.col-title` is set back to `#94a3b8` on a light card, or if the Today grid rule from SPEC-002 is removed. A comment in the test names those two regressions. — covered: no
- [ ] ac-3: `node --test` on this file is the command the completion summary tells the next person to run. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Where the test lives | `test/usability-regression.test.js`, reusing the existing helpers. Not a new harness. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `test/usability-regression.test.js`
- [ ] A short note in the spec completion summary listing the command

## Technical Requirements (domain-specific)

- [ ] Temporary `DATA_DIR` only, via the signed-in harness. Do not write under the repo's `data/`.
- [ ] No new dependency.
- [ ] Provenance comment: `INIT-014/SPEC-009`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/usability-regression.test.js` passes.
- **Rollback plan:** revert the commit. The test file is the only product of this spec.

## Security

- **Sensitivity:** light — triggers matched: none. The test uses the harness password already in `test/helpers/signed-in-server.js`. Do not copy it into a new file or log it.
- **Security acceptance criteria:**
  - [ ] The test does not print the session cookie or the harness password.
- **Review depth:** quick · **focus areas:** secrets in test output.

## Verification Strategy

- **Claim:** the five ASMT-002 measurements stay true after the other specs land.
- **Check + executor:** mechanized — `node --test test/usability-regression.test.js`.
- **Pass condition:** the process exits 0, and the test file contains an assertion for each of the five measurements. A reviewer who did not write the test can map each assertion to one bullet in Scope.
- **Declared unmechanized:** a five-learner phone session (tasks: start the first lesson, find where you left off, review today's words) and one VoiceOver pass plus one NVDA pass over Today, a lesson, and Review. The role is the operator. The completion summary either attaches the notes or states that these passes were not run. They do not block the mechanized check.

## Integration Points

- Depends on SPEC-002, SPEC-003, SPEC-004, SPEC-006, SPEC-007, and SPEC-008 having landed. SPEC-001 and SPEC-005 are not in `blocked_by` because this test does not assert visibility rows or the tab structure; SPEC-005 is already required by SPEC-006, SPEC-007, and SPEC-008.
- Produces **Change:** a single Chrome test that guards the usability fixes.
- **Provenance tag:** `INIT-014/SPEC-009`
