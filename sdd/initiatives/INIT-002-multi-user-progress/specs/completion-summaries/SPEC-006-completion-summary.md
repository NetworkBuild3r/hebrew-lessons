# SPEC-006 Completion Summary — Lesson visibility

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-006 · Lesson visibility and per-user content filtering
**Status:** complete
**Provenance:** INIT-002/SPEC-006

## Change

`server/visibility.js` decides who can see a lesson and seeds the three existing lessons as visible to everyone, once. `server/content-filter.js` rebuilds the shared catalog scripts for the signed-in user. A lesson that cannot be parsed is omitted.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 9 tests passed. See `SPEC-006-eval-report.md`.

## Security

Non-admins cannot change visibility. Filtered scripts are emitted with `JSON.stringify`, and `<` is escaped so a lesson title cannot close a script tag. Parsing uses the literal-only lesson reader. Hash-named word recordings still require sign-in but are not filtered per lesson. That residual is recorded in ADR-0002. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`.
