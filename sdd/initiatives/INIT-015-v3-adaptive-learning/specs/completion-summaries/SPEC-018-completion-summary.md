# SPEC-018 completion summary: Progress per skill

- **Status:** complete, with the Figma comparison not done (below). Worker commit `3a21a06`, merged into local `v3` as `b2b7d88`. Not pushed. No migration (the next free number is still 012).
- **Worker:** [SPEC-018 worker](3af6a3ca-05e4-4f50-86aa-1acbf6a9a687), a general-purpose sub-agent in worktree `Hebrew-v3-s018`.
- **Merge:** one conflict, `layout.templ` (both `placement.css` and `progress.css` links kept; `layout_templ.go` regenerated, not hand-merged). The worker branched before the D9 renumber and added no migration, so nothing collided.

## What shipped

- `GET /progress` now shows a skill bar and, per skill, three 90-day charts drawn from the daily snapshots, each with a table that holds the same values, a "This week" block, placement history, and the weekly report card. `?skill=listen|read|recall|spell` picks the tab; an unknown value falls back to listen. No route was added.
- New: `internal/web/chart.go`, `progress.go`, `views/progress.templ`, `views/progress.go`, `static/css/progress.css`, and their tests. `db/progress.go` gains `RegressionRow.LexemeID` (additive).
- The old Progress code in `learn.go`, `web.go`, `viewmodel.go`, `page.go` and `pages.templ` is removed.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestProgressTabsShowThreeChartsWithTheModelsAndSnapshotsNumbers` (all four skills, 90-day fixture); `TestThisWeekAgreesWithGetProgress` compares the page with `mcpstore.Progress` | pass |
| ac-2 | `TestEveryChartHasATableWithTheSameValues` | pass |
| ac-3 | `TestChartColoursComeOnlyFromTokens` (no literal colours or `style=`, `progress.css` passes `lintColors`, no `--go`/`--stop`, seven distinct state fills, hatch on struggling and slipping); a browser check that no chart mark computes to the feedback green or red | pass |
| ac-4 | `TestNoSnapshotsShowsTheEmptyStateAndNoChart`, `TestEvidenceWithoutSnapshotsShowsTheBarAndAHint...`, `TestOneSnapshotDrawsADotAndNoLine` | pass |

`TestProgressInBrowser` (127 checks) passes with the other browser suites.

## Verified by the orchestrator

Worker worktree clean, no migration in the diff. On the merged `v3`: `gofmt` empty, `go vet` clean, templ generate clean, tokens check ok, `go test ./...` passes in every package. The browser suites were re-run on the merged tree (result in the final report). I did not repeat the worker's mutations: about 50 on the Go side and 15 in the browser, all caught except `pressed-looks-same`, which is covered by another mutant. The worker's first batch left 5 survivors; it added a clamp test and a keyboard-reach check and rewrote four, and then all were caught.

**Real bug found by the mutations:** the window showed 89 days. It started 89 days back, but the job writes yesterday. Fixed to 90.

## Deviations and open items

- **Skill tabs** are four `c-answer` buttons in a GET form, not ARIA tabs. The selected one is filled with the text colour so it reads in dark mode.
- **"Left struggling"**: there is no per-word history, so the page shows the net struggling count per skill from the snapshots, and "Back on track" lists words with a regression event in the last 28 days that are not weak now.
- **Figma:** frames `182:204` (phone) and `183:204` (desktop), page `139:2`, drawn before the code with Lamplight variables bound. **The running app was not compared with them.** Not drawn: dark variants, the empty state, the hatch, and the shipped selected-tab style. They show five destinations including Me; older Today frames still show Words.
- **Live vs snapshot:** the bar and headline are live; the charts come from daily snapshots and can differ by up to a day.
- **Timezone:** the page asks for the UTC date; the job writes days in `TZ` (default Chicago). Up to a day of skew.
- **Placement history** shows per-skill levels only for v3 runs; v2 runs show the single stored level.
- **Performance:** each load runs `Model.Summary` (about 90 ms at 50k attempts).
- Only Chromium was tested (no Firefox, WebKit or screen reader). New copy is English only. Hebrew pixels have not been reviewed by a person.
- Screenshots: `C:\Users\BrianNelson\Projects\spec018-shots` (40 PNGs). (These capture folders were deleted in the 2026-10-20 cleanup. `TestShots` from SPEC-024 regenerates every screen.)
