# SPEC-008 Completion Summary — Media pipeline: sniff, re-encode, store, archive

**Traceability:** INIT-004/SPEC-008
**Status:** complete. The operator approved `sharp` at the `dependency_add` gate. [SPEC-008 finish](924c3299-4fe6-4c15-8ee1-4f51e3ea9221) added the codec. Independent pass 5 passed ac-1b on the real decoder.
**Domain:** service
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/media/ingest.js` — ingest, upload rate limit, path guards, archive-before-delete, approval, media response helpers. `sharp` is now the default codec.
- [x] `server/media/sniff.js` — magic bytes, header dimensions, multi-frame structure, and the allowlist for stored PNG chunks. Unchanged in the finishing pass.
- [x] `server/media/codec.js` — the sharp codec behind the `decodeAndReencode` seam. New in the finishing pass.
- [x] `test/media-ingest.test.js` — 31 tests on a temporary `DATA_DIR`, all real (no `todo`). No network.
- [x] `test/fixtures/media/` — 18 tiny synthetic fixtures. Unchanged.
- [x] `package.json` / `package-lock.json` — `"sharp": "0.35.5"` (exact). The operator ran `npm install sharp --save-exact` at the gate.

## Acceptance Criteria

- [x] ac-1: a file that starts with `<svg` and is named `.png` is refused and leaves no file. With the real sharp codec, PNG, JPEG, and WebP each become exactly one PNG directly in `DATA_DIR/media/`, named `^[0-9a-f]{64}\.png$`. The PNG is re-encoded from the decoded pixels alone, so no EXIF, GPS, camera, text, ICC, or resolution data from the input is copied. If decode or re-encode fails, nothing is stored: no file and no row.
- [x] ac-2: delete copies to `_archive/` while the live file exists, compares SHA-256, then unlinks. A mismatch keeps the live file and removes the bad copy. If unlinking fails after a verified copy, the error is now the coded `delete_failed` and both files are kept.
- [x] ac-3: more than 5242880 bytes, or a header width or height over 4096, is refused before the decoder is called and before any write. The 41st upload by one actor in a UTC clock hour is refused.
- [x] ac-4: actor kind `mcp` cannot approve. Alt text with no letter or digit cannot be approved.

## AC Verification (independent)

There were five passes, each by a fresh sub-agent that did not write the code:

- **Pass 1** passed.
- **Pass 2** failed two items: alt text made only of invisible characters, and JPEG trailing data. Both were fixed.
- **Pass 3** passed everything except ac-1b, which was blocked at the gate.
- **Pass 4** ([SPEC-008 ac-1b verifier](4a894db0-093c-47d7-8794-9a3e5181cd59)) ran after the gate and failed one narrow part of ac-1b. sharp carried the input's resolution into the stored PNG's `pHYs` chunk: EXIF XResolution/YResolution, JFIF density, or an input `pHYs` copied byte for byte. That is "other metadata" under ADR-0004 §5. The codec now decodes, orients, and resizes into 8-bit raw pixels, then encodes the PNG from those pixels alone.
- **Pass 5** ([SPEC-008 re-verifier](1edcd1ed-f40e-4dae-b14d-fe51409bd091)) passed every row on the real sharp path. Every stored PNG now has the same `pHYs` value, `000003e8000003e801`, across 12 inputs from 1 dpi to 0xDEADBEEF.

Overall: **PASS** · remediation loops: 3 (two before the gate, one after).
**Full eval report:** [`SPEC-008-eval-report.md`](SPEC-008-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[dependency_add]` (approved) · security_review=`required` · domain=`service` · autonomy_level=`auto_tests` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict (pass 5; ac-2..4 from pass 3, unchanged code paths re-run green) | Evidence |
| ---- | ------- | -------- |
| ac-1a | PASS | Test "ac-1: a file that starts with <svg and is named .png is refused and leaves no file". Probe: `unsupported_type`, no row, no `media/` folder, and sharp never loaded. |
| ac-1b | PASS | The probes ran the default codec with no codec injected. Each upload added exactly one 64-hex file directly in `media/`. The stored bytes always differed from the input. The only chunks written were `IHDR, pHYs, IDAT, IEND`, with a constant `pHYs`. sharp reports no `exif`, `icc`, `xmp`, `iptc`, `comments`, or `pages`. A byte scan found no `Exif`, camera strings, TIFF headers, or GPS bytes. EXIF orientation 6 rotates the image. 4096×1 is stored as 2048×1, and 8×6 stays 8×6. WebP alpha survives. A truncated or corrupt IDAT, or a truncated JPEG, gave `malformed_image` and stored nothing. Tests: "ac-1: with the default sharp codec…", "ac-1: exif-gps.jpg is stored with no eXIf chunk…", "ac-1: the input's resolution does not reach the stored PNG", "ac-1: a buffer the sniff accepts but the decoder rejects stores nothing". |
| tr: decoded dimensions | PASS | The byte length and header are checked before decode. The codec reads the decoder's own width and height before decoding pixels and refuses a side over 4096. It checks this itself because `limitInputPixels` alone lets 4097×1 through. Ingest checks the reported values again. |
| ac-2 | PASS | Pass 3 evidence stands. The new test "ac-2: an unlink failure after a verified archive copy is a coded error…" covers the `delete_failed` path. |
| ac-3 | PASS | Pass 3 evidence stands. New test: "ac-3: a later clock value opens only that actor's next hour…". |
| ac-4 | PASS | Pass 3 evidence stands. Code unchanged. |
| sec-1..3 | PASS | Pass 4. Called directly, the codec refuses SVG, GIF, TIFF, and AVIF bytes when they arrive declared as png, jpeg, or webp. Polyglots were stored only as clean re-encodes. A filename marker appeared in no file name, file content, or row. Pass 5 re-checked sec-2. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-008`
- **Title:** Media ingest pipeline with sharp decode and re-encode: sniff, limits, upload rate limit, safe storage, archive-before-delete, approval, response helpers
- **Files changed (finishing pass):**
  - `server/media/codec.js`: new, 109 lines.
  - `server/media/ingest.js`: default codec, `decoder_unavailable` pass-through, the Low fixes, and the coded `mkdir` failure.
  - `test/media-ingest.test.js`: 24 tests became 31; the 2 `todo` tests are now real.
  - The spec file and this report.
- **Change status:** proposed (no commit, per dispatch)
- **Reversibility:** run `npm uninstall sharp` and delete `server/media/codec.js`. Ingest would then fail on the missing require, so also restore the "no default codec" line, or simply stop calling ingest. Images already stored stay valid PNGs. No migration.

---

## Tests

| Test file | New tests (finishing pass) | Total |
| ------------- | --------- | ----- |
| `test/media-ingest.test.js` | 8 new; 2 `todo` and 1 no-decoder test replaced by real tests | 31 |

**All tests passing:** Yes.

- `node --test test/media-ingest.test.js`: 31 pass, 0 fail, 0 skipped, 0 todo, exit 0.
- `npm test` (`node --test "test/*.test.js"`): 527 pass, 0 fail, 0 todo, exit 0. Other workers added tests in parallel; it was 476 before this pass.
- `npm run audio:verify`: 65 recordings, exit 0.
- No coverage tool is configured.

**Node 22 and Alpine:** `docker build` of the repo's `Dockerfile` (`node:22-alpine`, `npm ci --omit=dev`) installed `@img/sharp-linuxmusl-x64` and `@img/sharp-libvips-linuxmusl-x64` from the committed lockfile. Inside that image, `require('sharp')` loaded (Node 22.23.3, sharp 0.35.5, libvips 8.18.7), and the media test file passed 30/30 with nothing skipped. That run predates the `pHYs` fix and its added test. The temporary image was removed. `npm ci` also installs the optional `@img/sharp-wasm32` fallback, which is unused while the native binary loads. The lockfile lists `linuxmusl-arm64` too.

**Mutation check:** 22 deliberate breaks, applied one at a time; every one made at least one test fail, and the sources were restored byte for byte. The breaks were:

- codec: format check, frame check, and side check removed.
- codec: `autoOrient` removed, metadata kept, enlargement allowed.
- codec: decode errors not mapped, load failure not coded, bad limits not refused.
- codec: PNG encoded straight from the input pipeline (the `pHYs` leak).
- ingest: sharp not the default, `decoder_unavailable` not passed through.
- ingest: partial file kept, existing file removed on EEXIST, unlink error not coded.
- ingest: no check before `mkdir`, the old `realOrResolved`, raw `mkdir` error.
- sniff: second SOF, differential SOF, DHP, and second JPEG after EOI each accepted.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| dependency_add | one_way | **yes** — operator, 2026-10-09, `sharp` (`npm install sharp --save-exact`) | media tests pass; `npm test` pass; `audio:verify` pass; independent verification PASS (pass 5); security 0 Critical / 0 High / 0 Medium | The finishing pass did not re-run the gate or add any other dependency. The lockfile gains `sharp` and its runtime dependencies (`@img/colour`, `detect-libc`, `semver`). It also gains `@emnapi/runtime` and `tslib` for the optional wasm build, plus 26 optional `@img/sharp-*` platform packages. Licenses: sharp Apache-2.0, libvips binaries LGPL-3.0-or-later. |

---

## Security Review

- **Sensitivity / depth:** required · thorough.
- **Method:** three `security-review` passes before the gate. After the gate, passes 4 and 5 each did a security pass on the new and changed code. They used the verifier sub-agent because this harness reserves the dedicated security-review agent type for explicit user requests.
- **SAST and secret scan:** none configured.
- **Findings, new and changed code:** Critical 0 · High 0 · Medium 0 · Low 3 open (below). The Low `pHYs` finding from pass 4 is fixed, and so is the Low raw `mkdir` error.

| Finding | Severity | Location | Status |
| ------- | -------- | -------- | ------ |
| Input resolution copied into stored `pHYs` (weak device fingerprint, 8-byte side channel) | Low | `codec.js` re-encode | **Fixed.** The PNG is encoded from raw pixels. Test: "the input's resolution does not reach the stored PNG". |
| A file or dangling link at `DATA_DIR/media` threw a raw fs error containing the server path | Low | `ingest.js` `ensureMediaDir` | **Fixed.** It is now the coded `store_failed`. |
| `fs.mkdirSync` for `_archive` in `deleteMedia` can still throw a raw fs error with the path | Low | `ingest.js` `deleteMedia` | Open. Nothing is deleted when it happens. SPEC-011 must not echo raw error messages. |
| The largest allowed image (4096², 16-bit RGBA) peaked at about 254 MiB during decode | Low (accepted by the limits) | codec decode | Accepted. The pixel cap, a 20 s timeout, and 40 uploads per actor per hour bound it. Compressed-chunk bombs stayed at about 3 MiB. |
| No `.timeout()` on the final encode from raw pixels | Low | `codec.js` final encode | Accepted. The input is at most 2048×2048×4, already decoded. |

- **Positive observations:**
  - sharp loads lazily, so a missing native binary refuses uploads with `decoder_unavailable` instead of stopping the server.
  - The codec refuses when the decoder's format differs from the sniffed format, because sharp would otherwise decode SVG, GIF, TIFF, or HEIF.
  - Bad limits are refused, so `limitInputPixels` can never become 0 or false.
  - `failOn: 'warning'` is used.
  - Every error carries fixed text and a `code`. Nothing is logged, including at the file-descriptor level.
  - A failed write removes its partial file, except on `EEXIST`, which never deletes another row's file.
  - `media/` is checked where it will land before `mkdir`.
- **Defence-in-depth option, not taken:** sharp 0.35's `sharp.block()` can switch off loaders other than PNG, JPEG, and WebP for the whole process.

---

## Files Changed

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/media/codec.js` | Created. Sharp codec: lazy load, format, frame, and side checks before pixel decode, `failOn: 'warning'`, pixel cap, EXIF orientation, resize inside 2048 without enlargement, 20 s timeout, PNG encoded from 8-bit raw pixels. | `INIT-004/SPEC-008` |
| `server/media/ingest.js` | Modified. sharp is the default codec when `options.codec` is null or undefined. `decoder_unavailable` passes through. A partial file is removed after a failed write (never on `EEXIST`). `realOrResolved` resolves a missing path through its nearest existing ancestor. `media/` is checked before and after `mkdir`, and a `mkdir` failure is `store_failed`. An unlink failure is `delete_failed`. | `INIT-004/SPEC-008` |
| `test/media-ingest.test.js` | Modified. Real decoder tests: fixtures, metadata, orientation, resolution, decoder rejection, the codec's own checks, decoder unavailable. Low-fix tests: partial write, name collision, file at `media/`, unlink failure, `login/` and `shared/` folders, directly and through junctions. Also JPEG multi-frame branches and later clock values. | `INIT-004/SPEC-008` |
| `server/media/sniff.js`, `test/fixtures/media/*` | Unchanged in the finishing pass (created earlier). | `INIT-004/SPEC-008` |
| `package.json`, `package-lock.json` | `sharp` 0.35.5 added by the operator at the gate. | operator (gate) |
| `sdd/.../specs/SPEC-008-service-media-pipeline.md` | Decoder note resolved, aud-1, deliverable, decoded-dimension requirement, ac-1 note. The orchestrator had already set `status: complete`. | `INIT-004/SPEC-008` |
| `sdd/.../completion-summaries/SPEC-008-eval-report.md`, this file | Updated for passes 4 and 5. | `INIT-004/SPEC-008` |

---

## Notes for the specs that call this module

- **SPEC-011 and SPEC-012:**
  - Call `ingestImage({ bytes, id, actor, altText })`. Never pass a caller-supplied `codec` or `now`, and never pass `deleteMedia`'s second argument.
  - New and changed codes:
    - `decoder_unavailable` means sharp could not load, for example a native binary missing from the image. Treat it as a server error, such as 503.
    - `delete_failed` means the verified archive copy and the live file are both kept, and a retry is safe.
    - `store_failed` now also covers a `media/` folder that cannot be created.
  - Never echo raw error messages. One raw fs error path remains, at `_archive` creation.
  - Pass `actorKind: 'admin'` or `'mcp'` to `approveMedia` and `deleteMedia`. This module writes no audit rows.
- **SPEC-009:** pass the provider's bytes through `ingestImage` with `promptText`, `provider: 'openai'`, `modelId: 'gpt-image-1'`. Generated images are re-encoded like uploads.
- **SPEC-017 / SPEC-020:** the backup set is `DATA_DIR/media/` and `DATA_DIR/media/_archive/` with the SQLite file. The image needs no compiler; `npm ci` installs the musl binaries from the lockfile.

---

## Deviations from the Spec

- Git commit, push, and PR were not performed, per dispatch.
- The persona file `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo, and none was invented.
- **The stored PNG is always 8-bit, sRGB, with a constant `pHYs`.** It is encoded from decoded raw pixels, so nothing from the input's metadata can reach it. A 16-bit input is stored as 8-bit, and an embedded ICC profile is converted to sRGB, not kept.
- **sharp loads lazily** on the first upload, so the server starts even if the native binary is missing.
- **20-second decode timeout** (`sharp().timeout`) and `failOn: 'warning'` are hardening the spec did not ask for. Any sharp error, including a timeout, is `malformed_image`, as the dispatch said.
- **`{ codec: null }` now means the default**, the same as omitting it. An object without a `decodeAndReencode` function still gives `decoder_unavailable`.
- **Header dimensions are still checked before decode**, and the decoder's own dimensions after metadata, before any pixel is decoded.
- Earlier deviations still hold:
  - Delete keeps the media row.
  - Refused attempts count toward the hourly limit.
  - Alt text needs a letter or digit.
  - The caller supplies the media id.
  - No audit rows here.
  - The decoder gets only the validated span.
- **Low items fixed in the finishing pass:**
  - The partial file left after a failed write.
  - `media/` created before its real-path check.
  - The raw `EBUSY` on unlink.
  - The missing repo tests for the second SOF, hierarchical and differential frames, a second JPEG after EOI, the `login/` and `shared/` folders, and later clock values.
- **Open Low items:**
  - The raw fs error from `_archive` creation.
  - Outside this spec, `paths.assertDataDirNotServed` still omits `login/` and compares paths without resolving aliases. That is a follow-up for the INIT-002 guard.
- **Reviewer models:** the verifier passes ran on the inherited session model. The SDD table names `composer-2.5-fast`, but this harness allows a different model only when the user names one. Each prompt forbade edits.
- Node v26 ran the local tests. The Node 22 Alpine container run covers the project standard.
- During `npm test`, other suites rewrite `worksheets/_review/vocab-index.js` and `worksheets/_audio/words/catalog.js` with identical content. The media test alone touches neither (checked).
- Another worker added `server/media/openai-images.js` (SPEC-009) during this pass. This spec did not touch it.
- `INIT-004-lesson-system-build-initiative.md` line 98 still calls the decoder package "Open". This pass left the initiative file to the orchestrator.

---

## Effort Tracking

**Agent session time:** not recorded. The timer script is absent and owned by the orchestrator.
**Spec effort estimate:** 5 hours
