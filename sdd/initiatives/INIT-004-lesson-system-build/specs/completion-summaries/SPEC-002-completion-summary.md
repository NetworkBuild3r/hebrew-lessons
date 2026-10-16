# SPEC-002 Completion Summary — Plan validator: JSON Schema and the research-rule registry

**Traceability:** INIT-004/SPEC-002
**Status:** Complete
**Domain:** service
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/plans/validate.js` — reads `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (draft-07), applies it with a ported keyword walker, and reports the ADR-0003 Decision 6 registry by rule id and severity. Exports `validatePlan` (also as `validate`), `RULES`, `FUNCTION_WORDS`, `SCHEMA_PATH`, `tokenize`.
- [x] `test/plan-validate.test.js` — 70 tests: the four samples plus in-memory mutations, and drift checks that read ADR-0003 itself.
- [x] Schema markup pattern made linear (see Deviations): `plainText.not` and `altText.not` in `lesson-plan.schema.json`.

## Acceptance Criteria

- [x] ac-1: the four samples validate with no block; `anal-sex-vocab` and `shai-date-indirect-speech` return N1 as warn; `my-morning` without `migration.legacy` and with a ninth new word returns N1 as block.
- [x] ac-2: tag-like string → `html`; missing alt → `N8`; text over the N11 limit → `N11`; URL media id → `schema` block on `mediaId`; missing `title` → `schema` naming it; `not-a-real-picture` is not rejected.
- [x] ac-3: a word list on `fluency-pass` → N12 block; N12 warns beside new words; the café unit has no N12; each legacy lesson warns.
- [x] ac-4: the module contains none of `coffee-cup`, `water-glass`, `milk`, `sugar`, and imports nothing from `spikes/`.
- [x] Security: a string that starts an HTML tag is a block; an absolute or scheme-bearing `audioPath` is a block.

## AC Verification (independent)

Verified by a fresh sub-agent (model `cursor-grok-4.5-high-fast`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-002-eval-report.md`](SPEC-002-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`standard` `FAIL` · domain=`service` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | Verifier scratch run plus tests `all four samples return no block`, `…return N1 as warn because migration.legacy is true`, `my-morning without migration.legacy and with a ninth new word returns N1 as block`; `checkNewWordCap` `server/plans/validate.js:654-661`. |
| ac-2 | PASS | Six tests under `ac-2: the spike broken cases are rejected for that reason` (`test/plan-validate.test.js:127-179`) plus the verifier's own mutations. |
| ac-3 | PASS | Suite `ac-3` (`test/plan-validate.test.js:181-220`); `checkFluency` `validate.js:678-688`. |
| ac-4 | PASS | Source scan and test `the module source does not contain the spike picture ids`; spike `MEDIA_IDS` stays at `spikes/skeleton/validate.js:17`, not imported. |
| security ×2 | PASS | Schema `lesson-plan.schema.json:79,85,87-91`; suite `security: markup and audio paths`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-002`
- **Title:** Plan validator: the ADR-0003 schema and the N1–N12 registry, with linear-time text checks
- **Files changed:** 3 · additions +2,068 · deletions −2 (validate.js +1,160, test +906, schema ±2), plus this summary, the eval report, and the spec status
- **Change status:** proposed
- **Reversibility:** branch. Rollback: delete the module and the test, and restore the two pattern lines in the schema. No data change.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/plan-validate.test.js` | 70 | 70 |

**All tests passing:** Yes.

- `node --test test/plan-validate.test.js` → 70 pass, 0 fail.
- `npm test` → baseline before this spec 341/341. After it, 432 tests: 430 pass, 0 fail, 2 `todo` (the verifier's run: 433 / 431 / 0 / 2). The extra files come from SPEC-001, SPEC-006, SPEC-008, and SPEC-013 running in the same checkout. The two `todo` tests are in `test/media-ingest.test.js`.
- The spike still passes against the edited schema: `node spikes/skeleton/validate.js` → `validate: ok`, and `node --test spikes/skeleton/test/` → 21/21.
- **Coverage (new code):** no coverage tool is configured. In its place, a mutation check ran: twelve deliberate bugs were injected into a temp copy of the module, and every one was caught by at least one failing test. The unmutated copy passed.

Results on the samples (for SPEC-003, SPEC-007, and SPEC-018):

| Sample | Blocks | Warnings | Informs |
|---|---|---|---|
| `beginner-unit-1` | 0 | 0 | `questions`, `recall-day` |
| `my-morning` | 0 | N12 ×2, N3, N6 (`בֹּקֶר`, prefixed only) | `questions` |
| `anal-sex-vocab` | 0 | N1, N12 ×2, N3, N6 ×2 | `questions` |
| `shai-date-indirect-speech` | 0 | N1, N12, N3, N6 ×3 | `questions` |

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. No dependency was added.

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** the `sdd-code-security` procedure, run inline by the implementing agent (`self_reviewed: true`). The reviewer model the config maps for Cursor (`cursor-grok-4.6-xhigh-fast`) is not available in this session, and a different model was not substituted. SAST and secret scan: not run — none configured. Registry steps (SECSCAN id, findings ledger) were not run: `scripts/dev/security_scan_indexer.py` and `sdd/security-reviews/` are not in this repo.
- **Findings:** Critical: 0 · High: 0 · Medium: 3 · Low: 3
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| MED-1 Quadratic backtracking in the schema markup pattern `<\s*/?\s*[A-Za-z!]`: 60,000 spaces after a `<` cost 1.2 s, and about 5 ms per 4,000-character field. Authenticated callers could stall the single Node process. | Medium | API4, CWE-1333 | `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` `plainText.not`, `altText.not` | fixed — pattern `<\s*[A-Za-z/!]` (500,000 spaces: under 1 ms). It is a superset of the old pattern: of 19,627 generated strings, none the old blocked is now allowed. The walker also never runs `pattern` or `not` past `maxLength` (`checkString`). Tests `the schema markup pattern runs in linear time`, `long whitespace after a < does not stall validation`. |
| MED-2 Phrase matching in the first draft cost (entries × tokens × phrase length): adversarial plans of about 290 KB took 1.5–2.1 s, and more at the N9 ceiling. | Medium | API4, CWE-407 | `server/plans/validate.js` text estimates | fixed — token tries, cost (tokens × longest phrase): the same plans run in 87–217 ms, with identical findings on 6,004 sample and fuzzed plans. Test `many phrases that share a long run of one token do not stall the text estimates`. The pre-fix copy fails it. |
| MED-3 The tokenizer's edge-strip regex `[\p{P}\p{S}\p{Cf}]+$` backtracks quadratically: 60,000 punctuation marks in one token cost 1.8 s. | Medium | API4, CWE-1333 | `server/plans/validate.js` `tokenize` | fixed — linear code-point scan (`trimEdges`). Test `a long run of punctuation inside one token does not stall the tokenizer`. |
| LOW-1 Report amplification: an 8 MB input produced about 210,000 findings in 1.2 s. | Low | API4 | `validatePlan` | fixed — a plan over N9 is reported by size alone: one finding in about 0.2 s. Tests `a plan over N9 is reported by size alone`, `an input far over N9 returns one finding…`. |
| LOW-2 Finding `path` strings can carry untrusted key text. It is escaped for RFC 6901, not for HTML. Messages never echo plan text. | Low | output handling | `server/plans/validate.js` `pointer` | accepted — consumer note: SPEC-004 and SPEC-005 responses, and the SPEC-015 and SPEC-016 builder, render `path` as text. |
| LOW-3 Under N9, an error-dense document can still return tens of thousands of findings, about 10× the input size. It is bounded by N9 and needs an authenticated caller. | Low | API4 | `validatePlan` result | accepted — SPEC-004 and SPEC-005 may cap the findings they return. |

- **Positive observations:** Unknown properties fail at every level, and an unknown `schemaVersion` is reported before anything else is parsed. Plans hold no URLs: a media id is a slug, and `audioPath` is a relative `.mp3` path. The markup and N11 checks scan every string and every key, including gloss keys and values under unknown properties. The scan is iterative, so nesting 100,000 levels deep does not exhaust the stack. `loadSchema` fails closed on any draft-07 keyword, type, or `$ref` the walker does not implement. The input is round-tripped through JSON, so getters, cycles, and `toJSON` cannot change what is checked. Word lookups use `Map`. A fuzz run of 6,000 type-confused plans threw nothing and polluted no prototype. The module requires only `fs` and `path`, does not log, adds no dependency, and holds no secrets.

---

## Files Changed

Provenance is `INIT-004/SPEC-002` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/plans/validate.js` | Created — schema walker, registry, rule checks (1,160 lines) | `INIT-004/SPEC-002` |
| `test/plan-validate.test.js` | Created — 70 tests (906 lines) | `INIT-004/SPEC-002` |
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | Modified — markup pattern in `plainText.not` and `altText.not` (2 lines) | `INIT-004/SPEC-002` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-002-service-plan-validator.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-002` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-002-eval-report.md` | Created — verifier's per-AC verdicts | `INIT-004/SPEC-002` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-002 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Seven step transitions (tracked in the session todo list instead) | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No event for the inline security review or the prompt-only read-only verifier | Disclosed here and under Deviations |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run (scripts absent) |

---

## Deviations from the Spec

- **Schema asset edited.** The spec's deliverables name only the module and the test. The schema's markup pattern was quadratic (MED-1), and the validator applies the schema, so the pattern is now `<\s*[A-Za-z/!]`. That matches ADR-0003's Security prose: "`<` followed by a name, `/`, or `!`". The new pattern blocks a few strings the old one let through (`</>`, `< /`), and lets through nothing the old one blocked. The spike validator still passes. No other schema field changed, and no `1.1.0` construct was added.
- **A plan over N9 is reported by size alone** (LOW-1). It is refused either way. Plans under N9 get the full report.
- **Further-rule ids.** ADR-0003 numbers only N1–N12, and ADR-0009 claims N13–N19. The registry therefore names the further rules, in ADR table order: `later-context`, `later-checkpoint`, `recall-day`, `semantic-set`, `frequency-band`, `pattern-focus`, `pointed-word`, `romanized-answer`, `questions`, `queue-order`, `meaning`, `image-placement`, `form-recall-image`, `html`, `audio-path`. Structural failures are `schema`. N4, `semantic-set` (reviewer judgment), and `form-recall-image` (no 1.0.0 block asks for the written form) are registered with `checked: false` and are never reported. A test checks the registry's ids, severities, figures, order, and the function list against the ADR text.
- **Interpretations the ADR leaves open:**
  - The N3 tokenizer strips punctuation, symbols, and invisible format marks (`\p{P}\p{S}\p{Cf}`) at token edges.
  - Words match as token runs, by exact key or by a stripped key that names exactly one declared word (ADR-0001). Prefixes are never stripped. Multi-word entries match as a run.
  - N6 counts only the contexts the ADR lists: example sentence, story paragraph, listening passage, dialogue `pointed` and `followUp`, and cloze sentence.
  - N1 and N2 count distinct `exactKey`s.
  - `later-context` compares sentences by their stripped tokens.
  - N9 measures compact JSON.
  - An out-of-range pattern answer index counts as `romanized-answer`, because there is no Hebrew answer.
  - Informs are reported where they apply: `questions` on blocks with questions, `recall-day` on each checkpoint, `frequency-band` when a unit sets one, `queue-order` on personal plans.
- **Notes for SPEC-003:**
  - Call `validatePlan(plan)`; it accepts an object or JSON text. `ok` means no block, so publish may proceed.
  - Refuse a write on any structural or security block (`schema`, `html`, `audio-path`, `image-placement`, N8–N11), not only `schema`.
  - Store compact JSON so the stored bytes match N9's measure. `store.js` keeps the caller's text unchanged.
- **Git:** no commit, push, or PR (the user did not ask). The guard identity list is empty, so the guard is inert.
- **Persona:** `.claude/agents/` does not exist in this repo (`agents_dir_absent`). No persona file was invented. The work followed the spec and the conventions of `server/` and `test/`.
- **Harness (cursor):** the AC verifier used `cursor-grok-4.5-high-fast` (config `model_fast` on cursor). The Task tool has no readonly flag, so the prompt forbade edits; afterwards, no repository file had been changed by the verifier. The security review was inline (`self_reviewed: true`). The compensating control is the initiative-wide Security Review at `complete-initiative`.
- **Quality gates:** no lint, type-check, or coverage tool is configured. `node --check` and the IDE linter are clean, and `npm test` is the gate. No suppression comment was added.
- **Side effects of running the suites:**
  - `spikes/skeleton/test/a11y.test.js` regenerates the untracked `spikes/skeleton/public/index.html` from the café sample.
  - Existing tests rewrite `worksheets/_audio/words/catalog.js`, `worksheets/_review/vocab-index.js`, and `shared/word-audio-catalog.js` with identical bytes (no git diff).
  - This spec does not read `worksheets/` or `LESSONS_DIR`, so `audio:inventory` / `audio:verify` do not apply. No worksheet `index.html` was touched.
- **Research index:** `RSCH-004` and `RSCH-005` are already applied inside ADR-0003. No new research decision.

---

## Effort Tracking

**Agent session time:** not recorded — the timer script is absent and the orchestrator owns it. This dispatch ran from about 10:14 to 11:20 local.
**Spec effort estimate:** 5 hours
