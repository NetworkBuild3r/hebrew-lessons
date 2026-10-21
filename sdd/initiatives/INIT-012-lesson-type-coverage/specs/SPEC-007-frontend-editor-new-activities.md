# SPEC-007 · frontend: `/editor/` adds and edits every new activity

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-012
title: A teacher builds an exercise, an assignment, and a future table without JSON
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-006, INIT-011 SPEC-006]
blocks: [SPEC-009]
```

## Description

**Context:** D4 of INIT-011: a teacher never types a path, an id, an index, a recipe slug, or JSON. The lesson screen opens an activity "with the fields to fill". After SPEC-002 the schema accepts the new blocks, but the editor does not yet know how to show an item list whose fields change with `kind`. Without this spec, only the AI can build the new activities.

**Scope (in):**

- The activity menu shows `Exercise` and `Assignment` (from SPEC-002's map). Adding one opens it with a first empty item the teacher fills.
- `Exercise` first asks "What kind?" with five plain choices (Choose, True or false, Put in order, Translate, Type what you hear). The choice sets `kind` and is a field the teacher can change before items exist.
- One item-list widget in `shared/admin/plan-widgets.js`. It shows one row per item, adds and removes items, and moves them up and down. The row's fields come from the kind: for Choose, the prompt, the choices as a list with a "this one is right" marker, and the reason; for True or false, the statement, a True/False toggle, and the reason; for Put in order, the words in the right order (with a line that says the learner sees them shuffled) and optional other correct orders; for Translate, the English, the Hebrew answer, and other accepted spellings; for Type what you hear, the audio file (the existing clip picker, with "make the sound" for a pointed word) and the Hebrew answer.
- The `Assignment` form: the prompt, the minimum sentences (a number field), words to use (picked from the plan's word bank, not typed as keys), and homework lines.
- Future tense: the `Pattern` form's tense list gains Future. The cells list shows the nine rows. A form that is not yet in the dictionary shows the existing "not in the dictionary" message from N13, and one that is unverified shows the N14 note.
- Saving follows the existing rule: the activity is staged and saved when valid. A cleared optional field is removed, not saved as empty.
- The learner preview on the right shows the new activities through the same module the learner uses (SPEC-004 to SPEC-006). It is not a second renderer.
- Every label is one language. The Hebrew labels and English labels are separate strings (INIT-011 D3).

**Scope (out):** Drag and drop. Importing a worksheet. Editing `match` or `type-it` forms (INIT-010 and INIT-011). A `transform` kind.

## Understood

- [ ] und-1: The editor writes through the existing plan operations. No new route, no new write path.
- [ ] und-2: A teacher with no AI can build a whole review lesson.

## Acceptance criteria

- [x] ac-1: A teacher adds a Choose exercise with three choices, marks one right, and the saved block has `kind: choose` and an `answer` that is one of the choices — covered: yes (`test/plan-editor-exercise.test.js:162-183`)
- [x] ac-2: Each of the five kinds can be added, filled, saved, and reopened with the same values — covered: yes (`test/plan-editor-exercise.test.js:186-320`)
- [x] ac-3: Removing the choice that is marked right is refused with a plain sentence, or clears the mark and says so — covered: yes (`test/plan-editor-exercise.test.js:339-349`)
- [x] ac-4: An Assignment can be added with a minimum, three words picked from the plan's word bank, and two homework lines — covered: yes (`test/plan-editor-assignment.test.js:115-149`)
- [x] ac-5: A Pattern with Future shows nine rows, and a form missing from the dictionary shows a sentence the teacher can read — covered: yes (`test/plan-editor-exercise.test.js:417-445`)
- [x] ac-6: No form field shows an id, a path, an index, or JSON — covered: yes (`test/plan-editor-exercise.test.js:148-158,387-394`)
- [x] ac-7: The preview shows the activity as the learner sees it, from the same module — covered: yes (`test/plan-editor-exercise.test.js:207-216`, `test/plan-editor-assignment.test.js:172-189`, `test/chrome-plan-editor-exercise.js:74-78`)
- [x] ac-8: With English selected, every label is English only. With Hebrew selected, every label is Hebrew only — covered: yes (`test/plan-editor-exercise.test.js:397-414`, `test/plan-editor-assignment.test.js:192-209`, `test/plan-editor-one-language.test.js:418-484`)
- [x] ac-9: Below 900px the item list stacks and nothing scrolls sideways — covered: yes (`test/plan-editor-assignment.test.js:212-217`, `test/chrome-plan-editor-exercise.js:81-87`)
- [x] ac-10: An item holding markup is saved as text and shown as text in the form and the preview — covered: yes (`test/plan-editor-exercise.test.js:367-384`)
- [x] ac-11: The existing editor, drift, and one-language tests have no new failure — covered: yes (96/96 pass)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | changing `kind` after items exist | the editor warns that the items will be cleared, and clears them only on a second confirm | resolved | yes |
| aud-2 | the right-choice marker | a radio set; exactly one is marked | resolved | yes |

## Deliverables

- [ ] `shared/admin/plan-widgets.js` (the item-list widget and kind-specific item forms; new names added to `DEFAULT_NAMES`)
- [ ] `shared/admin/plan-hints.js` (field labels and help, Hebrew and English as separate strings)
- [ ] `shared/admin/plan-builder.js` (the lesson screen: menu rows and the new forms' mount only)
- [ ] `shared/worksheet.css` (the item-list rows)
- [ ] Tests: `test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, additions to `test/plan-editor-drift.test.js` and `test/plan-editor-one-language.test.js`; a Chrome script `test/chrome-plan-editor-exercise.js`

## Technical Requirements

- [ ] Widgets follow the registry contract (`{name, isDefault, test, render}`), and `info.path` is the owning part's id path.
- [ ] A widget never concatenates authored text into `innerHTML`. It sets `textContent` or a value.
- [ ] The widget works in the fake DOM the tests use (no `insertBefore`, `childNodes`, or `nextSibling`), as the audio-clip widget does.
- [ ] A cleared optional field is saved through `putPart` with the field removed.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test` on the files above, then the Chrome script against a temp database.
- **Rollback plan:** remove the menu rows and the widget names. Existing activities are untouched.

## Security

- **Sensitivity:** standard. Trigger matched: external input into HTML, admin-only page.
- **Security acceptance criteria:**
  - [ ] The page stays admin-only (the same gate as `/editor/` today). No new route.
  - [ ] Writes send `expectedRevision`.
  - [ ] Authored text is set as text, never as markup.
- **Review depth:** standard. **Focus areas:** XSS, authorization.

## Verification Strategy

- **Claim:** a teacher can build each new activity by hand, and what saves is what the learner player accepts.
- **Check + executor:** `mechanized` — the node tests, including a round trip for each kind: build in the editor harness, save, validate with the SPEC-002 validator, render with the SPEC-004 to SPEC-006 modules. `human` — the operator builds one review lesson with a Choose, a Put in order, and a Translate in real Chrome, at a phone width and a wide width.
- **Pass condition:** every round-trip test passes. The Chrome script exits clean. The operator records the lesson's title and that it validated with no blocking issue.
- Edge cases: five choices and a sixth refused; removing the only item; two items with the same text; changing the kind with items present; a word-bank pick for a word that was later removed.

## Integration Points

- SPEC-009 builds a lesson by hand in this editor.
- Produces **Change:** a teacher can build every lesson type's activities on `/editor/`.
- **Provenance tag:** `INIT-012/SPEC-007`.
