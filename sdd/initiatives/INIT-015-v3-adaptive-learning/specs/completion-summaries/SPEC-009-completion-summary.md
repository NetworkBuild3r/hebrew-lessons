# SPEC-009 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-009 (frontend) · **Status:** complete (ac-3 and ac-4 verified in real Chromium 131 after two fixes found by the browser run) · **Date:** 2026-10-19

## Change

Commit `acb297f` on local branch `v3-spec-009`, merged into local `v3` as `efcca4f`. Not pushed. The merge conflicted with SPEC-010 in `layout.templ` and the generated `layout_templ.go`. The orchestrator kept both changes (`@brandHead()` and the `motion.css` link) and regenerated the templ output. The regenerated file holds the brand head, the motion link, `motion.js` and `BrandMark`, with no conflict markers.

- `internal/web/static/css/motion.css`: every movement sits inside `prefers-reduced-motion: no-preference`. Durations are tokens: fast 150 ms, base 250 ms, slow 400 ms (100 to 120 ms under reduce).
- `internal/web/static/js/motion.js`, 2.3 KB: enables htmx global view transitions only when reduced motion is not set, sets `data-playing` on an audio orb from the audio element's events, and handles `[data-advance]` auto-advance that any tap, key press or request cancels.
- `internal/web/motionlint.go` and `motionlint_test.go`, new: the motion lint, with an exact-count allowlist for the older placement motion in `base.css`.
- Named transitions: `screen-exit`, `screen-enter`, `option-press`, `option-picked`, `answer-correct`, `answer-wrong`, `feedback-rise`, `sheet-out`, `card-flip`, `progress-fill`, `orb-pulse`, `stagger-in`. Each has a reduced-motion fallback that keeps the state change visible.
- Tokens: `--ease-spring` is now a real `linear()` spring with a cubic-bezier fallback. `--dur-slow` went from 420 to 400 ms. `--dur-word` (680 ms, unused) was removed. New: `--dur`, `--dur-stagger`, `--motion-press`.
- Shared files edited: `layout.templ` (a link and a script), `tokens.static.css` and generated `tokens.css`, and the script-count assertions in `layout_test.go` and `placement_test.go`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMotionLintFixtures`, `TestMotionLintAllowlistIsExact`, `TestShippedCSSMotionLint`, `TestMotionPatternsHaveFallbacks`. |
| ac-2 | PASS | `TestDurationTokens`: no duration token over 400 ms and no literal duration in the new rules. |
| ac-3 | PASS (browser) | Real Chromium 131, `TestMotionInBrowser`. The first run FAILED: the UA's `view-transition-group` animation ran 250 ms with a transform even under reduced motion. Fixed in `28b83b4` (`::view-transition-group(*) { animation: none }` under reduce). Now only two 120 ms opacity fades remain. The static test had passed. |
| ac-4 | PASS (browser, reworded) | The first run FAILED: the first tap during a swap was lost (taps = 0), because Chromium returns `<html>` from hit-testing for the whole ~300 ms view transition, and `pointer-events: none` on every pseudo does not change it. Operator chose (Gate 7) to skip the view transition when the incoming screen has answer options. `motion.js` now cancels `htmx:beforeTransition` when the response has `c-answer` or `data-no-transition`. The browser test fakes an incoming option, sees no transition and exactly 1 tap, and checks that a plain swap still transitions. |

Orchestrator checks on merged `v3` at `efcca4f`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok. The orchestrator did not run the browser-tagged test and did not run `motion.js`.

## Deviations and disclosures

- **ac-4 was reworded** from "tap during a transition registers" to "a swap bringing in answer options has no transition". Static `TestMotionNeverBlocksInput` had passed and was wrong; it now also requires the handler. The `::view-transition { pointer-events: none }` rule is kept as belt and braces only. No screen has `c-answer` yet; SPEC-012 and SPEC-016 get the behaviour for free, but other tap targets (for example the placement grade buttons) need `data-no-transition` if they arrive inside a transitioning swap.
- **Browser harness fixes in `28b83b4`:** `bypassCSP` (Playwright's `waitForFunction` uses eval, which the app's CSP rightly refuses), the script's stderr is now shown on failure, and the tap message printed a pointer. The test passes ac-3 and fails ac-4 on purpose until the decision above.
- **Old placement motion is allowlisted, not migrated.** `base.css` keeps motion from before this spec, including two literal 140 ms durations. An exact-count allowlist covers it, and each entry has a reduce override in `motion.css`. SPEC-012 should move that motion.
- **View transitions under reduced motion.** The body's `hx-swap="... transition:true"` still runs a 120 ms opacity cross-fade, as the spec's scope text asks. Global view transitions are off under reduce.
- **Auto-advance stays on under reduced motion**, since it is timing and not movement.
- **No screen uses the new hooks yet** (`data-motion`, `data-leaving`, `data-advance`, `data-audio`). SPEC-012, SPEC-016 and SPEC-017 wire them.
- **Script-count assertions** in `layout_test.go` and `placement_test.go` (3 to 4) will conflict with any later branch that adds a script tag.
- No personas, timer, trace or independent AC verifier.

## Gates

None. Do not push `v3`: a push publishes an image that may auto-deploy.

## Correction (2026-10-19, later the same day)

The first version of the ac-4 browser test passed even with the fix removed: it tapped the instant the answer option appeared, which can be before Chromium builds the transition overlay. It was rewritten to tap while a transition would be in flight (commit `73700d3`). Verified by removing the `preventDefault` in `motion.js`: the test then FAILS (a view transition ran, taps = 0), and with the fix restored it passes. The ac-4 PASS above stands only from that commit on.
