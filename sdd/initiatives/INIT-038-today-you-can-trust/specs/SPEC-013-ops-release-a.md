# SPEC-013 · ops: release A, listening on, one session, and the cold check

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-038
title: "Release A: listening on, one session, and the cold check"
domain: ops
status: planned
primary_prompt: none
complexity: low
complexity_reason: the normal deploy, a batch of clips, and one recorded word list, with no migration
model: inherit
autonomy_level: human_approval
gate_actions: [deploy, production_content_write]
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-003, SPEC-006, SPEC-007, SPEC-011]
blocks: [SPEC-014]
```

## Description

Release A ships the changes that do not move what a Progress number means: SPEC-003, SPEC-006, SPEC-007, and SPEC-011. It has no migration.

In this order:
1. **Check.** Brian has confirmed SPEC-001's lines. The commit to deploy contains none of SPEC-002, SPEC-004, SPEC-005, SPEC-008, SPEC-009, or SPEC-010. Run the local Docker check.
2. **Back up** with `hebrew backup --to`.
3. **Deploy.** Pin the image in `home_k3` (the kustomize `newTag`), because the image auto-updater is out of sync. Let Argo CD sync, and check that `/version` reports the commit.
4. **Make clips.** Check the ElevenLabs balance, then make a word clip for every taught word that has none, using `generate_audio` with the exact pointed texts. Due listening cards come first.
5. **Record the cold-check words.** Choose about 40 words from the tutor material and the mistakes bank, all ones the app calls unseen or learning.
   - Make their clips first, because a taught word gets a listening card only if it has a clip.
   - Record them with `record_taught_words`, dated within 30 days (`maxTaughtAge`, `internal/evidence/taught.go:14`).
   - Leave a dated coach note naming the cold check.
   - Their first review asks are the check. Only typed and listening answers count.
6. **Confirm** the release, with the checks listed in the acceptance criteria. Run `scripts/smoke-v3.sh` if it still fits.

**What Brian will see differently:** listening questions come back in review, Today shows one session of about 25 minutes, and about 40 words he may already know appear as a quick check.

**Removes:** the stuck listening cards. After this release, no due listening card is skipped for want of a clip.

**Scope (out):** the void, the ladder, and the page (SPEC-014).

## Acceptance criteria

- [ ] `/version` reports the deployed commit, and that commit contains no release B spec (the daily snapshot is still stamped rule version 4).
- [ ] The audit's `ReviewSkippedNoAudio` is 0 for learner 1.
- [ ] One listening review card plays its clip on the live site, recorded with a screenshot.
- [ ] With auto-publish off, `put_unit` with `publish: true` on a learner scope saves a draft, checked on a test learner, not learner 1.
- [ ] Today shows one session with its minutes, and no side-feature link.
- [ ] The cold-check words are recorded with their date, and each has a clip.
- [ ] The voice credits used are recorded.

## Verification Strategy

- **Claim:** listening is back, Today offers one session, and the cold check has started.
- **Check + executor:** mechanized for `/version`, the audit count, and the recorded words. Human for the listening card and Today, by Brian or the agent with screenshots.
- **Pass condition:** every acceptance case passes, and the screenshots are in the completion summary.
