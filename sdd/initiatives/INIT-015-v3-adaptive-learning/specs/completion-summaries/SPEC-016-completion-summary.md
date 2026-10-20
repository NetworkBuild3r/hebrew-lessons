# SPEC-016 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-016 (frontend) · **Status:** complete, with deviations listed below · **Date:** 2026-10-19

## Change

Commit `846ea10` on local branch `v3-spec-016`, merged into local `v3` as `09cd3c2` with no conflicts. Not pushed. No migration.

- **New:** `internal/web/activity.go` (answer and Continue handlers), `views/activity.templ` and `views/activity.go` (+ generated `activity_templ.go`), `static/css/activity.css`, `static/js/activity.js`, `activity_test.go`, `activity_browser_test.go` (build tag `browser`), `testdata/activity-browser.mjs`, `testdata/activity-shots.mjs`.
- **Edited:** `web.go`, `learn.go`, `viewmodel.go` (review handler), `views/pages.templ` (`ReviewBody` renders the new frame), `page.go` (`ReviewCard.Act`), `layout.templ` (links `activity.css`), `views/components.templ` (adds `AnswerOptionWith`; `AnswerOption` output unchanged), `internal/app/app.go` (passes the session secret so item tokens use the derived key).
- **Routes:** `POST /_hx/review/answer` (CSRF, queue owner against the signed-in learner, card id and item token must match the served item, `Engine.Check`, `Session.Answered`, feedback sheet), `POST /_hx/review/next` (Continue). `/_hx/review/reveal` and `/grade` return 400 for a checked visit. A visit the lexicon cannot support falls back to a flashcard. A new word records `Expose`.
- **Existing tests edited:** `today_test.go` `openAt` builds the review with a closed letters gate so the flashcard tests still test flashcards; three assertions moved (`href="/audio/…"` to `data-src="/audio/…"`, and `<p class="count">` to `<div class="act__word">`); an unexported `study` test seam on `Options`.
- **Figma:** three frames drawn on page `139:2` from existing components: word match `174:19`, audio match `174:56`, dictation `175:68` (the last is light, derived from the placement typing frame; the other two are dark). Not checked by the orchestrator.

## Acceptance criteria

| AC | Result | Evidence |
| --- | --- | --- |
| ac-1 | PASS, with a deviation | `TestActivityScreensHaveOneGoldAction` (render). The revealed flashcard has no gold action: Figma `151:142` draws four neutral grades and ac-3 forbids colour on them. The spec's ac-1 and ac-3 contradict each other here; the design frame decided. |
| ac-2 | PASS | `TestWrongAnswerSheetShowsTheRightResponse`, `TestTypedAnswerIsEscaped`, `TestReviewCheckedActivityFlow` (right gloss, pointed Hebrew, audio button). The audio button is rendered; playback was not tested. |
| ac-3 | PASS | `TestFlashcardGradesAreNeutral` (labels exactly Forgot, Hard, Got it, Easy; one class `act-grade`; neutral tokens only) |
| ac-4 | PASS | `TestMatchTilesHaveNamesAndKeys` (render) and `TestActivityInBrowser` (real Chromium): names on 8 `audio_match` and 10 `word_match` tiles, Tab/Enter/Space and 1 to 5 / A to E keys, a made pair locks, RTL Hebrew column |
| ac-5 | PASS | `TestPlacementViewsShareNoFeedbackComponent` (parses placement imports and identifiers) |

Orchestrator checks on merged `v3` at `09cd3c2`: `templ generate` made no changes, `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok; `TestActivityInBrowser` and `TestMotionInBrowser` both pass in real Chromium 131. The orchestrator viewed screenshots from the worker's set (listen pick wrong, word match, audio match right on desktop dark): layout, feedback sheet, locked pairs and RTL column look right.

## Orchestrator findings

- **The worker's tap check cannot detect a regression.** With the SPEC-009 transition cancel disabled, `TestActivityInBrowser` still passes, because no review swap uses a view transition at all. The "no view transition on any swap of the review" and "first tap registers" checks therefore hold trivially. They are still true; they do not prove the `c-answer` rule. That rule is proven by `TestMotionInBrowser`.
- **The SPEC-009 ac-4 test had the same weakness and was fixed** (see the correction in the SPEC-009 summary): commit `73700d3` on `v3`. It now fails without the fix.
- **Light-mode Continue is navy, not gold.** By design: the token `action/bg` is ink `#10153a` in light and gold `#f2b84b` in dark.

## Deviations and risks

- **Pick flow is select, then Check, then Continue.** "1 to 4 pick" selects; Enter takes the gold action.
- **`hx-select="unset"`** added on the review forms the worker rewrote, because the body's inherited `hx-select="#app"` dropped fragment swaps. How the old forms behaved was not checked.
- **Replaying the prompt clip before answering posts `hinted=1`** (the worker's reading of `Response.Hinted`). That caps a right answer at Hard.
- **`activity.js` loads from the Review page**, not the layout; on phones the dock and top bar are hidden during an activity, as in the placement deck.
- **English copy only.** Match feedback says "You picked X. That's Y." per wrong pair without naming the word asked.
- **Not verified:** a real session reaching `read_pick` or `word_match` (the latter ran from a fixture page); audio playback, autoplay and the replay-to-hint path (clips 404 in the test server); screen-reader speech; any browser other than Chromium; desktop interactions (screenshots only); reduced motion for these screens in a browser; the Figma frames.
- Screenshots (88 PNGs, phone 390x844 and desktop 1440x900, light and dark) are in the OS temp folder `spec016-shots` and are not committed. No personas, timer or independent verifier.

## Gates

None. Do not push `v3`: a push publishes an image that may auto-deploy.
