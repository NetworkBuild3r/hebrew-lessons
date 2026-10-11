# SPEC-018 AC Verification Eval Report — Starter verb guide and workshop

**Traceability:** INIT-006/SPEC-018
**Verifier:** fresh sub-agent, did not write the code · model `composer-2.5-fast` · temperature 0 by prompt · read-only by prompt (the Task tool has no readonly flag)
**Evaluated at:** 2026-10-10
**Verdict:** PASS · **Remediation loops:** 0

Verifier run: [Verify SPEC-018 criteria](ddd4943b-1855-49bc-a892-59ae01a7885e).

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence |
| ---- | ----------- | ------- | -------- |
| ac-1 | Both drafts validate under schema 1.1.0 with no block-level errors; unverified-form warnings are expected. | PASS | `node --test test/starter-content.test.js` exit 0, 5/5. Test `ac-1 and ac-2: drafts validate and 52 forms stay unverified` asserts `validatePlan` `blocks` is `[]` and `schemaVersion` is `1.1.0` (`test/starter-content.test.js`). N14 (unverified form) is present on the verb workshop. `writePlan` refuses to store a plan that has block findings (`scripts/seed-starter-content.js`). |
| ac-2 | 4 lemmas × 13 forms = 52 forms tagged with tense, cell, binyan, and root class. | PASS | `WORKSHOP_FORMS.length` is guarded at 52. The same test checks four lemma keys and, on each stored row, `tense`, `cell`, `binyan`, `root_class`, and `verified === 0`. Feel is `hifil`. Answer is `lamed-he`. |
| ac-3 | The checklist lists all 52 forms; a second run marks only ticked forms verified. | PASS | Checklist has 52 unticked lines. Test `ac-3: a second run verifies only ticked forms` ticks two present masculine-singular forms, asserts only those two are verified, then unticks one and asserts the verified count is 1. Guide lemmas stay unverified. |
| ac-4 | No teacher name, learner name, or class transcript text in the content. | PASS | Test `ac-4: stored content has no name and no copied class line` scans the script, the checklist, both plan documents, and every gloss. Banned needles: Brian, Nelson, Shai, Shay, and the class-sheet word from the existing indirect-speech worksheet. The verifier also searched the seed script. No class transcript is stored in this repo to diff against; the check is the fixed needle list plus original sentences. |

## Definition of done

- [x] `scripts/seed-starter-content.js` writes drafts through `lexicon.upsert`, `setVerified`, and plan ops.
- [x] Forms checklist lists 52 forms, all unticked.
- [x] `test/starter-content.test.js` exits 0 on a temporary database.
- [x] Drafts stay `hidden` and `draft`. Nothing is published.
- [x] Stored keys are NFC.

## Tests run by the verifier

```text
node --test test/starter-content.test.js
→ tests 5, pass 5, fail 0, exit 0
```

---

## Remediation log

| Loop | Failing AC | Gap | Fix | Re-verify |
| ---- | ---------- | --- | --- | --------- |
| — | — | No failures. | — | — |

## Notes

- Pointing is a draft. The verifier did not treat linguistic correctness as an acceptance criterion. Forms stay `verified=0` until a person ticks the checklist.
- ac-4 cannot be a full transcript diff: the class sheets are not in the repo. The mechanized check is the banned-name list and a known worksheet fragment.
