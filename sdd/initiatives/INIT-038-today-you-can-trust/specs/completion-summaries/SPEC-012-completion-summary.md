# INIT-038/SPEC-012 completion summary: tutor-material audit and the held-out probe

**Status:** in_review, 2026-11-09. The audit is recorded. The probe clips are not made. A `production_content_write` gate is open.

## Audit

G2 passes. 286 usable tutor-written lines across the last 10 classes, and all 10 loaded without Brian. Counts are in [SPEC-012-audit.md](SPEC-012-audit.md). No class text was written into the repo. No write tool was called.

## Probe

Not started. On 2026-11-09 Brian refused the clip gate. The tutor is outside the app, so tutor lines are not held out of lessons that never contained them. He also refused computer-read clips: that voice is not good enough. `generate_audio` was not called. The same voice is what release A would use for missing word clips, so that batch waits for a source he accepts.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
