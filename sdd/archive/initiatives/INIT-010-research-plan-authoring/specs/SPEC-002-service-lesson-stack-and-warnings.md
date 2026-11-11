# SPEC-002 · service: lesson stack, one activity map, research warnings

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-010
title: Add a lesson to a plan, and warn when the research is skipped
domain: service
status: complete
primary_prompt: .claude/agents/service-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-004, SPEC-005]
```

## Description

**Context:** `insert_block` can fill a lesson. It cannot add Family 2. The research warnings `later-context` and `N12` already exist and must stay warnings. Two checks do not exist yet: a picture used as the cue on Type it, and a new-word list that is a single closed set.

**Scope (in):** `insert_lesson`, `move_lesson`, and `remove_lesson` on the same plan `apply()` the other operations use, with the same revision check, audit row, and admin routes. Remove the schema cap of 12 lessons on a unit (`lessons.maxItems`) and the cap of 12 on `buildsOn`. A plan may hold one lesson or 100. `N10` must not refuse a plan for lesson count. The plan byte cap (`N9`, 512KB) still applies to content size. A fixture of 100 small lessons must pass. If that fixture exceeds 512KB, raise the plan byte cap until it passes. An activity-name map, one module, from the initiative section 4 names to stored block types. MCP tools for the three lesson operations. A `picture-on-recall` warning when a `type-it` block's cue is a picture. A `closed-set` warning when every new word in a lesson is in one kinship list shipped with the check (mother, father, sister, brother, son, daughter, and the Hebrew forms the lesson uses for those). No other classifier. `list_plan_issues` returns the new warnings with the old ones. Publish still allows warnings.

**Scope (out):** The admin screens (SPEC-003). Match and Type it playback (SPEC-004). The skill text (SPEC-005). The review scheduler.

## Acceptance criteria

- [ ] ac-1: After `create_plan`, two `insert_lesson` calls yield three lessons in order, and `move_lesson` swaps the last two. A plan of 100 lessons validates, and lesson 13 is not a schema error — covered: no
- [ ] ac-2: The activity map's Words, Story, Match, and Type it rows are the block types in initiative section 4 — covered: no
- [ ] ac-3: A plan whose Family 1 word never appears in Family 2 reports `later-context` and still publishes — covered: no
- [ ] ac-4: A Type it block that shows a picture reports `picture-on-recall` at severity warn. A kinship-only word list reports `closed-set` at severity warn. Neither is in the publish block list — covered: no
- [ ] ac-5: A non-admin session receives 403 on the new routes. The MCP tools require the existing bearer token — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | removing the last lesson | Refused. A plan keeps at least one lesson. There is no maximum. Removing the last block of a lesson stays the existing rule | resolved | yes |

## Deliverables

- [ ] `server/plans/ops.js` and the plan routes in `server/api/plan-routes.js`
- [ ] `mcp/tools.js` tools `insert_lesson`, `move_lesson`, `remove_lesson`
- [ ] One activity-map module imported by the validator and, later, the admin
- [ ] `server/plans/validate.js` warnings `picture-on-recall` and `closed-set`
- [ ] Node tests for ac-1 through ac-5

## Verification

- **Claim:** lesson order round-trips, and the two new findings are warnings that do not stop publish.
- **Check + executor:** `mechanized` — `node --test` on the new plan-ops and validate tests.
- **Pass condition:** the tests assert the stored lesson order, the warning codes, a successful publish when only those warnings fire, and 403 for a non-admin. This repo's bar is that test run, not a coverage percentage.

## Gates

No migration. No new dependency. Admin checks stay on the server.
