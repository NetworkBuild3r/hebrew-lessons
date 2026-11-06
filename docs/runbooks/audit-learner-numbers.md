# Audit learner numbers

Read-only operator check for ADR-0027 / INIT-032 SPEC-009. Confirms Home, Progress, and MCP live numbers match an independent recompute (`StateOf`, `PlacementSkillBands`, retention, due, spelling cards, stored zone).

## Production run (after deploy)

Run **after** the image that includes SPEC-009 is deployed. Do it from **inside the cluster**, against a copy of the production DB (same method used on 2026-11-06: ephemeral debug container that copies the DB). Never open the live write path from a laptop.

```sh
hebrew audit --learner 1 --db /data/hebrew.sqlite
```

Or with `DATA_DIR` set in the pod:

```sh
hebrew audit --learner 1
```

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

## Local / fixture

```sh
PATH="/tmp/go/bin:$PATH" GOTOOLCHAIN=go1.26.9 \
  go test -count=1 -run 'NumbersAgree|Audit' ./internal/app ./internal/web ./cmd/hebrew
```

## History and standing

The rows `history_rule1_days`, `history_last_day`, `replay_mismatch`, per-direction verdict and `accuracy14`, and `review_skipped_no_audio` are added by INIT-033 SPEC-008.
