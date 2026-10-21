# SPEC-007 · Service: Lesson authoring service and MCP lesson assignment

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-002
title: Lesson authoring service and MCP lesson assignment
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-002, SPEC-006]
blocks: [SPEC-010]
```

---

## Description

**Context:** today lessons are published through the MCP `upsert_lesson` tool (`mcp/store.upsertLesson`). It writes `worksheet.js` and `catalog.js`, creates `index.html` from `_template` for new lessons, runs the validator, and rolls back on failure; `mcp/tools.js` then optionally generates audio. REQ-011 adds a browser editor for admins. This spec gives it the same single write path, so the browser and the MCP can never diverge, plus a safe audio step.

REQ-016 adds the operator's main use case: the AI authors lessons and assigns them to users through the MCP. The MCP server runs in the same process as the web server (`server/start.js`), so it can use the same database and visibility service.

**Scope (in):**
- `server/authoring.js`:
  - `loadForEdit(slug)` returns `{ worksheet, catalogEntry, visibility, audioSummary }`. It is read with `server/lesson-data.js`, never executed.
  - `validateDraft(slug, worksheet, catalogEntry)` returns the existing validator's errors and warnings without writing.
  - `save(actor, slug, worksheet, catalogEntry, { create })` calls `mcp/store.upsertLesson` (with `audio: false`), rebuilds the vocab index as the MCP path does, and writes an audit row. New lessons start with ADR-0002's default visibility (aud-2, proposed `hidden`).
  - `audioPlan(slug)` is a dry run: the counts of word, sentence, and story clips that would be generated or regenerated, and how many existing clips would be archived.
  - `generateAudio(actor, slug, { forceSentences })` runs `server/audio.js` with SPEC-002's archive guard. It is audited and refused while another generation for the same slug is running.
- Every write runs the SPEC-002 inventory before and verify after, and a verify failure is reported to the caller as an error.
- **MCP lesson assignment, in `mcp/tools.js`:**
  - `list_users` returns the usernames and display names of active accounts, and nothing else.
  - `upsert_lesson` gains an optional `visibility`: `"everyone"` or `{ "users": ["name", ...] }`. Unknown usernames fail the whole call before anything is written. When omitted, a **new** lesson takes the ADR-0002 default (proposed `hidden`) and the reply says so; an **existing** lesson keeps its current visibility.
  - A new tool `set_lesson_visibility` takes `slug` and `visibility` (`"everyone"`, `"hidden"`, or `{ "users": [...] }`).
  - Every visibility change is audited with the actor `mcp`, and the tool reply states the resulting visibility in plain words.
  - **Fail closed:** `list_users`, `set_lesson_visibility`, and the `visibility` argument refuse unless the MCP has `MCP_AUTH_TOKEN` configured.
  - No MCP tool reads learner progress, answer history, or account details beyond what `list_users` returns.

**Scope (out):** HTTP routes (SPEC-010); editor UI (SPEC-017); drafts separate from published files (ADR-0002: `hidden` visibility serves as the draft); a third "instructor" role.

---

## Understood (agent's read of the intent)

- [ ] und-1: The browser editor never writes lesson files itself. Everything goes through `mcp/store.upsertLesson`.
- [ ] und-2: Saving never generates audio. Audio is a separate, explicit, previewed action that costs ElevenLabs credit.
- [ ] und-3: Existing lesson `index.html` files are never edited. A new lesson gets one copied from `_template`, as today.

## Acceptance Criteria

- [ ] ac-1 (REQ-011): `save` on an existing lesson produces the same `worksheet.js` and catalog entry as the MCP `upsert_lesson` given the same input. The test compares file bytes.
- [ ] ac-2 (REQ-011): `save` with a validator error writes nothing and returns the errors. The prior files are byte-identical afterward.
- [ ] ac-3 (REQ-012): `generateAudio` after a sentence text change archives the old clip (SPEC-002) and passes `audio:verify`. A forced verify failure surfaces as an error.
- [ ] ac-4: `audioPlan` makes no network call and writes no file, and its counts match what `generateAudio` then does (fake TTS).
- [ ] ac-5 (REQ-014): a slug that fails `assertSlug`, or `_template`/`_audio`/`_review`, is refused. A worksheet value containing `</script>`, a backtick, or `${` round-trips as data and never runs.
- [ ] ac-6 (REQ-010): a newly created lesson has the default visibility from ADR-0002 aud-2.
- [ ] ac-7: an existing lesson's `index.html` is unchanged after `save` (hash compared).
- [ ] ac-8 (REQ-016): through the MCP, `upsert_lesson` with `visibility: { users: ["alice"] }` creates a lesson that alice can see and bob cannot. With `"everyone"` both can. With nothing given for a new lesson, the lesson is `hidden` and the reply says so. For an existing lesson, no visibility argument leaves visibility unchanged.
- [ ] ac-9 (REQ-016): `set_lesson_visibility` changes access immediately, writes an audit row with actor `mcp`, and rejects an unknown username without changing anything.
- [ ] ac-10 (REQ-016): `list_users` returns only usernames and display names of active accounts. A scan of its output finds no hash, token, role, or activity field.
- [ ] ac-11 (REQ-016): with `MCP_AUTH_TOKEN` unset, `list_users`, `set_lesson_visibility`, and the `visibility` argument all refuse with a clear message, and no lesson is written when `visibility` was supplied. Plain `upsert_lesson` without `visibility` keeps today's behavior.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Validator warnings (nudges from INIT-001 SPEC-008) | shown to the admin, do not block save; errors block | resolved | yes — matches MCP behavior |

---

## Deliverables

- [ ] `server/authoring.js`
- [ ] `mcp/tools.js` (`list_users`, `set_lesson_visibility`, `visibility` on `upsert_lesson`, reply text); `mcp/store.js` (only if a small export is needed; behavior unchanged for existing MCP calls)
- [ ] Tests: `test/authoring.test.js` (temp `LESSONS_DIR`, fake TTS, no network); `test/mcp-assignment.test.js` (MCP tool handlers against a temp database and two users, with and without `MCP_AUTH_TOKEN`)

---

## Technical Requirements (domain-specific)

- [ ] No network in tests. Never read `.env`; the ElevenLabs key is read from `process.env` by `server/audio.js` only.
- [ ] A per-slug in-process lock for `generateAudio`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`, plus the §6 audio gate (`audio:inventory` before, `audio:verify` after) on the real tree.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** required. Triggers matched: admin writes to files that every learner's browser loads; spend on an external API; an AI-driven tool that changes who can see what.
- **Review depth:** full. Security acceptance: ac-2, ac-5, ac-7, ac-10, and ac-11, plus the single-flight audio lock.

---

## Verification Strategy

- **Claim:** browser-edited lessons are written exactly as MCP-authored ones. Invalid input writes nothing, and audio generation can't lose a recording.
- **Check + executor:** `mechanized`. `node --test test/authoring.test.js`, plus `npm run audio:verify` on the real tree after the suite.
- **Pass condition:** all tests pass. The byte-equality test against the MCP path passes, and `audio:verify` exits 0.

---

## Integration Points

- Consumed by SPEC-010 (admin lesson routes) and SPEC-017 (editor UI).
- **Provenance tag:** `INIT-002/SPEC-007`
