# SPEC-012 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/app-shell.test.js` (8 pass) and `node test/chrome-signin-smoke.js` (ok: true). Read `shared/app-shell.js` nav labels and `shared/api.js` `safeNextPath`. Fetched `http://127.0.0.1:8080/login/` and saw the username and password fields. `git diff --name-only -- worksheets` lists no lesson `index.html`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Smoke: signed-out `/` lands on `/login/?next=%2F`. Sign-in returns to `/`. `next=//evil.example` lands on `/`. `safeNextPath` rejects `//`, schemes, and backslashes. |
| ac-2 | PASS | Smoke: the same shell is on Today, Review, and `/worksheets/my-morning/`, with `aria-current` set. A user session has no Admin link. The DOM test renders Lessons, Progress, and Admin targets. Those three pages are not built yet, so they can still 404. |
| ac-3 | PASS | After a password reset, the forced dialog is open and a nav click does not navigate. After a valid new password, the dialog closes. |
| ac-4 | PASS | The privacy note is on the login page and in the account menu, in the Hebrew-plus-English wording the operator confirmed. |
| ac-5 | PASS | No lesson `index.html` is in the worksheets diff. The lesson page still loads `worksheet.js` by script tag (`worksheetScript: true`). The player's Home button is gone (`hasPlayerHome: false`). |
| ac-6 | PASS | A display name of `<img src=x onerror=alert(1)>` is escaped with `HebrewUtils.escapeHtml`. |
| ac-7 | PASS | The shell uses real buttons and links, Enter and Escape on the account menu, and `:focus-visible` styles. The smoke exercises the forced dialog from the keyboard path the test drives. |

Nav labels in code: היום (Today), שיעורים (Lessons), חזרה (Review), התקדמות (Progress), ניהול (Admin). No remediation loop.
