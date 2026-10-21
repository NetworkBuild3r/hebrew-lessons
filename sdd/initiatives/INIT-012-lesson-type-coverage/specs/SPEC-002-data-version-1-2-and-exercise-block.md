# SPEC-002 · data: schemaVersion 1.2.0 and the Exercise block

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-012
title: The plan schema has a 1.2.0 version, and the validator accepts the exercise block
domain: data
status: complete
primary_prompt: .claude/agents/data-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-004]
```

## Description

**Context:** ADR-0003 §3 lets a minor version add things only by publishing a schema that names that version. ADR-0009 Decision 7 did that for the five 1.1.0 blocks: `schemaVersion` takes `1.0.0` or `1.1.0`, a `v110BlockType` guard refuses those blocks in a 1.0.0 document, and a part operation that adds one raises `schemaVersion` in the same revision (`usesV110` in `server/plans/ops.js`). The exercise block, the assignment block, and the future tense are the same kind of change, so they get the same treatment under `1.2.0`. (The Type it block sits in the schema today with no version guard. That gap is INIT-010's and is noted in SPEC-001, not fixed here.)

A new block must also be added to the schema, the validator, the hint registry, and the activity map together, or the drift tests fail (INIT-011 D8).

**Scope (in):**

- Version mechanism: `schemaVersion` enum gains `1.2.0`. A `v120BlockType` guard (`exercise`, `assignment`) and a `v120Only` field guard, built the way `v110BlockType` and `v110Only` are. A 1.0.0 or 1.1.0 document that holds a 1.2.0 construct fails. `usesV120` and the auto-raise in `server/plans/ops.js`; `create.js` and `revisions.js` read the new constant the way they read `V110`. A plan never goes back down.
- `exerciseBlock` (`type: "exercise"`) with `kind` of `choose`, `true-false`, `unscramble`, `translate`, or `dictation`, an optional `heading`, and `items` (1 to 12). Item shape per kind, as ADR-0014 fixes it:
  - `choose`: `prompt`, `choices` (2 to 5), `answer` (one of the choices), optional `why`.
  - `true-false`: `statement`, `answer` (boolean), optional `why`.
  - `unscramble`: `words` (3 to 10, in the correct order), optional `alt` (up to 3 other correct orders), optional `english`.
  - `translate`: `english`, `answer`, optional `accept` (unpointed spellings, up to 8).
  - `dictation`: `audioPath`, `answer`, optional `accept`.
- Validator rules N20 (a choose answer must be one of its choices), N21 (an unscramble has at least three words and every `alt` has the same words), N22 (a dictation has an `audioPath`), and the exercise-count warning from ADR-0014. The existing markup check, audio-path regex, N9 size, N10 list caps, and N11 text limits apply to the new fields.
- `shared/admin/activity-map.js`: `Exercise` → `exercise`, with a hint (Hebrew and English) and menu order.
- `shared/admin/plan-hints.js`: a hint entry for each exercise field.
- The drift tests stay green, and the test that every menu name is a schema block covers `Exercise`.

**Scope (out):** The assignment block and the future tense (SPEC-003). The player (SPEC-004, SPEC-005, SPEC-006). The editor widgets (SPEC-007). INIT-010's `match` block and the version guard for `type-it`.

## Understood

- [ ] und-1: `exercise` is an addition. No existing block changes.
- [ ] und-2: The `accept` rule is the lexicon's: no vowel points and no cantillation, up to eight, each 1 to 60 characters.
- [ ] und-3: `answer` in a choose item names a string that is also in `choices`. The schema cannot check that, so N20 does.
- [ ] und-4: Every existing 1.0.0 and 1.1.0 plan, sample, and published worksheet stays valid and keeps its version.

## Acceptance criteria

- [x] ac-1: One valid example of each of the five kinds passes the schema and the validator in a 1.2.0 document with no blocking finding — covered: yes (`test/plan-exercise-schema.test.js:57-63`)
- [x] ac-2: The same exercise block in a 1.1.0 document is refused with the message that says it needs 1.2.0 — covered: yes (`test/plan-exercise-schema.test.js:66-73`)
- [x] ac-3: `insert_block` of an exercise into a 1.1.0 plan succeeds and the plan comes back as 1.2.0 in the same revision. A 1.2.0 plan never returns to 1.1.0 — covered: yes (`test/plan-schema-version-1-2.test.js:121-187`)
- [x] ac-4: A choose item whose `answer` is not in `choices` is refused by N20 — covered: yes (`test/plan-exercise-schema.test.js:76-80`)
- [x] ac-5: An unscramble with two words, or with an `alt` that holds different words, is refused by N21 — covered: yes (`test/plan-exercise-schema.test.js:83-92,143-147`)
- [x] ac-6: A dictation with no `audioPath`, or with `../` escaping the audio directory, is refused (N22 and the audio-path regex) — covered: yes (`test/plan-exercise-schema.test.js:95-104`)
- [x] ac-7: `accept` holding a pointed string, and any new text field holding `<` or `>`, are refused — covered: yes (`test/plan-exercise-schema.test.js:107-116`)
- [x] ac-8: A block over the item cap is refused (N10). A plan over the size limit is refused (N9) — covered: yes (`test/plan-exercise-schema.test.js:119-140`)
- [x] ac-9: `Exercise` appears in the activity map and the menu test passes — covered: yes (`test/activity-menu-schema.test.js:43-53`)
- [x] ac-10: The four ADR-0003 sample plans and every existing fixture keep their version and their result — covered: yes (`test/plan-schema-version-1-2.test.js:55-61`, `test/plan-language-1-1.test.js`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | item shapes | as in ADR-0014 (SPEC-001) | resolved | yes |
| aud-2 | does `exercise` text count toward N3 coverage | no. N3 counts story and listening text only; SPEC-001 may overrule it in the ADR | resolved | yes |
| aud-3 | the exercise-count warning level | warn, never block (D7) | resolved | yes |
| aud-4 | version for these constructs | 1.2.0, by ADR-0003 §3 and the ADR-0009 Decision 7 precedent (O3, resolved) | resolved | yes |

## Deliverables

- [ ] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (`1.2.0`, the guards, `exerciseBlock`, one `oneOf` entry)
- [ ] `server/plans/validate.js` (version constants, N20 to N22, the count warning, messages)
- [ ] `server/plans/ops.js`, `server/plans/create.js`, `server/plans/revisions.js` (the `1.2.0` raise and floor)
- [ ] `shared/admin/activity-map.js`, `shared/admin/plan-hints.js`
- [ ] `.cursor/skills/hebrew-worksheet/schema.md` (version note and the exercise field table)
- [ ] Tests: `test/plan-exercise-schema.test.js`, `test/plan-schema-version-1-2.test.js`, additions to `test/activity-menu-schema.test.js` and `test/plan-editor-drift.test.js`

## Technical Requirements

- [ ] Every new object has `additionalProperties: false` and `required` fields named.
- [ ] Text fields use the existing `plainText`, `text80`, `text120`, `text200` definitions. No new free-text type.
- [ ] Per-kind item shape is enforced with `if`/`then`, as `conjugationTableBlock` does for `tense`.
- [ ] Messages are plain words, with no regex or schema path in front of a teacher.
- [ ] No new dependency.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. No stored plan changes version until someone adds a 1.2.0 construct to it.
- **Blocking checks:** `node --test` on the files above and on `test/plan-validate.test.js`, `test/plan-editor-drift.test.js`, `test/plan-revisions.test.js`, `test/plan-ops.test.js`.
- **Rollback plan:** revert the schema and server diff. Nothing stored holds a 1.2.0 construct yet. After a real plan has been raised, the rollback is a restore from its previous revision, which the revision history keeps.

## Security

- **Sensitivity:** standard. Trigger matched: external input (authored text that reaches learner HTML in later specs).
- **Security acceptance criteria:**
  - [ ] The new fields go through the same markup rejection as every other text field.
  - [ ] `audioPath` uses the existing regex and has no path to `..` outside the audio root.
  - [ ] A block cannot exceed the N9 size.
  - [ ] The version raise is the only field an operation changes that it was not given.
- **Review depth:** standard. **Focus areas:** input validation.

## Verification Strategy

- **Claim:** the schema and validator accept exactly the exercise shapes ADR-0014 defines, refuse the ways each can be wrong, and move a plan to 1.2.0 only when it uses a 1.2.0 construct.
- **Check + executor:** `mechanized` — `node --test` on the files in the blocking-checks list. One good fixture per kind; one bad fixture for each of ac-2 and ac-4 to ac-8.
- **Pass condition:** every good fixture has no blocking finding. Every bad fixture produces the named rule or message. The existing suites have no new failure compared with the run before this spec. The pre-existing failures are listed in the completion summary by name.
- Edge cases: an `alt` that equals the main order; a choose answer that differs from a choice by vowel points only; an `accept` with a plene and a defective spelling; a 12-item block and a 13th; a plan that is restored to a revision from before the raise.

## Integration Points

- SPEC-003, SPEC-004, SPEC-005, and SPEC-007 read these definitions.
- Produces **Change:** the plan has a 1.2.0 version, and the validator guards the exercise block.
- **Provenance tag:** `INIT-012/SPEC-002`.
