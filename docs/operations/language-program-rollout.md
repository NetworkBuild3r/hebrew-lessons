# Language-program rollout

Checklist for backing up the learner database, applying INIT-006 migrations, verifying stage counts, and switching `SRS_SCHEDULER` to FSRS. It follows [INIT-006 §14](../../sdd/initiatives/INIT-006-language-program/INIT-006-language-program-initiative.md). Commands are PowerShell from the repository root locally, and `kubectl exec` on the cluster. Secret **names** only — never paste a value into this file, a command line, or a log.

Provenance: `INIT-006/SPEC-020`.

## Rules on every step

- No `.mp3` is deleted, moved, or overwritten.
- `hebrew.sqlite` is not replaced by a wipe. Migrations add columns and tables beside the existing rows.
- `DATA_DIR` stays outside every served root. Backups stay under `DATA_DIR/backups/`, which is not served.
- `replicas` stays 1. `strategy` stays `Recreate`.
- `MCP_AUTH_TOKEN` stays required. No new secret is introduced.
- `SRS_SCHEDULER` is a plain environment variable (`ladder` or `fsrs`), not a secret.
- Worksheet `index.html` files are not edited.
- Do not flip production `SRS_SCHEDULER` to `fsrs` without written operator approval (GATE REQUEST in the SPEC-020 completion summary).

## Two backup layouts

| Command | Output | What it covers |
| --- | --- | --- |
| `node scripts/backup-db.js` (or `npm run db:backup-sqlite`) | `DATA_DIR/backups/hebrew-<ISO stamp>.sqlite` — one flat file after a WAL checkpoint | SQLite only. Prints path and size. Use for the pre-rollout DB checkpoint in this checklist. |
| `npm run db:backup` (`scripts/db.js`) | `DATA_DIR/backups/hebrew-<stamp>/` with `hebrew.sqlite`, `media/`, and `media/_archive/` | Database plus generated images. Prefer this when you also need media restored together. |

Both live on the data volume. Neither reads or writes lesson `.mp3` files.

## Before you start

Keep a run log at `data/rollout/language-program-run-log.md` (under gitignored `data/`). Record inventory path, backup path and size, and each stage-count block.

Default scheduler until step 5 is `ladder` (unset or empty means ladder). Take before/after migration stage counts under the **same** scheduler.

---

## Steps

### 0 — Backup and audio inventory

**Do**

```powershell
npm run audio:inventory
# Copy the printed inventory path into the run log. Later verifies use --against that file.

node scripts/backup-db.js
# Or: npm run db:backup-sqlite
# Log the printed path and size. Do not log row contents.
```

Optional media bundle (does not replace the flat SQLite backup above):

```powershell
npm run db:backup
```

**Cluster**

```bash
kubectl exec -it deploy/<name> -- npm run audio:inventory
kubectl cp <namespace>/<pod>:/app/data/audio-inventory/<stamp>.json ./data/audio-inventory/pod-<stamp>.json
kubectl exec -it deploy/<name> -- node scripts/backup-db.js
# Copy the printed flat file off the volume if you keep an off-cluster copy.
```

**Rollback:** none required. Keep the dated file. Do not delete recordings.

---

### 1–4 — New image (migrations and additive features)

Covered by earlier INIT-006 specs (FSRS columns, lexicon, routes, MCP/Today/AI panel). `SRS_SCHEDULER` stays `ladder` until step 5.

**Do (before starting the new image)**

```powershell
$env:SRS_SCHEDULER = "ladder"   # or leave unset
node scripts/stage-counts.js
# Save the printed block as "before" in the run log (usernames and counts only).
```

**Do (start the new image)**

Locally: rebuild/restart compose so `npm start` applies pending migrations.  
Cluster: roll the new image (`replicas: 1`, `strategy: Recreate`). Migrations run at boot.

**Do (after migrations)**

```powershell
$env:SRS_SCHEDULER = "ladder"
node scripts/stage-counts.js
# Save as "after". Per-user new/learning/longTerm per direction must match "before".
```

**Rollback**

| Concern | Rollback |
| --- | --- |
| Bad image / routes | Roll the previous image. Ladder data was not rewritten by FSRS backfill; down migrations drop additive columns/tables only when you intentionally run them. |
| Stage counts differ under ladder | Stop. Do not set `fsrs`. Restore the dated backup (last resort; see below). |
| Learner-facing Words / AI panel | Hide the `words` route; Review keeps working on the ladder. Unregister new MCP tools / remove the panel if needed. |

Do **not** flip learner links or wipe `hebrew.sqlite`.

---

### 5 — Switch scheduler to FSRS

**Gate:** operator written approval required before any real environment. This checklist does not flip production.

**Do**

1. Confirm step 1–4 stage counts matched under `ladder`.
2. Set `SRS_SCHEDULER=fsrs` in the environment (compose `.env`, or plain Deployment env — not a Secret).
3. Restart the process / roll the pod so the new value is read at startup.
4. Re-run stage counts under `fsrs`. Totals should still match the ladder before/after block (backfill maps step → stability).

```powershell
$env:SRS_SCHEDULER = "fsrs"
# Restart the server / pod with that env, then:
node scripts/stage-counts.js
```

**Local compose:** set in `.env` (see `.env.example`) or pass through `docker-compose.yml` (`SRS_SCHEDULER`). Recreate the container.

**Cluster:** plain env on the Deployment (see `deploy/k8s-database.md`). Not a `secretKeyRef`.

**Rollback:** set `SRS_SCHEDULER=ladder` and restart. No down migration. Ladder `step` / `streak` kept updating under FSRS, so scheduling resumes on the ladder.

---

### 6 — Flashcard smoke and audio verify

**Do**

```powershell
# Flashcard / review smoke (Chrome), against the running server you just restarted:
node test/chrome-review-smoke.js
# Or the flashcard smoke your branch documents, if separate.

npm run audio:verify -- --against data/audio-inventory/<stamp-from-step-0>.json
# Expect 65 recordings, same relative paths.
```

**Rollback:** if audio verify fails, stop content changes; restore recordings only from a known-good lessons volume backup — never delete live clips to "fix" a mismatch. If review scheduling looks wrong, set `SRS_SCHEDULER=ladder` and restart (step 5 rollback).

---

## Last-resort restore (database)

Stop writers. Restore the flat file from step 0 (or the bundle from `npm run db:backup` if you need media too). Remove leftover `-wal` / `-shm` beside the live file before replacing it. Do not touch the lessons volume. Start again, run `npm run db:check` (expect `ok`), then stage counts and sign-in.

Cluster detail: [deploy/k8s-database.md](../../deploy/k8s-database.md).

---

## Env reference

| Name | Notes |
| --- | --- |
| `SRS_SCHEDULER` | `ladder` (default) or `fsrs`. Plain env. Read once at process start. |
| `MCP_AUTH_TOKEN` | Still required for MCP. Unchanged by this rollout. |
| `DATA_DIR` | Holds `hebrew.sqlite` and `backups/`. |

Day-to-day ladder fallback wording: [accounts-and-admin.md](accounts-and-admin.md#ladder-fallback).

---

## Tests during the rollout

| Disposition | What |
| --- | --- |
| Kept | Every existing `test/*.test.js` and Chrome smoke. Suite ambient default is ladder. |
| New | `test/rollout-scripts.test.js` — temp fixtures only; never the live learner DB for destructive ops. |

Blocking checks for this initiative's SPEC-020: `node --test test/rollout-scripts.test.js`, `npm test`, Chrome smokes when screens changed, `npm run audio:verify`.
