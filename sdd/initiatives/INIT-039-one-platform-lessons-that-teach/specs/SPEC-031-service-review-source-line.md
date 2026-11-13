# SPEC-031 · service: review says where each word came from

## Metadata

```yaml
spec_id: SPEC-031
initiative_id: INIT-039
title: "Review says where each word came from"
domain: service
status: complete
primary_prompt: none
complexity: low
complexity_reason: one field carried from the mix to the card view, and one line in the card
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 2 hours
blocked_by: [SPEC-024]
blocks: []
```

## Description

Brian asked that review mix words from his tutor's classes with the course and his failed words. It already does the failing part: missed words come back, struggle words are woven in, and letters get drills. What is missing is that nothing says where a card came from, so a sitting feels like one undifferentiated pile.

This spec carries the source to the card and shows it in one short line. It changes no ordering, no grading, and no card count.

## What to build

### `internal/session`

- `type Source string` with `SourceCourse`, `SourceTutor`, `SourceOther`.
- In `build` (`internal/session/session.go`), beside the course rank map already loaded, load the learner's class-sheet lexeme ids: `func (s *Service) sheetLexemeIDs(ctx, userID) (map[int64]bool, error)` reads `store.ClassSheets`, tokenizes each tutor line with `units.Tokenize`, and maps a token to a stored lexeme by the same spelling rule the course sync uses. A token in both the course and a sheet is `SourceCourse`, because the course is the ranking.
- `Candidate` gains `Source Source`. A lexeme in neither set is `SourceOther`, which today means a class-sheet unit from the old pipeline or a placement seed.
- `CardRef` gains `Source Source`, copied from the candidate.

### `internal/web`

- `ReviewCardVM` and `views.ReviewCard` gain `Source string`.
- `reviewView` sets it, and `views.ReviewBody` renders one line above the prompt, `data-source`, only while the front is showing:
  - course → "Common words";
  - tutor → "From your tutor";
  - other → "From an earlier lesson".
- No line on the back, so the answer stays the focus. No line on a drill step.

## Tests

- `internal/session`: `TestCardsCarryTheirSource`. Three course words, two words that came from a sheet, and one word in neither: each `CardRef.Source` is the right value. A word in both is `SourceCourse`.
- `internal/web`: `TestReviewShowsTheSource`. A card with each source renders its line once, on the front only, and a letter drill renders none.

## Acceptance criteria

- [ ] Every study visit carries a source, and the course wins over a sheet.
- [ ] The front of a review card names the source in one short line.
- [ ] Ordering, counts, grading, and requeue are unchanged. The existing session tests pass without edits to their expected queues.

## Verification Strategy

- **Claim:** review mixes both sources and names each one.
- **Check + executor:** mechanized. `go test ./internal/session/ ./internal/web/`.
- **Pass condition:** both pass with no changes to existing expected queues.
