# INIT-038/SPEC-007 completion summary: auto-publish off

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Auto-publish off](994885d7-30dd-499c-96c5-7bcde9693e47)'s return. Not independently verified.

## What changed

`LEARNER_AUTO_PUBLISH` defaults off. While it is off, `put_unit` with `publish: true` on a learner scope saves a draft and returns `Learner auto-publish is off (ADR-0033). The learner unit was not published.` `publish_unit` returns the same note. A shared unit still publishes. With the switch on, both paths behave as they do today. Server instructions and tool descriptions name `ruleVersion` and no longer restate the known-rule thresholds.

## Tests

`./internal/mcp`, `./internal/app`, and `./cmd/hebrew` passed. `./internal/mcpstore` failed only `TestSearchLexemeAudioPresentUsesEitherClip`, which is SPEC-003's untracked clip test (a foreign key on `lexemes.audio_id`). SPEC-007's own store test passed in a focused run.

## Deviations

- The bool lives on `Store` in `internal/mcpstore/store.go`. The spec named `learner.go`, which is where the refuse happens.
- To make `internal/db` compile, a duplicate `type rowQuery` was removed from untracked `internal/db/clip.go`. `users.go` keeps the type. That file belongs to SPEC-003.
- Kit timer and trace scripts are not on disk. No trace was written.
- AC verification tier is full. The implementer checked its own tests. An independent check waits until the other specs in this batch stop editing the tree.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
