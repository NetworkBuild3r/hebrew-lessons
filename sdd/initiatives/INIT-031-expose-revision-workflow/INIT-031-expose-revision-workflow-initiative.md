# INIT-031: The connection feeds the learner, and the agent owns the lessons it built

**Initiative ID:** INIT-031 · **Status:** planned · **Created:** 2026-11-05 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian, 2026-11-05. First: the connection has to expose the revision workflow before improvements can be published safely. Then: "Agent can delete, remove, rebuild and mix match etc anything lessons it built. The MCP is to feed it user data, so the AI makes informed and correct decisions."

This plan is written in the current checkout. It does not start the work, and it does not rewrite any lesson.

## What we found

The MCP connection this session is using is the older server. `get_lesson_brief` accepts a learner and a theme. It does not accept `unitId`. An agent that follows that tool list cannot see the revision plan and cannot aim a publish at a lesson it already built.

The code on `main` (`937805a`) is also the wrong shape for this request:

- `lessonBrief` in `internal/mcpstore/learner_model.go` drops `nextPattern`, `reusePatterns`, `previousUnits`, `pace`, and `minStoryTokens` before the reply. The agent never sees the plan the server computed.
- `get_context` does not describe `progress` (`untouched`, `started`, `finished`). `put_unit` does not say the agent may replace a lesson it already published.
- A started or finished unit is refused (`brief.revise_started`, "That unit is started and cannot be revised"). Three untouched units refuse a new brief. Those locks decide the lesson for the agent. The agent is the one who should decide, after it has read the learner.

`unpublish_unit` can take a learner unit off Today. `delete_unit` is named in `internal/mcp/doc.go` and is not registered. `get_unit` returns one published document and its revision. There is no tool that hands the agent several of its own lessons at once, so mixing them means a series of `get_unit` calls with no list of what is inside each one.

Shared units, and another learner's units, are not lessons this agent built.

## What you get

- **The agent owns the lessons it published for this learner.** It can replace one (`put_unit` with the same `unitId`), take it off Today (`unpublish_unit`), delete it (`delete_unit`), or write a new unit that combines lines from several of them. Started and finished are facts, not locks.
- **The connection feeds the learner.** `get_context` shows every unit with its progress, score, and a short content summary (pattern, new words, story length). `get_lesson_brief` returns the next pattern, the pace, the previous units' text, struggles, and accuracy. `get_unit` still returns the full document and the revision. The agent reads those and then chooses.
- **The tool list says this.** A test fails if the registered schema hides the fields or still says a started unit cannot be revised.
- **The live connection matches** after `/version` shows the commit. The check only reads. It does not publish a lesson.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-connection-is-the-contract.md) | docs | ADR: the agent owns its lessons, the connection feeds the learner | — | 1h | planned |
| [SPEC-002](specs/SPEC-002-mcp-return-the-revision-plan.md) | mcp | Return the learner data and let the agent rewrite its units | SPEC-001 | 5h | planned |
| [SPEC-003](specs/SPEC-003-mcp-tool-list-locks-the-names.md) | mcp | The registered tool list must name the data and the actions | SPEC-002 | 2h | planned |
| [SPEC-004](specs/SPEC-004-ops-live-tool-list.md) | ops | The live connection shows the same names | SPEC-003 | 2h | planned |

**Total:** 10h. **Critical path:** SPEC-001 → SPEC-002 → SPEC-003 → SPEC-004 = 10h.

**Order.** SPEC-004 only reads the running server. Rewriting learner 1's lessons stays a later agent's job, after this connection check passes.

## Decisions

- **The connection feeds. The agent decides.** Progress, accuracy, struggles, and earlier lessons are data. The server does not refuse a new brief because untouched units are waiting, and it does not refuse a publish because the learner has started or finished the unit.
- **The agent may change any learner unit it published.** Replace the document, unpublish it, delete it, or publish a new unit that reuses lines from several of them. A replacement keeps the same `unitId` and position unless the agent sends a new position. The previous revision stays in `unit_revisions`.
- **Shared units and other learners stay put.** `put_unit`, `unpublish_unit`, and `delete_unit` refuse `shared` and another learner's id. That refusal is a scope check, not a judgment about the lesson.
- **Add `delete_unit`. Do not add a mix tool.** Mixing is the agent reading units and writing one document. `delete_unit` removes a learner unit and its published pointer. Runs and the audit row stay, so the history of what the learner did is not erased.
- **The connection is the tool list.** A field that exists only in Go, or only in a reply the schema does not name, is not exposed. This supersedes the "only untouched units are revised" publish lock in ADR-0026. The data that lock was protecting is still returned.
- **No lesson writes here.** This initiative does not publish, unpublish, or delete a real unit. That waits until the live tool list matches.

## Threat model

- **No new trust boundary.** Admin-only tools, same as `put_unit`. A learner token cannot call them. A learner may still `unpublish_unit` on their own unit, as today.
- **Scope.** `delete_unit` and a replacing `put_unit` check `learner:<id>` in the query. A shared unit and another learner's unit are the same not-found answer, so one id cannot be probed.
- **History.** Delete drops the published pointer. It does not drop `lesson_runs` or `publish_audit`. A replace writes a new revision and an audit row.
- **Output.** Tool text names fields and rule ids. It does not include tokens or voice ids. Live lesson text is returned only by `get_unit` and the brief's previous-unit texts, to the admin caller who already may read that learner.
- **The live check is read-only.** SPEC-004 lists tools and reads `/version`. It does not call `put_unit`, `unpublish_unit`, or `delete_unit`.

## Depends on / unblocks

- Depends on INIT-030 specs 1 to 6, on `main` as `937805a`. Supersedes that initiative's rule that a started unit cannot be revised.
- Unblocks a later rewrite of learner 1's lessons. That rewrite is not a spec here.

## Done when

- The registered tool list shows `progress` on `get_context`, the revision plan on `get_lesson_brief`, and `put_unit`, `unpublish_unit`, and `delete_unit` as actions on a learner unit the agent built, including one the learner has started.
- A started unit can be replaced. The call is not `brief.revise_started`.
- The live connection shows the same names after `/version` matches.
- No real learner unit was published, unpublished, or deleted by this initiative.
