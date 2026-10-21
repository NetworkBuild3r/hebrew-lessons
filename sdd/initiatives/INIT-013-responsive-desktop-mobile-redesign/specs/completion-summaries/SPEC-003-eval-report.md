# SPEC-003 eval report

**Spec:** `INIT-013/SPEC-003` (`SPEC-003-frontend-adaptive-shell-rail-and-dock.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `cursor-grok-4.5-high-fast`, read-only constraint, did not author the files).

## Per-AC verdicts (Loop 2 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | Desktop `@media (min-width: 1024px)` (`shared/worksheet.css:4823-4901`): `body.has-hebrew-shell` grid `var(--shell-rail-w, 260px) minmax(0,1fr)`, sticky `#app-shell`/`.hebrew-shell` `260px` + `border-inline-end`, `.hebrew-shell-nav { flex-direction: column }`. Unit test `desktop rail: body.has-hebrew-shell grid + sticky 260px shell (ac-1, REQ-001)` passes; `chrome-restyle-a11y-smoke.js` at `1280x900` passes (`scrollWidth <= 1280`, all 8 admin / 6 learner links visible, targets >= 44px). |
| ac-2 | PASS | Mobile `@media (max-width: 767px)` (`shared/worksheet.css:4648-4756`): `.hebrew-shell-nav { position: fixed; bottom: 0; inset-inline: 0 }`, column links `min-height: 44px`, `#app-main` `padding-bottom: calc(6rem + env(...))`, `html { scroll-padding-bottom: 6.5rem }`. Unit test `mobile dock: fixed bottom nav… (ac-2, REQ-002)` passes; `chrome-restyle-a11y-smoke.js` at `360x800` passes (`scrollWidth === 360`, all links >= 44px). |
| ac-3 | PASS | Logical properties (`inset-inline`, `border-inline-end`, `margin-inline`, `padding-inline`) in `shared/worksheet.css:4692,4754,4865,4917,4938-4939`. Desktop rail mirrors via `:root[data-ui-lang="en"] body.has-hebrew-shell { direction: ltr }` vs `:root[data-ui-lang="he"] body.has-hebrew-shell { direction: rtl }` while `#app-main.wrap` keeps `direction: rtl` (`:4814-4848`). Unit tests `applyUiLang switches… (ac-3)` (`test/app-shell.test.js`) and `desktop grid mirrors rail for EN/HE… (ac-3)` (`test/restyle-gate.test.js`) pass. |
| ac-4 | PASS | `node --test test/design-tokens.test.js test/restyle-gate.test.js test/player-ui-a11y.test.js test/app-shell.test.js` (`64 pass, 0 fail`) and `node test/chrome-restyle-a11y-smoke.js` (`PASS`) both exit 0. |

## Remediation-loop log

- **Loop 1:** `ac-1`, `ac-2`, `ac-4` PASS; `ac-3` FAIL because `body.has-hebrew-shell` stayed `direction: rtl` when `applyUiLang('en')` set `data-ui-lang="en"` (since `<html>` stays `dir="rtl"` for lesson content), so the desktop grid column 1 (`.hebrew-shell`) did not mirror between left (`en`) and right (`he`).
- **Remediation 1:** Fix sub-agent added `:root[data-ui-lang="en"] body.has-hebrew-shell { direction: ltr }` and `:root[data-ui-lang="he"] body.has-hebrew-shell { direction: rtl }` while keeping `body.has-hebrew-shell > #app-main.wrap { direction: rtl }` in `shared/worksheet.css:4814-4848`, and added unit assertions in `test/restyle-gate.test.js` and `test/app-shell.test.js`.
- **Loop 2 (fresh independent verifier):** all-PASS (`ac-1` through `ac-4` and Verification Strategy).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
