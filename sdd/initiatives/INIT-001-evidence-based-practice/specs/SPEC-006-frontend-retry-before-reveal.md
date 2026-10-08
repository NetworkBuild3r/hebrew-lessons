# SPEC-006 · Frontend: Retry-before-reveal feedback

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-001
title: Retry-before-reveal feedback
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3.5 hours
blocked_by: [SPEC-003]
blocks: None
```

---

## Description

**Context:** `pattern` and `fill` mark answers right/wrong on "Check" and `listen-pick` colors the choice immediately; only `build-sentence` says "Try again". `RSCH-003`: corrective feedback works (Moderate), and prompting self-correction did at least as well as recasting the answer (Low).

**Scope (in):**
- `pattern`, `fill`, `listen-pick`: first wrong attempt on an item shows a bilingual "נסו שוב (Try again)" hint and keeps the answer hidden; the second wrong attempt reveals the correct form with its audio clip.
- Each item's first-attempt result is recorded to the review store (`miss` on a wrong first attempt for any vocab word in the item; no record when the item has no vocab word).
- Keep existing per-lesson answer saving and round reset behavior.

**Scope (out):** `speed-quiz` (timed; already retries missed items); `build-sentence` (already prompts); new hint content in lessons.

**Sizing (frontend band):** 0 new screens, 3 components touched, states: unanswered / wrong-once / wrong-twice (revealed) / correct — in band.

---

## Understood (agent's read of the intent)

- [x] und-1: Feedback stays immediate; only the reveal is delayed by one attempt.

## Acceptance Criteria

- [x] ac-1 (REQ-005): in `pattern`, `fill`, and `listen-pick`, a first wrong answer shows "Try again" without revealing the correct answer — covered: yes
- [x] ac-2 (REQ-005): a second wrong answer reveals the correct answer and plays or offers its audio — covered: yes
- [x] ac-3 (REQ-005): a wrong first attempt on an item containing a vocab word records a `miss` for that word — covered: yes
- [x] ac-4: saved answers restore correctly after reload, including the revealed state — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Attempts before reveal | one retry (reveal on second wrong) | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `shared/activities/pattern.js`, `shared/activities/fill.js`, `shared/activities/listen-pick.js`
- [x] Tests: Chrome smoke steps for each activity's wrong-once / wrong-twice path

---

## Technical Requirements (domain-specific)

- [x] Attempt counts live in `activityState` so they survive reload and clear on a new round.

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

- **Claim:** checked activities prompt one retry before revealing, and first-attempt misses reach the review schedule.
- **Check + executor:** `mechanized` — headless Chrome steps per activity on a fixture lesson.
- **Pass condition:** after one wrong answer the correct option is not marked and "Try again" is visible; after a second wrong answer the correct option is marked; the store shows a `miss` for the item's word; `npm test` exits 0.

---

## Integration Points

- Writes to `ctx.review` (SPEC-003).
- Produces **Change:** retry-before-reveal in three activities.
- **Provenance tag:** `INIT-001/SPEC-006`
