# Initiative: Go server port

**Initiative ID:** INIT-014
**Status:** approved 2026-10-18. SPEC-001 is complete: [ADR-0016](../../../docs/decisions/ADR-0016-go-server-port.md) is Accepted. The build phases Go-1 through Go-17 stay in that ADR. They are not specs yet.
**Slug:** INIT-014-go-server-port
**Discipline Profile:** `software`
**Created:** 2026-10-18
**Owner:** Brian Nelson
**Ticket:** None (`branch_naming.ticket_system: none`)
**Branch:** `cursor/adr-0016-review-notes-3397`. Hebrew PR Reviewer approves and merges. Brian Nelson does not approve pull requests himself.
**Target Completion:** not a promise. The phase table in ADR-0016 §8 starts when a phase is scheduled. This document does not promise a date.
**Source:** Brian Nelson, 2026-10-18, relayed by Hebrew Dev (team lead): move the server off Node to Go. Decisions: [ADR-0016](../../../docs/decisions/ADR-0016-go-server-port.md). Supersedes ADR-0005 (Node runtime, `node:sqlite`, in-process TypeScript MCP, no build step, no second runtime) and ADR-0007 (single-shell SPA, as screens move).

---

## 1. Executive summary

The server moves from Node 22 to Go one route group at a time, on the same ports, with the same SQLite file and schema. Sessions, MCP OAuth tokens, audio cache keys, and recording paths stay byte-compatible. Node stays the rollback image until Go-17.

ADR-0016 is the record. It re-scores ADR-0005's own criteria: options A and B stay 457 and 459, and Go scores 383. The operator chose Go on 2026-10-18 despite that lower score.

## 2. Operator decisions (2026-10-18)

| Id | Decision |
|----|----------|
| D1 | Move the server off Node to Go. Approved by Brian Nelson on 2026-10-18, relayed by Hebrew Dev. |
| D2 | Brian Nelson does not approve pull requests himself. Hebrew PR Reviewer approves and merges. |
| D3 | The decision record is ADR-0016, Accepted the same day, authored by `INIT-014/SPEC-001`. |

## 3. Requirements

| Id | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | ADR-0016 is Accepted and cites the 2026-10-18 approval. | must |
| REQ-002 | ADR-0005 and ADR-0007 name ADR-0016 on their Superseded by lines, and those files are otherwise unchanged by this initiative. | must |
| REQ-003 | Later phases follow ADR-0016. Data stays in place. An existing recording is never overwritten. | must |

## 4. Spec breakdown

| Spec | Domain | What it delivers | Effort | Blocked by |
|------|--------|------------------|--------|------------|
| SPEC-001 | docs | ADR-0016 Accepted, with this id in Authored by, and the Superseded by lines on ADR-0005 and ADR-0007 | 2h | none |

Go-1 through Go-17 in ADR-0016 §8 are the implementation phases. This initiative does not split them into specs.

## 5. Threat model

No new trust boundary in SPEC-001. It is a document. `security_review` on that spec is `light`. Later phases that open `/_hx/` or the database follow the checks ADR-0016 already records.

## 6. Out of scope

- Application code, tests, and config in SPEC-001.
- Editing ADR-0005 or ADR-0007 beyond the Superseded by line.
- A date for Go-17.

## 7. Expertise check

`scripts/dev/expertise_state.py` is not in this repository. No persona was manufactured. SPEC-001 names the docs `primary_prompt` the other initiatives use. Execution is the session agent.

## 8. Success

ADR-0016 is Accepted, `INIT-014/SPEC-001` is its author, and ADR-0005 and ADR-0007 point at it. Hebrew PR Reviewer can merge the phases. Brian Nelson does not have to approve those pull requests.
