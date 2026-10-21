# SPEC-011 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-011 (frontend) · **Status:** complete, with no browser verification · **Date:** 2026-10-19

## Change

Commit `49f2a17` on local branch `v3-spec-011` (11 new files, no shared file edited), merged into local `v3` as `d34a7cd` with no conflicts. Not pushed.

- `internal/web/views/keyboard.templ`, `keyboard.go` and the generated `keyboard_templ.go`: `HebrewKeyboard(target, lang)`, `HebrewInput(id, name, attrs)` and `HebrewKeyRows`, one table for the Israeli (SI-1452) layout: 27 keys (22 letters plus the five final forms, each on its own key), in rows of 8, 10 and 9.
- `internal/web/static/js/hekbd.js`, 3,821 bytes unminified (budget 4,096). Document-level delegation, so it survives htmx swaps and is safe to load twice.
- `internal/web/static/css/keyboard.css`: tokens only, logical properties, no transitions, no colour literals.
- Tests: Go `TestKeyboardKeyMap`, `TestKeyboardNormalizeVectors`, `TestKeyboardTypedAnswerGrades`, `TestKeyboardGolden`, `TestKeyboardMarkup`, `TestHebrewInputMarkup`, `TestKeyboardJS`, `TestKeyboardCSS`, and a Node suite `internal/web/testdata/keyboard/hekbd.test.js` (55 tests on a fake DOM).

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS, logic only | `TestKeyboardKeyMap` maps `t`, `c`, `,` to אבת. Node wiring tests drive the same on a fake DOM. No real keyboard events. |
| ac-2 | PASS, logic only | Node wiring tests: taps type in order, backspace removes one letter, caret respected. |
| ac-3 | PARTIAL | `TestKeyboardCSS` reads the stylesheet. Keys are 44px tall (`--size-tap`), about 30px wide at 360px, with no `overflow`, fixed or `100vw`. Nothing was rendered. |
| ac-4 | PASS, logic only | Node `hints:` and `jar:` tests: hints on for sessions 1 to 3, off from 4 unless toggled, stored in a `hekbd` cookie. |

Orchestrator checks on merged `v3` at `d34a7cd`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok. The orchestrator also ran `node --test internal/web/testdata/keyboard/hekbd.test.js` on the host (Node 26.12): all pass. The Go test that wraps the Node suite skips inside the Docker image because it has no Node.

## Deviations and disclosures

- **Plan defect in the spec.** ac-3 says every key is at least 44 × 44px. Its own decided assumption aud-1 allows keys at least 28px wide with a 44px hit area, because ten 44px keys cannot fit in 360px. The worker followed aud-1. The ac-3 wording needs correcting when the plan is next revised.
- **No browser test.** The spec asks for a browser test for ac-1 to ac-3. The worker did not write one it could not run. The Node fake-DOM tests stand in. SPEC-024 owns a real browser check.
- **No niqqud row**, because the spec puts niqqud entry out of scope.
- **Self-contained assets.** The component emits its own `<link>` and `<script defer>`, so it assumes one keyboard per page and the layout is untouched.
- **`inputmode="none"`** is set by the script only on a coarse pointer, so desktop learners keep their OS keyboard.
- **Stale line.** `htmx.NOTICE` says the shell ships two JS files. A third now exists. The worker left that shared file alone. Fix it when the other frontend branches are merged.
- **Not verified, anywhere.** Rendering, 360px layout, touch behaviour, hint legibility, whether `inputmode="none"` hides the OS keyboard, focus and scroll behaviour on mobile, htmx script re-execution after a boosted navigation, and `htmx:configRequest` against the real htmx 2 parameters object (tested against a plain object only).
- No personas, timer, trace or independent AC verifier.

## Gates

None. Do not push `v3`: a push publishes an image that may auto-deploy.
