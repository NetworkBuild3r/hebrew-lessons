# SPEC-016 completion summary

**Spec:** SPEC-016 · Testing: end-to-end AI-first check  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (tests written and passing; `security_review: standard`, no independent reviewer has run)  
**Provenance:** `INIT-008/SPEC-016`

## What shipped

- **`test/plan-ai-first-e2e.test.js`** (17 tests, `node --test`, part of `npm test`). One in-process web server and one real `McpServer` and `Client` joined with `InMemoryTransport`, on a temporary database through `test/helpers/signed-in-server.js`. The AI's steps are MCP tool calls through the client (so the SDK checks each argument); the person's steps are admin HTTP routes with a signed-in `example-admin` session; a learner (the harness's `learner` account) reads over HTTP. A guard replaces `fetch`, `http.request`, `https.request`, and `https.get`, records any call to a non-local host and throws, and is itself tested with a remote host at the end. The five provider-key names are set to the empty string for the run, and the test asserts each stayed empty.

| Test | What it proves |
|------|----------------|
| step 1 | `create_plan` (`example-plan`, recipe `topic-conversation`) makes a draft at revision 1 with a hidden lesson. A learner and an admin both get 404 from `GET /api/plans/:id/published`. `list_plans` shows it (draft, revision 1, not published, `humanEditedFields` 0). |
| step 2 | The AI builds with `insert_block` (twice), `remove_block` (the placeholder), `move_block`, `replace_part`, and eight `set_field` calls (meaning, sentence, clip path, lesson `newWords`, `buildsOn`, `recycles`, `visibility`, and a transliteration). One pointed word is not NFC; the second differs from a seeded **verified** lexicon entry with the same stripped key. |
| step 3 | `list_plan_issues` returns `pointing-form` and `lexicon-form` as `warn` (not hard) with id paths and `fix.suggested`, plus a `field-language` warning. The only hard finding is `meaning` on `newWords/1` (a consequence of the drift). One `set_field` per fix clears both codes and the hard finding; the `field-language` warning stays on purpose. |
| step 4 | `publish_plan` has only `id` and `expectedRevision`. It publishes with a warning present. The learner then reads the snapshot (`visibility: everyone`, revision equal to the publish revision). This is the accepted D1 consequence, asserted. |
| step 5 | Two `PATCH /api/plans/:id/fields` calls as `example-admin` (meaning and sentence). The plan is a draft at revisions 18 and 19 in the run. The learner still reads the published snapshot with the AI's text. `list_plans` shows `unpublishedChanges` and `humanEditedFields` 2. |
| step 6 | MCP `read_part` on the word: `editState.humanEdited` lists `meaning` and `sentence`, each with `previousValue` equal to the AI's text, `previousWriter: 'ai'`, and the person's revision; `lastHumanEdit` is the later one; no entry has `by`; the text holds no `example-admin`. |
| step 7 | `list_plan_issues` reports one `audio-stale` for the sentence, `warn`, with the clip field and the note. |
| step 8 | The AI's `set_field` on the person-written meaning succeeds, returns `overwrote` with the person's value and revision, and the field leaves `humanEdited` (the sentence stays). |
| step 9 | `list_plan_revisions` equals, row for row, the list of writes the test made (op and actor kind, revisions 1 to N). The MCP sees `human` and `mcp` as names; the admin route shows `example-admin` on the same rows. `get_plan_revision` holds the person's value at that revision. `diff_plan` (published to working) lists `meaning` and `sentence` with id paths and equals the admin route's answer; a one-revision diff lists `meaning` and `status`. |
| step 10 | The MCP restores the person's meaning from the person's revision; the admin route restores the AI's earlier sentence from the publish revision. Each makes exactly one new revision (`restoredFrom` set), and the document deep-equals the one before apart from that field and the revision. History shows both `restore` rows with the right actor kinds. |
| step 11 | `assign_plan` to `example-learner` and back to shared; `set_field` of `audience` and `assigneeId` are `assign_required`; a `replace_part` with another id is `id_mismatch`; a stale `expectedRevision` is `revision_conflict` with the stored revision. The refused calls change nothing (revision, row count, audience). |
| step 12 | `plan_revisions` rows, `plan.*` audit rows, and the test's count of successful writes are equal (24). Audit actions and actors match the writes; audit detail keys stay inside `revision`, `path`, `field`, `tool`, `requestId` and hold none of eleven known values; revision summaries hold none either. `list_lessons`, `list_activity_types`, `upsert_lesson` (`audio: false`, into the temporary lessons folder), and `get_lesson` work. `set_lexicon_verified` and `set_ai_planning` answer `forbidden`; no tool name mentions approval. |
| no refusal (behaviour) | The error codes met in the whole MCP run are exactly `assign_required`, `forbidden`, `id_mismatch`, `revision_conflict`; none names a mark, a person, a pin, or a lock. |
| no refusal (search) | The plan modules, plan repos, migration 006, the plan routes, every error code those files can return, the three tables' columns, and every plan tool's name, description, and input schema are free of `field_pinned`, `keepPinned`, `pinned_by`, `pinned_at`, `pinned`/`locked` keys and columns, and person-or-mark error codes. |
| parity | All eight new tools were called in the run. `findParityProblems` over `test/helpers/plan-parity-table.js` (17 rows) is empty against the tools a real client lists and the routes a real router registers; the human-only acts keep their routes. |
| no username, no network | A transcript of more than 40 MCP results holds no `example-admin` and no token. The outbound guard recorded zero calls (and records a probe). |
| learner and cookies | A learner gets 403 from every plan list, read, diff, issues, schema, and write route, and cannot create a plan. On the MCP port a learner cookie and an admin cookie are 401, a wrong bearer and no credentials are 401, and the bearer token works. The web server has no `/mcp`. The refused calls wrote nothing. |

- **`test/chrome-plan-editor-smoke.js`** gained one end-to-end section and a layout pass: a plan made by the registered MCP handlers (`create_plan`, `insert_block`, `remove_block`, `set_field`, `publish_plan`, `list_plan_issues`) and published; the Plans list shows it as published; a person opens it, fixes the meaning and the sentence in the editor (two revisions); the AI's `read_part` shows both as `editState` with `ai` as the earlier writer and no name; History shows `human:set_field` twice and the AI's `create` and `publish`; the Issues panel lists `field-language` and `audio-stale` as warnings and no hard error; a mark shows on each person-written field; **Compare** (published to working) then **Restore this value** asks once, sends nothing before the confirmation, and adds one `human:restore` revision with the AI's meaning back and the sentence untouched; the AI writes again and gets `overwrote`; the learner route still serves the published revision. The 360 and 1280 px contrast, target, name, and overflow audit now also covers that plan's page.

## Tests and checks run

| Command | Result |
|---------|--------|
| `node --test test/plan-ai-first-e2e.test.js` | 17 of 17 pass (about 2 s) |
| `node test/chrome-plan-editor-smoke.js` | PASS (about 29 s) |
| `npm test` with `SRS_SCHEDULER=ladder` | **1460 tests, 1460 pass, 0 fail** (1443 before; the 17 new are this spec's) |
| all 20 `test/chrome-*-smoke.js` | all exit 0, including `chrome-admin-lessons-smoke.js` (the script that failed once in SPEC-013's run passed on its first run here) |
| `npm run audio:verify` | OK, 65 recordings (against `data/audio-inventory/20261011T071128.json`) |

The real database was never opened or migrated. No `.mp3` and no `worksheets/*/index.html` was touched (`git status` shows no change under `worksheets/`).

## Findings, and which spec owns each

No defect was found in SPEC-001 to SPEC-015 by this test.

1. **`server/audio.js` loads a local `.env` into any variable that is `undefined`.** A test that deletes `ELEVENLABS_API_KEY` sees it come back the first time that module loads. The test sets the key names to the empty string, which every reader in `server/` treats as unset. This is not a defect (it is the documented local behaviour); it is why the "no provider key" setup differs from the spec's wording. It also means a test that only deletes the keys is not sealed against a developer's `.env` (the outbound guard is the real protection). A first draft of this test printed a key value in an assertion message; the assertions now compare to the empty string and never print a value. Nothing was written to a file.
2. **`test/design-tokens.test.js` scans `test/chrome-*.js` by regular expression for class names, pairing quote characters across the whole file.** A file with an unbalanced apostrophe (the smoke's header comment has one at line 15) pairs its strings off by one, so ordinary code such as `x.block` or `x.field` can be read as a class selector and fail the snapshot. Added code in the smoke avoids those two property names (`['block']`, `['field']`) and apostrophes in comments. Owner: SPEC-011 / INIT-005's design-token test. A follow-up (strip comments and parse strings properly) would remove the trap; not done here (out of scope).
3. **`test/chrome-plan-editor-smoke.js` reloaded whichever page was last open before the layout audit.** It was the sample plan only by accident of ordering. The loop now navigates to the sample plan at each width. Owner: SPEC-011 to SPEC-015 smoke; fixed in this change.

## Deviations from the spec text

- **`sentenceAudioPath`, not `audioPath`**, is the clip field set in step 2 and reported stale in step 7 (the rule pairs `sentence` with `sentenceAudioPath`).
- **The first issue list holds one hard finding** (`meaning` on `newWords/1`, because the lexicon's pointing and the word's drifted pointing do not match until the fix). The test asserts it and that the fix clears it.
- **The Chrome smoke uses the registered handlers, not the SDK client,** for the AI's steps: a browser test has no use for an MCP transport, and the node test already covers the SDK path.
- **The second restore is the admin's, from the AI's earlier sentence,** so the human restores "the earlier value" as the operator described; the spec's wording ("the same restore on a second field") is met by a second field and the same one-revision result.
- **Reduced motion** is checked by `test/chrome-restyle-a11y-smoke.js` (which passes), not by the plan editor smoke; the smoke checks contrast, target size, names, and overflow.

## Not closed

1. **No independent review** (`security_review: standard`).
2. The outbound guard cannot see a raw `net` or `tls` socket (a library that bypasses `http` and `fetch`). Nothing in the plan or MCP code opens one; the check is a tripwire for the usual paths.
3. The Chrome smoke does not run a real SDK client; the in-memory client run covers the SDK's argument validation.

## Files

New: `test/plan-ai-first-e2e.test.js`, this summary.  
Edited: `test/chrome-plan-editor-smoke.js`, `SPEC-016-testing-end-to-end-ai-first-check.md` (status `in_review`, boxes ticked, notes).  
No commit. No product code changed.

## Provenance

`INIT-008/SPEC-016`
