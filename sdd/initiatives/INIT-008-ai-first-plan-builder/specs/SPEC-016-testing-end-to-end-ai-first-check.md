# SPEC-016 · Testing: End-to-end AI-first check

## Metadata

```yaml
spec_id: SPEC-016
initiative_id: INIT-008
title: "An MCP-only script builds and publishes a plan; a simulated human fix lands; the AI re-edit sees editState and succeeds; restore brings the human value back; the admin screens show it"
domain: testing
status: complete
primary_prompt: .claude/agents/test-automator/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-009, SPEC-010, SPEC-012, SPEC-013, SPEC-014, SPEC-015]
blocks: [SPEC-017]
```

---

## Description

**Context:** each earlier spec proves its own part. This spec proves the **loop the operator asked for** (D1, D2, D4): the AI builds and publishes with no human step, a person can fix what it got wrong, the AI sees that a person did, the AI is not stopped from editing again, and either actor can put the person's value back — all through the operations layer, with the admin screens as a second client. It adds no feature. It is the check that the initiative's claim holds across the seams.

**Scope (in):**

- **`test/plan-ai-first-e2e.test.js`** (`node --test`, runs in `npm test`) — an in-process server (HTTP and MCP) on a temporary database, with a signed-in test admin (`test/helpers/signed-in-server.js`) and the MCP token set to a test value. **No network and no ElevenLabs or OpenAI call:** the test installs a guard that throws on any outbound `https.request` or `fetch` to a non-local host, and sets no provider key. The scenario, in order, using **only MCP tool calls** for the AI's steps:
  1. `create_plan` (`example-plan`, recipe `topic-conversation`) → draft, revision 1; the plan is absent from `GET /api/plans/:id/published` (404) and present in `list_plans`.
  2. Build: `insert_block` an `introduction` with two words (one pointed form is deliberately not NFC; one differs from a seeded **verified** lexicon entry with the same stripped key), `set_field` a sentence and an `audioPath` on a word, `set_field` the lesson `newWords`, `buildsOn`, `recycles`, and `visibility`.
  3. `list_plan_issues` → returns `pointing-form` and `lexicon-form` as warnings with id paths and `fix.suggested`; the AI applies both fixes with `set_field`; `list_plan_issues` no longer lists them. One other warning is left unfixed on purpose.
  4. `publish_plan` → succeeds **with the warning still present and with no approval argument or step**; `GET /api/plans/:id/published` now returns the snapshot for a lesson with `visibility: 'everyone'` (the accepted D1 consequence, asserted so it is visible and documented).
  5. **Simulated human fix** through the admin HTTP route (`PATCH /api/plans/:id/fields`, session `example-admin`) of a word's `meaning` and of its `sentence`; the plan is `draft` again at a higher revision.
  6. `read_part` (MCP) on that word → `editState.humanEdited` lists both fields with `previousValue` equal to the AI's earlier text and `previousWriter: 'ai'`; `lastHumanEdit` is the human's revision; **the result contains no username** (`example-admin` appears nowhere in any MCP result in the whole run).
  7. `list_plan_issues` → `audio-stale` is reported for the edited sentence whose `audioPath` was left (published plan, no snapshot match on text, same clip path), with its fix text.
  8. **The AI re-edits the human-fixed `meaning`** with `set_field` → **succeeds**, no refusal, and the result lists the field in `overwrote` with the person's value.
  9. `list_plan_revisions` → shows, in order, create, the AI's edits, the publish, the human's two edits, the AI's re-edit; `actorKind` values `ai` and `human`, the human rows named `human` for the MCP caller. `diff_plan` (`working` against `published`) lists the changed fields with id paths.
  10. `restore_plan_revision` (field `meaning`, from the human's revision) → succeeds in one call, creates one new revision, sets the person's value back, leaves the AI's other edits, and `list_plan_revisions` shows the restore row. The same restore is repeated through `POST /api/plans/:id/restore` by the admin on a second field with the same result.
  11. `assign_plan` to a fixture active account then back to `shared` through the MCP; an attempt to change `audience` through `set_field` returns `assign_required`; a `replace_part` with a mismatched id returns `id_mismatch`.
  12. Counts and invariants at the end: the number of `plan_revisions` rows equals the number of successful writes; the number of `plan.*` audit rows equals the same; every audit `detail` is value-free; `upsert_lesson`, `list_lessons`, and the worksheet tools still work (one call each); `set_lexicon_verified` and `set_ai_planning` still return `forbidden`.
- **No-refusal search:** the test searches the plan modules, routes, and plan-tool metadata for `field_pinned`, `keepPinned`, `pinned_by`, `pinned_at`, a column or key named `pinned` or `locked`, and any error string that mentions a person or a mark, and finds none.
- **Final Chrome smoke:** extend `test/chrome-plan-editor-smoke.js` with one end-to-end run on the database the e2e test built (or an equivalent fixture): the list shows `example-plan`; the History panel shows `ai` and `human` marks; the Issues panel lists the left-over warning; the edited-by-a-person mark appears on the human-edited field; restoring from the diff in the UI creates a new revision; the page makes only same-origin requests; contrast, target-size, and reduced-motion checks pass on the editor routes.
- **Suite check:** `npm test` passes in full; every test file added by SPEC-002 to SPEC-015 is in the run; the parity test (SPEC-010) and the drift test (SPEC-012) pass; `npm run audio:verify` matches the inventory taken at the start of the spec (no clip touched).

**Scope (out):** new features; fixing a defect found here is done in a follow-up change to the spec that owns it, recorded in this spec's completion summary.

---

## Understood (agent's read of the intent)

- [x] und-1: The AI's steps use the MCP tool interface, not the service functions, so the test exercises the adapters and the token gate. _(A real `McpServer` and `Client` joined with `InMemoryTransport`, as `test/plan-builder-mcp.test.js` does. The Chrome smoke calls the registered handlers directly, which skip the SDK's argument check but run the same adapter code.)_
- [x] und-2: The human's steps use the admin HTTP route with a session, so the test exercises the channel-derived actor.
- [x] und-3: Step 4 asserts, rather than hides, that the AI can publish and expose a lesson with no person involved. The initiative accepts that (ADR-0011 residual risk); the test documents it.

## Acceptance Criteria

- [x] ac-1 (REQ-014, REQ-019): steps 1 to 4 pass with only MCP calls: a plan is created, built, shown its issues, fixed, and published with a warning still present, with no human step, no approval argument, and no ElevenLabs or OpenAI call (the outbound guard never fires). _(Met. Steps 1 to 4 each pass; `publish_plan` has only `id` and `expectedRevision`. The guard replaces `fetch`, `http.request`, `https.request`, and `https.get` and is itself tested with a remote host. See "Notes" for the one hard finding the first issue list holds.)_
- [x] ac-2 (REQ-014, REQ-006): steps 5 to 8 pass: the human's fix is visible to the AI as `editState` with `previousValue` and `previousWriter`, the username appears in no MCP result, and the AI's re-edit of the human-fixed field succeeds and returns `overwrote`. _(Met. `example-admin` is checked in every MCP result of the run (a transcript of 40 or more calls) and in the token's absence from it.)_
- [x] ac-3 (REQ-014, REQ-005): steps 9 and 10 pass: history lists the writes in order with the right actor kinds; `diff_plan` lists the changed fields; restore by the MCP and by the admin route each create exactly one new revision with the person's value restored and the AI's other edits intact. _(Met. History equals the list of writes the test made, row for row; the admin route shows the account name where the MCP shows `human`. The two restores deep-equal the earlier document apart from the one field and the revision.)_
- [x] ac-4 (REQ-008, REQ-018): step 7 reports `audio-stale` for the edited sentence with its clip path left, and steps 3 reports `pointing-form` and `lexicon-form` with fixes that clear them in one `set_field` each. _(Met. See the deviation on the clip field below: it is `sentenceAudioPath`.)_
- [x] ac-5 (REQ-002, REQ-003): step 11 and 12 pass: `assign_required` and `id_mismatch` are returned; revision rows equal successful writes; audit rows equal them and hold no value; the three human-only acts remain refused for the AI. _(Met. Revision rows, `plan.*` audit rows, and the test's own count of successful writes are all equal (24 in the run); audit detail keys are within `revision`, `path`, `field`, `tool`, `requestId` and hold none of eleven known values. `set_lexicon_verified` and `set_ai_planning` answer `forbidden`; the seed row stays as it was; no tool name mentions approval, and the picture-approval route exists with no tool.)_
- [x] ac-6 (REQ-006, REQ-017): the no-refusal search finds nothing; the existing worksheet tools, `upsert_lesson`, and the learner 404 for a draft still behave as before. _(Met. The search covers the plan modules, repos, migration 006, the plan routes, every error code those files can return, the three table schemas, and every plan tool's metadata. The set of error codes met in the whole MCP run is exactly `assign_required`, `forbidden` (the two human-only tools), `id_mismatch`, and `revision_conflict`.)_
- [x] ac-7 (REQ-012, REQ-013, REQ-014): `node test/chrome-plan-editor-smoke.js` exits 0 with the end-to-end additions; `npm test` and `npm run audio:verify` pass. _(Met. The smoke passes (about 29 s); `npm test` with `SRS_SCHEDULER=ladder` is 1460 of 1460; all 20 `test/chrome-*-smoke.js` pass; `npm run audio:verify` is OK with 65 recordings.)_

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | The simulated human | The admin HTTP route with a test session, not a browser, for the service-level run; the Chrome smoke covers the browser. | resolved | yes |
| aud-2 | `audio-stale` fixture | A word with an `audioPath` matching the schema pattern, published, then its sentence edited. No audio file is needed or generated. | resolved | yes — the rule compares text and path only |
| aud-3 | Chrome availability | The Chrome smoke needs Chrome at the default Windows path or `CHROME_PATH`, as the other smokes. It is not part of `npm test`. | resolved | yes — existing pattern |

> **Readiness** = 100.

---

## Deliverables

- [x] `test/plan-ai-first-e2e.test.js`
- [x] Additions to `test/chrome-plan-editor-smoke.js`
- [x] Completion summary listing each finding and which spec owns the fix, if any (`completion-summaries/SPEC-016-completion-summary.md`)

---

## Technical Requirements (domain-specific)

- [x] Temporary database and in-process servers only. No test opens `data/hebrew.sqlite`. No network. No provider key set. _(The five provider key names are set to the empty string, not deleted, because `server/audio.js` reads a local `.env` into any variable that is undefined; the test asserts each stayed empty. See the completion summary.)_
- [x] Fixtures use `example-admin` and `example-learner`; Hebrew in fixtures is hand-written.
- [x] The AI's steps call tools through the MCP interface with the test token.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-ai-first-e2e.test.js`, `node test/chrome-plan-editor-smoke.js`, `npm test`, `npm run audio:verify`.
- **Rollback plan:** revert the test files; no product change.

---

## Security

- **Sensitivity:** standard. The test touches no secret and no real data; it asserts that no human username reaches an MCP result and that audit rows hold no value.
- **Review depth:** standard · **focus areas:** the outbound-call guard, the username-absence assertion.

---

## Verification Strategy

- **Claim:** the AI-first loop works end to end across the MCP, the admin routes, and the admin screens: an AI can build, publish, see a person's fix, edit again, and restore, with nothing refusing it because of who wrote last.
- **Check + executor:** `mechanized` — the four commands above.
- **Pass condition:** exit 0 for each; the scenario's twelve steps each assert their stated outcome; the username-absence assertion runs over every MCP result of the run; the outbound guard records zero calls.
- **Test levels:** end-to-end through in-process servers and headless Chrome. This is the one spec whose purpose is to cross every seam.

---

## Integration Points

- SPEC-017 documents the loop this test proves.
- Produces **Change:** the end-to-end check.
- **Provenance tag:** `INIT-008/SPEC-016`

## Notes (executor, 2026-10-12)

- **The clip field in steps 2 and 7 is `sentenceAudioPath`**, not a bare `audioPath`. The `audio-stale` rule pairs a word's `word` with `audioPath` and its `sentence` with `sentenceAudioPath`, so the edited sentence is stale against `sentenceAudioPath`. The test sets that field in step 2.
- **The first issue list holds one hard error.** The lesson's `newWords` holds the lexicon's pointing (`בַּיִת`) while the second introduction word still has the drifted pointing, so `meaning` (a block) names `newWords/1` until the `lexicon-form` fix lands. The test asserts it is the only hard finding and that it clears with the fix, so the "hard errors and warnings in one list" case is covered.
- **The left-over warning is a `field-language` finding** (Hebrew letters in a transliteration), and it stays through the publish.
- **The restore by the admin route restores the AI's earlier sentence**, from the revision before the person's edit (the publish revision), which is the one-step "put the earlier value back" the operator described. The MCP restore brings the person's `meaning` back, as the spec lists.
- **The test environment sets the five provider-key names to the empty string** instead of deleting them. `server/audio.js` reads a local `.env` file into any variable that is `undefined` the first time it loads, so a deleted key would reappear from a developer's `.env`; an empty value counts as unset everywhere and the outbound guard still records nothing. (A first draft of the test saw this when its own check found a key set; the check now never prints a value.)
- **Chrome smoke.** The additions sit in `test/chrome-plan-editor-smoke.js`: a plan made by the registered MCP handlers (not through the SDK client), published, edited by a person in the browser, re-read by the AI, restored from the published-to-working diff, and the AI writes again with `overwrote`. The layout and contrast loop now includes that plan's page, and starts each width on the sample plan explicitly instead of reloading whichever page was last open. `test/design-tokens.test.js` pairs quote characters across the whole smoke file by regular expression and compares the class names it finds with a snapshot; the new code avoids `.block` and `.field` property access (it uses `['block']`) so that scan is unchanged.
- **No defect found** in an earlier spec by this test. The three findings above are about the test's own fixtures.
