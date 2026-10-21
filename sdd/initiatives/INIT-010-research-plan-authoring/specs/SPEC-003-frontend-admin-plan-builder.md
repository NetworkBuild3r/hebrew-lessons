# SPEC-003 · frontend: admin builder for a stacked plan

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-010
title: Admin plan builder uses the same lesson stack and activity names
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: None
```

## Description

**Context:** The plan builder currently lives inside `/admin/` as tabs next to users and lessons. The operator wants two pages. `/editor/` is where a plan is built. `/admin/` stays users, lesson visibility, and the three worksheets.

**Scope (in):** Add an admin-only route `/editor/` in `shared/app-routes.js`, with a nav link that only admins see, following the same server gate as `/admin/`. Move the plan list and plan builder onto that page. A plan shows lessons as one ordered stack with no maximum. Add, reorder, and remove call the SPEC-002 lesson operations. Adding an activity shows Words, Story, Listening, Dialogue, Match, Type it, Fluency, Review, and Pattern, and inserts the mapped block. Nothing is pre-inserted. A picture on a Words entry is optional. `buildsOn` lists earlier lessons. While a later lesson is open, the page shows the words earlier lessons added, in order. `/admin/` no longer shows Plans. It still opens a worksheet in the lesson editor, including `#lesson/my-morning`. The server refuses `/editor/` for a non-admin the same way it refuses `/admin/`.

**Scope (out):** Match and Type it playback (SPEC-004). Practice scheduling (SPEC-006). MCP skill text (SPEC-005). The three worksheet files.

## Acceptance criteria

- [ ] ac-1: `/editor/` lists plans and opens one. Adding two lessons shows three rows, and moving one changes the saved order — covered: no
- [ ] ac-2: The add-activity control lists Words, Story, Match, and Type it, and does not pre-insert any of them. With Family 3 open, the word list includes Family 1's words and marks the lesson that added each one — covered: no
- [ ] ac-3: `buildsOn` offers only earlier lessons and saves the chosen id — covered: no
- [ ] ac-4: `/admin/` has no plan builder. Opening `#lesson/my-morning` on `/admin/` mounts the lesson editor. A non-admin request to `/editor/` is refused — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | where a new course starts | `/editor/`. `/admin/` keeps users and the worksheet lessons | resolved | yes |

## Deliverables

- [ ] `shared/admin/plan-builder.js` and the widgets it uses for the stack and the activity menu
- [ ] `shared/admin/admin-shell.js` and `shared/admin/lessons.js` restore the worksheet editor route
- [ ] A node test for ac-1 through ac-4
- [ ] A browser pass: create the three Family lessons, add Words only on the first, set builds-on, confirm the warning text is a warning

## Verification

- **Claim:** the admin builder writes the same lesson order and block types SPEC-002 stores, and the worksheet editor is back.
- **Check + executor:** `mechanized` — `node --test` on the plan-builder tests. `human` — the operator, in the browser, for the Family walkthrough.
- **Pass condition:** the node tests fail if the activity menu pre-inserts Words or if `#lesson/my-morning` renders `data-lesson-no-plan`. The browser pass shows three lessons and a warning that does not disable Publish.

## Gates

No dependency. Interface language stays one language per option, using `data-ui-text-en` and `data-ui-text-he`.
