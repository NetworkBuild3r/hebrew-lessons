# SPEC-007 · frontend: a 0.75× control on every clip

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-036
title: "A 0.75× control on every clip"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: touches four audio scripts and every replay surface, with one shared setting
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-012]
```

## Description

Only placement can slow a clip (`internal/web/static/js/placement.js:62`, `:144-147`). The project's own guidance A4 asks for a replay control and a 0.75× option on every feedback sheet (`internal/units/rules.go:228`), and RSCH-003 line 181 asks that the learner control audio pace.

- One shared helper in `internal/web/static/js` sets `playbackRate` on the clip it plays. `clips.js`, `dialogue.js`, `activity.js` and `follow.js` use it.
- A "Slower" toggle sits beside Replay on lesson feedback, the review prompt and feedback, the word page, and each dialogue line. Placement keeps its own button, now backed by the same helper.
- The choice is stored in the browser (`localStorage`), applies to the next clip, and defaults to normal speed. No server data and no new column.
- Autoplay keeps normal speed unless the learner has turned slow on.
- Slow playback keeps pitch (`preservesPitch`) where the browser supports it.

**Scope (out):** a speed slider, and the minimal-pair screen (SPEC-012), which reuses this control.

## Acceptance criteria

- [x] Every surface above has a Slower control that plays the clip at 0.75× and a Replay that follows the current setting.
- [x] The setting survives a reload and applies across surfaces.
- [x] Turning it off returns to 1×.
- [x] A clip that fails to play reports it as before; slow mode adds no new error state.
- [x] The control is a labelled button with a visible pressed state, reachable by keyboard, and at least 44px tall on a phone.
- [x] Placement behaviour and its browser test are unchanged.

## Verification Strategy

- **Claim:** every replayable clip can be played slowly, and the setting persists.
- **Check + executor:** mechanized. Extend the existing browser tests (`internal/web/testdata/*-browser.mjs`) to assert `playbackRate` after pressing Slower on each surface; `go test ./internal/web/...`.
- **Pass condition:** the browser run reads 0.75 on each listed surface after the toggle and 1 after it is off, including after a reload.
