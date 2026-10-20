# SPEC-001 · docs: ADR-0014, exercise, assignment, and future tense

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-012
title: Record the lesson-type table, the Exercise and Assignment blocks, and future tense
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
blocked_by: None
blocks: [SPEC-002]
```

## Description

**Context:** The teacher's five lesson types need exercise formats the plan schema cannot hold. INIT-012 adds them. Before any schema edit, one record says what the formats are, what they are not, and which rule ids they use. This is the ADR the rest of the initiative cites. It is numbered ADR-0014 (the highest on disk is ADR-0013; `scripts/dev/next_adr_number.py` is not in this repo, so the number was read from `docs/decisions/`; re-read it before saving).

**Scope (in):** `docs/decisions/ADR-0014-lesson-type-coverage.md`. It records:

1. The lesson-type table from the initiative §4: type, recipe, blocks in teaching order, what is new.
2. D2 to D9 of the initiative as decisions with the reason for each.
3. The `exercise` block: `kind` and the item shape for each kind, the item caps, and the text limits that apply.
4. The `assignment` block: fields and the rule that it stores nothing.
5. Future tense: same cell codes as past, imperative out.
6. New rule ids, in the numbering ADR-0009 §7 started (the last is N19): the next three are N20 (a choose answer must be one of its choices), N21 (an unscramble needs at least three words and every `alt` has the same words), N22 (a dictation needs an `audioPath`). Plus a warning when a `topic-conversation` lesson holds more than a set number of `exercise` blocks. The number is a product choice and is written down as one.
7. O3, resolved: the new constructs (`exercise`, `assignment`, `tense: future`) take `schemaVersion` `1.2.0`. The reason is ADR-0003 §3 (a minor version adds things only by publishing a schema that names it) and the ADR-0009 Decision 7 precedent (five blocks, `v110BlockType` guard, auto-raise in the same revision). The ADR also records that `type-it` is in the schema with no version guard today, and says that INIT-010 SPEC-004 should put `match` and `type-it` under the same guard. That is a note for INIT-010, not work here.
8. Why there are two families (`exercise` and INIT-010's `match` and `type-it`) for now, and the condition under which they fold together (O1).
9. The honest limit: answers are in the published plan and are readable (D6).

**Scope (out):** Any schema, validator, or player change. Those are SPEC-002 and later.

## Understood

- [ ] und-1: The lesson types already exist as recipes. The ADR records what is missing inside them.
- [ ] und-2: D6 is a decision the operator should see in writing, not a footnote.

## Acceptance criteria

- [x] ac-1: The ADR holds the lesson-type table, and every block name in it is either in the schema today, in INIT-010, or marked new — covered: yes (`ADR-0014:49-56`)
- [x] ac-2: Each of the five `kind` values has an item shape with every field named, its limits, and one example in plain Hebrew with English gloss — covered: yes (`ADR-0014:87-153`, `exercise-examples.json:5-76`)
- [x] ac-3: N20, N21, N22 and the exercise-count warning each have a one-sentence trigger and a block-or-warn level — covered: yes (`ADR-0014:191-199`)
- [x] ac-4: The ADR states that these constructs take 1.2.0, quotes ADR-0003 §3 and ADR-0009 Decision 7, and names the `type-it` guard gap for INIT-010 — covered: yes (`ADR-0014:72,217-233`)
- [x] ac-5: The ADR states that answers are readable by the learner — covered: yes (`ADR-0014:68,250-254`)

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | ADR number | ADR-0014 (highest on disk is ADR-0013) | resolved | yes |
| aud-2 | version bump (O3) | 1.2.0, by ADR-0003 §3 and the ADR-0009 Decision 7 precedent | resolved | yes |

## Deliverables

- [ ] `docs/decisions/ADR-0014-lesson-type-coverage.md`
- [ ] `docs/decisions/ADR-0014-assets/exercise-examples.json` (one valid example per kind, used by SPEC-002's tests)

## Verification Strategy

- **Claim:** every block, kind, version, and rule id the later specs rely on is defined in one place.
- **Check + executor:** `human` — the operator reads the ADR against the initiative §4 table. `mechanized` — a one-line script checks that every `type` named in the table is a schema block, an INIT-010 block, or marked new.
- **Pass condition:** the script lists no block that is neither in the schema, nor in INIT-010, nor marked new. The operator can find each of D2 to D9 in the ADR without asking.

## Gates

No code. No dependency. No secret.
