# SPEC-010 completion summary

**Spec:** SPEC-010 · Icons, logo and favicon in the shell and every view  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review (self-checked; no independent verifier or security reviewer was run, review depth is light)

Also done in the same pass, as directed: the RSCH-008 feedback-kit follow-ups (SPEC-009 leftovers), the image-prompt style sentence, and the `chrome-production-recall-smoke` investigation.

## What shipped

### Icons, logo and identity wiring (SPEC-010 core)

- **Sprite by name, no new dependency.** Icons are drawn with `<svg class="icon" aria-hidden="true" focusable="false"><use href="/shared/brand/sprite.svg#icon-NAME">`. External same-origin `<use>` was kept (no CSP on app responses) and not the inline sprite: it is one cached request and the lesson player loads only `app-shell.js`, so a server-embedded sprite would not reach it. `docs/design/icon-usage.md` keeps the inline form as the fallback if a CSP is ever added.
- **Shell (`shared/app-shell.js`).** The text/emoji logo is replaced by the real mark (`/shared/brand/mark-a.svg`, option A, as a decorative `<img alt="">`) beside live Hebrew wordmark text and the English gloss, which is now `lang="en"`. The file stays self-contained for the player. Nav keeps the visible Hebrew word and English gloss on every item; the icon is additive and `aria-hidden`. Words now has an icon (`words`); Guides stays text only (no icon exists). The brand link lost its mixed-language `aria-label` (its name is its visible wordmark). The nav landmark label is now Hebrew only (`ניווט ראשי`).
- **Favicon links on lesson pages without editing them.** `ensureBrandHead()` in the shell adds missing `rel=icon` (svg and png), `apple-touch-icon` and `manifest` links, so `worksheets/*/index.html` stay untouched (none of them were edited; `git status` shows nothing under `worksheets/`).
- **Views.** `shared/layout.js` page-heading icons and `shared/today-page.js` buttons use the sprite. Today's "next" arrow gets `icon-directional` so it is mirrored under `[dir=rtl]` (the only directional icon in use). `HebrewUtils.sprite()` is new in `shared/player/utils.js`, and `HebrewUtils.icon()` now maps `volume-high`, `play`, `check`, `house`, `rotate-right` and `list` onto sprite icons (`sound`, `play`, `check`, `home`, `retry`, `lessons`); the rest (pause, stop, microphone, the activity-type icons) keep their inline paths because the sprite has no equivalent. The feedback kit's check mark and retry button, and the plan block's audio button, use the sprite.
- **Icon-only controls get Hebrew-only accessible names** per `docs/design/icon-usage.md`: speak `השמעה`, record `הקלטה`, play recording `השמעת ההקלטה`, story gloss audio `השמעה`, build-sentence speak `השמעת המשפט`, admin speak `השמעה`. The sound toggle `#btn-sfx` is not icon-only (it has a visible word): it gained a `sound` icon, and keeps `data-sfx-toggle`, `aria-pressed`, the 44 px target and the `::after` state mark. The vowels toggle has a visible word and is unchanged.
- **Removed:** `shared/icons.js` (the SPEC-006 starter set, 1.8 px strokes) and its `<script>` tag in `app.html`.
- **Favicon, manifest, identity.** `shared/app.html`, `login/index.html` and `shared/plan/index.html` link the SVG and PNG favicons and the apple-touch icon (and the manifest on the last two); the sign-in page shows the logo-a lockup instead of the old inline text logo. `manifest.webmanifest` lists 192, 512 (any and maskable), the maskable SVG and `/icon.svg`. `/icon.svg` is now the same file as `shared/brand/favicon.svg`. `theme_color` and `background_color` stay `#1d4ed8`.
- **CSS** is one block at the end of `shared/worksheet.css`: `/* === INIT-005/SPEC-010 icons === */` … `/* === end INIT-005/SPEC-010 icons === */`. The "Light by default" block is untouched and nothing in the new block touches the theme.

### `/shared/brand/` public (server)

`server/web.js`: `isPublicStatic` now also opens the prefix `/shared/brand/`. `resolveFile` already allowed everything under `/shared/` once past the gate, so it needed no change. New test in `test/web-gating.test.js`: nine brand files (SVG and PNG, with content types) and the sprite symbols are reachable with no session; a missing brand file and the bare folder are 404 (not 401, no listing); traversal (`..`, `%2e%2e`, `..%2f`), look-alike prefixes (`/shared/brandx/`, `/shared/BRAND/`) and `worksheet.css`, `router.js`, `api.js`, `player/core.js` stay gated.

### Image prompt style (task 4)

`server/media/openai-images.js`: `IMAGE_STYLE_SENTENCE` (exported) is appended inside `buildGenerationBody` as `` `${prompt} ${IMAGE_STYLE_SENTENCE}` ``, after `readPrompt` has run, so refusals, the length limit, the dry-run estimate and the confirm check see only the operator's subject, and the stored `promptText` stays the subject. The existing ac-4 test's "no other literals in `buildGenerationBody`" check still passes because the sentence is a module constant. New tests: the sent prompt is subject-first and style-last; the constant equals the sentence in `docs/design/illustration-style.md` word for word; refusal, confirm and estimate mismatch still make no transport call; the dry-run makes none. No test calls OpenAI.

### RSCH-008 follow-ups on the feedback kit (task 5)

- (a) `feedback.js` header now cites RSCH-008. `SPEC-009-frontend-feedback-kit.md` "RSCH-007" → "RSCH-008" (research reference only).
- (b) The `miss` tone is 440 Hz (was 220 Hz), as quiet as the others, never lower than the tap tone. No test asserted 220; a new test pins it (≥ 400 Hz, not above the correct chime).
- (c) `match-wrong-shake` animation and keyframes removed; `.word-match-card.wrong` keeps its class and colours and gains a static 2 px outline. No test or ADR-0006 hook needed the animation.
- (d) `.is-loading` pulse is now `1s ease-in-out 5` (5 s total, was infinite). `.is-playing` still ends with the clip.
- (e) "Ducks and mutes" corrected where it was a claim about code: `feedback.js` header and `chime()` doc, ADR-0010, INIT-007/SPEC-002 spec, completion summary and eval report, and two test titles. They now say a new chime is suppressed while speech is active; nothing is ducked or faded.

### `chrome-production-recall-smoke` (task 6)

**Root cause: a stale test selector, plus a separate real leak found while checking.**

- The smoke failed with "Hebrew was visible before reveal" because it looked for `card.querySelector('.vocab-head .he-word')`. INIT-007/SPEC-004 wrapped the revealed word in a `.vocab-head` nested **inside** `.reveal.hidden`, so the selector matches the Hebrew while it is `display:none`. The Hebrew target was not on screen. Before that restyle the word was a direct child of `.reveal`, so the selector found nothing and the smoke passed.
- But the selector was hiding a real problem. In English → Hebrew recall the card also renders `exampleBlock` (the full pointed Hebrew sentence, which contains the target word, e.g. "הוּא שׁוֹתֶה קָפֶה" for "שׁוֹתֶה") **outside** `.reveal`, so the answer was visible before the learner recalled it. This predates INIT-007 (the same placement is in `68b1d46`), so it is not a regression from the restyle, but it is a learner-facing hole in recall practice.
- **Fix, code:** `shared/activities/words.js` moves the example block inside `.reveal` for the English → Hebrew card only. Recognition cards and Hebrew → English cards are unchanged.
- **Fix, test:** the smoke now judges visibility by layout (`getClientRects().length`) over every `.he-word` and `[data-vowels]` in the card, not by DOM position. Run against the old `words.js` it fails (the sentence is visible); against the fix it passes.

## Acceptance criteria

| Id | Result |
|----|--------|
| und-1 No external icon CDN | Pass. Scan test over `shared/` and `login/`. |
| und-2 Hooks stay | Pass. ADR-0006 hooks and classes kept; `player-ui-a11y` passes. |
| ac-1 Logo and favicon on signed-in screens, local | Pass. Checked in headless Chrome at 360 and 1280 px on Today, Words and the lesson player. |
| ac-2 Icon-only controls named; primary actions keep text | Pass for the controls found. The test enforces `<button>` + icon-only markup in `shared/`; there is no runtime accessibility audit. |
| ac-3 SVG has no script or external reference | Pass. `brand-assets.test.js` plus new checks on `/icon.svg` and same-origin `<use>`. |

## Verification

- `npm test` (`SRS_SCHEDULER=ladder`): **1040 pass, 0 fail** (was 1010). New: `test/icons-in-shell.test.js` (26 tests), one test each in `web-gating`, `feedback-kit`, and two in `openai-images`; the ac-4 body assertion was updated for the style sentence.
- Chrome smokes, all exit 0: `navigation`, `audio`, `words`, `review`, `progress`, `signin`, `today-lessons`, `production-recall` (now passes), plus `flashcards`, `learner-support`, `story-gloss`, `guides`, `today-mix`, `verb-workshop`.
- Screenshots of Today, Words and the player at 360 and 1280 px were taken and looked at. Icons render (nav 20 px, heading 26 px, sound 20 px), the mark loads, there is no horizontal overflow.

## Other defects fixed on the way

- `core.js` showed the raw text `<span class="en" dir="ltr">…` in the player's step indicator ("Lesson path") because `U.bi()` markup was assigned with `textContent`. Now `innerHTML` for the indicator and a plain Hebrew string for the live-region announcement. A test forbids `textContent = U.bi(`.
- `build-sentence.js` and `story.js` put `ctx.bi(...)` (HTML) inside `title="..."` and `aria-label="..."`, which broke the attribute. Fixed, and the same fix on four group labels (`story` toolbar, `words` direction switch, `ask-friend` compare group, `build-sentence` slot) so each label is one language. A test forbids the pattern.

## Not closed / for the next spec

- **The header is tall.** At 360 px the sticky shell is about 205 px (brand row, two nav rows, account row); at 1280 px the brand, nav (wrapped to two rows) and account button are also stacked. Measured with the icons hidden it is the same height, so this is the existing layout (the English glosses and the account row), not a cost of the icons. Re-flowing it is a layout job for SPEC-011.
- **Sound toggle shows a speaker icon and the ✕/✓ state mark.** Kept the SPEC-009 mark so the existing CSS test and state cue stay; the icon is static (`sound`).
- **Other full-page HTML:** `Worksheet Shais Date Crisis and indirect speech.html` at the repo root is a redirect stub and was not edited. Lesson pages get their icons from the shell script and still carry their own `<link rel=icon href="../../icon.svg">`, which is now the new mark.
- **No `guides`, `microphone`, `pause` or `stop` sprite icon exists.** Guides stays text only; the other three keep inline paths in `utils.js`. SPEC-006 can add them.
- **Dark-ground logo files** (`logo-a-dark.svg`) are not used anywhere yet: the sign-in page and shell header sit on a fixed ground.
- No commit.

## Files

Edited: `shared/app-shell.js`, `shared/app.html`, `shared/layout.js`, `shared/today-page.js`, `shared/ui/plan-render.js`, `shared/player/utils.js`, `shared/player/core.js`, `shared/player/feedback.js`, `shared/activities/{words,story,build-sentence,ask-friend}.js`, `shared/admin/users.js`, `shared/plan/index.html`, `shared/worksheet.css`, `login/index.html`, `manifest.webmanifest`, `icon.svg`, `server/web.js`, `server/media/openai-images.js`, `test/{web-gating,feedback-kit,player-feedback,openai-images,chrome-production-recall-smoke}.js`, `docs/design/{icon-usage,illustration-style}.md`, `docs/decisions/ADR-0010-…md`, SPEC-009 and SPEC-010 specs, INIT-007 SPEC-002 spec/summary/eval report.

New: `test/icons-in-shell.test.js`, this summary.

Removed: `shared/icons.js`.

## Provenance

`INIT-005/SPEC-010`
