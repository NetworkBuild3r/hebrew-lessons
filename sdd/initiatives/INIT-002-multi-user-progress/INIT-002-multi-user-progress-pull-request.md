# Pull Request: INIT-002 Multi-user accounts and server-side progress

> Git PR artifact produced at initiative completion. Copy this into the GitHub PR body when a PR is opened.
> **Guard:** this artifact is written, but `gh pr create` was NOT run. Nothing in this initiative has been committed.
> **Scope warning:** the working tree also contains uncommitted INIT-001 work and the lesson recordings. Split those, or exclude those paths, before opening a PR that is only INIT-002.

**Title:** `INIT-002: Signed-in multi-user practice, server-side progress, and an admin console`
**Branch:** work is on `main` in place (the planned branch name `multi-user-progress` was not a worktree) → `main`
**Initiative:** INIT-002-multi-user-progress · **Ticket:** none

---

## Summary

- Every lesson, recording, and review page requires a local account. Admins create accounts. There is no public sign-up.
- Review schedules and lesson rounds live on the server and follow the account. The spacing rules stay in one scheduler.
- One top bar: Today, Lessons, Review, Progress, and Admin. Admins manage users, see full answer history, choose lesson visibility, and edit lessons in the browser.
- The AI assigns lessons through the MCP. New lessons stay hidden until assigned. Lessons already on disk, including `anal-sex-vocab`, are visible to everyone.
- Sentence audio is archived before overwrite. Word clips are never overwritten.

## Type of change

- [x] Feature
- [x] Infrastructure
- [x] Docs

---

## Test plan

- [ ] `npm test` — at close-out, 273 passed. `test/retry-before-reveal-smoke.test.js` still expects a signed-out lesson page to return 200 and should be updated to sign in first.
- [ ] `npm run audio:verify` against `data/audio-inventory/20261008T153149.json` (65 files).
- [ ] Signed out: `/` redirects to `/login/`. A lesson folder for a hidden lesson is 404. `/admin/` as a normal user is 403 and returns no script.
- [ ] Sign in, finish a word, sign in on another browser with the same account, and confirm the review state followed.
- [ ] Admin: create a user, reset the password, disable the user, open their answer history, set a lesson to selected users, save a lesson, and cancel the audio dialog without generating audio.
- [ ] `docker compose up` only after `MCP_AUTH_TOKEN` is set in `.env`. An empty token must refuse to start.
- [ ] Confirm `.env` is not in the diff.

## Security notes for the reviewer

Phase 3.5 (initiative-wide thorough review) was not run at close-out. Per-spec reviews found no unresolved Critical or High in the new auth and admin paths. Residuals: no Content-Security-Policy; word mp3s are sign-in gated but not filtered per lesson; the IP throttle trusts the first `X-Forwarded-For` value; sessions last 30 days. Rotate the ElevenLabs key by 2026-10-15 (SEC-OPS-1, from INIT-001).
