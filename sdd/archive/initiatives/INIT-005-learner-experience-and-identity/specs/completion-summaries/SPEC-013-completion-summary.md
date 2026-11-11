# SPEC-013 completion summary

**Spec:** SPEC-013 · Accessibility, screenshot and hook-contract tests
**Initiative:** INIT-005
**Completed:** 2026-10-12
**Status:** in_review (self-checked; light review depth, no independent reviewer ran)
**Provenance:** `INIT-005/SPEC-013`

## What was built

| File | What it is |
|---|---|
| `test/restyle-gate.test.js` | 26 unit tests (no browser): hook contract, token contrast in light and dark, focus and reduced-motion rules in the CSS, shell and login markup, REQ-016 label scan, and a syntax check of the Chrome scripts. Runs in `npm test`. |
| `test/chrome-restyle-a11y-smoke.js` | Headless-Chrome accessibility gate over Today, Lessons, Guides, Words, Review, Progress, Admin and login. |
| `test/chrome-restyle-screenshots-smoke.js` | Writes 18 PNGs to `spikes/restyle-shots/` and asserts they are real. |
| `test/helpers/chrome-cdp.js` | The launch, CDP and Tab-key helper the two scripts share. Same pattern as the older smokes (spawn Chrome, throwaway profile, WebSocket); it has no `.test.js` suffix so `npm test` does not run it. |

Naming: both Chrome scripts are `chrome-*-smoke.js`. The screenshot script is `chrome-restyle-screenshots-smoke.js`, not `chrome-restyle-screenshots.js`, so it matches the existing pattern. There is **no smoke runner or `smoke` script** in `package.json`; smokes have always been run one file at a time with `node test/chrome-….js`, so nothing was registered. `design-tokens.test.js` already scans `chrome-*.js` for queried classes, so the new files are covered by that scan automatically.

## What the Chrome a11y smoke checks

Signed in as admin (and once as learner), at 360 and 1280 px, on every route, plus login signed out:

- Exactly one visible `h1`; the nav has an `aria-label`; `aria-current` is on exactly the active link (none elsewhere); admin sees 7 links, a learner 6.
- Every interactive element is at least 24 px (inline links in a sentence exempt; a smaller target passes only if a 24 px circle around it touches no other target, which is WCAG 2.5.8). Nav links, `.btn`, sign-in and path buttons are at least 44 px.
- Every interactive element has an accessible name (aria-label, label, text, alt, title), which covers the icon-only buttons.
- `document.documentElement.scrollWidth <= innerWidth` (no sideways scroll) at 360 and 1280.
- Computed text contrast on every visible text node: 4.5:1, 3:1 for large text, opacity and background layers included. Run in light and again with `data-theme="dark"`. 5 to 300 text nodes per screen; none skipped for gradients.
- Tab from the top of each route shows an outline or shadow on every stop, `:focus-visible` matches, and the ring survives the pointer resting on the control.
- Reduced motion (`Emulation.setEmulatedMedia`, same call the navigation smoke uses): no route transition on any of seven route changes; zero elements with a non-zero transition or any animation on every route and on login; `scroll-behavior: auto`; zero running animations. The feedback kit (loading pulse, playing pulse, answer reveal, correct panel, miss panel) is drawn into a probe on the Words screen. Under normal motion the pulse and reveal run, so the reduced result is not a blind pass. Under reduce nothing runs.
- REQ-018: every `Network.requestWillBeSent` the page makes (about 1,500) is same-origin. `chrome-extension:` requests that Chrome's own UI makes are ignored; only http(s)/ws(s) counts.
- No script errors on any page.

## Defects found by the tests and fixed (`shared/worksheet.css` only)

Four real defects. Each was reproduced by a failing run first, then fixed minimally.

1. **Reduced motion did not fully reduce.** The global reset (`animation: none; transition: none`) has no specificity, so later single-class rules (`.admin-tab`, `a.lesson-link`) kept their 0.15 s transitions under `prefers-reduced-motion: reduce`. Fix: `!important` on the two declarations in the ADR-0006 reset. A unit test pins it.
2. **Dark theme: legacy `--muted` never swapped.** `.lesson-card .fine`, `.lesson-level` and about 15 other rules read `#64748b` on the dark surface at 3.51:1 (Today, Lessons). Fix: `--muted: var(--color-muted)` in the SPEC-011 dark block.
3. **Light theme: legacy `--muted` too pale.** `#64748b` is 4.49:1 on the paper and 4.22:1 on the soft panel. Found by the new token-pair test (the Chrome sweep had no such text on screen). Fix: `--muted: #5b6b7f` (4.83:1 or better on every light surface).
4. **English gloss on a selected filter chip: 4.40:1.** `.btn .en, .lessons-filter-btn .en` had `opacity: 0.85`, which dropped the blue gloss on the tint (Lessons "(All)", Review "(Due)"). Fix: opacity 0.92.

No other production file changed. No server, auth or worksheet file was touched; no `worksheets/*/index.html` edit.

## Existing test edited

`test/design-tokens.test.js`: the `QUERIED_CLASS_SNAPSHOT` gained six classes the new smoke queries (`admin-row-btn`, `btn`, `btn-ghost`, `hebrew-shell-nav`, `path-btn`, `visually-hidden`). That test is designed to fail when the smokes query a new class, so this is the contract being extended, not loosened.

## Hook contract findings

- The old check (`player-ui-a11y.test.js` ac-5) counts a hook as present if *any* file under `test/` mentions it, so the ADR's own list could satisfy it. The new test counts production code only (`shared/` and `login/`).
- **Three ADR hooks are not emitted by production code any more:** `data-key`, `data-slug`, `lessons-grid`. The smokes reach them only through optional `||` fallbacks, so nothing is silently blind. They are in an explicit `NOT_EMITTED` list in the test, which also asserts they really are gone (so a rename cannot hide behind the list). The ADR still says "kept"; it should be updated or the hooks restored.
- The SPA shell files (`app.html`, `app-routes.js`, `router.js`, `layout.js`, `app-shell.js`, `app-boot.js`, every `*-page.js`, the admin shell) are in the scanned corpus, and each is checked for the hooks the smokes query (`data-nav`, `data-route-ready`, `data-page-heading`, `data-brand`, `data-shell-*`, `#app-main`, the live region).
- Every `data-*` attribute and element id the Chrome smokes and shell tests query (18 files) must still exist in production, except the two optional ones above. Ids built from a counter (`step-1`, `today-mix-…`) are matched by their prefix.
- The 70 + 50 count still holds.

## REQ-016 label scan

`restyle-gate.test.js` reads literal `aria-label` / `title` values (attribute form, `setAttribute`, `.title =`, object properties, and the `esc('…')` form) from the learner views, shell, router and login and fails on an English-only one (allow-list: "Hebrew Way"). It checks it still finds four known Hebrew labels, so it cannot go blind, and has a self-test. Result: **zero English-only labels** in the learner views, shell and login. Dynamic labels (concatenation, data) are not scanned.

**Known gaps in Admin** (operator tooling, outside the spec's `*-page.js` scope), pinned in a known list so new ones fail and fixed ones must be removed: lesson editor column labels `word`, `sound`, `translit`, `meaning`, `pos` and the `activity type` select; `title="Listen"` on the admin speak button; `aria-label="Progress"` on the learner-progress section; `aria-label="Visibility"` on the lesson visibility select.

## Verification

- `npm test` with `SRS_SCHEDULER=ladder`: **1066 pass, 0 fail** (was 1040; +26 from `restyle-gate.test.js`).
- Chrome smokes, all 19 `test/chrome-*.js` files run one after another after the last CSS edit, **all exit 0**: admin, admin-lessons, ai-profile, audio, flashcards, guides, learner-support, navigation, production-recall, progress, review, signin, story-gloss, today-lessons, today-mix, verb-workshop, words, plus the new restyle-a11y (about 60 s) and restyle-screenshots (about 37 s).
- Screenshots: 18 PNGs, all 360 or 1280 px wide at 1x, heights from 800 to 3668 px, pixel spread 15 to 48 (blank is about 0). Player Match step found at step 1 of `my-morning`.

## Screenshots are for human review

`spikes/restyle-shots/<screen>-<360|1280>.png` for today, lessons, guides, words, review, progress, admin, player-match, login. They are an artefact, not golden images; nothing diffs them. They are untracked, and `spikes/` is not served (`test/spikes-not-served.test.js`). Two things seen while looking that no test judges: at 1280 px the Admin users table scrolls sideways inside its card and clips the action buttons' text at the left edge; and the 360 px player Match step is a long single column (about 1,460 px tall).

## Checks I could not automate

- **Screen reader output.** No NVDA/VoiceOver run. The live region, focus move and names are asserted from the DOM, not heard.
- **Non-text contrast beyond tokens.** The 3:1 control-border and focus-ring ratios are checked as token pairs, not by sampling rendered borders.
- **States that need data.** The seeded database has no due words, published guides, weak words or plans, so the Chrome pass saw the empty states of Review, Guides and Progress. A live flashcard, a populated Guides shelf and filled progress bars are covered only by the static token checks and by the INIT-007 player tests.
- **Dark theme beyond contrast.** The dark sweep checks text contrast and names only; it does not judge layout. Dark remains opt-in (`data-theme="dark"`).
- **Reduced motion for hover and press.** The sweep reads each element's resting computed style; it does not hover or press every control. The global `!important` reset covers those states by rule, and a unit test pins it.
- **Admin `.admin-row-btn` is 36 px** (SPEC-011: dense table rows). It passes the 24 px minimum and is excluded from the 44 px primary check on purpose. Say so if you want it raised.
- **Player screens other than Match** (Story, Speed quiz, Build a sentence, Pattern, Ask a Friend, a live miss) are not in the screenshots or the Chrome a11y pass; INIT-007's `player-ui-a11y.test.js` covers their CSS.
- **Login in dark mode** is checked for contrast; the page is light unless `data-theme="dark"` is set.
- The `Node` in this shell is v26, not 22; nothing in the new code depends on the difference.

## Not done / for `complete-initiative`

- The deliverable "Initiative status ready for `complete-initiative`" is left unticked; the initiative file was not edited.
- ADR-0006 still lists `data-key`, `data-slug` and `lessons-grid` as kept.
- No commit.

## Provenance

`INIT-005/SPEC-013`
