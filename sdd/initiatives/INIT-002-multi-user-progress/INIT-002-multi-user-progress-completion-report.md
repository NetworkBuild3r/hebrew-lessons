# Initiative Completion Report: Multi-user accounts and server-side progress (INIT-002)

**Initiative ID:** INIT-002
**Initiative:** INIT-002-multi-user-progress
**Completion Date:** 2026-10-08
**Status:** COMPLETE (with recorded residuals; see Security and Follow-ups)
**Source research:** RSCH-003

---

## Executive Summary

INIT-002 turns the Hebrew practice site into a signed-in, multi-user app. Accounts live in a local SQLite database. Progress follows the account. Admins create users, see each learner's activity, choose who can see each lesson, and edit lessons in the browser. The AI can author and assign lessons through the MCP. There is no third instructor role.

Eighteen specs are complete. Each acceptance criterion was checked by the orchestrator, not only by the worker that wrote the code. Recorded audio was copied to `C:\Users\BrianNelson\Backups\Hebrew-audio-2026-10-08-1249\` before the work, and `npm run audio:verify` still matches the 65-file inventory.

Nothing in this initiative has been committed. A pull-request body was written as a file only. `gh pr create` was not run.

Agent session time was not recorded, because this repo has no timer.

---

## Spec Summary

Code authored under this initiative is traced as `INIT-002/SPEC-NNN`. Every change is uncommitted, per the standing rule that commits happen only on request.

| Spec | Domain | Title | Status | Independent check |
| ---- | ------ | ----- | ------ | ----------------- |
| SPEC-001 | docs | ADR-0002 | complete | ADR accepted; constraint 1 of ADR-0001 superseded only |
| SPEC-002 | service | Audio preservation guard | complete | archive-before-overwrite; inventory outside the served tree |
| SPEC-003 | data | SQLite schema, migrations, backup | complete | `node:sqlite`, WAL, `DATA_DIR` not under the served tree |
| SPEC-004 | service | Accounts, passwords, sessions, CLI | complete | scrypt, 30-day sessions, last-admin guard, no `--password=` |
| SPEC-005 | service | Server-side progress and metrics | complete | scheduler called, not copied; stages and direction accuracy |
| SPEC-006 | service | Visibility and content filtering | complete | 9/9; fail closed; seed-once `everyone` for lessons already on disk |
| SPEC-007 | service | Authoring service and MCP assignment | complete | 12/12; audio verify 65; unknown usernames fail before write |
| SPEC-008 | api | Sign-in and content gating | complete | 21/21; signed-out HTML → `/login/`; invisible lesson → 404 |
| SPEC-009 | api | Learner progress endpoints | complete | 14/14 with auth; session user only |
| SPEC-010 | api | Admin endpoints | complete | 8/8; events read writes an audit row |
| SPEC-011 | infrastructure | Volume, non-root, MCP auth required | complete | container user `node`; empty `MCP_AUTH_TOKEN` exits 1 |
| SPEC-012 | frontend | Sign-in page and shared top bar | complete | 8/8 + signed-in Chrome smoke |
| SPEC-013 | frontend | Server-backed progress client | complete | 26/26 + Chrome smoke; queue cap 1,000 |
| SPEC-014 | frontend | Today and Lessons | complete | 6/6 + Chrome smoke |
| SPEC-015 | frontend | Progress page | complete | 8/8 + Chrome smoke |
| SPEC-016 | frontend | Admin users and learner activity | complete | 6/6 + Chrome smoke; user on `/admin/` is 403 with no script |
| SPEC-017 | frontend | Admin lesson editor | complete | 8/8 + Chrome smoke; audio verify 65 |
| SPEC-018 | docs | Operator and admin guide | complete | first-admin and `db:backup` on a temporary database, then deleted |
| **Total** | | | **18/18** | |

Workers ran as `cursor-grok-4.5-high-fast`. This session has no persona files, so each spec used a general-purpose worker. That degradation was disclosed at dispatch.

---

## Agent Session Analytics

**Not available.** `scripts/dev/` (timer, trace, rehydrate) is absent from this repo.

| Metric | Value |
| ------ | ----- |
| Total specs | 18 |
| Total agent time | not recorded |
| Wall-clock span | 2026-10-08 (planning and execution the same day) |

---

## What shipped

- Sign-in is required for lessons, audio, review, and admin. The only public page is `/login/`.
- Roles are `user` and `admin`. Admins create accounts. There is no public sign-up.
- Sessions last 30 days from sign-in. There is no idle timeout and no remember-me.
- Review state is server-authoritative. The browser posts signals. The scheduler in `shared/player/review.js` is the only copy of the ladder.
- While signed in, the browser does not read or write `hebrew-review:v1` or `hebrew-worksheet:<slug>`, and it does not delete them. Old browser data is left in place. Accounts start fresh.
- The top bar is היום, שיעורים, חזרה, התקדמות, and ניהול for admins.
- Progress shows words by memory stage and recall accuracy by direction. No streaks or points.
- Existing lessons, including `anal-sex-vocab`, were seeded `everyone` once. A new lesson with no assignment stays `hidden`.
- MCP tools: `list_users`, `upsert_lesson` (optional visibility), `set_lesson_visibility`. No MCP tool reads learner progress.
- Learners are told, on the sign-in page and in the account menu, that admins can see every answer.

---

## Test results at close-out

`npm test` on 2026-10-08: **273 passed, 4 cancelled, exit code 1**.

The failure is `test/retry-before-reveal-smoke.test.js` (INIT-001/SPEC-006). Its `before` hook waits for `GET /worksheets/my-morning/index.html` to return 200. Signed-out lesson HTML now redirects to `/login/`, so the hook reports "Server not reachable" and the four cases are cancelled. The activity behavior itself was not re-checked by this run. The other INIT-001 Chrome smokes were updated to sign in first. This one was not.

One stale assertion was updated in this close-out: `test/review-page.test.js` looked for `id="review-today-slot"` on the old home page. Today renders the due card from `shared/today-page.js` and links to `/review/`. That file's 12 tests pass.

`node:sqlite` prints an experimental warning once per process. That is expected.

Coverage is not wired. The bar in the initiative (a normal-path test and a failure-path test per new export, plus a refusal test per authorization rule) was applied per spec, not as a percentage.

---

## Gates and controls

| Gate | Taken? |
| ---- | ------ |
| Commit / merge / deploy | No |
| `gh pr create` | No |
| Spec dehydration | No. `scripts/dev/rehydrate_specs.py` is absent, and deleting the specs would drop the record. |
| Risk acceptance this initiative | None new. SEC-OPS-1 from INIT-001 still stands until 2026-10-15. |

---

## Security

**Phase 3.5 was not run.** The skill's initiative-wide thorough review was not dispatched during this close-out. This report does not claim a new Critical/High clearance.

Per-spec reviews recorded no unresolved Critical or High in the new account, session, gating, progress, or admin code. Residuals that remain, and were accepted as part of the plan or left as follow-ups:

| Residual | Where it shows up |
| -------- | ----------------- |
| ElevenLabs key was exposed in earlier tool output (SEC-OPS-1, INIT-001) | Rotate the key by 2026-10-15. It is not in git. |
| `MCP_AUTH_TOKEN` is not in `.env` | The next `docker compose up` from a shell without that variable exits 1. Add it to the gitignored `.env`. Do not commit it. |
| No Content-Security-Policy | Out of scope for this initiative. |
| Hash-named word mp3s require sign-in but are not filtered per lesson | A signed-in user who can guess the file name can fetch the clip. |
| IP throttle trusts the first `X-Forwarded-For` hop | Username throttle still applies. |
| Sessions last 30 days with no idle timeout | Operator choice. |
| One replica | SQLite plus the in-memory audio throttle. A second replica is unsupported. |
| `node:sqlite` is experimental | `better-sqlite3` is documented as a fallback and was not added. |
| Oversize request bodies may abort the socket instead of a clean 413 | Tests accept either, and assert no state change. |
| INIT-001 HIGH-1 (`server/lesson-data.js`) | Fixed in that initiative. An independent re-review was recommended there and was not repeated here. |

---

## Deviations recorded during build

- The login page inlines its script and CSS. `/shared/` is gated when signed out, so the page cannot load those files.
- `seedVisibility()` runs from `server/start.js` after migrate, so the one-time seed actually runs.
- A display-name-only admin update writes SQL directly and one audit row, because accounts has no `setDisplayName`.
- Selected visibility writes two audits (`setVisibility` and `setGrants`).
- The editor has no server `modifiedAt`. It compares the worksheet JSON it opened with a fresh GET, warns if they differ, and the last save still wins.
- Cursor's MCP config is HTTP to `127.0.0.1:8787` with `Authorization: Bearer ${env:MCP_AUTH_TOKEN}`. The stdio entry still exists and is no longer what Cursor uses.

---

## Operator follow-ups

1. Put `MCP_AUTH_TOKEN` in `.env` before the next `docker compose up`.
2. Rotate the ElevenLabs key by 2026-10-15.
3. Create the first admin with `npm run user -- create <name> --admin --password-file <path>`. The guide is `docs/operations/accounts-and-admin.md`.
4. Update `test/retry-before-reveal-smoke.test.js` to sign in, the way the other Chrome smokes do, then re-run `npm test`.
5. Commit and open a pull request only when you ask. The draft body is `INIT-002-multi-user-progress-pull-request.md`. The working tree also still holds uncommitted INIT-001 work and the recordings.
