# SPEC-010 Completion Summary — MCP and HTTP adapters for profile, coverage, lexicon, library, and the AI-planning switch

**Traceability:** INIT-006/SPEC-010
**Status:** complete (orchestrator closed 2026-10-10 — 14/14 spec tests, suite 823/823)
**Domain:** api
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `mcp/tools.js` — `registerLearnerTools`, called from `registerLessonTools`
- [x] `server/api/learner-routes.js` — `GET /api/me/profile`, `POST /api/me/coverage`, `GET /api/me/mix`
- [x] `server/api/admin-routes.js` — `PUT /api/admin/lexicon`, `PATCH /api/admin/lexicon/:key/verified`, `PATCH /api/admin/users/:id/ai-planning`
- [x] Tests: `test/learner-mcp-tools.test.js`, `test/learner-parity.test.js`, `test/ai-planning-switch.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-013): a parity test calls each INIT-006 §12 row through MCP and HTTP where both exist and both reach the same service function.
- [x] ac-2 (REQ-007): every new tool refuses without `MCP_AUTH_TOKEN`; `get_learner_profile` with the switch off returns `{ error: "planning_off" }` only.
- [x] ac-3 (REQ-007): `PATCH /api/admin/users/:id/ai-planning` returns 401 signed out, 403 for a non-admin, 200 for an admin, and writes `ai_planning_switch` with the previous and new values.
- [x] ac-4: the 51st `add_library_words` word in a learner's local day is refused and nothing past the cap is written. A call that names a key missing from the lexicon returns `unknown_keys` and writes no `user_words` or `review_items` row.
- [x] ac-5: `list_users`, `list_words`, and `list_activity_types` still return their current shapes. The eight legacy tool names are still registered. `npm test` is green.

## AC Verification (independent)

Not run from this process. This agent wrote the code, so it does not grade the acceptance criteria. The orchestrator dispatches the fresh verifier (Phase 4.5, full tier).

**Verification tier:** `full` · **pre-flight:** the dispatch did not classify this spec as light; `security_review: required` makes the light tier ineligible · **checks skipped:** `None`

| AC | Implementer evidence (not an independent verdict) |
| ---- | ------------------------------------------------- |
| ac-1 | `ac-1: each §12 row reaches the same service…` in `test/learner-parity.test.js`. Spies on `library.list`, `library.add`, `library.setSuspended`, `cards.deck`, `progress.applySignals`, `profile.buildProfile`, `coverage.checkCoverage`, `mix.compose`, `concepts.list`, `lexicon.list`, `lexicon.get`, `lexicon.upsert`, `lexicon.setVerified`, `accounts.setAiPlanning`. |
| ac-2 | `ac-2: every new tool refuses without MCP_AUTH_TOKEN`; `ac-2: get_learner_profile with the switch off returns planning_off only`. |
| ac-3 | `ac-3: 401 signed out, 403 for a user, 200 for an admin, and an audit row`. Actor `admin:admin`, detail `{ previous: 0, next: 1 }`. |
| ac-4 | `ac-4: an unknown lexicon key writes nothing`; `ac-4: the 51st word in a local day…`. A 2-word call at 49 words is refused with both keys unwritten; the 50th then succeeds; the 51st returns `daily_cap`. |
| ac-5 | `ac-5: the eight legacy tools keep their names and list shapes`. `list_activity_types` deep-equals the catalog. `list_users` rows have only `username` and `displayName`. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-010`
- **Title:** MCP and HTTP adapters for the learner profile, coverage, lexicon, library, and the AI-planning switch
- **Files changed:** 9 (4 new, 5 modified)
- **Change status:** proposed
- **Reversibility:** branch. Rollback: remove `registerLearnerTools` and its call, delete `server/api/learner-routes.js`, remove the three admin routes and `accounts.setAiPlanning`, and drop the three test files. No migration. No new npm dependency.

The adapters call the services. MCP passes `audience: 'mcp'` and the tool's username. HTTP passes `audience: 'self'` and `ctx.user.id`. Neither forwards a request `now`. `GET /api/me/mix` calls `mix.compose(userId, Date.now())` with no options object. A profile `strength` of `null` is left in the JSON.

`add_library_words` checks the switch, then the lexicon, then the daily cap, and only then calls `library.add` with `source: 'ai'`. A call that would pass 50 new `ai` words on the server's local day is refused whole. `set_lexicon_verified` and `set_ai_planning` are registered so an attempt returns `{ error: "forbidden" }`. They do not call `lexicon.setVerified` or `accounts.setAiPlanning`.

The admin switch body is `{ enabled: true|false }`. The 200 body is `{ ai_planning, previous, next }`.

---

## Tests

| Test file | New tests | Result |
| ------------- | --------- | ------ |
| `test/learner-mcp-tools.test.js` | 10 | pass |
| `test/learner-parity.test.js` | 2 | pass |
| `test/ai-planning-switch.test.js` | 2 | pass |

**All tests passing:** Yes · `node --test` on the three files → 14/14, exit 0 · `npm test` → 823/823, exit 0 · **Coverage (new code):** no coverage collector is configured. The parity spies and the cap/unknown-key assertions are the evidence the new branches run.

Quality gates: no lint, type-check, or coverage tool is configured. `node --check` is clean on the changed JavaScript. No suppression comment was added and no threshold was moved.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | the three spec test files and `npm test` | Both exited 0. No migration, no new dependency. |

None — branch/reversible only.

**Blocker for `complete`:** independent AC verification has not been run by a fresh agent. No product defect is open.

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** inline review by the implementing agent (`self_reviewed: true`). A separate security-review subagent was not dispatched from this process. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0 · Info: 2 (accepted)
- **Unresolved Critical/High:** None

| Finding | Severity | Control | Location | Status |
| ------- | -------- | ------- | -------- | ------ |
| SEC-I-01: the daily cap is checked and then `library.add` runs, with no second lock | Info | resource limit | `mcp/tools.js` `addLibraryWords` | accepted — the handler does not await between the check and the write, and the process is one replica |
| SEC-I-02: coverage text longer than 4096 characters is refused as `invalid_input` before the service writes an audit row | Info | audit completeness | `mcp/tools.js` `check_text_coverage` | accepted — the text is not stored or echoed; 4001–4096 characters still reach `checkCoverage` and are audited as `invalid_text` |

- **Positive observations:** every new tool calls `requireMcpAuthToken` before any service. Zod failures return `{ error: "invalid_input" }` and do not include the Zod message, so a long coverage text is not copied into the reply. Profile and coverage options are `{ audience }` only. Mix takes the server clock and no request options. Library adds use the session user on HTTP and the username lookup on MCP; `source` is `self` or `ai`, never a request field. Unknown keys and the daily cap return before `library.add`. The library audit detail is `{ outcome, count }` and the test checks that the refused key is absent. A missing account is audited with a null target. `verified` and `source` are stripped before lexicon upsert; the stored MCP row is `source: mcp` and `verified: 0`. The verify and switch tools do not call the success functions. The switch route is session-plus-admin, accepts only `enabled`, and the audit actor is `admin:<username>` with previous and next. A library add does not consume the 60-read profile hour.

---

## Files Changed

Provenance is `INIT-006/SPEC-010` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `mcp/tools.js` | Modified — `registerLearnerTools` and the library-add checks | `INIT-006/SPEC-010` |
| `server/api/learner-routes.js` | Created — profile, coverage, and mix routes | `INIT-006/SPEC-010` |
| `server/api/admin-routes.js` | Modified — lexicon write, verify, and the AI-planning switch | `INIT-006/SPEC-010` |
| `server/accounts.js` | Modified — `setAiPlanning` writes the `ai_planning_switch` audit row | `INIT-006/SPEC-010` |
| `server/web.js` | Modified — mounts `learner-routes` | `INIT-006/SPEC-010` |
| `test/helpers/signed-in-server.js` | Modified — clears the new route module from the require cache | `INIT-006/SPEC-010` |
| `test/learner-mcp-tools.test.js` | Created — 10 tests | `INIT-006/SPEC-010` |
| `test/learner-parity.test.js` | Created — 2 tests | `INIT-006/SPEC-010` |
| `test/ai-planning-switch.test.js` | Created — 2 tests | `INIT-006/SPEC-010` |
| `test/media-generation-mcp.test.js` | Modified — expects `get_learner_profile` to be registered; `get_learner_snapshot` stays absent | `INIT-006/SPEC-010` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 sentinel, Phase 1 steps, Phase 4.5 dispatches | `trace_open`, `task_step`, `dispatch`, `trace_complete` | The orchestrator owns the timer and trace for a dispatched spec. `scripts/dev/lib/sdd_trace` is not in this repo. | No `.sdd-trace/SPEC-010.jsonl` from this process. | A Tier-5 reader finds no trace and must not read PASS. Same gap as the INIT-006 SPEC-007 summary. |

---

## Deviations from the Spec

- Commit, push, and PR were not done. The dispatch said not to commit.
- `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. The work followed the spec: thin adapters, zod limits, services own the rules.
- **`accounts.setAiPlanning`.** The §12 row names that function. `users.setAiPlanning` stores the flag and does not audit. The new accounts function calls it and writes `ai_planning_switch`.
- **Grade route.** §12 says `POST /api/progress/signals`. The existing route is `POST /api/review/signals`. The parity test calls that route. No second grade route was added.
- **Service names.** §12 says `profile.build` and `coverage.check`. The services export `buildProfile` and `checkCoverage`.
- **Refusal tools.** `set_lexicon_verified` and `set_ai_planning` are registered so an attempt has a forbidden result. Their descriptions say not to call them.
- **Daily cap.** Counted `user_words` rows with `source: 'ai'` whose `added_at` falls on the server's local calendar day. A call that would pass 50 new words is refused entirely.
- **Switch JSON.** The body field is `enabled`. SPEC-017 can read `ai_planning`, `previous`, and `next` from the 200 response.
- **`test/media-generation-mcp.test.js`.** It asserted `get_learner_profile` was absent, which was true for INIT-004. This spec registers that tool, so the assertion now expects the function. The eight legacy handlers were not edited.
- SPEC-014 and SPEC-018 files were not edited.

### Notes for later specs

- SPEC-015 calls `GET /api/me/mix` and receives `{ sections }`.
- SPEC-017 calls `GET /api/me/profile` (the profile plus `ai_planning`) and `PATCH /api/admin/users/:id/ai-planning` with `{ enabled: boolean }`.
- SPEC-019 tool names: `get_learner_profile`, `check_text_coverage`, `list_grammar_concepts`, `list_lexicon`, `get_lexicon_entry`, `upsert_lexicon_entries`, `add_library_words`. `planning_off` means the switch is off. The profile must not be pasted into a lesson file.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
