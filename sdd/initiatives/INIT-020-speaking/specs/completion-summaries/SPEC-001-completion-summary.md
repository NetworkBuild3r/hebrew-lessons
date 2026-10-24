# SPEC-001 completion summary

**Initiative:** INIT-020 · **Spec:** SPEC-001 (infrastructure) · **Status:** complete · **Date:** 2026-10-24

## Change

The voice account still has room for the next month of lessons, and the API key the app sends can now spend that plan. The key's own credit cap was 10,000 while the plan is 40,000, so ElevenLabs was refusing clips that the account could pay for. On 2026-10-24 that cap was raised to 40,000. The same day a production `generate_audio` call with speaker b stored a clip. The male voice was already in Vault. The balance, the key cap, and the clip are recorded in `docs/ops/voice-service.md`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | Production `generate_audio` for speaker b, text בֹּקֶר טוֹב, returned `status: generated` and audio id `baceec6c73b95d5834ad5f35bb5f33d162781dcdca0c4885c07e0b9b5c4b4899`. |
| ac-2 | PASS | `docs/ops/voice-service.md` records 9,997 used of 40,000 on 2026-10-24, a key cap of 40,000, and a heavy month at 10,880. The balance covers the estimate. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. No key and no voice id is in the repo or the doc.
- The mechanized check is the stored clip. Listening to it is the operator's.
- The first production call, before the cap was raised, failed with an ElevenLabs 401 naming a 10,000 key quota and 3 credits left on that key. No clip was stored on that call.
- SPEC-002, SPEC-003, and SPEC-004 were already complete. This summary closes SPEC-001 and the initiative.
