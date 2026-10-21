# SPEC-020 Completion Summary — Infrastructure: Migration and rollout

**Traceability:** INIT-006/SPEC-020
**Status:** complete (orchestrator closed 2026-10-10 — rollout-scripts 6/6; audio:verify 65; deploy GATE REQUEST still open for `SRS_SCHEDULER=fsrs`)
**Domain:** infrastructure
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `scripts/backup-db.js` — WAL checkpoint, flat copy to `DATA_DIR/backups/hebrew-<ISO stamp>.sqlite`, path and size only
- [x] `scripts/stage-counts.js` — per-user `new` / `learning` / `longTerm` per direction (usernames only); read-only; adaptive to pre/post FSRS schema
- [x] `docs/operations/language-program-rollout.md` — INIT-006 §14 steps with per-step rollback; both backup layouts documented
- [x] Edits: `.env.example` (`SRS_SCHEDULER` comment, empty value), `docker-compose.yml` pass-through, `deploy/k8s-database.md`, `deploy/k8s-lessons-volume.md` (plain env, not a Secret)
- [x] Tests: `test/rollout-scripts.test.js` (temp fixtures only)
- [x] Optional package scripts: `db:backup-sqlite`, `stage:counts` (existing `db:backup` unchanged)

## Acceptance Criteria

- [x] ac-1 (REQ-016): on a fixture real-shaped DB, before/after FSRS migration stage counts under `ladder` are equal
- [x] ac-2 (REQ-016): `audio:verify` matches the start-of-work inventory; 65 recordings, same paths
- [x] ac-3 (REQ-016): under `SRS_SCHEDULER=ladder`, grading advances `step` (rollback path)
- [x] ac-4: backup script stdout has no row data; `.env.example` has empty values for secrets

## AC Verification (independent)

Mechanized checks were run in this implementing session. A fresh read-only verifier was **not** dispatched (this run is already a subagent; nesting one was skipped and disclosed). Status stays `in_review` until the orchestrator records an independent all-PASS.
**Eval notes:** [`SPEC-020-eval-report.md`](SPEC-020-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not declare light-tier · gate_actions=`[deploy]` `HALT` · security_review=`standard` · domain=`infrastructure` · autonomy_level=`auto_tests` ⇒ light_tier_eligible=`false` · **checks skipped:** Chrome smokes (no learner UI change in this spec)

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (in-session) | `test/rollout-scripts.test.js` — before/after migration user lines identical under ladder; fsrs totals match ladder after backfill |
| ac-2 | PASS (in-session) | `npm run audio:verify -- --against data/audio-inventory/20261010T225230.json` → 65 recordings, exit 0 |
| ac-3 | PASS (in-session) | Same test file: ladder `applySignals` moves `step` 1 → 2 on a fixture card |
| ac-4 | PASS (in-session) | Backup stdout is path + `N bytes` only; `.env.example` empty secret assignments; `SRS_SCHEDULER=` empty |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-020`
- **Title:** Language-program rollout scripts, runbook, and scheduler env wiring. No production `SRS_SCHEDULER=fsrs` flip.
- **Files changed:** scripts (2), runbook (1), env/compose/k8s docs (4), package.json scripts (2 entries), test (1), this summary + eval + spec status
- **Change status:** proposed (not committed; no commit was requested)
- **Reversibility:** branch for code/docs. The `deploy` gate (scheduler flip on a real environment) is one_way and **not** executed here.

---

## Tests

| Test file | New tests | Result |
| --- | --- | --- |
| `test/rollout-scripts.test.js` | 6 | pass |

**All tests passing:** Yes.

- `node --test test/rollout-scripts.test.js` → 6/6, exit 0
- `SRS_SCHEDULER=ladder npm test` → 846/846, exit 0
- `npm run audio:verify -- --against data/audio-inventory/20261010T225230.json` → 65 recordings, exit 0

**Coverage:** no coverage tool. Fixture tests cover backup output shape, before/after counts, fsrs vs ladder parity after backfill, ladder grade step advance, `.env.example` secrets empty, and live-DB path avoidance.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| --- | --- | --- | --- | --- |
| deploy (`SRS_SCHEDULER=fsrs` on a real env) | one_way (behavior change; rollback = set `ladder` + restart) | **pending** — GATE REQUEST below | tests pass; audio:verify 65; AC mechanized PASS | No live compose/cluster env flipped. No live `hebrew.sqlite` replaced. |

### GATE REQUEST — `deploy` (scheduler flip), step 5 only

```yaml
gate_action: deploy
reversibility_class: one_way
flipped_by_this_spec: none
blast_radius: >
  Setting SRS_SCHEDULER=fsrs on a real local compose or cluster Deployment causes
  every signed-in learner's review scheduling to use FSRS-6 instead of the five-step
  ladder after the next process start. Stage labels stay aligned with the FSRS
  backfill when counts are taken under the same rules; due dates and intervals change.
  MCP_AUTH_TOKEN and other secrets are unchanged. No learner links flip. No database
  wipe. Recordings are untouched.
blocking_checks:
  - name: rollout_scripts_tests  status: pass   # 6/6
  - name: npm_test               status: pass   # 846/846 under ladder
  - name: audio_verify           status: pass   # 65 vs data/audio-inventory/20261010T225230.json
  - name: ac_verified            status: pass   # mechanized in-session; independent verifier pending
  - name: production_flip        status: not done  # intentionally withheld
rollback_plan: >
  Set SRS_SCHEDULER=ladder (or unset) and restart the process/pod. No down migration.
  Ladder step/streak kept updating under FSRS. Last resort: restore the dated
  DATA_DIR/backups/hebrew-<ISO>.sqlite from step 0 (stop writers; drop -wal/-shm first).
confidence: 85
needs_from_brian_in_writing:
  - Approval to set SRS_SCHEDULER=fsrs on the named environment (local compose and/or cluster).
  - Confirmation that step 0 backup path/size and ladder before/after stage counts were
    recorded in the run log for that environment.
  - Confirmation that audio:verify against the pre-rollout inventory still reports 65
    on that environment after the image roll (steps 1–4).
facts_for_the_decision:
  - This session wrote scripts and docs only. It did not change live DATA_DIR/hebrew.sqlite,
    did not set production/compose/cluster SRS_SCHEDULER to fsrs, and did not change learner links.
  - npm run db:backup (media bundle under backups/hebrew-<stamp>/) still exists beside
    node scripts/backup-db.js (flat hebrew-<ISO>.sqlite). The runbook names both.
  - Default Deployment snippet keeps value: ladder until the operator changes it.
```

---

## Security Review

- **Depth:** standard · **method:** in-session review of new scripts and docs (`self_reviewed: true`). No separate security-review subagent.
- **Findings:** Critical 0, High 0, Medium 0, Low 0 unresolved.
- Backup and stage-count scripts print paths, sizes, usernames, and stage integers only. No password hashes, tokens, or answer text.
- `stage-counts.js` opens read-only via `server/db.openReadOnly` and does not migrate.
- `backup-db.js` checkpoints then copies; it does not migrate and does not replace the live file.
- `SRS_SCHEDULER` is documented as plain env in `.env.example`, compose, and both k8s docs — not a Secret. Secret keys in `.env.example` remain empty assignments.
- Backups remain under `DATA_DIR/backups/`, outside served roots (existing DATA_DIR guard).

---

## Files Changed

Provenance is `INIT-006/SPEC-020` for every row below.

| File | Change |
| --- | --- |
| `scripts/backup-db.js` | Created |
| `scripts/stage-counts.js` | Created |
| `docs/operations/language-program-rollout.md` | Created |
| `test/rollout-scripts.test.js` | Created |
| `.env.example` | `SRS_SCHEDULER=` + comment |
| `docker-compose.yml` | `SRS_SCHEDULER` pass-through |
| `deploy/k8s-database.md` | Plain `SRS_SCHEDULER` env; backup layouts |
| `deploy/k8s-lessons-volume.md` | Plain `SRS_SCHEDULER` env |
| `package.json` | `db:backup-sqlite`, `stage:counts` |
| `sdd/.../SPEC-020-infrastructure-rollout.md` | status `in_review`, checkboxes |
| `sdd/.../completion-summaries/SPEC-020-*.md` | This summary and eval report |

Not edited: `worksheets/*/index.html`, any recording, live `data/hebrew.sqlite`, production/cluster env values.

---

## Deviations

- **Persona.** `.claude/agents/principal-devops-engineer/AGENT.md` is not in this repo. Disclosed on the spec. Work followed SPEC-020, INIT-006 §14, and existing deploy docs.
- **Independent verifier / security subagent.** Not nested from this subagent.
- **Chrome smokes.** Not run — no learner-facing screen change in this spec. Blocking suite and audio verify were run.
- **Production step 5.** Not performed (hard gate).
- **Commit / push / PR.** Skipped per dispatch.
- **`db:backup` coexistence.** Flat-file script is additive; media bundle CLI unchanged.

---

## Trace Emission Degradation

Skipped — the orchestrator owns `open_trace` / `emit` / `close_trace` for a dispatched spec. No events were written by this session.

## Follow-ups

- Operator must approve the GATE REQUEST before any real `SRS_SCHEDULER=fsrs` flip.
- Orchestrator closes the spec after independent AC verification.
