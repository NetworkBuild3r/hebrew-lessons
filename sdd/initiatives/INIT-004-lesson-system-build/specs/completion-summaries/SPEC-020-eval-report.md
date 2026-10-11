# SPEC-020 AC Verification Eval Report — Migration and rollout

**Traceability:** INIT-004/SPEC-020 \
**Verifier:** four independent sub-agents in turn, none of which wrote the work: [Verify SPEC-020 acceptance criteria](0ee9c83b-9914-43a0-9c55-98c1b23503a7), [Re-verify SPEC-020 acceptance criteria](03d368dd-53e3-4a39-9878-fd32a5632256), [Final verify SPEC-020 criteria](e2c77028-2725-4886-ad76-e3e0671bf19e), and [Final check SPEC-020 tree](62b42096-817e-4c7b-86ab-04949a9ac007) · Cursor `generalPurpose` Task on the inherited session model · temperature 0 by prompt · read-only by prompt (the Task tool has no `readonly` flag) \
**Evaluated at:** 2026-10-09T23:00:00Z (final pass) \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence. Every pass returned PASS on every item. The table cites the final pass on the final tree.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Each §14 step has a rollback sentence; none deletes a recording without an archive | PASS | Checklist steps table, seven rows, each with a rollback cell; the rules say no `.mp3` is deleted, moved, or overwritten, and the rollback commands section says none of them reads or writes a recording. Test `the checklist gives every section 14 step a rollback, and no rollback deletes a recording` passes. |
| ac-2 | `audio:verify` against the pre-rollout inventory exits 0; 65 recordings; same relative paths | PASS | `npm run audio:verify -- --against data/audio-inventory/20261009T144848.json` → `audio:verify OK — 65 recordings accounted for`, exit 0. Preflight: `audio: ok inventory lists 65 recordings; 65 on disk; same relative paths`. The 65 inventory paths equal the 65 `.mp3` files tracked at `origin/main`. |
| ac-3 | A dry run (no link change) leaves `worksheets/*/index.html` unmodified and the `lesson_progress` count unchanged | PASS | All four tracked `worksheets/*/index.html`: `git hash-object` equals the blob at `HEAD` and at `origin/main`. Two verifiers reproduced the dry run on their own read-only `VACUUM INTO` copies (counts → check → import → check PASS → the checklist's rollback commands → check); `lesson_progress` stayed 1 at every stage. Live `data/hebrew.sqlite` sha256 unchanged; still schema version 1. Test `a dry run (counts, then check) leaves worksheets, recordings, progress rows, and the schema as they were` passes. |
| ac-4 | Test disposition matches §14: current tests kept, new tests beside them, none retired | PASS | The checklist's disposition table has the same four rows as initiative §14; Retired is "None in this initiative." `git diff --name-only --diff-filter=D origin/main -- test/` is empty. The nine edited and 22 new test files equal the ones the table names. `npm test` script unchanged. |
| und-1 | Default cutover is none | PASS | Checklist opening paragraph; both `lessonHref` functions unchanged; run log `link change: none`. |
| und-2 | No worksheet file deleted | PASS | Step 7 row "Not done in this initiative"; no deletion under `worksheets/` in git. |
| sec-1 | Preflight output has no snapshot body and no key | PASS | The plan query selects id, status, revision, and a has-snapshot flag; visibility prints mode names only and discards parse errors; child processes get no `*KEY`/`*TOKEN`/`*SECRET`/`*PASSWORD` variable. Canary tests pass, including an unparseable snapshot. |
| sec-2 | A lesson link does not change without the gate | PASS | `shared/today-page.js` and `shared/lessons-page.js` `lessonHref` return `'/worksheets/' + encodeURIComponent(String(slug \|\| '')) + '/'`, identical to `HEAD`. Test `no learner link opens the plan screen yet; changing that needs the written deploy gate` passes. |
| tech-1 | Prints counts and paths only | PASS | Live and fixture output: counts, statuses, mode names, paths. Paths outside the root under the home folder print as `~/...`. |
| tech-2 | Opens no socket | PASS | No network module is required; every fixture run passes under a `NODE_OPTIONS` guard that throws on any socket call, and the guard proves itself in its own test. |
| del-1 | `docs/operations/lesson-plan-rollout.md` | PASS | Exists; copies §14 with rollbacks. |
| del-2 | Preflight exits non-zero on a failed verify, a changed worksheet hash, or missing legacy plan rows | PASS | Fixture tests for each; live run exits 1 with `FAIL (plans)`. |
| del-3 | Tests on a temporary fixture, not the live database | PASS | Fixtures under the OS temp folder with their own git repo and database; live `data/` unchanged across full runs. |

## Definition of done

- [x] Checklist, preflight, and tests exist and pass — `node --test test/rollout-preflight.test.js` 20/20; `npm test` 573/573.
- [x] `audio:verify` against the inventory from the start reports the same 65 paths — verified in every pass.
- [x] No learner link changed — `lessonHref` unchanged; tripwire test passes.
- [x] No live cluster action and no worksheet deletion — none performed.

## Tests run by the verifiers

```text
node --test test/rollout-preflight.test.js
→ 20 pass / 0 fail ; exit 0   (final tree; earlier passes 15/15 and 20/20)

npm test
→ 573 pass / 0 fail ; exit 0  (rounds two and three)

npm run audio:verify -- --against data/audio-inventory/20261009T144848.json
→ exit 0 ; 65 recordings accounted for

node scripts/rollout-preflight.js check --inventory data/audio-inventory/20261009T144848.json --ref origin/main
→ exit 1 ; FAIL (plans) only, as the checklist expects before step 2 on this database
```

No coverage tool is configured in this repository.

---

## Remediation log (feedback loop)

No acceptance criterion failed in any pass, so there was no AC remediation loop. Each later pass re-verified the tree after changes made for security-review findings:

| Round | Failing AC(s) | Change between passes (INIT-004/SPEC-020) | Re-verify result |
| ---- | ------------- | ------------------------------- | ---------------- |
| 1 | none | — (first pass) | PASS |
| 2 | none | Visibility check; child-environment filter by name pattern; `--ref`; checklist backup layout, token loading, and rollback wording | PASS |
| 3 | none | Split down commands with version guards; test runner on copies of `server/db`; unparseable-snapshot test; in-pod re-check before step 2 | PASS |
| final | none | Post-import visibility rule and order; operator guide and skill sentences; step 4 order; `~/` paths outside the root | PASS |

---

## Notes

- The SDD config names `composer-2.5-fast` as the Cursor verifier model. This harness lets a sub-agent use a different model only when the user names one, so every pass ran on the inherited session model.
- Verifiers wrote throwaway helper scripts and database copies under the OS temp folder and deleted them. Their imports on those copies wrote new inventory files under `data/audio-inventory/` (gitignored). One verifier deleted the file its own run wrote after confirming it duplicated the pre-rollout inventory.
- Concerns below the FAIL bar, carried into the completion summary: the live database file was written at 21:02:28Z during this spec (explained there); the plan route's visibility source (a SPEC-004 follow-up); the link tripwire covers the two `lessonHref` builders only; the preflight's count check means "not fewer"; nothing is committed.
- No AC was "passed" by weakening a test or the criterion.
