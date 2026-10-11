# SPEC-020 Completion Summary — Migration and rollout

**Traceability:** INIT-004/SPEC-020
**Status:** Complete. The checklist, preflight, and tests are done, and the dry run passed with no learner link changed. The `deploy` gate (one learner link per approved lesson) stays open: see the GATE REQUEST below.
**Domain:** infrastructure
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `docs/operations/lesson-plan-rollout.md`: the seven §14 steps, each with a rollback. Also: who approves a link change, the rules on every step, commands that migrate, before step 1 and before step 2 (local and cluster), step 6, the MCP check, three tested rollback commands, the test disposition, and the run-log format.
- [x] `scripts/rollout-preflight.js`:
  - `counts --out` saves the `users`, `lesson_progress`, and `review_items` counts.
  - `check --inventory [--counts] [--ref]` exits 1 when:
    - `audio:verify` fails, or the recording paths differ from the inventory;
    - a tracked `worksheets/*/index.html` differs from git;
    - a legacy plan row is missing, unpublished, or has no snapshot;
    - the Lessons-tab visibility disagrees with the plan;
    - a table count drops.
  - It prints counts, statuses, mode names, and paths. It opens no socket, and it opens the database read-only.
- [x] `test/rollout-preflight.test.js`: 20 tests on temporary fixtures, each with its own git repo, worksheets, recordings, and database. Every preflight run is under a socket guard. The checklist's own rollback commands run verbatim against a fixture.

## Acceptance Criteria

- [x] ac-1: every §14 step has a rollback sentence, and none deletes a recording. The steps this initiative runs do not overwrite recordings.
- [x] ac-2: `npm run audio:verify -- --against data/audio-inventory/20261009T144848.json` exits 0 with 65 recordings, and the paths are the same.
- [x] ac-3: the dry run left all four tracked `worksheets/*/index.html` equal to `HEAD`, and `lesson_progress` stayed at 1.
- [x] ac-4: the test disposition matches §14. Every current test is kept, the new tests sit beside them, and none is retired.

## AC Verification (independent)

Verified by four fresh sub-agents that did not write the work:

- [Verify SPEC-020 acceptance criteria](0ee9c83b-9914-43a0-9c55-98c1b23503a7)
- [Re-verify SPEC-020 acceptance criteria](03d368dd-53e3-4a39-9878-fd32a5632256)
- [Final verify SPEC-020 criteria](e2c77028-2725-4886-ad76-e3e0671bf19e)
- [Final check SPEC-020 tree](62b42096-817e-4c7b-86ab-04949a9ac007)

Every pass was all-PASS. Three of the four reproduced the dry run on their own read-only copies of the database.
Overall: **PASS** · remediation loops: 0. The later passes re-verified after security-driven changes; no AC failed.
**Full eval report:** [`SPEC-020-eval-report.md`](SPEC-020-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not declare a light-tier pass, so the tier is full · gate_actions=`[deploy]` `HALT` · security_review=`standard` `FAIL` · domain=`infrastructure` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | Checklist steps table: seven rows, each with a rollback cell. Test `the checklist gives every section 14 step a rollback, and no rollback deletes a recording`. |
| ac-2 | PASS | `audio:verify OK — 65 recordings accounted for (against data/audio-inventory/20261009T144848.json)`, exit 0. Preflight: `audio: ok inventory lists 65 recordings; 65 on disk; same relative paths`. |
| ac-3 | PASS | `git hash-object` equals `HEAD` and `origin/main` for all four `index.html` files. The dry run on a database copy kept `lesson_progress` at 1 → 1 throughout. Test `a dry run (counts, then check) leaves worksheets, recordings, progress rows, and the schema as they were`. |
| ac-4 | PASS | Disposition table equals §14. `git diff --name-only --diff-filter=D origin/main -- test/` is empty. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-020`
- **Title:** Lesson-plan rollout checklist, read-only rollout preflight, and fixture tests. No learner link changed.
- **Files changed:** 3 new (checklist 214 lines, preflight 590, test 804). 4 edited:
  - `server/db/index.js`: +13
  - `scripts/audio-inventory.js`: +1
  - `docs/operations/accounts-and-admin.md`: one paragraph
  - `.cursor/skills/hebrew-worksheet/SKILL.md`: one sentence

  The SDD records are updated too (spec status, initiative status and one risk row, this summary, and the eval report).
- **Change status:** proposed (not committed; no commit was requested)
- **Reversibility:** branch. Every file reverts in git. No one-way action was taken.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/rollout-preflight.test.js` | 20 | 20 |

**All tests passing:** Yes.
- `node --test test/rollout-preflight.test.js`: 20 pass, 0 fail.
- `npm test`: 573 pass, 0 fail.
- `npm run audio:verify -- --against data/audio-inventory/20261009T144848.json`: 65 recordings, exit 0. Run at the start and after every change.

**Coverage (new code):** no coverage tool is configured. Each failure branch of the preflight has a fixture test:
- audio altered, moved, or added;
- worksheet changed, untracked, or the served copy changed;
- `--ref` set to an older commit, or to an unknown or option-like ref;
- plan row missing or unpublished;
- visibility `selected`, `hidden`, unparseable, or before the import;
- counts lower than saved;
- no git, no database, and every write refusal.

**Dry run (gitignored log `data/rollout/run-log.md`, details in `data/rollout/dry-run-20261009/`):**
- **Inventory first.** At the start, `npm run audio:inventory` wrote `data/audio-inventory/20261009T144848.json` (65).
- **On a temporary read-only copy of `data/hebrew.sqlite`** (deleted afterwards):
  - Counts: 1 / 1 / 0.
  - `check` before the import: FAIL (plans), as expected.
  - Import of migrations 002 and 003 plus the four plans.
  - `check` after the import: PASS. Visibility `everyone` = `everyone` for all three lessons.
  - The checklist's three rollback commands, run verbatim in PowerShell, with both version guards exercised: 4 rows deleted, 003 down, then 002 down.
  - `check` after the rollback: FAIL (plans), as expected. Counts unchanged.
- **MCP over loopback** (repo test): with a token, all 8 legacy tools are listed and `list_lessons` returns 200. Without a token: 401. The checklist's PowerShell lines returned 200, then 401, on Windows PowerShell 5.1.
- **Live `data/hebrew.sqlite`, read-only, `--ref origin/main`:** FAIL (plans) only. It is still at schema version 1. Its sha256 (`00e51c03a425e006`) and mtime did not change across the final runs.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| deploy (one learner link per lesson) | one_way | **pending**: GATE REQUEST below | tests pass; `audio:verify` 65; AC all-PASS; 0 Critical / 0 High | No link changed. `lessonHref` still returns `/worksheets/<slug>/`, and `npm test` fails if it points at the plan screen. |
| db_migration (carried) | revertible (down paths tested) | **not requested here**: named in the GATE REQUEST | same | SPEC-001 and SPEC-006 recorded "Production apply waits for SPEC-020". This spec's gate list names only `deploy`, so 002 and 003 and the import were run only on copies. |

### GATE REQUEST — `deploy` (learner link), no link flipped

```yaml
gate_action: deploy
reversibility_class: one_way
link_changed_by_this_spec: none
blast_radius: >
  For one named lesson, the Today and Lessons links (lessonHref in shared/today-page.js and
  shared/lessons-page.js) would open /shared/plan/?id=<id> instead of /worksheets/<id>/ for every
  signed-in learner. Other lessons, sign-in, review, and progress are unchanged.
blocking_checks:
  - name: tests              status: pass   # rollout-preflight 20/20; npm test 573/573
  - name: audio_verify       status: pass   # 65 recordings vs data/audio-inventory/20261009T144848.json
  - name: ac_verified        status: pass   # four independent passes, all-PASS
  - name: security_findings  status: pass   # 0 Critical / 0 High; 1 Medium open in the plan route (follow-up), gated by the preflight
  - name: coverage           status: n/a    # no coverage tool configured
  - name: preflight_target   status: not yet  # on data/hebrew.sqlite it fails on plans: steps 1-2 were never applied there
rollback_plan: >
  Revert the lessonHref change (both files) and its tripwire-test change, then redeploy.
  The worksheet index.html and its recordings were never edited.
confidence: 80
needs_from_brian_in_writing:
  - The lesson id to flip (none is named, so the default stands: every learner stays on worksheets).
  - Approval to apply rollout steps 1-2 to the target database first (migrations 002/003 run when the
    new image boots or `npm start` runs; then node scripts/import-lesson-plans.js). Not done here.
  - Before step 2, either accept in the run log that GET /api/plans/:id/published reads the plan's
    own visibility (not the Lessons tab) until SPEC-004's route also checks lesson_visibility, or hold step 2.
facts_for_the_decision:
  - The plan screen does not play recordings yet and does not save lesson progress or review answers.
    my-morning has 20 lesson clips and anal-sex-vocab has 24; flipping either would lose audio and progress for it.
    shai-date-indirect-speech has no lesson-folder clips but would also stop saving progress.
  - anal-sex-vocab: an uncommitted catalog edit by other work describes it as "Private vocab lesson —
    hidden unless assigned", but the local Lessons tab and the import sample both say everyone.
    Confirm the intended visibility before step 2.
  - Local heads-up: node server/web.js (running since 2026-10-08) uses data/hebrew.sqlite and does not
    migrate. Starting this branch with npm start would apply 002 and 003 to that file (step 1).
  - SPEC-017's secret_change and deploy gates are separate and still open.
```

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** independent Task sub-agents applying the `sdd-code-security` skill at standard depth, scoped to this spec's files:
  - [Security review SPEC-020 files](09ed491f-57bf-4b9b-925c-4118d3bdaefc)
  - [Security re-review SPEC-020 fixes](688612a9-d170-4ab0-8c94-a0bdfb2a95f5)
  - [Security check SPEC-020 round three](04e0c9bb-da5b-4884-af67-27a43ae8c530)
  - the security part of [Final check SPEC-020 tree](62b42096-817e-4c7b-86ab-04949a9ac007)

  SAST and secret scan: not run, because none is configured. The test scans output for canary secrets.
- **Findings:** Critical: 0 · High: 0 · Medium: 1 (open in the plan route; a follow-up, gated here) · Low: 7 found, all fixed · Info: 6 (5 fixed or documented, 1 accepted)
- **Unresolved Critical/High:** None.

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| MED-1 plan route reads the plan's lesson visibility, not `lesson_visibility` | Medium | API1 (object-level authorization) | `server/api/plan-routes.js:190-215` | Open in the route: follow-up for SPEC-004, recorded in initiative §9. Gated here by the preflight `visibility` check, the checklist's "Before step 2" and in-pod re-check, the post-import rule (plan first, then the Lessons tab), a run-log acceptance line, and warnings in the operator guide and the worksheet skill. |
| LOW-1 secret aliases reached child processes | Low | secrets handling | `scripts/rollout-preflight.js` `childEnv` | Fixed: any `*KEY`/`*TOKEN`/`*SECRET`/`*PASSWORD` name is stripped; tested. |
| LOW-2 down rollback had no backup step, scope, or version guard | Low | data loss | checklist rollback commands | Fixed: back up first, two single-purpose commands with exact-version guards; tested both orders. |
| LOW-3 cluster backup assumed the new folder layout | Low | recovery | checklist "Before step 1" | Fixed: the old image's `backups/hebrew-<stamp>.sqlite`. |
| LOW-4 full production DB copied to the workstation | Low | data exposure | checklist "Before step 1" | Fixed: delete the copy after the counts. |
| LOW-5 token loading could land in shell history | Low | secrets handling | checklist MCP check | Fixed: read from the Secret into a variable; `Remove-Variable`; `$Error.Clear()`; tested on PS 5.1 (200/401). |
| LOW-N1 test ran destructive doc commands with the live checkout as cwd | Low | test safety | `test/rollout-preflight.test.js` | Fixed: runs from a temp dir holding copies of `server/db` and `server/paths.js`, plus a path guard and a no-path-literal check. |
| LOW-R1 post-import restriction rule only in the checklist | Low | authorization drift | checklist, operator guide, skill | Fixed: in the checklist's rules with the order; operator guide and skill sentences; run-log line. |
| INFO-1/2/3, NEW-2, NEW-3 (step 4 order, 003 wording, test order, skill order, image already carries seed recordings) | Info | — | checklist, test, skill | Fixed or documented. |
| NEW-1 some error lines echo absolute paths (child process errors, the served-root error, the operator's own arguments) | Info | — | `scripts/rollout-preflight.js` | Accepted: paths only, no learner data; preflight paths outside the root print as `~/...`. |

- **Positive observations:**
  - The database is opened read-only, and the queries select only counts, status, revision, and a has-snapshot flag.
  - Plan visibility is reduced to known mode names, and JSON parse errors are discarded.
  - `--ref` is validated, and git gets argument arrays with no shell.
  - Inventory and counts files are confined to `data/`, and the counts file is written exclusively (`wx`).
  - The socket guard proves itself before it is relied on.

---

## Files Changed

Provenance is `INIT-004/SPEC-020` for every row below. Several files already carried other uncommitted edits on `lesson-system-build`; these rows describe only this spec's edits.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `docs/operations/lesson-plan-rollout.md` | Created: rollout checklist | `INIT-004/SPEC-020` |
| `scripts/rollout-preflight.js` | Created: `counts` and `check` | `INIT-004/SPEC-020` |
| `test/rollout-preflight.test.js` | Created: 20 fixture tests, socket guard, loopback MCP test | `INIT-004/SPEC-020` |
| `server/db/index.js` | Modified: `openReadOnly(file)` and its export (the only module allowed to require `node:sqlite`, ADR-0002 §1) | `INIT-004/SPEC-020` |
| `scripts/audio-inventory.js` | Modified: exports `inventoryRoots` | `INIT-004/SPEC-020` |
| `docs/operations/accounts-and-admin.md` | Modified: one paragraph in "Share lessons" on the plan route after step 2 | `INIT-004/SPEC-020` |
| `.cursor/skills/hebrew-worksheet/SKILL.md` | Modified: one sentence on `set_lesson_visibility` versus a published plan | `INIT-004/SPEC-020` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-020-infrastructure-migration-rollout.md` | Modified: status `complete`, checkboxes | `INIT-004/SPEC-020` |
| `sdd/initiatives/INIT-004-lesson-system-build/INIT-004-lesson-system-build-initiative.md` | Modified: Status line; one §9 risk row (plan route visibility) | `INIT-004/SPEC-020` |
| `sdd/.../completion-summaries/SPEC-020-eval-report.md` | Created | `INIT-004/SPEC-020` |
| `sdd/.../completion-summaries/SPEC-020-completion-summary.md` | Created: this file | `INIT-004/SPEC-020` |

Gitignored run evidence, not in git:
- `data/audio-inventory/20261009T144848.json`
- `data/rollout/run-log.md`
- `data/rollout/dry-run-20261009/*`
- the inventory files the importer wrote during dry runs

Not edited: `worksheets/*/index.html`, any recording, `shared/today-page.js`, `shared/lessons-page.js`, and the live `data/hebrew.sqlite`. See the first deviation about the 21:02Z write.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | ---------- | ------ | ------------- | ------ |
| Phase 0 to Phase 5 | `trace_open`, `task_step`, `harness_fallback`, `spec_lifecycle`, `trace_complete` | This repo has no `scripts/dev/lib/sdd_trace`. The orchestrator owns the sentinel for a dispatched spec. | No live `.sdd-trace/SPEC-020.jsonl` from this process | The Tier-5 invariants check and the model-dispatch check (`scripts/dev/validate_*.py`) are not in this repo, so neither could run. Disclosed, not a silent skip. |

---

## Deviations from the Spec

- **The live database file was written once during this spec.** My first preflight run opened `data/hebrew.sqlite` read-write, the way `npm run db:check` does. It closed as the last connection, and SQLite folded already-committed write-ahead-log pages into the main file. That is the file's 21:02:28Z mtime. The schema version (1) and the counts (1 / 1 / 0) were the same before and after. The preflight now uses a read-only connection (`server/db` `openReadOnly`), and later runs left the file byte-identical.
- **Steps 1–2 ran on temporary copies, not on `data/hebrew.sqlite`.** Applying 002, 003, and the import to a real database is a `db_migration`, and it is not in this spec's gate list. It is in the GATE REQUEST.
- **Beyond the listed deliverables, each prompted by a review finding:**
  - the `visibility` check, `--ref`, and the name-pattern environment filter in the preflight;
  - `server/db` `openReadOnly`;
  - the link tripwire test and the loopback MCP test;
  - one paragraph in the operator guide and one sentence in the worksheet skill;
  - one initiative risk row.
- **The count check is stricter than the spec's wording.** It fails on any drop, which includes "had rows, now empty". A dry run expects equal numbers, and the checklist says so.
- **Not run:**
  - the Chrome smokes (they need Chrome and a running server, are outside `npm test`, and no screen changed);
  - the live cluster MCP call and every cluster command (no cluster action, by instruction).
- **No commit, push, or PR.** The dispatch said to skip them. The spec's scope says the checklist is "committed at" that path; it is written there, uncommitted.
- **Persona `principal-devops-engineer` has no `AGENT.md` in this repo.** The work followed the spec, §14, and the existing deploy docs. Nothing was manufactured.
- **Pre-existing behavior seen, not changed:**
  - other test files leave temp folders behind;
  - `npm test` rewrites `worksheets/_review/vocab-index.js` and `worksheets/_audio/words/catalog.js` with identical bytes;
  - the importer prints a Node DEP0190 warning.

---

## Harness Adaptation

Cursor (T2): the AC verifiers and security reviews ran as fresh Task sub-agents, not as the implementer. The Task tool has no `readonly` flag, so every prompt forbade edits, and the file times confirm no reviewer edited a repo file. The SDD config names `composer-2.5-fast` for the verifier, but this harness allows a different model only when the user names one, so the sub-agents ran on the inherited model. `harness_fallback` was not written to a trace because the trace library is absent.

---

## Effort Tracking

**Agent session time:** not recorded here (the orchestrator owns the timer for a dispatched spec)
**Spec effort estimate:** 4 hours
