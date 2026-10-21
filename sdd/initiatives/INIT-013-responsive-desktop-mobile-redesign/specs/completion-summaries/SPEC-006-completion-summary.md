# SPEC-006 Completion Summary

**Spec:** SPEC-006 · Responsive viewport matrix, Light/Dark WCAG 2.2 contrast, multi-tense verb workshop, and Chrome smoke verification
**Initiative:** INIT-013 (`INIT-013-responsive-desktop-mobile-redesign`)
**Domain:** testing · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-013/SPEC-006`

## Change Produced

Added the responsive viewport matrix gate: static CSS/DOM assertions plus headless Chrome bounding-box checks for Desktop rail + Today Bento (`1440×960` / `1280×900`) and Mobile bottom dock + zero overflow (`390×844` / `360×800`), including the seeded `verb-tense-workshop` player. Extended `chrome-restyle-a11y-smoke.js` with the same geometry probes and refreshed `spikes/restyle-shots/` screenshots.

| File | Change | Provenance |
|------|--------|------------|
| `test/responsive-layout.test.js` | +new (static CSS/DOM + Chrome geometry for ac-1 / ac-3) | `INIT-013/SPEC-006` |
| `test/chrome-restyle-a11y-smoke.js` | +layoutGeometry + `1440×960` rail/Bento and `390×844` dock checks (ac-2, ac-3) | `INIT-013/SPEC-006` |
| `spikes/restyle-shots/*.png` | refreshed via `chrome-restyle-screenshots-smoke.js` | `INIT-013/SPEC-006` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=light` PASS · `domain=testing` PASS · `autonomy_level=auto_tests` PASS.

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `node --test test/responsive-layout.test.js … test/app-shell.test.js` → **95/95 pass** |
| ac-2 | PASS | `node test/chrome-restyle-a11y-smoke.js` → `PASS` (contrast, overflow, ≥44px, focus, reduced-motion; layout report includes `today@1440` / `*@390`) |
| ac-3 | PASS | Chrome geometry in `responsive-layout.test.js` + a11y smoke: shell width `260`, Bento `sideBySide: true`, nav `position: fixed` at `390` |
| ac-4 | PASS | `chrome-exercise-smoke`, `chrome-verb-workshop-smoke`, `chrome-lesson-types-smoke` all `PASS` |

## Security Review

Test-only changes. Theme allowlist and screenshot path under `spikes/restyle-shots/` (never served) unchanged. Seeded workshop published only inside ephemeral harness DATA_DIR.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Deviations

- Verb-workshop player URL in the live app is `/shared/plan/?id=verb-tense-workshop` (lesson id `decide-choose-believe-win`); the Chrome geometry check seeds, sets lesson visibility to `everyone`, publishes, and opens that plan page for overflow (there is no `/lesson/?id=` route).
