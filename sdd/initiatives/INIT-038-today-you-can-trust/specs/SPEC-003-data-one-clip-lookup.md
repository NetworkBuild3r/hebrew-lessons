# SPEC-003 · data: one way to find a word's clip, and no listening card without one

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-038
title: "One way to find a word's clip, and no listening card without one"
domain: data
status: in_review
primary_prompt: none
complexity: medium
complexity_reason: replaces three audio_id-only checks with one lookup and guards every card-creation path
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-006, SPEC-013]
```

## Description

Review looks for a word's clip only in `lexemes.audio_id`, and no code writes that column:
- `ActivityPool`, `internal/db/activity.go:34`;
- `CountLexemesWithAudio`, `internal/db/standing.go:71`;
- `internal/mcpstore/lexicon.go:151`.

Lessons and verb forms find clips by text in `audio_assets` (`internal/db/taught.go:212`, `internal/db/verbforms.go:124`). So review never sees the clips the app stores, and its due listening cards are dropped (`filterReviewAskable`, `internal/session/mix.go:267`) while staying "due".

- **One lookup.** A word has a clip when `lexemes.audio_id` is set or an `audio_assets` row exists for its exact pointed text. Every audio check uses it:
  - `ActivityPool`, so review's `HasAudio` is right (`internal/session/ask.go:72`);
  - `CountLexemesWithAudio`;
  - the MCP lexicon read;
  - `CountReviewSkippedNoAudio` (`internal/learner/audit.go:246`).
- **No listening card without a clip.** Every card-creation path creates a listen card only when the word has a clip:
  - `internal/db/lesson.go:825-835`;
  - `CreateNewCards` in `internal/db/cards_new.go`;
  - `insertCardTx` in `internal/db/evidence.go`.
- **Due counts** leave out listen cards whose word has no clip.

**What Brian will see differently:** nothing on its own. Once SPEC-013 makes word clips and deploys it, listening questions come back in review, and the due count stops including listening cards he cannot be asked.

**Removes:** the three `audio_id`-only checks, replaced by the one lookup.

**Scope (out):** making clips (SPEC-013); the session size (SPEC-006).

## Acceptance criteria

- [ ] On a fixture where a word's clip exists only in `audio_assets`, review asks its listening card and plays the clip.
- [ ] No path creates a listen card for a word with no clip, on fixtures for lessons, new cards, and recorded evidence.
- [ ] A clipless listen card is left out of the due count.
- [ ] `CountReviewSkippedNoAudio` uses the same lookup and returns 0 when every due listen word has a clip.
- [ ] `rg "audio_id IS NOT NULL"` (or the equivalent check) finds no audio check outside the one lookup.

## Verification Strategy

- **Claim:** review sees every clip the app stores, and no listening card exists that cannot be asked.
- **Check + executor:** mechanized. `go test ./internal/db/... ./internal/session/... ./internal/learner/...` with clip fixtures in each table.
- **Pass condition:** every acceptance case passes.
