# INIT-020: Speaking — say it aloud, and play a side in a dialogue

**Initiative ID:** INIT-020 · **Status:** complete · **Created:** 2026-10-23 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** [the hybrid model](../../../docs/design/learner-without-a-tutor.md), ADR-0019 (INIT-016/SPEC-001).

## What you get

- **Say it aloud.** An English cue, you say the Hebrew, then the model plays and you mark "Got it" or "Not yet". No Hebrew or audio shows before you try.
- **Play one side of a dialogue.** The other speaker talks; on your lines you get the English cue, say it, then hear it.
- **Both voices working** (female and male), with enough credits to cover the next month of lessons.

## Specs

| Spec | Domain | Title | Blocked by | Effort |
| --- | --- | --- | --- | --- |
| SPEC-001 | infrastructure | Voice service: credits and a male voice (operator) | — | 1h |
| SPEC-002 | service | Say-it-aloud activity type | INIT-017/SPEC-002 | 4h |
| SPEC-003 | frontend | Say-it-aloud step in the player | SPEC-001, SPEC-002 | 5h |
| SPEC-004 | frontend | Dialogue: play one side | SPEC-003 | 6h |

**Total:** 16h. **Longest chain:** SPEC-002 → 003 → 004 (15h), and SPEC-003 also waits on SPEC-001.

## Depends on / unblocks

- Depends on INIT-017/SPEC-002 (unit schema) and on SPEC-001. The voice account has 9,997 credits used of 40,000 (2026-10-24), and the API key cap is 40,000. The male voice is in Vault.
- Unblocks later work: branching role-plays, then an AI conversation partner (not planned yet).

## Done when

- A say step shows only the English cue until "I said it", then the model and its audio, and records one self-check.
- In a dialogue played as one side, each of the learner's lines records one self-check, and two voices are heard.

## Notes

- No recording and no speech recognition in this initiative; both need a microphone permission design first.
- Self-grading is used only because nothing can check speech yet (north star decision 3).
- Security: new unit fields from MCP are enum- and length-checked; no new learner data beyond attempts.
- SPEC-001 through SPEC-004 are complete. A say step shows the English cue, then the model. A dialogue can be played as one side, line by line. A production speaker-b clip is stored. The credit balance and the 40,000 API key cap are recorded in `docs/ops/voice-service.md`. The male voice is in Vault.
