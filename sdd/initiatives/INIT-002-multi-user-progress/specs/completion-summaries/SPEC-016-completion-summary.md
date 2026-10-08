# SPEC-016 Completion Summary — Admin users and learner activity

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-016 · Admin: users and learner activity
**Status:** complete
**Provenance:** INIT-002/SPEC-016

## Change

Admins have a Hebrew-and-English page to create accounts, reset passwords, and open each learner's progress, trouble words, and answer history. A normal learner who opens that page gets the server's refusal, and the admin scripts do not run.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 6 unit tests passed. Headless Chrome smoke passed. See `SPEC-016-eval-report.md`.

## Security

Password fields use `autocomplete="new-password"` and are cleared after submit. Names and trouble-word text are escaped. The last admin cannot be removed, and the table stays as it was. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. No browser tool was available. Verification used the headless Chrome smoke. The Lessons tab is a placeholder for the lesson editor.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-016](68b23c9a-674d-4028-9972-00414523ed1f).
