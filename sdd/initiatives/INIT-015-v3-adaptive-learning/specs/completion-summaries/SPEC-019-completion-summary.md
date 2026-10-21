# SPEC-019 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-019 (service) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `2fb37d9` on local branch `v3-spec-019`, merged into local `v3` as `2a86aab` with no conflicts. Not pushed. Six new files in `internal/brief` (`doc.go`, `build.go`, `store.go`, and three test files). No migration, no shared-file edit, no existing test edited. It uses the SPEC-007 `lesson_briefs` table.

API: `brief.New(store, model, clk, opts...)`, `Build(ctx, userID, theme)`, `Get(ctx, userID, id)`, `MarkUsed(ctx, userID, id, unitID)`. Errors: `ErrNotFound`, `ErrExpired`, `ErrUsed`, `ErrTheme`, `ErrTooLarge`. Payload is `"v": 1`; the learner id is never serialised. Selection rules are pinned by `limits_test.go`.

## Acceptance criteria

| AC | Result | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestAC1NewWordsFromLowestBandNeverStruggling` |
| ac-2 | PASS | `TestAC2StruggleWordsCarrySkillAndCount` (required_count is 4 for struggling, 3 for regressed) |
| ac-3 | PASS | `TestAC3SameDaySameWordSets`, `TestSelectionIsSeededByDayUserAndSeed` |
| ac-4 | PASS | `TestAC4OneLearnerCannotReadAnothersBrief` |

Orchestrator checks on merged `v3` at `2a86aab`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok, `go test -race ./internal/brief/` ok. Spot checks: ids come from `crypto/rand` (16 bytes) and there is no `math/rand` outside tests; every `SELECT` and `UPDATE` on `lesson_briefs` is scoped by `user_id`.

## Deviations and risks for later specs

- **Coverage target:** `targets.coverage` is 0.95 as the spec says; the staged beginner figure (0.60 / 0.90) is in `tier.min_known_share`. SPEC-020 must decide which it enforces.
- **Import cycle:** `brief` imports `db`, and `db` imports `units`. SPEC-020's `ValidateWithBrief` in `internal/units` cannot import `Brief`. Use an interface or a plain struct, or move the types to a leaf package.
- **Tags:** the lexicon has no tag column, so theme matching is by gloss keywords until `TagSource` is wired.
- **Struggle score** weights (`100*(1-acc) + 10*misses7d + 5*confusionKeys7d + 25 if struggling`) are the worker's choice.
- **Letters gate** is not recorded anywhere; `WithUnlockedSkills` is the hook and defaults to all four skills.
- **Briefs are never pruned**, and a deleted unit makes its brief reusable (FK sets `used_by_unit_id` to NULL).
- Activity names come from the SPEC-015 text, since that package is not on this base. Re-check when SPEC-015 merges.
- Never run against a real database. No personas, timer or independent verifier.

## Gates

None. Do not push `v3`.
