# SPEC-008 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-008 (frontend) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `b4b00aa` on local branch `v3-spec-008` (35 files, +2493 −81 against 97192b2). Merged into local `v3` as `8c1293c` with no conflicts. Not pushed.

- `internal/web/static/css/lamplight.tokens.json`: the 36 Figma variables (22 colours with light and dark, 14 px numbers).
- `internal/web/tokens/` and `cmd/tokens`: generator. `tokens.css` is now generated. Fonts, type scale, shadows, motion and tile colours stay in `tokens.static.css`, embedded in the generator.
- `internal/web/views/components.templ`: `ChipLabel`, `Button`, `ButtonWith`, `AnswerOption`, `AudioOrb`, `ProgressBar`, `Sheet`. CSS is in `components.css` (`c-*` classes).
- `csslint.go`: colour lint with an exact-count ratchet allowlist. One entry: Google's sign-in button (13 hex values).
- `base.css` migrated: no `--primary*`, `--check*` or `--note*` remains, and green and red appear only in feedback rules. The four `.grade[data-tone]` rules were removed.
- 16 golden HTML files under `internal/web/testdata/components/`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestTokensCSSIsGenerated`, `TestBuildIsDeterministic`. `go run ./cmd/tokens -check` prints nothing and exits 0 (orchestrator run on merged `v3`). |
| ac-2 | PASS | `TestColorLintFixtures`, `TestColorLintAllowlistIsExact`, `TestShippedCSSColorLint`. |
| ac-3 | PASS | `TestComponentGolden` (16 subtests), `TestComponentAnswerOptionStates` (default, selected, correct, wrong). |
| ac-4 | PASS | `TestContrastPairs`: every pair at 4.5 or more in both modes. Lowest: `feedback/go` on `go-soft` in light mode, 4.56. |

## Verified by the orchestrator

- Merged `v3` at `8c1293c`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok.
- **Token values against Figma.** The orchestrator read the 36 variables from file `NJ8HfTPCQgrFty2VVQBMhY` through `use_figma` (read-only) and compared them with the committed JSON in both modes: 0 mismatches. This was needed because the first export file was lost to an orchestrator script bug (it wrote the JSON over its own script) and was regenerated. All 22 colour values also appear in the generated `tokens.css`.

## Deviations and disclosures

- **First dispatch blocked, then resumed.** The worker stopped correctly because the input file was missing. The cause was the orchestrator's bug. Nothing was changed in that first run.
- **`Chip` became `ChipLabel`**, because `views.Chip` is already the new-word view-model type.
- **Old class names kept.** `btn btn-primary`, `btn-tonal` and `btn--lamp` still render because existing tests assert them. `btn-primary` is the action colour (slate in light mode, gold in dark mode). Inside `.ink-card` and `.today-place--open` it stays gold, since slate would vanish on slate.
- **Blue is no longer an action colour.** `--focus-ring` is blue and drives only the `:focus-visible` outline.
- **FSRS `--s1..--s4`** were remapped to a neutral and gold ramp (they were unused in `base.css`).
- **`placement.templ` untouched.** `gradeButton` still emits an unused `data-tone` attribute. SPEC-016 owns it.
- **No browser check.** Nothing visual is verified: slate against gold buttons, the CSS-drawn check and x marks, and the faint selected-option border (a key badge is the second cue). SPEC-024 does the visual review.
- **Not verified.** The last technical requirement, "spacing, radius and tap-size tokens used for padding, radius and min-height", is left unticked. The worker reports it, and the orchestrator did not audit it.
- **Selected-state contrast.** The selected answer border uses `--hairline-strong`, which is 16% alpha in light mode and may be too faint.
- **Fragile margin.** The light `go` on `go-soft` pair passes by 0.06. A small Figma tweak would fail the contrast test, and that is the test doing its job.
- **No personas, timer or trace**, and no independent AC verifier sub-agent (as in earlier specs).
- **Cursor trailer** added to the commit message by the tool.

## Gates

None triggered. Do not push `v3`: a push publishes an image that may auto-deploy.
