# SPEC-015 · Frontend: Today daily mix

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-006
title: "Today shows and runs the daily mix: review, weak words, one drill, one reading"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-010, SPEC-012]
blocks: [SPEC-020]
```

---

## Description

**Context:** Today is `shared/today-page.js`. The mix comes from `GET /api/me/mix` (SPEC-010, composed by SPEC-008). Flashcards are SPEC-012. The drill component is SPEC-014's when present.

**Scope (in):**

- A "Today's practice" card listing the mix sections with counts and a Start button for each, in order: Review → Weak words → Drill (concept label, e.g. "Past tense · אַתָּה / אַתְּ") → Reading.
- Review and Weak open the flashcard view with the matching deck (in place, not a new route).
- Drill uses SPEC-014's drill component when available; until then the section is hidden.
- Reading shows sentences with glosses on tap (first-language glosses, RSCH-003 §1.2 Moderate) and a play button where clips exist.
- After each section, the card marks it done for the day (lesson blob boolean or session memory; no new table).
- Existing Today content (continue lesson, shelf highlights) stays below.

**Scope (out):** composition (SPEC-008); new routes.

---

## Understood (agent's read of the intent)

- [x] und-1: The mix never invents a word; it only arranges the learner's own.

## Acceptance Criteria

- [x] ac-1 (REQ-012): a Chrome smoke with seeded due and weak cards shows both sections with counts, opens Review in place, and returns to Today.
- [x] ac-2 (REQ-012): with no weak concept, the drill section is absent; with no qualifying sentences, reading is absent.
- [x] ac-3: Today's existing smoke hooks still pass.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Streak counter for the mix | None (RSCH-006). | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] Changes to `shared/today-page.js`, styles
- [x] Tests: `test/chrome-today-mix-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] `ctx.signal` aborts the mix fetch on unmount.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** smoke and `npm test`.
- **Rollback plan:** remove the card.

---

## Security

- **Sensitivity:** light. Own data only.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** Today presents and runs the mix sections.
- **Check + executor:** `mechanized` — `node test/chrome-today-mix-smoke.js`.
- **Pass condition:** exit 0.
- **Test levels:** browser smoke.

---

## Integration Points

- Consumes SPEC-010 `/api/me/mix`; opens SPEC-012 decks.
- Produces **Change:** Today mix.
- **Provenance tag:** `INIT-006/SPEC-015`
