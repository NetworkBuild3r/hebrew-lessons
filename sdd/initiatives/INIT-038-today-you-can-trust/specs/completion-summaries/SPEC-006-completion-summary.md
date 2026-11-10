# INIT-038/SPEC-006 completion summary: due means due today

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Due today, one session](3231b815-e41e-44f0-b3d2-df54e2cc14de)'s return. Not independently verified.

## What changed

A review scheduled for later the same study day is stored at the next study-day cutoff. Cards already on that short step stay out of the due count until then. Clipless listen cards stay out. One session is 20 to 30 minutes: 25 minutes at a 12-second median over the last 200 answers, or 25 minutes at a 75-second default when fewer timed answers exist. Typing or dictation is asked before a meaning pick. `Session.Minutes()` returns the planned length.

`SessionCap` stays 20 so Today's current line still compiles. SPEC-011 owns the button. There is no local copy of learner 1's cards, so the due-count drop was checked on a fixture (3 raw, 1 askable). `internal/learner/replay.go` still walks raw FSRS dues, so a stored cutoff can disagree with that walk. SPEC-004 owns the card replay.

## Tests

`./internal/session/...`, `./internal/evidence/...`, `./internal/fsrs/...`, and `./internal/studyday/...` passed.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
