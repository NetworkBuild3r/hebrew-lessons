# SPEC-008 AC Verification Eval Report — Media pipeline: sniff, re-encode, store, archive

**Traceability:** INIT-004/SPEC-008 \
**Verifier:** independent sub-agents that did not author the code, a fresh one per pass · model inherited from the session (see Notes) · temperature 0 \
**Evaluated at:** 2026-10-09 (pass 1 about 17:18Z, pass 2 about 17:38Z, pass 3 about 18:10Z, pass 4 about 19:45Z, pass 5 about 20:15Z) \
**Verdict:** **PASS** on every item (pass 5, final code, real sharp decoder) · **Remediation loops:** 3 (loop 1 hardening; loop 2 fixed the two pass-2 failures; loop 3 fixed the pass-4 `pHYs` gap)

---

## Acceptance-criteria results after the gate (passes 4 and 5)

The operator approved `sharp` (`0.35.5`, exact) at the `dependency_add` gate. The finishing pass added `server/media/codec.js` as the default codec, replaced the two `it.todo` tests with real ones, and fixed the pass-3 Low items. Each verifier was told never to pass ac-1b on an injected codec.

| AC | Pass 4 | Pass 5 (final) | Evidence / gap |
| ---- | ------ | -------------- | -------------- |
| ac-1a | PASS | PASS | SVG named `.png`: `unsupported_type`, no row, no `media/` folder, and sharp never loaded. The result was the same with no options, `null`, `{}`, and `{ codec: null }`. |
| ac-1b | **FAIL (narrow)** | **PASS** | **Pass 4** ran 60 uploads with no codec injected. Each gave one 64-hex file directly in `media/`, re-encoded, and none had EXIF, GPS, camera, text, or ICC data. 953 failure attempts stored nothing. **Gap:** EXIF XResolution/YResolution (314×271 dpi) became stored `pHYs` 12362×10669 px/m, JFIF 17×23 dpi became 669×906, and an input `pHYs` of 0xDEADBEEF/0x0BADF00D was copied byte for byte. **Pass 5**, after the fix, used 12 inputs from 1 dpi to 0xDEADBEEF, all of which gave the identical `pHYs` `000003e8000003e801`. Output chunks were only `IHDR, pHYs, IDAT, IEND`, with no gAMA, cHRM, sRGB, sBIT, bKGD, or text carried over. sharp reports no exif, icc, xmp, iptc, comments, or pages. Orientation 6 rotates, 4096×1 becomes 2048×1, 8×6 stays 8×6, and alpha survives. A 16-bit input is stored as 8-bit. A corrupt IDAT or a truncated JPEG gives `malformed_image` and stores nothing. |
| tr: decoded dimensions | PASS | — | Byte length is checked at `ingest.js` before the sniff and decode. The decoder's width and height are checked in `codec.js` before pixel decode, and again in ingest. Recorded options: `failOn: 'warning'`, `limitInputPixels: 16777216`. sharp is not loaded until an input passes the size and header checks. |
| stored longest side ≤ 2048 | PASS | PASS | 4096² → 2048², 4000×3000 → 2048×1536, 4096×2048 with orientation 6 → 1024×2048. 2048², 8×6, and 1×1 are not enlarged. |
| sec-1 | PASS | — | Called directly, the codec refuses SVG, GIF, TIFF, and AVIF bytes declared as png, jpeg, or webp. Polyglots (SVG in `tEXt`, a JPEG comment, or a WebP chunk; HTML or ZIP appended) were stored only as clean re-encodes. |
| sec-2 | PASS | PASS | A missing `DATA_DIR` under a junction into `login/` was refused before `mkdir`, creating nothing. A `media/` junction into `review/` was refused. |
| sec-3 | PASS | — | A filename and content-type marker on 60 uploads appeared in no file name, file content, SQLite file, or row. |
| mkdir coded error | (Low finding) | PASS | A regular file or a dangling junction at `media/` now gives `store_failed` with fixed text and no path. |

Commands run by pass 5: `node --test test/media-ingest.test.js` → 31 pass, 0 fail, exit 0. `node --test "test/*.test.js"` → 527 pass, 0 fail, 0 skipped, exit 0. `npm run audio:verify` → 65 recordings, exit 0. Pass 4 confirmed that `git diff` on the lockfile adds only sharp 0.35.5 and its own tree.

Security, passes 4 and 5, new and changed code:
- Fixed: the Low `pHYs` copy, and the Low raw `mkdir` error.
- Open Low: the raw fs error from `_archive` creation in `deleteMedia`.
- Accepted Low: about 254 MiB peak memory for the largest allowed image, and no timeout on the final encode from raw pixels.
- No Critical, High, or Medium findings.

## Acceptance-criteria results before the gate (pass 3)

Overall **PASS** requires every AC to pass with concrete evidence. ac-1 was split so the injected test codec could not stand in for a real decoder: ac-1a is what can be shown without a decoder, ac-1b is real decode and re-encode. Each verifier was told to give ac-1b BLOCKED-BY-GATE only if no decoder exists and the default path fails closed, and never to pass it on the strength of the injected codec. Pass 3 wrote throwaway probe scripts outside the repo in addition to running the repo's tests.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1a | SVG named `.png` refused with no file; non-images refused; with no decoder nothing is stored; with a codec injected, exactly one `^[0-9a-f]{64}\.png$` file holding the codec output | PASS | Repo tests at `test/media-ingest.test.js:248`, `:307`, `:322` pass. Probe: an SVG named `.png` and 18 other non-image kinds gave `unsupported_type`, 0 decoder calls, no file, folder, or row. With no codec, 8 valid fixtures × 5 option shapes gave `decoder_unavailable` and stored nothing. With a codec, each upload added exactly one non-database file, `media/<64 hex>.png`, whose SHA-256 equals the codec output. |
| ac-1b | Real decode and re-encode to one single-frame PNG with no metadata | BLOCKED-BY-GATE | `package.json` lists only `@modelcontextprotocol/sdk`; the lockfile (94 packages) and `node_modules` (93 entries) contain nothing image-related. The default path fails closed (`server/media/ingest.js:414-417`). The two decoder tests are `it.todo` (`test:924-925`). |
| ac-2 | Delete writes `_archive/`, checks the hash, then removes the live file; a hash mismatch keeps the live file | PASS | Repo tests `:596`, `:619`, `:639`, `:663` pass. A trace of the real calls showed: read live, copy to `_archive` while live is present, read the archive, unlink live. Four bad copies (different bytes, truncated, one bit flipped, silent no-op) gave `archive_verify_failed` with the live file intact and the bad copy removed. A directory at the destination, `ENOSPC`, and `EEXIST` were refused with the live file kept. A different existing archive gave `archive_conflict`. |
| ac-3 | More than 5242880 bytes, or a header side over 4096, is refused before decode writes anything; the 41st upload in a clock hour for one actor is refused | PASS | Repo tests `:442`, `:467`, `:488`, `:525`, `:540`, `:560` pass. 5242881-byte PNG, JPEG, WebP, SVG, and `Uint8Array` inputs gave `too_large` with 0 decoder calls. Header sides of 4097, 65535, 16384, and 2³¹−1 gave `dimensions_too_large` before decode. On the real clock, the 41st upload was refused and stored nothing; 10:59:59.999 was refused and 11:00:00.000 granted; 45 concurrent uploads stored 40 and refused 5. |
| ac-4 | `mcp` cannot move a row to `approved`; empty alt text cannot be approved | PASS | Repo test `:680` passes. 14 actor kinds other than exactly `admin` were refused and the row stayed `pending` (including `mcp`, `MCP`, `" mcp"`, `new String('admin')`, an array). Empty and whitespace-only alt text gave `alt_text_required`. |
| claim | Invisible-only alt text cannot be approved | PASS | The 12 strings that passed pass 2 are now refused, plus 64 more letterless strings (U+1160, U+00AD, tag characters, variation selectors, bidi controls, U+FFF9, emoji-only, 1999 zero-width spaces). English, a single letter, a digit, pointed Hebrew with RLM/LRM and RLE/PDF, NFD text, Hangul, Arabic, and CJK still approve (`ingest.js:463-465`). |
| claim | A JPEG with no EOI is refused | PASS | `server/media/sniff.js:160` requires an EOI. Nine no-EOI variants (nothing appended, SVG payload, `FF00` stuffing, RST markers, fake APP1, fake COM, trailing `FF`, `FF FF` fill, a second SOI) were refused with 0 decoder calls. No proper prefix of the 7 valid fixtures passes. |
| und-1 | No thumbnail; one upload, one stored file anywhere under `DATA_DIR` | PASS | Every upload adds exactly one file under `DATA_DIR`. Two concurrent uploads with one id gave one success, one `id_taken`, and one file. A failed row insert removes the file. |
| und-2 | Media paths are not under `worksheets/`, `shared/`, `login/`, or any static root | PASS | Checked before and after links resolve (`ingest.js:124-131`, `:181-189`). With a fake root in temp, `DATA_DIR` under `login/` (including a case variant), `shared/`, `review/`, or `worksheets/`, the seed tree with `LESSONS_DIR` elsewhere, `LESSONS_DIR` at `media` or `media/_archive`, an 8.3 alias, and a `\\?\` prefix were all refused with no PNG in a served folder. |
| sec-1 | SVG and non-image bytes store nothing | PASS | As ac-1a; also UTF-16 SVG, gzipped SVGZ, PDF, GIF, BMP, TIFF, ICO, AVIF, ZIP, HTML, RIFF/WAVE, an executable, NUL bytes, and empty input. |
| sec-2 | Stored path stays under `DATA_DIR/media`; the resolved path cannot escape | PASS | `pathInside` requires `^[0-9a-f]{64}\.png$` and the same parent (`ingest.js:211-220`); 32 hostile names were refused. Junctions from `media` to `shared/`, to an outside folder, and to `login/` were refused on write, read, send, approve, and delete. An `_archive` junction was refused on delete with the live file kept. Repo test `:797` ran, not skipped. |
| sec-3 | Original filename not written to disk or the database | PASS | Ingest reads only bytes, actor, id, alt text, prompt, provider, and model (`ingest.js:292-327`). 11 client-filename fields carrying a marker left no trace in any file name or file content under `DATA_DIR`, including the SQLite file, `-wal`, and `-shm`. |
| tr-1 | Byte length before decode; header dimensions before decode and decoder-reported dimensions after; the decoder gets only the validated span | PASS | `ingest.js:406-412` before decode, `:364-365` after; `:418` passes `bytes.subarray(0, header.length)`. In 7 image-plus-trailer cases the decoder got exactly the image bytes. 17 bad decoder outputs were refused and stored nothing. |
| tr-2 | `Content-Type: image/png` and `X-Content-Type-Options: nosniff` | PASS | Checked on a real `http.ServerResponse`: exactly one of each header after pre-setting `image/svg+xml` and `off`; the body equals the stored PNG. |
| tr-3 | No logging of bytes or prompt | PASS | No `console`, `process.std*`, `debuglog`, or `emitWarning` in either file; 0 writes captured across success, refusals, approve, send, and delete; errors carry only `name` and `code`. |
| gate | `package.json` and lockfile unmodified; `audio:verify` passes | PASS | `git diff --stat` and `git status --porcelain` are empty for both files; `audio:verify` OK. |

## Definition of done

- [x] Only a sniffed PNG, JPEG, or WebP reaches the decoder, only its validated span is passed, and only decoder output that passes the stored-PNG check is written — ac-1a, tr-1.
- [ ] Only a **re-encoded** PNG is stored — blocked at the `dependency_add` gate (ac-1b). With no decoder, nothing is stored.
- [x] Delete archives before unlink, and a hash mismatch keeps the live file — ac-2.
- [x] `node --test test/media-ingest.test.js` exits 0 on the fixture set, including the SVG-named-`.png` refusal and the failed-archive case.
- [x] Security criteria — sec-1, sec-2, sec-3.
- [x] `npm test` and `npm run audio:verify` pass; `package.json` and `package-lock.json` are unchanged.

## Tests run by the verifier (pass 3)

```text
node --test test/media-ingest.test.js
→ tests 24, pass 22, fail 0, skipped 0, todo 2, exit 0
npm test
→ tests 451, pass 449, fail 0, todo 2, exit 0
→ rerun after other workers changed the tree (files under test unchanged): tests 476, pass 474, fail 0, todo 2, exit 0
npm run audio:verify
→ audio:verify OK — 65 recordings accounted for, exit 0
git diff --stat -- package.json package-lock.json
→ no output, exit 0
```

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-008 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| 1 | None. Pass 1: PASS, ac-1b BLOCKED-BY-GATE. | Low notes: the read path did not re-check the media folder's real location; zero-width characters counted as alt text; a rate-limit test could flake at an hour boundary; two assertions depended on test order; two refusal tests did not check rows; the one-file check looked only at the top of `media/`. Security pass 1 suggested handing the decoder only the validated span. | `server/media/ingest.js`: real-path check on reads; whitespace and format characters do not count as alt text; non-finite clock falls back to `Date.now()`; the decoder gets the validated span. `server/media/sniff.js`: `readImageHeader` returns `length`. Tests: mocked `Date`, whole-`DATA_DIR` comparisons, row checks, read-path junction case, validated-span test. | Pass 2: **FAIL** on ac-4 (invisible-only claim) and tr-1 |
| 3 | ac-1b (pass 4) | Input resolution (EXIF XResolution/YResolution, JFIF density, input `pHYs`) reached the stored PNG's `pHYs`, which is "other metadata" under ADR-0004 §5. Low: a file or dangling link at `media/` threw a raw fs error with the path. | `codec.js`: decode, orient, and resize into 8-bit raw pixels, then encode the PNG from those pixels alone. `ingest.js`: `mkdir` failure is `store_failed`. Tests: "the input's resolution does not reach the stored PNG"; a file at `media/` in the partial-write test. Both mutation-checked. | Pass 5: **PASS** |
| 2 | ac-4 (claim), tr-1 | ac-4: U+3164, U+115F, U+FFA0, U+034F, U+FE0F, U+E0100, U+17B4, U+180B, U+2800, U+0000, U+0007, a lone U+0301 could be approved as alt text. tr-1: a JPEG with no EOI used the whole buffer as its span, so appended text reached the decoder. Also noted: a caller-supplied clock could prune other actors' counters; size tests looked only at the top of `media/`; delete was not tested through a linked `media/`. | `ingest.js`: alt text needs a letter or digit after removing `Default_Ignorable_Code_Point` characters; per-actor hour buckets only move forward and are never pruned by another actor's call. `sniff.js`: a JPEG with no EOI is `malformed_image`. Tests: every probed character, pointed Hebrew still approves, JPEG with no EOI, clock manipulation, whole-`DATA_DIR` checks in the size tests, approve and delete through a linked `media/`. | Pass 3: **PASS**, ac-1b BLOCKED-BY-GATE |

Pass 1 ran `node --test test/media-ingest.test.js` (22 tests: 20 pass, 2 todo), `npm test` (433: 431 pass, 2 todo), and `npm run audio:verify` (65 recordings). Pass 2 ran the same commands (23: 21 pass, 2 todo; 434: 432 pass, 2 todo; 65 recordings) and its own probes. Each loop's fixes were mutation-checked: deliberately undoing each fix made at least one test fail.

---

## Notes

- Superseded on 2026-10-09: the gate cleared (`sharp` 0.35.5), the two `it.todo` tests became real tests, and pass 5 passed ac-1b on the real decoder. The spec is `complete`. The pass-3 Low items below were fixed in the finishing pass, except where the completion summary lists them as open.
- No AC was passed by weakening a test or the criterion. Both loops only added assertions.
- Pass 3 items that do not change a verdict, left for the post-gate re-dispatch (which must re-open and re-verify this module anyway): a write that fails partway leaves an unreferenced partial file (`ingest.js:423-427`); a later `options.now` opens a new hour early, so adapters must never pass a request-derived clock (the same trust as `server/auth/throttle.js`); the media folder is created before its real-path check, so a `DATA_DIR` that reaches a served folder through an alias gets an empty `media/` folder there before the refusal; an `unlink` failure after a verified archive copy surfaces as a raw `EBUSY` with both files kept; repo tests do not exercise the second-SOF, hierarchical-frame, or second-JPEG-after-EOI branches, `login/` and `shared/` overlap, or later clock values. Pass 3's probes covered each of these branches. Outside this spec: `server/paths.js` compares `DATA_DIR` to served roots without resolving aliases.
- All three verifier passes and all three security reviews ran on the session's inherited model. The SDD table names `composer-2.5-fast` for the cursor verifier; this harness allows a different model only when the user names one. Each prompt forbade edits because the Task tool has no readonly flag.
- Tests use a temporary `DATA_DIR`. They do not call ElevenLabs or OpenAI, and they do not write under the lessons tree. Pass 3's probe scripts were kept in the OS temp folder.
