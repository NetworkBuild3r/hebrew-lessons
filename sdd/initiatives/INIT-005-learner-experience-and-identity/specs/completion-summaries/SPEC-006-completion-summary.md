# SPEC-006 completion summary

**Spec:** SPEC-006 · Identity: logo, wordmark, favicon, icon set, illustration guide  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review (assets done; the operator can still revise the logo pick)

## What shipped

Assets only. Nothing is wired into the shell, the manifest, `app.html` or `server/web.js` (that is SPEC-010).

- **Three logo options** in `shared/brand/`, each as a lockup (`logo-a|b|c.svg`), a dark-ground lockup (`logo-?-dark.svg`), a mark (`mark-?.svg`) and a wordmark (`wordmark-?.svg`, `wordmark-?-dark.svg`). Abstract or typographic only: no mascot, no characters, no jokes. Product name as the shell shows it: דרך עברית with the gloss "Hebrew Way".
  - **A, Ayin tile:** ע on the existing blue-to-violet tile with the amber dot.
  - **B, The way:** a road that bends twice and ends in the amber dot, on flat blue.
  - **C, Dalet:** ד built from two bars with the amber dot as the next step, on flat indigo, stacked lockup.
- **Hebrew is outlines, not live text.** Paths were extracted once from the shipped `shared/fonts/rubik-hebrew.woff2` (SIL OFL 1.1) and `rubik-latin.woff2`. No font is needed to render the logos, and there is no `<text>` in any brand SVG.
- **Default pick: option A (Ayin tile).** Favicon set made from it: `favicon.svg`, `favicon-16.png`, `favicon-32.png`, `apple-touch-icon.png` (180), `icon-192.png`, `icon-512.png`, plus the full-bleed `icon-maskable.svg` source. 192, 512 and 180 are full-bleed with the mark inside the maskable safe zone.
- **Icons:** 23 icons on one 24 px grid, 2 px stroke, round caps and joins, `currentColor`, no hard-coded fill: the 22 asked for plus `today` (the shell nav already uses a today icon). One file each in `shared/brand/icons/` and one combined `shared/brand/sprite.svg` with `<symbol id="icon-NAME" viewBox="0 0 24 24">`.
- **Guides:** `docs/design/icon-usage.md` (accessible names, visible-word rule, sizes, sprite use, CSP and path findings, RTL mirroring of `next`/`back`) and `docs/design/illustration-style.md` (palette, line, background, composition, no text, no faces, the prompt sentence, where to append it).
- **Tooling:** `scripts/build-brand-assets.js` rasterizes the PNGs with `sharp` (already approved; no new dependency). `scripts/brand/generate-brand-svgs.py` re-draws the logo outlines and icons from the shipped font if the pick changes. It needs `fonttools` and `brotli` in a throwaway virtual environment outside the repo; it is not part of the app and the committed SVGs do not need it.
- **Test:** `test/brand-assets.test.js` (90 tests, no network).

## The pick, and why

**Option A, Ayin tile, is the default.** The operator asked to close out all specs, so this is a default and not an operator choice. It can be vetoed or revised later.

- It is the closest to what the shell already shows (the ע tile with the amber dot), so SPEC-010 is a swap and not a redesign, and `manifest.webmanifest` `short_name` "עברית" starts with the same letter.
- It reads best at 16 px: one heavy letter shape on a high-contrast tile. B's thin bend and C's dot lose clarity sooner.
- It is the only option whose mark is the Hebrew script itself, so it signals "Hebrew learning" with no explanation.
- Colours are all from `worksheet.css`: `#1d4ed8`, `#4338ca`, `#7c3aed`, `#fbbf24`, ink `#1e293b`, paper `#faf8f4`. White on the tile and ink on paper both clear 4.5:1 (checked in the test).

Runner-up: **B** if the operator wants the logo to say "way" literally. **C** is the most distinctive but looks like a sign or an exclamation at small sizes.

To change the pick: `BRAND_PICK=b python scripts/brand/generate-brand-svgs.py`, then `node scripts/build-brand-assets.js`, then `node --test test/brand-assets.test.js` (steps are in `docs/design/icon-usage.md`).

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 (REQ-010) | Pass. Three logos, wordmarks, favicon set, 23 icons, usage guide, all under `shared/brand/` and `docs/design/`. |
| ac-2 (REQ-011) | Pass. `docs/design/illustration-style.md` holds the sentence for INIT-004 SPEC-009 and where to append it. |
| ac-3 (REQ-018) | Pass. Every asset is a local file. The only URL in any SVG is the `xmlns` namespace, and the test enforces that. The guides' examples use `/shared/brand/...` paths only. |
| ac-4 | Operator directed close-out; default pick recorded (option A), revisable. This was not an operator decision, and the box is ticked on that basis only. |

## Verification

- `node --test test/brand-assets.test.js` passes: 90 tests, 0 fail. It checks that every SVG under `shared/brand` has no `<script`, no `on*=` handler, no `<foreignObject`, `<image`, `<style` or `<text`, no URL other than the `xmlns`, and no `href`, `xlink:href` or `url()` that leaves the document. It checks the sprite has all 23 symbol ids once each and no hard-coded colour, that each icon file follows the grid rules and matches its sprite symbol, that the wordmark fills clear 4.5:1 on `#faf8f4` and on `#141413`, and that the five PNGs have the right dimensions.
- `npm test` passes: 1010 pass, 0 fail. One earlier run failed `api-redirect.test.js` while other INIT-005 specs were editing `shared/router.js` and `server/web.js` in the same working tree. The next two runs were clean. This spec touches none of those files.
- Visual check: lockups, marks, the 16 px favicon, the 192 px icon and the icon grid were rendered and looked at. Self-checked; no independent verifier or security reviewer was run (the spec's review depth is light).

## Caveats for SPEC-010 (not changed here)

- **Public paths.** `server/web.js` serves only `/manifest.webmanifest`, `/icon.svg` and `/shared/fonts/` without a session. `/shared/brand/` is behind the session check, so the favicon, apple-touch icon, manifest icons and the logo on the sign-in page would get a 401. SPEC-010 should make `/shared/brand/` (or the specific files) public in `isPublicStatic` and `resolveFile`.
- **Sprite `<use>`.** No Content-Security-Policy is sent on app or static responses (only the MCP OAuth pages have one), and `.svg` is served as `image/svg+xml`, so same-origin external `<use href="/shared/brand/sprite.svg#icon-play">` works. An inline-sprite alternative is documented in `icon-usage.md` in case a CSP is added later.
- **Existing `shared/icons.js`** has 1.8 px strokes and its own names (`today`, `spark`). SPEC-010 replaces it with the sprite. `today` was added here so the nav loses nothing; `words` has an icon now and `guides` still has none.
- **Manifest** still points at `/icon.svg`, and its `background_color` and `theme_color` stay `#1d4ed8`, which matches the pick. SPEC-010 adds the PNG entries and `<link>` tags.
- **Prompt sentence** is not appended in code. The recommended place is `buildGenerationBody` in `server/media/openai-images.js`, after the prompt checks. That edit needs one test case and belongs to a spec that edits that module.

## Files

New:

- `shared/brand/`: `logo-{a,b,c}.svg`, `logo-{a,b,c}-dark.svg`, `mark-{a,b,c}.svg`, `wordmark-{a,b,c}.svg`, `wordmark-{a,b,c}-dark.svg`, `favicon.svg`, `icon-maskable.svg`, `favicon-16.png`, `favicon-32.png`, `apple-touch-icon.png`, `icon-192.png`, `icon-512.png`, `sprite.svg`, `icons/*.svg` (23)
- `scripts/build-brand-assets.js`, `scripts/brand/generate-brand-svgs.py`
- `docs/design/icon-usage.md`, `docs/design/illustration-style.md`
- `test/brand-assets.test.js`
- this summary

Edited: `SPEC-006-frontend-identity-logo-icons.md` (status `in_review`, boxes ticked).

Not touched: `shared/worksheet.css`, `shared/app-shell.js`, `shared/icons.js`, `manifest.webmanifest`, `icon.svg`, `server/`, `worksheets/*/index.html`. No commit.

## Provenance

`INIT-005/SPEC-006`
