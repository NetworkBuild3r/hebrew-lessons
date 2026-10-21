# SPEC-001 completion summary

**Spec:** SPEC-001 · Docs: ADR-0011 AI-first plan builder  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (the ADR is written and marked Accepted on the operator's approval of INIT-008; the operator can still amend it)

## What shipped

- **`docs/decisions/ADR-0011-ai-first-plan-builder.md`.** Re-scanned `docs/decisions/` before writing: ADR-0010 is the highest, so 0011 was free and is used. Status `Accepted`, dated 2026-10-12, with the acceptance recorded in the metadata. Ten numbered decisions, each with options considered and the chosen option:
  1. Operations layer and the actor (derived from the channel; shared bearer token limitation stated; the nine audit actions; the two defects).
  2. Revision log (keep every revision, no prune).
  3. Diff and one-step restore (restore never changes `audience` or `assigneeId`, never lowers `schemaVersion`).
  4. Provenance and the advisory edit state, and why there are no pins (three options with their `RSCH-009` grades, the cost in one sentence, the MCP sees `human` and not a username).
  5. Issues are first-class (the five rule codes, each `warn`, with a label and a grade; none blocks).
  6. Authoring with no human gate (what is removed, what is kept, the accepted residual risk of AI publish and `visibility`).
  7. Parity (the eight new tool and route names).
  8. The editor (schema-driven, fallback, drift test, live preview).
  9. Data model (both tables with columns and CHECKs, no pin or lock column).
  10. Changes to earlier decisions, and the carried constraints.
- **The waiver is in its own sentence.** "The `db_migration` gate was waived by the operator on 2026-10-12." appears in Decision 9, and again in Decision 6 and in the Summary.
- **ADR-0003, ADR-0004, ADR-0005** each gained one metadata row ("Extended by", naming the sections) and one changelog row. Nothing else in those files changed (`git diff --stat`: 2 lines added in each).
- **Two statements the ADR makes as facts from SPEC-002**, both checked in code: the installed MCP SDK does not give a tool handler the `clientInfo`, and this server is stateless (a new `McpServer` per POST, no `initialize`), so no `client` label is recorded. The MCP request id is recorded as `requestId`.
- **Retention measurement from SPEC-003** is quoted in Decision 2 (sample plans, 7,844 to 15,558 bytes compact, 1,809 to 4,852 bytes gzipped).

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. File exists, ten decision headings, status `Accepted`, options and a chosen option in each. |
| ac-2 | Pass. Decision 4 states that no write is refused because of who last wrote a field, names the three options with grades, states the cost in one sentence, and states that the MCP receives `human` and not a username. |
| ac-3 | Pass. Decision 1 states the channel-derived actor, the shared token, and lists the nine audit actions. |
| ac-4 | Pass. Decision 5 names the five codes, each `warn`, with a label and a grade, and says none blocks a write or a publish. |
| ac-5 | Pass. Decision 9 gives both tables with columns and CHECKs, says no pin or lock column exists, and states the waiver. |
| ac-6 | Pass. One metadata row and one changelog row naming ADR-0011 and the section extended, in each of ADR-0003, ADR-0004, ADR-0005. |
| ac-7 | Pass. The "Constraints (standing)" paragraph and the carried-constraints table list the dependency, build step, `node:sqlite`, ElevenLabs and OpenAI, server-side admin, secrets, `worksheets/*/index.html`, and `SRS_SCHEDULER` constraints. |

## Verification (mechanized, run 2026-10-12)

- Ten matches for `^### (1..10)\. ` in the ADR.
- Each of the five rule codes and the nine `plan.*` action names appears in the ADR; `RSCH-009` is cited 17 times.
- `field_pinned`, `keepPinned`, `pinned_by`, `pinned_at`, `pinned`, and `locked` appear only in sentences that reject them (lines in the metadata Supersedes row, the options table, Decision 4, Decision 9, and the Decision 10 table).
- `ADR-0011` appears exactly twice in each of ADR-0003, ADR-0004, ADR-0005 (metadata row and changelog row).
- The human check ("the operator reads Decisions 4 and 6 and accepts") is recorded as the operator's approval of INIT-008 on 2026-10-12. aud-2 and aud-3 carry the planning defaults and are listed in the ADR's open-decisions table.

## Deviations and leftovers

- **Leftover text that still says the migration gate is kept** (the operator waived it; each was stale and left as written when this summary was written; **all of it was edited later on 2026-10-12** to say the gate was waived by the operator, in the run that did SPEC-004 to SPEC-007; the line numbers below are the old ones):
  - `INIT-008-ai-first-plan-builder-initiative.md` §1 (line 54, "That migration stays behind the standing `db_migration` gate … this initiative keeps the gate"), REQ-016 (line 125, "behind the standing `db_migration` gate"), §11 step 3 (line 345, "SPEC-003 behind its `db_migration` gate").
  - `SPEC-001` description item 6 ("`db_migration` stays a standing gate"). The ADR does not repeat that.
  - `SPEC-017` (lines 41 and 60: "the operator's `db_migration` act", "the migration gate is kept"). SPEC-017 will need that wording changed when it runs.
  - `RSCH-009` report, appended section, last paragraph before "Deferred" ("is a standing project gate and is kept"). The report is meant to be left as delivered, so it is not edited.
- No independent reviewer ran. The ADR is a documentation spec at `standard` security depth.

## Files

New: `docs/decisions/ADR-0011-ai-first-plan-builder.md`, this summary.  
Edited: `docs/decisions/ADR-0003-course-and-lesson-plan-language.md`, `ADR-0004-learner-data-media-and-secrets.md`, `ADR-0005-stack-and-architecture.md` (one metadata row and one changelog row each), `SPEC-001-docs-adr-0011-ai-first-plan-builder.md` (status `in_review`, boxes ticked).  
No commit.

## Provenance

`INIT-008/SPEC-001`
