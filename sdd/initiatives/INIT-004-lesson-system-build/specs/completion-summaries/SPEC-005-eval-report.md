# SPEC-005 AC Verification Eval Report — Plan MCP tools

**Traceability:** INIT-004/SPEC-005 \
**Verifier:** independent readonly generalPurpose Task · model `composer-2.5-fast` · agent `f0f60dda-8451-4733-adbd-55b9093f814d` · temperature 0 \
**Evaluated at:** 2026-10-09T17:55:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Each new tool calls the matching function. A spy compares stored bytes with a direct call. The bytes match. | PASS | Test `ac-1: each plan tool calls its function once and stores the same bytes` passed (`test/plan-mcp.test.js:232-329`). Spy asserts `spyLog === [fn]` and `viaTool.document === viaFn.document`. Handlers: `read_part`→`readPart` (`mcp/tools.js:439`), `replace_part`→`replacePart` (450-454), `insert_block`→`insertBlock` (466-470), `move_block`→`moveBlock` (481-485), `remove_block`→`removeBlock` (495-499), `set_field`→`setField` (511-515), `publish_plan`→`publishPlan` (524-527). |
| ac-2 | A stale `expectedRevision` returns `revision_conflict` and does not change the stored plan. | PASS | Test `ac-2: a stale expectedRevision returns revision_conflict and does not write` passed (`test/plan-mcp.test.js:332-370`). `body.error === 'revision_conflict'`, `body.revision === seeded.revision`, stored row unchanged. Publish stale path included. Error shape from `planToolResult` (`mcp/tools.js:153-161`). |
| ac-3 | With the token unset, `read_part` returns no plan JSON. Legacy tools still require the token, as `test/mcp-auth.test.js` checks. | PASS | Test `ac-3: with the token unset, read_part returns no plan JSON` passed (`test/plan-mcp.test.js:372-400`). All seven plan tools refuse with `/MCP_AUTH_TOKEN/` and no `schemaVersion` or sample Hebrew. `MCP requires the bearer token` passed (`test/mcp-auth.test.js:27-46`). `requireMcpAuthToken` (`mcp/tools.js:42-48`) runs before the plan call (`171-174`). |
| ac-4 | `list_lessons`, `get_lesson`, `upsert_lesson`, `set_lesson_visibility`, and `generate_lesson_audio` still register. A test lists tool names. | PASS | Test `ac-4: legacy tools and the seven plan tools are registered` passed (`test/plan-mcp.test.js:208-230`). Eight legacy names and seven plan names are in `toolHandlers`. `upsert_lesson` schema has `worksheet` and no `plan` or `document`. |

Security acceptance criteria, same verifier pass:

| AC | Criterion | Verdict | Evidence |
| ---- | ----------- | ------- | -------- |
| sec-1 | No tool returns learner progress fields. | PASS | Test `a plan tool reply does not include learner progress fields` passed (`test/plan-mcp.test.js:447-457`). No `review_items`, `practice_events`, or `lesson_progress` in `mcp/tools.js`. |
| sec-2 | A bad token yields no working-copy JSON. | PASS | Unset env covered by ac-3. Test `a wrong bearer token is refused and the body has no plan JSON` passed (`test/plan-mcp.test.js:402-445`): HTTP 401, no `schemaVersion` in the body. |

## Definition of done

- [x] Seven plan tools registered in `mcp/tools.js` and each calls the matching function — ac-1.
- [x] `revision_conflict` includes the stored revision and the document is unchanged — ac-2.
- [x] Missing token returns no plan JSON; legacy registration intact — ac-3, ac-4.
- [x] `node --test test/plan-mcp.test.js test/mcp-auth.test.js` exit 0 — verifier: 8 pass, 0 fail.
- [x] `npm test` exit 0 — implementer: 459 pass, 0 fail, 2 todo.

## Tests run by the verifier

```text
node --test test/plan-mcp.test.js test/mcp-auth.test.js
→ tests 8 · pass 8 · fail 0 · duration_ms 791.051
```

Implementer blocking suite, same tree:

```text
npm test
→ tests 461 · pass 459 · fail 0 · todo 2 · exit 0
```

---

## Remediation log (feedback loop)

No loop. First verifier pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-005 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- No AC was passed by weakening a test or the criterion.
- The verifier noted ac-3's sentence names `read_part`, and the test also refuses the other six plan tools. That is stricter than the sentence, not a gap.
- Legacy token enforcement remains the HTTP bearer check in `test/mcp-auth.test.js`, which still passes. `list_lessons` and `get_lesson` do not call `requireMcpAuthToken` inside the handler; that matches their previous behavior.
