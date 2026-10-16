# SPEC-006 · Service: Provenance marks and the advisory edit state in `read_part`

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-008
title: "Record who last wrote each field; return an advisory editState in read_part; never refuse a write because of it"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-007]
blocks: [SPEC-009]
```

---

## Description

**Context:** `RSCH-009` recommended pinning a field a person edited and refusing an AI write to it. The operator decided against any blocking pin (D2). In its place: a provenance side table (SPEC-003's `plan_field_marks`), and an **informational** `editState` and `lastHumanEdit` inside `read_part` results, so an AI that re-edits a human-fixed field sees that a person changed it and what the value was before (D2(d)). The MCP spec leaves resources to the host, so the note rides inside the tool result (`RSCH-009` §1.5, inference). Marks live beside the plan because the plan schema rejects unknown properties and learners receive the snapshot. ADR-0011 Decision 4 is the contract.

**Scope (in):**

- `server/db/repos/plan-field-marks.js` — SQL only: `upsert`, `forPlan`, `forPathPrefix`, `deleteUnder(planId, pathPrefix)` using `substr` comparison, never `LIKE` with an unescaped id; `humanCounts()` for plan listing (SPEC-007); `lastHuman(planId)`.
- `server/plans/provenance.js`:
  - `recordWrite(planId, before, after, ctx, revision)` runs **inside the same `db.transaction`** as the write, the revision row, and the audit row. It calls `diffDocuments(before, after)` (SPEC-005) and, for each `changed` entry, upserts a mark keyed `(plan_id, path, field)`: `last_actor_*`, `last_revision`, `last_at`, `value_hash` (SHA-256 of the field's stable JSON). When the actor is `human`: set `human_revision`, `human_at`, `human_actor_name`, `previous_value_json` = the entry's `before` (NULL when over 32 KiB), and `previous_writer_kind` = the prior mark's `last_actor_kind`, or `'unknown'` when there was no mark (plans and fields that predate marks, and script-written content). When the actor is `ai` or `system`: update `last_*` and keep the `human_*` columns as a record of the last person's edit. `removed` nodes call `deleteUnder`; `added` and `moved` write no mark.
  - `editStateFor(planId, path, ctx)` returns, for a read at `path` (empty = whole plan): `{ advisory: true, lastHumanEdit: { revision, at } | null, humanEdited: [{ path, field, revision, at, previousValue, previousWriter }], more }`. `humanEdited` lists the marks at or below `path` whose **current last writer is a person**, newest first, at most 50, with `more` the remainder count; `lastHumanEdit` is the newest human edit **anywhere in the plan**. For an `ai` caller no username appears (the entry says nothing about who); for an `admin` caller each entry also carries `by`. A field an AI overwrote after a person is not listed as human-edited; the history shows both writes.
  - `overwrittenBy(planId, changedEntries, ctx)` runs **before** `recordWrite` for an `ai` or `system` write and returns `overwrote: [{ path, field, humanRevision, valueSetByPerson }]` (max 20) for every changed field whose mark's current last writer was a person. It is returned on the **write result** (`replacePart`, `setField`, `restore`, and the other writes).
- `ops.readPart(store, id, input, ctx)` returns `{ part, revision, editState }`. The HTTP read route returns the same keys. `editState` is added to the result and never changes `part` or `revision`.
- **No refusal, anywhere.** No function in this spec returns an error, throws, or changes a write's outcome because of a mark. A write by any actor to any field succeeds or fails on exactly the same grounds as before this spec (revision, validation, id mismatch, assign). The words `pinned` and `locked` are not used as an identifier, column, error code, or user-facing string in any plan module, route, or tool.
- A restore (SPEC-005) goes through the same persist transaction, so it records marks with the restoring actor.
- `listPlans` (SPEC-007) gains `humanEditedFields` per plan: the count of marks whose current last writer is a person, from one grouped query (`humanCounts()`), not one query per plan.

**Scope (out):** the MCP and HTTP exposure beyond the `read_part` result (SPEC-009, SPEC-010); the UI badges (SPEC-014); any gate.

---

## Understood (agent's read of the intent)

- [x] und-1: `editState` advises. An AI may read it and edit anyway. The recovery is the restore, not a block.
- [x] und-2: Marks are computed from the same field-level diff that powers history, so there is one differ, not two.
- [x] und-3: A missing mark means "unknown", never "AI wrote it". The note says `previousWriter: 'unknown'` for content that predates marks.
- [x] und-4: The `overwrote` list on a write result is the one thing past D2(d)'s wording (initiative §16 A3). It is one function, `overwrittenBy`, and removing it changes nothing else.

## Acceptance Criteria

- [x] ac-1 (REQ-006): an AI sets a word's `meaning` (revision 2); an admin changes it (revision 3). `readPart` for that word with an `ai` context returns an `editState.humanEdited` entry with the word's id path, `field: 'meaning'`, `revision: 3`, `previousValue` equal to the AI's text, and `previousWriter: 'ai'`; `lastHumanEdit.revision` is 3; no username appears in the result. The same read with an admin context also carries `by: 'example-admin'`.
- [x] ac-2 (REQ-006): after the AI then overwrites that field (revision 4), the field is no longer in `humanEdited`; `lastHumanEdit` still reports revision 3; the write result of revision 4 carries `overwrote` naming the path, field, revision 3, and the person's value; the history lists revisions 2, 3, and 4 with their actors.
- [x] ac-3 (REQ-006): content with no mark that a person then edits gets `previousWriter: 'unknown'`; a plan read before any human edit returns `humanEdited: []` and `lastHumanEdit: null`.
- [x] ac-4 (REQ-006): **no refusal.** For every combination of actor kind (`ai`, `human`, `system`) and last-writer kind (`ai`, `human`, `system`, none), the same write is accepted and produces the same document; a write that fails fails only with `revision_conflict`, `validation`, `id_mismatch`, `assign_required`, `not_found`, or `invalid_op`. A test lists every error string the plan modules can return and finds none that mentions a mark, a person, a pin, or a lock.
- [x] ac-5 (REQ-006): a source and metadata search over `server/plans/`, `server/db/repos/plan-*.js`, `server/db/migrations/006-*`, `server/api/plan-routes.js`, and the registered plan tools' names and descriptions finds no occurrence of `field_pinned`, `keepPinned`, `pinned_by`, `pinned_at`, and no column or key named `pinned` or `locked`.
- [x] ac-6 (REQ-006): a removed block deletes the marks at and under its path; an added block writes none; a moved block writes none; a restore by an admin records `human` marks for the restored fields.
- [x] ac-7: a field value over 32 KiB written by a person stores `previous_value_json` NULL and `editState` reports `previousValue: null` with `previousValueOmitted: true`; a list of 60 human-edited fields returns 50 entries and `more: 10`; `humanEdited` is ordered newest first.
- [x] ac-8 (REQ-003): marks are written in the same transaction as the working copy, revision row, and audit row: a stubbed failure of the marks upsert rolls all four back.
- [x] ac-9 (REQ-007): `listPlans` over a database with one plan holding two human-edited fields, one holding a field a person edited and an AI then overwrote, and one never edited returns `humanEditedFields` 2, 0, and 0, from a single grouped query (asserted by a statement counter).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Human name to the MCP | Not returned: an `ai` caller sees `human` and no username. | ambiguous | no — planning default applied (initiative §16 A2; data minimization); the operator may reverse it by one condition in `editStateFor` and `revisions.list` |
| aud-2 | `overwrote` on write results | Included, advisory (initiative §16 A3). | ambiguous | no — planning default applied; removable at approval |
| aud-3 | Mark granularity | Field-level on id-addressable nodes, from the SPEC-005 diff. A change inside a non-bucket array is a change to that array's field. | resolved | yes — same granularity as the diff |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `server/db/repos/plan-field-marks.js`, `server/plans/provenance.js`
- [x] Changes to `server/plans/ops.js` (`persist`, `readPart`), `server/plans/create.js` (`listPlans`), and the HTTP read route's result mapping
- [x] Tests: `test/plan-provenance.test.js`, `test/plan-no-refusal.test.js`

---

## Technical Requirements (domain-specific)

- [x] Prepared statements; `substr` prefix compare; hashes from `node:crypto`; no dependency.
- [x] Temporary database in tests. Fixtures use `example-admin`.
- [x] `editState` is bounded (50 entries) and contains no more than the field value a caller can already read through `read_part`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the two test files and `npm test`.
- **Rollback plan:** revert the commit; marks tables stay unused; `read_part` loses one key.

---

## Security

- **Sensitivity:** standard. Triggers matched: identity of a writer derived from the channel; a previous value returned to the MCP.
- **Review depth:** standard · **focus areas:** no username to the MCP, no refusal path, bounded output.

---

## Verification Strategy

- **Claim:** an AI that reads a part sees which fields a person last changed and what they replaced, and nothing about who wrote a field ever causes a write to fail.
- **Check + executor:** `mechanized` — `node --test test/plan-provenance.test.js test/plan-no-refusal.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-8 each asserted; ac-4's matrix covers all twelve actor and last-writer combinations; ac-5's search returns zero hits.
- **Test levels:** unit for `editStateFor` shaping and caps; integration on a temporary database for the write, mark, and read cycle through the real operation functions.
- **Property:** the matrix test is a property test because "never refuses" is a statement over all combinations; it earns its keep here.

---

## Integration Points

- SPEC-007's `listPlans` gains `humanEditedFields` here. SPEC-009 and SPEC-010 return `editState` unchanged. SPEC-014 renders it. SPEC-016 asserts the AI-first loop end to end.
- Produces **Change:** provenance marks and the advisory edit state.
- **Provenance tag:** `INIT-008/SPEC-006`

---

## Executor notes (2026-10-12)

- **`overwrote` is one function.** `provenance.overwrittenBy`, called from one line in `server/plans/write-log.js`. Delete both and nothing else changes. The key is present on a write result only when the list is not empty.
- **Name rule is one condition.** `showNames(ctx)` in `provenance.js` (and `shapeMeta` in `revisions.js`): a name goes to an admin or a script context and never to an `ai` context or a read with no context.
- The plan's `status` is bookkeeping (every edit sets it to `draft`, a publish sets it back), so it never gets a mark.
- A field a person newly adds has `previous_value_json` `'null'` with `previousWriter` set as usual; `previousValueOmitted` is reserved for a value over 32 KiB.
- To make `read_part` carry `editState` end to end, the two existing adapters were touched and nothing else: the admin read route and the `read_part` tool now pass a context (admin and `mcp`), `sendMapped` forwards `editState` and `overwrote`, and the `read_part` description mentions the note. No tool or route was added.
