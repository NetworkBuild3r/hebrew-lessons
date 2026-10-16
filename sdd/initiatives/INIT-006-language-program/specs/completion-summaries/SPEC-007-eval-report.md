# SPEC-007 AC Verification Eval Report — Learner profile contract and text coverage

**Traceability:** INIT-006/SPEC-007
**Verifier:** independent sub-agents that did not author the code · [Verify SPEC-007 acceptance criteria](a4eed042-e043-480e-b8ec-1e002d022c70) (full pass) and [Re-verify SPEC-007 gate after clock fix](4ca1bf47-16fa-46d9-b66a-f09bd355825e) (after the security remediation) · inherited model (the SDD config names `composer-2.5-fast`; this harness uses a different model only when the user names one) · temperature 0 · read-only by prompt (the Task tool has no readonly flag)
**Evaluated at:** 2026-10-10
**Verdict:** PASS · **Remediation loops:** 1 (a security finding, not an AC failure)

---

## Acceptance-criteria results

Overall **PASS** requires **every** row to PASS with concrete evidence. Line numbers moved during the remediation, so evidence below names tests and functions.

| AC | Criterion | Verdict | Evidence (test name + result · file · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | With the switch on, the profile contains every ADR-0009 §1 field, and a recursive key and value walk finds no `NEVER_RETURNED` name, no ISO timestamp, and no numeric user id. | PASS | Test `ac-1: the profile holds every ADR-0009 §1 field…` compares the output's sorted field paths with `RETURNED_FIELDS` by deep equality, in both directions, using a walker written in the test and `profile-contract.fieldPaths`. Test `ac-1: a key and value walk finds no never-returned name…` visits every key and leaf. The verifier checked `RETURNED_FIELDS` and `NEVER_RETURNED` against the two ADR tables by hand. Its own probe found 38 of 38 contract paths and 318 values with no violation, under a stricter time-of-day and number-size rule. That rule was then added to the test. |
| ac-2 | Switch off yields `{ error: 'planning_off' }` only, and an audit row with `fields: none`. | PASS | Tests `ac-2: switch off returns planning_off…` (profile) and `switch off: the MCP gets planning_off…` (coverage) use exact deep equality on the body and on the audit detail `{ outcome: 'planning_off', fields: 'none' }`. Probed with a user id, a username, and a mixed-case username. Re-verified after the clock change. |
| ac-3 | The 61st MCP read in a clock hour, profile and coverage combined, returns `rate_limited` and writes an audit row. | PASS | Test `ac-3: the 61st MCP read in a clock hour…` makes 29 profile and 29 coverage reads, one `planning_off`, and one `not_found` (60 in all). The 61st (profile) and 62nd (coverage) are `rate_limited`, with 62 audit rows, the last audited `fields: 'none'` at the server-clock time. The next UTC hour admits reads again. The verifier probed a coverage read as the 61st after 60 profile reads, and an hour boundary. Re-verified after the clock change. |
| ac-4 | `audience: 'self'` returns a deep-equal object to the `mcp` result, minus `ai_planning`, and writes no audit row. | PASS | Test `ac-4: self returns the MCP object plus ai_planning…` uses `node:assert/strict` deep equality with the switch on and off. It checks the audit count is unchanged and that 5 self reads leave 60 MCP reads available. Both audiences call the same `project()`. Re-verified after the clock change. |
| ac-5 | 20 Hebrew words with 18 known give 0.9, and `unknown` lists the other two. A known word with a leading ו is unknown until a `form` entry with that lemma exists. No `matched_by`. | PASS | Tests `ac-5: twenty Hebrew words with eighteen known…` and `ac-5: a known word with a leading ו…`. The verifier counted the fixture token by token: 18 known by five routes (passed rows, a unique vowel-stripped form, a library-only word, a never-passed library word, a form of a passed lemma), and 2 unknown. It probed ה, ל, ב, and וּ prefixes, which all stay unknown. `coverage.js` has no prefix-stripping code. |
| TR-1 | Projection is an explicit list; a test fails if a new key appears. | PASS | `profile.js` builds every field by name. A search found no spread of model output. The verifier injected junk keys into all three model outputs and the profile's key set did not change. The ac-1 path comparison fails on any extra key. |
| TR-2 | No `SELECT *`. | PASS | The verifier captured SQL at the `node:sqlite` driver level, below the test's own wrapper: 10 distinct statements, all with explicit columns. Test `profile and coverage queries name their columns…`. `lesson-progress.js` `find` and `listByUser` now name their columns. |
| TR-3 | Profile queries never select `password_hash`, `display_name`, or `sessions`. | PASS | `users.js` `findLearnerById` and `findLearnerByUsername` select `id, username, status, ai_planning`. The driver-level capture shows none of the three. |
| SEC-1 | Never-returned walk passes; no timestamps; no ids. | PASS | Same evidence as ac-1, now also checking all three fixture user ids, numbers below 1e6, and time-of-day strings. |
| SEC-2 | Switch-off and rate-limit responses carry no profile field. | PASS | Every refusal is built as `{ error }` in `gate.admit`; the tests use exact deep equality. |
| SEC-3 | Audit rows hold counts and field names only. | PASS | Tests `an ok audit row holds the outcome, field names, and counts only` and `an ok MCP read writes one audit row…` check the exact detail keys, that the detail has no Hebrew letter and no date, and that the coverage row has no text. The verifier scanned every refusal type for both actions. |

## Definition of done

- [x] The profile and coverage outputs match ADR-0009 §1, gated, rate-limited, and audited — ac-1 through ac-5.
- [x] `node --test test/learner-profile.test.js test/learner-coverage.test.js` exits 0 — verifiers and implementer.
- [x] `npm test` exits 0 — verifier run 787/787 (after a transient failure in another agent's in-progress file), implementer final run 789/789.

## Tests run by the verifiers

```text
First pass:
node --test test/learner-profile.test.js test/learner-coverage.test.js
→ 23 pass / 0 fail ; exit 0
npm test → 778 pass / 2 fail ; exit 1 (test/spa-shell.test.js: shared/words-page.js was being created by another agent)
node --test test/spa-shell.test.js → 18 pass / 0 fail ; exit 0
npm test → 787 pass / 0 fail ; exit 0
probe.js 17/17 · mutate.js 18 of 18 injected defects caught · edges.js exit 0 (all under %TEMP%)

After the clock fix:
node --test test/learner-profile.test.js test/learner-coverage.test.js
→ 24 pass / 0 fail ; exit 0
probe under %TEMP%: 61st read with now = 30 days ahead → { error: 'rate_limited' }; audit_log.at = server time
mutation: re-creating the old defect fails the caller-now test and the ac-3 test
```

Neither verifier changed a repository file or opened `data/hebrew.sqlite`.

---

## Remediation loop log

**Loop 1 — security finding SEC-M-01 (Medium).** The hourly MCP count and the audit time came from the caller's `now`. A caller passing a far-future `now` could reset the 60-per-hour count and set the audit time. Fix: `gate.js` reads a server clock (`Date.now`, replaceable only through the test helper `_setClock`). `buildProfile`'s `now` sets only the day strength is measured on. `checkCoverage` takes no clock. New test: `a caller-supplied now does not reset the hourly count or set the audit time`. A fresh verifier re-checked the change and ac-2 to ac-4: 6 of 6 PASS.

**Follow-up from the re-verifier (Low).** A `now` whose conversion throws used up a rate slot and wrote no audit row. Fix: `buildProfile` converts `now` before admission, so a bad argument throws before the call is counted, as a bad `audience` already did. New test: `a now that cannot be read throws before the call is counted`. That two-line change was checked by the test run, not by a third verifier: spec files 25/25, `npm test` 789/789, exit 0.
