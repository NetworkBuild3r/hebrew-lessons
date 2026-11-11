# SPEC-002 · service: play the blocks that were stored

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-009
title: Stop inventing the six-part activity ladder
domain: service
status: complete
primary_prompt: .claude/agents/service-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: None
```

---

## Description

**Context:** `shared/player/core.js` `fallbackActivities` turns a worksheet with no activities into words, fill, grammar, reading, and ask-a-friend. A lesson the instructor built with only a story would still grow that ladder if the activity list were empty. Create-plan already stores one placeholder block, which is the right minimum.

**Scope (in):** The player resolves steps from the stored list only. A lesson with no blocks shows an empty state the learner can leave, and does not synthesize the ladder. Plan create and validate keep accepting a lesson whose blocks are any legal subset. The three published worksheet files are not edited.

**Scope (out):** The admin screens (SPEC-003). New block types. Rewriting published lessons.

---

## Understood

- [ ] und-1: "Not forcing" applies to playback as well as to the editor. An omitted section is omitted on the learner's screen.
- [ ] und-2: A fluency block with a `newWord` stays a validation warning or block already defined by the schema, and this spec does not add a new hard publish rule beyond what ADR-0012 records.
- [ ] und-3: Folder names `my-morning`, `shai-date-indirect-speech`, and `anal-sex-vocab` stay.

## Acceptance Criteria

- [ ] ac-1: A worksheet whose `activities` is `[]` produces zero steps and does not include fill, grammar, reading, or ask-a-friend — covered: no
- [ ] ac-2: A worksheet whose `activities` is a single story step produces that step only — covered: no
- [ ] ac-3: The three published `worksheets/*/worksheet.js` files are unchanged by this spec — covered: no
- [ ] ac-4: Creating a plan still yields a document the validator accepts, with only the blocks create stored — covered: no

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| ---- | ---- | ---- | ---- | ---- |
| aud-1 | empty lesson | Zero steps and a short empty state, not an error page and not the old ladder | resolved | yes |

---

## Deliverables

- [ ] `shared/player/core.js` (`fallbackActivities` / `resolveSteps`)
- [ ] A node test that feeds an empty activity list and a one-block list
- [ ] No edit under `worksheets/my-morning/`, `worksheets/shai-date-indirect-speech/`, or `worksheets/anal-sex-vocab/`

---

## Technical Requirements

- [ ] No new dependency. No build step. Tests do not call ElevenLabs or OpenAI.
- [ ] Admin authorization is unchanged. This spec does not add a route.

---

## Verification Strategy

- **Claim:** playback steps equal the stored activities, including the empty list.
- **Check + executor:** `mechanized` — `node --test` on the new player test plus the existing player tests that cover the three published worksheets.
- **Pass condition:** the new test fails if `fallbackActivities` still injects fill, grammar, reading, or ask-a-friend, and the published-worksheet tests still pass. This repo's completion bar is that node test run, not a coverage percentage.

---

## Gates & Controls

No `db_migration`, no dependency, no secret. Execute only after the operator asks.
