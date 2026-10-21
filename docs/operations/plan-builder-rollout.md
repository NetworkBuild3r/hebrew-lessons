# Plan-builder rollout

Checklist for backing up the learner database, applying migration `006` (the plan revision log and field marks), and checking the AI-first plan builder after the new code starts. It follows [INIT-008 §14](../../sdd/initiatives/INIT-008-ai-first-plan-builder/INIT-008-ai-first-plan-builder-initiative.md) and has the same shape as [language-program-rollout.md](language-program-rollout.md). Commands are PowerShell from the repository root locally, and `kubectl exec` on the cluster. Secret **names** only: never paste a value into this file, a command line, or a log.

Provenance: `INIT-008/SPEC-017`. Decisions: [ADR-0011](../decisions/ADR-0011-ai-first-plan-builder.md). What the screens do: [accounts-and-admin.md, Plan builder](accounts-and-admin.md#plan-builder).

## Two statements first

- **The operator's no-human-in-the-loop direction applies to authoring.** The AI builds and may publish plans through the MCP with no approval step. Hard validation and a pending picture on a visible lesson still refuse a publish. Picture approval, marking a lexicon form `verified`, and the AI-planning switch stay a person's acts. Nothing in this runbook adds an approval step.
- **The `db_migration` gate was waived by the operator on 2026-10-12** (a local-only project with no other users and no sign-offs of any kind). There is no sign-off in this checklist. A backup first is still good practice, because it is cheap, so step 0 takes one.

## What this rollout changes

- Migration `006` (`server/db/migrations/006-plan-revisions.js`) adds two tables: `plan_revisions` (one row per plan write, with a gzipped copy of the document) and `plan_field_marks` (the last writer of each field). It is additive. `plans`, `audit_log`, and every other table are not touched, and no `plans` row or snapshot changes.
- `npm start` (`server/start.js`, the container command) applies every pending migration at boot. So the migration runs when the new code starts, and not before. The operator starts it; no spec does.
- Plans that predate the log have no history. The first write after the migration adds a `baseline` row (writer `system`) holding the document as it was, then the write's own row.
- Learners keep reading the last published snapshot throughout.

## Rules on every step

- No `.mp3` is deleted, moved, or overwritten. This rollout generates no audio.
- `hebrew.sqlite` is not replaced by a wipe. The migration adds tables beside the existing rows.
- `DATA_DIR` stays outside every served root. Backups stay under `DATA_DIR/backups/`, which is not served.
- `replicas` stays 1. `strategy` stays `Recreate`.
- `MCP_AUTH_TOKEN` stays required. No new secret and no new environment variable.
- `SRS_SCHEDULER` is not touched. This rollout does not flip it.
- Worksheet `index.html` files are not edited.

## Before you start

Keep a run log at `data/rollout/plan-builder-run-log.md` (under gitignored `data/`). Record the inventory path, the backup path and size, and the counts from steps 1 and 3. Write no row contents.

`node scripts/backup-db.js` does not migrate: it opens the file, checkpoints the log, and copies. So you can take the backup with this branch's code before the new container starts. `npm run db:backup`, `npm run db:migrate`, and `npm start` do migrate; do not use them for step 0.

---

## Steps

### 0 — Back up and take the audio inventory

**Do**

```powershell
npm run audio:inventory
# Copy the printed inventory path into the run log. Later verifies use --against that file.

node scripts/backup-db.js
# Prints the dated path under DATA_DIR/backups/ and its size. Log both. Do not log row contents.
```

For the media bundle as well (the flat file above is the database only):

```powershell
npm run db:backup
# This migrates first. Use it only if you want the media trees in the same backup, and only after step 2.
```

**Cluster**

```bash
kubectl exec -it deploy/<name> -- npm run audio:inventory
kubectl cp <namespace>/<pod>:/app/data/audio-inventory/<stamp>.json ./data/audio-inventory/pod-<stamp>.json
kubectl exec -it deploy/<name> -- node scripts/backup-db.js
# Copy the printed flat file off the volume if you keep an off-cluster copy.
```

**Rollback:** none needed. Keep the dated file. Do not delete recordings.

---

### 1 — Save row counts

**Do**

```powershell
node scripts/rollout-preflight.js counts --out data/rollout/counts-before-plan-builder.json
```

It saves the row counts of `users`, `lesson_progress`, and `review_items` and nothing about any row. It will not replace an existing file. It opens the database read-only and never migrates.

**Rollback:** none needed.

---

### 2 — Restart so the new code applies migration 006

The operator does this step, outside any spec.

**Do (local compose)**

```powershell
docker compose restart hebrew
```

The container runs `node server/start.js`, which applies migration `006` before it listens. `docker compose logs --tail 40 hebrew` shows the start; there is no separate migration line, so step 3 is how you confirm it.

**Cluster:** roll the new image (`replicas: 1`, `strategy: Recreate`). Migrations run at boot.

**Rollback:** see [Rollback](#rollback).

---

### 3 — Verify the migration

**Do**

With the server running or stopped, read the file read-only. This prints version numbers and counts only:

```powershell
node -e "const { DatabaseSync } = require('node:sqlite'); const d = new DatabaseSync('data/hebrew.sqlite', { readOnly: true }); console.log('migrations: ' + d.prepare('SELECT version FROM schema_migrations ORDER BY version').all().map((r) => r.version).join(',')); for (const t of ['plans', 'plan_revisions', 'plan_field_marks']) console.log(t + ': ' + d.prepare('SELECT COUNT(*) AS n FROM ' + t).get().n); d.close();"
```

Expect `migrations: 1,2,3,4,5,6`, `plan_revisions: 0`, and `plan_field_marks: 0`. Expect the `plans` count to equal what it was before the restart (the migration does not touch it). Then:

```powershell
node scripts/rollout-preflight.js check --inventory data/audio-inventory/<stamp-from-step-0>.json --counts data/rollout/counts-before-plan-builder.json
```

Expect it to pass: audio unchanged, the plan rows present, and no table with fewer rows than the saved counts. If a line fails, read it; the check is read-only. `npm run db:check` (expect `ok`) is also safe now, because the migration has run.

**Rollback:** see [Rollback](#rollback).

---

### 4 — Smoke check

**Do**

1. Sign in as an admin. Open **Admin**, then the **Plans** tab. Every imported plan is listed, and each shows its status and revision.
2. Open `beginner-unit-1`, the imported sample. Editing never changes what learners read; only a publish does. In the lesson form, add one character to the lesson **title** and press **Save field**.
3. Open **History**. Expect a `system` baseline row (the plan as it was) and your edit as a `human` row with your account name.
4. Use **Restore this value** on the title from the baseline row and confirm. Expect a new `restore` row. The plan's content is back to what it was; the revision number is two higher.
5. Check **Issues** shows the plan's warnings and no error you did not expect.

Do not press **Publish** during this check. The plan's content ends as it was; only its revision number and its status (a published plan becomes a draft again until you publish) change.

**Rollback:** if a screen fails, see [Rollback](#rollback). A failed smoke check is not a reason to restore the backup.

---

### 5 — Audio verify

**Do**

```powershell
npm run audio:verify -- --against data/audio-inventory/<stamp-from-step-0>.json
# Expect the same recordings and the same relative paths as the inventory.
```

**Rollback:** if audio verify fails, stop content changes. Restore recordings only from a known-good lessons volume backup; never delete live clips to "fix" a mismatch. This rollout does not touch them, so a mismatch has another cause.

---

## Rollback

| Concern | Rollback |
| --- | --- |
| The new code or a screen misbehaves | Roll the previous image back (locally, check out the previous commit and `docker compose restart hebrew`). The two new tables stay in the file and the older code ignores them. Learners keep reading published snapshots throughout. |
| You want the two tables gone | With the server stopped, run `down` from migration 006 deliberately. It drops `plan_field_marks`, `plan_revisions`, their indexes, and the version-6 row, and never names `plans` or `audit_log`. Plan history is lost. |
| The file itself is damaged | Last resort: restore the backup (below). |

```powershell
# server stopped
node -e "const { DatabaseSync } = require('node:sqlite'); const db = new DatabaseSync('data/hebrew.sqlite'); require('./server/db/migrations/006-plan-revisions').down(db); db.close();"
```

## Last-resort restore (database)

Stop writers. Restore the flat file from step 0. Remove leftover `-wal` and `-shm` beside the live file before replacing it. Do not touch the lessons volume. Start again, run `npm run db:check` (expect `ok`), and sign in. Anything written after the backup (plan edits, progress, accounts) is lost, so use this only when the file is damaged. Cluster detail: [deploy/k8s-database.md](../../deploy/k8s-database.md).

---

## Env reference

| Name | Notes |
| --- | --- |
| `MCP_AUTH_TOKEN` | Still required for MCP. Unchanged by this rollout. |
| `DATA_DIR` | Holds `hebrew.sqlite` and `backups/`. |
| `SRS_SCHEDULER` | Not touched. |

## Tests during the rollout

| Disposition | What |
| --- | --- |
| Kept | Every existing `test/*.test.js` and Chrome smoke. |
| New | `test/plan-revisions-migration.test.js` (migration 006 up and down on a temporary database), and `test/plan-ai-first-e2e.test.js` (the whole loop on a temporary database). Neither opens the real database. |

Blocking checks for INIT-008/SPEC-017: the searches in the spec's verification strategy. For the whole initiative: `npm test`, every `test/chrome-*-smoke.js`, and `npm run audio:verify`.
