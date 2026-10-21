# SPEC-002 Eval Report — Cross-lesson vocabulary index generator

**Traceability:** INIT-001/SPEC-002
**Verifier:** fresh explore Task (`composer-2.5-fast`, id `b5c13251-5b91-41d8-860e-829edf65354d`) — did **not** author the code
**Tier:** full · checks skipped: None
**Remediation loops:** 0
**Final verdict:** **PASS** (all-PASS)

## Tests run by verifier

| Command | Result |
|---------|--------|
| `node --test test/vocab-index.test.js` | 6 pass, 0 fail, exit 0 |
| `npm run vocab-index` | exit 0 — 27 words from 3/3 lessons |

## Per-AC verdicts

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | CLI `package.json` → `scripts/build-vocab-index.js`; generated index 27 unique keys; `קָמָה` lessons `["anal-sex-vocab","my-morning"]` at `worksheets/_review/vocab-index.js`; tests merge + repo CLI ✔ |
| ac-2 | PASS | `catalogAudioFor` + merge prefers catalog; test attaches shared URL ✔; repo `קָמָה` audio `../_audio/words/a2e3165352468e3a501b294e.mp3` |
| ac-3 | PASS | throw/missing vocab → warn + skip; `writeFileSync` still runs; fixture test warnings for broken/hostile/novocab ✔ |
| ac-4 | PASS | `mcp/tools.js:125` `rebuildVocabIndexSafe()` after audio paths; safe unit test ✔; source-contract test ✔ (no upsert E2E mock — residual, not a FAIL) |
| ac-5 | PASS | `path.join(LESSONS_DIR, name)` + escape guard; write only `_review/vocab-index.js`; body via `JSON.stringify`; test asserts sole review file |

## Remediation log

None — first pass all-PASS.
