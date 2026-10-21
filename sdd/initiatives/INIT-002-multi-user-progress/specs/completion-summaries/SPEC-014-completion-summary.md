# SPEC-014 Completion Summary — Today and Lessons

**Initiative:** INIT-002-multi-user-progress
**Spec:** SPEC-014 · Today and Lessons pages
**Status:** complete
**Provenance:** INIT-002/SPEC-014

## Change

The home page is now Today: review if something is due, continue the lesson in progress, or start the next new one. Lessons is the library, with a level filter and a status chip. Both use the filtered catalog and the server's progress.

## Gates

- Autonomy `auto_tests`. `gate_actions: []`.
- Pre-flight: gate_actions PASS · security_review=light PASS · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false. Tier: full.

## AC verification

Independent of the author. 6 unit tests passed. Headless Chrome smoke passed. See `SPEC-014-eval-report.md`.

## Security

Titles and blurbs go through the existing escape helper. A lesson the learner cannot see is absent from both pages. Requests in the smoke are only `/api/`. No Critical or High finding.

## Degradation

Timer and trace skipped (`scripts/dev` absent). No persona. No commit. No browser tool was available. Verification used the headless Chrome smoke. There is no bulk progress API, so each lesson's progress is loaded with its own request.

## Harness

Cursor. Model passed: `cursor-grok-4.5-high-fast`. Worker: [SPEC-014](511ff120-b29a-45be-ad07-5aff53d02384).
