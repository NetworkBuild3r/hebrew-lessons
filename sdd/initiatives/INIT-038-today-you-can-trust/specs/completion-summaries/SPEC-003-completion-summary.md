# INIT-038/SPEC-003 completion summary: one clip lookup

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Clip lookup](640f57a5-5784-4a83-a44a-82b3c6fc8e70)'s return. Not independently verified.

## What changed

A word has a clip when `lexemes.audio_id` is set or an `audio_assets` row matches its exact pointed text. `LexemeHasClipSQL` in `internal/db/clip.go` is the only check. Review, due counts, the lexicon read, and the skipped-audio audit use it. Lessons, new cards, and recorded evidence create a listen card only when the lookup is true. A clipless listen card is left out of the due count.

## Tests

`internal/db`, `internal/mcpstore`, `internal/session`, and `internal/evidence` passed. `internal/learner` failed only `TestListFirstPageOver50kAttempts` (241ms and 291ms against a 200ms limit). That bench does not call the clip lookup. CI already sets `LEARNER_PERF_SLACK=2` because this bench fails on a loaded machine. The rest of `internal/learner` passed with that test skipped. `TestListenPick` in `internal/web` passed.

## Deviations

- `InsertCard` and placement inserts are not guarded. The spec names the lesson, new-card, and `insertCardTx` paths. Due counts still omit a clipless listen card inserted the other way.
- A dry-run replay still counts a clipless word as written, because that path does not call `insertCardTx`. The real write skips the card.
- Kit timer and trace scripts are not on disk. No trace was written.
- AC verification tier is full. An independent check waits until this batch stops editing the tree.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
