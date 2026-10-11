# SPEC-002 AC Verification Eval Report — Plan validator: JSON Schema and the research-rule registry

**Traceability:** INIT-004/SPEC-002 \
**Verifier:** independent sub-agent that did not author the code · model `cursor-grok-4.5-high-fast` (`ac_eval.verifier_model` → §6 `model_fast` on cursor) · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-09, about 17:15Z (the verifier returned before 17:16:32Z) \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence. Transcribed from the verifier's returned table; line numbers are the verifier's citations.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | The four samples validate; `anal-sex-vocab` and `shai-date-indirect-speech` return N1 as warn under `migration.legacy`; `my-morning` without `migration.legacy` and with a ninth new word returns N1 as block | PASS | Verifier's own scratch script: all four samples `ok: true`, 0 blocks; N1 warn at `/units/0/lessons/0/newWords` on both legacy lessons; `my-morning` plus a ninth word warns while legacy and blocks once `migration.legacy` is removed. Tests `all four samples return no block`, `anal-sex-vocab and shai-date-indirect-speech return N1 as warn…`, `my-morning without migration.legacy and with a ninth new word returns N1 as block` passed. Implementation `checkNewWordCap`, `server/plans/validate.js:654-661`; registry `legacySeverity`, `:29-33`. |
| ac-2 | Spike broken cases are rejected for their own reason; a well-formed media id outside the four spike pictures is not | PASS | Scratch: tag-like title → `html` at `/title`; missing alt → `N8`; 4001 characters → `N11` at `/subtitle`; URL media id → `schema` block on the `mediaId` path; missing title → `schema` "missing required field: title"; `not-a-real-picture` → `ok: true`, no finding on the image. Six tests under `ac-2: the spike broken cases are rejected for that reason` passed (`test/plan-validate.test.js:127-179`). |
| ac-3 | N12 blocks a word list on `fluency-pass`, warns beside new words; the café unit does not warn; each legacy lesson does | PASS | Scratch: `words` on the café fluency block → N12 block; a fluency block beside new words → N12 warn only; café → no N12; my-morning and anal-sex-vocab warn on blocks 2 and 3, shai on block 2. Suite `ac-3: …` passed (`test/plan-validate.test.js:181-220`). Implementation `checkFluency`, `server/plans/validate.js:678-688`, and the schema mapping at `:380-386`. |
| ac-4 | `server/plans/validate.js` does not contain `coffee-cup`, `water-glass`, `milk`, or `sugar` as an allow-list | PASS | Source scan: none of the four strings, no `spikes` reference, only `require('fs')` and `require('path')`. Test `the module source does not contain the spike picture ids` passed. The spike's `MEDIA_IDS` remains at `spikes/skeleton/validate.js:17` and is not imported. |
| security | A string that starts an HTML tag is a block | PASS | Scratch: `<b>x</b>` → `html`, severity block. Schema `plainText.not` and `altText.not` are `<\s*[A-Za-z/!]` (`lesson-plan.schema.json:79,85`); registry `html` is block (`validate.js:142-145`). Suite `security: markup and audio paths` passed. |
| security | An absolute or scheme-bearing `audioPath` is a block | PASS | Scratch: `/x.mp3`, `https://e.example/x.mp3`, `//e.example/x.mp3` → `audio-path` block each. Schema pattern `lesson-plan.schema.json:87-91`; mapping `validate.js:370-374`. Test `an absolute, scheme-bearing, or escaping audioPath is a block` passed. |

## Definition of done

- [x] Pure function of a document: no database, no filesystem except the schema file, no network — only I/O is `fs.readFileSync(SCHEMA_PATH)` in `loadSchema` (`validate.js:289-291`); requires only `fs` and `path`; test `requires only fs and path: no database or network` passed.
- [x] Return value separates `blocks`, `warnings`, `informs`, each item naming the rule id and a path — `createFindings().result()` (`validate.js:262-277`); scratch keys `ok, blocks, warnings, informs`; test `separates blocks, warnings, and informs, each naming a rule id and a path` passed.
- [x] und-1: rules cited by id; no ADR cap restated as a number in a comment; N1–N12 severities match ADR-0003 Decision 6 (N1 block and legacy warn, N4 never reported, N12 block and warn) — registry `validate.js:24-87`; suite `the registry matches ADR-0003 Decision 6` passed.
- [x] und-2: tag start is `<` then a name, `/`, or `!` — the verifier brute-forced every string up to length 6 over a small alphabet: the old pattern matched nothing the new one misses (`old_match_new_miss_count: 0`).
- [x] Verification Strategy: `node --test test/plan-validate.test.js` exits 0, including the assertion that a non-catalog slug of the right shape is not a schema failure (`a well-formed media id outside the four spike pictures is not rejected`); `npm test` exits 0.

## Tests run by the verifier

```text
node --test test/plan-validate.test.js
ℹ tests 70 · suites 12 · pass 70 · fail 0 · cancelled 0 · skipped 0 · todo 0 · duration_ms 2374.2313

npm test
ℹ tests 433 · suites 81 · pass 431 · fail 0 · cancelled 0 · skipped 0 · todo 2 · duration_ms 162629.0174
```

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-002 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| — | — | First pass was all-PASS | — | — |

---

## Notes

- The verifier was a fresh general-purpose sub-agent that did not author the code. It was told not to edit, create, or delete any repository file and to keep scratch scripts in the OS temp directory. After it returned, `server/plans/validate.js` (47,470 bytes) and `test/plan-validate.test.js` (43,626 bytes) were unchanged and no new file appeared in the repository from it. This report was written from its returned table by the implementer.
- Observations outside the criteria, none changing the verdict: the two `todo` tests are `it.todo` entries in `test/media-ingest.test.js:828-829` (a parallel spec's `dependency_add` gate); the schema markup change is stricter than the old pattern (`</>`, `< /` now block) and loosens nothing; finding messages compute their figures from the registry at run time, which und-1 allows.
- No AC was passed by weakening a test or the criterion.
