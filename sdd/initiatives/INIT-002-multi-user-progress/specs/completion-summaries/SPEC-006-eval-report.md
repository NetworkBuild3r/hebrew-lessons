# SPEC-006 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/visibility.test.js test/content-filter.test.js`. Confirmed `seedVisibility`, the `mcp` actor, and `JSON.stringify` with `<` escaped in the filter.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (9 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | A user without a grant does not see a restricted lesson's slug, title, or unique words in any of the four scripts. |
| ac-2 | PASS | A granted user and an admin see the selected lesson. The admin also sees the hidden lesson. |
| ac-3 | PASS | A word shared with a visible lesson stays, and its lesson list contains only the visible lesson. |
| ac-4 | PASS | Filtered output parses again as data. `</script>` and quotes are escaped. |
| ac-5 | PASS | A malformed catalog becomes an empty dataset. The test log shows "fail closed". |
| ac-6 | PASS | A non-admin is refused. A bad slug is refused. An admin and the `mcp` actor succeed, and each write an audit row. |
| ac-7 | PASS | The first seed marks the three lessons already on disk, including `anal-sex-vocab`, as everyone. A second run changes nothing. A lesson added later stays hidden. |

No remediation loop.
