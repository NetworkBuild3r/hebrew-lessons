# SPEC-020 Eval Report — Infrastructure: Migration and rollout

**Traceability:** INIT-006/SPEC-020
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** evidenced PASS on mechanized checks · independent confirmation not recorded here
**Date:** 2026-10-10

This report records commands run while implementing the spec. It is not an independent all-PASS. The completion summary keeps the spec `in_review` for that reason. The `deploy` gate was not exercised on any real environment.

## Per AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | Fixture DB seeded through migrations 001–003 with ladder-shaped `review_items`. `node scripts/stage-counts.js` under `SRS_SCHEDULER=ladder`, then migration 004 up + backfill, then stage-counts again. Per-user count lines identical. After backfill, fsrs and ladder stage lines also match. |
| ac-2 | PASS (in-session) | Start inventory `data/audio-inventory/20261010T225230.json` (65). End: `npm run audio:verify -- --against data/audio-inventory/20261010T225230.json` → `audio:verify OK — 65 recordings accounted for`, exit 0. |
| ac-3 | PASS (in-session) | Fixture with 004 applied. `progress` loaded with `SRS_SCHEDULER=ladder`. Two `pass` signals on a new word advance `step` from 1 to 2. |
| ac-4 | PASS (in-session) | Backup stdout is two lines (absolute path, `N bytes`). No username, Hebrew gloss, or `hash-not-printed`. `.env.example` has `SRS_SCHEDULER=` empty and secret keys empty. |

## Tests run

- `node --test test/rollout-scripts.test.js` — 6 pass, 0 fail, exit 0
- `$env:SRS_SCHEDULER='ladder'; npm test` — 846 pass, 0 fail, exit 0
- `npm run audio:inventory` (start) — 65 → `20261010T225230.json`
- `npm run audio:verify -- --against data/audio-inventory/20261010T225230.json` — exit 0, 65

Chrome smokes: not run (no UI deliverable in this spec).

## Hard gates observed

- Did not set live/compose/cluster `SRS_SCHEDULER` to `fsrs`.
- Did not replace or wipe live `DATA_DIR/hebrew.sqlite`.
- Did not change learner-facing links.
- Destructive script tests used `os.tmpdir()` fixtures only.

## Remediation

First run: Windows `EPERM` on temp cleanup after an orphaned SQLite handle when `unload()` ran before `db.close()`. Fixed by closing before unload and tolerating brief `EPERM`/`EBUSY` on `rmSync`. `review.find` returns `undefined` (not `null`); assertion updated. No acceptance criterion was weakened.

## Final

Mechanized checks pass. Independent verifier: not run in this session. Operator must still approve the GATE REQUEST before step 5 on any real environment.
