# SPEC-003 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-003 (data) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `0c891f1` on local branch `v3-spec-003` (worktree `Hebrew-v3-s003`), cut from `v3` at 97192b2. 41 files, +1016 −253. Not pushed. Not yet merged into `v3`.

- `migrations/006_evidence.sql`, new, 89 lines. Adds `attempts` and `exposures`, and rebuilds `cards` with `skill` in place of `dir` (`he_en` → `read`, `en_he` → `recall`). Card ids and the unique key are kept. Every column has an inline comment.
- `internal/fsrs`, `internal/db`, `internal/session`, `internal/placement`, `internal/mcp`, `internal/mcpstore`, `internal/web`: `Dir` became `Skill`. `SkillListen` and `SkillSpell` constants added. `pages.templ` and its generated file now emit `read` and `recall`.
- `internal/db/migrate_006_test.go`, new, about 668 lines.
- Existing tests changed only for the rename and for the migration count (5 → 6).

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMigrate006KeepsCardsIdsAndReviewHistory`: 6 cards with an id gap and 13 `review_log` rows keep their ids, count and content. |
| ac-2 | PASS | `TestMigrate006AttemptsRejectUnknownSkillOrSource`, `...ExposuresRejectUnknownContext`. |
| ac-3 | PASS | Orchestrator re-ran `go vet ./...`, `gofmt -l .` and `go test -count=1 ./...` in Docker `golang:1.26`. All packages ok, gofmt empty. |
| ac-4 | PASS | `TestMigrate006AtomicWhenCopyFails`, `...AtomicWhenFailureComesAfterTheDrop`, `...RunnerRollsBackDDL`. |

The orchestrator verified ac-3 itself. It also confirmed that no `he_en` or `en_he` remains in non-test Go code, templates, skills or scripts. The only remaining mentions are the old migrations, `006`, and ADR-0018 prose. The ac-1, ac-2 and ac-4 results rest on the test names and the implementer's description. The orchestrator ran those tests as part of the full suite but did not read each test body.

## Deviations and disclosures

- **Rebuild pattern differs from the spec.** The runner holds `foreign_keys=on`, and SQLite ignores that pragma inside a transaction. A plain `DROP TABLE cards` therefore cascades into `review_log`. The migration copies `review_log` to a TEMP table first and restores it after the rename. The implementer confirmed by mutation that without the restore all 13 rows were lost.
- **`PRAGMA foreign_key_check` cannot fail the migration.** The runner discards its rows. The row-by-row foreign key check on the `review_log` restore insert is the real guard.
- **MCP contract break.** The `get_context` and `get_learner_words` card field `dir` is now `skill`, with `read` or `recall`. No consumer was found in this repo outside Go. External agents that read `dir` would break. SPEC-021 should decide whether to keep an alias.
- **HTML change.** `data-kind` now carries `read` or `recall`. No CSS or JS selector uses the old values.
- **Cursor trailer.** The commit message carries a `Co-authored-by: Cursor` trailer added by the tool.
- **No personas, timer or trace** (as in SPEC-001). No independent AC verifier sub-agent.

## Gates

- **`db_migration`: open.** The migration has run only against temp databases built from embedded migrations. It has not run against a copy of a real database file. The gate requires that first run, with the operator told. Do it before any deploy of a build containing `006`. It is one-way: roll back by restoring the SQLite file.
- **Security review (required, standard depth):** not run as a separate review. The orchestrator read the SQL for integrity and scope issues and found none: the foreign keys are `RESTRICT` or `CASCADE` as intended, the CHECK vocabularies are closed, and `attempts.response` is learner input. The security criterion on `attempts.response` (escape on output, learner scope only) is for SPEC-021 and is not ticked here.
- **Do not push.** Pushing `v3` publishes an image that may auto-deploy, and this branch carries a one-way card-table rebuild.
