# SPEC-017 completion summary: App shell

- **Status:** complete, with ac-5 (Figma match) only partial. Merged into local `v3` as `9474858` (worker commit `6fc29ca`, 40 files, no migration). Not pushed.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s017`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Merge:** one conflict, in the generated `player_templ.go` (SPEC-026 and SPEC-017 both changed `player.templ`; the source merged cleanly). I regenerated it with `templ` from the merged source.

## What shipped

Today with one next action (a state table, tested), a five-destination dock (phone) and rail (from 1024px), a lesson frame, a Me page (theme and log out), a Lessons page, a report path that hides a unit for that learner in the same response, a "Words to watch" card, a 7-day band-slip alert, and the placement re-check entry (through a `web.PlacementEngine` interface).

Routes added: `GET /lessons`, `GET /me`, `GET|POST /lessons/{id}/report`, `POST /_hx/review/band`, `POST /_hx/placement/recheck`.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestChooseTodayAction` (incl. check-in never outranks a lesson in progress), `TestTodayStatesRenderOneGoldAction`, `TestTodayCheckinNeverOutranksALessonInProgress` | pass |
| ac-1b | `TestBandSlipShowsForSevenDays` (shown days 0 to 6, gone on day 7) | pass |
| ac-2 | `TestDockHasFiveDestinationsWithFullLabels`; browser at 320, 360, 390 and Hebrew at 320 (five full labels, one line each); rail from 1024, dock at 1023 | pass |
| ac-3 | `TestReportHidesTheUnitInTheSameResponse`, `TestReportedUnitIsNotOfferedByTheLessonReads`, `TestLearnerUnitProgress`, browser report flow; refusals (no CSRF 403, bad reason 400, long note 400, another learner's unit 404) | pass |
| ac-4 | `TestWordsToWatchNamesTheFailingSkill` | pass |
| ac-5 | Figma frames `149:102`, `153:124`, `149:178`, `155:153` compared by inspection | **partial** |

Real Chromium 131, 80+ checks (contrast, 44px targets, focus rings, `lang`/`dir`, theme, report sheet from the keyboard, first tap after a swap, log out). The worker ran 33 mutations; one survivor (ac-4 sort order, fixture shape) was fixed and caught.

## Verified by the orchestrator

- On the merged `v3`: `gofmt` empty, `go vet` clean, tokens check exit 0, `templ generate` leaves the host tree clean, full `go test ./...` passes in every package.
- All four browser suites pass together in Chromium on the merge: `TestShellInBrowser`, `TestDialogueInBrowser`, `TestMotionInBrowser`, `TestActivityInBrowser`.
- I did not repeat the worker's 33 mutations. I did not look at the screenshots.

## A real bug found, and fixed (commit `bc557fc`)

Under `Referrer-Policy: no-referrer`, Chromium sends `Origin: null` on a **native** form POST, and the CSRF check refuses it on purpose (403 `{"error":"csrf"}`). The existing logout form hit this. Fix: the web app's header is now `same-origin` (`internal/web/web.go`), which sends the real `Origin` to this site and no referrer to other sites. The CSRF check was not loosened. A new browser section ("native form post") submits the logout form natively and checks the request's `Origin`, that the status is not 403, and that the learner lands on sign-in. It passes with the fix; with the header put back to `no-referrer` it fails with `Origin: null` and 403. The OAuth pages in `internal/auth` keep `no-referrer` (they handle `null` on purpose). `shell.js` still sends logout with `fetch`, which also works.

## Deviations from the spec / Figma (why ac-5 is partial)

- No greeting or avatar, and no Review/Words stat cards (Progress belongs to SPEC-018).
- The rail is 92px with labels under icons (the token is pinned); Figma shows wider rows.
- Reviews outrank the next lesson in the hero (spec order).
- No search field or Teacher Studio section on desktop.
- The lesson frame has no "Auto audio" control.
- Lessons has no Guides/Verbs/Words tabs (those pages do not exist on this line).
- The band alert says "level N" where N is the band plus one.
- Placement hero keeps the classes `today-place today-place--open` and `--resume` so `placement_test.go` stays untouched.

## Open items and risks

- **Copy is English only.** New strings are English in the Hebrew shell too, except nav labels and existing Hebrew review strings. The new Hebrew labels "שִׁעוּרִים" and "אֲנִי" are not Pealim-verified.
- Only Chromium was tested. No Firefox, WebKit or screen reader. The Hebrew-shell pixels were not inspected.
- The `app.go` wiring is compile-checked only. No test shows the real engine returning a due check-in through `NewApp`.
- Each Today load calls `learner.Model.List` (about 90 ms at 50k attempts); a slip alert adds two more.
- The slip window compares the event day with the store clock's UTC date, but the daily job writes its day in `America/Chicago`. That can skew by up to a day.
- A reported shared unit still shows in the MCP learner outline.
- `shell.js` is 2042 of 2048 bytes (its size cap).
- The re-check needs SPEC-012's `/placement` to resume an open run. `Engine.Start` does that today, and a re-check in progress is labelled "Resume placement".

## For other specs

- `web.PlacementEngine` (`RecheckDue`, `Latest`, `NewRecheck`) wired through `web.Options.Placement`; nil turns the check-in off. SPEC-012 must reuse one `placement.Engine` in `app.go`.
- Exported: `web.ChooseTodayAction` and `web.TodayInputs`, `session.Service.BuildSessionFor`, `db.Store.RecentBandSlips`, `InProgressLessonRun`, `LearnerUnitProgress`.
- SPEC-018 edits `pages.templ` (merge hot spot). SPEC-024 should use the screenshots in `spec017-shots` as a baseline.
