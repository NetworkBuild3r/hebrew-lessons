# SPEC-017 Completion Summary — Admin lesson editor

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-017 · Admin: lessons, visibility, and lesson editor
**Status:** complete
**Provenance:** INIT-002/SPEC-017

## Change

Admins can choose who sees each lesson and edit a lesson in the browser. Save stays off until the server's checks pass. Audio runs only after the admin sees the clip counts and confirms. A new lesson stays hidden until it is shared.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 8 unit tests passed. Headless Chrome smoke passed ac-1 through ac-7. Real-tree audio verify: 65 recordings. See `SPEC-017-eval-report.md`.

## Security

Lesson text is escaped, including a vocab meaning that looks like a script tag. Cancel on the audio dialog does not start generation. Invalid lessons are not written. No Critical or High finding. No real ElevenLabs call.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. No browser tool was available. Verification used the headless Chrome smoke. The API has no last-modified field, so the editor records when the copy was opened and warns if the server copy changed before save. The last save still wins.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-017](48f2d6bd-810f-4b43-8c6c-7f4a7a95b6b9).
