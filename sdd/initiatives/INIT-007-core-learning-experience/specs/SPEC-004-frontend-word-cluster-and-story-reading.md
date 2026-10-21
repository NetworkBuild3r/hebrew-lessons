# SPEC-004 · Frontend: Word cluster and story reading experience

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-007
title: "Word cluster and story reading experience"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-007]
```

---

## Description

**Context:** The core learning moments in Hebrew worksheets are meeting new vocabulary words (`shared/activities/words.js`) and reading the pointed story (`shared/activities/story.js`). Currently, word cards look like database rows with small fonts and plain audio icons. The story reader has basic styling where follow-along highlighting can be hard to track, and gloss popovers look like standard browser tooltips.

This spec implements the research recommendations:
1. **The 48px Pointed Word Cluster (`RSCH-006` Rule 1, 9):**
   - Pointed Hebrew rendered as live text at 48px (40px on small mobile, 56px on desktop) with line-height 1.5–1.7 to ensure no vowel mark clipping.
   - Word + English gloss + audio button + optional contextual image rendered in **one unified cluster card**.
   - Prominent, beautiful audio play button right next to the word that triggers the active playing pulse from SPEC-002.
   - Part of speech and memory traps styled cleanly in distinct pills.
2. **Warm Immersive Story Reader (`RSCH-006`, `RSCH-007`):**
   - Story text rendered on warm paper surface (`#fffbeb` / soft cream ground) with 1.7 line height for comfortable, fatigue-free reading.
   - Follow-along read-aloud: current sentence highlights with a warm amber wash and subtle left border accent.
   - Read-aloud toolbar: Pause, Resume, Stop buttons styled with clear icon + text labels and touch-friendly targets.
3. **Interactive Gloss Popover (`INIT-001/SPEC-007`):**
   - Tapping an unknown word token opens an elevated, rounded card with pointed Hebrew, English gloss, and an inline audio button to hear the isolated word.
   - Popover closes cleanly on outside click or escape key, maintaining keyboard accessibility.

**Scope (in):**
- Restyling `shared/activities/words.js` and `shared/activities/story.js`.
- CSS rules for `.vocab-card`, `.he-word`, `.story-body`, `.story-gloss-popover`, and `.reading-now` in `shared/worksheet.css`.

**Scope (out):**
- Other drill activities (handled in SPEC-005).

---

## Understood

- [x] und-1: Pointed Hebrew must remain live resizable text, never an image of text (RSCH-006 Rule 2).
- [x] und-2: Pointed text line height must never drop below 1.5, to prevent vowel marks from colliding or clipping (RSCH-006 Rule 3).
- [x] und-3: Follow-along highlighting must transition smoothly and follow speech playback.

---

## Acceptance Criteria

- [x] ac-1: In the vocabulary introduction screen, the pointed Hebrew word renders at font size ≥40px with line-height ≥1.5 and `overflow: visible`.
- [x] ac-2: The word cluster places the pointed Hebrew word, English gloss, audio button, and image (if present) in one unified container.
- [x] ac-3: In the story screen, the currently reading paragraph receives the `.reading-now` class with high-contrast background and text visibility.
- [x] ac-4: Tapping any `.story-token` displays the `.story-gloss-popover` containing the pointed word, English meaning, and an audio playback button.
- [x] ac-5: Story controls (Pause, Resume, Stop) have pointer targets of at least 44x44px and clear visual active states.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Word cluster layout | Vertical stacking on mobile (image/cluster top, word, gloss, tools), side-by-side on wide screens | resolved | yes |
| aud-2 | Audio trigger | Clicking the word or its speaker button plays the pronunciation | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- Updates to `shared/activities/words.js`
- Updates to `shared/activities/story.js`
- Updates to `shared/worksheet.css` (`.vocab-card`, `.he-word`, `.story-body`, `.story-gloss-popover`)
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-004-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test` passes; `test/story-gloss.test.js` passes.
- **Rollback plan:** Revert `words.js`, `story.js`, and story CSS.

---

## Verification Strategy

- **Claim:** Pointed words render at ≥40px without clipping; word clusters unify text and audio; story highlighting and gloss popovers function without regression.
- **Check + executor:** `mechanized` — run `npm test` including `test/story-gloss.test.js` and `test/chrome-story-gloss-smoke.js`.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-004`
