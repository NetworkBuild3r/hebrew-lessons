# SPEC-003 · frontend: Learner copy that is true

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-014
title: "Learner copy that is true"
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
blocks: [SPEC-009]
```

## Description

**Context:** ASMT-002 found learner-facing sentences that are untrue or name internal machinery. REQ-007. The bilingual pair pattern (`bi()`, Hebrew plus an English gloss) stays. This spec does not switch the UI to one language.

Strings and sites:

- `shared/player/core.js` around 639 and 686: "Answers save automatically in this browser". For a signed-in learner, `saveLocalProgress` (around 807–817) calls `HebrewSync.saveLessonProgress` and does not write `localStorage`. Every learner page is behind login (`server/web.js` around 411–417).
- `shared/player/core.js` around 676–684: "Practice again" is rendered beside Start on the path screen even when no step has been completed.
- `shared/player/core.js` around 531: "Next round …" is shown on step 1 of a first lesson.
- `shared/activities/story.js` around 377–380: "Coverage unknown" and "Estimated known words from the review store — no data yet."

**Scope (in):** those four sites, both languages of each pair.

**Scope (out):** Review's "All caught up!" (SPEC-008). The Today colon (SPEC-002). Removing the Hebrew/English pair pattern.

## Understood (agent's read of the intent)

- [ ] und-1: A learner should not be told progress stays in the browser when it is saved to the account.
- [ ] und-2: "Practice again" and "Next round" appear after a round exists, not before the first one.
- [ ] und-3: When coverage data is missing, the story step says nothing about it, rather than naming the review store.

## Acceptance Criteria

- [ ] ac-1: The path screen and the completion screen say the answers are saved to the account, in Hebrew and in English. Neither contains "in this browser" or the Hebrew equivalent of that phrase. — covered: no
- [ ] ac-2: On a lesson with no completed step, the path screen has Start and does not have "Practice again". After a round is complete, "Practice again" is still offered on the completion screen. — covered: no
- [ ] ac-3: On step 1 of a lesson with no prior round, the "Next round" line is not shown. — covered: no
- [ ] ac-4: When the story step has no coverage data, the page contains neither "Coverage unknown" nor "review store". — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Replacement for the save sentence | "Saved to your account" / a matching Hebrew sentence. Both halves of `bi()` change together. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `shared/player/core.js`
- [ ] `shared/activities/story.js`
- [ ] Tests: a node test that renders or reads the path HTML for a fresh lesson and for a completed round, and asserts ac-1 through ac-4

## Technical Requirements (domain-specific)

- [ ] Do not change `saveLocalProgress` itself. The bug is the sentence, not the save path.
- [ ] Keep `lang` and `dir` on both halves of each pair.
- [ ] Provenance comment: `INIT-014/SPEC-003`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** the new test passes, and existing player tests still pass.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: none.
- **Security acceptance criteria:**
  - [ ] The new sentences are fixed strings, not built from user input.
- **Review depth:** quick · **focus areas:** none.

## Verification Strategy

- **Claim:** the four sites no longer show the untrue or internal sentences, and "Practice again" remains after a finished round.
- **Check + executor:** mechanized — `node --test` on the new player-copy test.
- **Pass condition:** the test asserts the forbidden phrases are absent in the fresh-lesson HTML and present only where ac-2 allows "Practice again".

## Integration Points

- SPEC-007 also edits the player chrome. It must not restore these sentences. SPEC-009's checks assume this wording.
- Produces **Change:** learner copy matches where progress actually goes.
- **Provenance tag:** `INIT-014/SPEC-003`
