# SPEC-002 · Frontend: Feedback kit, tactile interactions, and audio playback states

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-007
title: "Feedback kit, tactile interactions, and audio playback states"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005, SPEC-006]
```

---

## Description

**Context:** During self-study, learning satisfaction and perceived difficulty depend heavily on immediate, clear feedback and physical UI affordances. When a user presses a button, plays an ElevenLabs clip, or submits an answer, current feedback is either non-existent or a plain colored border.

This spec builds the interactive foundation:
1. **Audio Playback Indicators:** A dedicated playback state component for ElevenLabs audio. When audio starts loading, the speaker button indicates loading; when audio plays, a clear animated waveform or speaker pulse is visible; on error, a graceful fallback notice appears. Instant replay affordance with keyboard shortcut (e.g. Space / 'r' when not in an input).
2. **Tactile Interaction States:** Micro-interaction styles across all player buttons, chips, and cards: subtle active scale (`transform: scale(0.98)`), accessible focus rings with offset, and hover lift (`translateY(-1px)`).
3. **Correct / Miss Competence Components:**
   - On Correct: Clean emerald check with a subtle affirmation (no patronizing confetti).
   - On Miss: Immediate display of the correct pointed Hebrew form, highlighting the key syllable or root, with one clean retry button before advancing (RSCH-006 Rule 12, 13).
4. **Subtle Audio Effect System (Off by Default):**
   - Web Audio API synthesizer for tiny, gentle tactile clicks on match/submit (pure tone/chime, no external mp3 assets).
   - Off by default per INIT-005 REQ-014 / RSCH-007, with a persistent mute toggle in the shell. Never overlaps with Hebrew speech audio.
5. **Reduced Motion:** Everything honors `prefers-reduced-motion: reduce` by zeroing transition durations and transforms.

**Scope (in):**
- Implementation of feedback components in `shared/player/feedback.js` (or in `shared/worksheet.css` and `shared/player/speech.js`).
- Audio state hooks in `shared/player/speech.js`.
- Sound effect synthesizer with persistent local toggle.
- CSS classes and animations in `shared/worksheet.css`.

**Scope (out):**
- Specific activity template layout restructuring (handled in SPEC-004 and SPEC-005).

---

## Understood

- [ ] und-1: Sound effects must be off by default and never play over Hebrew voice clips.
- [ ] und-2: When an answer is wrong, the feedback component must show the correct pointed Hebrew word before the user leaves the step.
- [ ] und-3: All animations must shut off when `prefers-reduced-motion: reduce` is detected.

---

## Acceptance Criteria

- [ ] ac-1: Playing any audio clip via `HebrewSpeech.playClip` adds an active playing class (e.g. `is-playing`) to the associated audio button or container, and removes it cleanly on `ended` or `error`.
- [ ] ac-2: When an answer is incorrect, the feedback container displays the correct pointed Hebrew word with class `correction` and preserves `data-action` retry hooks.
- [ ] ac-3: Interactive buttons (`.btn`, `.choice-card`, `.word-match-card`, `.bank-word`) render an active scale transformation (`scale(0.98)`) and high-contrast focus rings.
- [ ] ac-4: Web Audio tactile clicks can be enabled via user setting, are silent by default, and automatically duck/mute whenever Hebrew speech audio is active.
- [ ] ac-5: Under `prefers-reduced-motion: reduce`, all transitions and transforms resolve to `none` with 0s duration.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Web Audio synthesis | Generate gentle sine-wave chimes in JS rather than downloading third-party audio files | resolved | yes |
| aud-2 | Audio button hook | Audio buttons bind to `data-audio` or `data-speak` and receive dynamic status attributes | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- `shared/player/feedback.js` (feedback and optional audio chime module)
- Updates to `shared/player/speech.js` (playback event dispatching)
- Updates to `shared/worksheet.css` (tactile states, audio playing pulse, correction panels)
- Completion summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-002-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test` passes without errors; existing `chrome-audio-smoke.js` passes.
- **Rollback plan:** Revert changes to `shared/player/speech.js` and `shared/worksheet.css`.

---

## Verification Strategy

- **Claim:** Audio buttons visually indicate playback state; incorrect answers immediately display the correct form; buttons respond with physical press states; and reduced-motion stops all motion.
- **Check + executor:** `mechanized` — run `npm test` and `node test/chrome-audio-smoke.js`; test CSS properties for reduced motion queries.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-002`
