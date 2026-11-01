# SPEC-003 · service: One learner-day rule, used by the lesson brief

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-026
title: "One learner-day rule, used by the lesson brief"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-005, SPEC-006, SPEC-007]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- The app zone is read from `TZ` in `cmd/hebrew/jobs.go:31` (`jobsLocation`, default `America/Chicago` at `:25`).
- `NewDaily` turns a nil zone into UTC (`internal/jobs/daily.go:113`).
- The review cutoff is a Denver-only helper with a TODO for a shared `studyday.DueCutoff` (`internal/web/studyday.go:8`).
- The lesson brief takes its day from UTC (`internal/brief/build.go:364`).

**Scope (in):**
- `internal/studyday` package (embeds `time/tzdata`):
  - `ParseZone(name string) (*time.Location, error)`. Refuses empty, whitespace, `Local`, and more than 64 characters. Otherwise `time.LoadLocation`. `UTC` and `Etc/UTC` are valid when named.
  - `Zone{Name string; Loc *time.Location; Source string}`, where Source is `learner` or `app`.
  - `Resolver`, built from the store and the app zone. `NewResolver` returns an error for a nil app zone. There is no UTC default.
  - `Resolver.For(ctx, userID) (Zone, error)` returns the learner's stored zone, else the app zone. A stored name that no longer loads is an error, not a silent app zone.
  - `Resolver.All(ctx)` returns a zone for every user, for the jobs.
  - `Today(now, z) string`: the calendar day `YYYY-MM-DD` at midnight in `z`.
  - `DueCutoff(now, z) time.Time`: the next 4:00 in `z`, the same rule `studyDayCutoff` has today. It is DST-safe.
- Move `jobsLocation` into `studyday.AppZone(tz string)`. `cmd/hebrew` builds one Resolver and hands it to the web server, the MCP store, and the jobs. The jobs switch in SPEC-004 and the pages in SPEC-006. This spec only wires the value through.
- `internal/brief/build.go` takes its day from `studyday.Today` with the learner's zone instead of `now.UTC()`.

**Scope (out):** Jobs (SPEC-004). Pages and review (SPEC-006). MCP (SPEC-007). Writing a zone (SPEC-007).

## Acceptance Criteria

- [ ] ac-1: `ParseZone` accepts `America/Denver` and `UTC`, and refuses `""`, `"  "`, `Local`, `Not/AZone`, and a 65-character name.
- [ ] ac-2: `For` returns `Source: learner` for a user with a zone and `Source: app` with the app zone's name for one without. A stored invalid name returns an error.
- [ ] ac-3: `NewResolver` with a nil app zone returns an error. A test reads `internal/studyday/*.go` and fails on any `time.UTC` that is not inside a test or a named-zone branch.
- [ ] ac-4: `Today` at 2026-11-01T05:30:00Z in `America/Denver` is `2026-10-31`. `DueCutoff` keeps every case of `TestStudyDayCutoffIsTheNextFourDenver` (`internal/web/review_anki_test.go:610`) when given Denver.
- [ ] ac-5: The brief for a Denver learner built at 2026-11-01T05:30:00Z uses the day `2026-10-31`.

## Deliverables

- [ ] `internal/studyday/studyday.go` and test
- [ ] `cmd/hebrew/jobs.go` and `cmd/hebrew` wiring (`jobsLocation` moves). Keep `cmd/hebrew/jobs_test.go` green.
- [ ] `internal/brief/build.go` and test

## Verification Strategy

- **Claim:** every caller can get one learner's day from one rule, and the rule has no UTC fallback.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/studyday ./cmd/hebrew` and `/tmp/go/bin/go test -count=1 -run 'TestBrief|TestLoad' ./internal/brief`. Unit tests for the pure functions, plus one store-backed test for `For`. A property test is not worth it: the DST table already covers the edges.
- **Pass condition:** ac-1 to ac-5 pass. Planting `if loc == nil { loc = time.UTC }` in `NewResolver` fails ac-3.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-003`
- Closes the `#103 DueCutoff` TODO in `internal/web/studyday.go` once SPEC-006 switches the caller.
