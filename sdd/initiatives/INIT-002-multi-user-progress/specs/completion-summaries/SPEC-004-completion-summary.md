# SPEC-004 Completion Summary — Accounts and sessions

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-004 · Accounts, passwords, sessions, roles, user CLI
**Status:** complete
**Provenance:** INIT-002/SPEC-004

## Change

Password hashing, sessions, sign-in throttling, role checks, the accounts service, and `npm run user`. The password is never taken from the command line.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 36 tests passed. See `SPEC-004-eval-report.md`.

## Security

Hashes are salted scrypt. Only the session token's SHA-256 is stored. Unknown usernames take the same verify path as a wrong password. The last admin cannot be removed. No Critical or High finding. Login throttling is in memory, which matches the one-replica limit.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`.
