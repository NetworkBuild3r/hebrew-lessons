# SPEC-004 AC verification

**Verifier:** orchestrator (not the author). Re-ran the five auth test files. Confirmed `dummyHash` and the `scrypt$N=…` stored format exist in `server/auth/passwords.js`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (36 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Stored hashes omit the password, differ for the same password, reject a one-character change, and record scrypt parameters. |
| ac-2 | PASS | Token is 256-bit. Only its SHA-256 is stored. Day 29 still resolves. Day 30 and revoke return null. |
| ac-3 | PASS | After the failure limit, the right password is refused. Failures from another IP still count against the username. |
| ac-4 | PASS | An unknown username runs verify against the dummy hash. The test asserts that path ran. |
| ac-5 | PASS | Demoting or disabling the last admin throws. Role, status, and password reset revoke sessions and write an audit row. |
| ac-6 | PASS | `create --admin --password-file` creates an admin. `--password=` is rejected, including through `npm run user`. |
| ac-7 | PASS | Every admin action is refused for a user-role account. |

No remediation loop.
