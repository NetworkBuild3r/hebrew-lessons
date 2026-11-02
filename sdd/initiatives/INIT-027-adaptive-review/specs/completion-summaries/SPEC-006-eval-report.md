# SPEC-006 Eval Report

**Verifier:** independent generalPurpose read, model grok-4.7-high. Not the implementer.
**Result:** PASS (ac-1 through ac-8) after one fix.

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | 16:50 MST serve is graded at 17:30 MST; a queue built at 03:30 is rebuilt after 04:00 |
| ac-2 | PASS | No `UTC().Date()` in non-test `internal/web` |
| ac-3 | PASS on recheck | Example word is blanked; no asked-letter name, sound, cue, or pair-letter clip before the pick. First read failed on the letter-clip fallback; that fallback was removed. |
| ac-4 | PASS | Right and wrong picks write one drill answer; a bad pick, a repeat, or another learner is 400 |
| ac-5 | PASS | The third quick right shows Cleared and drops later steps |
| ac-6 | PASS | Reveal and grade return 400 for a two-letter step |
| ac-7 | PASS | Implied and slow lines render on checked answers |
| ac-8 | PASS | Done screen shows non-zero tally rows and keeps them on reload |

The drill handler refuses a pick that is not a pair letter or `dont_know`, measures latency on the server, and checks the queue belongs to the signed-in learner.
