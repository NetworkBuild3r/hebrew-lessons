# SPEC-013 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-013 (service) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `9065ddb` on local branch `v3-spec-013`, merged into local `v3` as `d3ceb88` with no conflicts. Not pushed.

- `internal/learner`: `Model.Word`, `Model.List` (paged, 50 default, 200 maximum, cursor), `Model.Summary`, `ConfusionPairs`, and the pure functions `StateOf`, `Overall`, `Diagnose`, `DominantPair`, `Rules`. Every threshold is a named constant pinned by `thresholds_test.go`.
- `internal/db/learner.go`: the evidence queries. `internal/fsrs`: `Retrievability` and `MissOther`.
- `migrations/008_confusion_other.sql`: rebuilds `confusion_misses` so `kind` also allows `other`. All rows and ids are kept and the index is recreated. Nothing references the table, and a test guards that.
- `internal/evidence` widened (the SPEC-005 ac-4 carry-over): a wrong pick that names another word now writes a `confusion_misses` row, with kind `other` when no letter pair applies, keyed as the two words in code point order. Kind `agreement` is still never written from the recorder.
- `mcpstore.StruggleStats` now goes through `Model.List(state=struggling)` and keeps the MCP output shape.

## State and cause rules

The first rule that applies wins. An Again is an attempt that was not correct.

| # | State | Rule |
| --- | --- | --- |
| 1 | unseen / seen | no outcome and no card: unseen, or seen if exposed |
| 2 | struggling | 3 or more Again in the newest 8 outcomes, or accuracy under 60% over at least 5 (newest 20; exactly 60% is not struggling) |
| 3 | regressed | card with 3 or more successful reviews and recall chance under 0.70 |
| 4 | seen | outcomes but no card yet |
| 5 | learning | card with fewer than 3 successes |
| 6 | solid | known, and stability 21 days or more |
| 7 | known | recall chance 0.85 or more and the newest 3 outcomes right |
| 8 | learning | anything else with a card |

A word's overall state is its weakest skill with evidence. Causes (struggling or regressed words only): confusion (one pair is at least 3 times and over half of the word's confusions in 30 days), decoding (listen known or solid, read struggling or regressed), production (read known or solid, recall struggling or regressed), spelling (recall known or solid, spell struggling or regressed), band slip (5 or more struggling or regressed words of the word's band seen in 7 days).

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestSkillStateTable` (exactly 3 of 8 Again, exactly 60% over 5, 0.85, 0.70, 21 days), `TestEverySkillStateIsReachable`, `TestWordStateFromStoredEvidence`. |
| ac-2 | PASS | `TestEachCauseComesFromItsOwnPatternOnly`, `TestNoPatternProducesAnotherPatternsCause`, `TestNearMissesGiveNoCause`, `TestRulePrecedence`. |
| ac-3 | PASS, narrowly | `TestListFirstPageOver50kAttempts`. See below. |
| ac-4 | PASS | `TestSummaryEstimatedKnownMatchesHandComputedFixture` (479.75 read, 479 listen), `TestEstimateKnown`, `TestBandRankWidthsMatchPlacement`. |

The orchestrator verified on merged `v3` at `d3ceb88`, which also holds SPEC-006's 40-word deck: `gofmt -l .` empty, `templ generate` 0 updates, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok.

## ac-3 timing, as measured

The worker reported 88.7 ms best of 5 and said the machine was loaded by parallel workers. The orchestrator re-ran the test three times on a quiet machine, after every worker had finished:

- First page, `state=struggling`: 84 to 92 ms for the first run, best of 5 at 84 to 86 ms.
- Other filters: 85 to 92 ms, and one reading of 101 ms ("known in listen").
- `Summary` over the same data: 94 to 96 ms.

So the timing is not a loaded-machine artefact. It is under the 100 ms budget, but with little margin, and the test asserts a limit of 200 ms. The query streams a backward index scan over all of the learner's attempts and counts in Go, so cost grows with the learner's attempt count. 50,000 attempts is a deliberately large case, but `List` and `Summary` will sit near 100 ms for it. SPEC-014 (daily snapshots) or SPEC-021 (the MCP tools) may want to read from snapshots or cache for heavy callers.

## Deviations and disclosures

- **Struggling wins over regressed.** The worker followed north-star 4.3 precedence. The spec's "regressed: was known or solid, and is now struggling" branch therefore never fires, because such a word is reported as struggling. Regressed means learned and recall chance under 0.70, and not struggling. SPEC-014 should detect regressions as a known or solid to struggling transition, using `daily_snapshots` and `regression_events`.
- **"Was known"** is approximated as a card with 3 or more successful reviews, since there is no per-word state history yet.
- **Accuracy window** (the spec was silent): the newest 20 outcomes, so the accuracy rule stays distinct from the 3-of-8 rule.
- **Bands:** the lexicon has no band column. A word's band comes from placement answers joined to `placement.Lookup`, so only placed words have one. Others are band -1 and are left out of the band filter and band slip.
- **Struggle callers:** the session mix still reads `db.StruggleStatsAt`, which counts every reviewed word. Switching it would change mix behaviour and belongs to SPEC-015. `db.StruggleStats` stays for existing tests. The `lexemes` list in the MCP `get_context` now holds only struggling words, not every reviewed word.
- **Side effect:** `other` confusion rows also feed the session mix's confusion signal.
- **Existing tests edited:** migration-version assertions 7 to 8 in `db_test.go`, `learning_test.go`, `migrate_006_test.go`, `migrate_007_test.go`. `TestMigrate007AppliesOnSeededDatabaseAndLeavesExistingTablesAlone` now applies 007 alone. `recorder_test.go` replaced the "no confusion for unrelated words" case. `TestStruggleStats` fixtures got 3 or more Again each and a non-struggling control word.
- **Not audited by the orchestrator:** whether the page query is one query or several, and the estimated-known formula beyond its fixture (the worker documented it in code).
- No personas, timer, trace or independent AC verifier.

## Gates

- **`db_migration` is open for 006, 007 and 008.** All three have run only on temp databases. 008 rebuilds a table, and the data in it (confusion counts) is lower value than review history, but the first run on a copy of a real database still needs the operator. Run 006 to 008 together on a copy before any deploy.
- Do not push `v3`: a push publishes an image that may auto-deploy.
