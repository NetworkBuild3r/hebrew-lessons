# SPEC-011 · ops: deploy, sweep, and check a lesson's buttons

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-037
title: "Deploy, sweep, and check a lesson's buttons"
domain: ops
status: in_progress
primary_prompt: none
complexity: low
complexity_reason: the normal deploy plus two read-only checks
model: inherit
autonomy_level: human_approval
gate_actions: [deploy]
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-003, SPEC-004, SPEC-007, SPEC-008, SPEC-010]
blocks: [SPEC-012]
```

## Description

This spec takes over INIT-032 SPEC-007 (list the bad lessons) and deploys this initiative.

- Deploy SPEC-003 to SPEC-010 together, with the normal deploy, and confirm `/version` shows the new commit.
- Run the `lesson_items` rebuild from SPEC-003 once on production.
- Run SPEC-009's sweep for learner 1, and save its output under `sdd/initiatives/INIT-037-taught-words-only/specs/completion-summaries/`.
- Open one of units 1 to 11 that has a cloze, and record its four buttons over three runs.

**Scope (out):** writing units (SPEC-012).

## Acceptance criteria

- [ ] `/version` reports the deployed commit.
- [ ] The sweep lists every published unit on learner 1's path with its block count, and none of units 12 to 34 is published.
- [ ] The recorded buttons are all words learner 1 has a card for, or words of a finished unit.

## Verification Strategy

- **Claim:** the live site runs the fixes, and the published path is known.
- **Check + executor:** mechanized for `/version` and the sweep; human for the button check, done by Brian or the agent with a screenshot.
- **Pass condition:** the commit matches, the sweep file exists with 11 published units listed, and every recorded button is in learner 1's card set or a finished unit.
