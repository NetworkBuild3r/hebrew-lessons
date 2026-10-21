# SPEC-016 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/admin-users-view.test.js` (6 pass) and `node test/chrome-admin-smoke.js` (ok: true). Read password autocomplete in `shared/admin/users.js`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | An admin creates a user. That user signs in. A reset forces the password dialog. Disable then returns 401 and the browser goes to `/login/`. |
| ac-2 | PASS | Demoting the last admin shows "Cannot demote or disable the last active admin". The row stays `data-role="admin"`. |
| ac-3 | PASS | The learner page shows the progress view, trouble words, and the notice. Load older appends 5 unique events and then hides the button. |
| ac-4 | PASS | A user-role request to `/admin/` is 403 Forbidden, the body has no script, and there are no `/api/admin` calls. |
| ac-5 | PASS | An injection display name renders as text. There is no live image tag. |
| ac-6 | PASS | The password field is `autocomplete="new-password"`. |

No remediation loop.
