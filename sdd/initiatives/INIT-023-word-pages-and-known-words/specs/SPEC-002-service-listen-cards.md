# SPEC-002 · service: Lesson listening answers build a listening card

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-023
title: "Lesson listening answers build a listening card"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-003, SPEC-010]
```

## Description

**Context:** `enqueueNewWords` (`internal/db/lesson.go`) inserts `read` and `recall` cards for a lesson's new words. `recordLessonAnswer` (`internal/web/lesson.go`) sets `SkipSchedule` for every type except `recall`, so a `listen_pick` answer is stored with no card. `StateOf` returns `seen` for a skill with answers and no card. In production, 29 words have listen answers and none has a listen card.

**Scope (in):**
- A lesson `listen_pick` answer schedules the word's `listen` card the way a lesson `recall` answer schedules `read`: `SkipSchedule` is false for `listen_pick`. A first right answer creates the card. `holdEasyUntilReview` applies, so a quick first answer stays at Good.
- `enqueueNewWords` also inserts a `listen` card for a new word when the lesson has a `listen_pick` that targets it and the word has a clip.
- The review mix already accepts `listen` (`session/mix.go`). Check that a due `listen` card is asked as an audio question and never falls back to a read face.

**Scope (out):** Spell cards. Dictation sits behind the letters gate. The overall-state rule (North Star 4.3) is unchanged.

## Acceptance Criteria

- [ ] ac-1: A right lesson `listen_pick` on a word with no `listen` card creates one with one review log row.
- [ ] ac-2: A wrong lesson `listen_pick` on a word with a `listen` card rates Again on that card. A wrong first answer creates no card.
- [ ] ac-3: Finishing a lesson whose `listen_pick` targets a new word with audio leaves that word with `read`, `recall`, and `listen` cards. A word with no audio gets no `listen` card.
- [ ] ac-4: A due `listen` card in a review session plays the clip and grades `listen`.
- [ ] ac-5: The `StateOf` table test gains a row: listen answers plus a young `listen` card is `learning`.

## Deliverables

- [ ] `internal/web/lesson.go`
- [ ] `internal/db/lesson.go`
- [ ] `internal/web/lesson_test.go`
- [ ] `internal/db/lesson_test.go`
- [ ] `internal/session/mix_test.go`
- [ ] `internal/learner/state_test.go`

## Verification Strategy

- **Claim:** listening can now advance past seen.
- **Check + executor:** mechanized — `go test ./internal/web ./internal/db ./internal/session ./internal/learner`.
- **Pass condition:** ac-1 to ac-5 tests pass.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-002`
