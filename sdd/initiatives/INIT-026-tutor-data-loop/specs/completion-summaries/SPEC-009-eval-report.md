# SPEC-009 Eval Report

**Verifier:** orchestrator re-ran the spec filters. Not the implementer.

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcp 0.980s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore 0.054s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/jobs 0.482s
```

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestStudyDigestSectionsAndClosedLoop` |
| ac-2 | PASS | same test; a same-day right answer does not count as `right_since` |
| ac-3 | PASS | `TestStudyDigestIsolation`; tool is on `learnerOwnDataTools` and not on `learnerWriteTools` |
| ac-4 | PASS | `TestStudyDigestSizeCap` |
| ac-5 | PASS | `TestStudyDigestMinutesMatchesJobs` and `TestActiveMinutes` |
| ac-6 | PASS | `TestTutorResource` |
