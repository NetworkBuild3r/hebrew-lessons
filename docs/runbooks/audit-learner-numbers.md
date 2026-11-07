# Audit learner numbers

Read-only operator check for ADR-0027 / INIT-032 SPEC-009 and ADR-0028 / INIT-033 SPEC-008. Confirms Home, Progress, and MCP live numbers match an independent recompute (`StateOf`, `PlacementSkillBands`, retention, due, spelling cards, stored zone, history, standing).

## Production run (after deploy)

Run **after** the image that includes INIT-033 SPEC-008 is deployed. The image is distroless (no shell), so run the binary in the `hebrew` container by path. The audit opens the database query-only, so it is safe beside the running server. Never open the database from a laptop.

```sh
POD=$(kubectl -n hebrew get pod -o name | head -n1)
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew audit --learner 1 --db /data/hebrew.sqlite
```

`DATA_DIR` defaults to `/data` in the image, so `--db` can be left off.

The command sets `PRAGMA query_only = ON`. A write while it is open fails. Output is **counts only**: each row has value, source, and match. No word text, no email.

Exit `0` when every row matches. Exit `1` and name the mismatched row otherwise.

Record the production table (counts only) in the initiative completion summary. That production pass is the orchestrator's job after deploy; this runbook does not require a laptop connection to production.

## What it checks

| Row | Meaning |
| --- | --- |
| `known_any_skill` | Words known or solid in at least one skill (ADR-0027 D1) |
| `{skill}.known_solid` | Per-skill known + solid via `StateOf` |
| `{skill}.estimate` / `estimate_source` | Per-skill estimate from `PlacementSkillBands` (D2) |
| `{skill}.due` / `retention` | Due reviews and 30-day share (D5) |
| `retention` / `due` | All-skill totals |
| `spell_cards` | Spelling card count |
| `zone` | Stored `users.time_zone` (`none` when unset; never invents UTC) |
| `history_rule1_days` | Count of `daily_totals` days still on `rule_version = 1`. Expected `0` after `hebrew history rebuild --apply` |
| `history_old_rule_days` | Count of `daily_totals` days stamped below the current `jobs.RuleVersion` (3 under ADR-0030). Expected `0` after rebuild under the current rules |
| `history_last_day` | Last stored `daily_totals.known_any_skill` versus `SnapshotAt(end of that day)`, in the zone the daily job used (the learner's zone, else the app zone from `TZ`) |
| `replay_mismatch` | Live `Snapshot.ReplayMismatch` (stored cards that differ from a replay of `review_log`); mismatches unless `0` |
| `{skill}.verdict` / `{skill}.accuracy14` | Standing versus an independent AttemptTally recompute (ADR-0028 D5) |
| `review_skipped_no_audio` | Due listen cards with no `lexemes.audio_id` (ADR-0028 D6). A count to read (source `count`), not a comparison |

## Local / fixture

```sh
PATH="/tmp/go/bin:$PATH" GOTOOLCHAIN=go1.26.9 \
  go test -count=1 -p 1 -run 'NumbersAgree|Audit' ./internal/app ./cmd/hebrew
```

## History and standing

After a deploy that includes INIT-033 SPEC-004, rebuild history first (`docs/runbooks/rebuild-history.md`: backup, dry run, then `--apply` behind the prod_data_write gate), then audit:

```sh
POD=$(kubectl -n hebrew get pod -o name | head -n1)
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew history rebuild --learner 1
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew history rebuild --learner 1 --apply
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew audit --learner 1
```

The audit must exit `0`: `history_rule1_days = 0`, `history_old_rule_days = 0`, `history_last_day` agreeing with `SnapshotAt`, `replay_mismatch = 0`, and per-direction verdict / accuracy14 rows matching Standing.
