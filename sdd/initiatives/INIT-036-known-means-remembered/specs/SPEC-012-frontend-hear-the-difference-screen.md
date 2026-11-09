# SPEC-012 · frontend: the "hear the difference" screen

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-036
title: "The \"hear the difference\" screen"
domain: frontend
status: planned
primary_prompt: none
complexity: medium
complexity_reason: one new screen in the review flow, audio-led, with accessibility needs for a listening task
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-007, SPEC-011]
blocks: []
```

## Description

The screen for the SPEC-011 drill, in the same shell as the letter drill screen.

- The prompt is the clip. Play, and Slower from SPEC-007, are the first controls. The clip plays once on arrival only if the learner's autoplay setting allows it.
- Two large choices show the two pointed words. Nothing about the sound is written on the prompt, so the learner cannot answer by reading.
- After a pick, feedback shows both words with both clips replayable, and says which one was played. A wrong pick is not marked as a failure of the learner; it names the difference, using the contrast's "what to listen for" text from `docs/sound-contrasts.md`.
- A short intro the first time the drill appears says what it trains and that it is not graded against the learner's progress numbers.
- Works with keyboard only, on a phone, and in dark mode. Every control has a text label. Because this is a listening task, the page offers a way to continue if audio cannot play, and says so.

**Scope (out):** recording the learner's voice and any automatic checking.

## Acceptance criteria

- [ ] The screen shows the clip controls first and two pointed-word choices, with no text hint of the answer before a pick.
- [ ] Feedback replays both clips, normal and slow.
- [ ] A wrong pick shows the contrast's listening note and no failure styling.
- [ ] The first-time intro appears once per learner.
- [ ] If audio fails to load or play, a visible message appears and the learner can continue.
- [ ] Keyboard-only completion works, and the choices are at least 44px tall at 360px wide.
- [ ] Dark mode keeps contrast at WCAG 2.2 AA.

## Verification Strategy

- **Claim:** the drill is usable by ear, by keyboard, on a phone, and when audio fails.
- **Check + executor:** mechanized. A browser test walks the screen with a fake clip, with keyboard only and at 360px, once with audio blocked; `go test ./internal/web/...`.
- **Pass condition:** the walk completes in all three modes and finds no answer text before the pick, no failure styling on a wrong pick, and the failure message when audio is blocked.
- **Declared human check:** a native speaker or the tutor listens to the live clips once before release. This is SPEC-008's sign-off applied to the served pairs, and cannot be mechanized.
