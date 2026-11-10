# INIT-038/SPEC-002 completion summary: voids beside answers

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Voids beside answers](0a40f8be-86d6-4ed7-832a-87eeac94316c)'s return. Not independently verified.

## What changed

Migration 023 adds `answer_voids`. Store functions insert a batch, mark a batch undone, and list by learner or batch. Evidence readers skip a void while `undone_at` is null and count it again once it is set. The same reader returns the first and last checked-success study day per word and skill. No existing answer row is edited or removed. The replay and the void command are SPEC-004.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
