# SPEC-004 · Service: Accounts, passwords, sessions, roles, user CLI

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-002
title: Accounts, passwords, sessions, roles, user CLI
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5.5 hours
blocked_by: [SPEC-003]
blocks: [SPEC-008, SPEC-011]
```

---

## Description

**Context:** REQ-001 and REQ-003 need local accounts with two roles, safe password storage, server-side sessions, and a way to create the first admin with no browser and no public sign-up. The MCP's `mcp/oauth.js` is single-owner and keyed off `MCP_AUTH_TOKEN`. It is not reused here; only its patterns are (timing-safe compare, throttling).

**Scope (in):**
- `server/auth/passwords.js`:
  - `hash(password)` and `verify(password, stored)` use `crypto.scrypt` with ADR-0002's parameters, a 16-byte random salt, and a self-describing stored format, so the parameters can rise later.
  - `verify` is timing-safe.
  - `policyCheck(password)` follows ADR-0002 aud-4.
- `server/auth/sessions.js`:
  - `create(userId, meta)` returns a 32-byte random token (base64url) and stores only its SHA-256.
  - `resolve(token, now)` applies the single 30-day lifetime from sign-in (ADR-0002; no idle timeout) and updates `last_seen_at` at most once a minute for the admin view.
  - Also `revoke(token)` and `revokeAllForUser(userId, exceptHash?)`.
- `server/auth/throttle.js`:
  - Failed sign-ins are counted per username and per client IP, with backoff and a temporary lockout (ADR-0002 numbers).
  - Counts are kept in memory; one replica is a documented limit.
  - A failed attempt for an unknown username is answered by verifying against a dummy hash, so its timing matches a real one.
- `server/auth/policy.js`:
  - `can(user, action, resource)` holds the ADR-0002 authorization matrix as data, plus `requireAdmin(user)`.
- `server/accounts.js`:
  - `signIn(username, password, meta)` returns a user and session, or throws one generic error.
  - Also `createUser`, `setRole`, `setStatus`, `resetPassword` (sets `must_change_password`), `changePassword` (checks the current password and revokes other sessions), and `listUsers`.
  - **Guards:** the last active admin cannot be demoted or disabled, and an admin cannot disable themselves.
  - **Revocation:** a role change, a disable, or a password reset revokes all of that user's sessions.
  - **Audit:** every change writes an `audit_log` row.
- `scripts/user.js` (`npm run user -- <cmd>`):
  - **Commands:** `create <username> [--admin]`, `reset <username>`, `disable <username>`, `enable <username>`, `role <username> user|admin`, and `list`.
  - **Password input:** the password is read from a no-echo TTY prompt, or from the file named by `--password-file`. It is never taken from argv or an env var, so it stays out of shell history and the process list.

**Scope (out):** HTTP endpoints and cookies (SPEC-008); OAuth (REQ-015 schema hook only).

---

## Understood (agent's read of the intent)

- [ ] und-1: No account can be created except by an admin action or the CLI (REQ-001).
- [ ] und-2: Every sign-in failure returns one generic error with matching timing, whether the cause is an unknown user, a wrong password, a disabled account, or a lockout. Only the throttle response says "try again later".

## Acceptance Criteria

- [ ] ac-1 (REQ-003): stored hashes never contain the password. Two users with the same password get different hashes. `verify` rejects a one-character change. The stored format records its scrypt parameters.
- [ ] ac-2 (REQ-003): a session token is 256-bit random, and only its SHA-256 is in the database. `resolve` returns null once 30 days have passed since sign-in, and after `revoke`. It still returns the user on day 29, even with no activity in between.
- [ ] ac-3 (REQ-003): after the configured number of failures for one username, further attempts are refused for the lockout window, even with the right password. A different IP's failures count against the username too.
- [ ] ac-4 (REQ-003): a sign-in for an unknown user takes the same code path (a dummy verify) as a wrong password. The test asserts the dummy verify ran.
- [ ] ac-5 (REQ-008): demoting or disabling the last active admin throws. `setRole`, `setStatus`, and `resetPassword` each revoke the target's sessions and write an audit row.
- [ ] ac-6 (REQ-001): `npm run user -- create alice --admin --password-file <tmp>` creates an active admin. The password is not taken from argv, so a test confirms that `--password=` is rejected.
- [ ] ac-7: `can()` refuses every admin action for a `user`-role account, and the refusal is tested for each action name in the matrix.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Throttle numbers | 5 failures → 30 s backoff doubling to 15 min lockout; per-IP cap 20/15 min | resolved | yes — ADR-0002 §3 states these numbers |

---

## Deliverables

- [ ] `server/auth/{passwords,sessions,throttle,policy}.js`, `server/accounts.js`
- [ ] `scripts/user.js`; `package.json` script `user`
- [ ] Tests: `test/auth-passwords.test.js`, `test/auth-sessions.test.js`, `test/auth-throttle.test.js`, `test/accounts.test.js`, `test/user-cli.test.js`

---

## Technical Requirements (domain-specific)

- [ ] `node:crypto` only (`scrypt`, `randomBytes`, `timingSafeEqual`, `createHash`); no dependency.
- [ ] Injected clock (`now`) for expiry and throttle tests.
- [ ] Password, token, and hash values are never logged. Error messages never echo the password.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files; delete `data/`.

---

## Security

- **Sensitivity:** required. Triggers matched: authentication, authorization, password storage, sessions, admin privilege.
- **Review depth:** full. Security acceptance: ac-1 to ac-5 and ac-7.

---

## Verification Strategy

- **Claim:**
  - Passwords are stored only as salted scrypt hashes.
  - Sessions are unguessable, revocable, and expire as specified.
  - Brute force is throttled.
  - User enumeration by timing or message is closed.
  - Role changes can't strand the site without an admin.
- **Check + executor:** `mechanized`. The five test files run with a temp `DATA_DIR` and an injected clock.
- **Pass condition:** all tests pass. Each refusal (wrong password, expired, revoked, locked out, last admin, user-role admin action) has its own named test that fails if the refusal is removed.

---

## Integration Points

- Consumed by SPEC-008 (sign-in endpoints, session middleware), SPEC-010 (admin user routes), SPEC-011 (first-admin bootstrap docs), and SPEC-018 (operator guide).
- **Provenance tag:** `INIT-002/SPEC-004`
