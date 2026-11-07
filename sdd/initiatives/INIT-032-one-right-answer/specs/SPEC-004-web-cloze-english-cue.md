# SPEC-004 · web: show the English line on a cloze

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-032
title: "Show the English line on a cloze"
domain: web
status: built, uncommitted
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: [SPEC-001]
blocks: [SPEC-005]
```

## Description

In `player.step` (`internal/web/lesson.go`), a cloze sets `activity.Cue` to the trimmed `translation` on the ask and on the reveal. The player template already writes `Cue` as `<p class="say-cue" lang="en">` above the prompt, the same line `say` and `order` use, so no `.templ` change and no `templ generate`.

`activityPrompt` for a cloze with a translation is "Pick the word that makes the Hebrew say this." A cloze with no translation keeps "Pick the missing word."

The buttons do not change: pointed Hebrew, no reading, no gloss.

Built in the working tree. `TestClozeHidesChoiceTranslitUntilGraded` (`internal/web/cloze_translit_test.go`) now gives its cloze "I study Hebrew." and checks the cue on the ask, on a full page and on an htmx swap.

Still to do: a screenshot of the cloze ask at 390 px, to check that the cue, the line, and four buttons fit without scrolling.

## Acceptance criteria

- [x] A cloze ask with a translation shows it in `say-cue`.
- [x] No reading or gloss appears on the buttons before the answer.
- [x] `go test ./internal/web/...` passes.
- [ ] At 390 px the cue, the line, and the buttons are on one screen.
