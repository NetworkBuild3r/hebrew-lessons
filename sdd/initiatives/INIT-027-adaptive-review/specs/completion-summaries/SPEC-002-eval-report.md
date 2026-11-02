# SPEC-002 Eval Report

**Verifier:** independent generalPurpose read, model grok-4.7-high. Not the implementer.
**Result:** PASS (ac-1 through ac-7).

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestMigrate021KeepsRowsAndDefaults`: old rows kept, `implied` 0, new tables empty |
| ac-2 | PASS | `TestMigrate021Constraints` rejects bad kind, empty or 65-character key, `correct = 2`, negative latency, `implied = 2` |
| ac-3 | PASS | `ON DELETE CASCADE` on both new tables; `TestMigrate021ForeignKeys` |
| ac-4 | PASS | `TestMigrate021FreshMatchesMigrated` |
| ac-5 | PASS | Clear filter on `confusionPairs`; count 3 without a clear and 1 after one |
| ac-6 | PASS | Accuracy and retention queries use `implied = 0`; `TestImpliedReviewsExcludedFromAccuracy` |
| ac-7 | PASS | Reads filter on `learner_id`; `TestDrillAnswersAndClearsLearnerIsolation` |

Filtered `go test` for `./internal/db` and `./cmd/hebrew` passed. No accuracy or retention count still includes implied rows.
