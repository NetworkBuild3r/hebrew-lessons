# SPEC-009 · testing: five lesson types, built and played

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-012
title: One original sample lesson per lesson type, built by MCP and by hand, played in a browser
domain: testing
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-007, SPEC-008]
blocks: None
```

## Description

**Context:** The initiative's claim is that the plan can hold every lesson type the teacher teaches. A claim like that is true only if a lesson of each type validates, builds through both paths, and plays. This spec proves it, once, with original material.

**Scope (in):**

- Five sample lessons in one plan, one per type, each following its recipe card from SPEC-008:
  - A. Topic conversation: a neutral topic (a weekday morning), review checkpoint, introduction in three groups, dialogue, personal question, assignment.
  - B. Review worksheet: introduction with fill-in, match (if INIT-010 SPEC-004 has landed), choose, unscramble, translate, two short readings, assignment.
  - C. Bible verse: one short pointed verse from a public-domain pointed edition (the card's `source` rule), the pivot as an explainer, translate for numbers, a personal question.
  - D. Verb workshop: explainer, tables in present, past, and future for one verb, a drill. Future forms are written over MCP and left unverified.
  - E. Short story and true or false: graded story, true-false, choose, dictation.
- Each lesson is built twice: once over MCP with the plan operations (following the card), and once by hand on `/editor/` (one lesson, type B, by hand end to end).
- A matrix test: for each lesson type in the initiative §4 table, a fixture lesson that validates with no blocking finding and renders every block through the player's registry.
- A signed-in Chrome pass, as a learner, at 390px and at 1280px wide, in English and in Hebrew: play one item of each exercise kind, the assignment, and the future table.
- A line-by-line record of anything a lesson type still cannot hold, with the reason. An honest "not held" beats a quiet gap.

**Scope (out):** New formats or fixes. A bug found here goes back to the spec that owns it as a finding. Real learner data. Any real class-doc text.

## Understood

- [ ] und-1: Original lines only. Pointed forms come from the dictionary or are written by the operator.
- [ ] und-2: The browser pass uses a temp database and the `learner` fixture user. It never touches `data/hebrew.sqlite`.
- [ ] und-3: Autoplay needs a real mouse event, so audio steps use one.

## Acceptance criteria

- [x] ac-1: Each of the five sample lessons validates with no blocking finding and publishes. The plan that holds them is 1.2.0 — covered: yes (`test/plan-lesson-types-matrix.test.js:118-147`, `test/plan-lesson-types-mcp.test.js:77-146`)
- [x] ac-2: The MCP build of each lesson used only the existing operations. The test log lists the tool calls and none is a new tool — covered: yes (`test/plan-lesson-types-mcp.test.js:148-159`)
- [x] ac-3: One type-B lesson is built by hand on `/editor/` with no JSON and no id shown, and validates — covered: yes (`test/chrome-lesson-types-smoke.js:188-198,249-348`)
- [x] ac-4: The matrix test fails when a lesson type in §4 has no fixture, or a fixture block has no player module — covered: yes (`test/plan-lesson-types-matrix.test.js:149-177`)
- [x] ac-5: In real Chrome as a learner, one item of each exercise kind is answered, with a retry and a reveal — covered: yes (`test/chrome-lesson-types-smoke.js:364-396,499-503`)
- [x] ac-6: The assignment keeps typed text on the page and sends no request — covered: yes (`test/chrome-lesson-types-smoke.js:398-408`)
- [x] ac-7: The future table shows nine rows and an unverified form is not scored — covered: yes (`test/chrome-lesson-types-smoke.js:411-435`)
- [x] ac-8: A field holding `<img src=x onerror=...>` is shown as text in the editor, the preview, and the player, for each new surface — covered: yes (`test/plan-lesson-types-matrix.test.js:368-413`, `test/chrome-lesson-types-smoke.js:203-213,437-472`)
- [x] ac-9: At 390px nothing scrolls sideways on any new screen — covered: yes (`test/chrome-lesson-types-smoke.js:199-200,361-362,499-503`)
- [x] ac-10: A written "not held" list exists and names each exercise a lesson type still cannot do — covered: yes (`SPEC-009-not-held.md:1-19`)
- [x] ac-12: The forms-to-check list exists and holds every future form the samples wrote. No form in it is `verified` — covered: yes (`SPEC-009-forms-to-check.md:7-17`, `test/plan-lesson-types-matrix.test.js:353-366`)
- [x] ac-11: The full node suite has no new failure compared with the run taken before SPEC-001. Pre-existing failures are named — covered: yes (`SPEC-009-not-held.md:21-91`, `test/plan-lesson-types-matrix.test.js:415-425`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | the verse for type C | one verse chosen from the public-domain pointed edition the card names; the operator confirms the pointing in the human check | resolved | yes |
| aud-2 | Match in type B | included only if INIT-010 SPEC-004 is done; otherwise the test says "match: dependent" and passes | resolved | yes |

## Deliverables

- [ ] `test/fixtures/lesson-types/` (five lesson fixtures, original text)
- [ ] `test/plan-lesson-types-matrix.test.js`
- [ ] `test/plan-lesson-types-mcp.test.js` (builds each lesson over MCP in a temp database)
- [ ] `test/chrome-lesson-types-smoke.js` (the signed-in Chrome pass)
- [ ] `sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-not-held.md`
- [ ] `sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-forms-to-check.md` (every future form the samples wrote, pointed, beside the dictionary's past form of the same verb, for the operator and the teacher to read before anyone verifies one)

## Technical Requirements

- [ ] Uses `test/helpers/signed-in-server.js` and `test/helpers/chrome-cdp.js`.
- [ ] No model call, no speech call, and no network call in the node tests. The dictation clip is a fixture mp3.
- [ ] The Chrome script cleans up its temp directory.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test` on the new files, then the Chrome script.
- **Rollback plan:** delete the new test files and fixtures. Nothing in the app changes in this spec.

## Security

- **Sensitivity:** standard. Trigger matched: external input into HTML (checked in ac-8).
- **Security acceptance criteria:**
  - [ ] The XSS fixture runs on every new surface (editor, preview, player).
  - [ ] No learner answer reaches the server.
  - [ ] The tests never write to `data/hebrew.sqlite`.
- **Review depth:** standard.

## Verification Strategy

- **Claim:** every lesson type the teacher teaches can be built over MCP, built by hand, published, and played, with the new blocks.
- **Check + executor:** `mechanized` — the node tests and the Chrome script. `human` — the operator reads the five lessons for pointing and for anything that reads as a copy of a class doc.
- **Pass condition:** the matrix test is green. The Chrome script exits clean at both widths and in both languages. The operator confirms the pointing of the verse and of the verb forms. Marking a form verified is the operator's act alone, and no test does it. The not-held list is written.
- Edge cases: a lesson with only one block; a lesson with 12 exercise items; a plan with all five lessons in a row, in one revision.

## Integration Points

- Closes the initiative. Its not-held list feeds the follow-up content initiative (health, hobbies, phone, calendar, culture and news) and any later `transform` or generated-drill work.
- Produces **Change:** proof, in tests and in a browser, that the five lesson types are buildable.
- **Provenance tag:** `INIT-012/SPEC-009`.
