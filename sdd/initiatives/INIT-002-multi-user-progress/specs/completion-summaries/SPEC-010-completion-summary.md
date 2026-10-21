# SPEC-010 Completion Summary — Admin API

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-010 · Admin endpoints: users, progress, visibility, lessons
**Status:** complete
**Provenance:** INIT-002/SPEC-010

## Change

Admins can manage accounts, read a learner's full answer history, set who sees a lesson, and save or generate lesson audio. Every one of those routes requires an admin session.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 8 tests passed. See `SPEC-010-eval-report.md`.

## Security

A user-role session gets 403 on every admin route, and a signed-out request gets 401. The last admin cannot be demoted or disabled, and an admin cannot disable themselves. Reading answer history writes an audit row. Responses do not include password hashes, session tokens, or the ElevenLabs key. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. Changing only a display name writes the column directly because the accounts service has no display-name function; that write is still audited. Setting visibility to specific users records two service audits (the mode, then the grant list).

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-010](be6fd8a8-c630-48f0-b216-2bbca7f075c3).
