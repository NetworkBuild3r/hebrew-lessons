# INIT-015 Results Report: v3 Adaptive Learning

**TL;DR.** 25 of 26 specs are built and verified; the 26th, the production cutover, is a runbook the operator still has to run. The app now places a learner, models each skill per word, picks activities from the model, and lets an AI coach write lessons that the server checks and publishes on its own. 109 of 116 acceptance criteria are met, 4 partial, 3 open. The security review found nothing Critical or High and fixed four smaller issues, but it covered only part of the code. **Confidence: 0.65.** The score is capped because two acceptance criteria are unresolved (the staging smoke and the rehearsed rollback) and the security review was partial.

## Executive Summary

INIT-015 replaces the fixed lesson list with a loop: placement, evidence, learner model, activities, brief, lesson. Everything runs in one Go binary on one SQLite file. The work is 60 commits (396 files, +82,251 and -7,973 lines) against the deployed baseline, 26 specs, and 11 migration files in all (006 and 008 to 011 are new). It was pushed to `origin/v3` on 2026-10-19. It has not been cut over: production has not been backed up, migrated or smoke-tested with v3.

## What the Process Caught

- **SPEC-018 (mutation check):** a 90-day chart drew 89 days. The tests passed until a mutation exposed that the window started 89 days back while the job writes yesterday.
- **SPEC-024 (no-scroll check):** the placement typing screen scrolled sideways by 4 px at 360 px, caused by a hidden label overflowing its box.
- **Orchestration (merge of `origin/v3`):** a migration number collision. A duplicate version stops the app and production already holds version 7. Fixed before anything shipped, with a regression test.
- **Security review:** a read-only MCP token that could unpublish a unit, a placement `Replace` capped only in the template, and a rate-limit gap on `validate_unit` and `lookup`.
- **SPEC-025:** the old image opens a migrated database and passes its health check, then fails on card queries. A rollback needs the backup file, and the runbook says so.
- **SPEC-023:** the spec's automation credential cannot work as written, because the brief and publish tools are admin-only.

## Methodology

Specs were walked in dependency order. Each went to a separate worker in its own git worktree, and the orchestrator verified acceptance criteria independently rather than trusting the worker's report, then merged with `git merge --no-ff`. Only the orchestrator wrote SDD records. Design came first for frontend specs: Figma frames on page `139:2` (file `NJ8HfTPCQgrFty2VVQBMhY`), then tokens generated from the Figma variables.

| Gate | What it checks |
| --- | --- |
| `gofmt`, `go vet`, `templ generate` | style, vet, and generated templ output has no diff |
| `go run ./cmd/tokens -check` | tokens match the Figma variables; contrast is checked |
| `go test ./...` | about 1,040 tests; migrations tested on temporary databases |
| Browser suites | Shell, Placement, Dialogue, Motion, Activity, Progress in headless Chromium |
| `TestShots` | 63 screens x light and dark x 4 widths: axe, one gold action, feedback colours, 44 px targets, no scroll, reduced motion |
| Mutation checks | each fix and each check is proven able to fail |

## Spec-by-Spec Results

| Spec | Domain | Title | Status | AC met / partial / open |
|------|--------|-------|--------|-------------------------|
| SPEC-001 | docs | ADR-0018: the v3 line and the north-star decisions | complete | 3 / 0 / 0 |
| SPEC-002 | infrastructure | Cut v3 and move CI to it | complete | 4 / 0 / 0 |
| SPEC-003 | data | Evidence tables and card skills | complete | 4 / 0 / 0 |
| SPEC-004 | data | Learner-model tables | complete | 3 / 0 / 0 |
| SPEC-005 | service | One attempt recorder and the grading rule | complete | 4 / 0 / 0 |
| SPEC-006 | data | Placement deck: 200 verified words with audio | complete | 4 / 0 / 0 |
| SPEC-007 | service | Placement engine v3 | complete | 7 / 0 / 0 |
| SPEC-008 | frontend | Lamplight tokens and components | complete | 4 / 0 / 0 |
| SPEC-009 | frontend | Motion system with reduced-motion fallbacks | complete | 4 / 0 / 0 |
| SPEC-010 | frontend | Icons, brand mark, and illustrations | complete | 3 / 0 / 1 |
| SPEC-011 | frontend | Hebrew on-screen keyboard | complete | 3 / 1 / 0 |
| SPEC-012 | frontend | Placement screens | complete | 5 / 2 / 0 |
| SPEC-013 | service | Learner model: states and diagnosis | complete | 4 / 0 / 0 |
| SPEC-014 | scheduled | Daily snapshot and regression job | complete | 4 / 0 / 0 |
| SPEC-015 | service | Activity engine and skill-aware session mix | complete | 5 / 0 / 0 |
| SPEC-016 | frontend | Activity screens and the review card | complete | 5 / 0 / 0 |
| SPEC-017 | frontend | App shell: Today, dock and rail, lesson frame | complete | 5 / 1 / 0 |
| SPEC-018 | frontend | Progress per skill | complete | 4 / 0 / 0 |
| SPEC-019 | service | LessonBrief | complete | 4 / 0 / 0 |
| SPEC-020 | service | Brief checks and auto-publish safety | complete | 5 / 0 / 0 |
| SPEC-021 | api | MCP tools for the learner model and briefs | complete | 5 / 0 / 0 |
| SPEC-022 | docs | One authoring skill for every AI | complete | 3 / 0 / 0 |
| SPEC-023 | scheduled | Weekly report and the coach automation | complete | 3 / 0 / 0 |
| SPEC-024 | frontend | Visual and usability regression checks | complete | 4 / 0 / 0 |
| SPEC-025 | infrastructure | Cutover to production | in_progress | 2 / 0 / 2 |
| SPEC-026 | service | Two-voice dialogue (one male voice, one female voice) | complete | 8 / 0 / 0 |

## Design References

Frames live on Figma page `139:2` of file `NJ8HfTPCQgrFty2VVQBMhY`: placement (light and some dark), App Today, Lesson and Progress for phone and desktop (`182:204`, `183:204`), and the review card. 19 are exported for the SPEC-024 side-by-side index (`internal/web/testdata/figma/`), and the index shows "no frame" for every screen without one. The frames for Today, flashcards and dark variants are out of date against the shipped app. The "Brand & illustrations" frame (SPEC-010 ac-4) was never drawn. Comparison is by inspection only.

## Quality and Verification Evidence

- Go suite and the six browser suites pass on the merged tree at `b2b7d88`. `844f16d` adds SPEC-024; the worker's full run on identical content passed (`TestShots` 838 s plus the six suites).
- The operator's final full re-run of `844f16d` was stopped before it finished. CI run [39365515384](https://github.com/NetworkBuild3r/hebrew-lessons/actions/runs/39365515384) is the check on the pushed commit; its result is not recorded here.
- Security: no unresolved Critical or High; four findings fixed (see `INIT-015-security-review.md`). **Not reviewed:** `internal/mcpstore`, `internal/units`, most web handlers and templates, and `internal/auth` cookie flags.
- Smoke: `scripts/smoke-v3.sh` passed 7 of 7 against a local server only.

## Risks and Follow-Ups

- **Production cutover is not done.** The push deploys on both hosts, the database migrates on start, and there is no backup yet. The migration has never run on real data, and the outage length is unknown. Rollback loses everything after the cutover.
- The security review gap above. A follow-up pass closes it.
- The coach automation needs a credential decision before it is turned on.
- The 206 placement clips are not on the cluster; the six letter clips and the male voice are unheard.
- `ELEVENLABS_VOICE_ID_MALE` is not in `home_k3`; its CI check rejects the name until the runbook's diff 2 is applied.
- Figma is behind the app. English-only copy; the Hebrew nav labels are not Pealim-verified. Only Chromium was tested.
- A reported shared unit still shows in the MCP outline; the slip window can skew by a day (UTC against Chicago time).

## Conclusion

The build is done and the evidence is strong where it was gathered: the suites pass, the regression matrix passes, every check was shown able to fail, and the problems found were fixed before they shipped. What is not done is the part that touches production. Treat INIT-015 as ready for the operator to cut over, not as cut over. Confidence **0.65** (capped: unresolved ACs on SPEC-025 and a partial security review).
