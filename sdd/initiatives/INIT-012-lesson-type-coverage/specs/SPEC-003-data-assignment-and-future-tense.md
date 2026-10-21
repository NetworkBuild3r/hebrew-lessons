# SPEC-003 · data: the Assignment block and future tense

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-012
title: The schema holds an assignment block and a future-tense table, and the checks cover them
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
blocked_by: [SPEC-002]
blocks: [SPEC-006, SPEC-008]
```

## Description

**Context:** Two more 1.2.0 constructs, both small, both on the version mechanism SPEC-002 builds.

1. **Assignment.** The teacher ends most topic lessons with a short writing task, and some with homework. `personal-question` is one open answer and cannot say "write six sentences using these words".
2. **Future tense.** The verb workshop teaches present and past. Future is a named gap in the teacher's own list. Today `conjugation-table` takes `tense` of `present` or `past`, and `server/lexicon/validate.js` knows the word `future` in its `TENSE` set but holds no cells for it. The recipe card says "Future-tense and imperative cells are not in the registry." Future uses the same nine person codes as past (`1s 2ms 2fs 3ms 3fs 1p 2mp 2fp 3p`), so `cellCode` already has them. What changes is the tense value, the cell-to-tense check (N13), and the lexicon cell table.

Forms enter the lexicon through `upsert_lexicon_entries` (source `mcp`, `verified` ignored on that call; `set_lexicon_verified` is already forbidden to the AI). The importers read dictionary words, not verb tenses, and this spec does not change them.

**Scope (in):**

- `assignmentBlock` (`type: "assignment"`) with `prompt`, `minSentences` (1 to 12), optional `useWords` (up to 12 plain words), optional `homework` (up to 6 lines). It stores no answer. `assignment` joins `v120BlockType`. `Assignment` joins the activity map and the hints.
- `conjugationTableBlock.tense` accepts `future`, with a `futureCell` definition and its `if`/`then` branch. A table with `tense: future` in a 1.0.0 or 1.1.0 document fails, and a part operation that adds one raises the plan to 1.2.0 (the `usesV120` check reads it).
- `server/lexicon/validate.js`: `CELLS.future` with the nine codes. `imperative` stays without cells. `server/lexicon/service.js` and `list_grammar_concepts` list the future cells, and `upsert_lexicon_entries` accepts a future form with a future cell.
- N13 (a `formKey` must be a stored form of `lemmaKey` in that tense and cell) and N14 (an unverified form warns and is not scored) read future forms the same way as past.
- The recipe-card and skill text stay for SPEC-008.

**Scope (out):** Imperative. Marking any form verified (the operator's act, O4). Drawing the assignment or the table (SPEC-006). Timed drills (ADR-0009 ruled them out). Any importer change. Any write to `data/hebrew.sqlite`.

## Understood

- [ ] und-1: Future cell codes are the past codes. The tense carries the difference.
- [ ] und-2: A form written over MCP is not a checked form. It stays unverified until a person verifies it.
- [ ] und-3: A future table with no verified form still validates. It warns (N14) and the drill does not score it.
- [ ] und-4: An assignment holds no learner text and no answer.

## Acceptance criteria

- [x] ac-1: A valid assignment passes in a 1.2.0 document, and is refused in a 1.1.0 document — covered: yes (`test/plan-assignment-schema.test.js:53-65`)
- [x] ac-2: An assignment with `minSentences` of 0 or 13, seven homework lines, or markup in any field is refused — covered: yes (`test/plan-assignment-schema.test.js:68-99,110-119`)
- [x] ac-3: A `conjugation-table` with `tense: future` and nine cells passes when every form is a stored future form of the lemma — covered: yes (`test/plan-future-tense.test.js:89-93`)
- [x] ac-4: A future cell whose form is a past form of the same lemma is refused by N13 — covered: yes (`test/plan-future-tense.test.js:148-155`)
- [x] ac-5: A future table whose form is stored with `verified = 0` validates and carries an N14 warning — covered: yes (`test/plan-future-tense.test.js:158-171`)
- [x] ac-6: `tense: imperative` is still refused — covered: yes (`test/plan-future-tense.test.js:174-177`)
- [x] ac-7: `list_grammar_concepts` returns the future cells, and `upsert_lexicon_entries` stores a future form with cell `3ms` and refuses cell `ms` for future — covered: yes (`test/plan-future-tense.test.js:300-331`)
- [x] ac-8: A future form written over MCP is stored with `verified = 0` even when the call says `verified: true` — covered: yes (`test/plan-future-tense.test.js:334-348`)
- [x] ac-9: Adding a future table to a 1.1.0 plan raises it to 1.2.0 in the same revision — covered: yes (`test/plan-future-tense.test.js:229-249`)
- [x] ac-10: Present and past tables and their fixtures keep their results — covered: yes (`test/plan-future-tense.test.js:364-370`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | where future forms come from | the AI writes them with `upsert_lexicon_entries`; a person verifies | resolved | yes |
| aud-2 | which verbs get future forms first | verbs already in the lexicon with a stored past form | resolved | yes |
| aud-3 | one assignment block for the writing task and homework | yes (D4) | resolved | yes |

## Deliverables

- [ ] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (`assignmentBlock`, `tense`, `futureCell`, the version guards)
- [ ] `server/lexicon/validate.js`, `server/lexicon/service.js` (cells and concept list)
- [ ] `server/plans/validate.js`, `server/plans/ops.js` (N13/N14 tense handling, `usesV120` for the future tense)
- [ ] `shared/admin/activity-map.js`, `shared/admin/plan-hints.js` (`Assignment`)
- [ ] `.cursor/skills/hebrew-worksheet/schema.md` (the assignment field table)
- [ ] Tests: `test/plan-assignment-schema.test.js`, `test/plan-future-tense.test.js`, additions to the lexicon tests that exist and to `test/activity-menu-schema.test.js`

## Technical Requirements

- [ ] No schema field renamed. Past and present branches byte-unchanged.
- [ ] Pointing is stored exactly as given. Nothing "fixes" it.
- [ ] `verified` stays ignored on the MCP upsert, as it is today.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. Tests use a temp database.
- **Blocking checks:** `node --test` on the files above and on the validate, ops, and drift tests.
- **Rollback plan:** revert the diff. No stored plan holds either construct yet.

## Security

- **Sensitivity:** standard. Trigger matched: external input (assignment text reaches learner HTML in SPEC-006).
- **Security acceptance criteria:**
  - [ ] The assignment fields go through the same markup rejection as other text.
  - [ ] An MCP call can never set `verified`.
- **Review depth:** standard. **Focus areas:** input validation, human-only verification.

## Verification Strategy

- **Claim:** an assignment and a future table are accepted and checked like their neighbours, and no unchecked form is ever scored as correct.
- **Check + executor:** `mechanized` — `node --test` on the new files plus the lexicon tests. `human` — a person reads each future form against a pointed reference before verifying it. This spec does not do that step.
- **Pass condition:** each of ac-1 to ac-10 has a passing test. The tests find no `verified = 1` row created by an MCP upsert.
- Edge cases: a verb with no stored future form; a form that is the same string in past and future; a lemma with two stored future spellings; an assignment with no `useWords`.

## Integration Points

- SPEC-006 draws the assignment and the future table. SPEC-009 writes sample future forms through the MCP call and leaves them unverified.
- Produces **Change:** the schema holds an assignment and a future table.
- **Provenance tag:** `INIT-012/SPEC-003`.
