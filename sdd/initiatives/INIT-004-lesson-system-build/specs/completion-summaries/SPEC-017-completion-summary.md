# SPEC-017 Completion Summary — Media volume, backup, and the generation secret

**Traceability:** INIT-004/SPEC-017
**Status:** Complete (repo files and tests). Cluster secret and Deployment apply remain an operator gate.
**Domain:** infrastructure
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `npm run db:backup` writes `DATA_DIR/backups/hebrew-<stamp>/` with `hebrew.sqlite`, `media/`, and `media/_archive/`.
- [x] Restore notes in `deploy/k8s-database.md` and `docs/operations/accounts-and-admin.md`: stop writers, restore the database and both media trees from the same backup, then check that approved images still resolve.
- [x] Deployment snippet in `deploy/k8s-database.md` adds `OPENAI_API_KEY` as `secretKeyRef` on Secret `hebrew-secrets`. No value. `replicas: 1` and `strategy: Recreate` unchanged. `fsGroup` and `runAsUser` stay `1000`.
- [x] Lessons volume doc states pictures live on `hebrew-data`, and `.mp3` files stay on `hebrew-lessons`.
- [x] `.env.example` has empty `OPENAI_API_KEY=`. `.dockerignore` still excludes `.env`. Dockerfile has no `ENV OPENAI_API_KEY`.
- [x] `test/media-backup.test.js`.

## Acceptance Criteria

- [x] ac-1: the backup set includes `hebrew.sqlite`, `media/`, and `media/_archive/`. A temporary directory test copies those paths. An `.mp3` outside `DATA_DIR` is not required and is not copied or deleted.
- [x] ac-2: the Deployment snippet keeps `replicas: 1` and `Recreate`, and adds `OPENAI_API_KEY` only as a secret reference. The test fails if a line assigns a key value.
- [x] ac-3: `npm run audio:verify` passed (65 recordings). This spec did not write under `LESSONS_DIR`.

## AC Verification (independent)

Verified by a fresh sub-agent that did not author the code: [Verify SPEC-017 criteria](7d7654ea-ef5e-4f39-94ee-cd0b323bce5e).
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-017-eval-report.md`](SPEC-017-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not declare a light-tier pass, so the tier is full · gate_actions=`[secret_change, deploy]` `HALT` · security_review=`required` `FAIL` · domain=`infrastructure` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `node --test test/media-backup.test.js` 6 pass, 0 fail. Copy and CLI listing tests. `server/db/index.js` backup bundle; `scripts/db.js` prints the three names. |
| ac-2 | PASS | Manifest test in the same file. `deploy/k8s-database.md` `secretKeyRef` for `OPENAI_API_KEY`, `replicas: 1`, `type: Recreate`. |
| ac-3 | PASS | Verifier re-ran `npm run audio:verify` → exit 0, 65 recordings. No lesson-tree edits. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-017`
- **Title:** Back up `DATA_DIR/media` with the database, and name `OPENAI_API_KEY` as a cluster secret reference
- **Files changed:** backup implementation, tests, deploy and operator docs, empty env placeholder
- **Change status:** proposed (not committed; cluster apply not done)
- **Reversibility:** the script and docs revert in git. The one-way actions (`secret_change`, `deploy`) have **not** been performed.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/media-backup.test.js` | 6 | 6 |
| `test/db.test.js` | 0 (basename assertion updated for the bundle path) | existing suite |

**All tests passing:** Yes · `node --test test/media-backup.test.js` → 6 pass, 0 fail, exit 0 · `npm test` → 509 pass, 0 fail, exit 0 · `npm run audio:verify` → 65 recordings, exit 0 · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus `npm test`.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| secret_change | one_way | **pending** — GATE REQUEST below | repo tests pass; no key value written | Agent did not create or rotate a cluster secret and did not print a key. |
| deploy | one_way | **pending** — GATE REQUEST below | same | Agent did not apply or roll a live Deployment. `replicas` stays 1. `strategy` stays `Recreate`. |

### GATE REQUEST — `secret_change` and `deploy`

```yaml
gate_action: [secret_change, deploy]
reversibility_class: one_way
blast_radius: >
  The operator adds key OPENAI_API_KEY to the existing Secret hebrew-secrets
  (do not replace that Secret; other keys must stay). Then the operator applies
  the Deployment env change from deploy/k8s-database.md so the single hebrew
  replica, strategy Recreate, mounts the same hebrew-data PVC. Media files
  already on the volume stay there. No second replica. No image rebuild is
  required for the secret reference itself.
blocking_checks:
  - name: tests              status: pass   # media-backup 6/6; npm test 509 pass / 0 fail
  - name: audio_verify       status: pass   # 65 recordings
  - name: coverage           status: n/a    # no coverage tool; spec bar is the test file plus npm test
  - name: ac_verified        status: pass   # independent verifier all-PASS
  - name: security_findings  status: pass   # 0 Critical / 0 High / 0 Medium / 0 Low
rollback_plan: >
  Point the Deployment back at the previous manifest (drop the OPENAI_API_KEY
  env entry). The previous backup script remains in git. Media files already
  written stay on hebrew-data. Do not delete /data/worksheets.
confidence: 80
operator_steps:
  - Add key OPENAI_API_KEY to Secret hebrew-secrets in the cluster. Do not paste the value into git, docs, or chat. Do not replace the Secret.
  - Apply the Deployment snippet in deploy/k8s-database.md (secretKeyRef only). Keep replicas 1 and strategy Recreate. Do not change fsGroup or runAsUser.
  - Confirm the pod becomes Ready. Do not print the key.
what_is_already_done: >
  Repo script, tests, and docs. npm run db:backup copies hebrew.sqlite, media/,
  and media/_archive/ under DATA_DIR. The manifest text names the secret and
  contains no value.
```

SPEC-020 can proceed on docs. Generation-dependent lesson cutover still waits until this gate is done if those lessons need new pictures.

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** `security-review` sub-agent [Security review SPEC-017](0e4302d6-8024-4aab-9738-a5543b1d4eb5), scoped to this spec's files. SAST and secret scan: not run — none configured. The spec's own test scans the files it tracks for a non-empty `OPENAI_API_KEY` assignment.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | no findings |

- **Positive observations:** The manifest uses `secretKeyRef` only. `.env.example` is empty. `.dockerignore` excludes `.env`. The backup refuses a media link that resolves outside `DATA_DIR/media` and does not read lesson recordings. Compose does not set `OPENAI_API_KEY` to an empty override.

Residuals noted by the reviewer, below the reporting threshold: a `DATA_DIR/media` symlink that still lands inside `DATA_DIR` would be copied; backups sit on the same PVC until the operator copies them off.

---

## Files Changed

Provenance is `INIT-004/SPEC-017` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/db/index.js` | Modified — backup bundle copies `media/` and `media/_archive/`; refuses links outside that tree | `INIT-004/SPEC-017` |
| `scripts/db.js` | Modified — prints the backup directory and the three relative names | `INIT-004/SPEC-017` |
| `test/media-backup.test.js` | Created — copy, CLI list, link refusal, manifest, empty-key scan | `INIT-004/SPEC-017` |
| `test/db.test.js` | Modified — integrity test expects `hebrew-<stamp>/hebrew.sqlite` | `INIT-004/SPEC-017` |
| `deploy/k8s-database.md` | Modified — `secretKeyRef`, media paths on `hebrew-data`, backup and restore | `INIT-004/SPEC-017` |
| `deploy/k8s-lessons-volume.md` | Modified — media is not on this volume; recordings stay | `INIT-004/SPEC-017` |
| `docs/operations/accounts-and-admin.md` | Modified — backup set and restore; `OPENAI_API_KEY` name only | `INIT-004/SPEC-017` |
| `.env.example` | Modified — `OPENAI_API_KEY=` with an empty value | `INIT-004/SPEC-017` |
| `.dockerignore` | Modified — comment; `.env` exclusion unchanged | `INIT-004/SPEC-017` |
| `docker-compose.yml` | Modified — comment only; no secret value and no empty override | `INIT-004/SPEC-017` |
| `sdd/.../SPEC-017-infrastructure-media-volume-and-key.md` | Modified — status `complete`, AC checkboxes | `INIT-004/SPEC-017` |

Some of these paths already had other uncommitted edits on `lesson-system-build` before this spec. This table describes the SPEC-017 edits only.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 / Phase 5 | `trace_open`, `trace_complete`, `task_step`, `harness_fallback`, `spec_lifecycle` | This repo has no `scripts/dev/lib/sdd_trace`. This session was orchestrator-dispatched, so the sentinel belongs to the orchestrator. No emit call was possible here. | No live `.sdd-trace/SPEC-017.jsonl` from this process | A Tier-5 check in this repo cannot read a trace file. Disclosed; not a silent skip. Gate events belong to the orchestrator when it records the GATE REQUEST. |

---

## Deviations from the Spec

- Persona `principal-devops-engineer` has no `AGENT.md` in this repo (the spec and the initiative already say so). Work followed the spec and the existing deploy docs. Nothing was manufactured.
- No commit, push, or PR (dispatch instruction).
- `secret_change` and `deploy` were not performed. The repo change is complete; the live apply is the GATE REQUEST above.
- Timer start/complete was not run here. The orchestrator owns the timer for a dispatched spec.

---

## Harness Adaptation

Cursor. AC verification and the security review ran as Task sub-agents (T2): fresh agents, not the author. `harness_fallback` was not written to a trace file because the trace library is absent (see above).

---

## Effort Tracking

**Agent session time:** not recorded here (orchestrator owns the timer)
**Spec effort estimate:** 4 hours
