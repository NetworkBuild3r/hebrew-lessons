# SPEC-001 · Docs: ADR-0011 AI-first plan builder

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-008
title: "ADR-0011: AI-first plan builder — operations layer, actor and provenance, revisions, advisory edit state, no pins, issues"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 4 hours
blocked_by: []
blocks: [SPEC-002, SPEC-003, SPEC-008]
```

---

## Description

**Context:** `RSCH-009` recommended pinning any field a person edited and refusing an AI write to it. The operator replaced that on 2026-10-12 with D1–D6 (initiative §2): AI first, no human gate in the authoring flow, no blocking pins, history with one-step restore, an advisory edit note, issues as warnings, one operations layer with MCP primary, a schema-driven editor, and two defect fixes. ADR-0003 (operations, rules), ADR-0004 §10 (audit), and ADR-0005 §5 (one implementation) are the accepted records this extends. ADR-0010 is the highest ADR in `docs/decisions/`; this record is ADR-0011. This repository has no number-claim script, so the executor re-scans `docs/decisions/` immediately before writing and uses the next free number (expected `0011`).

**Scope (in):** write `docs/decisions/ADR-0011-ai-first-plan-builder.md` in the ADR-0003/ADR-0009 format (metadata table, summary, context, requirements, options per decision, decision, consequences, carried-constraints table, security, open decisions, references, changelog). Sections:

1. **Operations layer and the actor.** Every plan write takes a context `{ actor: { kind: 'ai'|'human'|'system', name, tool?, client? }, requestId? }`. The actor is **derived from the channel** and never read from a request: the MCP is `ai` with name `mcp`; an admin session is `human` with the account username; a script is `system` with the script name. State plainly: the MCP uses one shared bearer token, so two AI clients cannot be told apart; `tool` (and `client`, if the SDK exposes it — `RSCH-009` did not check) are labels, not identities. Audit rows: one per plan mutation, publish, restore, create, and assign, written in the same transaction as the write; actors `admin:<username>`, `mcp`, `system:<name>`; `detail` carries path, field, revision, tool, request id, and never a field value.
2. **Revision log.** One row per successful write (including create, publish, restore) with actor, op, path, field, a value-free summary, and the full document gzipped, written in the same transaction as the working copy. A baseline row is added lazily for a plan that predates the log. Retention default: keep everything, no prune (initiative §16 Q1), with measured growth recorded by SPEC-004.
3. **Diff and one-step restore.** A field-level diff over id-addressable nodes between two revisions, or the working copy and the published snapshot. Restore of a whole revision, one part, or one field as a **new** revision, by either actor, in one call. History is never edited. Restore never changes `audience` or `assigneeId`, and never lowers `schemaVersion` from `1.1.0`.
4. **Provenance and the advisory edit state — and why there are no pins.** Provenance lives in a side table, because the plan schema rejects unknown properties and learners receive the snapshot (`RSCH-009` D1 item 1, High). `read_part` returns an `editState`: the fields under that part whose last writer is a person, with the revision, the time, and the value before the person's edit. It is advice and never changes whether a write succeeds. The AI's own write result carries an `overwrote` list when it changed a field a person last wrote (initiative §16 A3; the operator may drop it). The MCP sees `human`, not the admin's username. Options considered, each with its `RSCH-009` evidence grade: (A) pins with a refusal — Low as evidence of effect (vendor and open-source practice only, [47]–[50]), rejected by the operator because it is a gate; (B) a proposal queue — Low to Moderate ([18]–[22]), rejected by D1; (C) advisory note plus history plus restore — chosen, a product choice resting on the operator's priorities, not on a finding. The cost is named: an AI that ignores the note can overwrite a person's fix, and the recovery is the restore.
5. **Issues are first-class.** Five new named rules — `pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale` — each severity `warn`, never `block`, each with a label (product choice or adapted) and the Hebrew evidence grades from `RSCH-009` §1.4 (experts miss sparse pointing errors: Moderate; an LLM auditor does not carry over: Low). Findings carry a stable code, an id path, a field, a message, and a fix hint where one is known. Existing hard errors are unchanged. `list_plan_issues` and the admin issues panel expose them.
6. **Authoring with no human gate.** What D1 removes and what it does not (initiative §2 "What D1 does and does not remove"): hard validation and `pending_media` still refuse; picture approval, lexicon `verified`, and the AI-planning switch stay person-only as carried constraints; the `db_migration` gate was waived by the operator on 2026-10-12 (local-only project, no sign-offs; a backup first is still good practice). The accepted residual risk: the AI can publish and can set a lesson's `visibility`.
7. **Parity.** The capability table (initiative §12) becomes ADR-0005 §5's table extended. Every user action has an MCP tool and an admin route. Names: `list_plans`, `create_plan`, `assign_plan`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `list_plan_issues`.
8. **The editor.** Schema-driven from the plan JSON Schema with a widget registry, per-block hints, a generic JSON fallback, and a drift test that fails when a block type, block field, or lesson field has neither a widget nor an exemption (the `1.1.0` gap is the local evidence). Live preview renders the unsaved form state with `shared/ui/` components. WYSIWYG-first editing on the player render is rejected (Low evidence, high cost, needs the same backend).
9. **Data model.** Migration `006` tables `plan_revisions` and `plan_field_marks`, with column lists and CHECK constraints. No `pinned` or `locked` column. No change to `plans` or to the plan schema.
10. **Changes to earlier decisions.** A table of what is kept, extended, or superseded: ADR-0003 §4 (kept; two writes added), §6 (extended by five named warn rows), §7 (kept; no approval step); ADR-0004 §10 (extended with `plan.*` actions and the `system` actor); ADR-0005 §5 (table extended). `RSCH-009`'s recommendation (pins, `field_pinned`, `keepPinned`, proposal queue) is marked superseded with a pointer to the appended section of that report.

Add metadata rows and one changelog line each to ADR-0003, ADR-0004, and ADR-0005 naming the extended sections (the pattern ADR-0009 used).

**Scope (out):** code; the migration (SPEC-003); the skill and operator guide (SPEC-017); any edit to INIT-004, INIT-005, INIT-006, or INIT-007 files.

---

## Understood (agent's read of the intent)

- [x] und-1: The ADR records the operator's decisions as decisions. It does not re-argue D1–D6. Where the research disagreed (pins), the ADR says so once and cites the appended report section.
- [x] und-2: "No HITL" governs the authoring flow. The `db_migration` gate was waived by the operator on 2026-10-12; the ADR records that waiver in a sentence of its own.
- [x] und-3: Every number is cited to a report section or labeled "product choice". The Hebrew evidence is graded exactly as `RSCH-009` graded it.

## Acceptance Criteria

- [x] ac-1 (REQ-001): `docs/decisions/ADR-0011-ai-first-plan-builder.md` exists with the ten sections above, each with options considered and the chosen option. Status `Accepted` (operator approved INIT-008 on 2026-10-12); previously: Proposed until the operator accepts, then `Accepted`.
- [x] ac-2 (REQ-006): the ADR's provenance section states that no write is refused because of who last wrote a field, names the three options with their evidence grades, states the cost in one sentence, and states that the MCP receives `human` and not a username.
- [x] ac-3 (REQ-003): the ADR states that the actor is derived from the channel, that the bearer token is shared, and lists the audit actions (`plan.create`, `plan.set_field`, `plan.replace_part`, `plan.insert_block`, `plan.move_block`, `plan.remove_block`, `plan.publish`, `plan.restore`, `plan.assign`).
- [x] ac-4 (REQ-008): the ADR names the five rule codes, each as `warn`, with a label and a grade, and states that none blocks a write or a publish.
- [x] ac-5 (REQ-016): the ADR defines the two tables (columns and CHECKs), states that no pin or lock column exists, and states that the `db_migration` gate was waived by the operator on 2026-10-12.
- [x] ac-6 (REQ-001): ADR-0003, ADR-0004, and ADR-0005 each gain one metadata row and one changelog line naming ADR-0011 and the section extended. No other change to those files.
- [x] ac-7 (REQ-017): the ADR lists every standing constraint (no dependency, no build step, `node:sqlite`, tests never call ElevenLabs or OpenAI, admin enforced server-side, no secret in a doc, `worksheets/*/index.html` untouched, `SRS_SCHEDULER` untouched).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | ADR number | 0011: ADR-0010 is the highest on disk as of 2026-10-12. Re-scan before writing. | resolved | yes — directory scan |
| aud-2 | Revision retention | Default keep everything, no prune (initiative §16 Q1). The ADR records the default and the follow-up. | ambiguous | no — planning default applied; the operator may change it at approval and only this section and SPEC-003/004 change |
| aud-3 | `overwrote` list on the AI's write result | Included as advisory (initiative §16 A3), slightly past D2(d)'s wording. | ambiguous | no — planning default applied; removable at approval |

> **Readiness** = 100 against the stated defaults. Each ambiguous row carries a safe default that the specs are written to; a different answer edits one spec before it runs, and initiative §17 asks the operator to confirm them.

---

## Deliverables

- [x] `docs/decisions/ADR-0011-ai-first-plan-builder.md`
- [x] Metadata and changelog lines in ADR-0003, ADR-0004, ADR-0005
- [x] Tests: none (documentation). The verification checklist below.

---

## Technical Requirements (domain-specific)

- [x] Plain style, one claim per sentence, no marketing language (ADR-0003 / ADR-0009 style).
- [x] No real username, learner data, key, or teacher text. Samples use `example-learner` and `example-admin`.
- [x] `pinned` and `locked` appear only in sentences that reject them.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none. The operator approved INIT-008 on 2026-10-12, which accepts ADR-0011 as written; record that date in the ADR metadata and set its status to `Accepted`. SPEC-002, SPEC-003 and SPEC-008 may start once the ADR is written.
- **Blocking checks:** the verification checklist.
- **Rollback plan:** revert the ADR and the three metadata and changelog edits from git.

---

## Security

- **Sensitivity:** standard. The ADR defines who is recorded as the actor, what an audit row may hold, and that a human's name does not reach the MCP.
- **Review depth:** standard · **focus areas:** actor derivation, audit content, the accepted residual risk of AI publish and `visibility`.

---

## Verification Strategy

- **Claim:** ADR-0011 is a complete, citable contract that matches D1–D6, and the three earlier ADRs point to it.
- **Check + executor:** `mechanized` — a search finds ten numbered decision headings; the five rule codes; the nine audit action names; `RSCH-009` section references; and no occurrence of `field_pinned`, `keepPinned`, `pinned_by`, or `pinned_at` outside a sentence that says they are not built. A second search finds `ADR-0011` once in each of ADR-0003, ADR-0004, and ADR-0005's metadata and changelog. `human` — the operator reads Decisions 4 and 6 and accepts, or answers aud-2 and aud-3.
- **Pass condition:** both searches return the stated counts; acceptance recorded in the ADR metadata with the date.

---

## Integration Points

- SPEC-002, SPEC-003, and SPEC-008 cite ADR-0011 by section. Every later spec cites it by section.
- Produces **Change:** ADR-0011; metadata and changelog rows in ADR-0003, ADR-0004, ADR-0005.
- **Provenance tag:** `INIT-008/SPEC-001`
