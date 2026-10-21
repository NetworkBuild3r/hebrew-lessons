# SPEC-012 completion summary: Placement screens

- **Status:** complete, with ac-1 partial (deviation) and ac-7 by inspection only. Merged into local `v3` as `496f08c`. Not pushed. No migration.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s012`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Merge:** one conflict, in `internal/web/views/layout.templ`. I kept both stylesheet links (`shell.css` and `placement.css`) and regenerated `layout_templ.go`.

## What shipped

`placement.templ` rewritten and handlers in `internal/web/placement*.go`: intro, letters (Part 0), listening, reading, optional typing with the Hebrew keyboard, the picked state (no green or red, auto-advance after 0.8 s or on a tap), part breaks, a progress bar with a count, a confirm-on-close that keeps the run resumable, results, and review of misses. Autoplay with a "Tap to listen" fallback, Replay and Slower (0.75x) chips, keys 1-4, `?` and Space. A broken clip is replaced by the server.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | Render tests count the gold actions per screen | **deviation**: on the pick screens the orb is the gold element, so there is no `Button Kind=Primary` there |
| ac-2 | `TestPlacementFullRun` walks a scripted run and rejects `data-state="correct"`, `data-state="wrong"`, `--go`, `--stop`, feedback markup; `TestPlacementPickedReplyIsTheSameWhateverTheAnswer`; `TestPlacementCSS` rejects `var(--go` and `var(--stop` | pass |
| ac-3 | `TestPlacementListeningMarkup` (no Hebrew text node on listening), `TestPlacementReadingMarkup` (no transliteration, no clip) | pass |
| ac-4 | `<audio ... autoplay>` asserted; `TestPlacementReplacesABrokenClip`, `TestPlacementNoReplacementSaysSo`; browser test for the error path | pass |
| ac-5 | `TestPlacementInBrowser` (keys 1-4, `?`, Space, first tap after a swap, focus order) | pass |
| ac-6 | `TestPlacementReviewListsEveryMiss` | pass |
| ac-7 | 56 captures (light and dark, every screen) in `Hebrew-shots\spec012-placement`, compared to the Figma frames by inspection | **inspection only** | (These capture folders were deleted in the 2026-10-20 cleanup. `TestShots` from SPEC-024 regenerates every screen.)

Security criteria: `TestPlacementCSRF` (answer posts need the token) and `TestPlacementOwnershipAndStaleAnswers`. Escaping goes through templ.

## Verified by the orchestrator

- On the merged `v3`: `gofmt` empty, `go vet` clean, tokens check clean, full `go test ./...` passes in every package.
- All five browser suites pass together in Chromium on the merge: `TestShellInBrowser`, `TestPlacementInBrowser`, `TestDialogueInBrowser`, `TestMotionInBrowser`, `TestActivityInBrowser`.
- I did not repeat the worker's mutations. I did not look at all the screenshots.

## Bugs found at the merge, and fixed (commit `7a86522`)

- **Two placement engines.** `app.go` built one engine for `Deck:` and a second, with a `nil` recorder, for `Placement:`. Today started re-checks on the second and `/placement` resumed on the first, so a re-check would have said "engine is not configured". There is now one `a.placementEngine`, and `TestOnePlacementEngine` fails if a second `placement.NewEngine(` call, a `nil` argument, or a different `Deck:` or `Placement:` value appears. Proven both ways by mutation.
- **`silentMP3` was declared twice** in the `browser`-tagged tests. The placement copy was removed and the dialogue one is shared. Test-only.

## Differences from Figma (ac-7) and other deviations

- The intro says "10-12 minutes"; Figma says 8.
- The typing screen has no orb.
- "We'll work on first" lists the missed words.
- The orb halo style differs. There is no headphones icon.
- On the results screen a re-check result also shows this screen. SPEC-023 owns that.

## Open items and risks

- "Start my first lesson" and "Skip to lesson 1" go to `/`.
- Copy is English only.
- Only Chromium 131 was tested.
- The old placement CSS in `base.css`, the legacy `Service` and `legacy_*.go` are dead code. Remove them later.
- The 206 placement clips are not on the cluster yet. Run `hebrew audio-ensure` in the pod after deploy (SPEC-025 runbook). Until then a listening item with no clip is replaced or skipped, never shown silent.
