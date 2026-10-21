# SPEC-026 · service: two-voice dialogue

## Metadata

```yaml
spec_id: SPEC-026
initiative_id: INIT-015
title: "Two-voice dialogue (one male voice, one female voice)"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-020]
blocks: [SPEC-021, SPEC-022]
```

## Description

**Context:** Operator request, 2026-10-19: "when it's a conversation, have more than one voice. We should have a male and female voice." Today a `story` activity is one `text` string, and every clip uses one voice (`DefaultVoiceID`). Clip lookup is by text alone, so the same text cannot be played in two voices. `audio_assets` already stores `voice` on each row and does not make `(text_nfc, voice)` unique, so **no migration is needed**.

**Scope (in):**

- **Unit shape.** A new activity type `dialogue`. It has `lines`, a list of `{ "speaker": "a" | "b", "text": "<pointed Hebrew>" }`. Two speakers only. Speaker `a` is the female voice and speaker `b` is the male voice, fixed for the whole unit so a learner can follow who is talking. A `story` stays a single-voice narration and keeps working.
- **Validation (`internal/units`).**
  - `dialogue` needs 2 to 8 lines, and both speakers must speak at least once.
  - Each line is held to the existing per-text length limit (`brief.text_too_long`), checked per line.
  - Each line's words count toward brief coverage and new-word use exactly as story text does. A dialogue is not a way around the brief.
  - Rule ids: `unit.dialogue_lines`, `unit.dialogue_speakers`.
- **Voices.** Speaker `a` (female) uses the existing `ELEVENLABS_VOICE_ID`, which is also the default voice for narration and every placement clip, so nothing already generated changes. Speaker `b` (male) uses a new setting, `ELEVENLABS_VOICE_ID_MALE`. If `ELEVENLABS_VOICE_ID_MALE` is unset, `generate_audio` returns a clear tool error for each speaker `b` line. It never falls back silently to the female voice.
- **`generate_audio`.** A text may carry a `speaker` (`a`, `b`, or none). Input becomes `texts` (strings, as now) plus `lines: [{text, speaker}]`. `unitId` expands a unit's dialogue lines with their speakers. The skip rule becomes "this text already has a clip **in this voice**". At most 40 clips per call, and clips are generated in batches small enough to finish inside the 30 s server write timeout (found in SPEC-006 phase 2: a 40-clip call is cut off, ten per call is safe).
- **Lookup.** Add a voice-aware lookup next to `audioByTextQuery`. A dialogue line resolves to the clip for its speaker's voice. A single-voice text resolves as before. A line with no clip in its voice is silent, and validation rejects it (`brief.audio_all` already covers this; it must use the voice-aware lookup).
- **Playback.** The unit player plays `dialogue` line by line, in order, with the speaker's label (two neutral names, for example "Dana" and "Avi", fixed in copy) and distinct alignment. The learner can replay one line or the whole conversation. Reduced motion and the `c-answer` rule from SPEC-009 are unchanged.
- **Authoring.** SPEC-022's skill documents the `dialogue` shape and the rule that a conversation uses it, not a `story`.

- **`hebrew audio-ensure`.** A new CLI command, run inside the deployed pod (the image is distroless, so only the binary can do it). `hebrew audio-ensure [--dry-run]` reads the placement audio manifest (embedded in the binary), and for each text with no clip in the default voice it calls the same generator as `generate_audio`, ten texts at a time. It prints counts only (`generated`, `skipped`, `failed`), never a key. `--dry-run` prints how many would be generated and calls no provider. It is idempotent. This is how the 206 placement clips reach the cluster volume.

**Scope (out):** More than two speakers. Choosing voices per learner. Regenerating existing placement clips. Changing the TTS model.

## Understood (agent's read of the intent)

- [ ] und-1: Any content that is a conversation between people must sound like two people, one woman and one man. Narration and single words stay as they are.

## Acceptance Criteria

- [x] ac-1: A unit with a `dialogue` of two speakers validates. A dialogue with one speaker, one line, nine lines, or an unknown speaker fails with the rule ids above. — covered: yes, see completion summary
- [x] ac-2: `generate_audio` for a dialogue creates one clip per line. The clips for speaker `a` use the female voice id and the clips for speaker `b` use the male voice id (checked on the provider call, with a fake TTS client). — covered: yes, see completion summary
- [x] ac-3: The same Hebrew text spoken by both speakers produces two `audio_assets` rows with different `voice`. A repeat call is `skipped` for both and does not call the provider. — covered: yes, see completion summary
- [x] ac-4: With `ELEVENLABS_VOICE_ID_MALE` unset, a speaker `b` line returns a tool error naming that setting, and speaker `a` lines still generate. No clip is stored in the wrong voice. — covered: yes, see completion summary
- [x] ac-5: `validate_unit` rejects a dialogue whose line has a clip only in the other speaker's voice. — covered: yes, see completion summary
- [x] ac-6: In a real browser, a published dialogue unit plays its lines in order with two distinct voice ids requested, and replaying one line plays only that line. — covered: yes, see completion summary
- [x] ac-8: `hebrew audio-ensure --dry-run` on an empty data dir reports 206 to generate and makes no provider call. With a fake TTS client, a real run stores 206 clips, and a second run reports 206 skipped and 0 generated. Output never contains the API key. — covered: yes, see completion summary
- [x] ac-7: Every existing test passes unchanged. In particular the 206 placement clips and `TestItemsAudioRealDeck` are not affected. — covered: yes, see completion summary
## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Which voices | **Male: given.** `ELEVENLABS_VOICE_ID_MALE` is in `.env`; one test synthesis (2026-10-19) returned a valid mp3, so the voice works on the plan. **Female: not given.** The key cannot read voice metadata (401 on `/v1/voices/{id}`), so gender cannot be checked by API. **Resolved by the operator (2026-10-19): the existing default `ELEVENLABS_VOICE_ID` is the female voice.** So there is no `_FEMALE` setting: speaker `a` uses `ELEVENLABS_VOICE_ID` and speaker `b` uses `ELEVENLABS_VOICE_ID_MALE`. Narration and placement clips already use the female voice. | decided | yes |
| aud-2 | Speaker to voice | Speaker `a` is always female and `b` always male. A conversation between two women or two men is out of scope. | assumed | no |
| aud-3 | Where "conversation" appears | Only the new `dialogue` type. Existing `story` units are not rewritten. The coach and authoring skill use `dialogue` for conversations from now on. | assumed | no |
| aud-4 | Model | Dialogue clips use the same model as the rest (`ELEVENLABS_MODEL`, now `eleven_v3`). | assumed | no |

> **Readiness** = 100.

## Deliverables

- [x] `internal/units`: `dialogue` activity, rules, tests
- [x] `internal/audio` and `internal/db/audio.go`: voice-aware lookup and store, tests
- [x] `internal/mcp/tool_generate_audio.go` and `internal/app/audio_gen.go`: `lines` and `speaker`, tests
- [x] `cmd/hebrew/config.go`: the male voice setting (`ELEVENLABS_VOICE_ID_MALE`), tests, plus `audio_ensure.go`
- [x] Unit player template and a browser check for ac-6
- [x] Update to `docs` for the authoring skill (SPEC-022 consumes it)

## Technical Requirements (domain-specific)

- [ ] No schema change and no migration.
- [ ] A voice id is validated as a path segment before use (as `ElevenLabs.endpoint` already does).
- [ ] The two voice ids are read from the environment only. They are not written to logs or test output.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none. Spending TTS credits on real dialogue clips happens when lessons are authored, not in this spec's tests.
- **Blocking checks:** `go test ./internal/units/... ./internal/audio/... ./internal/db/... ./internal/mcp/... ./internal/app/...`, and the browser check for ac-6.
- **Rollback plan:** revert the commit. Existing units and clips are unaffected.

## Security

- **Sensitivity:** standard.
- **Security acceptance criteria:**
  - [ ] `speaker` is validated against `a` and `b` before it selects a voice id. No client-supplied voice id is accepted.
- **Review depth:** quick · **focus areas:** input validation on `generate_audio`.

## Verification Strategy

- **Claim:** a conversation is stored, validated, generated and played in two distinct voices, and nothing already built changes.
- **Check + executor:** mechanized — the Go tests and the browser check.
- **Pass condition:** ac-1 to ac-7 pass.

## Integration Points

- SPEC-020 owns `internal/units` validation. SPEC-021 exposes `generate_audio` and must wait for this spec so the tool contract is final. SPEC-022 documents the shape.
- **Provenance tag:** `INIT-015/SPEC-026`
