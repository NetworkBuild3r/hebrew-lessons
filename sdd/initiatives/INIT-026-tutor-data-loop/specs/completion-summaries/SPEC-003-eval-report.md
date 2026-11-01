# SPEC-003 Eval Report

**Verifier:** orchestrator re-ran the spec's commands after the implementer's own check. Not the implementer.

```text
/tmp/go/bin/go test -count=1 ./internal/studyday ./cmd/hebrew
ok internal/studyday 0.094s
ok cmd/hebrew 2.203s

/tmp/go/bin/go test -count=1 -run 'TestBrief|TestLoad' ./internal/brief
ok internal/brief 0.314s
```

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestParseZone` inside the studyday package run |
| ac-2 | PASS | `TestResolverFor` |
| ac-3 | PASS | `TestNewResolverNilApp`, `TestNoUTCDefaultInSources` |
| ac-4 | PASS | `TestTodayAndDueCutoffDenver` |
| ac-5 | PASS | `TestBriefUsesLearnerCivilDay` inside the brief filter |
