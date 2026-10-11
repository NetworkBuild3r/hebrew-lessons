# SPEC-008 · Service: Media pipeline

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-004
title: "Media pipeline: sniff, re-encode, store, archive"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: [dependency_add]
reversibility_class: one_way
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-006]
blocks: [SPEC-009, SPEC-011, SPEC-012, SPEC-017]
```

---

## Description

**Context:** ADR-0004 §5–§6 and §8–§10. Bytes live in `DATA_DIR/media/` and archives in `DATA_DIR/media/_archive/`. The plan stores a slug, never a path.

**Scope (in):**

- Accept PNG (`89 50 4E 47`), JPEG (`FF D8 FF`), or WebP (`RIFF` … `WEBP`) by bytes. Ignore the client filename and `Content-Type`. Refuse SVG, including a `.png` name.
- Refuse a buffer the decoder rejects, more than one frame, animated PNG, animated WebP, or multi-frame JPEG.
- Limits, stated in ADR-0004 §5: 5242880 input bytes, 4096 pixels on either side, stored PNG longest side 2048, stored name 32 cryptographic random bytes in hex plus `.png`, 40 uploads per actor per clock hour.
- Re-encode to a single-frame PNG. Do not copy EXIF, GPS, or other metadata. If re-encode fails, store nothing.
- Delete copies to `_archive/`, verifies the copy, then removes the live file. Admin only. Dropping a lesson reference does not delete the file.
- Status starts `pending`. Alt text is stored. Approval is a separate function the HTTP layer will call: it refuses while alt text is empty, and it is not available to the `mcp` actor (the route spec enforces the actor; this module takes an explicit actor kind and rejects `mcp`).

**Scope (out):** the HTTP routes (SPEC-011); the OpenAI call (SPEC-009), which must pass its bytes through this ingest; choosing the decoder library in this plan.

**Open:** ADR-0005 Option A adds no direct dependency and names no PNG/JPEG/WebP decoder. Magic-byte refusal can ship with no package. Re-encode does not ship until the operator approves one library at the `dependency_add` gate. This spec does not name the package. *(Resolved 2026-10-09: the operator approved `sharp` at the gate, pinned at `0.35.5`. `server/media/codec.js` uses it.)*

---

## Understood (agent's read of the intent)

- [x] und-1: There is no second thumbnail file. The stored PNG is the image ADR-0004 §5 describes.
- [x] und-2: Media paths are not under `worksheets/`, `shared/`, or any static root.

## Acceptance Criteria

- [x] ac-1 (REQ-006): a PNG, a JPEG, and a WebP fixture within limits become one PNG under `DATA_DIR/media/` whose name matches `^[0-9a-f]{64}\.png$`. A file that starts with `<svg` and is named `.png` is refused and leaves no file. *(2026-10-09: operator approved `sharp@0.35.5`; `server/media/codec.js` is the default. Orchestrator re-ran `node --test test/media-ingest.test.js` → 30 pass / 0 todo. Independent pass 4 found input resolution copied into `pHYs`. The stored PNG is now encoded from decoded pixels alone, and pass 5 passed ac-1b on the real sharp path; the file now has 31 tests.)*
- [x] ac-2 (REQ-006): a delete writes `_archive/`, checks the hash, then removes the live file. If the archive copy's hash differs, the live file remains.
- [x] ac-3 (REQ-006): an oversize buffer (byte length or dimension) is refused before decode writes anything. The 41st upload in an hour for one actor is refused.
- [x] ac-4 (REQ-006): `mcp` cannot move a row to `approved`. An empty alt cannot be approved.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Decoder package | Open at the gate. Magic-byte checks proceed. Re-encode waits for the operator's named package. This plan does not choose one. | resolved | yes — the operator approved `sharp` (`0.35.5`, exact) at the `dependency_add` gate on 2026-10-09 |

> **Readiness** = 100. The library name is an execution gate, not an ambiguous product choice.

---

## Deliverables

- [x] `server/media/ingest.js` (name may follow the repo's folder style if `server/media/` is wrong at execution; the module is not `server/plans/`). Byte checks are in `server/media/sniff.js` beside it. Decode and re-encode are in `server/media/codec.js` (sharp), the default codec.
- [x] Tests: `test/media-ingest.test.js` using fixture bytes in the test file's folder (`test/fixtures/media/`). No network.

---

## Technical Requirements (domain-specific)

- [x] Measure byte length before decode. Measure width and height from the decoded image. *(Byte length and header dimensions are checked before decode. The sharp codec reads the decoder's own width and height before any pixel is decoded and refuses a side over 4096; ingest checks the reported values again.)*
- [x] Response helpers set `Content-Type: image/png` and `X-Content-Type-Options: nosniff`. The route spec calls them.
- [x] Do not log file bytes or the prompt.

---

## Gates & Controls

- **Autonomy level:** `auto_tests` until a package is required. Stop and ask at `dependency_add` before editing `package.json`.
- **Gate actions triggered:** `dependency_add`.
- **Blocking checks:** `node --test test/media-ingest.test.js` and `npm test`. `npm run audio:verify` (this spec must not write under the lessons tree).
- **Rollback plan:** stop calling ingest. Delete test files under the temporary `DATA_DIR`. Do not delete `_archive/` copies that a real admin already made.

---

## Security

- **Sensitivity:** required. Triggers matched: untrusted upload, metadata, path location, polyglot files.
- **Security acceptance criteria:**
  - [x] SVG and non-image bytes store nothing.
  - [x] Stored path is under `DATA_DIR/media` and the resolved path cannot escape it.
  - [x] Original filename is not written to disk or the database.
- **Review depth:** thorough · **focus areas:** file upload, path traversal, content-type confusion.

---

## Verification Strategy

- **Claim:** only a sniffed, re-encoded PNG is stored, and delete archives before unlink.
- **Check + executor:** `mechanized` — `node --test test/media-ingest.test.js`.
- **Pass condition:** exit 0 on the fixture set, including the SVG-named-png refusal and the failed-archive case that keeps the live file.
- **Test levels:** unit and filesystem integration in a temporary directory. No property-based fuzzer is required; the magic-byte table is finite. No coverage tool is configured; the bar is this file plus `npm test`.
- **Test data:** tiny generated PNG/JPEG/WebP fixtures committed beside the test, plus a one-line SVG. No photo of a person.

---

## Integration Points

- SPEC-009's successful provider response is ingested here. SPEC-011 authorizes the caller first.
- Produces **Change:** media ingest module.
- **Provenance tag:** `INIT-004/SPEC-008`
