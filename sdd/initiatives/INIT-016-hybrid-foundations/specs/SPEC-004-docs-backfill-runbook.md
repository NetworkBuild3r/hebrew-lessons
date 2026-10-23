# SPEC-004 · docs: Class backfill runbook through MCP

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-016
title: "Class backfill runbook through MCP"
domain: docs
status: planned
primary_prompt: .claude/agents/implementation-planner/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 2 hours
blocked_by: [SPEC-001]
blocks: []
```

## Description

**Context:** The class spreadsheet has ten tables (four classes so far) on the operator's machine. Data comes in through MCP, not as repo files (D3), and only once the backfill is under way. Without a written procedure, loads will be ad hoc and names or transcripts can leak into shared content.

**Scope (in):** `docs/operations/class-data-through-mcp.md`:

- Each class table → the MCP tool that loads it, and which spec adds that tool (words → `upsert_lexemes` now; patterns, explainers → INIT-017/SPEC-002; mistakes, verb forms → INIT-019/SPEC-002; phrases with prompts → `put_unit` with INIT-020/SPEC-002's `say` step; class order → `put_unit` positions).
- Order of loading (lexicon first, then patterns, then units).
- Shared versus personal: what goes to the shared course and what goes only to the learner's own scope.
- Privacy rules: no tutor or learner names, no transcripts, no verbatim class answers in shared tables; personal mistakes are the learner's only.
- A dry run with `validate_unit` before any `put_unit`, and a check after each load (`lookup`, `get_learner_model`).
- What "under way" means before loading: the operator's call, recorded here.

**Scope (out):** Running the loads. Building any tool.

## Acceptance Criteria

- [ ] ac-1: Every one of the ten tables is mapped to a tool or marked "not loaded yet" with the spec that unblocks it. (REQ-016)
- [ ] ac-2: The privacy rules are stated as checks a loader can run, not advice. (REQ-016)
- [ ] ac-3: The document contains no names of the tutor or the learner.

## Deliverables

- [ ] `docs/operations/class-data-through-mcp.md`

## Gates & Controls

- **Autonomy level:** plan_only — the operator runs the loads. **Rollback:** revert the doc.

## Security

- **Sensitivity:** standard — governs personal data handling. Review the privacy section.

## Verification Strategy

- **Claim:** a loader agent can follow the doc without asking which tool loads what.
- **Check + executor:** human — the operator walks one sample class through the doc with the class-data agent.
- **Pass condition:** every table in the sample maps to a step in the doc, and the loaded rows contain no names.

## Integration Points

- Updated as INIT-017/SPEC-002, 014, 017, 023 land.
- **Provenance tag:** `INIT-016/SPEC-004`
