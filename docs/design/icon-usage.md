# Icon and logo usage

**Status:** draft for operator review · **Provenance:** `INIT-005/SPEC-006` · **Requirements:** REQ-010, REQ-016, REQ-018

All assets live in `shared/brand/` and are served from this site. Nothing here loads from another host.

## What is in `shared/brand/`

| File | Use |
|------|-----|
| `logo-a.svg`, `logo-b.svg`, `logo-c.svg` | Three logo options (mark plus Hebrew wordmark plus English gloss). Light-ground colours. |
| `logo-?-dark.svg` | Same lockups with light text for the dark ground. |
| `mark-a.svg`, `mark-b.svg`, `mark-c.svg` | Mark only (square tile). |
| `wordmark-?.svg`, `wordmark-?-dark.svg` | Wordmark only, no tile. |
| `favicon.svg`, `favicon-16.png`, `favicon-32.png` | Browser tab icon, from the recommended option. |
| `apple-touch-icon.png` (180) | iOS home screen. Full-bleed square, no transparency. |
| `icon-192.png`, `icon-512.png`, `icon-maskable.svg` | Web-app manifest icons. Full-bleed, mark inside the maskable safe zone. |
| `icons/NAME.svg` | One file per icon. |
| `sprite.svg` | All icons as `<symbol id="icon-NAME">`. |

The Hebrew in every logo file is outlines (paths), not live text, so it does not depend on a font being loaded. The outlines come from the shipped Rubik Hebrew file (SIL OFL 1.1).

## The icon set

23 icons on one 24 px grid: 2 px stroke, round caps and joins, `stroke="currentColor"`, `fill="none"`. No colour is written into any icon. The icon takes the text colour of whatever holds it, so dark mode and disabled states work with no extra file.

`play` `replay` `next` `back` `check` `wrong` `retry` `home` `lessons` `review` `progress` `settings` `theme` `mute` `sound` `lock` `done` `help` `admin` `upload` `image` `words` `today`

`today` is not in the original list. It is there because the shell nav already uses a today icon. There is no `guides` icon; Guides stays a text-only link until one is needed.

Meaning is never carried by colour alone. `check` (bare tick, a right answer), `done` (tick in a circle, a finished item) and `wrong` (cross) differ in shape.

### Directional icons and right-to-left

`next` points right and `back` points left, as drawn. In a right-to-left screen "next" is leftward. Mirror those two (and only those two) under RTL:

```css
[dir="rtl"] .icon-directional { transform: scaleX(-1); }
```

Give `next` and `back` the class `icon-directional`. Do not mirror `play`, `replay`, `retry`, `review`, `sound`, `mute`, or any icon that shows a real-world object or rotation.

## Using the sprite

Reference a symbol with `<use>`. Set the size on the `<svg>`; the colour comes from CSS `color`.

```html
<svg class="icon" width="24" height="24" aria-hidden="true" focusable="false">
  <use href="/shared/brand/sprite.svg#icon-play"></use>
</svg>
```

```css
.icon { width: 24px; height: 24px; flex: none; color: inherit; }
```

Always add `aria-hidden="true"` and `focusable="false"` to a decorative icon. Put the accessible name on the control, not on the icon (next section).

### Does the external `<use>` work on this server?

Checked in `server/web.js` and `server/http/security.js`:

- **Path is real.** `/shared/**` is served from the repo `shared/` folder (`resolveFile`), and `.svg` is sent as `image/svg+xml`. With `X-Content-Type-Options: nosniff` that correct type matters, and it is correct.
- **No Content-Security-Policy on app or static responses.** The only CSP headers in the repo are on the MCP OAuth pages (`mcp/oauth.js`). The site sends `X-Content-Type-Options`, `Referrer-Policy: same-origin` and `X-Frame-Options: DENY` only. None of them blocks a same-origin `<use>`. External `<use href="/shared/brand/sprite.svg#icon-x">` works in current browsers when the file is same-origin, which it is.
- **Caveat 1, sign-in. Resolved in `INIT-005/SPEC-010`:** `/shared/brand/` is now public (`isPublicStatic` in `server/web.js`; `resolveFile` already allowed `/shared/`), and `test/web-gating.test.js` proves it while the rest of `/shared/` stays gated. The text below is the original finding. `/shared/brand/` was behind the session check. Only `/manifest.webmanifest`, `/icon.svg` and `/shared/fonts/` are public (`PUBLIC_STATIC`, `PUBLIC_FONT_PREFIX`). Inside the signed-in shell that is fine. But the **favicon, apple-touch icon, manifest icons, and the logo on the sign-in page are fetched with no session** and would get a 401. SPEC-010 must add `/shared/brand/` (or just the favicon, PNG and logo files) to the public list and to `resolveFile`'s allow-list in `server/web.js`. Do not change that in this spec.
- **Caveat 2, caching.** Static files are sent with `Cache-Control: no-cache`, so the sprite is revalidated on each load. It is about 6 KB, so this is acceptable. The sprite is requested once per document, not once per icon.
- **Caveat 3, a future CSP.** If a strict CSP is added later, same-origin `<use>` still works under `default-src 'self'`. It would break under `default-src 'none'` with no `img-src`/`default-src` allowance for the same origin, and it never works from a `file://` page.

**Inline alternative** (use it if a CSP change ever blocks the external form, or to avoid a second request): insert the contents of `sprite.svg` once, as the first child of `<body>` in the shell document, then reference by fragment only:

```html
<body>
  <!-- contents of shared/brand/sprite.svg pasted or server-embedded here, once -->
  <svg class="icon" width="24" height="24" aria-hidden="true" focusable="false">
    <use href="#icon-play"></use>
  </svg>
</body>
```

The sprite root is already `width="0" height="0"` and `position:absolute`, so it takes no space. The server already embeds data into the shell for first paint (ADR-0007), so embedding the sprite there is cheap.

## Accessible names for icon-only controls

An icon-only button has no visible word, so it needs an accessible name.

Rules:

1. **One language per label (REQ-016).** The `aria-label` is Hebrew only. Do not write "השמעה (Play)" as an `aria-label`. Screen readers would read two languages in one voice.
2. **English is a gloss, not the label.** If you want to show English, put it as visible, secondary text marked `lang="en" dir="ltr"` (the shell already does this with `.en` spans), or in a `title` tooltip. The Hebrew stays the accessible name.
3. **Label in name (WCAG 2.5.3).** If a control shows a visible Hebrew word, the `aria-label` must start with that exact word.
4. **Say the action, not the picture.** "השמעה", not "משולש".
5. **Set the language.** The page is `lang="he"`. Mark any English gloss `lang="en"`.
6. **State toggles.** For `mute` / `sound` and `theme`, use `aria-pressed` and keep the label the same, or change the label to the new action. Pick one and be consistent.
7. **The sprite carries no `<title>`.** The name is always on the button or link.

```html
<button type="button" class="icon-btn" aria-label="השמעה">
  <svg class="icon" width="24" height="24" aria-hidden="true" focusable="false">
    <use href="/shared/brand/sprite.svg#icon-sound"></use>
  </svg>
</button>
```

### Suggested names

| Icon | Hebrew `aria-label` | English gloss (visible or tooltip only) |
|------|---------------------|-----------------------------------------|
| `play` | השמעה | Play |
| `replay` | השמעה חוזרת | Play again |
| `next` | הבא | Next |
| `back` | הקודם | Previous |
| `check` | נכון | Correct |
| `wrong` | לא נכון | Not correct |
| `retry` | ניסיון נוסף | Try again |
| `home` | דף הבית | Home |
| `today` | היום | Today |
| `lessons` | שיעורים | Lessons |
| `review` | חזרה | Review |
| `progress` | התקדמות | Progress |
| `words` | מילים | Words |
| `settings` | הגדרות | Settings |
| `theme` | מראה בהיר או כהה | Light or dark |
| `mute` | השתקה | Mute |
| `sound` | צליל | Sound |
| `lock` | נעול | Locked |
| `done` | הושלם | Done |
| `help` | עזרה | Help |
| `admin` | ניהול | Admin |
| `upload` | העלאה | Upload |
| `image` | תמונה | Image |

Note: the shell nav already labels Review as "חזרה". `back` is therefore "הקודם" so two controls never share a name.

## When a visible word is required

Icon-only is allowed for **secondary, well-known controls**: play or replay audio, mute, close, theme, help, settings.

A **visible word is required** (icon optional, beside the word) for:

- Primary actions: start a lesson, check an answer, next step, finish, sign in, save.
- Anything destructive or that sends data: delete, upload, publish, generate.
- Navigation items. The nav keeps its Hebrew word next to the icon.
- Any control a first-time learner must understand without guessing.

Where an icon sits beside a word, mark the icon `aria-hidden="true"`. The word is the name.

## Sizes and targets

- **Icon drawing:** 24 px (the grid). Use 20 px only inside a text line. Below 18 px the 2 px stroke fills in.
- **Minimum target:** 24 px by 24 px for quiet controls, with at least 24 px of clear space (WCAG 2.2 target size). Do not use a bare 24 px icon as the only target on a phone.
- **Primary actions:** 44 px by 44 px minimum, per ADR-0006 and REQ-017. Draw the 24 px icon centred in a 44 px button.
- Focus ring: keep the existing focus style. Do not remove it from icon buttons.
- Contrast: the icon is a UI part and needs 3:1 against its background. `currentColor` on the paper ground and the ink text colour is well above that.

## Logo placement

- Use `logo-?.svg` on the light ground and `logo-?-dark.svg` on the dark ground. Swap on `prefers-color-scheme` and on `data-theme`.
- In the 30 px shell header use the **mark** (`mark-?.svg`) plus live Hebrew text, as the shell does now. The lockup is for sign-in, empty states and share images.
- Clear space: at least the height of the sun dot on every side. Minimum lockup height 32 px; minimum mark size 16 px (favicon).
- Do not recolour, stretch, add a shadow, or put the logo on a busy picture.
- The wordmark text is Hebrew "דרך עברית" with the Latin gloss "Hebrew Way", the same pair the shell shows today.

## Logo options in one line each

- **A, Ayin tile.** The letter ע on the existing blue-to-violet tile with the amber dot. Closest to what the shell already shows.
- **B, The way.** A road that bends twice and ends in the amber dot, on flat blue. Literal reading of "דרך" (way).
- **C, Dalet.** The letter ד (first letter of דרך) built from two bars, with the amber dot as the next step, on flat indigo, with a stacked lockup.

The default pick is recorded in the SPEC-006 completion summary. The operator can change it; see "Revising the pick" below.

## Revising the pick

Everything for the favicon and app icons is generated from one choice:

```
BRAND_PICK=b python scripts/brand/generate-brand-svgs.py   # needs fonttools + brotli in a throwaway venv
node scripts/build-brand-assets.js                         # sharp, already a dependency
node --test test/brand-assets.test.js
```

The Python step is only for re-drawing the logo outlines from the shipped font. It is not part of the app and nobody needs it to use the committed files.
