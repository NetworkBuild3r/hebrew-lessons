# SPEC-005 AC Verification Eval Report — Lexicon service, validator, and seed

**Traceability:** INIT-006/SPEC-005
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` (`ac_eval.verifier_model` on Cursor) · [Verify SPEC-005 criteria](630e8312-4615-4d58-b298-55103ee3d13c) · temperature 0 · read-only by prompt (the Task tool has no readonly flag)
**Evaluated at:** 2026-10-10
**Verdict:** PASS · **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · file:line · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | A batch with one invalid entry writes nothing and returns every error with `key`, `field`, and `code`. | PASS | Test `ac-1: a batch with one invalid entry writes nothing and returns every error` passed. `validateEntries` returns the full error list; `upsert` returns it before the transaction when shape checks fail, and throws inside `db.transaction` when the lemma lookup fails, so the write loop never runs. The test requires `gloss_en:markup`, `pos:enum`, and `root:root_shape`, an unchanged row count, and no `lexicon_upsert` audit. `node --test test/lexicon-service.test.js`: 7 pass, 0 fail, exit 0. |
| ac-2 | Text containing `<script>` is rejected. NFD input is stored NFC. | PASS | Test `ac-2: markup is rejected and NFD input is stored NFC` passed. `MARKUP` rejects `<script>` on `gloss_en`. Keys go through `exactKey` (`server/vocab-index.js`), other text through `normalize('NFC')`. The stored key and gloss equal the NFC forms, including `café`. Same command: 7 pass, 0 fail. |
| ac-3 | A stale `expectedRev` returns `revision_conflict` and leaves the row unchanged. | PASS | Test `ac-3: a stale expectedRev returns revision_conflict and leaves the row` passed. Every key is compared to `expectedRev` before `lexicon.upsert`. A mixed batch that includes one stale row does not insert the new key. Same command: 7 pass, 0 fail. |
| ac-4 | Editing a verified form clears `verified`. `setVerified` writes an audit row and refuses actor `mcp`. | PASS | Test `ac-4: editing a verified form clears verified; setVerified audits and refuses mcp` passed. `rowForWrite` stores `verified: 0`. `setVerified` returns `forbidden` for `mcp` and writes no `lexicon_verify` row. An `admin:ada` call writes `lexicon_verify` with `{ count, value }` and no entry text. A later edit clears the flag and records `cleared: 1`. Same command: 7 pass, 0 fail. |
| ac-5 | Seeding the three current lessons twice keeps the row count. An `mcp` row with the same key keeps its gloss. | PASS | Test `ac-5: seeding the three lessons twice keeps the row count and an mcp gloss` passed. The fixture copies `my-morning`, `shai-date-indirect-speech`, and `anal-sex-vocab`. The second `seed.run` inserts 0 and updates 0. After an MCP upsert, a third seed leaves `gloss_en` as `kept-by-mcp` and `source` as `mcp`. Seed rows have null `pos`, `root`, and `binyan`. Same command: 7 pass, 0 fail. |
| security | Markup is rejected on the server, independent of rendering. | PASS | Same evidence as ac-1 and ac-2. `seed.js` `cleanField` also rejects `<` and `>` before insert. |
| security | `setVerified` is unreachable for actor `mcp`. | PASS | Same evidence as ac-4. The guard is `actor === 'mcp'` or a writer that is not `admin:`. |

## Definition of done

- [x] Lexicon writes are validated, atomic, revisioned, and audited — ac-1 through ac-4.
- [x] The seed is idempotent and does not overwrite a non-seed gloss — ac-5.
- [x] `node --test test/lexicon-service.test.js` exits 0 — verifier and implementer.
- [x] `npm test` exits 0 — implementer run, 590 pass / 0 fail (the verifier ran the spec file only).

## Tests run by the verifier

```text
node --test test/lexicon-service.test.js
→ 7 pass / 0 fail ; exit 0
```

The verifier did not run `npm test` and did not open `data/hebrew.sqlite`.

---

## Remediation loop log

None. First verification was all-PASS.
