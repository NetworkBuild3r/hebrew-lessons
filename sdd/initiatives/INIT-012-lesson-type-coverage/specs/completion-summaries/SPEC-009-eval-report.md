# SPEC-009 eval report

**Spec:** `INIT-012/SPEC-009` (`SPEC-009-testing-five-lesson-types.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test runs (blocking checks)

```text
node --test test/plan-lesson-types-matrix.test.js test/plan-lesson-types-mcp.test.js test/plan-revisions-migration.test.js
-> tests 19 | pass 19 | fail 0 | exit 0

node test/chrome-lesson-types-smoke.js
-> chrome-lesson-types-smoke: PASS (exit 0)
```

## Per-AC verdicts (Loop 3 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-lesson-types-matrix.test.js:118-147` and `test/plan-lesson-types-mcp.test.js:77-146` — each of the five sample lessons (A–E) validates with zero blocking findings and publishes in a `1.2.0` plan (`test/fixtures/lesson-types/five-lesson-types.plan.json:2`). |
| ac-2 | PASS | `test/plan-lesson-types-mcp.test.js:148-159` — MCP build logs all tool calls (`upsert_lexicon_entries`, `create_plan`, `insert_block`, `remove_block`, `set_field`, `insert_lesson`, `list_plan_issues`, `publish_plan`) and confirms none is a new tool. |
| ac-3 | PASS | `test/chrome-lesson-types-smoke.js:188-198,249-348` — Type B lesson built by hand on `/editor/` with no JSON toggle and no `id` shown, validates with 0 blockers, and publishes. |
| ac-4 | PASS | `test/plan-lesson-types-matrix.test.js:149-177` — matrix guard fails when a lesson type in §4 has no fixture or a block has no renderer (`test/fixtures/lesson-types/fixture-lib.js:12-21`). |
| ac-5 | PASS | `test/chrome-lesson-types-smoke.js:364-396,499-503` — in real Chrome as a learner at 390px and 1280px, in English and Hebrew, one item of each exercise kind (`choose`, `true-false`, `unscramble`, `translate`, `dictation` via real CDP mouse click) is answered with retry and reveal. |
| ac-6 | PASS | `test/chrome-lesson-types-smoke.js:398-408` — assignment keeps typed text on the page, updates sentence count, and sends zero network requests (`assertLearnerNetworkQuiet`). |
| ac-7 | PASS | `test/chrome-lesson-types-smoke.js:411-435` — future table renders 9 rows, shows `not yet checked`, and unverified future forms have zero enabled drill inputs and are not scored. |
| ac-8 | PASS | `test/plan-lesson-types-matrix.test.js:368-413` and `test/chrome-lesson-types-smoke.js:203-213,305-335,437-472` — `<img src=x onerror=alert(1)>` shown as text in editor, preview, and player across all new and lesson block surfaces. |
| ac-9 | PASS | `test/chrome-lesson-types-smoke.js:199-200,361-362,499-503` — at 390px nothing scrolls sideways on `/editor/` or `/shared/plan/`. |
| ac-10 | PASS | [SPEC-009-not-held.md](SPEC-009-not-held.md) lines 1–19 lists every activity a lesson type still cannot hold. |
| ac-11 | PASS | `test/plan-lesson-types-matrix.test.js:415-425` and [SPEC-009-not-held.md](SPEC-009-not-held.md) lines 21–91 — full suite has 43 failures (down from 51 pre-SPEC-001 baseline), all named, with zero failures in any `INIT-012` or touched test file. |
| ac-12 | PASS | `test/plan-lesson-types-matrix.test.js:353-366`, `test/plan-lesson-types-mcp.test.js:87-90`, and [SPEC-009-forms-to-check.md](SPEC-009-forms-to-check.md) lines 7–17 — all 9 future forms of `לְדַבֵּר` listed in pure pointed Hebrew beside the dictionary's past forms, with `verified = 0`. |

## Remediation-loop log

- **Loop 1:** `ac-1`, `ac-4`–`ac-7`, `ac-10`, `ac-12` PASS; `ac-2` (tool list not printed to test log), `ac-3`/`ac-9` (390px and no-JSON not checked on `/editor/` hand build), `ac-8` (XSS not tested across all surfaces), `ac-11` (`migration 010` missing `down()` in `test/rollout-preflight.test.js:631`, and pre-existing failure list not written) FAIL.
- **Remediation 1:** Fix sub-agent added `console.log`/`t.diagnostic` tool logging in `test/plan-lesson-types-mcp.test.js`, 390px/no-JSON/network-quiet checks in `test/chrome-lesson-types-smoke.js`, multi-surface XSS checks in `test/plan-lesson-types-matrix.test.js`, `down()` in `server/db/migrations/010-lexicon-future-cells.js` (so `test/rollout-preflight.test.js` passes 20/20 and `test/restyle-gate.test.js` passes 26/26).
- **Loop 2:** `ac-1`–`ac-10` PASS; `ac-12` FAIL (two Latin characters `i`/`na` in `SPEC-009-forms-to-check.md` rows `2fs` and `2fp`), `ac-7` (Chrome drill input check), and `ac-11` (`test/plan-revisions-migration.test.js` migration count 9->10 and written pre-existing failure list) FAIL.
- **Remediation 2:** Fix sub-agent fixed `SPEC-009-forms-to-check.md` (`תְּדַבְּרִי`, `תְּדַבֵּרְנָה`), added `ac-8`, `ac-11`, and `ac-12` automated tests in `test/plan-lesson-types-matrix.test.js`, added future drill non-scoring and player scratch XSS checks in `test/chrome-lesson-types-smoke.js`, updated `test/plan-revisions-migration.test.js` (10/10 pass), and documented all 43 pre-existing failures in `SPEC-009-not-held.md`.
- **Loop 3 (fresh independent verifier):** all-PASS (`9/9` in matrix+MCP, `10/10` in revisions-migration, `chrome-lesson-types-smoke: PASS`).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=testing` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
