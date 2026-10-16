# SPEC-006 · frontend: the player runs Assignment and the future-tense table

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-012
title: A learner sees a writing task and homework, and a future-tense table
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-007]
```

## Description

**Context:** The teacher ends most topic lessons with a short writing task, and some with homework. `personal-question` is one open answer and cannot say "write six sentences using these words". Separately, the future-tense table is accepted by the schema after SPEC-003 but the player only knows present and past: its pronoun labels, cell order, and drill.

**Scope (in):**

- `shared/ui/blocks/assignment.js` with `render` and `bind`, registered as `assignment`.
  - Shows the prompt, "Write at least N sentences", and the words to use as chips that play the word clip on tap (the existing `playWord`).
  - A text area for the learner, with a live count of sentences. Nothing is saved on the server. Like `personal-question`, the text stays in the page.
  - Homework lines show as a checklist whose ticks the learner can set. Ticks are not stored.
  - A short "Done" state that appears once the sentence count is met. It is a nudge and not a gate.
- `shared/ui/blocks/conjugation-table.js`: the future tense, with the nine rows in the same order as past, the same pronoun labels (אני, אתה, את, הוא, היא, אנחנו, אתם, אתן, הם/הן), and the drill working the same way. A future form that is unverified is shown but not scored (N14), as an unverified past form is.
- A sentence counter that counts Hebrew sentence-ending punctuation and line breaks, and says so in a help line. It does not judge grammar.

**Scope (out):** Grading or collecting a learner's writing. Editor widgets (SPEC-007). Imperative. Any change to the review scheduler.

## Understood

- [ ] und-1: Nothing a learner writes leaves the page (the same rule as `personal-question`).
- [ ] und-2: The sentence count is a hint. A learner can write fewer and move on.

## Acceptance criteria

- [x] ac-1: An assignment block renders the prompt, the minimum, the word chips, and a text area — covered: yes (`test/plan-render-assignment.test.js:211-235`)
- [x] ac-2: The count goes up as the learner writes, and the Done state appears at the minimum and goes away if the text is cut below it — covered: yes (`test/plan-render-assignment.test.js:238-252`)
- [x] ac-3: Tapping a word chip plays that word's clip, or does nothing quietly when no clip exists — covered: yes (`test/plan-render-assignment.test.js:255-292`)
- [x] ac-4: Homework ticks toggle with the keyboard, and survive nothing: a reload clears them, and no request is sent — covered: yes (`test/plan-render-assignment.test.js:295-315,355-372,488-515`)
- [x] ac-5: A prompt, a word, and a homework line holding markup render as text — covered: yes (`test/plan-render-assignment.test.js:317-352`)
- [x] ac-6: A future-tense table renders nine rows in the past-tense order, with the same pronoun labels — covered: yes (`test/plan-render-assignment.test.js:328-350`, `test/render-conjugation.test.js:144-165`)
- [x] ac-7: In drill mode, a verified future form is scored and an unverified one is shown and not scored — covered: yes (`test/plan-render-assignment.test.js:352-375`, `test/render-conjugation.test.js:167-182`)
- [x] ac-8: Present and past tables render exactly as before — covered: yes (`test/plan-render-assignment.test.js:377-395`)
- [x] ac-9: The text area has a visible label, the right `lang` and `dir`, and keeps the learner's text on a resize between phone and wide widths — covered: yes (`test/plan-render-assignment.test.js:398-520`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | sentence count rule | `.`, `?`, `!`, and a line break end a sentence; empty ones are ignored | resolved | yes |
| aud-2 | does Done block moving on | no | resolved | yes |

## Deliverables

- [ ] `shared/ui/blocks/assignment.js`
- [ ] `shared/ui/blocks/conjugation-table.js` (future branch)
- [ ] `shared/ui/plan-render.js` (registry entry and `bind` call only)
- [ ] The assignment styles
- [ ] Tests: `test/plan-render-assignment.test.js`, additions to the existing conjugation-table render tests

## Technical Requirements

- [ ] Vanilla JS, no dependency. Escape every authored string.
- [ ] The text area never calls `fetch`, a beacon, or `localStorage`.
- [ ] No change to the present or past branches' output.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test` on the files above and on the existing render tests.
- **Rollback plan:** remove the registry entry and the future branch. Present and past are untouched.

## Security

- **Sensitivity:** standard. Trigger matched: external input into HTML. A learner's text stays local and is never rendered back as markup.
- **Security acceptance criteria:**
  - [ ] Authored text is escaped. The learner's own text is read as text and never written back as HTML.
  - [ ] No learner text is sent or stored.
- **Review depth:** standard. **Focus areas:** XSS, data leaving the page.

## Verification Strategy

- **Claim:** an assignment shows what the teacher wrote and keeps what the learner writes on the page. A future table matches past in layout and in scoring rules.
- **Check + executor:** `mechanized` — the node tests, including a test that wraps `fetch` and fails on any call. `human` — the operator writes two sentences in a real browser and reloads to see them gone.
- **Pass condition:** the ac-1 to ac-9 fixtures pass. The wrapped `fetch` is never called. The operator records the reload result.
- Edge cases: a minimum of 1; no `useWords`; no `homework`; a Hebrew sentence with no punctuation; a pasted block of text; a future table of three cells.

## Integration Points

- SPEC-007 adds the editor form. SPEC-009 builds Type A and Type D samples that use these.
- Produces **Change:** the plan player runs Assignment and the future tense.
- **Provenance tag:** `INIT-012/SPEC-006`.
