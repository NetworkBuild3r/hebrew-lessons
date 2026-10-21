# SPEC-008 Completion Summary — Sign-in and content gating

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-008 · API foundation, sign-in endpoints, content gating
**Status:** complete
**Provenance:** INIT-002/SPEC-008

## Change

The site now requires a session for every path except the login page. Sign-in, sign-out, and password change go through `/api/`. Lesson catalogs are filtered per user, and a lesson the user cannot see returns 404.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 21 tests passed. See `SPEC-008-eval-report.md`.

## Security

Sessions are HttpOnly, rotated on sign-in, and revoked on sign-out. Mutating `/api/` calls require JSON and a matching Origin or Referer. Invisible lessons are 404. No Critical or High finding. Residuals already in ADR-0002: no CSP, and hash-named word recordings are not filtered per lesson. The login page is a stub; the real form is SPEC-012.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. `server/start.js` also runs the one-time visibility seed after migrate so existing lessons stay visible on first boot.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-008](c78722c5-b05b-45a2-8f7f-fb8496a9bdd3).
