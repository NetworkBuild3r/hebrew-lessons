# SPEC-007 · Data: Import the three lessons and the café unit

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-004
title: "Import the three legacy lessons and the café unit"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-003]
blocks: [SPEC-020]
```

---

## Description

**Context:** ADR-0003 §8 and §10 already express the worksheets as sample plans. This spec loads those files into the store. It does not map worksheets again and does not edit them.

**Scope (in):**

- A load script, invoked by tests and by the operator, that reads only:
  - `docs/decisions/ADR-0003-assets/samples/my-morning.plan.json`
  - `docs/decisions/ADR-0003-assets/samples/anal-sex-vocab.plan.json`
  - `docs/decisions/ADR-0003-assets/samples/shai-date-indirect-speech.plan.json`
  - `docs/decisions/ADR-0003-assets/samples/beginner-unit-1.plan.json`
- The three legacy plans are stored `published`, lesson visibility `everyone`, `migration.legacy` left as in the file. Their published snapshot equals the working copy at insert.
- The café unit is stored `draft` with a null snapshot. Lessons stay `hidden` as in the file.
- Every `audioPath` and `sentenceAudioPath` string in the stored JSON equals the same string in the sample, which ADR-0003 §8 says was copied from the worksheet character for character.
- Before the script writes, `npm run audio:inventory`. After, `npm run audio:verify`. The test runs both against a fixture inventory so CI does not depend on the operator's backup path.
- No write under `worksheets/`. No change to any `index.html`.

**Scope (out):** re-recording audio; publishing the café unit; deleting worksheet files (SPEC-020 step 7, and only after the operator's gate).

---

## Understood (agent's read of the intent)

- [x] und-1: If a sample path is absent from a plan, the import does not invent an `.mp3` path.
- [x] und-2: Re-running the import on an existing plan id fails without a second copy and without touching the worksheets.

## Acceptance Criteria

- [x] ac-1 (REQ-005): after import, the three legacy plan ids and `beginner-unit-1` are in the store with the status ADR-0003 §10 names. Validator warnings for N1 on the two lessons over 8 words are recorded and do not refuse the import.
- [x] ac-2 (REQ-005, REQ-013): a test walks every audio field in the stored JSON and in the sample and asserts equal strings. `git diff -- worksheets` in the test's setup is empty of content changes; the test does not write those files.
- [x] ac-3 (REQ-013): `audio:verify` exits 0 against the inventory taken at the start of the test. The test's fake tree is not the live tree; the spec's operator instructions say to run inventory and verify on the real `LESSONS_DIR` when loading a non-test database.
- [x] ac-4 (REQ-014): no `index.html` under `worksheets/my-morning`, `worksheets/anal-sex-vocab`, or `worksheets/shai-date-indirect-speech` is in the diff this spec produces.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Source of the import | The sample JSON, not a fresh parse of `window.WORKSHEET`. The samples are the mapping ADR-0003 already accepted. | resolved | yes — ADR-0003 §8 |

> **Readiness** = 100.

---

## Deliverables

- [x] `scripts/import-lesson-plans.js` (or the repo's existing scripts folder convention)
- [x] Tests: `test/import-lesson-plans.test.js`
- [x] Operator note inside the script's header comment: inventory, import, verify, and the row-delete rollback

---

## Technical Requirements (domain-specific)

- [x] Insert through `server/plans/store.js`. Do not open SQLite from the script with a second wrapper.
- [x] Refuse to run if `validate.js` returns a block failure.
- [x] Rollback documented in the script: delete the four plan rows. That does not delete recordings.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`. Loading the operator's real database is a manual step the operator runs after reading the inventory output. It is not a `db_migration` (no schema change).
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/import-lesson-plans.test.js`, `npm test`, and `npm run audio:verify` on the real tree before the spec is called done.
- **Rollback plan:** delete the four plan rows. Worksheets and `.mp3` files are unmodified, so they need no restore.

---

## Security

- **Sensitivity:** standard. Triggers matched: lesson text includes the existing adult lesson. This spec does not add names, transcripts, or secrets. Visibility of that lesson stays `everyone`, matching ADR-0003 §10.
- **Security acceptance criteria:**
  - [x] The importer reads only the four sample paths.
  - [x] It does not log plan bodies.
- **Review depth:** standard · **focus areas:** path traversal in the sample path, no worksheet writes.

---

## Verification Strategy

- **Claim:** the stored plans match the samples' audio strings and statuses, and the worksheet tree is untouched.
- **Check + executor:** `mechanized` — `node --test test/import-lesson-plans.test.js`, plus `npm run audio:verify` on the operator's tree when the real database is loaded (`human` confirms the command's exit code).
- **Pass condition:** the test exits 0; `audio:verify` exits 0; `git status -- worksheets` shows no modifications from this spec.
- **Test levels:** integration, temporary database. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- Gives SPEC-005 documents to edit. SPEC-020 will not switch a learner link until this import has run.
- Produces **Change:** four plan rows and an import script.
- **Provenance tag:** `INIT-004/SPEC-007`
