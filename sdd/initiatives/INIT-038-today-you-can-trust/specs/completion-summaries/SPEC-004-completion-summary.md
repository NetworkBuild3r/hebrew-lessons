# INIT-038/SPEC-004 completion summary: void with a true undo

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Void and undo](bd13f884-87bd-49c5-b54c-9270b35f0d3b)'s return. Not independently verified.

## What changed

A void writes the flag, skips that answer's confusion row, and replays the card from the reviews that still count, using the same scheduler as a live answer, including the study-day cutoff. Undo marks the batch undone and replays again. `hebrew void-evidence` dry-runs unless `--apply` and `--backup-taken` are both set. It prints counts by word, skill, and reason, and never prints answer text. `--undo` also requires `--backup-taken`.

The dry run was a fixture. There is no local copy of learner 1's cards. An answer that is not the last review leaves later answers in the replay, which is the card without the voided answer. If no reviews remain, the card is new. An answer already voided is skipped. An answer in both selections is stored once, as `unit_12_34`.

## Tests

`./internal/evidence/...` and `./cmd/hebrew/...` passed. `--apply` was not run on a real database.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
