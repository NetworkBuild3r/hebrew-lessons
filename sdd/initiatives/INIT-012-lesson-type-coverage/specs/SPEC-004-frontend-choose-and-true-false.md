# SPEC-004 · frontend: the player runs Exercise (choose, true or false)

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-012
title: A learner can answer choose and true-or-false exercises in a published plan
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
blocked_by: [SPEC-002, INIT-010 SPEC-004]
blocks: [SPEC-005, SPEC-007]
```

## Description

**Context:** `shared/ui/plan-render.js` draws blocks through `renderBlock` and a `REGISTRY` of block modules in `shared/ui/blocks/`. A block with no module renders nothing. The review sheets and short stories use "choose the right one" and "true or false" constantly. They are the two simplest formats, so they go first and set the pattern the next spec reuses.

**Scope (in):**

- `shared/ui/blocks/exercise.js` with `render(block, options)` and `bind(doc, live)`, registered in `REGISTRY` as `exercise`.
- `choose`: the prompt, the choices as buttons, one tap answers. A wrong answer gets one more try, then the right choice is marked and the `why` shows. A right answer shows the `why`.
- `true-false`: the statement and two buttons. Same try-again rule, same `why`.
- A progress line ("2 of 6") and a plain summary at the end. No score is stored and nothing is sent to the server.
- Keyboard: every choice is a real button, reachable with Tab, answered with Enter or Space. The result is announced in a live region. Correct and wrong are told in words and an icon, never by color alone.
- RTL: Hebrew prompts and choices render right to left. An English `why` renders left to right, in its own `lang`.
- Pointed Hebrew shows as authored. The learner's pointing toggle (ADR-0001 #5) applies as it does for other blocks.
- If INIT-010 SPEC-004 has not landed, lift its vowel-point-insensitive compare and retry-before-reveal rule into `shared/ui/blocks/compare.js` and have both use it. Do not copy it.

**Scope (out):** Unscramble, translate, dictation (SPEC-005). The editor (SPEC-007). Match and Type it (INIT-010).

## Understood

- [ ] und-1: Answers are in the plan and are checked in the browser. They are not secret (D6).
- [ ] und-2: No learner answer is sent to the server.

## Acceptance criteria

- [x] ac-1: A lesson with a choose block renders each choice as a button and no other activity — covered: yes (`test/plan-render-exercise.test.js:261-290`)
- [x] ac-2: A wrong choice shows a try-again message and does not show the answer. A second wrong choice shows the answer and the `why` — covered: yes (`test/plan-render-exercise.test.js:291-314`)
- [x] ac-3: A true-false block works the same way — covered: yes (`test/plan-render-exercise.test.js:317-329`)
- [x] ac-4: A choice that contains `<img src=x onerror=alert(1)>` renders as text, and no element is created — covered: yes (`test/plan-render-exercise.test.js:332-377`)
- [x] ac-5: Every control is a button with a visible focus ring. Keyboard alone completes a block — covered: yes (`test/plan-render-exercise.test.js:375-426,589-598`)
- [x] ac-6: The result is announced in a live region and shown with an icon and words — covered: yes (`test/plan-render-exercise.test.js:429-438,600-620`)
- [x] ac-7: No `fetch` or beacon call is made while answering — covered: yes (`test/plan-render-exercise.test.js:441-469`)
- [x] ac-8: At 360px wide, nothing scrolls sideways — covered: yes (`test/plan-render-exercise.test.js:472-632`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | one retry before reveal | same rule as Type it and the worksheet player | resolved | yes |
| aud-2 | order of choices | as authored, not shuffled, so the author controls "all of the above" style items | resolved | yes |

## Deliverables

- [ ] `shared/ui/blocks/exercise.js`
- [ ] `shared/ui/blocks/compare.js` (only if INIT-010 SPEC-004 has not landed it)
- [ ] `shared/ui/plan-render.js` (registry entry and `bind` call only)
- [ ] `shared/worksheet.css` or the plan page stylesheet (the exercise rules)
- [ ] Tests: `test/plan-render-exercise.test.js`; additions to `test/plan-render.test.js` for the registry

## Technical Requirements

- [ ] Vanilla JS, no new dependency. Authored text goes in with the file's existing `escapeHtml`, never `innerHTML` of raw text.
- [ ] The module loads in Node (for tests) and in the browser, as the other blocks do.
- [ ] Only the existing CSS variables (the tokens of ADR-0006); no new hex values.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test` on the files above and on `test/plan-render.test.js`. Front-end files are live in the container, so no restart is needed.
- **Rollback plan:** remove the registry entry. A block with no module renders nothing, as today.

## Security

- **Sensitivity:** standard. Trigger matched: external input into HTML.
- **Security acceptance criteria:**
  - [ ] Every authored string is escaped. A test feeds markup into every field.
  - [ ] No answer is sent to the server.
  - [ ] No inline event handler attribute is generated.
- **Review depth:** standard. **Focus areas:** XSS.

## Verification Strategy

- **Claim:** a learner can complete choose and true-or-false exercises, and authored text can never become markup.
- **Check + executor:** `mechanized` — `node --test test/plan-render-exercise.test.js test/plan-render.test.js` against the fake DOM. `human` — the operator answers one block in real Chrome with the keyboard only and with a screen-reader pass of the live region.
- **Pass condition:** the fixtures for ac-1 to ac-8 pass. The XSS fixture contains no element created from the markup string. The operator records one keyboard-only run with the block, the result text, and the browser name.
- Edge cases: a choose with five choices; a choice that is the same text as the prompt; an item with no `why`; a lesson with only exercise blocks; a very long English `why`.

## Integration Points

- SPEC-005 adds three kinds to the same module. SPEC-007 previews it in the editor.
- Produces **Change:** the plan player runs choose and true-or-false.
- **Provenance tag:** `INIT-012/SPEC-004`.
