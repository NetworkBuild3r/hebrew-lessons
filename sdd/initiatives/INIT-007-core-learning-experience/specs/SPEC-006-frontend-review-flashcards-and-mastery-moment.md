# SPEC-006 · Frontend: Review flashcards and session completion mastery moment

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-007
title: "Review flashcards and session completion mastery moment"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-007]
```

---

## Description

**Context:** How a study session finishes is what determines whether a learner returns tomorrow. Today, completing a lesson in Hebrew worksheets ends abruptly: the screen displays a plain bullet list and a "Practice again" button. On the Review page (`/review/`), flashcards look like a plain form with "Miss" and "Knew" buttons. RSCH-006 Rule 14 and RSCH-007 (SDT Competence support) establish that learners need to feel what they achieved ("words you can now use", items completed, next review scheduled) rather than chasing meaningless points, streaks, or animated cartoon confetti.

This spec implements the review player and completion moments:
1. **Spaced Recall Flashcard Player (`shared/review-page.js`):**
   - Active retrieval card: prompts with Hebrew or English; flip/reveal reveals the answer with smooth transition.
   - Distinct, comfortable rating buttons: Knew (`ידעתי` in calm emerald) vs Missed (`פספסתי` in soft rose), with minimum 48px height.
   - Shows current queue progress (e.g. "Card 4 of 12") without anxiety.
   - Audio pronunciation button automatically available when the Hebrew card is revealed.
2. **The Mastery Completion Ceremony (`shared/player/core.js` / `shared/review-page.js`):**
   - Clean, proud celebration card: "סיבוב הושלם — Round Completed!"
   - Competence summary:
     - Number of words practiced in this session.
     - Highlight of words mastered or moved to long-term memory.
     - Clear date/time for when the next review round is due ("החזרה הבאה: מחר בבוקר").
   - Clear, unambiguous next actions:
     - Primary: "בחזרה להיום / Back to Today" (returns to home hub).
     - Secondary: "עוד סיבוב / Practice Again" (for learners who want immediate repetition).
3. **Empty / Caught-Up States:**
   - When no review words are due: a calm, satisfying empty state ("הכול מעודכן! אין מילים לחזרה כרגע") showing the exact date of the next due batch.

**Scope (in):**
- Restyling `shared/review-page.js`.
- Updating lesson-complete screen in `shared/player/core.js`.
- CSS rules in `shared/worksheet.css` for review cards and mastery summaries.
- Preserving review hooks (`#review-due-count`, `#review-new-count`, `data-signal="miss"`).

**Scope (out):**
- Review scheduling math (governed by `shared/player/review.js` / INIT-006).

---

## Understood

- [x] und-1: No gamified streaks, leaderboards, experience points, or confetti animations (RSCH-006 Rule 14 / operator constraint).
- [x] und-2: Completion must focus on competence: words learned, items completed, and next due schedule.
- [x] und-3: Review page DOM hooks for test coverage must be preserved.

---

## Acceptance Criteria

- [x] ac-1: On `/review/`, active recall cards display prompt text with hidden answer until reveal action is triggered, keeping `data-signal` hooks intact.
- [x] ac-2: Review action buttons (Knew / Missed) have min-height 48px and distinct, high-contrast visual styles.
- [x] ac-3: When all review items are finished, the view displays the completion summary showing total words reviewed and the timestamp of the next due word.
- [x] ac-4: At the end of a lesson worksheet, the final step presents the completion ceremony with total words met, round summary, and primary button returning to Today (`/`).
- [x] ac-5: No element in the review or completion view contains the words "streak", "points", "xp", or "leaderboard".

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Next action on complete | "Back to Today" is the primary action so learners return to their daily hub | resolved | yes |
| aud-2 | Review rating | Supports binary knew/missed today; forward-compatible with FSRS 4-button rating from INIT-006 | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- Updates to `shared/review-page.js`
- Updates to `shared/player/core.js` (complete screen rendering)
- Updates to `shared/worksheet.css` (review cards, mastery ceremony)
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-006-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test` passes, including `review-page.test.js` and `progress-view.test.js`.
- **Rollback plan:** Revert `review-page.js`, `core.js`, and completion CSS.

---

## Verification Strategy

- **Claim:** Review player renders active recall cards with comfortable touch targets; completion ceremony reflects competence without gamification; review hooks remain intact.
- **Check + executor:** `mechanized` — run `npm test` and `node test/chrome-review-smoke.js`; assert no gamification terms in rendered markup.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-006`
