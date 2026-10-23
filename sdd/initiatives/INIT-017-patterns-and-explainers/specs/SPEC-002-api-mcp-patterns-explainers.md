# SPEC-002 · api: MCP patterns, explainers, the unit's pattern, and the path-order check

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-017
title: "MCP: patterns, explainers, unit pattern, path-order check"
domain: api
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: [.claude/agents/security-specialist/AGENT.md]
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, INIT-020/SPEC-002, SPEC-004]
```

## Description

**Context:** Patterns and explainers come from the class data and from authoring agents through MCP (D3, D6). The default path must not teach a pattern before its prerequisite (D4).

**Scope (in):**

- `upsert_patterns` (admin): slug, title, summary, example, prerequisites. One transaction, at most 100.
- `upsert_explainer` (admin): pattern slug, title, body, status. The body is checked against the restricted markdown grammar: headings level 2–3, paragraphs, bold and italic, lists, Hebrew inline marked with `{he:...}` and an optional reading `{he:...|tr}`, links only to `/learn/...`, `/roots/...`, `/lesson...`. Raw HTML, images and other links are refused with a JSON pointer.
- `list_patterns` (any token): slugs, titles, prerequisites, whether an explainer is published.
- `put_unit` and `validate_unit` take an optional `pattern` (a known slug).
- New validator rule `unit.pattern_order` for shared units: a unit whose pattern requires a pattern first taught at a later position fails, with the conflicting unit named. For a learner unit, a pattern whose prerequisites the learner has not met is a warning.
- `check_path_order` (admin): every violation in the shared path and a proposed order (a stable topological sort that keeps class order wherever it is valid).
- Authoring resource and the lesson-author skill updated.

**Scope (out):** Pages (SPEC-003).

## Acceptance Criteria

- [ ] ac-1: An admin can create patterns with prerequisites and an explainer; a learner token is refused on both writes. (REQ-003, REQ-005)
- [ ] ac-2: An explainer body with `<script>`, any raw HTML tag, an image, or an external link is refused and writes nothing. (REQ-005)
- [ ] ac-3: `validate_unit` fails a shared unit placed before the unit that teaches its prerequisite and names that unit. (REQ-004)
- [ ] ac-4: `check_path_order` on a fixture path with two violations lists both and proposes an order that fixes them while keeping every other unit's relative order. (REQ-004)

## Deliverables

- [ ] `internal/mcp/tool_patterns.go`, `tool_explainer.go`, `tool_path_order.go`; `put_unit`/`validate_unit` changes
- [ ] `internal/units` rule and its id in `rules.go`
- [ ] A restricted-markdown parser package (`internal/richtext`) shared with SPEC-003, producing an allow-listed node tree
- [ ] Tests for each tool, the parser (table of refused inputs) and the order rule

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** required — content written over MCP is shown to every learner.
- **Security acceptance criteria:**
  - [ ] Bodies are parsed to an allow-listed node tree at write time; anything outside it is refused, not stripped.
  - [ ] Writes are admin-only, including the static token rule `upsert_lexemes` uses.
  - [ ] Each write is logged in the publish audit with the token's identity.
- **Review depth:** standard · **focus areas:** stored injection, authorization, size limits.

## Verification Strategy

- **Claim:** patterns and explainers can be maintained over MCP safely, and the path order is checkable.
- **Check + executor:** mechanized — `go test ./internal/mcp/... ./internal/units/... ./internal/richtext/...`.
- **Pass condition:** ac-1 to ac-4 tests pass, including the refused-input table.

## Integration Points

- INIT-016/SPEC-004 runbook adds the load steps. SPEC-003 renders the node tree. INIT-020/SPEC-002 and SPEC-004 extend the unit schema next to `pattern`.
- **Provenance tag:** `INIT-017/SPEC-002`
