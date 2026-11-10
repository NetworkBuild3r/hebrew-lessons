# SPEC-006 · service: due means due today, and one session sized in minutes

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-038
title: "Due means due today, and one session sized in minutes"
domain: service
status: in_review
primary_prompt: none
complexity: medium
complexity_reason: changes when a card is due and how many are asked, which every review reads
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001, SPEC-003]
blocks: [SPEC-011, SPEC-013]
```

## Description

- **Due today.** A review whose next due time falls inside the same study day is due at the next study-day cutoff (`studyday.DueCutoff`, `internal/studyday/studyday.go:150`). Apply this where the recorder schedules (`internal/evidence/recorder.go`, `internal/fsrs/review.go`). A card already due minutes after its last review counts as due at the next cutoff.
- **One session sized in minutes.**
  - The fixed `SessionCap = 20` (`internal/session/mix.go:18`) is replaced by a 20 to 30 minute budget.
  - The budget is turned into a card count using his median seconds per answer over his last 200 answers, with a default when there are fewer.
  - The session returns its size in minutes for SPEC-011 to show.
- **Weighted toward production.** When a word has several due skills, the choice prefers typing from English (`type_it`), and dictation where a clip exists, over a meaning pick.
- **The due count** counts only cards that can be asked today: no clipless listen cards (SPEC-003) and no same-day steps.

**What Brian will see differently:** the due count shows only cards he can be asked today, review stops after about 25 minutes, and more questions ask him to type the Hebrew.

**Removes:** the fixed 20-card cap.

**Scope (out):** `PaceFor` and the 40-learning-words and 75% gates in `internal/brief`, which only lesson briefs read and which are deferred to Stage 2; Today's button (SPEC-011).

## Acceptance criteria

- [ ] A learning step whose next due time is later the same study day is due at the next cutoff.
- [ ] On a copy of learner 1's cards, the due count drops by exactly the clipless listen cards and the same-day steps, and the counts are recorded.
- [ ] A session's planned length, at the learner's median seconds per answer, is between 20 and 30 minutes.
- [ ] With typing, dictation, and meaning pick due for the same word, the session asks typing or dictation first.
- [ ] With no answer history, the session uses the default and still ends within the budget.

## Verification Strategy

- **Claim:** "due" means askable today, and one session fits the time budget.
- **Check + executor:** mechanized. `go test ./internal/session/... ./internal/evidence/... ./internal/studyday/...`, plus the count on a production copy through the local Docker check.
- **Pass condition:** every acceptance case passes, and the before and after due counts are in the completion summary.
