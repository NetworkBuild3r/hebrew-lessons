# SPEC-007 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/authoring.test.js test/mcp-assignment.test.js` (12 pass, 0 fail) and `npm run audio:verify` (65 recordings, exit 0). Read `mcp/tools.js` `requireMcpAuthToken`, `resolveActiveUserIds`, and the `upsert_lesson` / `list_users` / `set_lesson_visibility` handlers.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (12 pass, 0 fail; audio verify OK)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | `save` bytes match `upsertLesson` for `worksheet.js` and the catalog entry. |
| ac-2 | PASS | A validator error writes nothing; prior bytes stay the same. |
| ac-3 | PASS | A sentence change archives the old clip. A forced verify failure is returned as an error. The test log shows the archive lines. |
| ac-4 | PASS | `audioPlan` writes nothing and its counts match the following fake-TTS `generateAudio`. |
| ac-5 | PASS | Bad and reserved slugs are refused. `</script>`, a backtick, and `${` round-trip as data. |
| ac-6 | PASS | A newly created lesson is hidden. |
| ac-7 | PASS | An existing `index.html` hash is unchanged after `save`. |
| ac-8 | PASS | `{ users: ["alice"] }` is visible to alice only. `"everyone"` is visible to both. Omitting visibility on a new lesson stays hidden and the reply says so. Omitting it on an existing lesson leaves visibility unchanged. |
| ac-9 | PASS | `set_lesson_visibility` changes access, audits actor `mcp`, and an unknown username changes nothing. |
| ac-10 | PASS | `list_users` maps active accounts to `username` and `displayName` only. |
| ac-11 | PASS | With `MCP_AUTH_TOKEN` unset, `list_users`, `set_lesson_visibility`, and `upsert_lesson` with `visibility` refuse. The visibility case writes no lesson. Plain `upsert_lesson` still writes. |

`requireMcpAuthToken()` and `parseVisibilityArg()` (which resolves usernames) both run before `store.upsertLesson`. No remediation loop.
