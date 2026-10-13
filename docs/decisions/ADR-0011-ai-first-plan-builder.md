# ADR-0011: AI-first plan builder

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-12 |
| **Accepted** | 2026-10-12 by the operator, who approved INIT-008 (D1 to D6) the same day. Open items aud-2 and aud-3 carry the planning defaults. |
| **Authored by** | `INIT-008/SPEC-001` |
| **Initiative** | INIT-008 · ai-first-plan-builder |
| **Research grounding** | `RSCH-009` (technical analysis; §1.1 to §1.6, D3, D4, D8, and the appended "Operator decisions (2026-10-12)" section); `RSCH-006` and `RSCH-008` for the look and icon rules of the admin screens; `RSCH-004` and `RSCH-005` for the plan rules the new checks sit beside; operator decisions D1 to D6, 2026-10-12 |
| **Supersedes** | The recommendation in `RSCH-009` (its "pinned" mark, the refusal error, the `keepPinned` argument, and the proposal queue), for the build only. The report is left as delivered; its appended section lists each replaced item. No ADR is superseded. |
| **Extends** | [ADR-0003](ADR-0003-course-and-lesson-plan-language.md) 4 (two writes, create and restore, join the revision rule), 6 (five named warn rows), 7 (publish has no approval step). [ADR-0004](ADR-0004-learner-data-media-and-secrets.md) 10 (`plan.*` audit actions and the `system` actor). [ADR-0005](ADR-0005-stack-and-architecture.md) 5 (the one-implementation table gains rows). [ADR-0009](ADR-0009-learner-model-and-language-program.md) 6 and 7 (read by the `lexicon-form` rule, not changed). |
| **Superseded by** | - |

## Summary

Today a person can fix a lesson plan only in a builder that knows six block types, and the AI can edit a plan through seven MCP tools but cannot create one, list them, read their history, or undo anything. Two code defects let a write change a part's id or hand a personal plan to any account name. Nothing records who made a plan edit.

This ADR makes the plan builder **AI first** and keeps a person able to fix anything. There is one operations layer and the MCP is the primary way in. Every write carries an actor taken from the channel, and writes one audit row. Every successful write is kept in a revision log, so either actor can diff two revisions and restore one in a single call. An advisory note inside `read_part` tells an AI that a person changed a field and what the earlier value was. The note never refuses a write. No pin and no lock exists. Five new Hebrew and consistency checks report as warnings and never block. The admin editor is generated from the plan JSON Schema, with a generic JSON fallback and a drift test.

The operator waived the `db_migration` gate on 2026-10-12. The one schema change, migration `006`, is additive and has a tested `down()`.

---

## Context

### Problem statement

`RSCH-009` recommended pinning any field a person edited and refusing an AI write to it. On 2026-10-12 the operator replaced that with six decisions. The operator's words: "I wanted all the plans to be built via MCP, but we should allow the admin to modify things that AI may make mistakes," and "it's ai first. No hitl if possible. if an issue is found the human or ai can fix it." This is a local-only project with no other users.

### Current state (read in code on 2026-10-12)

- Six part operations and one `apply()` in `server/plans/ops.js`. One revision integer on the plan. A stale `expectedRevision` returns `revision_conflict` and writes nothing.
- `replace_part` wrote `value` over the node at the path without comparing `value.id` to the node's id. `set_field` and `replace_part` on the plan could write `audience` and `assigneeId`, past the account check in `server/plans/personal.js`.
- The plan store keeps `working_copy` and `snapshot` on one row of `plans`. There is no history and no list function.
- The MCP plan tools are `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, and `publish_plan`. No tool creates a plan, lists plans, or reads history.
- Plan edits and publishes wrote no `audit_log` row. Media and library actions use the fixed actor `mcp`.
- The validator reports `{ rule, severity, path, message }`. It does not check malformed pointing, Hebrew in an English field, a pointed form that differs from the lexicon, the same word pointed two ways, or stale audio.
- The admin builder holds hand-written lists of block types and fields and has no form for the five `1.1.0` blocks.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | No approval step in the authoring flow. The AI builds and may publish. |
| R2 | Either actor can fix any issue at any time. No refusal depends on who edited a field last. |
| R3 | Every plan write names an actor, taken from the channel, and writes one audit row. |
| R4 | History, a field-level diff, and a one-step restore, for both actors. |
| R5 | Issues are first-class: stable codes, warnings, listable, fixable. |
| R6 | Every user action has an MCP tool and an admin route, on one operations layer. |
| R7 | The admin editor covers every block type and cannot silently miss a new one. |
| R8 | The two code defects are closed without a migration. |

### Constraints (standing)

No new npm dependency. No build step. `node:sqlite` only. Tests never call ElevenLabs or OpenAI. Admin checks are enforced on the server. No secret appears in any file, log line, audit row, or revision. `worksheets/*/index.html` is never edited. `SRS_SCHEDULER` is not changed. The server never calls a text model (ADR-0003 aud-4). No learner name, typed answer, or transcript appears in a plan, a revision, a mark, or an audit row.

---

## Options considered

`RSCH-009` D2 compared three architectures. Grades are the report's.

| Option | What it is | Evidence | Outcome |
|--------|------------|----------|---------|
| A. Operations layer with pins | Actor, revision log, a pin on every human-edited field, a refusal error for AI writes to a pinned field | Low as evidence of effect: vendor and open-source practice only (`RSCH-009` [47] to [50]) | Rejected by the operator. A pin is a gate. |
| B. A plus a proposal queue | An AI write to a human-edited part is stored for a person to accept | Low to Moderate (`RSCH-009` [18] to [22]) | Rejected by D1. It is an approval step. |
| C. WYSIWYG-first editing on the player render | Edit text in place on the learner render | Low (`RSCH-009` [35] to [38]) | Rejected. It needs the same backend and has the weakest evidence. |
| **D. Operations layer, advisory note, history, one-step restore** | Actor and audit on every write, a revision log, an advisory `editState`, a restore for both actors | The diagnosis is Moderate: people miss AI errors, and evidence beside the AI's answer beats asking "is it right?" (`RSCH-009` [10], [13], [14], [17]). The mechanism is a product choice. | **Chosen.** |

---

## Decision

### Chosen options (summary)

| # | Decision |
|---|----------|
| 1 | One operations layer. Every write takes a context whose actor comes from the channel, and writes one audit row in the same transaction. |
| 2 | A revision log: one row per successful write, with the document gzipped. |
| 3 | A field-level diff and a one-step restore, usable by both actors. History is never edited. |
| 4 | Provenance in a side table and an advisory `editState`. No pin, no lock, no refusal. |
| 5 | Issues are first-class. Five new rules, all warnings. |
| 6 | Authoring has no human gate. What that does and does not remove. |
| 7 | Parity: every action has an MCP tool and an admin route. |
| 8 | The editor is generated from the plan schema, with a fallback and a drift test. |
| 9 | Data model: migration `006`, two tables. |
| 10 | Changes to earlier decisions, and the carried constraints. |

### 1. Operations layer and the actor

**Options considered:** (1) one operations layer, the actor derived from the channel — **chosen**; (2) the caller states its actor in the request — rejected, a client could claim to be `human`; (3) a separate write path for the MCP — rejected, ADR-0005 5 requires one implementation.

Every plan write takes a context `{ actor: { kind, name, tool?, client? }, requestId? }`. `kind` is `ai`, `human`, or `system`. The adapter builds the context from its own channel. No request body key, no MCP argument, and no JSON value can set it.

| Channel | Kind | Name | Audit actor string |
|---------|------|------|--------------------|
| MCP tool call | `ai` | `mcp` | `mcp` |
| Admin session | `human` | the signed-in account's username | `admin:<username>` |
| Script | `system` | the script or subsystem name | `system:<name>` |

A write with no valid context throws `TypeError` before it reads or writes anything. The MCP uses one shared bearer token (`MCP_AUTH_TOKEN`), so two AI clients cannot be told apart. `tool` is the tool name. `client` is a label and not an identity, and it is recorded only if the SDK passes it. `INIT-008/SPEC-002` checked: the installed SDK does not pass `clientInfo` to a tool handler, and this server creates a new `McpServer` per request without `initialize`, so no `client` label is recorded. The handler's second argument carries the JSON-RPC request id. It is recorded as `requestId`, a client-chosen label.

**Audit.** One `audit_log` row per plan create, mutation, publish, restore, and assign, written in the same `db.transaction` as the plan write. If the audit insert fails, the plan write rolls back. A refused write (conflict, validation, `id_mismatch`, `assign_required`) writes no row. `target` is the plan id. `detail` is compact JSON holding the revision, path, field name, tool, and request id. It never holds a field value or document text.

| Action | Written by |
|--------|------------|
| `plan.create` | `create_plan` (SPEC-007) |
| `plan.set_field` | `set_field` |
| `plan.replace_part` | `replace_part` |
| `plan.insert_block` | `insert_block` |
| `plan.move_block` | `move_block` |
| `plan.remove_block` | `remove_block` |
| `plan.publish` | `publish_plan` |
| `plan.restore` | `restore_plan_revision` (SPEC-005) |
| `plan.assign` | `assign_plan` |

**The two defects.** `replace_part` on a part refuses a value whose `id` differs from the id in the path (`id_mismatch`). `set_field` of `id` on a part is refused the same way. `set_field` and `replace_part` on the plan cannot change `audience` or `assigneeId` (`assign_required`). Those two fields change only through `assignPlan`, which `personal.assign` calls after its active-account check. An adapter reaches it only through `personal.assign`.

### 2. Revision log

**Options considered:** (1) one row per successful write, holding the full document gzipped — **chosen**; (2) event sourcing — rejected, the integration cost with audio files and media rows is real (`RSCH-009` [45]) and a snapshot per revision restores without replay; (3) keep only publish snapshots — rejected, it cannot undo an AI edit made before publish.

One `plan_revisions` row per successful write, including create, publish, and restore. The row holds the actor, the operation, the path, the field, a value-free summary, the gzipped full document, and its SHA-256. It is written in the same transaction as the working copy. A plan that predates the log gets a `baseline` row on its first write, so restore to "before the first edit" works.

**Retention (aud-2).** Keep every revision. There is no prune function and no job. A document is capped at 512 KiB by N9, and gzip makes a revision a fraction of that. SPEC-003 recorded measured sizes: the four ADR-0003 sample plans are 7,844 to 15,558 bytes compact and 1,809 to 4,852 bytes gzipped. A prune function is a follow-up if the table grows.

### 3. Diff and one-step restore

**Options considered:** (1) a field-level diff over id-addressable nodes, and a restore as a new revision — **chosen**; (2) a line diff of the JSON text — rejected, ids and small fields make a per-field compare simpler than a line merge (`RSCH-009` [44]); (3) an automatic merge — rejected by ADR-0003 4.

`diff_plan` compares two revisions, or the working copy against the published snapshot, and returns path, field, before, after, and actor. Restore takes a whole revision, one part, or one field, and writes the result as a **new** revision, in one call, for either actor. History is never edited. Restore goes through the same validation as any write and never writes an invalid document. Restore never changes `audience` or `assigneeId`, even on a whole-document restore, and never lowers `schemaVersion` from `1.1.0`.

### 4. Provenance and the advisory edit state — and why there are no pins

**Options considered**, each with its `RSCH-009` grade: (A) a pin with a refusal — Low as evidence of effect, rejected by the operator because it is a gate; (B) a proposal queue — Low to Moderate, rejected by D1; (C) an advisory note, plus history, plus restore — **chosen**. It is a product choice resting on the operator's priorities, not on a finding.

Provenance lives in a side table, because the plan schema rejects unknown properties and learners receive the snapshot (`RSCH-009` D1, High). `plan_field_marks` records, per plan, id path, and field, who last wrote it and when a person last wrote it.

`read_part` returns an `editState` and a `lastHumanEdit` note: the fields under that part whose last writer is a person, with the revision, the time, and the value before the person's edit. It is advice. **No write is refused because of who last wrote a field.** The AI's own write result carries an `overwrote` list when it changed a field a person last wrote (aud-3, slightly past the operator's wording; removable in one function). No `field_pinned` error, no `keepPinned` argument, and no `pinned_by` or `pinned_at` column exists. No column or table is named `pinned`, `locked`, `pin`, or `lock`.

The MCP receives the kind `human`, not the admin's username. An admin's name is shown to admins only.

**Cost.** An AI that ignores the note can overwrite a person's fix. The recovery is the one-step restore.

### 5. Issues are first-class

**Options considered:** (1) named warn rules with stable codes, a list tool, and an admin panel — **chosen**; (2) block publish on them — rejected by D3; (3) an LLM audit on the server — rejected, the server never calls a text model (ADR-0003 aud-4), and the one audit result does not carry over (Low, `RSCH-009` [17]).

| Code | Severity | What it checks | Label | Grade |
|------|----------|----------------|-------|-------|
| `pointing-form` | warn | Malformed pointing: two vowels on one letter, a point with no letter | product choice | Moderate that experts miss sparse pointing errors (`RSCH-009` [17]) |
| `field-language` | warn | Hebrew in an English field, or none in a Hebrew field | product choice | Low: no study of the check |
| `lexicon-form` | warn | A pointed form, `meaning`, or `translit` that differs from the lexicon entry with the same stripped key | adapted from ADR-0009 6 | Moderate that AI pointing needs verification (`RSCH-009` [55] to [57]) |
| `pointing-consistency` | warn | The same word pointed two ways in one lesson | product choice | Low: no study of the check |
| `audio-stale` | warn | A text field differs from the published snapshot while its `audioPath` is the same | product choice | Low: it cannot tell a clip that was already stale at publish |

**None of the five blocks a write or a publish**, in any setting. Each finding carries a stable code, an id path, a field, a message, and a fix hint where one is known. Existing hard errors (`schema`, `html`, `audio-path`, `image-placement`, N8 to N11 on write; `meaning`, N1, N5, `romanized-answer`, N13, N19 on publish) are unchanged. `list_plan_issues` returns every finding the validator produces, hard errors included. A person who reads Hebrew confirms the pointing table. Pointing that is well formed but wrong in context stays with the lexicon `verified` flag and a person.

### 6. Authoring with no human gate

**Options considered:** (1) no gate between the AI and a published plan, the validator and `pending_media` still refuse — **chosen**; (2) a person approves each publish — rejected by D1.

- **Removed:** any approval step between the AI and a published plan, any proposal queue, any pin or lock.
- **Kept, not an authoring gate:** hard validation. A plan the validator blocks does not publish. A publish that references a pending picture on an `everyone` or `selected` lesson is refused (`pending_media`).
- **Kept, unchanged:** a person approves picture bytes and alt text (ADR-0004); only a person marks a lexicon form `verified` (ADR-0009 6); only an admin turns the AI-planning switch on. The MCP cannot do any of the three.
- **Waived:** the operator waived the `db_migration` gate on 2026-10-12. This is a local-only project with no other users, and no rule requires a person to sign off a database migration. Migration `006` still gets a `scripts/backup-db.js` copy when it is applied to the real database, because that is cheap.

**Accepted residual risk.** A new lesson defaults to `visibility: hidden` (ADR-0003 7), but `visibility` is an ordinary field the AI may set. The AI can therefore publish a plan and expose it to learners with no person involved. The protection is the audit row, the revision log, and one-step restore. There is no gate.

### 7. Parity

**Options considered:** (1) every user action has an MCP tool and an admin route, a parity test enforces it — **chosen**; (2) UI-only actions — rejected by D4.

The table extends ADR-0005 5. The MCP is primary and the admin UI is a client of the same functions.

| Action | Function | MCP tool | Admin route |
|--------|----------|----------|-------------|
| List plans | `listPlans` | `list_plans` | `GET /api/plans` |
| Create a plan | `createPlan` | `create_plan` | `POST /api/plans` |
| Assign a plan | `personal.assign` | `assign_plan` | `POST /api/plans/:id/assign` |
| List revisions | `revisions.list` | `list_plan_revisions` | `GET /api/plans/:id/revisions` |
| Read one revision | `revisions.get` | `get_plan_revision` | `GET /api/plans/:id/revisions/:revision` |
| Diff | `diff.between` | `diff_plan` | `GET /api/plans/:id/diff` |
| Restore | `revisions.restore` | `restore_plan_revision` | `POST /api/plans/:id/restore` |
| List issues | `issues.list` | `list_plan_issues` | `GET /api/plans/:id/issues` |

The existing tools keep their names and shapes; new keys are additive. Reading the schema for the editor is UI plumbing, a read helper with no MCP tool: `GET /api/plan-schema`. The MCP uses one shared bearer token, so the actor is derived from the channel and two AI clients stay indistinguishable. MCP resources and prompts are out of scope, because the MCP spec leaves resources to the host application and the edit state comes back inside tool results.

### 8. The editor

**Options considered:** (1) a form generated from the plan JSON Schema, with a widget registry, per-block hints, a generic JSON fallback, and a drift test — **chosen**; (2) the hand-written lists that exist today — rejected, the `1.1.0` gap is the local evidence that they drift; (3) WYSIWYG-first on the player render — rejected (Low evidence, high cost, needs the same backend).

A drift test fails when a schema block type, block field, or lesson field has neither a widget nor an explicit exemption, and it runs against a mutated schema so it cannot pass by being blind. An uncovered node is editable as JSON from day one. The editor creates a plan from scratch and edits every block type, including `explainer`, `conjugation-table`, `personal-question`, `verse-reading`, and `letter-intro`, and the lesson `recipe`, `newWords`, `buildsOn`, `recycles`, and `visibility`. A live preview renders the unsaved form state with `shared/ui/` components and reads the saved working copy through the existing admin read. No preview route is added, and `GET /api/plans/:id/published` keeps answering 404 for a draft. The editor sends the same operations as the MCP and holds no local JSON patch (INIT-004 SPEC-015 ac-1).

### 9. Data model

**Options considered:** (1) two side tables in one additive migration — **chosen**; (2) provenance inside the plan document — rejected, the schema rejects unknown properties and learners receive the snapshot; (3) a new column on `plans` — rejected, a history needs rows.

Migration `006` (`server/db/migrations/006-plan-revisions.js`) adds two tables and two indexes. `plans`, `audit_log`, and the plan schema are not changed. Foreign keys follow 005: `REFERENCES plans(id) ON DELETE CASCADE`.

`plan_revisions`:

| Column | Type and constraint |
|--------|---------------------|
| `plan_id` | TEXT NOT NULL, foreign key to `plans(id)` |
| `revision` | INTEGER NOT NULL, at least 1. Primary key with `plan_id`. |
| `actor_kind` | TEXT NOT NULL, CHECK in `ai`, `human`, `system` |
| `actor_name` | TEXT NOT NULL, 1 to 128 characters |
| `tool`, `client`, `request_id` | TEXT, nullable |
| `op` | TEXT NOT NULL, CHECK in `create`, `baseline`, `set_field`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `assign`, `publish`, `restore` |
| `path`, `field` | TEXT, nullable |
| `summary` | TEXT NOT NULL, at most 500 characters, holds no field value |
| `document_gz` | BLOB NOT NULL |
| `document_sha256` | TEXT NOT NULL, 64 characters |
| `created_at` | TEXT NOT NULL |

Index `idx_plan_revisions_plan_created` on `(plan_id, created_at)`.

`plan_field_marks`:

| Column | Type and constraint |
|--------|---------------------|
| `plan_id`, `path`, `field` | TEXT NOT NULL. Primary key of the three. `plan_id` is a foreign key to `plans(id)`. |
| `last_actor_kind` | TEXT NOT NULL, CHECK in `ai`, `human`, `system` |
| `last_actor_name` | TEXT NOT NULL, 1 to 128 characters |
| `last_revision` | INTEGER NOT NULL, at least 1 |
| `last_at`, `value_hash` | TEXT NOT NULL |
| `human_revision` | INTEGER, at least 1 when set |
| `human_at`, `human_actor_name` | TEXT, nullable. The last time a person wrote this field, kept after an AI overwrote it. |
| `previous_value_json` | TEXT, nullable, at most 32 KiB. The value the most recent human edit replaced. |
| `previous_writer_kind` | TEXT, nullable, CHECK in `ai`, `human`, `system`, `unknown` |

Index `idx_plan_field_marks_last_actor` on `(plan_id, last_actor_kind)`.

**No `pinned` or `locked` column exists.** `down(db)` drops both tables and both indexes. The `db_migration` gate was waived by the operator on 2026-10-12. Plans that exist today have no log rows until their first write after the migration. The container applies the migration on restart.

### 10. Changes to earlier decisions and carried constraints

| Source | Wording in force before | Status after this ADR |
|--------|-------------------------|-----------------------|
| ADR-0003 4, six operations, one revision integer, no merge | Six operations. Any success increments the revision by 1. | **Kept.** Two writes are added on the same revision rule: create and restore. `assign` is a seventh function on that rule. No merge. |
| ADR-0003 6, rule registry (block, warn, inform; each labeled) | N1 to N19 and the named rows | **Extended.** Five named warn rows, labeled in Decision 5. None blocks. |
| ADR-0003 7, draft and publish; a new lesson is `hidden` | Publish snapshots the working copy | **Kept.** Publish has no approval step (Decision 6). |
| ADR-0003 aud-4, the server never calls a text model | As written | **Kept.** |
| ADR-0004 10, one `audit_log` row per action; actors `admin:<username>` and `mcp` | Media and snapshot actions | **Extended.** `plan.*` actions (Decision 1) and the actor `system:<name>`. Same table, same format. |
| ADR-0004 1 and 8, a person approves picture bytes and alt text | As written | **Kept, unchanged.** |
| ADR-0005 5, one implementation, adapters authenticate, call, and map | Six functions and `publishPlan` | **Extended.** The table in Decision 7 adds rows. A parity test (SPEC-010) enforces it. |
| ADR-0005 8, migrations are additive; backup first | As written | **Kept.** One additive migration with a tested `down()`. |
| ADR-0009 6, only a person sets `verified` | As written | **Kept.** The `lexicon-form` rule reads it. Nothing here writes it. |
| ADR-0009 7, rules N13 to N19, schema `1.1.0` | As written | **Kept.** The new rules are named and not numbered, as ADR-0003 names rules outside N1 to N12. |
| `RSCH-009` recommendation (pins, a refusal error, `keepPinned`, the proposal queue) | Recommendation Architecture A and B | **Superseded for the build.** See the appended section of the report. |

**Carried constraints.**

| Source | Constraint | In force for this build |
|--------|------------|-------------------------|
| ADR-0001 #3 | No edits to lesson `index.html` | Kept |
| ADR-0001 #6 | Bilingual UI labels | Kept on every new admin screen |
| ADR-0001 #7 | No names, transcripts, or secrets in lesson files | Kept, extended to revisions, marks, and audit rows |
| ADR-0002 | Server owns the review schedule; `SRS_SCHEDULER` untouched | Kept |
| ADR-0005 | No new dependency, no build step, `node:sqlite` | Kept |
| ADR-0005 7 | Test hooks; Chrome smokes change with their hook | Kept |
| ADR-0006, ADR-0008, ADR-0010 | Tokens, floors, reduced motion | Kept on every new admin screen |
| ADR-0007 | Route table and view contract | Kept |
| Project | Tests never call ElevenLabs or OpenAI; admin enforced server-side; no secret in a doc; `worksheets/*/index.html` untouched | Kept |

---

## Consequences

### Positive

- Every plan write has a named actor, an audit row, and a revision that can be restored in one call.
- The AI can create, list, diff, restore, and read issues for a plan without a person.
- A person can fix anything, and the next AI read says what the person changed.
- The two authorization and id defects are closed with no migration.

### Negative / trade-offs

- An AI that ignores `editState` can overwrite a person's fix and can publish and expose a lesson. The audit row, the revision log, and restore recover it. There is no gate.
- One shared bearer token: every MCP caller is `mcp`, and two AI clients cannot be told apart.
- The revision table only grows. Gzip bounds each row; the prune function is a follow-up.
- A human text fix leaves the old audio clip attached. The `audio-stale` warning names it, and the existing media tools fix it. No new media feature is added.

### Follow-ups (out of scope here)

A proposal queue if the operator ever wants one. A worksheet-to-plan converter and per-lesson cutover. An AI identity per client. A revision prune. A real-player preview of a draft, which needs a new route that must not leak drafts.

### Rollback

Revert the code specs. `down(db)` of migration `006` drops the two tables. The dated `scripts/backup-db.js` copy restores the database. The audit rows stay; they are plain rows in the existing table.

---

## Implementation plan (downstream)

SPEC-002 hardens the operations and adds the context and audit rows. SPEC-003 adds migration `006`. SPEC-004 to SPEC-007 write the log, diff, restore, marks, `editState`, `createPlan`, and `listPlans`. SPEC-008 adds the issue rules. SPEC-009 and SPEC-010 add the MCP tools and the admin routes with the parity test. SPEC-011 to SPEC-015 build the editor. SPEC-016 is the end-to-end check. SPEC-017 writes the operator guide, the skill update, and the rollout runbook.

---

## Security considerations

### Authentication and authorization

The actor is derived from the channel. A client cannot claim to be `human`. Every new route sits behind the session check and the admin check, and the MCP tools require the token. `restore` cannot set `audience` or `assigneeId`. The MCP still cannot mark a lexicon form `verified`, approve a picture, or turn the AI-planning switch on.

### Data protection

An audit row holds the actor, the action, the plan id, the revision, the path, the field name, the tool, and the request id. It never holds a field value. A revision row holds a copy of a document, so it holds nothing a plan may not hold (the `html` rule, N9 to N11). An admin's username is shown to admins and is not returned to the MCP. No secret and no learner data is stored in any new table.

### Residual risks (accepted)

1. **AI publish and `visibility`.** The AI can publish and expose a lesson with no person involved (Decision 6).
2. **Overwrite of a human fix.** An AI that ignores `editState` can overwrite it (Decision 4).
3. **Coarse identity.** One shared token (Decision 1).
4. **`pointing-form` on valid Hebrew.** It is a warning and never blocks. A person who reads Hebrew confirms the table.

---

## Open decisions (planning defaults applied)

| Id | Item | Value applied | Why it was open |
|----|------|---------------|-----------------|
| aud-2 | Revision retention | Keep every revision, no prune | Real production plan sizes were not measured (initiative 16 Q1). |
| aud-3 | `overwrote` list on the AI's write result | Included as advisory | It goes slightly past the operator's wording ("inside `read_part`"). Removable in one function. |

---

## References

- `sdd/research/RSCH-009-worksheet-builder-design/RSCH-009-worksheet-builder-design-report.md` - 1.1 to 1.6, D1 to D8, and the appended "Operator decisions (2026-10-12)".
- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` and `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` - the plan rules the new checks sit beside.
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md)
- [ADR-0004: Learner data, media, and secrets](ADR-0004-learner-data-media-and-secrets.md)
- [ADR-0005: Stack and architecture](ADR-0005-stack-and-architecture.md)
- [ADR-0009: Learner model and language program](ADR-0009-learner-model-and-language-program.md)
- `sdd/initiatives/INIT-008-ai-first-plan-builder/INIT-008-ai-first-plan-builder-initiative.md`
- `sdd/initiatives/INIT-008-ai-first-plan-builder/specs/SPEC-001-docs-adr-0011-ai-first-plan-builder.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-12 | Initial record and acceptance - `INIT-008/SPEC-001`. The operator approved INIT-008 (D1 to D6) the same day, and waived the `db_migration` gate for migration `006`. Open items aud-2 and aud-3 carry the planning defaults. |
