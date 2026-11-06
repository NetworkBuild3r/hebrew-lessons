# Back-fill spelling from past typed answers

One-time operator command for ADR-0027 D3 / INIT-032 SPEC-008. Past correct and near-miss `type_it` answers on recall become spelling credit the same way new answers do after SPEC-004.

The product letters gate is currently OpenGate (no code decides the letters check yet), so this back-fill treats the gate as open for past answers.

## Do not run `--apply` until the `prod_data_write` gate is approved

This command can write learner rows. Dry run is safe. `--apply` is the gated production write.

The cluster Job that would run this against the data PVC is **not** part of this change; it waits for the gate and lives under `home_k3` when approved.

## Steps

### 1. Dry run (counts only)

```sh
hebrew backfill-spell --learner 1 --db /data/hebrew.sqlite
```

Or, in-cluster with `DATA_DIR` set:

```sh
hebrew backfill-spell --learner 1
```

Output is counts only: `lexemes`, `cards_to_create`, `reviews_to_write`. No word text, no email.

Record the counts before apply.

### 2. Backup first

```sh
hebrew backup --to /backups/hebrew-before-backfill-spell.sqlite
```

Record `bytes=` and `sha256=` from the command output. Keep that file until after you have verified the apply.

### 3. Apply (gate required)

Only after backup and gate approval:

```sh
hebrew backfill-spell --learner 1 --apply --backup-taken --db /data/hebrew.sqlite
```

`--apply` without `--backup-taken` exits non-zero and writes nothing.

A second `--apply --backup-taken` is a no-op: spell attempts already stored at the same `created_at` are skipped.

### 4. Verify

Compare dry-run `cards_to_create` with the spelling card count from `hebrew audit --learner 1` (SPEC-009) before and after: after apply, spelling cards should equal prior spelling cards plus the dry-run `cards_to_create`.

Idempotency check: run the dry run again; `reviews_to_write` and `cards_to_create` should be `0`.

## What it writes

For each eligible past answer (in order):

- A `spell` attempt with the original `created_at` (same activity, source, prompt, response, latency).
- When no spelling card exists and the letters gate is open: create the card, then an implied review (Good for exact, Hard for a one-letter near miss).
- Wrong and don't-know answers are skipped.

`Recorder.Implied` stays Good-only; this path uses the spelling helper rule from `internal/evidence/spell.go`.
