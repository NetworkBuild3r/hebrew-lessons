# Codebase Assessment: Hebrew test suite → Hebrew lessons app

**Assessment ID:** ASMT-007  ·  **Status:** Complete
**Source:** `c:\Users\BrianNelson\Projects\hb` test surface (`*_test.go`, `internal/web/testdata/*.mjs`, `.github/workflows/ci.yml`). Cited tree: `origin/main` at `c7a41501d431055c36752e4795d3cf6e1f587be7`. Local `main` is one commit ahead (`628258a`). `internal/web/testdata/layout-audit.mjs` is modified in the working tree; line cites for that file are the working copy read on 2026-11-11.
**Assessed:** 2026-11-11  ·  **Analyst:** Cursor agent (session harness resolved `unknown`; see Objectivity)
**Recommendation:** Refactor  ·  **Confidence:** 0.86
**Pipeline mode:** brownfield

## Bottom Line Up Front (BLUF)

**Recommendation: Refactor.** Keep the Go tests as the thing that decides whether a change is safe, and turn the screen measurements that matter into one of those tests so CI runs them. Do not add an agent that looks at screenshots and decides pass or fail. A wrong lesson or a covered answer has to fail the same way every time. The maintenance cost of a second, hand-run checker is that a layout bug can ship while `go test` stays green.

## Objectivity & Independent Validation

This assessment compares the app's own tests with the checks the same app needs. The objection "of course the suite looks fine" is fair, so the limits of this run are stated here.

- **Evidence discipline:** every load-bearing claim below is Tier A. The file was read, or a command was run and its output was seen. Counts (355 test files, 1,745 `Test` functions, about 97,204 test lines) were taken on this tree on 2026-11-11. The discovery script was **not run**: `scripts/dev/run_py.sh` is not under the kit root this skill's path climb produced (`C:\Users\BrianNelson`), because the skill folder is a directory junction. Those counts are from reading the tree, not from the script.
- **Falsification pass:** the Refactor call was checked against three ways it could be wrong. None of them held. See Evidence & Falsification.
- **Independent LLM-judge eval:** **not run.** Harness detection did not confirm sub-agent dispatch (no user declaration, the Cursor bootstrap rule was not in context, and `AskUserQuestion` is not in this session's tool list; the manifest's `verified_on` of 2026-07-25 is older than its 90-day staleness window). The fail-safe profile is `unknown`, which does not include dispatch. The property-check script was also **not executed**, for the same missing launcher. `eval/` records that, and does not claim a pass. There is no PDF: the branded renderer was not run.

## Executive Summary

**We recommend one of the following paths:**

1. **Leave the suite as it is** — `go test` plus a hand-run layout walk.
2. **Hand screen checks to an agent** — a model looks at pages and says whether they are polished.
3. **Refactor** — one Go gate, and the screen measurements that can be stated as numbers live inside it.

**Based on these findings, the recommendation is path 3.**

- **Where the source is today:** The suite is large and mostly deterministic. CI on push and pull request runs `go test -race` in three shards, plus two Node tests that must not skip (`.github/workflows/ci.yml`). Those two include one real browser check: the review question and its lowest action stay on screen at 390×844. A separate layout walk measures many screens and is not in that workflow. Browser-tagged Playwright tests are compiled out of `go test ./...`.
- **What it is missing:** CI does not measure the dock, the lesson keyboard, or a lesson step for overlap, spill, or type that runs off the edge. A string check can pass while a box still covers the answer.
- **Capability parity:** The engine checks (reviews, migrations, auth, lesson HTML) and the screen checks are not at the same level. The first are in the race gate. The second are one viewport in CI, plus a walk you start by hand.
- **Harvestable deltas:** The hand walk is worth keeping as a local tool. It opens every lesson step at several widths, in light and dark, in Hebrew, and at larger text. CI should not try to be that wide, and an agent should not replace the measurements. The walk's own comment says it does not answer a review card (`layout-audit.mjs` lines 11–12), which is the right safety property to keep.

## Assessment

- **Domain summary:** Verification for one adult learning Hebrew in this app. A change is safe when a lesson still plays, a grade still saves, a migration does not drop rows, and a control the learner must press is not off the screen or under another box.
- **Tech stack:** Go tests against SQLite opened in `t.TempDir()` (`internal/web/today_test.go` `openAt`, lines 750–752). Node and headless Chrome for a few web tests. GitHub Actions on the repo's own runner (`ci.yml` lines 1–10, 59–66). No model SDK in any `*_test.go` (search for `openai`, `anthropic`, `bedrock`, and `ChatCompletion` returned no matches).
- **Complexity:** 355 `*_test.go` files, 1,745 `Test` functions, about 97,204 test lines and 90,055 non-test Go lines. The layout walk is 364 lines. Six Go folders have no `*_test.go`: `internal/clock` (15 lines), `internal/content` (starter text), `internal/learner/rule` (the known-word thresholds, 113 lines), `internal/placement/gen`, `cmd/tokens`, `cmd/unit-health`. The numeric pins for the known-word rule live next door in `internal/learner/thresholds_test.go`, not in package `rule`.
- **Modernization opportunities:**
  - Put off-screen, covered, spill, and overlap into a Go test the existing Chrome step can run, on one phone width and one desktop width.
  - Keep the wide walk local. Do not promote it to an agent.
  - Leave `-tags browser` and `-tags smoke` out of the default gate, and say so in one place so a green `go test` is not read as "a person clicked through Chromium."
- **Risk areas:**
  - `TestListFirstPageOver50kAttempts` asserts a time limit (`internal/learner/bench_test.go` lines 125–159). Locally the limit is 200ms (`perfLimit` is twice the 100ms budget, lines 20–27). CI sets `LEARNER_PERF_SLACK=2` (400ms) because a healthy page took 229ms on a shared runner (`ci.yml` lines 629–633). The page-size assertion still runs under `-race`. This is a known flake, already slackened, not a reason to drop the test.
  - `TestSeedLocalFourLessons` writes `data/local-dev/hebrew.sqlite` only when `SEED_LOCAL_LESSONS=1`, and it refuses a database that already has units (`internal/web/lesson_four_types_test.go` lines 105–126). The default path does not touch the learning record.
  - The layout walk throws if it clicks a choice that has gone away mid-navigation. A run on 2026-11-11 died with `TypeError` inside `walkLesson` on that click. A later run, after a null check was added in the working copy, still reported the Hebrew dock off the 320px edge and keyboard keys of 18×27 and 20×27 as too small.
- **Pipeline mode:** Brownfield. The seams are already named: default `go test`, `-tags browser`, the two Node tests in CI, and the hand walk.

## Persona-Lens Findings

| Lens | Headline finding | Evidence (Tier A) |
| --- | --- | --- |
| Software Architect | The suite is ordinary Go and belongs in this app. The part that does not fit is a hand-run script beside the tests, not a framework to port. Idiomaticity 4. Capability of the checks 3. | `ci.yml` `go test -race`; `layout-audit.mjs` is not named in `ci.yml` (search returned no matches). |
| Data Architect | Each web test opens its own SQLite file under `t.TempDir()`. There is no `tenant_id` and no row-level security in `*.go` or `*.sql` (search returned no matches). That is one learner by design, not a missing table in the tests. Migration files `migrate_006` through `migrate_024` plus `migrate_upgrade_test.go` exist under `internal/db/`. Durable model 4. Persistence of test runs 3 (CI logs, no saved audit). Tenancy 4 for this product, not 1. | `today_test.go` 750–752. Migration file list from the tree. |
| Security & Compliance | Auth has its own tests (JWKS, OAuth, session, CSRF, access, local, proxy). The default test database is temporary. CI refuses an image name that contains `hebrew-lessons` and refuses a skipped Node test. Auth 4. Secrets 4 (no model key in tests). Governance 4. Isolation 4 for one learner. | `internal/auth/*_test.go` (7 files). `ci.yml` lines 736–746 and 812–816. |
| Platform / Cloud Architect | No model in the test path, which is the right control for a deterministic suite. Delivery is a real CI workflow on this repo's runner, not a laptop-only script. The wide screen walk is the laptop-only part. Model governance: not applicable (no model calls). Deployment 4. Packaging 4 (Go module plus Node 22 and Chrome only on the web shard). Observability 2. | `ci.yml` lines 674–725. Search of `*_test.go` for model SDKs: no matches. |
| Day-2 Operations | A failed `go test` is in the Actions log and can be re-run. A failed layout walk leaves `report.json` in a temp folder and is not a CI artifact. Rollback of a bad test is git. Observability 2. Operability 3. | `ci.yml` test job, lines 749–775. Layout walk header, `layout-audit.mjs` lines 1–15, which tells a person to set `OUT` and run Node. |
| Capability-Parity / Reuse | Partial. The platform (this app's CI gate) already proves engine behavior and one review viewport. It does not prove the dock, the keyboard, or a lesson step against overlap and spill. Coverage 3. Do not build a second product. Adapt the gate. | `keyboard_test.go` line 415 runs `ReviewGradesStayInView`. `review_layout_test.go` lines 17–21 name the 390×844 check. `ci.yml` line 732 runs only `TestClipPlayAfterSwap` and `TestKeyboardNode`. |
| Agentic / Judgment | About 97k lines of tests are deterministic. The 364-line walk is a pile of measurements and a few exemptions, not a judgment. Its pass/fail must stay reproducible, so it stays a test. Nature clarity 4. Agent-fit 2 (there is almost nothing to put in an agent). | Issue kinds are numeric (`layout-audit.mjs` lines 3–10). Key exemption is `width >= 28 && height >= 40` (line 252). Go pins key width only (`keyboard_test.go` lines 332–333). |

The Data lens's usual "no tenancy, cap the recommendation" veto does not apply. This product is one learner, and the tests already refuse to share a database file with that learner unless a seed flag is set.

## Capability & Readiness Scoring Matrix

Scores are 1–5. **Suite** is the tests as they run today. **Bar** is what this one learner needs before a change is trusted. The example platform catalog in the assessment skill (agent runtime, Postgres RLS, Bedrock) is not this product and is not scored.

| Dimension | Suite | Bar | Notes (Tier A) |
| --- | --- | --- | --- |
| *Capability* | | | |
| Engine checks (schedule, migrations, auth, lesson HTTP) | 4 | 5 | Large Go suite, including `internal/auth` and `internal/db/migrate_*_test.go`. Not a claim that every package is covered: `internal/learner/rule` has no test file of its own. |
| Screen geometry (off-screen, covered, spill, overlap) | 2 | 4 | One review viewport is inside `TestKeyboardNode`. The walk that measures the rest is not in `ci.yml`. |
| A green run means what it says | 3 | 5 | `-tags browser` tests are documented as outside `go test ./...` (for example `internal/web/shell_browser_test.go` lines 22–25). Easy to over-read a green default run. |
| *Readiness* | | | |
| Fixture isolation | 4 | 4 | `t.TempDir()` SQLite. Seed of `data/local-dev` is opt-in and refuses an existing unit count. |
| Auth checks | 4 | 4 | Seven auth test files. CSRF escaping is asserted in `internal/web/layout_test.go` lines 35–36. |
| Gate discipline | 4 | 4 | Race detector, skip forbidden on the two Node tests, image-name guard. |
| Record of a failed screen check | 2 | 4 | Actions log for `go test`. The walk's `report.json` is local only. |
| Delivery of the gate | 4 | 4 | `ci.yml` runs on push to `main` and on pull request. |
| **Capability (avg)** | **3.0** | **4.7** | The gap is screen proof and the meaning of green, not the engine. |
| **Readiness (avg)** | **3.6** | **4.0** | The readiness gap is the missing record of a screen failure, not auth or isolation. |

## Technical Component Analysis

| Source unit | LOC | Nature | Target home | Platform support | Status | Rationale |
| --- | --- | --- | --- | --- | --- | --- |
| `*_test.go` engine, auth, migrations, lesson HTTP | ~97k | Deterministic | Stay in `go test` | CI `go test -race` | Provided | Same input, same assertion. This is the gate to keep. |
| `TestReviewGradesStayInView` | (inside `review_layout_test.go`) | Deterministic | Stay in the Node step | Already a subtest of `TestKeyboardNode` | Provided | Proves one review layout. Does not prove the rest of the shell. |
| `internal/web/layout_test.go` and keyboard CSS pins | (inside the suite) | Deterministic | Stay, but do not treat them as geometry | CI via `go test` | Adapt | They assert markup and a 28px key width. They do not assert that a box is on screen. |
| `internal/web/testdata/layout-audit.mjs` | 364 | Deterministic measurement, with brittle click steps | A Go test on the existing Chrome step, for a few viewports | Not referenced by `ci.yml` | Adapt | Measurements are numbers (lines 3–10). The click loop is what crashed. Do not turn the numbers into a prompt. |
| `*_browser_test.go` (`//go:build browser`) | (inside the suite) | Presentation, opt-in | Stay opt-in | Not in `ci.yml` (`-tags browser` absent) | Provided as a manual extra | Useful, and not the gate. Say that where the gate is described. |
| `TestListFirstPageOver50kAttempts` | (in `bench_test.go`) | Deterministic plus a time bound | Stay | CI slack 2 | Provided | Page size is the hard check. The 200ms bound is local only. |
| `TestSeedLocalFourLessons` | (in `lesson_four_types_test.go`) | Config | Stay skipped unless asked | Not the default | Provided | Must keep refusing a database that already has units. |
| `internal/learner/rule` | 113 | Deterministic | A test beside the package, or a direct call from `thresholds_test.go` | Pins live in `internal/learner/thresholds_test.go` | Adapt | Small. The rule is authoritative (known-word bar). Its package has no `_test.go`. |

## Recommendation

**Refactor.** Confidence **0.86**.

## Recommendation Options Key

1. Leave the suite as it is. `go test` stays the gate and the layout walk stays something a person remembers to run. Not recommended: the walk already crashed once, and CI will not notice.
2. Hand screen checks to an agent that looks at pages and says whether they are polished. Not recommended: pass or fail would change between runs, and a covered answer is a measurement, not a judgment.
3. Refactor. Keep the Go tests. Move the numeric screen checks for a few viewports into the Chrome step CI already has. Keep the wide walk as a local tool. Recommended.

**Why:** Capability average 3.0 against a bar of 4.7, almost all of it in screen proof. Readiness is close (3.6 vs 4.0). The Agentic lens found no judgment worth an agent package. The deterministic core (grades, migrations, the known-word numbers) must stay reproducible, which rules out Agentify and rules out No action, because the screen gap is real.

**Open unknowns (confidence is under 0.9):**

- This assessment did not re-run `go test ./...`. It does not claim the suite is green today.
- A 2026-11-11 walk reported keyboard keys at 18×27 and 20×27. The walk exempts a key only when it is at least 28px wide and 40px tall. The Go test pins width, not height. This report does not decide whether those keys are actually 27px tall or the walk measured badly. One shared check should settle it.

## Evidence & Falsification

- **Load-bearing claims:** all Tier A. The judge and the property-check script did not run (see Objectivity). That lowers how independently this report was checked. It does not leave a load-bearing claim at Tier C.
- **Falsification pass:** hypothesis "Refactor: fold numeric screen checks into `go test`, do not agentify them, do not leave the gate as it is."

| # | How it could be wrong | Tested against | Survived? |
| --- | --- | --- | --- |
| F1 | CI already runs the layout walk or `-tags browser`, so there is no gap. | `ci.yml` was read. Search for `layout-audit`, `-tags browser`, and `-tags smoke` returned no matches. The Node step's `-run` is only `TestClipPlayAfterSwap` and `TestKeyboardNode` (line 732). | No |
| F2 | The walk is real judgment (taste, "polished"), so an agent is the right home. | The issue list is numeric (lines 3–10). The 2026-11-11 failure was a `TypeError` on a missing button, then off-screen and small-tap numbers. Taste is not what it returns. | No |
| F3 | `TestReviewGradesStayInView` already proves layout, so the right call is No action. | That test names one review page at 390×844 (`review_layout_test.go` lines 17–21). It does not open Today, the dock, or a lesson keyboard. | No |

### Claims-Evidence Ledger

| # | Claim | Type | Tier | Source | How verified | Load-bearing? | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C1 | Default CI is `go test -race` plus two Node tests that must not skip | readiness | A | `ci.yml` 659–672, 727–746 | Read | yes | verified |
| C2 | The layout walk is not in CI | parity | A | `ci.yml` search; `layout-audit.mjs` header | Read and search | yes | verified |
| C3 | One review viewport is in CI, via `TestKeyboardNode` | parity | A | `keyboard_test.go` 415; `review_layout_test.go` 17–21; `ci.yml` 732 | Read | yes | verified |
| C4 | Web tests use a temp SQLite, not the learning file | readiness | A | `today_test.go` 750–752 | Read | yes | verified |
| C5 | Seeding `data/local-dev` is opt-in and refuses an existing unit count | readiness | A | `lesson_four_types_test.go` 105–126 | Read | yes | verified |
| C6 | The 50k-attempt test's time limit is slackened in CI and still checks page size | nature | A | `bench_test.go` 20–27, 125–159; `ci.yml` 629–633 | Read | no | verified |
| C7 | The walk's checks are measurements, and its click loop crashed on 2026-11-11 | nature | A | `layout-audit.mjs` 3–10; that run's `TypeError` | Read and ran | yes | verified |
| C8 | No `tenant_id` / RLS, and no model SDK in tests | readiness | A | repo search | Search | no | verified |
| C9 | `internal/learner/rule` has no `_test.go`; pins are in `thresholds_test.go` | capability | A | directory listing; `thresholds_test.go` 7–19 | Read | no | verified |
| C10 | The suite is green on `origin/main` | readiness | — | not run | — | no | dropped |

## Spec Skeleton

Refactor, so there is no agent package. Each item is one domain and a few hours. `create-initiative` would expand these. Nothing here writes the learning database.

### Phase 1: Foundation

- SPEC-001 · Docs — Write the bar in one place: what a green CI run proves (engine, auth, migrations, one review viewport) and what it does not (the wide walk, `-tags browser`). 2h.

### Phase 2: Backend Core

- SPEC-002 · Service — Add one Go test, using the Chrome and Node the web shard already installs, that fails when a control is off-screen, covered, spilled, or overlapping on Today and one lesson step, at 390px and 1280px, light only. Wait until the lesson frame is in the document before clicking. Do not walk sixteen steps. 4–6h.
- SPEC-003 · Service — Call `internal/learner/rule` from a test, or move the known-word pins so a change in `rule.go` fails a test in that package. 2h.

### Phase 4: Frontend

- SPEC-004 · Frontend — Keep the HTML and token pins. Add a one-line comment at `TestTodayLayout` and the keyboard width pin that they do not prove a box is on screen. Do not delete them. 2h.

### Phase 5: Deploy

- SPEC-005 · Infrastructure — Point the existing Node step at the new geometry test and fail the step on `--- SKIP`, the same way it already fails a skipped keyboard test. Leave `-tags browser` and the wide walk out of that step. 3h.

## Risks

| Risk | Probability | Impact | Mitigation |
| --- | --- | --- | --- |
| The new geometry test clicks during an htmx swap and flakes, the way the walk did | M | M | SPEC-002 waits for the frame and does not loop |
| Shard C gets slow if the wide walk is copied into CI | M | M | SPEC-005 keeps the wide walk local |
| Someone treats a green default `go test` as "Chromium was clicked" | M | H | SPEC-001 states the bar |
| A seed flag writes `data/local-dev` | L | H | Leave the skip and the "already has units" refusal in place |
| Key height is a real 27px bug, and this report treats it as a measurement dispute | M | M | SPEC-002 measures the key once, in the same test that pins width |

## Next Step

Proceed to SDD `create-initiative` if you want these five specs opened. This assessment does not open the initiative. Source on that initiative would be ASMT-007.
