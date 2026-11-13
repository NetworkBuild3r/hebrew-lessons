# SPEC-019 · service: lessons from the words that cover speech

## Metadata

```yaml
spec_id: SPEC-019
initiative_id: INIT-039
title: "Lessons from the words that cover speech"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: adds the frequency list and one lesson page, and leaves the class-sheet assembler unused by Today
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: []
blocks: []
```

## Description

Brian said on 2026-11-11 not to build lessons from his classes, and to build them from the common words that let a person do most of the speaking.

The list is Juan D. Pinto's Frequency Dictionary of Spoken Hebrew (2018): 5,000 lemmas from OpenSubtitles2018. The first 30 cover about 48 percent of that subtitle corpus. All 5,000 cover about 91 percent. Subtitles are the published stand-in. A recorded conversation corpus of this size was not finished.

The first lesson uses six lemmas from the top of the list. He hears them on the page as pointed words, types one with the Hebrew hidden, and sees one line to say. The app does not grade the speaking, and it does not store the type answer. A graded word that is not on the list is refused. When nothing is due, Today links to this lesson. A class-sheet lesson is not what Today offers.

## Acceptance criteria

- [x] The list has 5,000 lemmas. The first 30 cover about 48 percent of the subtitle corpus, and all 5,000 cover about 91 percent.
- [x] The sample lesson's graded words are on the list. A word that is not on the list is refused.
- [x] When nothing is due, Today links to the lesson, and the page shows the words, the type step, and an ungraded line to say.
- [x] Today does not offer a lesson built from a class sheet.

## Verification Strategy

- **Claim:** the lesson is built from the subtitle frequency list, and a word off that list is not graded right.
- **Check + executor:** mechanized. `go test` on `./internal/speech/` and `TestSpeechLessonAfterReview` in `./internal/web/`.
- **Pass condition:** those tests pass.
