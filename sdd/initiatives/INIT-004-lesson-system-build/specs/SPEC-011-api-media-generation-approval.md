# SPEC-011 · API: Media, generation, and approval

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-004
title: "Media, generation, and approval routes"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-008, SPEC-009]
blocks: [SPEC-016, SPEC-019]
```

---

## Amended 2026-10-09 by INIT-006/SPEC-001

The route `PATCH /api/admin/users/:id/ai-planning` moved to INIT-006 SPEC-010, so the learner profile does not wait on the image pipeline (ADR-0009 §1, Accepted 2026-10-09). The `users.ai_planning` column stays in SPEC-006. Effort went from 5h to 4h. The dependency on SPEC-010 is gone, because nothing here reads the switch any more.

---

## Description

**Context:** ADR-0004 §4 is the matrix. This spec is the browser columns for media. MCP columns are SPEC-012, except the 403s this spec must already return if a session is the wrong role.

**Scope (in):**

| Route | Signed out | `user` | `admin` |
|-------|------------|--------|---------|
| `POST /api/media` | 401 | 403 | Allow, then ingest |
| `GET /api/media/:id` | 401 | 200 only when a lesson this user may see references the id; otherwise 404 | 200 when the id exists; 404 when it does not |
| `POST /api/admin/media/:id/approval` | 401 | 403 | Allow. Refuses empty alt. |
| `GET /api/admin/images/image-plan` | 401 | 403 | Dry-run. No provider call. |
| `POST /api/admin/images` | 401 | 403 | Generate only with `confirm: true` and the caps |

- Publish of a lesson whose visibility is `everyone` or `selected` refuses while it references a `pending` image. A `hidden` lesson may reference `pending`.
- Fetch sets `Content-Type: image/png` and `X-Content-Type-Options: nosniff`.
- Approval is admin session plus the existing CSRF checks. It is not an MCP route.
- Audit rows for upload, approve, and delete, as ADR-0004 §10. The generation prompt is not copied into the log line.

**Scope (out):** MCP tools (SPEC-012); Kubernetes secret (SPEC-017); builder panels (SPEC-016); the AI-planning switch route (INIT-006 SPEC-010).

---

## Understood (agent's read of the intent)

- [x] und-1: Missing and forbidden media ids are both 404 for a non-admin.
- [x] und-2: An admin may approve their own upload. The approval is its own audited action. Upload is not itself approval.

## Acceptance Criteria

- [x] ac-1 (REQ-006): the matrix cells for upload, fetch, and approval match the table for signed-out, user, and admin. A user fetch of an unreferenced id is 404.
- [x] ac-2 (REQ-007): `image-plan` returns the dollar estimate and the fake transport records zero calls. `POST /api/admin/images` without confirm records zero calls.
- [x] ac-3 (REQ-006): publishing a plan to `everyone` while an introduction word references a `pending` media id is refused. The same reference on a `hidden` lesson is allowed.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Delete route | Admin delete is in this spec if a route is required to reach the archive function. It is `POST /api/admin/media/:id/delete` or the method the router already uses for similar admin actions. It is not granted to `user` or to MCP. | resolved | yes — ADR-0004 §9 admin only |

> **Readiness** = 100.

---

## Deliverables

- [x] Routes on the existing API router
- [x] Tests: `test/media-routes.test.js`

---

## Technical Requirements (domain-specific)

- [x] Handlers call ingest and the image client. They do not re-encode or call OpenAI themselves.
- [x] Multipart limits reject a body over 5242880 bytes before the pipeline sees it.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/media-routes.test.js` and `npm test`.
- **Rollback plan:** unmount the routes. Archived files stay archived.

---

## Security

- **Sensitivity:** required. Triggers matched: upload, generation key path, approval.
- **Security acceptance criteria:**
  - [x] Every row in the table is tested, including 401 and 403.
  - [x] The generation response and the error body do not contain `OPENAI_API_KEY`.
  - [x] CSRF on every mutation, same as the other admin routes.
- **Review depth:** thorough · **focus areas:** authz matrix, file upload, secrets in responses.

---

## Verification Strategy

- **Claim:** the ADR-0004 browser matrix is what the router returns, and generation is not contacted on a dry-run.
- **Check + executor:** `mechanized` — `node --test test/media-routes.test.js`.
- **Pass condition:** exit 0; each matrix cell has an assertion; the transport counter stays 0 on dry-run and on missing confirm.
- **Test levels:** HTTP integration, temporary `DATA_DIR`, injected image transport. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-016's media library calls these routes. SPEC-012 mirrors upload, dry-run, and generate, and must receive 403 for approval.
- Produces **Change:** media HTTP API.
- **Provenance tag:** `INIT-004/SPEC-011`
