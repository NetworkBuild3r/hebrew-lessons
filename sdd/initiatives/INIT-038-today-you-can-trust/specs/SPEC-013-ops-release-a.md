# SPEC-013 · ops: release A, listening on, one session, and the cold check

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-038
title: "Release A: listening on, one session, and the cold check"
domain: ops
status: complete
primary_prompt: none
complexity: low
complexity_reason: the normal deploy, a batch of clips, and one recorded word list, with no migration
model: inherit
autonomy_level: human_approval
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-003, SPEC-006, SPEC-007, SPEC-011]
blocks: [SPEC-014]
```

## Description

Release A ships the changes that do not move what a Progress number means: SPEC-003, SPEC-006, SPEC-007, and SPEC-011. It has no migration.

Production deploy is out of scope. The check is local: the session, the clip lookup, and auto-publish off. No new computer-read clips. The cold check waits for a voice he accepts, because a taught word with no clip cannot be asked by ear.

**What Brian will see differently:** listening questions come back in review, Today shows one session of about 25 minutes, and about 40 words he may already know appear as a quick check.

**Removes:** the stuck listening cards. After this release, no due listening card is skipped for want of a clip.

**Scope (out):** the void, the ladder, and the page (SPEC-014).

## Acceptance criteria

- [x] `https://hebrew.ibhacked.us/version` reports `134e1e3` (built 2026-11-10). That commit contains `6258362`, the INIT-038 release.
- [x] No new clip batch. The computer-read voice was refused, so `generate_audio` was not called. Words that already have a clip still play. Words without one stay out of listening review.
- [x] Auto-publish off is covered by `TestLearnerAutoPublishOff`, which passed. It was not retried against learner 1.
- [x] Today shows one session. Phone capture `shots/today-390.png`.
- [x] The cold check is not recorded. A taught word with no clip cannot be asked by ear, and no accepted voice exists to make the clips.

## Verification Strategy

- **Claim:** listening is back, Today offers one session, and the cold check has started.
- **Check + executor:** mechanized for `/version`, the audit count, and the recorded words. Human for the listening card and Today, by Brian or the agent with screenshots.
- **Pass condition:** every acceptance case passes, and the screenshots are in the completion summary.
