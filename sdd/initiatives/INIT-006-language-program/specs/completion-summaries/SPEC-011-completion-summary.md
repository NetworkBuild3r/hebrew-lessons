# SPEC-011 Completion Summary — Plan language 1.1: five blocks, recipes, rules

**Traceability:** INIT-006/SPEC-011
**Status:** Complete
**Domain:** service
**Completed Date:** 2026-10-11

---

## Deliverables Checklist

- [x] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` at `1.0.0` or `1.1.0`: five blocks, lesson `recipe`, introduction-word `group`, and a branch that rejects any of them in a `1.0.0` document
- [x] Rules N13–N19 and the N1, N2, N6, N7 changes in `server/plans/validate.js`; the `schemaVersion` raise in `server/plans/ops.js`
- [x] Fixtures under `test/fixtures/plans-1.1/`
- [x] Tests: `test/plan-language-1-1.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-010): the four ADR-0003 sample plans validate unchanged under `1.1.0` rules with the same results as before.
- [x] ac-2 (REQ-010): one fixture per new block validates; one hostile fixture per block (markup, oversize, unknown property) fails with the rule id.
- [x] ac-3 (REQ-011): a conjugation table naming a form of another lemma blocks; naming an unverified form warns.
- [x] ac-4 (REQ-010): each inform/warn rule from ADR-0009 §7 has a positive and a negative fixture.
- [x] ac-5: `insertBlock` of each new type through `ops.js` succeeds and bumps the revision. On a `1.0.0` plan the same revision sets `schemaVersion` to `1.1.0`; on a plan already at `1.1.0` it changes nothing else.
- [x] ac-6: a `1.0.0` document with a new block, a `recipe`, or a `group` is rejected; a `letter-intro` with a picture field fails; a `guide` lesson and a `letters-and-vowels` lesson do not trigger N2 or N7. Met under ADR-0009's reading of N2; see Deviations.
- [x] ac-7: a `topic-conversation` lesson with 9 new words warns, 16 blocks, and an introduction group of 6 warns. A lesson of another recipe keeps the ADR-0003 N1 thresholds.
- [x] und-1: a `1.0.0` plan never fails a rule introduced here. (Failed in verification loop 1; fixed.)
- [x] und-2: the validator checks that a cell points at a form, not that the Hebrew is right.
- [x] Security: every new string field is length-limited and markup-free at validation.
- [x] Security: `personal-question` has no field that stores an answer.

## AC Verification (independent)

Verified by fresh sub-agents that did **not** write the code — not self-reported. Model: inherited session model (see Harness Adaptation).
Overall: **PASS** (all-PASS required to complete) · remediation loops: 2.
**Full eval report:** [`SPEC-011-eval-report.md`](SPEC-011-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `PASS` · security_review=`required` `FAIL` · domain=`service` `FAIL` · autonomy_level=`auto_tests` `PASS` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `test/plan-language-1-1.test.js:148`, `:239`: deep-equal to the pre-change baseline; loops 1 and 2 rebuilt the old validator and reproduced it 11/11 |
| ac-2 | PASS | `:298` five fixtures validate; `:325` five hostile fixtures fail with `html` and `schema` at exact paths; `:360`, `:370` probe every new string field |
| ac-3 | PASS | `:512` N13 block (other lemma), `:521` N14 warn; `server/plans/validate.js:988-1004` |
| ac-4 | PASS | N14 `:593`, N15 `:600`, N16 `:619`, N17 `:662`, N18 `:678`, N13/N19 negatives `:696` |
| ac-5 | PASS | `:1117`, `:1135` compare the whole stored plan; `server/plans/ops.js:122-128` |
| ac-6 | PASS (ADR reading) | `:808`, `:822`, `:841`, `:859`, `:871`; schema `:61-71`, `:910-942` |
| ac-7 | PASS | `:887`, `:899`, `:916`; `validate.js:837-874` |
| und-1 | PASS (loop 3) | `:173`, `:210`; loop-3 fuzz of 21,497 validations of 1.0.0 documents found no N13–N19 |
| security ×2 | PASS | `:360`, `:370`, `:396`, `:425`, `:467`, `:474`; schema `:804-820` |

Final run, verifier loop 3: `node --test test/plan-language-1-1.test.js` 77/77 · `node --test test/plan-validate.test.js` 70/70 · `npm test` 781/781.

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-011`
- **Title:** Plan language 1.1.0: five blocks, the lesson recipe, rules N13–N19, and the `schemaVersion` raise
- **Files changed:** 5 source and test files modified, 14 files created (13 fixtures and one test file). The plan files are untracked in git, so there is no line diff.
- **Change status:** proposed
- **Reversibility:** branch. Rollback: restore the INIT-004 versions of the schema, `validate.js`, `ops.js`, `publish.js`, and `test/plan-validate.test.js`, and delete the new test and fixtures. `1.0.0` documents give the same results either way. A plan already raised to `1.1.0` would then be refused as an unknown version until it is removed or the change is restored.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/plan-language-1-1.test.js` | 77 | 77 |
| `test/plan-validate.test.js` | 0 (two assertions narrowed, one title renamed) | 70 |

**All tests passing:** Yes · `npm test` 781/781 at the final run (640/640 before this spec; other agents added tests in the same checkout meanwhile) · `node spikes/skeleton/validate.js` OK and `node --test spikes/skeleton/test/` 21/21 (ADR-0003's mechanized schema check) · `npm run audio:verify` OK, 65 recordings · **Coverage (new code):** no coverage tool is configured. Instead, every new rule, the version raise, and the 1.0.0 gating were mutation-checked: 25 deliberate code breaks, all caught by `test/plan-language-1-1.test.js`, each file restored afterwards.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | `node --test test/plan-language-1-1.test.js` (77/77), `npm test` (781/781) | No migration, no deploy, no dependency. `package.json` was not changed by this spec. |

None — branch/reversible only.

---

## Security Review

- **Sensitivity / depth:** required · standard (spec focus: schema closure, size limits) · **method:** [Security Review](2688c757-4504-42fb-949d-d6191523b889) subagent (Cursor T2), scoped to this spec's files. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **After the review:** verification loop 1 found that schema errors relabelled as N13/N19 let a draft store a wrong-shaped `ref.source` or a non-code cell, because those rules do not refuse a write. Fixed: wrong shapes stay `schema` and are refused at write time. Loops 2 and 3 confirmed it, including an 18,519-write probe. The fix only narrows what a draft can hold. The security review was not re-run after it, because this harness allows one security-review dispatch per request. The compensating control is the initiative-wide Security Review at `complete-initiative`.
- **Recommendation considered, not adopted:** add N13 and N19 to `WRITE_BLOCK_RULES`. This keeps INIT-004's split: a draft may hold a table whose forms are not stored yet, or a verse with no source, and publish refuses both.
- **Reviewer's file accounting:** its footer lists the schema as outside its diff budget, but its findings cite schema-only details (`tableCell` limits, `v110Only` as `not: {}`). The schema's closure, length, and markup are also checked by tests on every new field.
- **Positive observations:** every new object is closed. Every new string has a length limit and the markup ban. Finding messages never echo plan text. The validator fails closed (N13 block) without a lexicon. Lexicon lookups are memoized, skipped for keys the schema refuses, and bounded by the 9-cell table and N9. A 1.1.0 plan cannot be lowered to 1.0.0. `validate.js` still requires only `fs` and `path`.

---

## Files Changed

Provenance is `INIT-006/SPEC-011` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | Modified — `schemaVersion` enum; five block definitions and their items; `recipe`, `group`; text types from ADR-0009 §7; `lexiconKey`, `root`, cell codes; the 1.0.0 restriction branch | `INIT-006/SPEC-011` |
| `server/plans/validate.js` | Modified — registry rows N13–N19 and the N1, N2, N6, N7, pointed-word changes; `validatePlan(document, { lexicon })`; `usesV110`, `V100`, `V110` exports; schema-error mapping | `INIT-006/SPEC-011` |
| `server/plans/ops.js` | Modified — `finish()` raises `schemaVersion` and refuses a downgrade; `mutate()` passes the lexicon repo | `INIT-006/SPEC-011` |
| `server/plans/publish.js` | Modified — passes the lexicon repo to `validatePlan` | `INIT-006/SPEC-011` |
| `test/plan-validate.test.js` | Modified — unknown-version list uses `1.2.0`; N1–N12 order check compares the first twelve numbered keys; one describe title | `INIT-006/SPEC-011` |
| `test/plan-language-1-1.test.js` | Created — 77 tests, pure and temporary-database | `INIT-006/SPEC-011` |
| `test/fixtures/plans-1.1/{explainer,conjugation-table,personal-question,verse-reading,letter-intro}.plan.json` | Created — one valid 1.1.0 plan per block | `INIT-006/SPEC-011` |
| `test/fixtures/plans-1.1/*.hostile.plan.json` (five) | Created — markup, list over its limit, unknown property | `INIT-006/SPEC-011` |
| `test/fixtures/plans-1.1/lexicon.json` | Created — two lemmas and their forms, one unverified | `INIT-006/SPEC-011` |
| `test/fixtures/plans-1.1/variants-1.0.0.js`, `samples-1.0.0-results.json` | Created — 1.0.0 variants and the pre-change validator's results | `INIT-006/SPEC-011` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-011-service-plan-language-1-1.md` | Status `complete`; criteria checked | `INIT-006/SPEC-011` |
| `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-011-eval-report.md` | Created | `INIT-006/SPEC-011` |

`shared/player/fsrs.js` and `server/progress.js` were not edited. No renderer and no starter content.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | `subagent_dispatch` | T2 (Cursor) | [Security Review](2688c757-4504-42fb-949d-d6191523b889), inherited session model |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | Three fresh `generalPurpose` subagents: [loop 1](7359f247-f01c-471d-aab9-1fff8d17ec1d), [loop 2](bbd6e829-995a-4ba9-bc64-cc2835232273), [loop 3](2bbfde72-6055-47a0-8a96-df31daa322e4). `ac_eval.verifier_model` names `composer-2.5-fast`; this harness passes a model only when the user names one, so all ran on the inherited model. The Task tool has no readonly flag; each prompt forbade edits. |

The spec pins `opus`; this session ran on Claude Opus 5.5.

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo (`scripts/dev/` does not exist). This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | ---------- | ------ | ------------- | ------ |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is absent | No opening sentinel for SPEC-011 | No live trace file for a Tier-5 reader. Disclosed, not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Recorded in Harness Adaptation instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | The Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- **Git:** no commit, push, or PR. The dispatch said to skip them.
- **Persona:** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. No persona file was invented; the work followed the spec, ADR-0009 §7, ADR-0003, and the INIT-004 code.
- **Files beyond the deliverables:** `publish.js` passes the lexicon, and so does `ops.js`, in addition to the version raise the spec names as "one change to `ops.js`". N13 is a publish block and needs stored forms, and `validate.js` must keep requiring only `fs` and `path`. So the lexicon is an option, `validatePlan(document, { lexicon })`. Without it, every conjugation-table form is an N13 block. `scripts/import-lesson-plans.js` passes none; it imports only 1.0.0 plans.
- **Two INIT-004 assertions narrowed** in `test/plan-validate.test.js`: `1.1.0` was listed as an unknown version, and the registry check required exactly N1–N12. Both encoded the old contract. The file's ADR-0003 checks are otherwise unchanged; every verifier checked this.
- **ac-6 and N2 (needs an operator decision):** ADR-0009 §7 exempts only `guide` from N2, and this spec's Scope says the same. A `letters-and-vowels` lesson that builds on another with fewer than three recycles therefore gets the N2 warn; one that builds on nothing triggers neither N2 nor N7. All three verifiers accepted ac-6 on that reading. If letters-and-vowels should also skip N2, it is a one-line registry change (`RULES.N2.exemptRecipes`) and an ADR-0009 amendment. Related: the recipe puts its review checkpoint last, so N15 warns on letters-and-vowels lessons that set `buildsOn`.
- **N18 and N1:** for a `topic-conversation` lesson the word count is reported once, as N18 (warn above 8, block above 15), and N1 does not also report it. N18 keeps N1's `migration.legacy` warn above 15, following "N1's migration.legacy rule is unchanged". The N18 row itself has no such exception.
- **Draft versus publish:** N13, N14, and N19 do not refuse a write, so a draft can hold a table whose forms are not stored yet or a verse with no source. Publish refuses both. A value of the wrong shape (a non-string source, a non-code cell) is `schema` and is refused at write time.
- **`schemaVersion` given by the caller:** a whole-plan `replace_part`, or `set_field` of `schemaVersion`, keeps the value it was given. A 1.0.0 document holding a 1.1.0 construct is refused, not raised. Any operation that would take a 1.1.0 plan back to 1.0.0 returns `invalid_op`, as `set_field` already does for `id`, `revision`, and `status`.
- **Shape choices the ADR leaves open:**
  - Structural ceilings: `cells` at most 9 (the past tense has nine cells), verse `newWords` at most 24 (as lesson `newWords`), `sounds` 16, `findIn` 12, `chapter` 1–150, verse numbers 1–176, `root` at most 7 characters.
  - Fields with no ADR limit use plain text (N11, 4000). A table cell may be empty.
  - Every object list item has an `id`, `sounds[]` included. A conjugation cell is keyed by its `cell` code, which N13 keeps unique.
  - Required fields: explainer sections need only `id` and `heading`. A verse-reading needs `ref` and `verse`; `personalQuestion` is optional, as N16 implies. A letter-intro needs `letter`, `name`, and `sounds`. The order of `ref.from` and `ref.to` is not checked.
- **Registry text:** the `form-recall-image` summary now names the conjugation-table drill as the written-form task; it stays unchecked, because the block has no image field and `image-placement` reports any image.
- **Mutation checks** edited repository files in place for seconds at a time. Each file was restored in a `finally` block, and the suite was re-run green afterwards.
- **Parallel work:** one `npm test` run failed with `MODULE_NOT_FOUND` for `server/api/library-routes.js`. Another agent had edited `server/web.js` before creating that file. The re-run passed once the file existed. No SPEC-011 file was involved.

### Follow-ups (not in this spec)

- Operator: settle N2 (and N15) for letters-and-vowels lessons, and confirm the N18 legacy reading.
- A test that pins the schema's cell codes to ADR-0009 Decision 4. The list is correct but not pinned.
- INIT-004, pre-existing: a draft write can store a wrong-typed introduction `meaning` or a fluency-pass `words` list, because those map to `meaning` and N12, which do not refuse a write.

### Notes for SPEC-014, SPEC-016, SPEC-018, SPEC-019

- Call `validatePlan(plan, { lexicon })` with `server/db/repos/lexicon.js` whenever a plan may hold a conjugation table. Without a lexicon every form is an N13 block.
- A working copy can hold a conjugation table whose forms are not stored yet, or a verse-reading with an empty or missing `ref.source`. Renderers of drafts must handle both. A published snapshot has neither.
- Any part operation that adds a new block, a `recipe`, or a `group` to a 1.0.0 plan raises it to 1.1.0 in the same revision. A plan never returns to 1.0.0.
- The fixtures in `test/fixtures/plans-1.1/` are synthetic test data, not content. They copy no teacher text, and the verse is Genesis 1:1 from the public-domain Masoretic text.

---

## Pre-flight

```text
Spec: SPEC-011 validated · domain: service · prompt: principal-backend-developer (file absent; degradation disclosed)
Autonomy: auto_tests · gates expected: none
Dependencies: SPEC-005, INIT-004/SPEC-002, INIT-004/SPEC-003 complete · context loaded · timer owned by the orchestrator
Baseline before edits: npm test 640/640 · spike validator OK · audio:verify 65 recordings
```

---

## Effort Tracking

**Agent session time:** not measured here (the orchestrator owns the timer)
**Spec effort estimate:** 5 hours
