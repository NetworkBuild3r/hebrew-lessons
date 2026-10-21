# SPEC-008 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/web-gating.test.js test/api-auth.test.js test/http-security.test.js test/lessons-dir.test.js`. Read `server/http/cookies.js`, `server/http/security.js`, `server/web.js` (`decodePath`, `handleStatic`, `handleApi`), `server/api/auth-routes.js`, and `server/start.js`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (21 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Signed-out paths in the test table are refused. HTML gets 302 to `/login/?next=`. Other requests get 401. `/login/` returns 200. |
| ac-2 | PASS | Path-trick table still gated. `decodePath` rejects `..`, a leading dot, a trailing dot, backslash, and NUL, and collapses repeated slashes before `resolveFile`. |
| ac-3 | PASS | Cookie is HttpOnly and SameSite=Lax. `COOKIE_SECURE=1` adds Secure and the `__Host-` name. Sign-in replaces the previous cookie. Sign-out revokes the session, and the old cookie then gets 401. Password change revokes other sessions. |
| ac-4 | PASS | Mutating `/api/` calls without JSON or with a foreign or missing Origin get 403 before the handler. Non-GET methods outside `/api/` get 405. A Referer that matches Host is accepted. |
| ac-5 | PASS | `safeNextPath` sends `https://evil.example`, `//evil.example`, `/\evil.example`, and `javascript:` to `/`. The sign-in response returns that `redirect`. |
| ac-6 | PASS | A user without a grant gets 404 for the restricted lesson folder. The four data scripts are filtered and send `Cache-Control: no-store`. |
| ac-7 | PASS | `/admin/index.html` is 403 for a user and 200 for an admin. |
| ac-8 | PASS | Lesson `index.html` files still load `worksheet.js` by script tag. The test asserts those files were not modified. |

No remediation loop.

Noted, not blocking: `clientIp` trusts the first `X-Forwarded-For` value, so a client can pick the address used for the per-IP throttle. The per-username throttle still applies. No CSP, matching ADR-0002.
