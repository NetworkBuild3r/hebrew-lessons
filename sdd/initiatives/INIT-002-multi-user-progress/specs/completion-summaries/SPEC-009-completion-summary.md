# SPEC-009 Completion Summary — Learner progress API

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-009 · Learner progress and metrics endpoints
**Status:** complete
**Provenance:** INIT-002/SPEC-009

## Change

A signed-in learner can read and write only their own review state, lesson progress, and metrics. A lesson they cannot see is treated as missing.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 14 tests passed (6 progress, 8 existing auth). See `SPEC-009-eval-report.md`.

## Security

Routes use the session user only. A `userId` in the query or body is ignored. Hidden lessons return 404. Signal batches over 200 items are refused before any write. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. A request body over 64 KB may abort the socket instead of returning a clean 413; the test checks that nothing is written. A batch of 201 signals still returns 413.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-009](8037c559-50fe-4e1a-8ec7-bb27a742e038).
