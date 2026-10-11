# SPEC-007 Completion Summary — ADR-0005 stack and architecture

**Traceability:** INIT-003/SPEC-007
**Status:** Complete (ADR written; spec `status: complete`)
**Domain:** docs
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `docs/decisions/ADR-0005-stack-and-architecture.md`
- [x] SPEC-007 yaml `status: complete`

## Acceptance Criteria

- [x] ac-1: Decision §§1–8. §9 records the carried constraints the spec's ac-4 also requires.
- [x] ac-2: Options A–E are rows in the scored table. Each score has a sentence under "Why each score".
- [x] ac-3: A, B, and C do not move the server or the database. D and E each have a carry table for accounts, progress, the 65 clips, the three lessons, the MCP, the tests, and the Deployment. Neither table deletes or overwrites a recording without an archive.
- [x] ac-4: §9 marks each ADR-0003 constraint kept, changed, or superseded. The script-tag row is superseded for the plan when the API serves it, and kept for legacy worksheets.
- [x] ac-5: §5 names `server/plans/ops.js` as the only mutation logic, with one function per ADR-0003 operation, called by `mcp/tools.js` and by `server/api/plan-routes.js`.
- [x] ac-6: §7 lists authoring, MCP, admin editor, gating, visibility, and the Chrome scripts, and says how each stays green. Decision 3 has the port-or-drop tables for D and E.
- [x] ac-7: No secret values, learner names, or transcript text. Framework pages are cited with access date 2026-10-08.

## AC Verification (independent)

The spec assigns the pass check to a human who did not write the ADR: pick two criteria, re-score two options, and walk the migration table. That review was not run in this session. A mechanized check confirmed options A–E in the score table, the weighted totals (A 457, B 459, C 439, D 295, E 378), and that the tree counts match commit `2f1f0b0` (`HEAD` = `origin/main`).

Overall: **not independently verified** · remediation loops: 0.

**Verification tier:** `full` (security sensitivity is standard; the ADR states auth consequences) · **pre-flight:** gate_actions `[]` · security_review=`standard` · domain=`docs` · autonomy_level=`plan_only` · **checks skipped:** test-suite re-run — the spec names no test file.

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (author) | ADR-0005 Decision §§1–8 |
| ac-2 | PASS (author) | Score table and the five "Why each score" paragraphs |
| ac-3 | PASS (author) | Decision 3 carry tables for D and E; archive rule stated |
| ac-4 | PASS (author) | §9 constraint table |
| ac-5 | PASS (author) | §5 operations table |
| ac-6 | PASS (author) | §7 surfaces table; Decision 3 port tables |
| ac-7 | PASS (author) | Author scan; citations are vendor URLs and env-var names |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-007`
- **Title:** Keep Node, add one plan-operations module and a hand-built design system
- **Files changed:** 3 · ADR-0005, this summary, spec status
- **Change status:** written on branch `lesson-system-foundations`
- **Reversibility:** revertible (delete ADR-0005 and this summary; set spec status back to `ready`)

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| —         | 0         | 0     |

**All tests passing:** N/A (docs) · **Coverage (new code):** N/A

---

## Gates & Controls

`gate_actions: []`. aud-2 through aud-4 were already resolved by the operator on 2026-10-08 ("go"): one maintainer with agents writing most code, the existing one-replica Docker layout, and weights proposed in the ADR. The letter is **A**, status **Proposed**. The spec's blocking check still asks the operator to confirm that letter before SPEC-009. No commit, push, or pull request.

---

## Security Review

- **Sensitivity / depth:** standard · full enough for a docs record that fixes runtime and auth consequences · **method:** inline review of the ADR against the spec's security acceptance criteria (`self_reviewed: true` — documentation only; no executable code)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

Accepted residuals are written in ADR-0005 "Residual risks": string HTML can drop an escape; `node:sqlite` stays experimental; CSP is still the ADR-0002 follow-up; tokens replace hex values gradually.

- **Positive observations:** No new direct dependency. Plan writes use the existing session and CSRF checks. Migration steps do not serve the site without sign-in. Media stays outside served roots. A Phoenix or Next port is not taken, so the session stack is not reimplemented under time pressure.

---

## Files Changed

Provenance is `INIT-003/SPEC-007` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `docs/decisions/ADR-0005-stack-and-architecture.md` | Created — ADR-0005 | `INIT-003/SPEC-007` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-007-docs-adr-0005-stack-and-architecture.md` | Modified — status `complete`; checkboxes marked | `INIT-003/SPEC-007` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-007-completion-summary.md` | Created — this summary | `INIT-003/SPEC-007` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | trace and timer scripts | This session was an orchestrator-dispatched worker; timer and `open_trace` stay with the orchestrator | Run-trace and timed effort | Effort is an estimate only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/implementation-planner/AGENT.md` is not in this repo. Disclosed in the spec. The ADR follows the section order of ADR-0001 and ADR-0002.
- **Harness:** `cursor`.
- **Worktree / branch:** shared checkout on `lesson-system-foundations`. No commit.
- **AC verifier:** not a second person. Score-table and tree-count check only.

---

## Deviations from the Spec

- The architecture-decision-record template path named in the spec is not present in this repo. Section order follows ADR-0001 and ADR-0002 (Metadata, Summary, Context, Options, Decision, Consequences, Implementation plan, Security considerations, References, Changelog).
- B's weighted total is 2 points above A's. Decision 1's close-total rule (a lead smaller than the lightest weight does not separate the options) plus the lower migration cost selects A. The raw totals are in the table so a reviewer can move one score and flip the letter.
- An independent reviewer did not re-score two criteria.

---

## Effort Tracking

**Agent session time:** not timed · one documentation session
**Spec effort estimate:** 5 hours

## Facts re-checked

Commit `2f1f0b0ad87ad940ed8593af6d4a6e5c05554f58` is `HEAD` and `origin/main`. `shared/**/*.js` is 30 files and 9,409 lines. `shared/worksheet.css` is 1,472 lines, 158 hex colors, one `@media (min-width: 800px)`. `test/*.test.js` is 44. `test/chrome-*-smoke.js` is 10. `.mp3` files outside `node_modules` are 65. Lesson directories are `my-morning`, `anal-sex-vocab`, and `shai-date-indirect-speech`.
