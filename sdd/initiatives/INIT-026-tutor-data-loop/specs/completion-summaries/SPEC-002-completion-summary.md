# SPEC-002 Completion Summary

**Spec:** SPEC-002 — Migration 020: learner zone, taught words, dated corrections, note author role
**Initiative:** INIT-026-tutor-data-loop
**Domain:** data
**Status:** complete
**Provenance:** INIT-026/SPEC-002
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/tutor-data-loop` (branch `tutor-data-loop`)

## Change Produced

| File | Action |
| --- | --- |
| `migrations/020_learner_day_and_tutor.sql` | created |
| `internal/db/users_zone.go` | created (`UserTimeZone`, `SetUserTimeZone`, `UserTimeZones`) |
| `internal/db/users_zone_test.go` | created |
| `internal/db/migrate_020_test.go` | created |
| `internal/db` migrate companions | version lists 1..19 became 1..20; `m20Aside` ignores `taught_words` and the columns 020 adds |

## What shipped

- `users.time_zone` is nullable, 1 to 64 characters when set.
- `learner_mistakes.first_on` and `last_on` are nullable civil dates. Old rows stay null.
- `taught_words` is unique per learner, word, and day. Deleting the learner deletes the rows. Deleting a lexeme that has a taught row is refused.
- `coach_notes.author_role` defaults existing notes to `admin`.
- `author_role` was added with `ADD COLUMN ... DEFAULT 'admin'`, the same shape as migration 016. This SQLite accepts it.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=standard FAIL · domain=data FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-002](2a0a7eac-692f-4a04-913e-bc2ef6d1378f) re-ran the filter and returned PASS on ac-1 through ac-5.

```text
/tmp/go/bin/go test -count=1 -run 'TestMigrate020|TestUserTimeZone' ./internal/db
ok github.com/NetworkBuild3r/hebrew-lessons/internal/db 0.272s
```

## Security Review

Standard, inline. Learner rows cascade on user delete. Lexeme delete is restricted while a taught row exists. No secrets. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose re-ran the test filter | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- No git commit yet. The batch is uncommitted.
- `m20Aside` drops `users` row dumps and the `users` and `coach_notes` schema from older "nothing else changed" checks, because 020 adds columns there. User row counts stay in those checks. `TestMigrate020KeepsRowsAndDefaults` still compares the old user, note, and mistake rows directly.
