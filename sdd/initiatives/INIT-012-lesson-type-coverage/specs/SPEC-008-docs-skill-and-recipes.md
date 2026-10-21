# SPEC-008 · docs: the skill and the recipe cards for the five lesson types

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-012
title: The AI knows how to build each of the five lesson types with the new blocks
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-003, SPEC-005]
blocks: [SPEC-009]
```

## Description

**Context:** The AI builds plans through the `hebrew-worksheet` skill. Its recipe cards say "Do not invent a choice block" and "Future-tense and imperative cells are not in the registry". After SPEC-002 and SPEC-003 those lines are wrong and an AI that obeys them leaves the new blocks unused. The skill must also say what the teacher's five lesson types are and how each is built, so a request like "make a review sheet on the kitchen" lands on the right recipe and blocks.

**Scope (in):**

- `.cursor/skills/hebrew-worksheet/recipes.md`:
  - `topic-conversation`: add `assignment` as the writing task, and the rule that homework goes in the same block.
  - `review-worksheet`: replace "pick the right sentence has no block" with `exercise` choose. Add unscramble and translate as the second and third check. Add a "short story with true or false" variant (`graded-story`, then `exercise` true-false, then `exercise` choose). It is the same recipe value (D2).
  - `verse-reading`: add the note that the grammar pivot after the verse is the teaching, and that numbers practice is `exercise` translate.
  - `verb-workshop`: add future tense, one tense per table, same limits. Remove the line that says future is not in the registry. Say that imperative still is not.
  - A "lesson types" table at the top mapping the teacher's five types to a recipe and the blocks in teaching order (the initiative §4 table).
  - `worksheet-to-lesson`: replace the O1 lines. Choose-the-correct-sentence is now `exercise` choose. Match stays "not held until INIT-010 SPEC-004 lands" if it has not.
  - A new "How to write each exercise" section with the item shape for each kind, one good example each, and the typical mistakes (a choose whose right answer is not in the list, an unscramble of two words, a dictation with no clip).
- `.cursor/skills/hebrew-worksheet/SKILL.md`: the activity-name table gains `Exercise` and `Assignment`. The "what you leave for the teacher" list loses the lines that no longer apply. The step that says to make word sound is extended to dictation clips.
- `.cursor/skills/hebrew-worksheet/schema.md`: only if SPEC-002 did not already add the field tables.
- A short "Using the teacher's class docs" note: read them for structure and topics, write original lines, take every pointed form from the dictionary, and never copy a learner's typed answer into a plan. The class docs hold real errors.

**Scope (out):** Tool or schema code. The editor. New topic content (health, hobbies, calendar). Those come after the formats exist.

## Understood

- [ ] und-1: This spec only changes text the AI reads. A wrong line here is a wrong plan later.
- [ ] und-2: The recipe cards and the validator must not disagree. The card cites the rule ids.

## Acceptance criteria

- [x] ac-1: The lesson-types table lists the five types, each with a recipe value that is in the schema's `recipe` enum — covered: yes (`test/skill-recipes.test.js:107-114`)
- [x] ac-2: Every block name in `recipes.md` is a block the schema accepts or is marked "INIT-010, not held yet" — covered: yes (`test/skill-recipes.test.js:116-131`)
- [x] ac-3: No line in the skill still says choose, unscramble, true or false, or future tense is not held — covered: yes (`test/skill-recipes.test.js:133-148`)
- [x] ac-4: Each of the five exercise kinds has an item example that passes the SPEC-002 and SPEC-003 validator when pasted into a lesson of a 1.2.0 plan — covered: yes (`test/skill-recipes.test.js:150-157`)
- [x] ac-5: The activity-name table in SKILL.md equals the names in `activity-map.js` — covered: yes (`test/skill-recipes.test.js:159-166`)
- [x] ac-6: The class-docs note says to write original lines and to take pointing from the dictionary — covered: yes (`test/skill-recipes.test.js:168-174`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | examples' Hebrew | original short lines on a neutral topic (the kitchen, a weekday), pointed from the dictionary | resolved | yes |
| aud-2 | whether Match is in the table | yes, marked dependent on INIT-010 SPEC-004 | resolved | yes |

## Deliverables

- [ ] `.cursor/skills/hebrew-worksheet/recipes.md`
- [ ] `.cursor/skills/hebrew-worksheet/SKILL.md`
- [ ] `.cursor/skills/hebrew-worksheet/schema.md` (only if needed)
- [ ] Tests: `test/skill-recipes.test.js` (the checks in the Verification Strategy)

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the test file above.
- **Rollback plan:** revert the text diff.

## Security

- **Sensitivity:** light. No code, no secret.
- **Review depth:** quick. One point: the examples contain no real learner name, account id, or class-doc text.

## Verification Strategy

- **Claim:** the AI's instructions describe the blocks and rules the server enforces, and nothing else.
- **Check + executor:** `mechanized` — `node --test test/skill-recipes.test.js`: it parses the lesson-types table and checks every recipe value against the schema enum, every block name against the schema, the activity table against `activity-map.js`, and each example against the validator. `human` — the operator reads the review-worksheet card and the verb-workshop card.
- **Pass condition:** the test passes with no unknown recipe, block, or activity name, and every example validates. The operator can follow one card to a lesson without help.
- Edge cases: a card that cites a rule id the validator does not have; an example with `<` in it.

## Integration Points

- SPEC-009 builds the five samples by following these cards.
- Produces **Change:** the skill and the recipe cards match the schema.
- **Provenance tag:** `INIT-012/SPEC-008`.
