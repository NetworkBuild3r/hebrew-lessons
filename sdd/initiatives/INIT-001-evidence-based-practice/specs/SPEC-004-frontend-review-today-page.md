# SPEC-004 · Frontend: "Review today" page and home queue

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-001
title: "Review today" page and home queue
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: None
```

---

## Description

**Context:** with per-word schedules (SPEC-003) and a cross-lesson index (SPEC-002), the learner needs one place to do today's due recall across all lessons — spaced (High), active recall (Moderate), interleaved (Moderate) per `RSCH-003`.

**Scope (in):**
- Home page (`index.html`): a "חזרה היום (Review today)" card at the top showing due count and new-word count, linking to the review page; hidden when nothing is due and no words are known yet.
- New page `review/index.html` + `shared/review-page.js` (loads `shared/player/utils.js`, `speech.js`, `review.js`, the two word-audio catalogs, and `worksheets/_review/vocab-index.js` by script tag):
  - session of up to the daily cap, ordered by `dueList` (interleaved);
  - each item shows the prompt for its direction (`he→en`: pointed Hebrew + audio button; `en→he`: English meaning), a "Show answer" button, then the answer with audio and source lesson, then "ידעתי (Knew it)" / "פספסתי (Missed it)";
  - optional typed answer for `en→he` with vowel-insensitive comparison (reuse `U.stripVowels`);
  - vowel toggle that remembers the existing preference (no forced fading);
  - end screen: counts reviewed, next due time.
- Audio plays the shared ElevenLabs clip when one exists, browser TTS otherwise (existing `Speech.play`).

**Scope (out):** new activity types inside lessons; streaks or time logs (out of initiative); editing lesson files.

**Sizing (frontend band):** 2 screens (home card, review page), ~4 components (queue card, review item, answer panel, end summary), states: loading / nothing due / reviewing / answer shown / finished / index missing — in band.

---

## Understood (agent's read of the intent)

- [x] und-1: The answer is never visible before the learner asks for it (recall before reveal).
- [x] und-2: Words already met only in round 1 of a lesson still appear once due.

## Acceptance Criteria

- [x] ac-1 (REQ-003): the review page shows due items from at least two lessons interleaved when both have due items (fixture with `my-morning` and `anal-sex-vocab`) — covered: yes
- [x] ac-2 (REQ-003): the answer and meaning are hidden until "Show answer"; grading updates the record via `HebrewReview.record` and the item's next due date moves per ADR-0001 — covered: yes
- [x] ac-3 (REQ-004): items appear in both directions once a word has a productive record; typed `en→he` answers are accepted without nikud — covered: yes
- [x] ac-4: the home card shows the correct due count and disappears when nothing is due — covered: yes
- [x] ac-5 (REQ-009): page loads lesson data only through script tags; all labels are bilingual; nikud on by default with the existing toggle — covered: yes
- [x] ac-6: if `vocab-index.js` is missing, the page shows a bilingual "no review data yet" message instead of failing — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | When does a word get its first `en→he` record? | after its first `he→en` pass, so production follows recognition | resolved | yes — orchestrator 2026-10-08 (mode-e run) |
| aud-2 | Page path | `review/index.html` at the site root (not inside `worksheets/`) | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `review/index.html`, `shared/review-page.js`, styles appended to `shared/worksheet.css`
- [x] `index.html` — review card and its status logic
- [x] Tests: extend `test/chrome-audio-smoke.js` (or new `test/chrome-review-smoke.js`) to seed `hebrew-review:v1`, open the page, reveal, grade, and assert the stored record changed

---

## Technical Requirements (domain-specific)

- [ ] RTL layout, Rubik font, existing button and card classes; inline SVG icons via `U.icon`.
- [ ] All text from the index rendered through `U.escapeHtml`.
- [ ] Keyboard: Space reveals, 1/2 grade.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; review smoke check.
- **Rollback plan:** delete `review/` and `shared/review-page.js`; revert `index.html` and CSS.

---

## Security

- **Sensitivity:** standard — triggers matched: rendering author-supplied text.
- **Security acceptance criteria:**
  - [x] No `innerHTML` with unescaped index values; a fixture meaning containing `<img onerror>` renders as text.
- **Review depth:** standard · **focus areas:** XSS.

---

## Verification Strategy

- **Claim:** the review page runs recall-before-reveal over interleaved due items from all lessons and updates schedules correctly.
- **Check + executor:** `mechanized` — headless Chrome smoke script against the local server, with a seeded store and fixture index; plus `human` — the operator does one real session on a phone and confirms audio, RTL, and labels.
- **Pass condition:** the smoke script observes hidden answer before reveal, interleaved lesson sources, the record's due date advancing after "Knew it" and resetting after "Missed it", and the XSS fixture rendered as text; the operator confirms the phone session works. `npm test` exits 0.

---

## Integration Points

- Reads `window.HEBREW_VOCAB_INDEX` (SPEC-002) and `window.HebrewReview` (SPEC-003).
- Produces **Change:** new review page and home card.
- **Provenance tag:** `INIT-001/SPEC-004`
