# SPEC-006 · frontend: Letter pick drill, done-screen changes, and the study-day queue

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-027
title: "Letter pick drill, done-screen changes, and the study-day queue"
domain: frontend
status: pending
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-003, SPEC-004, SPEC-005]
blocks: []
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- **The study day is the UTC date.** `sameStudyDay` (`internal/web/learn.go:434`) compares UTC dates. It gates `queueLocked` (`:327`, `:354`), `evictLocked` (`:376`), the grade handler (`internal/web/web.go:499`), and the answer and next handlers (`internal/web/activity.go:115`, `:191`).
  - In Denver (MST), 00:00 UTC is 5 PM. Answers to a card served before 5 PM are refused with 400 after it, and the queue is rebuilt mid-evening.
  - `studyday.DueCutoff(now, zone)` (`internal/studyday/studyday.go:150`) is the next 4:00 in the zone.
  - `server.learnerZone` (`learn.go:726`) resolves the learner's zone, falling back to the app zone the resolver holds, never UTC.
- **A letter drill is a self-graded flashcard.**
  - `fillDrill` (`learn.go:600`) fills the sides, the cues, and the example word (`confusionExample`, `learn.go:712`).
  - `pickPart` (`learn.go:737`) chooses the asked letter on the server.
  - Reveal (`web.go:420`) and grade (`web.go:481`) check `askedInPair` and `q.asked`.
  - The front is rendered by `flashFrame`, from `ReviewBody` (`internal/web/views/pages.templ:17`).
- **The done screen** says only "Done for now" and `ReviewedLine(c.Answered)` (`pages.templ:23-27`).
- **From the service specs:**
  - SPEC-003: `Session.DrillAnswered(ctx, cardID, correct, latency) (DrillOutcome, error)`.
  - SPEC-004: `Session.Answered(ctx, cardID, res) (AnswerOutcome, error)` and `Session.Tally()`.
  - SPEC-005: seeded words arrive with `Intro`, which the screen already handles.
- ADR-0023 D1 and D6 (SPEC-001).

**Scope (in):**
- **The study-day queue.**
  - `studyQueue` gains `rollover time.Time`. When the queue is built, it is set to `studyday.DueCutoff(builtAt, zone)` from `learnerZone`.
  - `sameStudyDay(built, now)` is replaced by `q.sameDay(now) bool`, which is `!q.rollover.IsZero() && now.Before(q.rollover)`.
  - Every caller listed above uses it.
  - If the zone cannot be resolved when the queue is built, return the error (500). Never fall back to a UTC date.
  - Rewrite the comments that say "UTC day" to say "study day".
  - Update the tests that pinned UTC midnight. Add one that serves at 16:50 MST on 2026-11-03, answers at 17:30 MST (00:30 UTC on 2026-11-04), and gets the answer graded. Add another where an answer at 04:01 the next local morning is refused and the queue is rebuilt.
- **The letter pick.**
  - A letter-pair step with two parts is no longer revealed and self-graded.
  - The front shows the example word from `confusionExample` with the asked letter's first occurrence replaced by a blank, plus its gloss and clip. With no example, it shows the clip of the asked letter, or its sound, and no letter. It then shows two buttons, one per letter of the pair, in a stable order, and a "Don't know" link.
  - The page carries no answer before the pick: no name, sound, cue, or transliteration of the asked letter.
  - The asked letter is chosen as today (`pickPart`) and stored on the queue when the step is served (`q.asked`), with `q.servedAt = s.now()`.
  - New handler `POST /_hx/review/drill` (CSRF and Origin like the others) with `card_id` and `pick`:
    - It refuses with 400 and writes nothing when: the queue is not this learner's; the card is not the one served; the step is not a two-letter letter step; the step was already answered; `pick` is neither letter of the pair (nor `dont_know`); or the study day ended.
    - Otherwise it calls `DrillAnswered(ctx, cardID, pick == q.asked, s.now().Sub(q.servedAt))`. "Don't know" is wrong.
    - It then renders feedback: right or wrong, the two letters with their names and the tell-apart cue (the current back), the full example word, and "Cleared" when the outcome cleared the pair. A Continue button posts to `/_hx/review/next`.
  - `/_hx/review/next` accepts a drill step whose feedback is set, the same way it accepts a checked answer.
  - The old reveal and grade paths refuse a two-letter letter step with 400. Single-letter steps and pattern steps keep reveal and self-grade.
- **Feedback on checked answers.** When `AnswerOutcome.Implied` is true, the feedback adds one line: "Also counts for reading" (or "for listening"). When `AnswerOutcome.Slow` is true, it adds "Right, but slow. It will come back sooner."
- **The done screen.** Under the reviewed line, show the non-zero rows of `Session.Tally()`:
  - "N moved up";
  - "N letter pairs cleared";
  - "N slow, coming back sooner";
  - "N new words".
  - It keeps "Back to Today". The tally of the finished sitting is kept on the queue, so a reload of the done screen shows the same rows.
- **Copy.** The "write like a person" voice of the existing review strings. English strings go in the same string table the review uses. Hebrew UI language gets the same rows (put the Hebrew in the table; if a Hebrew string is missing, the English shows, as other review strings do).
- Run `/tmp/go/bin/templ generate` from the module root and commit the generated files.

**Scope (out):** Pattern drills as a pick. Timing flashcards. The Today page. MCP.

## Acceptance Criteria

- [ ] ac-1: A card served at 16:50 MST and answered at 17:30 MST the same evening is graded (200), not refused. A queue built before 04:00 local time is rebuilt after it.
- [ ] ac-2: No code in `internal/web` decides the study day from a UTC date. `rg -n 'UTC\(\)\.Date\(\)' internal/web` finds nothing in non-test code.
- [ ] ac-3: A two-letter letter step renders the example word with a blank and two buttons, and no name, sound, or cue of the asked letter appears in the HTML before the pick.
- [ ] ac-4: Picking the right letter writes one right drill answer with latency equal to the clock difference between serve and pick. Picking the other letter writes one wrong answer and requeues the step. A third pick for the same serve, a pick that is not a pair letter, or another learner's post is 400 and writes nothing.
- [ ] ac-5: The third quick right pick shows "Cleared", and the pair's remaining steps do not appear in the sitting.
- [ ] ac-6: Reveal and grade on a two-letter letter step return 400.
- [ ] ac-7: A right moved-up answer shows the "Also counts" line. A slow right answer shows the slow line.
- [ ] ac-8: The done screen shows the moved-up, cleared, slow, and new rows with the sitting's counts and omits zero rows.

## Deliverables

- [ ] `internal/web/learn.go`, `internal/web/web.go`, `internal/web/activity.go`, `internal/web/drill.go` (new handler)
- [ ] `internal/web/views/pages.templ`, the drill and activity templates, the review strings, and generated `*_templ.go`
- [ ] tests in `internal/web` (study day, drill pick, done screen, feedback lines)
- [ ] CSS only if the two buttons need it, in the existing review stylesheet

## Verification Strategy

- **Claim:** Review keeps working through the learner's evening. A letter drill is a timed pick that can end, and the done screen says what the sitting changed.
- **Check + executor:** mechanized. `/tmp/go/bin/templ generate` (clean diff after), `/tmp/go/bin/go build ./...`, `/tmp/go/bin/go test -count=1 -run 'Review|Drill|StudyDay|Done|Answer|Next|Reveal|Grade' ./internal/web`, then `/tmp/go/bin/go test -count=1 ./internal/web/views`.
- **Pass condition:** ac-1 to ac-8 pass. Restoring the UTC `sameStudyDay` fails ac-1. Rendering the cue before the pick fails ac-3.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-006`
- Uses SPEC-003 `DrillAnswered`, SPEC-004 `Answered` and `Tally`, and SPEC-005 seeded `Intro` visits.
