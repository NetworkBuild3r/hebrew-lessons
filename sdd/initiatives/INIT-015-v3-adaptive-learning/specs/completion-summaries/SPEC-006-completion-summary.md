# SPEC-006 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-006 (data) · **Status:** in_progress. Phase 1 (content) done. Phase 2 (audio) waits for the TTS key. · **Date:** 2026-10-19

## Phase 1: change

Commit `e57484e` on local branch `v3-spec-006` (8 files, +3151 −213), merged into local `v3` as `0568c22` with no conflicts. Not pushed.

- `internal/placement/items.json`: 40 words in each of the five bands, 200 in all. The deck's bands are numbered 0 to 4 (band 0 is the tutor words), and the worker treated the spec's "bands 1 to 5" as these five. 100 words were added and the original 100 are unchanged.
- `internal/placement/letters.json`: six Part 0 letter items, each with three distractors.
- `internal/placement/audio-manifest.json`: the 206 clip texts (200 words, 6 letters) in a fixed order.
- `internal/placement/clips.go`, `gen/curated.go`, `gen/main.go`, `NOTICE`: the generator now writes the manifest, rejects a band under 36 words, requires a well-formed Pealim path on any row that has one, and records the Pealim path of each new word. `NOTICE` is updated.
- `internal/placement/items_test.go`: deck rules (gloss, part of speech, at least 12 nouns and 12 non-nouns a band, at least 36 words a band, NFC, unique glosses), the manifest-matches-deck check, and a silent-word fixture that must fail.

| Band | Words | Nouns | Non-nouns | Dropped |
| --- | --- | --- | --- | --- |
| 0 | 40 | 22 | 18 | 0 |
| 1 | 40 | 17 | 23 | 0 |
| 2 | 40 | 20 | 20 | 0 |
| 3 | 40 | 20 | 20 | 0 |
| 4 | 40 | 22 | 18 | 0 |

Part of speech is limited to noun, verb, adjective and adverb, so distractors can match by part of speech. Pronouns, particles and interjections were left out for that reason.

## Acceptance criteria

| AC | Status | Where to verify |
| --- | --- | --- |
| ac-1: at least 36 verified words a band | MET | `TestItemsDeck`, `TestItemsDeckRulesFail`. All bands have 40, so none is listed as short. |
| ac-2: every word resolves to an `audio_assets` row | NOT MET (phase 2) | `TestItemsAudioRealDeck` skips with "audio phase 2 is pending" until `testdata/audio-available.json` is committed. It does not pass. |
| ac-3: the test fails on a silent-word fixture and passes on the real deck | PARTIAL | `TestItemsAudioSilentWordFails` proves the fixture fails. The real-deck pass waits for phase 2. |
| ac-4: six letter items, each with audio | PARTIAL | `TestItemsLetters` checks the items. Audio waits for phase 2. |

## Verified by the orchestrator

- Merged `v3` at `0568c22`: `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` all packages ok. Running the `TestItems*` tests with `-v` shows six pass and `TestItemsAudioRealDeck` skip with the phase-2 message.
- **Pealim spot-check.** The orchestrator fetched Pealim pages for five words chosen from the deck and compared them with the deck's rows. All five match on pointing, part of speech and gloss, and gender where it applies: `סַכִּין` (m. noun, knife), `דֶּרֶךְ` (f. noun, way), `אֵיפֹה` (adverb, where), `לִישׁוֹן` (verb, to sleep) and `לִשְׁכּוֹחַ` (verb, to forget). The other 195 words were not independently checked, so the worker's statement that all 100 new words passed Pealim on the first run is the worker's claim, with a 5-word sample behind it.

## Deviations and disclosures

- **Test change:** `TestBareLettersMatchV2` no longer requires new items to be in the v2 snapshot. Every v2 item must still match, and `gen` checks the new items against the frequency list.
- **Added rule:** a unique-gloss rule, so two words cannot share an English meaning as distractors.
- **The audio manifest count is an upper bound.** The only committed data the worker could count against was the legacy word manifest (31 texts). That covers 15 clips, so at most 191 are missing (185 words and 6 letters). The real `audio_assets` table is not committed, so the true number could be lower. The earlier import brought in 123 word clips, and the generator skips texts that already have a clip.
- **Letter clips are a risk.** The TTS model may read a lone letter plus vowel as the letter's name instead of the syllable. Those six clips need a listen in phase 2.
- **Model default.** The app's default TTS model is `eleven_v3`. The spec prefers `eleven_v4`, so set `ELEVENLABS_MODEL=eleven_v4` for phase 2.
- No TTS API was called, no audio was generated, and no key was read or written.
- No personas, timer, trace or independent AC verifier.

## Phase 2: done (2026-10-19)

- 206 clips generated through the `generate_audio` MCP tool on a locally run v3 server with a scratch data dir (model `eleven_v3`, default female voice). 166 generated, 40 skipped (the first 40-clip call was cut off by the 30 s write timeout; the clips were already stored). No failures.
- A second pass over the whole manifest returned 206 `skipped` and 0 generated, so a row exists for exactly each manifest text.
- `testdata/audio-available.json` is committed (650b05b). `TestItemsAudioRealDeck` passes.
- The mp3s and `hebrew.sqlite` are in `C:\Users\BrianNelson\Projects\Hebrew-v3-audio-data` (outside git).
- **Open:** the clips are not on the cluster volume. SPEC-026 adds `hebrew audio-ensure`; SPEC-025 runs it after deploy. The six letter clips have not been listened to.
- Lessons: keep a call to 10 clips (a 40-clip call exceeds the 30 s server write timeout). PowerShell 5 decodes the JSON reply as Latin-1 unless it reads `RawContentStream` as UTF-8.

## Phase 2: what it needed from the operator (historical)

1. `ELEVENLABS_API_KEY` set in the environment of the running server (staging `hebrew-next`), and `ELEVENLABS_MODEL=eleven_v4` if v4 is wanted.
2. Send the texts in `audio-manifest.json` to the `generate_audio` MCP tool, at most 40 per call (about 5 calls). Texts that already have a clip are skipped, so the spend equals the missing count (at most 191 clips).
3. If the clips come from the legacy Node pipeline instead, dry-run `hebrew import-audio --from <tgz> --dry-run` first, as in `docs/import-audio.md`.
4. Commit `internal/placement/testdata/audio-available.json` listing the clip texts that have an `audio_assets` row, so `TestItemsAudioRealDeck` stops skipping and passes.

## Gates

TTS credits are spent in phase 2, with the operator's approval already given at Gate 5. Do not push `v3`: a push publishes an image that may auto-deploy.
