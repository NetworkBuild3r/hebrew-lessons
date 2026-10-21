# SPEC-012 AC Verification Eval Report — Media and image-generation MCP tools

**Traceability:** INIT-004/SPEC-012 \
**Verifier:** independent sub-agent that did not author the code · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 (prompt) · read-only by prompt (the Task tool has no readonly flag) \
**Evaluated at:** 2026-10-09T20:30:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | `upload_media`, `plan_image`, and `generate_image` are registered, and the SPEC-005 plan tools stay registered. | PASS | Tools registered in `mcp/tools.js` (`upload_media` 714–735, `plan_image` 737–753, `generate_image` 755–780). Plan tools remain at 614–711. Test `ac-1: media tools and the SPEC-005 plan tools are registered` (`test/media-generation-mcp.test.js`). `node --test test/media-generation-mcp.test.js test/plan-mcp.test.js` → 18 pass, 0 fail, exit 0. Plan suite `ac-4` still passes. |
| ac-2 | `plan_image` does not call the transport. `generate_image` without confirm does not call the transport. Both require the token. | PASS | `callMediaTool` calls `requireMcpAuthToken` before the service (`mcp/tools.js`). Tests `ac-2: plan_image does not call the transport and returns the dry-run` and `ac-2: generate_image without confirm does not call the transport`: transport count 0. Unset token: `a confirm string and a missing token do not return an image body`. |
| ac-3 | No registered tool approves media. HTTP approval with only the MCP token returns 403. | PASS | No handler name matches `/approv/i`. `refuseMcpApproval` runs before the session guard (`server/api/media-routes.js`). Test `ac-3: no tool approves, and HTTP approval with only the MCP token is 403` → status 403, row stays `pending`. |
| sec-1 | A bad or missing token returns no image body. | PASS | Unset `MCP_AUTH_TOKEN` refuses all three tools with no PNG body. Wrong bearer on `/mcp` is 401 and the body has no image (`a wrong MCP bearer is refused and the body has no image`). |
| sec-2 | The tool list has no approve tool. | PASS | Same registration scan as ac-3. `handlers.approve_media` is undefined. `get_learner_snapshot` is absent. |
| sec-3 | Errors do not contain the API key. | PASS | `safeErrorMessage` drops a message that contains the configured key. Test `an ingest error that contains the API key is not returned`. |

## Definition of done

- [x] `node --test test/media-generation-mcp.test.js` exits 0 — verifier re-ran it with `test/plan-mcp.test.js`: 18 pass, 0 fail.
- [x] Confirm-less generate has a zero transport count — ac-2.
- [x] No tool name approves media — ac-3.

## Tests run by the verifier

```text
node --test test/media-generation-mcp.test.js test/plan-mcp.test.js
→ tests 18, pass 18, fail 0, exit 0
```

---

## Remediation log (feedback loop)

| Loop | Failing AC | Fix applied | Re-verify result |
| ---- | ---------- | ----------- | ---------------- |
| 0 | — | — | PASS on the first verifier pass |

**Loops used:** 0 of 3.
