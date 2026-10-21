# SPEC-023 completion summary: Weekly report and the coach automation

- **Status:** complete, with one open operator decision (the automation's credential, below). Worker commit `bcc10b8`, merged into local `v3` as `d4b00ff`. Not pushed. No schema change (the next free migration number is still 012).
- **Worker:** [SPEC-023 worker](b9bfd708-d618-46b2-b453-7af85ea72755), a general-purpose sub-agent in worktree `Hebrew-v3-s023`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Merge:** clean. The worker branched before the `origin/v3` merge and the migration renumber (D9) and did not touch migrations, so nothing collided.

## What shipped

- `internal/jobs/weekly.go`: `Weekly.Run`. One `weekly_reports` row per learner per week with gained and lost words (each lost word with its cause), struggle words that cleared, retention against 85%, minutes studied, and band slips. The summary is three template sentences. Registered on the SPEC-014 scheduler for Monday 04:00 `America/Chicago`. Shares the daily job's lock. `HEBREW_JOBS=off` turns it off.
- `internal/db/weekly.go`: the queries.
- `hebrew jobs weekly [--week YYYY-MM-DD]`: prints counts only. Without `--week` it writes the last finished week.
- `docs/ops/weekly-coach.md`: the Cursor Automation definition (Monday 07:00 Central), how to turn it on and off, limits, failure checks.
- Tests: `internal/jobs/weekly_test.go`, `internal/mcp/coach_doc_test.go`, and `cmd/hebrew/jobs_test.go` (the "unknown job" case used `weekly`, now `monthly`).

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestWeeklyReportNamesGainedAndLostWordsWithCauses` (exact gained words, cleared word, each lost word's cause and day, a recovered word, retention, minutes, band slip, full summary; out-of-week and other-learner rows excluded); `TestWeeklyThroughTheDailyJob` (real daily job end to end) | pass |
| ac-2 | `TestWeeklyRunTwiceLeavesOneRow`, `TestWeeklyReportRowIsUniquePerLearnerAndWeek`, `TestJobsWeeklyCommandWritesTheReport` | pass |
| ac-3 | `TestCoachDocNamesAndClaimsMatchTheServer`, `TestCoachDocRolesMatchTheServer` (live learner call to every tool the doc marks "no"), `TestCoachDocPromptFollowsTheRealGuide`, `TestCoachDocChecksCanFail` (17 broken copies of the page) | pass |

Also tested: the scheduler writes at Monday 04:00 and not at 03:30; the daily lock is honoured; bad CLI input is refused.

## Verified by the orchestrator

On the merged `v3`: host `git status` of the worker tree clean; `gofmt` empty, `go vet` clean, tokens check clean, `go test ./...` passes in all 24 packages. I did not repeat the worker's ten mutations (cause logic, idempotence, gained-words, UTC minutes, baseline, weekly hour, cleared words, three doc mutations). The worker reports that all were caught, and that a first `sed -i` mutation run on the Docker mount was inconsistent, was discarded, and was redone by copy.

## Open decision for the operator: the automation's credential

The spec says the automation uses a learner OAuth token and never the static admin token. That cannot work as written. `get_lesson_brief` and `add_coach_note` are admin-only, and so are `put_unit`, `generate_audio` and `upsert_lexemes`, which the authoring guide needs to publish. A learner-role session can call only `get_progress`, `get_learner_model`, `get_context`, `lookup` and `validate_unit` for its own data. Authorization was not changed. The doc offers three options:

1. An admin-role OAuth session. Works today, broader than the spec wanted. It has the same tool power as the static token, plus a user identity, expiry, revocation, and notes under the user's id.
2. A learner-role session. Cannot work.
3. A server authorization change. Needs a security review. Outside this spec.

The doc says not to turn the automation on until the operator picks 1 or 3. With D5 ("no HITL"), this is the credential that decides what an auto-publishing AI is allowed to do, so it is worth a deliberate choice.

## Choices to review

- `weekStart` is the Monday of the week that just ended. The 04:00 run on Monday the 19th reports the week of the 12th. A non-Monday or unfinished week is refused.
- Weeks are Monday to Sunday in `America/Chicago`, the daily job's zone. Minutes are counted over local days. The SPEC-017 UTC comparison was not reused.
- Snapshots hold counts only, so named gained words come from a baseline in a `job_state` row (`weekly.words.<user>`). A first report names no gains and says so.
- Retention is compared with a fixed 85%, as `get_progress` does. There is no per-learner target.
- Minutes come from `attempts` only, so story reading and flashcard grades that wrote no attempt are not counted.
- `get_progress` returns the summary only, so the coach sees struggle words through `regressions` and `get_learner_model`.
- The prompt tells the model that the brief's `activities` names are not unit types (the SPEC-022 contradiction) and points it at the guide's mapping.
- A report written late uses the newest daily states, not the end of that week. This is disclosed in the payload (`stateDay`).

## Not proven

- `HEBREW_JOBS=off` through `startJobs` end to end (needs a full app). `jobsEnabled` is unit-tested.
- No real model has run the prompt against a real server.
- The Cursor Automations editor was not opened, so its field names and cron time-zone behaviour are unverified.
- Whether an OAuth session survives weekly use beyond its 30-day lifetime.

## For other specs

SPEC-018 shows the latest report. SPEC-025 step 6 turns on the automation, and must put the credential decision first.
