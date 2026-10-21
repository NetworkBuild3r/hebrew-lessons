# ASMT-002 evidence

Live measurements of the Hebrew web app, taken through the repo's own test harness (`test/helpers/signed-in-server.js` with a temporary `DATA_DIR`, and `test/helpers/chrome-cdp.js` driving headless Chrome). The scripts write only to this folder and `../screens/`.

Run from the repo root:

| Script | Output | What it measures |
| --- | --- | --- |
| `node sdd/assessments/ASMT-002-hebrew-web-app/evidence/capture.js` | `ux-metrics.json`, `../screens/{phone,desktop}-*.png` | Every learner route at 390×844 and 1440×900 in light mode, plus dark Today; four lesson steps; target sizes, text contrast, overflow, headings, primary buttons, nav; reflow at 320px; an eight-Tab focus sample; Admin and Editor |
| `node sdd/assessments/ASMT-002-hebrew-web-app/evidence/probe.js` | `probe.json`, `../screens/probe-*.png` | Viewport-only screenshots and DOM geometry at 1024, 1280, and 1440px; dock at 360 and 390px; player shell against app shell |
| `node sdd/assessments/ASMT-002-hebrew-web-app/evidence/probe2.js` | `probe2.json` | Overflow on every learner route at 360, 1024, and 1280px; step-heading position inside a lesson on phone |

**Capture artifacts.** The full-page `phone-*` and `desktop-*` screenshots use Chrome's beyond-viewport capture. In them the fixed phone dock appears mid-page, and desktop cards look cut off at the right edge. Neither happens in the app. Use the viewport-only `probe-*` shots for layout.

**Known measurement limit.** The `truncated` flag in `probe.json` misses a visible ellipsis on some dock labels. The screenshots are the evidence for label truncation.
