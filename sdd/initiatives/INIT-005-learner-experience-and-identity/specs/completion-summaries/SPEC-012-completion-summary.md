# SPEC-012 completion summary

**Spec:** SPEC-012 · Restyle login and Admin  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review (self-checked; the security review depth is standard but no independent security reviewer ran)  
**Provenance:** `INIT-005/SPEC-012`

## Login (`login/index.html`)

Still one self-contained page outside the shell. Its styles and script stay inline because `/shared/worksheet.css` is gated while signed out. The logo (`/shared/brand/logo-a.svg`, public since SPEC-010) and the self-hosted Rubik are the only assets; no external request.

- **Look.** Warm paper ground, logo lockup, a white 20px card, 48px inputs with a 3.7:1 border and a 3px focus ring, a tactile 48px primary button, error in a tinted box (`role="alert"` kept). Same token names and values as the app, with a `data-theme="dark"` set (the page is light unless that attribute is set).
- **Hooks kept.** Every id, class and `data-*` hook is still there: `login-form`, `login-username`, `login-password`, `login-error`, `login-submit`, `login-privacy`, `data-ai-disclosure`, `.login-*`.
- **Markup changes.** The privacy note and the AI disclosure appeared twice (above and below the form, with a duplicate `id="login-privacy"`). Each now appears once: the short admin-visibility note above the form, the AI disclosure below it. Each label wraps its text in `.login-label` so the English gloss stays on the Hebrew line.
- **Script change (presentation only).** The two notes are built as the Hebrew sentence, a space, then the English sentence in `<span class="en" lang="en" dir="ltr">`, so the brackets and full stop land on the correct side. The visible sentences are the same. The sign-in request, `next` handling, `safeNextPath` and the error messages are unchanged.
- **New Hebrew wording to confirm.** The English-only tag line ("Order coffee. Chat with Shai. Read the story. 10 minutes a day.") now leads with a Hebrew line and keeps the English as the gloss; the footer line "No app to install" became "אין צורך להתקין אפליקציה" with the English gloss. Both Hebrew lines are mine and should be read by the operator before they count as approved copy.

## Admin

- **Shell.** Admin shares the SPEC-011 header, headings, cards and buttons. Admin remains gated by the server; no route, server or auth file changed.
- **Tabs.** A segmented control (Hebrew word over its gloss, current tab raised with an underline and `aria-current` kept) that scrolls instead of wrapping on a phone.
- **Tables.** Users, lessons and history tables sit in a bordered, scrolling wrapper with a shaded header and calm rows. Before this, the users table pushed the whole page to 823 px at 360 px; the cause was a grid child with `min-width: auto`, fixed for the lesson editor and the user detail page as well.
- **Forms and dialogs.** Inputs, selects and textareas set their own background, colour and border. Create-user, reset-password and the forced password dialog use the 18px panel and put the gloss on the label line.
- **Lesson editor.** Sections as cards with a divided heading, metadata as a responsive grid, the vocabulary table scrolls inside its card, activity rows wrap their row buttons, JSON areas are shaded monospace.
- **Plans tab and media preview.** Controls and images inherit the same control and card styles. The plan picker was looked at; the plan builder with real plan content and the media preview with an actual picture were not.
- The CSS is one block at the end of `shared/worksheet.css`: `/* === INIT-005/SPEC-012 restyle login+admin === */`.

## Screens looked at (360 and 1280 px, PNGs)

Login; Admin users, lessons, plans, lesson editor, user detail; create-user dialog; the account menu. No horizontal overflow on any of them after the grid fix.

## Verification

- `npm test` with `SRS_SCHEDULER=ladder`: **1040 pass, 0 fail.**
- Chrome smokes, all exit 0: `signin` (login, hostile `next`, forced password), `admin`, `admin-lessons`, `ai-profile`, plus the other 13 listed in the SPEC-011 summary.
- No auth rule changed: no cookie flag, CSRF check, rate limit, route or server file is in the diff.

## Not closed

- The forced-password dialog was only checked by smoke, not as a PNG.
- Admin lessons table still shows the raw level word (`basic`, `intermediate`) because it is data in an operator tool; it was not given a Hebrew label.
- Dark mode on login and Admin is "in principle" only (token swap); not looked at.
- No independent security reviewer ran.
