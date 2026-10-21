# SPEC-001 · docs: ADR-0016 Go server port

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-014
title: "ADR-0016: port the server from Node to Go"
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: None
blocks: []
```

## Description

**Context:** Brian Nelson approved moving the server off Node to Go on 2026-10-18, relayed by Hebrew Dev. He does not approve pull requests himself. Hebrew PR Reviewer approves and merges. The next free initiative number on that date is INIT-014 (`sdd/initiatives/` held INIT-001 through INIT-013). Docs ADRs in this repo are SPEC-001 of their initiative (`INIT-013/SPEC-001` authored ADR-0015).

**Scope (in):** `docs/decisions/ADR-0016-go-server-port.md` set to Accepted, authored by `INIT-014/SPEC-001`. The Superseded by line on ADR-0005 and on ADR-0007.

**Scope (out):** Any other line in ADR-0005 or ADR-0007. Application code.

## Understood

- [x] und-1: ADR-0016 supersedes ADR-0005 (Node runtime, `node:sqlite`, in-process TypeScript MCP, no build step, no second runtime) and ADR-0007 (single-shell SPA, as screens move).
- [x] und-2: The ADR-0005 re-score stays as written. Go scores 383, behind A at 457. The operator chose Go despite that score.
- [x] und-3: Hebrew PR Reviewer approves and merges. The operator does not approve each pull request.

## Acceptance Criteria

- [x] ac-1: `docs/decisions/ADR-0016-go-server-port.md` has Status Accepted, Authored by `INIT-014/SPEC-001`, and Initiative INIT-014 · go-server-port (`REQ-001`).
- [x] ac-2: The ADR cites Brian Nelson's approval on 2026-10-18, relayed by Hebrew Dev, and states that Hebrew PR Reviewer approves and merges (`REQ-001`).
- [x] ac-3: ADR-0005 and ADR-0007 name ADR-0016 on their Superseded by lines, and this spec changes nothing else in those files (`REQ-002`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Initiative number | `INIT-014`. Directories INIT-001 through INIT-013 already exist. | resolved | yes |
| aud-2 | Who merges | Hebrew PR Reviewer. The operator's 2026-10-18 approval is of the move to Go, not of each later pull request. | resolved | yes |

## Deliverables

- [x] `docs/decisions/ADR-0016-go-server-port.md`
- [x] Superseded by line on `docs/decisions/ADR-0005-stack-and-architecture.md`
- [x] Superseded by line on `docs/decisions/ADR-0007-single-page-app-structure.md`

## Verification Strategy

- **Claim:** ADR-0016 is Accepted under this id, and the two earlier records point at it.
- **Check + executor:** `human` — read the three metadata tables.
- **Pass condition:** ac-1 through ac-3 hold, and a diff of ADR-0005 and ADR-0007 shows only the Superseded by line.

## Gates

No production code, no dependencies, no secrets.

## Integration Points

- **Provenance tag:** `INIT-014/SPEC-001`.
