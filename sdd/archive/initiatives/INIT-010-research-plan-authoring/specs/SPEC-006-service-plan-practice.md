# SPEC-006 · service: words reached, place in the plan, practice

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-010
title: Practice the words and verb forms a learner has reached
domain: service
status: ready
primary_prompt: .claude/agents/service-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: None
```

## Description

**Context:** `RSCH-003` rates spaced recall as High and both recall directions as Moderate. A single flashcard session is not enough: delayed recall falls off (Moderate). `RSCH-004` rule 14 says a review may insert due items and must not skip ahead in the lesson order. ADR-0001 already schedules each word in two directions. ADR-0009 already allows untimed conjugation practice with feedback and forbids timed root and binyan drills. `SRS_SCHEDULER` stays on the ladder unless the operator says otherwise.

**Scope (in):** For one learner and one plan, the reached set is the `newWords` of every lesson up to and including the last lesson they finished, in plan order. The plan screen lists that set. The next lesson is the following one. Practice asks for due cards from that set only, Hebrew to English and English to Hebrew, using the existing review rows. A verb form is eligible when its lemma is in the reached set and a finished lesson contained a Pattern block for it. Conjugation practice is untimed and shows the right form after a miss. Verb listening plays the form and asks the learner to pick or produce it. Neither drill offers a verb from a lesson they have not finished.

**Scope (out):** Flipping `SRS_SCHEDULER` to FSRS. Timed speed drills. A second scheduler. Rewriting the three published worksheets.

## Acceptance criteria

- [ ] ac-1: After Family 1 is finished, the reached list is Family 1's words and the next lesson is Family 2 — covered: no
- [ ] ac-2: Practice returns a due card for a Family 1 word and returns nothing from Family 2 — covered: no
- [ ] ac-3: A verb form whose lemma is only in an unfinished lesson is not in conjugation or listening practice — covered: no
- [ ] ac-4: A miss on conjugation shows the form and does not start a timer. A pass writes the existing review result for that form — covered: no
- [ ] ac-5: One learner cannot read another learner's place or queue — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | what "finished" means | The learner completed that lesson once. Later review does not move the place backward | resolved | yes |
| aud-2 | scheduler | The ladder in ADR-0001. FSRS is not turned on | resolved | yes |

## Deliverables

- [ ] A plan-progress read: reached words, in order, and the next lesson id
- [ ] Practice selection filtered by that reached set, both directions
- [ ] Untimed conjugation and verb-listening selection from reached lemmas only
- [ ] Node tests for ac-1 through ac-5

## Verification

- **Claim:** practice never includes a word or verb form from a lesson the learner has not finished, and the next lesson is the next one in order.
- **Check + executor:** `mechanized` — `node --test` on the new plan-progress tests.
- **Pass condition:** the Family 2 word and the unfinished verb are absent from the practice result, Family 2 is the next lesson after Family 1, and a second learner's request does not return the first learner's place.

## Gates

No change to `SRS_SCHEDULER`. No migration unless the place cannot be stored on a table that already exists. If a migration is required, stop and return a gate request. The operator has waived `db_migration` sign-off on this local project before. A backup still runs first.
