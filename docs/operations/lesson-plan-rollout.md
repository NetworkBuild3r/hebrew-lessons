# Lesson-plan rollout

Checklist for moving learners from a worksheet to its published plan, one lesson at a time. It copies the seven steps of the INIT-004 initiative, §14, and gives each step a rollback. **The default is that no learner link changes.** You can finish this checklist with every learner still on the worksheets.

Provenance: `INIT-004/SPEC-020`. The plan is §14 of [the INIT-004 initiative](../../sdd/initiatives/INIT-004-lesson-system-build/INIT-004-lesson-system-build-initiative.md); the step order is ADR-0005 §8. Commands are PowerShell from the repository root locally, and `kubectl exec` on the cluster. Secret **names** only. Never paste a value into this file, a command line, or a log.

## Who approves a link change

Brian Nelson, in writing, for each lesson, naming the lesson id (for example `my-morning`). Until that written approval exists, the lesson's learner link stays `/worksheets/<slug>/`. A passing preflight is not approval. An AI or MCP session never changes a learner link. `npm test` fails if either learner link builder points at the plan screen (`test/rollout-preflight.test.js`, "no learner link opens the plan screen yet"), so a flip has to change that test in the same reviewed change as the approval. A lesson that exists only as a published plan, with no worksheet, is not a flip: the Lessons page lists it through `planHref` in `shared/lessons-page.js`, and the same test allows that one builder and nothing else.

## Rules on every step

- No `.mp3` is deleted, moved, or overwritten. A future overwrite still archives first and verifies after.
- Clip paths and URLs do not change.
- No step adds recordings to the image. The image already carries the seed `worksheets/` tree, recordings included, as it does on `origin/main` (`COPY . .`); the live clips stay on the lessons volume.
- `hebrew.sqlite` is not replaced. The plan and media tables sit beside the INIT-002 tables, each with a down path.
- `DATA_DIR` stays outside every served root.
- `replicas` stays 1. `strategy` stays `Recreate`.
- `MCP_AUTH_TOKEN` stays required.
- Nothing is served without sign-in except the login page and its assets.
- Worksheet `index.html` files are not edited. No worksheet file is deleted in this initiative.
- From step 2 on, the Lessons tab alone does not restrict a legacy lesson's plan. Change and publish the plan first, then the Lessons tab, then run the preflight (see [Before step 2](#before-step-2)).

## Commands that migrate the database

Read this before you take a backup.

- `npm start` (`server/start.js`) applies every pending migration at boot. Starting this branch, or rolling out an image built from it, applies migration 002 (plans) and 003 (media and `users.ai_planning`). That is step 1. `npm run web` alone does not migrate.
- `npm run db:backup`, `npm run db:migrate`, and `npm run user -- ...` also migrate first. On a database still at version 1, a backup taken with this branch's code is a backup of the migrated file.
- So take the pre-rollout backup with the code that is running now: in the current pod, before the new image rolls out. Locally, stop the server, then copy `data/hebrew.sqlite` (and `hebrew.sqlite-wal` if one is left beside it).
- `node scripts/rollout-preflight.js` never migrates. It opens the database read-only and runs only `SELECT`.

## Before step 1

Keep a run log at `data/rollout/run-log.md`. `data/` is gitignored, so the log, the inventory, and the counts files stay out of git. Write each path and result into it as you go.

1. **Recordings.** `npm run audio:inventory`. Copy the printed path into the run log. Every later `audio:verify` uses this file (`--against`), not a newer one.
2. **Backup.** Database and media, with the current code (see above).
3. **Row counts.** `node scripts/rollout-preflight.js counts --out data/rollout/counts-before.json`. It saves the counts of `users`, `lesson_progress`, and `review_items`, and nothing about any row. It will not replace an existing file.
4. **Visibility.** `node scripts/rollout-preflight.js check --inventory <that inventory> --counts data/rollout/counts-before.json`. It fails on `plans` here, which is expected. Read the `visibility` lines; see [Before step 2](#before-step-2).

On the cluster, the running image does not have the preflight yet, and `/app/data` is not on a volume, so it is lost when the pod restarts. That image writes its backup as one file, `backups/hebrew-<stamp>.sqlite`, with no media folder:

```bash
kubectl exec -it deploy/<name> -- npm run audio:inventory
kubectl cp <namespace>/<pod>:/app/data/audio-inventory/<stamp>.json ./data/audio-inventory/pod-<stamp>.json
kubectl exec -it deploy/<name> -- npm run db:backup
kubectl cp <namespace>/<pod>:/data/app/backups/hebrew-<stamp>.sqlite ./data/rollout/hebrew-<stamp>/hebrew.sqlite
```

Then read the counts and the visibility from that copy, locally. The local `check` uses an inventory of your checkout, because the pod's inventory names paths inside the pod. That copy holds every account, session hash, and progress row, so the last line deletes it; skip that line only if this copy is also your off-cluster backup.

```powershell
npm run audio:inventory
$env:DATA_DIR = "data/rollout/hebrew-<stamp>"
node scripts/rollout-preflight.js counts --out data/rollout/counts-before.json
node scripts/rollout-preflight.js check --inventory data/audio-inventory/<local stamp>.json --counts data/rollout/counts-before.json --ref <commit the pod runs>
Remove-Item Env:DATA_DIR
Remove-Item -Recurse data/rollout/hebrew-<stamp>
```

After the new image is up, copy the pod inventory back in (`kubectl cp ./data/audio-inventory/pod-<stamp>.json <namespace>/<pod>:/app/data/audio-inventory/pod-<stamp>.json`) and run `npm run audio:verify -- --against /app/data/audio-inventory/pod-<stamp>.json` there.

## Before step 2

The import publishes the three legacy plans with every lesson visible to `everyone`. `GET /api/plans/:id/published` decides who may read a plan from the visibility inside the plan. It does not read the Lessons tab (`lesson_visibility`), which is what the worksheets use. So a lesson that is `hidden` or `selected` in the Lessons tab becomes readable by every signed-in learner as soon as step 2 runs, with no link change.

The preflight compares the two. Before step 2 it compares the Lessons tab with the sample plan the import will publish (`visibility: ok my-morning table=everyone import=everyone`). After step 2 it compares the Lessons tab with the published plan (`... plan=everyone`). A missing Lessons-tab row reads as `hidden`, as it does for the worksheets.

If any `visibility` line says `FAIL`, do not run step 2. Brian decides how to make the two agree: share that lesson with everyone in the Lessons tab, or wait until the plan route also checks `lesson_visibility`.

Check again right before step 2, against the live database. On the cluster the new image (step 1) already has the preflight, so run it in the pod; its `worksheets` line fails there (no git) and its `plans` line fails (no import yet), so read the `visibility` lines:

```bash
kubectl exec -it deploy/<name> -- node scripts/rollout-preflight.js check --inventory /app/data/audio-inventory/pod-<stamp>.json
```

After step 2, the Lessons tab alone no longer limits who can read a legacy lesson's plan. To restrict one, change the plan first: `set_field` with `path` `units/<unit id>/lessons/<lesson id>`, `field` `visibility`, then `publish_plan`. Then change the Lessons tab to match, and run the preflight again; its `visibility` line for that lesson must be `ok`. The other way out is to roll back step 2.

Until the plan route also checks `lesson_visibility`, write in the run log either that Brian accepts this or that step 2 waits for that route change.

## The preflight

```powershell
node scripts/rollout-preflight.js check --inventory data/audio-inventory/<stamp>.json --counts data/rollout/counts-before.json --ref <commit>
```

It prints one line per check and ends with `rollout-preflight: PASS` (exit 0) or `rollout-preflight: FAIL (<checks>)` (exit 1). It fails when:

- `audio:verify` fails against that inventory, or the recordings on disk are not the same relative paths the inventory lists (a missing, moved, or new file all fail);
- a `worksheets/<lesson>/index.html` tracked at `--ref` (default `HEAD`) differs from it, or its copy under a separate `LESSONS_DIR` does, or one of the three legacy lessons has no tracked `index.html`. Pass the commit that is deployed now as `--ref`, so a committed edit is caught too;
- the `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech` plan row is missing, unpublished, or has no snapshot, or the `beginner-unit-1` row is missing;
- a legacy lesson's Lessons-tab visibility differs from its plan's (see [Before step 2](#before-step-2));
- `users`, `lesson_progress`, or `review_items` has fewer rows than the saved counts file. That covers a table that had rows and now has none. Without `--counts` it only prints the counts. On a dry run with no learner activity, expect the same numbers.

It prints counts, statuses, visibility mode names, and paths. It does not print user names, progress or plan JSON, or environment values, and its child processes get no variable whose name ends in `KEY`, `TOKEN`, `SECRET`, or `PASSWORD`. It opens no socket. It writes nothing of its own except the counts file. The database is opened read-only, so `hebrew.sqlite` is not changed; SQLite still updates its shared-memory `-shm` file, and may leave empty `-wal` and `-shm` files beside a copy that had none. The inventory and both counts files must be under `data/`.

It does not check `worksheets/catalog.js` or any worksheet file other than `index.html`. Review `git diff <deployed commit> -- worksheets` for those.

Run it from the repository checkout: the worksheet check compares against git, and the pod has no `.git`. Point `DATA_DIR` at the database you are checking (the local file, or a backup copied out of the pod). In the pod, use `npm run audio:verify -- --against <file>` for the recordings.

Before step 2 the preflight fails on `plans`. That is expected: there is no plan table yet.

## The seven steps

Do them in order. Do not start a step until the one before it passes its check.

| Step | Spec | What changes | What stays up | Check before you go on | Rollback |
|------|------|--------------|---------------|------------------------|----------|
| 1 | SPEC-001, SPEC-003, SPEC-004, SPEC-005 | Plan table, operations, validator, plan routes, plan MCP tools. Arrives when the new image boots (migrations 002 and 003 run then). Routes use the existing session, CSRF, and token checks. | Legacy pages, worksheets, the eight MCP tools, sign-in. | `npm run db:check` prints `ok`. Sign-in works. The [eight MCP tools answer](#the-eight-mcp-tools) a tokened `list_lessons`. Learner links are still `/worksheets/<slug>/`. | Roll the Deployment back to the previous image (locally, check out the previous commit and restart). The previous code does not read the new tables and ignores `users.ai_planning`, so they may stay; to remove them, back up and run [the 003 and then the 002 down command](#rollback-commands) from the new code before you roll back. No snapshot has been served, and no recording is touched. |
| 2 | SPEC-007 | `node scripts/import-lesson-plans.js` inserts the four sample plans. It runs `audio:inventory` before and `audio:verify` after on its own. The three legacy plans are `published` with `migration.legacy: true`; `beginner-unit-1` is a `draft`. Run it only when the preflight's `visibility` lines are all `ok`. | The worksheet player. | The preflight `check` against the pre-rollout inventory and `counts-before.json` passes. `npm run audio:verify -- --against <pre-rollout inventory>` reports 65 recordings. | Back up, then [delete the four plan rows](#rollback-commands); that also stops `GET /api/plans/:id/published` from serving them. Worksheets and recordings were not modified by the import, and the delete does not touch them. |
| 3 | SPEC-013 | Token block appended to `shared/worksheet.css`. Existing selectors keep their names. Ships in the same image as step 1. | Every page. | `npm test` passes (`test/design-tokens.test.js`). The legacy screens look the same; their Chrome smokes still pass. | Revert `shared/worksheet.css` to the previous commit and redeploy; no worksheet or recording is part of that change. |
| 4 | SPEC-006, SPEC-008, SPEC-011, SPEC-017 | Media table, upload pipeline, media routes, pictures under `DATA_DIR/media`, and the backup copy of `media/` and `media/_archive/`. `OPENAI_API_KEY` and its Deployment entry are SPEC-017's own gate. | The static allowlist. Media is not added to it. | `npm run db:backup` lists `hebrew.sqlite`, `media/`, and `media/_archive/`. `GET /api/media/:id` needs a sign-in. | Back up, run [the 003 down command](#rollback-commands) from the new image, then roll the image back; that also stops the plan routes, which ship in the same image. The 003 down drops the `media` table and `users.ai_planning` and keeps the plan rows. PNG files under `DATA_DIR/media/` and `media/_archive/` stay where they are, and nothing on the lessons volume is touched. |
| 5 | SPEC-015, SPEC-016 | Admin plan builder (the Plans tab). The old editor stays for worksheets. | Learner site. | The admin Lessons tab still saves a worksheet, and the MCP `upsert_lesson` tool still writes one. A learner cannot open the builder. | Hide the builder: roll back the image, or remove the Plans tab from `shared/admin/admin-shell.js` and `/shared/admin/plan-builder.js` from the admin route in `shared/app-routes.js`. Worksheets still save through the Lessons tab and `upsert_lesson`. |
| 6 | SPEC-014, then SPEC-020's `deploy` gate | One learner link opens `/shared/plan/?id=<id>`, which fetches `GET /api/plans/:id/published`. The other lessons stay on their worksheets. Only after Brian Nelson's written approval names that lesson id. | Sign-in, the other lessons, review, progress. | See [Step 6](#step-6-one-learner-link). | Point that lesson's link back at `/worksheets/<slug>/` (revert the `lessonHref` and test change) and redeploy. The worksheet file is still there and was never edited. |
| 7 | SPEC-020 | Retire a worksheet, only after its snapshot is what learners open and that screen's Chrome smoke is green. Not done in this initiative. | The other worksheets. | Learners have opened that lesson from the plan screen, and its smoke is green. | Restore the worksheet link. No worksheet file was deleted in earlier steps, and none is deleted here. |

## Step 6: one learner link

Learner lesson links are built in two places: `lessonHref(slug)` in `shared/today-page.js` and in `shared/lessons-page.js`. Both return `/worksheets/<slug>/`. A cutover changes both to return `/shared/plan/?id=<slug>` for the approved id only. Every other slug keeps its worksheet link. The admin editor's view link (`shared/admin/lesson-editor.js`) is not a learner link and stays.

Before you ask for approval, know what the plan screen does today:

- It does not play recordings. A clip button says the recording does not play from this page. `my-morning` has 20 lesson clips and `anal-sex-vocab` has 24.
- It does not save lesson progress or review answers.
- `GET /api/plans/:id/published` requires a session, and `/shared/plan/` is not a public path. Who may read it comes from the plan's own visibility (see [Before step 2](#before-step-2)).

In the same change as the flip:

- Change the "no learner link opens the plan screen yet" test in `test/rollout-preflight.test.js` to allow that one id.
- If the lesson is `my-morning`, update `test/chrome-today-lessons-smoke.js`. It checks that the Continue link opens `/worksheets/my-morning`.
- Do not edit the worksheet's `index.html`. Do not move its recordings.

Check, in order: the preflight passes, with its `visibility` line for that lesson `ok`; `npm test` passes; a signed-in learner sees the lesson on the plan screen; a signed-out request goes to sign-in; the other lessons still open their worksheets; `npm run audio:verify -- --against <pre-rollout inventory>` reports 65; that screen's Chrome smoke passes.

Write the approval and the lesson id into the run log before the change, and the check results after it.

## The eight MCP tools

`list_lessons`, `get_lesson`, `list_words`, `list_activity_types`, `list_users`, `upsert_lesson`, `set_lesson_visibility`, and `generate_lesson_audio` stay registered beside the plan and picture tools.

In the repository: `node --test test/rollout-preflight.test.js test/mcp-auth.test.js test/plan-mcp.test.js`. Over loopback, the first lists all eight with a token, calls `list_lessons` with the token (200, the three legacy lessons), and gets 401 for the same call without it. The second checks a missing or wrong token gets 401. The third checks all eight names stay registered.

Live, without typing the token (it is read from the cluster Secret, so it never reaches your shell history):

```powershell
$token = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String((kubectl get secret hebrew-secrets -n <namespace> -o jsonpath='{.data.MCP_AUTH_TOKEN}')))
$body = '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_lessons","arguments":{}}}'
$headers = @{ Authorization = "Bearer $token"; Accept = 'application/json, text/event-stream' }
(Invoke-WebRequest -Method Post -Uri https://<mcp-host>/mcp -Headers $headers -ContentType 'application/json' -Body $body -UseBasicParsing).StatusCode
try { Invoke-WebRequest -Method Post -Uri https://<mcp-host>/mcp -Headers @{ Accept = 'application/json, text/event-stream' } -ContentType 'application/json' -Body $body -UseBasicParsing | Out-Null } catch { [int]$_.Exception.Response.StatusCode }
Remove-Variable token, headers
$Error.Clear()
```

Expect `200`, then `401`. The last line matters: if the first call fails, PowerShell keeps that request, with its header, in `$Error`.

## Rollback commands

Run from the repository root locally, or as `kubectl exec -it deploy/<name> -- <command>` on the cluster (the image's working directory is `/app`). No INIT-002 table or row is removed; the 003 down drops only the `users.ai_planning` column it added. None of them reads or writes a recording.

Back up first: `npm run db:backup`. On the new code it does not migrate anything, because nothing is pending.

Delete the four imported plan rows (step 2). It removes those four plans, including any edits made after the import, and the plan route stops serving them.

```powershell
node -e "const db=require('./server/db'); db.open(); const r=db.run('DELETE FROM plans WHERE id IN (?, ?, ?, ?)', 'my-morning', 'anal-sex-vocab', 'shai-date-indirect-speech', 'beginner-unit-1'); console.log('plan rows deleted: ' + r.changes); db.close();"
```

Down migrations, newest first. Each runs in one transaction and stops unless the schema is at its own version, so 002 cannot run before 003. Rows in `users`, `sessions`, `review_items`, `lesson_progress`, grants, and `audit_log` are kept. The next boot of code that has these migrations applies them again, so roll the image back after running them.

The 003 down command (step 4, and the first half of step 1). It removes every `media` row (alt text, prompt, approval) and `users.ai_planning`, then schema version 3. PNG files under `DATA_DIR/media/` stay on disk without rows. Plan rows stay.

```powershell
node -e "const db=require('./server/db'); const d=db.open(); const v=d.prepare('SELECT MAX(version) AS v FROM schema_migrations').get().v; if (v === 3) { db.transaction(function () { require('./server/db/migrations/003-media-and-ai-planning').down(d); }); console.log('down 003 applied'); } else { console.log('stopped: schema version is ' + v + ', not 3'); process.exitCode = 1; } db.close();"
```

The 002 down command (the second half of step 1). It removes every `plans` row (imported, personal, and builder plans), then schema version 2.

```powershell
node -e "const db=require('./server/db'); const d=db.open(); const v=d.prepare('SELECT MAX(version) AS v FROM schema_migrations').get().v; if (v === 2) { db.transaction(function () { require('./server/db/migrations/002-plan-store').down(d); }); console.log('down 002 applied'); } else { console.log('stopped: schema version is ' + v + ', not 2'); process.exitCode = 1; } db.close();"
```

Last resort: restore the pre-rollout backup as in [deploy/k8s-database.md](../../deploy/k8s-database.md). That drops every progress row and review answer written after the backup, so use the commands above first. Copy `DATA_DIR/media` aside before a restore. A backup from the old image has no media folder: restore only its `hebrew.sqlite` and leave `DATA_DIR/media` in place.

## Tests during the rollout

Under ADR-0005 Option A nothing in the current suite is dropped. This matches §14:

| Disposition | Files |
|-------------|-------|
| Kept, still run | Every `test/*.test.js` that `npm test` runs (`node --test test/*.test.js`), and the `test/chrome-*-smoke.js` scripts. No test file has been removed relative to `origin/main`. Nine existing test files were edited on this branch to follow a changed page or path, and all are still in place: `test/db.test.js` (backup bundle path), `test/admin-users-view.test.js`, `test/http-security.test.js`, `test/progress-view.test.js`, `test/review-page.test.js`, `test/helpers/signed-in-server.js`, and three smokes (`chrome-production-recall`, `chrome-review`, `chrome-signin`) that other work changed, for example to seed progress through the server. None of those edits is a plan cutover. |
| Added beside them | `test/plan-store.test.js`, `test/plan-validate.test.js`, `test/plan-ops.test.js`, `test/plan-routes.test.js`, `test/plan-mcp.test.js`, `test/import-lesson-plans.test.js`, `test/media-switch-store.test.js`, `test/media-ingest.test.js`, `test/openai-images.test.js`, `test/personal-plan.test.js`, `test/media-routes.test.js`, `test/media-generation-mcp.test.js`, `test/design-tokens.test.js`, `test/plan-render.test.js`, `test/plan-builder.test.js`, `test/plan-builder-media.test.js`, `test/media-backup.test.js`, and `test/rollout-preflight.test.js`. They use a temporary database or fixture tree, the sample JSON, fake image and speech clients, and no outside network. Other work on this branch added `test/spa-shell.test.js`, `test/chrome-navigation-smoke.js`, `test/spikes-not-served.test.js`, and `test/bootstrap-admin.test.js`; they stay too. |
| Updated in the same change as the screen | Only a smoke script whose queried hook or link changes when that screen switches, plus the "no learner link opens the plan screen yet" test. None yet, because no screen has switched. `test/chrome-today-lessons-smoke.js` is the smoke to update if `my-morning` switches. |
| Retired | None in this initiative. The legacy authoring tests stay while `upsert_lesson` stays. |

While a screen is still the legacy player, this rollout does not change its Chrome smoke.

## Run log

One block per run in `data/rollout/run-log.md`:

```text
date:
who:
inventory: data/audio-inventory/<stamp>.json (count)
counts-before: data/rollout/counts-before.json (users / lesson_progress / review_items)
step and result:
preflight: PASS | FAIL (<checks>)
visibility: ok | FAIL (<lesson ids>)
plan route visibility: accepted by Brian Nelson on <date> until the route checks lesson_visibility | step 2 waits | route fixed
audio:verify: <count> recordings, exit <code>
link change: none | <lesson id>, approved in writing by Brian Nelson on <date>
```
