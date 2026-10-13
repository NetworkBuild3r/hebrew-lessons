# SPEC-009 · API: MCP tools for the AI-first builder

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-008
title: "Eight MCP tools: create, list, history, get, diff, restore, issues, assign; edit state in read_part"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-006, SPEC-008]
blocks: [SPEC-010, SPEC-016]
```

---

## Description

**Context:** the MCP is the primary writer (D4). Plan tools are registered in `mcp/tools.js` with `register(server, name, meta, handler)` and call `callPlanTool(fn)`, which requires `MCP_AUTH_TOKEN` and maps the result (`mcp/tools.js:180`–`:207`). `planInput(args, keys)` forwards only named keys, so an argument cannot smuggle a field into an operation. Seven plan tools exist (`read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, `publish_plan`) at `mcp/tools.js:1035`–`:1133`; no tool creates, lists, shows history, diffs, restores, or lists issues, and `personal.assign` has no tool. SPEC-002 gave every plan write a context; SPEC-004 to SPEC-008 wrote the functions. This spec is the adapter layer for the MCP. ADR-0011 Decision 7 is the contract.

**Scope (in):** register eight tools in a new `registerPlanBuilderTools(server)` block called from `registerLessonTools`, each a thin adapter: token check, parse (`z` schemas with length limits on every string, as the existing tools), build an `mcp` context (kind `ai`, name `mcp`, the tool's own name), call one function, shape the result. Names are namespaced, short, and match what the AI is doing (`RSCH-009` [51]).

| Tool | Calls | Annotations | Notes for the AI (in the description) |
|------|-------|-------------|---------------------------------------|
| `list_plans` | `listPlans` | `readOnlyHint` | Start here to find a plan id. No assignee name is returned, only `assigned`. |
| `create_plan` | `createPlan` | not read-only | New draft at revision 1 from `id`, `title`, optional `recipe`. An existing id returns `already_exists`. Invisible to learners until published. |
| `list_plan_revisions` | `revisions.list` | `readOnlyHint` | Newest first. Filters `sinceRevision`, `actorKind`, `pathPrefix`, `limit`. Use it to see what a person changed since the revision you last read. |
| `get_plan_revision` | `revisions.get` | `readOnlyHint` | The whole plan as it was at one revision. |
| `diff_plan` | `revisions.diff` | `readOnlyHint` | Between two revisions, or `working` and `published`. Entries have id paths you can pass to `read_part`. |
| `restore_plan_revision` | `revisions.restore` | not read-only (`destructiveHint: false`; it makes a new revision) | Put back a whole revision, one part (`path`), or one field (`path` and `field`) in one call. Pass `expectedRevision`. History is kept. Cannot change who a personal plan is assigned to. |
| `list_plan_issues` | `issues.list` | `readOnlyHint` | Every finding with a stable code, an id path, a field, and a fix where known. Warnings never block publish. Fix with `set_field` or `replace_part`. |
| `assign_plan` | `personal.assign` | not read-only | `audience` `shared` or `personal`, with `assigneeId` an active account name, and `expectedRevision`. The only way to change audience or assignee. `assignee_refused` writes nothing. |

- `read_part`'s description gains: "The result includes `editState`. It lists fields a person last changed under this part, with the value they replaced. It is advice: you may edit anyway, and nothing is refused because a person edited a field. If you overwrite one, your result lists it in `overwrote`, and `restore_plan_revision` puts the person's value back." The seven existing tools keep their names and input schemas; their results gain `editState` (read) and `overwrote` (writes) as additive keys, and each passes its own tool name in the context.
- `publish_plan` is unchanged. Its description does **not** say it needs a person; the AI may publish (D1). The hard validation and `pending_media` refusals still apply and are named in the description.
- **Not added, by design:** any tool that marks a lexicon form `verified`, approves a picture, or turns the AI-planning switch on. `set_lexicon_verified` and `set_ai_planning` keep returning `forbidden`. No tool accepts an `actor` argument.
- Every new and existing plan tool refuses without `MCP_AUTH_TOKEN` through `callPlanTool`.

**Scope (out):** HTTP routes and the parity test (SPEC-010); the skill text (SPEC-017); resources and prompts (not in this initiative).

---

## Understood (agent's read of the intent)

- [x] und-1: Each handler is a few lines. The logic is in the functions SPEC-002 to SPEC-008 wrote.
- [x] und-2: The actor is never an input. The context is built by the adapter, as in SPEC-002.
- [x] und-3: Error results use the existing shape (`isError`, a JSON body with `error`), so an AI that already handles `revision_conflict` handles the new names. _Two additive keys on a refusal body, `field` and `reason`, when a service names one (which input was refused; why a restore could not place a part)._

## Acceptance Criteria

- [x] ac-1 (REQ-007, REQ-009): the in-process MCP server lists the eight new tool names exactly once each, with the annotations above; the seven existing plan tools, the media tools, and the learner tools are still registered with unchanged names and input schemas (a snapshot comparison of names and schemas taken before this change). _`test/fixtures/mcp-tools-before-spec-009.json` holds the 27 earlier tools' JSON input schemas and annotations, taken through a real MCP client before any edit; the test compares each and checks the only new names are the eight._
- [x] ac-2 (REQ-009): every new tool refuses without `MCP_AUTH_TOKEN`, with no plan content in the refusal; a spy on each function confirms each tool calls the one function in the table above, with a context whose actor is `{ kind: 'ai', name: 'mcp' }` and the tool's name.
- [x] ac-3 (REQ-003): no tool's input schema has a key that can set an actor, a kind, or a name; an argument `actor: { kind: 'human' }` passed to `set_field` is not forwarded (the written revision row says `ai`). _Also asserted for `restore_plan_revision`, `create_plan`, and `assign_plan`, and for every audit row._
- [x] ac-4 (REQ-007): `create_plan` then `read_part` with an empty path, `list_plans`, `assign_plan`, `list_plan_issues`, `list_plan_revisions`, `diff_plan`, and `restore_plan_revision` each succeed on a temporary database with the shapes SPEC-004 to SPEC-008 define; `assign_plan` with a disabled account returns `assignee_refused` and writes nothing.
- [x] ac-5 (REQ-006): `read_part` returns `editState` after a human edit; an AI `set_field` of that field returns its result with `overwrote`; the call is not refused.
- [x] ac-6 (REQ-019): `publish_plan` publishes a plan whose only findings are warnings, with no approval argument; a plan with a block finding or a pending picture on an `everyone` lesson is still refused.
- [x] ac-7 (REQ-005): `restore_plan_revision` with a stale `expectedRevision` returns `revision_conflict` and the stored revision, and writes nothing; with `field: 'audience'` returns `assign_required`.
- [x] ac-8 (REQ-017): tool descriptions contain no key, no real username, and the words `pinned` and `locked` appear in none of the plan tools' names or descriptions.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Tool names | `list_plans`, `create_plan`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `list_plan_issues`, `assign_plan`. The new ones carry a `plan` word so the AI can tell them from the worksheet tools. | resolved | yes — planning choice; ADR-0011 records the names |
| aud-2 | MCP resources and prompts | Not registered. The edit state rides in the tool result. | resolved | yes — `RSCH-009` §1.5 (resources are host-chosen) |
| aud-3 | Existing tool results | Additive keys only (`editState`, `overwrote`). | resolved | yes — REQ-017 |

> **Readiness** = 100.

---

## Deliverables

- [x] Changes to `mcp/tools.js`: `registerPlanBuilderTools`, context building for the seven existing plan tools, description edits _(the context building for the seven tools was SPEC-002's; this spec edited their descriptions and added `planArgs` and the `field` and `reason` keys on a refusal)_
- [x] Tests: `test/plan-builder-mcp.test.js`; updated `test/plan-mcp.test.js` only where a deep-equal on a result needs the new key _(no change to `test/plan-mcp.test.js` was needed)_

---

## Technical Requirements (domain-specific)

- [x] Input schemas with `z`, a length limit on every string field, integers for revisions. `planInput`-style forwarding of named keys only. _`z` is used only to declare `inputSchema`, as every existing tool does; a handler forwards named keys (`planArgs`, `planInput`) and each service validates its own input, so there is no second parser in a handler._
- [x] No plan or revision content in a log line. Errors carry no echo of the input.
- [x] No new dependency. Temporary database and in-process server in tests; no network; no call to ElevenLabs or OpenAI.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-builder-mcp.test.js test/plan-mcp.test.js` and `npm test`.
- **Rollback plan:** remove `registerPlanBuilderTools`; the existing tools keep working.

---

## Security

- **Sensitivity:** required. Triggers matched: MCP writes that create plans, restore content, and change assignment; token-gated reads of plan history and issues.
- **Security acceptance criteria:**
  - [x] Token required on every plan tool; no tool reaches verification, picture approval, or the AI-planning switch.
  - [x] The actor is never taken from an argument.
  - [x] `assign_plan` keeps the active-account check; `restore_plan_revision` cannot change audience or assignee.
- **Review depth:** thorough · **focus areas:** token gate, actor derivation, assign and restore authorization, that no human username reaches the MCP.

---

## Verification Strategy

- **Claim:** the AI can create, list, inspect, diff, restore, and assign plans and read their issues through the MCP alone, with the existing gates intact and no way to name itself as a person.
- **Check + executor:** `mechanized` — `node --test test/plan-builder-mcp.test.js test/plan-mcp.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-8 each asserted; the before/after tool snapshot in ac-1 shows no change to an existing name or schema.
- **Test levels:** integration (in-process MCP server and a temporary database, as `test/plan-mcp.test.js`).

---

## Integration Points

- SPEC-010 adds the HTTP twins and the parity test that iterates initiative §12. SPEC-016 drives these tools in the end-to-end run. SPEC-017 documents the names exactly as registered.
- Produces **Change:** eight MCP tools; edit state in `read_part`.
- **Provenance tag:** `INIT-008/SPEC-009`
