# SPEC-018 Completion Summary — Operator guide

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-018 · Operator and admin guide, authoring-skill updates
**Status:** complete
**Provenance:** INIT-002/SPEC-018

## Change

There is an operator guide for the first admin, backups, lesson sharing, and audio checks. The lesson-authoring skill now says a new lesson stays hidden until it is assigned. The README says sign-in is required.

## Gates

- Autonomy `plan_only`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=light PASS · domain=docs PASS · autonomy_level=plan_only PASS ⇒ light_tier_eligible=true. Tier: light.

## AC verification

Independent of the author. The first-admin command and a backup were run against a temporary database, then deleted. See `SPEC-018-eval-report.md`.

## Security

The guide names environment variables and the Kubernetes secret name. It does not contain a key or a token. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-018](42f7c008-b31e-4fde-82a4-2fe169ddbba9).
