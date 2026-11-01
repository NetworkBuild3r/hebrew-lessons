# SPEC-005 Eval Report

**Verifier:** orchestrator re-ran the spec filters. Not the implementer.

```text
/tmp/go/bin/go test -count=1 -run 'TestTaught|TestRemoveTaught' ./internal/evidence ./internal/db
ok internal/evidence 0.262s
ok internal/db 0.093s

/tmp/go/bin/go test -count=1 -run 'TestRecordMistakes' ./internal/db
ok internal/db 0.095s
```

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestTaughtCreatesNewCards` |
| ac-2 | PASS | `TestTaughtLeavesExistingCardUnchanged`; store uses `INSERT OR IGNORE` and no `UPDATE cards` |
| ac-3 | PASS | `TestTaughtRejectsBadOnAndIds` |
| ac-4 | PASS | `TestRemoveTaughtKeepsReviewed` |
| ac-5 | PASS | `TestRemoveTaughtSkipsForeignCards` |
| ac-6 | PASS | `TestRecordMistakesDayBounds` |
| ac-7 | PASS | `TestTaughtLearnerIsolation` |
