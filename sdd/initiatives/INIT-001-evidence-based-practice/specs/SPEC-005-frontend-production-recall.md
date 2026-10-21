# SPEC-005 · Frontend: Production recall in lessons

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-001
title: Production recall in lessons
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-003]
blocks: None
```

---

## Description

**Context:** in recall rounds the `words` activity shows Hebrew and hides the meaning (receptive recall only). `type-it` (typing Hebrew) is limited to `advanced`. `RSCH-003` finds each recall direction builds its own knowledge and productive recall helps speaking (Moderate), and leans toward recall over recognition formats (Low).

**Scope (in):**
- `words` activity, recall rounds: a direction switch "עברית ← אנגלית / English → Hebrew" that shows the meaning and hides the Hebrew until revealed (with audio on reveal), then "Knew it / Missed it"; records go to `en→he` in the review store.
- `type-it`: make it available at `intermediate` as well as `advanced`, with vowel-insensitive matching and a "show letters" hint; results record `en→he` pass/miss.
- Round order: round 1 keeps recognition-first; recall rounds default to alternating directions per card.

**Scope (out):** new activity types; speaking/recording assessment; changes to `basic` level activity sets beyond the `words` direction switch.

**Sizing (frontend band):** 0 new screens, 2 components touched (`words` card, `type-it`), states: recognition / receptive recall / productive recall / revealed / marked — in band.

---

## Understood (agent's read of the intent)

- [x] und-1: No speed pressure is added (no root/binyan speed drills).
- [x] und-2: Nikud display still follows the learner's toggle; nothing fades vowels automatically.

## Acceptance Criteria

- [x] ac-1 (REQ-004): in a recall round, the `words` activity can present a card English-first with the Hebrew hidden until revealed, and grading records `en→he` — covered: yes
- [x] ac-2 (REQ-004): `type-it` is offered for `intermediate` lessons; typed Hebrew without nikud is accepted; results record `en→he` — covered: yes
- [x] ac-3: round 1 behavior is unchanged (Hebrew and meaning both shown) — covered: yes
- [x] ac-4 (REQ-009): no lesson file changes are required for existing lessons to get the new behavior — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Direction default in recall rounds | alternate per card; the switch lets the learner force one direction | resolved | yes — orchestrator 2026-10-08 (mode-e run) |
| aud-2 | `type-it` at `basic` | not enabled (typing Hebrew is hard for beginners); revisit after use | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `shared/activities/words.js`, `shared/activities/type-it.js`, `shared/activities/catalog.js` (levels)
- [x] Tests: `test/validate-activities.test.js` updated for `type-it` levels; Chrome smoke step for an English-first card

---

## Technical Requirements (domain-specific)

- [x] Reuse `U.stripVowels` and NFC normalization for matching.
- [x] Bilingual labels via `ctx.bi`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; smoke check.
- **Rollback plan:** revert the three activity files.

---

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** learners can practice production (English→Hebrew) in lessons at intermediate level and above, and those results reach the review schedule.
- **Check + executor:** `mechanized` — `npm test` plus a headless Chrome step that opens `my-morning` in round 2, reveals an English-first card, marks it, and reads the `en→he` record.
- **Pass condition:** the activity-level test lists `type-it` for `intermediate`; the smoke step finds the Hebrew hidden before reveal and an `en→he` record after marking; `npm test` exits 0.

---

## Integration Points

- Writes to `ctx.review` (SPEC-003); feeds SPEC-004's productive items.
- Produces **Change:** production-direction recall in `words` and wider `type-it`.
- **Provenance tag:** `INIT-001/SPEC-005`
