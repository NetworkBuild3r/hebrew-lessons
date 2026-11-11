# SPEC-003 · frontend: ordered stack and a chosen-block lesson

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-009
title: Author a lesson as chosen blocks in an ordered plan
domain: frontend
status: in_review
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: None
```

---

## Description

**Context:** Admin → Lessons opens `shared/admin/lesson-editor.js`, which always renders a vocabulary table, an activity path, and JSON editors for fill, grammar, reading, and askFriend. The operator asked not to be forced through those parts. The plan editor already edits blocks. This spec makes that the place a lesson is built, and makes the lesson order visible as a stack.

**Scope (in):** The plan editor lists the plan's lessons in order and can reorder them. Adding a block offers the plan schema's block types, including the `RSCH-004` types the schema already has, and does not pre-insert vocabulary, fill, grammar, reading, or ask-a-friend. `buildsOn` is chosen from earlier lessons in the stack. A recycle row can name the earlier lesson. Opening a lesson from Admin → Lessons to change its teaching content goes to that plan lesson, not to the six-section editor. Visibility of who can see a published lesson stays on the Lessons list.

**Scope (out):** New block types. A frequency-list check. Changing the three published worksheet files. Player fallback behavior (SPEC-002).

---

## Understood

- [ ] und-1: The instructor adds a block only when that lesson needs it.
- [ ] und-2: One ordered list is what the operator sees. The document may still store that list inside one unit.
- [ ] und-3: Hebrew lesson titles stay Hebrew. Chrome labels follow the existing interface-language switch.

## Acceptance Criteria

- [ ] ac-1: A new lesson form does not include sections titled for vocabulary, fill, grammar, reading, and ask-a-friend until the instructor adds those blocks — covered: no
- [ ] ac-2: Reordering two lessons in the editor changes their order in the saved plan — covered: no
- [ ] ac-3: The `buildsOn` control lists lessons earlier in the same plan and saves the chosen lesson id — covered: no
- [ ] ac-4: The Lessons visibility dropdown still saves visibility and still shows one interface language at a time — covered: no

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| ---- | ---- | ---- | ---- | ---- |
| aud-1 | which screen is the builder | The plan editor. The Lessons list keeps visibility. Teaching content is not edited in the six-section form. | resolved | yes |

---

## Deliverables

- [ ] Plan editor changes in `shared/admin/` (the files that render the lesson list and the block form)
- [ ] Lessons admin routes content editing to the plan lesson
- [ ] A node test for the empty new-lesson form and the reorder
- [ ] Browser check of: new lesson has no forced sections; reorder sticks; `buildsOn` lists the earlier lesson; visibility still saves

---

## Technical Requirements

- [ ] No new dependency. No build step. Interface language uses the existing `data-ui-text-en` / `data-ui-text-he` pattern on `<option>` elements.
- [ ] Do not put a secret in a doc or a test. Do not edit `worksheets/*/index.html`.

---

## Verification Strategy

- **Claim:** a new lesson starts without the six sections, lesson order round-trips, and `buildsOn` stores an earlier lesson id.
- **Check + executor:** `mechanized` — `node --test` on the plan-editor tests touched by this spec. `human` — the operator, in the browser, adds a second lesson, adds one block, points it at the first lesson, and confirms the six sections are absent.
- **Pass condition:** the node tests that assert the six section markers fail if those sections are in the initial form; the reorder test reads the saved order back; the operator's browser pass matches ac-1 through ac-4. No coverage percentage is the bar.

---

## Gates & Controls

No `db_migration`, no dependency, no secret. Execute only after the operator asks. Verify the changed screens in the browser before calling the spec done.
