# SPEC-005 · Frontend: Interactive drills restyle (Match, Sentence Builder, Speed Quiz, Type-It)

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-007
title: "Interactive drills restyle (Match, Sentence Builder, Speed Quiz, Type-It)"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-007]
```

---

## Description

**Context:** The interactive activities in Hebrew worksheets — Match (`match.js`), Sentence Builder (`build-sentence.js`), Speed Quiz (`speed-quiz.js`), Type-It (`type-it.js`), and Pattern (`pattern.js`) — represent the active retrieval practice that drives language retention. Right now, these drills look and feel like wireframe prototypes: basic borders, sharp 2px corners, minimal visual affordances, and plain alert text for wrong answers.

This spec gives each interactive drill a tactile, rewarding feel:
1. **Match (`match.js`):**
   - Word cards with comfortable touch targets, distinct hover states, and clear selection borders.
   - When pairs match, they snap into matching soft color pairs (`.pair-c0` through `.pair-c7`) with a checkmark.
   - Wrong match attempts flash a gentle red border with subtle shake animation (120ms), immediately resetting without penalty.
2. **Sentence Builder (`build-sentence.js`):**
   - Dashed recipient slots that invite tapping or placement.
   - Word tile chips that feel magnetic: tapping moves the word into the slot; tapping a slotted word returns it to the bank.
   - Audio preview button for the completed sentence.
3. **Speed Quiz (`speed-quiz.js`) & Fill-in-the-Blank (`fill.js`):**
   - Calm, non-punitive countdown timer (gentle circular or bar countdown; no jarring alarm colors).
   - High-contrast choice cards that indicate selection immediately.
   - Fill inputs with auto-focus and clear word-bank chips.
4. **Type-It (`type-it.js`):**
   - Input field with automatic RTL direction and font scaling.
   - Soft keyboard hint assist for learners without a physical Hebrew keyboard or on mobile.
   - On Miss: Displays the correct pointed Hebrew form immediately with the exact discrepancy highlighted (RSCH-006 Rule 12).
5. **Pattern & Ask-Friend (`pattern.js`, `ask-friend.js`):**
   - Beautiful grammar pattern highlight box with color-coded variables (e.g. prefix in purple, root in bold).
   - Speaking recorder button with smooth pulsing red ring during recording (`.recording`), and immediate side-by-side comparison with the native audio clip.

**Scope (in):**
- Restyling `shared/activities/match.js`, `build-sentence.js`, `speed-quiz.js`, `fill.js`, `type-it.js`, `pattern.js`, `ask-friend.js`.
- Drill-specific CSS in `shared/worksheet.css`.
- Preserving all existing test hooks (`data-action`, `data-match`, `data-fill-feedback`, `data-grammar`, `pair-c0`..`pair-c7`).

**Scope (out):**
- Vocabulary introduction and story reader (covered in SPEC-004).

---

## Understood

- [x] und-1: The eight pair color classes (`pair-c0` through `pair-c7`) must be preserved for `learner-support.test.js`.
- [x] und-2: All input fields and selection cards must have minimum 44px touch targets.
- [x] und-3: Corrective feedback must always show the correct pointed Hebrew form on error.

---

## Acceptance Criteria

- [x] ac-1: In Match, selected cards show clear active borders; matched pairs display assigned `.pair-c0`..`.pair-c7` styles; mismatched taps trigger temporary `.wrong` state without blocking.
- [x] ac-2: In Sentence Builder, word bank tiles have min-height 44px, move to the target slot upon click, and can be tapped again to return to the bank.
- [x] ac-3: In Type-It, wrong answers display the correct pointed Hebrew word with retry option, preserving `data-action="type-check"` and `data-action="type-hint"` hooks.
- [x] ac-4: In Speed Quiz, choices display distinct selected and result states with clear visual contrast.
- [x] ac-5: In Speaking / Ask-Friend, recording button displays animated state during active mic recording and exposes audio playback for user recording and model pronunciation.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Test hook stability | Every class and data-* hook listed in ADR-0006 is preserved exactly | resolved | yes |
| aud-2 | Audio recording | Web MediaRecorder API handled via existing `shared/player/speech.js` bindings | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- Updates to `shared/activities/match.js`, `build-sentence.js`, `speed-quiz.js`, `type-it.js`, `fill.js`, `pattern.js`, `ask-friend.js`
- Updates to `shared/worksheet.css` (drill components and interactive states)
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-005-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test` passes, including `learner-support.test.js` and `retry-before-reveal-smoke.test.js`.
- **Rollback plan:** Revert activity files and CSS changes.

---

## Verification Strategy

- **Claim:** All drills render with tactile cards and clear states; match pairs and type-it hints retain hooks; error states display corrections.
- **Check + executor:** `mechanized` — run `npm test` and `node test/chrome-learner-support-smoke.js`.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-005`
