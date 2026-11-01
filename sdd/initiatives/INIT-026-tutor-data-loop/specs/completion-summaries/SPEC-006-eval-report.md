# SPEC-006 Eval Report

**Verifier:** orchestrator re-ran the spec filter. Not the implementer.

```text
/tmp/go/bin/go test -count=1 -run 'TestProgress|TestThisWeek|TestLessonsList|TestReviewPair|TestStudyDay|TestFinishedSitting' ./internal/web
ok github.com/NetworkBuild3r/hebrew-lessons/internal/web 2.927s
```

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | Progress uses `studyday.Today` (`progress.go`); filter includes `TestThisWeekAgreesWithGetProgress` |
| ac-2 | PASS | nil resolver uses `AppZone` |
| ac-3 | PASS | `shell.go` slipped window starts from `studyday.Today` |
| ac-4 | PASS | `TestReviewPairAskFollowsTheStudyDay` and `TestReviewPairAskSurvivesRollover` in the green run; cutoff is `studyday.DueCutoff` |
| ac-5 | PASS | no `now().UTC()` in `progress.go` or `shell.go`; `internal/web/studyday.go` deleted |
