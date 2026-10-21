# SPEC-012 · Frontend: Flashcards on the Review screen

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-006
title: "Review becomes four-grade flashcards with decks, both directions, audio, keyboard"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-009]
blocks: [SPEC-015, SPEC-020]
```

---

## Description

**Context:** the Review screen is `shared/review-page.js`, mounted by the SPA router under ADR-0007's view contract (`mount(el, ctx)` / `unmount()` / `canLeave()`, ready signal `#app-main[data-route-ready="review"]`). It drives the ladder through `HebrewSync` and `HebrewReview`. RSCH-003 §1.1: recall in both directions (Moderate); recall before reveal.

**Scope (in):**

- Deck picker: Due (default), Weak, New, by lesson, by concept (`binyan:*`, `tense:*`), All. Counts from `GET /api/me/cards?deck=`.
- Card: prompt side (Hebrew pointed for `he_en`; English for `en_he`), play-audio button when a clip exists, **Show answer** (Space), then the answer side with gloss, transliteration toggle, root and binyan chip when tagged, and four buttons Again / Hard / Good / Easy (keys 1–4) labeled with the interval from `intervals`.
- Grading posts a signal with `grade` through `HebrewSync` (server-authoritative; no `localStorage` schedule).
- `en_he` cards offer optional typing; the typed text is compared locally (vowel-stripped against `accept[]` or the stripped key) to suggest a grade; the learner still chooses the grade. Typed text is not sent.
- Session summary: cards reviewed, grades by count. No points, no streak reward (RSCH-006).
- Vowels toggle respected (ADR-0001 #5). Bilingual labels (ADR-0001 #6). ADR-0006 tokens.
- Keep existing `data-*` hooks used by `test/chrome-review-smoke.js`, or update that smoke in this change.
- Comment at the top of `review-page.js` stating: grades go to the server; never schedule in the browser.

**Scope (out):** Words screen (SPEC-013); daily mix (SPEC-015).

---

## Understood (agent's read of the intent)

- [x] und-1: With `SRS_SCHEDULER=ladder`, the screen still works: Hard and Easy map server-side as SPEC-004 defines.
- [x] und-2: Answer is never visible before Show answer (recall before reveal).

## Acceptance Criteria

- [x] ac-1 (REQ-003): a Chrome smoke seeds two due cards through the API, presses Space then 3, and the server shows one Good grade on that card.
- [x] ac-2 (REQ-003): keys 1–4 and Space work; buttons show four interval labels.
- [x] ac-3: the answer text is absent from the DOM before Show answer (smoke checks).
- [x] ac-4: unit test for the local typed-answer comparison: pointed vs unpointed, an `accept[]` variant, a wrong answer.
- [x] ac-5: `test/chrome-navigation-smoke.js` still reaches Review and sees the ready signal.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Separate `/cards` route | No. One review place; the Review screen becomes the flashcards. | resolved | yes — fewer routes, same habit |
| aud-2 | INIT-005 feedback kit | If INIT-005 SPEC-009 has landed, use its feedback components; otherwise plain ADR-0006 styles. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] Changes to `shared/review-page.js`; small helpers in `shared/player/` if needed
- [x] Styles in `shared/worksheet.css`
- [x] Tests: `test/flashcard-answer.test.js`, `test/chrome-flashcards-smoke.js`; updated `test/chrome-review-smoke.js` if hooks change

---

## Technical Requirements (domain-specific)

- [x] All text via `textContent`; no `innerHTML` with data.
- [x] `dir="rtl"` and `lang="he"` on Hebrew nodes.
- [x] Unmount removes key listeners (view contract).

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** unit and Chrome smokes; `npm test`.
- **Rollback plan:** revert the view; server still accepts gradeless signals.

---

## Security

- **Sensitivity:** standard. Escaping of lexicon text.
- **Review depth:** quick · **focus areas:** DOM text insertion.

---

## Verification Strategy

- **Claim:** the learner can review due cards with four grades in both directions, recall before reveal.
- **Check + executor:** `mechanized` — `node --test test/flashcard-answer.test.js`; `node test/chrome-flashcards-smoke.js` against the signed-in test server.
- **Pass condition:** exits 0; server row shows the grade.
- **Test levels:** unit, browser smoke.

---

## Integration Points

- SPEC-015 opens a deck from the daily mix. SPEC-013 links "Review these" to a deck.
- Produces **Change:** flashcards.
- **Provenance tag:** `INIT-006/SPEC-012`
