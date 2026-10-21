# SPEC-010 · API: Admin endpoints — users, progress, visibility, lessons

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-002
title: "Admin endpoints: users, progress, visibility, lessons"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-005, SPEC-007, SPEC-008]
blocks: [SPEC-016, SPEC-017]
```

---

## Description

**Context:** the admin console (SPEC-016/017) needs JSON routes over the account (SPEC-004), progress (SPEC-005), visibility (SPEC-006), and authoring (SPEC-007) services. Every route is admin-only on the server (REQ-003, REQ-008) and audited.

**Scope (in):** `server/api/admin-routes.js`, registered with `requireAdmin`:

| Group | Route | Request | What it does |
| --- | --- | --- | --- |
| Users | `GET /api/admin/users` | — | Lists users: id, username, display name, role, status, created, last sign-in. Never hashes. |
| Users | `POST /api/admin/users` | `{ username, displayName?, role, password }` | Creates a user. The password goes through the policy check. |
| Users | `PATCH /api/admin/users/:id` | `{ displayName?, role?, status? }` | Updates a user. The last-admin and self-disable guards return 409. |
| Users | `POST /api/admin/users/:id/password` | `{ password }` | Resets the password and forces a change at next sign-in. |
| Progress | `GET /api/admin/users/:id/progress` | — | Returns metrics plus trouble words. |
| Progress | `GET /api/admin/users/:id/events?before=&limit=` | — | Returns the learner's full answer history, newest first, paged (limit 1 to 200). Each read writes an audit row (admin, target user), so reads of a learner's history are traceable. |
| Lessons | `GET /api/admin/lessons` | — | Lists every lesson with visibility mode and grant count. |
| Lessons | `PUT /api/admin/lessons/:slug/visibility` | `{ mode, userIds? }` | Sets visibility and grants. |
| Lessons | `GET /api/admin/lessons/:slug` | — | Returns editor data (`authoring.loadForEdit`). |
| Lessons | `POST /api/admin/lessons/:slug/validate` | `{ worksheet, catalogEntry }` | Returns errors and warnings without writing. |
| Lessons | `PUT /api/admin/lessons/:slug` | `{ worksheet, catalogEntry }` | Saves the lesson. Body limit 1 MB. |
| Lessons | `POST /api/admin/lessons` | `{ slug, worksheet, catalogEntry }` | Creates a lesson; 409 if the slug exists. |
| Audio | `GET /api/admin/lessons/:slug/audio-plan` | — | Dry-run clip counts. |
| Audio | `POST /api/admin/lessons/:slug/audio` | `{ forceSentences? }` | Generates audio. 409 while a run for that slug is in progress. Throttled per admin (ADR-0002). |

**Scope (out):** UI (SPEC-016/017).

---

## Understood (agent's read of the intent)

- [ ] und-1: The role check runs on the server for every admin route. Hiding the Admin link in the UI is not a control.
- [ ] und-2: No admin route ever returns a password hash, session token, or the ElevenLabs key.

## Acceptance Criteria

- [ ] ac-1 (REQ-003): every route in the table returns 403 for a user-role session and 401 signed out. The test iterates the route table, so a new route without `requireAdmin` fails it.
- [ ] ac-2 (REQ-008): demoting or disabling the last active admin returns 409. An admin disabling themselves returns 409. A disabled user's existing session gets 401 on its next request.
- [ ] ac-3 (REQ-008): each mutating route writes exactly one audit row with actor, action, and target.
- [ ] ac-4 (REQ-009): `GET /api/admin/users/:id/progress` returns metrics and trouble words. `GET …/events` returns that user's events only, newest first, with a cursor, and clamps the limit at 200. A user-role session gets 403 on both.
- [ ] ac-4b (REQ-009): each `…/events` read writes one audit row naming the admin and the target user.
- [ ] ac-5 (REQ-011): a `PUT` lesson save with validator errors returns 422 with the errors, and the files are unchanged. A valid save returns 200, and the lesson is immediately served with the new content.
- [ ] ac-6 (REQ-012): `POST …/audio` passes `audio:verify` afterward. A concurrent second request for the same slug gets 409.
- [ ] ac-7: no response body from any admin route contains `password_hash`, a session token, or the value of `ELEVENLABS_API_KEY`. The test scans the responses for the test key and fields.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Admin audio throttle | 1 generation in flight per slug; at most 10 runs per admin per hour | resolved | yes — ADR-0002 §6 states these numbers |

---

## Deliverables

- [ ] `server/api/admin-routes.js` (registered in `server/web.js`)
- [ ] Tests: `test/api-admin.test.js` (live server; admin, user, and second-admin fixtures; fake TTS)

---

## Technical Requirements (domain-specific)

- [ ] Route table defined as data, so ac-1 can iterate it.
- [ ] The lesson-save body limit (1 MB) applies only to the lesson routes; everything else uses 64 KB.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`, plus the §6 audio gate.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** required. Triggers matched: authorization (admin privilege), PII, file writes, external spend.
- **Review depth:** full. Security acceptance: ac-1, ac-2, ac-3, ac-4b, ac-6, and ac-7.

---

## Verification Strategy

- **Claim:** only admins can call admin routes. The account guards hold, every change is audited, and no secret leaves the server.
- **Check + executor:** `mechanized`. `node --test test/api-admin.test.js` runs against a live server with a fake TTS. `npm run audio:verify` runs afterward.
- **Pass condition:** all tests pass. The route-table authorization test covers every route, the secret-scan test finds none of the forbidden values, and `audio:verify` exits 0.

---

## Integration Points

- Consumed by SPEC-016 (users and learner progress UI) and SPEC-017 (lessons, visibility, editor UI).
- **Provenance tag:** `INIT-002/SPEC-010`
