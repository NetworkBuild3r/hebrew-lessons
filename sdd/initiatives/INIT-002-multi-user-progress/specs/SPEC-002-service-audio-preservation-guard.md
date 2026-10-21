# SPEC-002 · Service: Audio preservation guard and checksum inventory

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-002
title: Audio preservation guard and checksum inventory
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3 hours
blocked_by: []
blocks: [SPEC-007]
```

---

## Description

**Context:** the operator's hard rule is "don't lose any of our audio already recorded". Today, `server/audio.js` never overwrites word clips (`_audio/words/`). It does overwrite lesson sentence and story clips in place (`writeFileSync`) when their text hash changes or `forceSentences` is set. Its legacy migration also renames and unlinks leftovers in `shared/audio-cache/words`. 37 of the 65 recordings are untracked in git. A browser lesson editor (SPEC-007/017) will make regeneration easier, so both protections must exist first.

**Scope (in):**
- **Archive-before-overwrite in `server/audio.js`.**
  - Before any write to a path that already holds a clip, copy the existing file to `<lesson>/audio/_archive/<basename>.<first 12 hex of its SHA-256>.mp3`.
  - If that archive name already exists with the same hash, skip the copy. An archive file is never overwritten.
  - Log one line per archived clip.
  - The legacy `shared/audio-cache` migration deletes a source only after the destination exists with the same byte length and hash.
- **`scripts/audio-inventory.js` with two subcommands:**
  - `npm run audio:inventory [-- --out <file>]` walks `LESSONS_DIR` (and `shared/audio-cache` if present). It writes a JSON inventory of every `.mp3`/`.m4a`/`.ogg`/`.wav`: relative path, bytes, SHA-256. The default output is `data/audio-inventory/<timestamp>.json`, outside the served tree.
  - `npm run audio:verify [-- --against <file>]` checks every inventoried file. Each must still exist with the same hash, or have an `_archive/` copy with that hash. It exits non-zero and lists any file that is neither. The default `--against` is the newest inventory.
- Tests for both.

**Scope (out):** moving or renaming any existing clip; changing audio URLs; an audio admin UI.

---

## Understood (agent's read of the intent)

- [ ] und-1: Nothing in this spec deletes or moves an existing recording. Archiving only adds copies.
- [ ] und-2: The inventory and verify pair is the gate that every later lesson-touching spec runs (initiative §6).

## Acceptance Criteria

- [ ] ac-1 (REQ-012): regenerating a lesson sentence clip whose text changed leaves the previous bytes in `audio/_archive/` with a matching SHA-256, and writes the new clip at the original path. Running it twice creates no second archive copy of identical bytes.
- [ ] ac-2 (REQ-012): `audio:verify` passes on an unchanged tree. It fails, naming the file, when a clip is deleted or its bytes change with no archive copy. It passes when the old bytes are in `_archive/`.
- [ ] ac-3 (REQ-012): every `unlinkSync`, `rmSync`, and `renameSync` call reachable from `server/audio.js` is preceded by a verified copy, or touches only temp files. A test covers the legacy migration path.
- [ ] ac-4: the inventory is written outside `LESSONS_DIR` and never contains file contents, only paths, sizes, and hashes.
- [ ] ac-5: running `audio:inventory` on the current repo lists 65 recordings. Its hashes match the planning backup's `SHA256SUMS.txt` for the same paths.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Archive location | inside each lesson's own `audio/_archive/` so it travels with the lesson and stays served-but-gated like other lesson files | resolved | yes — plan default; ADR-0002 item 12 may override |

---

## Deliverables

- [ ] `server/audio.js` (archive-before-overwrite; verified legacy migration)
- [ ] `scripts/audio-inventory.js`; `package.json` scripts `audio:inventory`, `audio:verify`
- [ ] `.gitignore` / `.dockerignore`: `data/` (if not already added by SPEC-011)
- [ ] Tests: `test/audio-preservation.test.js` (temp `LESSONS_DIR`, fake TTS, no network)

---

## Technical Requirements (domain-specific)

- [ ] No network calls in tests. Reuse the fake-TTS pattern from `test/generate-audio-cache.test.js`.
- [ ] Hashing uses `node:crypto` streaming, so large files don't load whole.
- [ ] Never print or read `.env`. The key stays in `process.env` only.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`. Run `npm run audio:inventory`, then `npm run audio:verify`, on the real tree; both pass.
- **Rollback plan:** revert `server/audio.js`, delete the new script and test. Archived copies are additive and can stay.

---

## Security

- **Sensitivity:** standard. Triggers matched: filesystem writes under `LESSONS_DIR`.
- **Review depth:** standard. Path containment: archive and inventory paths are resolved and checked to stay under `LESSONS_DIR` and `data/`.

---

## Verification Strategy

- **Claim:** no code path in the audio pipeline can remove a recording's bytes without leaving a verified copy, and the verify command detects any loss.
- **Check + executor:** `mechanized`. `node --test test/audio-preservation.test.js` runs against a temp tree with a fake TTS. Then `npm run audio:inventory && npm run audio:verify` runs on the repo tree.
- **Pass condition:** all tests pass. On the repo tree, verify exits 0 with 65 recordings listed. A test that deletes one clip makes verify exit non-zero and name that clip.

---

## Integration Points

- Consumed by SPEC-007 (editor audio generation) and by every lesson-touching spec as the §6 gate.
- **Provenance tag:** `INIT-002/SPEC-002`
