# SPEC-009 completion summary

**Spec:** SPEC-009 · API: eight MCP tools for the AI-first builder  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; `security_review: required` is a `complete-initiative` phase and no independent reviewer has run)  
**Provenance:** `INIT-008/SPEC-009`

## What shipped

`registerPlanBuilderTools(server)` in `mcp/tools.js`, called from `registerLessonTools`. Each handler is token check, forward the named arguments, build the `ai` context, call one function, shape the result. No handler edits plan JSON, opens the database, or writes the store (a source scan in the parity test asserts it).

| Tool | Calls | Arguments | Annotations |
|------|-------|-----------|-------------|
| `list_plans` | `create.listPlans` | `limit?` (1 to 200) | `readOnlyHint` |
| `create_plan` | `create.createPlan` | `id`, `title`, `recipe?`, `unitId?`, `lessonId?` | none |
| `list_plan_revisions` | `revisions.list` | `id`, `sinceRevision?`, `beforeRevision?`, `actorKind?` (`ai` `human` `system`), `pathPrefix?`, `limit?` | `readOnlyHint` |
| `get_plan_revision` | `revisions.get` | `id`, `revision` | `readOnlyHint` |
| `diff_plan` | `revisions.diff` | `id`, `from`, `to` (a revision number, `working`, or `published`), `path?` | `readOnlyHint` |
| `restore_plan_revision` | `revisions.restore` | `id`, `fromRevision`, `path?`, `field?`, `expectedRevision` | `destructiveHint: false` |
| `list_plan_issues` | `issues.list` | `id`, `severity?` (`block` `warn` `inform` `all`, or a list), `code?`, `pathPrefix?`, `limit?` | `readOnlyHint` |
| `assign_plan` | `personal.assign` | `id`, `audience` (`shared` or `personal`), `assigneeId?`, `expectedRevision` | none |

- **Descriptions are written for an AI caller.** They say when to use the tool, what comes back, and the notes: what `editState` and `overwrote` mean (advice, never a refusal, with the restore path), the restore workflow (find the revision with `list_plan_revisions` or `diff_plan`, narrowest `path` and `field`, pass `expectedRevision`), that a restore cannot change audience or assignee, that `warn` never stops a publish and `block` does, and that `assign_plan` is the only way to change audience or assignee.
- **Existing tools, description edits only.** `read_part` (the `editState` and `overwrote` text, "read a part before you edit it"), `replace_part` and `set_field` (the `id_mismatch`, `assign_required`, and `overwrote` notes; `set_field` names `list_plan_issues` as the usual source of a fix), `insert_block` (a stale revision writes nothing), and `publish_plan` ("No person has to approve a publish", refused by a block finding or `pending_media`, a lesson is visible only if `everyone` or `selected`). No input schema of any existing tool changed.
- **Refusal bodies** gain two additive keys, `field` and `reason`, when a service names one (which input was refused; `parent_missing` or `absent_in_revision` for a restore). Both are fixed identifiers and are checked against a pattern before they are sent.
- **No tool** marks a form verified, approves a picture, or changes the AI-planning switch. `set_lexicon_verified` and `set_ai_planning` still answer `forbidden`. No tool has an `actor`, `kind`, `by`, `name`, or `client` argument.

## Tests

`test/plan-builder-mcp.test.js`: 12 tests, 12 pass. The tools are called through a real `McpServer` and `Client` joined with `InMemoryTransport`, so the SDK validates each argument as in production. Temporary database through the signed-in server harness; no network, no ElevenLabs, no OpenAI.

- ac-1: eight names, once each, with annotations; the 27 earlier tools' JSON input schemas and annotations equal `test/fixtures/mcp-tools-before-spec-009.json` (captured through a real client before any edit); the only new names are the eight.
- ac-2: no token gives `MCP_AUTH_TOKEN` and no plan text, and writes nothing; a spy on each function sees one call with context `{ kind: 'ai', name: 'mcp', tool: <the tool> }`.
- ac-3: no schema has an actor-like key; forged `actor`, `by`, `kind`, `client` arguments reach `restore_plan_revision`, `create_plan`, `assign_plan`, `set_field` and are not forwarded (every revision row says `ai`/`mcp`, every audit row says `mcp`).
- ac-4: the full flow, including `already_exists`, `invalid_id`, `invalid_recipe`, custom `unitId`/`lessonId`, `assigned` with no `assigneeId` and no username in the text, `assignee_refused` for a disabled and an unknown account with the tables unchanged, paging, `no_snapshot`, and a whole restore that keeps the assignment.
- ac-5: after an admin-route edit, `read_part` shows `editState` with no `by` and no account name; `list_plan_revisions` shows `human`; an AI `set_field` is not refused and lists `overwrote` with the person's value; `restore_plan_revision` brings it back.
- ac-6: a plan with a `pointing-form` warning publishes with no approval argument; an N1 block refuses with `validation` and writes no snapshot; a pending picture on an `everyone` lesson is `pending_media`, then publishes once approved.
- ac-7: stale `expectedRevision` is `revision_conflict` with the stored revision and writes nothing; `field: 'audience'` and `'assigneeId'` are `assign_required`; a missing revision or part is `not_found`.
- ac-8: no key, token, or real username in any plan tool's text; no `pinned`, `locked`, `pin`, or `lock` word.
- Also: the descriptions teach the notes (asserted by phrase), and a learner cannot reach any tool: a learner or admin session cookie on the MCP port is 401, a wrong bearer is 401, and the web server has no `/mcp`.

Full `npm test` with `SRS_SCHEDULER=ladder` after SPEC-009 and SPEC-010 together: **1278 tests, 1278 pass, 0 fail** (the baseline you gave was 1242; the 36 new are SPEC-009's 12 and SPEC-010's 24).

## Deviations

- **`z` is used only to declare `inputSchema`.** Your brief said "zod-free input validation consistent with existing tools". The SDK's `registerTool` takes a zod shape, and every existing plan tool declares one, so the shapes stay. A handler does no parsing of its own: it forwards named keys (`planArgs`, `planInput`) and each service validates its input and returns an error code, as the existing plan tools do.
- **`create_plan` takes `unitId` and `lessonId`** (optional slugs, default `unit-1` and `lesson-1`), which the spec's table does not list. `createPlan` already supports them, and a part id cannot be renamed afterwards (`id_mismatch`), so without them an AI could never name its first lesson.
- **`list_plans` takes `limit`, and `list_plan_revisions` takes `beforeRevision`** (for paging back when `truncated` is true). Both exist in the services.
- **`sinceRevision` is exclusive** (revisions after it), and `pathPrefix` is a text prefix, not an id-path segment match; the descriptions say so.
- **The function behind `diff_plan` is `revisions.diff`**, not `diff.between` as the initiative table's proposal named it.
- **`publish_plan`'s description is longer** than "unchanged"; the spec asks it to name the two refusals.

## Not closed

1. **Independent security review** (`security_review: required`) has not run. The token gate, actor derivation, and assign and restore authorization have tests, not a reviewer.
2. **`client` label** is still never recorded (SPEC-002's finding stands: the SDK does not give a handler the client info, and the server is stateless).
3. **The `read_part` description grew long.** It is the one tool every AI session reads first, so it carries the edit-state explanation; it can be shortened if it costs too much context.

## Files

New: `test/plan-builder-mcp.test.js`, `test/fixtures/mcp-tools-before-spec-009.json`, this summary.  
Edited: `mcp/tools.js`, the SPEC-009 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `test/plan-mcp.test.js`.

## Provenance

`INIT-008/SPEC-009`
