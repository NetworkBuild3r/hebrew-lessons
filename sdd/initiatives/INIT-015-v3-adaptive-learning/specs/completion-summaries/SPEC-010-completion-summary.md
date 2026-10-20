# SPEC-010 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-010 (frontend) · **Status:** complete, ac-4 (Figma frame) not done · **Date:** 2026-10-19

## Change

Commits `e482676` (icons, brand, illustrations) and `395373e` (lamp redraw) on local branch `v3-spec-010`. Merged into local `v3` as `323d8d6` and `026a462`. Not pushed.

- `internal/web/static/icons/sprite.svg`: 27 symbols on a 24px grid, 2px round `currentColor` stroke, 20 distinct glyphs plus 7 aliases for older ids. Lucide-derived glyphs carry the licences in `static/LICENSES/lucide.txt`.
- `internal/web/views/icons.templ` and `icons.go`: `Icon(name, label)`, `IconClass`, `BrandMark()`, `Illustration(name, label)`, `brandHead()`. Unlabelled icons are `aria-hidden`; labelled ones get `role="img"`, `aria-label` and `<title>`.
- Seven illustrations under `static/illustrations/`, each a `<symbol id="art">` using only theme tokens (`placement-intro`, `results`, `empty-review`, `empty-words`, `empty-progress`, `band-up`, `offline`). None is placed on a screen yet; SPEC-012, 016, 017 and 018 place them.
- Brand: `static/brand/mark.svg`, `mark-maskable.svg`, `glyph.svg`, `manifest.webmanifest`. `BrandMark()` replaced the `ע` letter in the header tile. The wordmark stays live text.
- Every inline `<svg>` in `layout`, `pages`, `placement` and `player` templates was replaced with `@Icon…`. This also fixed a real defect: the review audio link had no accessible name.
- Shared files edited: `layout.templ` (`@brandHead()`, brand tile, theme button, dock link), `pages.templ`, `placement.templ`, `player.templ`, `page.go` (`IconHref` now delegates to `iconRef`, so `replay` resolves instead of falling back to help), and four regenerated `*_templ.go` files.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestNoEmojiInTempl`, `TestNoEmojiInViewCopy`, `TestNoInlineSVGOutsideComponents`. The lint also allows `icons.templ`. |
| ac-2 | PASS | `TestIllustrationsUseOnlyTokenColours`, `TestIllustrationAllowlistIsReal`. The older `hero-skyline-night.svg` keeps hex colours as an allowlisted `<img>` exception, and the test fails if that goes stale. |
| ac-3 | PASS as markup and CSS | `TestEveryIconRendersAt20And24`, `TestIconOnlyControlsHaveNames`, `TestIconSizesInCSS`. Nothing was rendered in a browser. |
| ac-4 | NOT MET | The "Brand & illustrations" Figma frame on page `139:2` was not drawn. The worker had read-only Figma access. The SVGs were hand-written. |

Orchestrator checks on merged `v3` at `026a462`: `gofmt -l .` empty, `templ generate` 0 updates, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok.

## The lamp defect (found by the orchestrator, fixed)

The first commit's lamp read as a coffee cup: a bowl with a ring handle and a small spout, with steam-like dots. The tests could not see it. The orchestrator rendered the SVGs with `rsvg-convert` in Docker and looked at them. The worker then redrew the lamp as a classical oil lamp (flattened teardrop body, pinched spout, a yod-shaped flame on the spout tip, a small foot, no handle) in seven files, and viewed its own renders. The orchestrator re-rendered `mark.svg` at 384px and 40px, and `empty-review` and `results`, and confirmed the lamp reads as a lamp and the flame touches the spout. The worker also viewed all 20 icons at 48, 24 and 20px and redrew only `lamp`.

## Deviations and disclosures

- **No PWA PNG icons** (192, 512, maskable). There is no rasteriser and no new dependency was allowed. The manifest points at the SVGs. Installability may be weaker on some platforms, and iOS ignores SVG touch icons.
- **Standalone brand files use literal hex**, because a favicon cannot read page tokens. A test checks each hex equals a token value from `lamplight.tokens.json`.
- **Size limits.** The 6 KB limit holds for each standalone SVG. The sprite is 7.4 KB with an 8 KB ceiling. The existing hero is 8.1 KB with a 9 KB ceiling.
- **Icon helpers** are in `icons.templ`, not `components.templ`, to avoid conflicts with parallel workers.
- **Not verified, anywhere:** how any of this looks in a real browser or in dark mode (the orchestrator's renders used light-mode colours), the art at 16px, whether `var(--…)` resolves inside an external `<use>` target, whether the CSP permits the manifest and favicon, and the brand tile inside a real page.
- No personas, timer, trace or independent AC verifier.

## Gates

None. Do not push `v3`: a push publishes an image that may auto-deploy.
