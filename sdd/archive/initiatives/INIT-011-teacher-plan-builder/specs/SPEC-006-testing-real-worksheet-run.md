# SPEC-006 · testing: rebuild two real worksheets, by hand and by the AI

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-011
title: A teacher builds a real two-lesson plan for one learner
domain: testing
status: ready
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-005]
blocks: None
```

## Description

**Context:** Node tests prove the operations. They do not prove a teacher can get from nothing to a published lesson. This spec is that run, with the teacher's own material from the Drive folder: `Worksheets/02 אוהב לא אוהב` and `Worksheets/03 המשפחה שלי`.

**Scope (in):**
- **By hand.** Signed in as an admin, English UI. New plan "Likes and family", for one learner. Lesson 1 from `02`: Words with the word bank and Ex. 1 as fill-in-the-blank, Story from Ex. 3 with its four questions, Writing from Ex. 5. Lesson 2 from `03`: Words with the family words, Match from Ex. 2 (if Match is present), Story from Ex. 3, Writing from Ex. 5, building on lesson 1. Publish. Done at 1100px, then the plan screen and one lesson checked at 700px.
- **By the AI.** The AI reads `03` and builds lesson 2 in a second plan over MCP, following the SPEC-001 recipe. The teacher opens it on `/editor/`, changes one Story sentence, and publishes.
- **The learner.** The learner account sees the published lessons on Lessons.

**Scope (out):** A new test framework. Learner page styling. Exercises the schema cannot hold (O1, the family tree drawing).

## Acceptance criteria

- [ ] ac-1 (REQ-005, REQ-009, REQ-011, D4): The by-hand run finishes without typing an id, a path, an index, or a recipe slug, and without a silent failure.
- [ ] ac-2 (REQ-014, D1): The AI-built lesson opens on the two screens, one sentence is changed, and History shows both the AI's writes and the teacher's.
- [ ] ac-3 (REQ-002, REQ-006, REQ-008): In English, at both widths, chrome is English only, the columns follow the frames, and Publish works with a warning showing.
- [ ] ac-4: The learner sees both lessons after publish.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | the learner account | Any non-admin account the operator already has. The run does not create a public account. | resolved | yes |
| aud-2 | automating the run | The by-hand run may be scripted in `test/chrome-plan-editor-smoke.js` if it fits, but the operator's pass is the one that counts. | resolved | yes |

## Deliverables

- [ ] A completion summary that lists each step, the width, what was on screen, and anything that needed a workaround
- [ ] The smoke script extended with the by-hand steps, if it fits

## Verification Strategy

- **Claim:** a teacher who is not a programmer can build and publish a real two-lesson plan, and can fix a lesson the AI built.
- **Check + executor:** `human` — the operator does the run. `mechanized` — the existing plan tests and the smoke script, re-run.
- **Pass condition:** the summary shows every step done with no id typed, both widths for both screens, the AI's and the teacher's writes in History, and the learner seeing both lessons. Any step that needed a workaround is a failed step and goes back to SPEC-004 or SPEC-005.

## Gates

The run uses the operator's admin account and an existing learner account. The test plans are drafts or personal plans, so other learners do not see them.
