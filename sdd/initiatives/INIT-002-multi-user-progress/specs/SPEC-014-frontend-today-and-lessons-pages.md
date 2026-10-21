# SPEC-014 · Frontend: Today and Lessons pages

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-002
title: Today and Lessons pages
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-012, SPEC-013]
blocks: []
```

---

## Description

**Context:** the home page (`index.html`) combines a "Review today" card, shown only when something is due, with the full lesson list sorted by round status. The status comes from localStorage. REQ-006 splits this into two places:
- **Today** answers "what should I do now?"
- **Lessons** is the library.

Both read the server state through `HebrewSync` and the per-user filtered catalog.

**Scope (in):**
- **Today: root `index.html` + `shared/today-page.js`.** Cards in order:
  1. **Review.** Due count and new-word count from `HebrewReview.dueList` over the server state, with a Start button to `/review/`. When nothing is due it shows a "nothing due — next review on <date>" state.
  2. **Continue.** The most recently opened lesson that isn't finished, with its round line and a Continue button.
  3. **Next new lesson.** The first catalog lesson with no progress.
  4. **Welcome.** Shown only for a brand-new account with no progress: a short note and "start your first lesson".
- **Lessons: `lessons/index.html` + `shared/lessons-page.js`.**
  - Every lesson in the filtered catalog as cards (title, subtitle, level, blurb), with a status chip: new / in progress / round N due / done for now.
  - A level filter (all / basic / intermediate / advanced).
  - The existing sort: due-again first, then catalog order.
- Shared status logic moves into one helper (`HebrewUtils.lessonStatus`), so Today, Lessons, and the player stop duplicating the 24-hour round rule.

**Sizing (frontend band):**
- Screens: 2.
- Components: 4 (today card, lesson card, status chip, level filter).
- States: 8 (loading, error, new account, nothing due, due, continue present, no lessons visible, filtered empty).

**Scope (out):** Progress page (SPEC-015); admin pages.

---

## Understood (agent's read of the intent)

- [ ] und-1: Today never lists every lesson. It shows at most the three or four cards above, so the next step is obvious.
- [ ] und-2: Lessons a user can't see never appear, because the catalog is filtered on the server (SPEC-006/008).

## Acceptance Criteria

- [ ] ac-1 (REQ-006): with 3 words due and 1 lesson in progress, Today shows the Review card (3 due) and the Continue card for that lesson. Start goes to `/review/`, and Continue opens the lesson at its saved place.
- [ ] ac-2 (REQ-006): a brand-new account sees the Welcome card and a Next lesson card, with no Review card.
- [ ] ac-3 (REQ-006): Lessons lists exactly the lessons in the filtered catalog with the correct status chips. The level filter narrows the list, and an empty filter result shows an empty state.
- [ ] ac-4 (REQ-010): a user without a grant for a restricted lesson sees it on neither page.
- [ ] ac-5 (REQ-014): every label shows Hebrew with English, nikud stays on, and no lesson or vocab data is fetched.
- [ ] ac-6: the round-status rule exists in one helper, and its unit test covers new, in-progress, due-again, and not-yet-due.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | "Done for now" | all activities in the current round completed and the next round is not yet due | resolved | yes — plan default |

---

## Deliverables

- [ ] `index.html`, `shared/today-page.js`, `lessons/index.html`, `shared/lessons-page.js`, `shared/player/utils.js` (`lessonStatus`), `shared/worksheet.css`
- [ ] Tests: `test/lesson-status.test.js`; `test/chrome-today-lessons-smoke.js` (seeded server state for the ac-1 to ac-4 scenarios)

---

## Technical Requirements (domain-specific)

- [ ] Reuse the existing lesson card styles where possible; the new styles use the existing tokens.
- [ ] Every value comes from catalog or server data and is escaped.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; `node test/chrome-today-lessons-smoke.js`.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** light. Triggers matched: none (renders filtered, escaped data).
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** Today shows the learner's next step from server state, and Lessons shows exactly the lessons they may see, with the right status.
- **Check + executor:** `mechanized`. `test/lesson-status.test.js` (unit) and `test/chrome-today-lessons-smoke.js` (headless Chrome, seeded database for each scenario).
- **Pass condition:** each ac-1 to ac-4 scenario renders the stated cards, chips, and counts, which the smoke test asserts by DOM query, and the unit test passes.

---

## Integration Points

- Closes the learner-facing layout. SPEC-018 documents it.
- **Provenance tag:** `INIT-002/SPEC-014`
