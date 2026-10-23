# SPEC-003 · frontend: Learn pages and the lesson link

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-017
title: "Learn pages and the lesson link"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: [.claude/agents/security-specialist/AGENT.md]
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: [INIT-019/SPEC-004]
```

## Description

**Context:** "Explain at the point of need": when a pattern first appears, a short explainer is one tap away, and the explaining stays out of the drills.

**Scope (in):**

- `GET /learn`: published explainers grouped by pattern, in path order.
- `GET /learn/{slug}`: the explainer rendered from SPEC-002's node tree (no HTML passthrough); Hebrew runs in `bdi` with niqqud-safe line height and a reading under it when given; "Used in" links to the lessons with this pattern.
- In the lesson player: on the first step of a unit whose pattern has a published explainer, a quiet "How this works" link under the prompt, opening the page in a sheet (htmx) and as a plain link without script.
- Five starter explainers written as our own content and loaded through `upsert_explainer`: present-tense agreement; past tense with ־תִּי; "going to" plus a verb; אֶפְשָׁר plus a verb; prepositions with endings (לִי, לְךָ).

**Scope (out):** Common mistakes on the page (INIT-019/SPEC-003 adds that block).

## Acceptance Criteria

- [ ] ac-1: `/learn/{slug}` shows a published explainer and 404s a draft or unknown slug. (REQ-005)
- [ ] ac-2: The lesson link appears on the first step of a unit with that pattern and nowhere else in the unit. (REQ-005)
- [ ] ac-3: A body containing every allowed node renders with no raw HTML from the body in the output, and Hebrew runs carry `lang="he" dir="rtl"`. (REQ-005)
- [ ] ac-4: Pages pass at 390 and 1440, light and dark; the sheet opens and closes by keyboard.

## Deliverables

- [ ] `internal/web/learn.go`, `views/learn.templ`, player link, CSS
- [ ] Explainer content (five) as a seed through MCP, recorded in `docs/operations/class-data-through-mcp.md`
- [ ] Go render tests; browser test with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** required — renders authored content to every learner.
- **Security acceptance criteria:**
  - [ ] The renderer accepts only SPEC-002's node types; all text is escaped by templ.
  - [ ] Link targets are re-checked against the allow-list at render time.
- **Review depth:** standard · **focus areas:** output encoding, link targets.

## Verification Strategy

- **Claim:** a learner meets the explainer at the step where the pattern first appears, and the page is safe.
- **Check + executor:** mechanized — `go test ./internal/web/...`; browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-4 tests and checks pass.

## Integration Points

- INIT-019/SPEC-004 adds Learn to navigation. INIT-019/SPEC-003 adds common mistakes to the page.
- **Provenance tag:** `INIT-017/SPEC-003`
