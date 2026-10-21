# SPEC-026 completion summary: Two-voice dialogue

- **Status:** complete. Merged into local `v3` as `60ef682` (worker commit `76a0e85`, 50 files, no migration). Not pushed.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s026`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Decision recorded:** speaker `a` is female and uses the existing `ELEVENLABS_VOICE_ID`. Speaker `b` is male and uses the new `ELEVENLABS_VOICE_ID_MALE` (the operator confirmed both on 2026-10-19).

## What shipped

A `dialogue` activity (two speakers, 2 to 8 lines) with validation, voice-aware audio lookup and storage, `generate_audio` with `lines` and `speaker`, a player that plays the conversation line by line with one control per line, and `hebrew audio-ensure [--dry-run]`, which generates the embedded placement manifest's missing clips from inside the pod. Docs: `docs/dialogue.md`, `docs/audio-ensure.md`.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestDialogueValidates`, `TestDialogueShapeRules`, `TestDialogueTextAndLinesPlacement` | pass |
| ac-2 | `TestGenerateItemsUsesTheSpeakersVoice`, `TestGenerateAudioDialogueTwoVoices`, `TestGenerateAudioUnitIDExpandsADialogue`, `TestPutUnitDialogueThenGenerateThenValidate` (fake TTS records each call's voice) | pass |
| ac-3 | `TestSameTextInTwoVoices`: same text in a and b makes two rows with different `voice`; a repeat skips both with no provider call | pass |
| ac-4 | `TestMissingMaleVoiceFailsBLinesOnly`, `TestGenerateAudioWithoutMaleVoice` | pass |
| ac-5 | `TestDialogueClipInOtherVoiceIsRejected`, `TestValidateUnitRejectsAClipInTheOtherVoice`, `TestHasAudioForSpeaker` | pass |
| ac-6 | `TestDialogueInBrowser` (Chromium): two voices requested in order, one orb plays one line, a second press stops it, no overflow at 390px, controls at least 44px | pass |
| ac-7 | Full suite, no existing test file modified | pass |
| ac-8 | `TestAudioEnsureDryRun`, `TestAudioEnsureStoresThenSkips`, `TestAudioEnsurePartialFailureAndTheKey`, `TestEmbeddedManifestHas206Texts` | pass |

The worker made about 50 mutations and reports each failed the intended test, with two weak catches (below).

## Verified by the orchestrator

- Merge into `v3` was conflict-free. On the merged `v3`: `gofmt -l .` empty, `go vet ./...` clean, tokens check exit 0, `templ generate` leaves the tree clean (checked on the host), and the full `go test ./... -count=1` passes in every package.
- `-race` on units, audio, mcp, mcpstore and cmd/hebrew passes.
- Read `audio_ensure.go`: the API key is scrubbed from error text.
- I did not repeat the worker's mutation checks or its browser run.

## Deviations from the spec

- Line audio is checked by rule `audio.missing`, not `brief.audio_all`, because that is the rule that owns activity text in SPEC-020's code. It is voice-aware and runs with or without a brief.
- A text with **no** speaker keeps the old skip rule (skipped if any voice has a clip) because an existing test requires it. Speakers `a` and `b` are exact-voice.
- `generate_audio` keeps the 40-per-call cap and adds a 22 s budget per call. Items past the budget return `failed` with "not attempted ... call again", and stored clips are skipped next time. `audio-ensure` uses batches of 10.
- No startup warning for a missing male voice (an existing test forbids warnings from `warnOptionalConfig`). An info line logs a `male_voice` boolean, never the id.
- The `hebrew://schema/unit` resource no longer requires `text`, since a dialogue has none. The validator still requires it for other types.
- Extras: speaker `b` fails if the male voice equals the female one; `ValidVoiceID` allows only letters, digits, `-` and `_`, up to 64 characters.

## Risks and open items

- **audio-ensure and older clips.** A placement text whose existing clip has a different voice than the current `ELEVENLABS_VOICE_ID` gets a new default-voice clip, which could then win playback. Run `--dry-run` first and read `would_generate`.
- A call cut mid-request by the budget is re-sent next call. At most one clip per cut call can be paid twice.
- Per-line coverage is strict for short lines at the 0.90 and 0.95 tiers.
- The male voice has not been tried with `eleven_v3` through the app. Only one direct test clip was made on 2026-10-19.
- The browser test serves a synthetic silent MP3, so real audio output was not checked.
- Weak catches: `TestDialogueShapeRules` was not confirmed to catch a loose speaker check on its own. A mutation of "a line orb plays the whole conversation" failed the browser run as a script timeout, not a clean assertion.
- During mutation testing a test briefly built a real ElevenLabs client with a fake key and got a 401 from the provider host. No real key and no credits were involved. The test was fixed to clear the key.
- **Deploy:** the cluster secret for `hebrew` and `hebrew-next` must now also carry `ELEVENLABS_VOICE_ID_MALE` (home_k3 `elevenlabs-externalsecret.yaml`). SPEC-025 writes that change.

## Contracts for later specs

- `generate_audio` input: `{texts?: string[], lines?: [{text, speaker?: "a"|"b"}], unitId?: number}`, at most 40 in all. Result per item: `{text, textNfc, speaker?, status, audioId?, error?}`. There is no voice field.
- SPEC-021: any new wrapper around `units.Lookup` must also implement `units.SpeakerLookup` (`HasAudioForSpeaker`), or a unit with a dialogue fails with `schema.lookup`. `mcpstore.New(store).WithVoices(...)` supplies the voices. The activity catalog now has eight types. A dialogue counts as the read skill and is an exposure step.
- SPEC-022: use `docs/dialogue.md`. A conversation is a `dialogue`, not a `story`.
