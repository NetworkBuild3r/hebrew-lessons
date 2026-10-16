# SPEC-011 AC Verification Eval Report — Plan language 1.1: five blocks, recipes, rules

**Traceability:** INIT-006/SPEC-011 \
**Verifier:** three fresh sub-agents, none of which wrote the code · model inherited from the session (Claude Opus 5.5; see Notes) · temperature 0 by prompt · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-11 (loop 3) \
**Verdict:** PASS · **Remediation loops:** 2 (loop 1 FAIL on und-1, fixed; loop 2 PASS; loop 3 PASS on the final tree)

Verifier runs: [Verify SPEC-011 criteria](7359f247-f01c-471d-aab9-1fff8d17ec1d) (loop 1), [Re-verify SPEC-011 after fix](bbd6e829-995a-4ba9-bc64-cc2835232273) (loop 2), [Final check SPEC-011 tree](2bbfde72-6055-47a0-8a96-df31daa322e4) (loop 3). The table below is loop 3, run against the tree as delivered.

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | The four ADR-0003 sample plans validate unchanged under 1.1.0 rules with the same results as before. | PASS | `test/plan-language-1-1.test.js:148` and `:239` deep-equal current results to `test/fixtures/plans-1.1/samples-1.0.0-results.json`, with and without a lexicon, and with the samples read as 1.1.0. Loops 1 and 2 each rebuilt the pre-change validator from the recorded edits and reproduced the baseline 11 of 11. The baseline was written before the first schema or validator edit. |
| ac-2 | One fixture per new block validates; one hostile fixture per block (markup, oversize, unknown property) fails with the rule id. | PASS | `:298`: five fixtures, no block; the only finding is the N7 warn ADR-0009 intends on a workshop-only unit. `:325`: each `*.hostile.plan.json` fails with `html`, `schema` (list over its limit), and `schema` (unknown property) at the exact path. Markup and 4,001-character probes on every new string field: `:360`, `:370`. |
| ac-3 | A conjugation table naming a form of another lemma blocks; naming an unverified form warns. | PASS | `:512` (N13 block, "formKey names a form of another lemma"), `:521` (N14 warn, `ok` stays true). Code `server/plans/validate.js:988-1004`. Repeated by the verifier against the real lexicon on a temporary database; `:1253` covers publish. |
| ac-4 | Each inform/warn rule from ADR-0009 §7 has a positive and a negative fixture. | PASS | N14 `:593`, N15 `:600`, N16 `:619`, N17 `:662`, N18 `:678`; N13 and N19 negatives `:696`. Positives are built in memory from the fixture files, as INIT-004's tests do. Mutants that remove each rule are caught. |
| ac-5 | `insertBlock` of each new type succeeds and bumps the revision; on a 1.0.0 plan the same revision sets `schemaVersion` 1.1.0; on a 1.1.0 plan it changes nothing else. | PASS | `:1117`, `:1135` (ten tests) deep-compare the stored plan. `server/plans/ops.js:122-128`. The loop-3 verifier repeated 40 inserts (5 types × 4 samples × 2 versions): only `revision`, the target lesson's `blocks`, and `schemaVersion` (1.0.0 case) changed; `status` resets to `draft` on samples stored as `published`, which is existing ADR-0003 §7 behavior. |
| ac-6 | A 1.0.0 document with a new block, a `recipe`, or a `group` is rejected; a letter-intro with a picture field fails; a guide lesson and a letters-and-vowels lesson do not trigger N2 or N7. | PASS (ADR-0009 reading) | `:808`, `:822` (schema restriction at `lesson-plan.schema.json:61-71`, `:910-942`); `:841` (`image`, `picture`, `mediaId` on the block and its items); `:859` (guide: no N2, no N7; control under another recipe triggers both); `:871` (letters-and-vowels: no N7; no N2 when it builds on nothing). See Notes on N2. |
| ac-7 | Topic-conversation: 9 new words warn, 16 block, a group of 6 warns; another recipe keeps the N1 thresholds. | PASS | `:887` (9 and 15 warn, 16 block, all N18, never N1), `:899` (group of 6), `:916` (other recipes and none: "9 new words; N1 allows 8"; 8 passes). `validate.js:837-874`. |
| und-1 | A 1.0.0 plan never fails a rule introduced here. | PASS (loop 3) | N13–N19 and the recipe, verse-word, and N7-list changes run only for 1.1.0 (`validate.js:823-831`, `:1342-1400`, `:1054`); the N13/N19 relabels need 1.1.0 (`:514-527`). Tests `:173`, `:210`. Loop-3 fuzz: 21,497 validations of 1.0.0 documents, zero N13–N19 findings. |
| und-2 | The validator checks that a cell points at a form; it cannot check the Hebrew. | PASS | `checkFormKey` checks lemma, tense, cell, and `verified` only (`validate.js:988-1004`). |
| Security | Every new string field is length-limited and markup-free at validation. | PASS | Schema walk: every new string has a length limit and the markup ban, an anchored pattern, or an enum. Tests `:360`, `:370`, `:396`, `:425`. `html`, `schema`, and `N11` refuse the write (`ops.js:24-33`). |
| Security | `personal-question` has no field that stores an answer. | PASS | Fields `id, type, prompts, answer` (`answer` is `speak`/`type`/`either`), prompts `id, he, en` (`lesson-plan.schema.json:804-820`, `:636-645`). Test `:467`, `:474`. |
| Tech req | JSON Schema draft-07, `additionalProperties: false` on every new object. | PASS | `lesson-plan.schema.json:2`; all 15 new object definitions closed (`:266` plus the verifier's walk). Loop 2 also checked the schema with ajv 8 against the draft-07 meta-schema. |
| Tech req | Rule ids and severities in one table in code, matching ADR-0009 §7 one for one. | PASS | `RULES` in `validate.js:28-195` (N13 `:95` … N19 `:129`); test `:951-996` parses the ADR table and compares ids, severities, figures, and names. |

## Definition of done

- [x] Schema 1.1.0 is additive and closed — ac-1, ac-2, tech req 1.
- [x] Rules N13–N19 and the N1, N2, N6, N7 changes match ADR-0009 §7 — ac-3, ac-4, ac-7, tech req 2.
- [x] `ops.js` raises `schemaVersion` and never lowers it — ac-5.
- [x] `node --test test/plan-language-1-1.test.js` exits 0 — every loop.
- [x] `npm test` exits 0 — every loop.

## Tests run by the verifier

```text
node --test test/plan-language-1-1.test.js   → 77 pass / 77 ; exit 0
node --test test/plan-validate.test.js        → 70 pass / 70 ; exit 0
npm test                                      → 781 pass / 781 ; exit 0
```

Loop 3 also ran its own probes from `%TEMP%`: a validator probe (270 checks, including the 21,497-validation fuzz), a temporary-database probe of `ops.js` and `publish.js` (494 checks), and 38 code mutants on a copy (36 caught; the two survivors are described in Notes). Loop 2 ran an 18,519-write probe through `replacePart`, `insertBlock`, and `setField`; no stored block failed an independent type check.

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-006/SPEC-011 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| 1 | und-1 | `ruleForSchemaError` relabelled some schema errors as N13 (cell codes) and N19 (`ref.source`) whatever the version, so a 1.0.0 document holding a new block could show N13/N19. The same relabel let a draft store a non-string `ref.source` or a non-code cell, because N13 and N19 do not refuse a write. | A non-code cell and a non-string source stay `schema`; a valid code in the wrong tense becomes N13 and an empty or missing source becomes N19, both only in 1.1.0. Tests for 1.0.0 documents with defective new blocks and for the write path. `server/plans/validate.js`, `test/plan-language-1-1.test.js` | Loop 2: PASS, all items |
| 2 (follow-up after a PASS) | none | Not a failure. Loop 2 noted a redundant N13 beside the `schema` finding for a junk cell, and no test pinning the 1.0.0 gating of the N1, N2, and N7 changes. | A junk cell is one `schema` finding at `/cell`; the cell-code list is read from the schema's `cellCode` enum (`cellAt`). New test `the N1, N2, and N7 changes reach only a 1.1.0 document`. Same files. | Loop 3: PASS, all items, final tree |

---

## Notes

- **ac-6 and N2.** ac-6 says a letters-and-vowels lesson does not trigger N2. ADR-0009 §7 ("N2 does not apply to a `guide`") and this spec's own Scope exempt only `guide` from N2; both recipes are left out of N7. All three verifiers applied the ADR reading. Under it, a letters-and-vowels lesson that builds on nothing has no N2, and one that builds on earlier lessons with fewer than three recycles gets the N2 warn, which test `:871` states in its name. Read as "never triggers N2", ac-6 would fail. That is a spec-versus-ADR wording conflict for the operator, not a code defect.
- **Surviving mutants (loop 3).** Removing the version check before N17 changes nothing, because a 1.0.0 lesson's recipe is already unread. Adding a code to the cell-code list breaks no test; the list is correct today but not pinned to ADR-0009 Decision 4.
- **Residual notes, not failures.** A non-code cell also gets "formKey names a form of another cell" at the sibling `/formKey`, harmless because the `schema` finding refuses the write. N18 keeps N1's `migration.legacy` warn above 15 words, which the N18 row does not state. `scripts/import-lesson-plans.js` validates without a lexicon, so an imported 1.1.0 plan with a conjugation table would get N13 (fail-closed; today's imports are 1.0.0).
- **Model.** `ac_eval.verifier_model` names `composer-2.5-fast` on Cursor. This harness passes a subagent model only when the user names one, so all three verifiers ran on the inherited session model.
- No AC was passed by weakening a test or the criterion. The two narrowed assertions in `test/plan-validate.test.js` were checked by every loop and keep that file's ADR-0003 checks.
