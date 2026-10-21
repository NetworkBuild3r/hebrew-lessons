# SPEC-019 Completion Summary — Operator guide: media and backup

**Traceability:** INIT-004/SPEC-019
**Status:** Complete
**Domain:** docs
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Updated `docs/operations/accounts-and-admin.md` with a Pictures section: upload, admin-only approval with alt text, dry-run, confirm, the 15-per-hour cap, the $1 UTC-day cap, and archive-before-delete.
- [x] Tests: none (documentation). The existing `test/media-backup.test.js` still scans this file for a non-empty `OPENAI_API_KEY` assignment.

## Acceptance Criteria

- [x] ac-1: the guide documents upload, approval, dry-run, confirm, the caps, and delete-archives-first. The sentence "The AI and MCP tools cannot read progress or answer history." was not edited.
- [x] ac-2: the guide states that `DATA_DIR/media` and `media/_archive` are backed up with the database, that restore stops writers first, and that an approved image is checked after both trees are restored.
- [x] ac-3: a search of the file finds no key assignment and no learner transcript. `OPENAI_API_KEY` is named only.

## AC Verification (independent)

Verified by a fresh sub-agent that did not author the guide: [Verify SPEC-019 criteria](39952405-07fa-4cd1-b4b6-46acc4241f6e).
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-019-eval-report.md`](SPEC-019-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not declare a light-tier pass, so the tier is full · gate_actions=`[]` `pass` · security_review=`standard` `FAIL` · domain=`docs` `PASS` · autonomy_level=`plan_only` `PASS` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | Pictures section `docs/operations/accounts-and-admin.md:88-125`. Privacy sentence `:173` unchanged (`git blame` `bbfb307`, 2026-10-08). Hebrew-worksheet skill not edited. |
| ac-2 | PASS | Backup set `:64`. Restore stops writers, restores both trees, then checks an approved image `:72`, `:86`, `:90`. |
| ac-3 | PASS | `rg` found no `sk-`, no `ELEVENLABS_API_KEY=`, and no `OPENAI_API_KEY=` assignment. `node --test test/media-backup.test.js` → 6 pass, 0 fail. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-019`
- **Title:** Operator guide for picture upload, approval, generation caps, and archive-before-delete
- **Files changed:** 1 guide, plus this summary and the eval report
- **Change status:** proposed (not committed)
- **Reversibility:** revertible

The working-tree diff of `docs/operations/accounts-and-admin.md` against HEAD is larger than this spec. `INIT-004/SPEC-017` had already written the backup-directory sentences, the `OPENAI_API_KEY` name in the env table, and the first-run bootstrap text, and those edits were still uncommitted. This spec added the Pictures section, the three MCP picture tools, and the provenance tag. It did not rewrite the account, sign-in, or audio sections, and it did not edit `.cursor/skills/hebrew-worksheet/SKILL.md`.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| none (documentation) | 0 | — |

**All tests passing:** N/A for a new suite. Existing guard: `node --test test/media-backup.test.js` → 6 pass, 0 fail, exit 0, including the scan that rejects a non-empty `OPENAI_API_KEY` assignment in this file. **Coverage (new code):** N/A.

---

## Gates & Controls

None — branch/reversible only. `gate_actions` is empty. No merge, deploy, secret change, or database migration. Commit, push, and pull request were not performed (dispatch said to skip them).

Tier-5 run-trace invariants and model-dispatch conformance were not run in this session. The orchestrator owns the trace sentinel for a dispatched spec. See Trace Emission Degradation.

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** implementer review of the guide diff (`self_reviewed: true`), plus the independent verifier's secret search and `test/media-backup.test.js`
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None
- **SAST / secret scan:** not run — no SAST scanner is configured. The mechanized check that is configured for this file is the non-empty `OPENAI_API_KEY` assignment scan in `test/media-backup.test.js` (pass).

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** The guide names `OPENAI_API_KEY` and does not assign a value. No new command prints a secret. Approval is described as admin-only, with empty alt text refused, and MCP cannot approve. Delete is admin-only and copies to `DATA_DIR/media/_archive` before the live file is removed. Generation requires `confirm` and states the 15-per-hour and $1 daily caps. Learner free text is named as something that is never the prompt; no transcript is quoted.

---

## Files Changed

Provenance is `INIT-004/SPEC-019` for every row below (the spec that authored the change).

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `docs/operations/accounts-and-admin.md` | Modified — Pictures section (upload, approval, dry-run, confirm, caps, archive-before-delete); MCP picture tools; provenance tag | `INIT-004/SPEC-019` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-019-docs-operator-guide.md` | Modified — status `complete`, criteria checked | `INIT-004/SPEC-019` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-019-completion-summary.md` | Created | `INIT-004/SPEC-019` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-019-eval-report.md` | Created | `INIT-004/SPEC-019` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `trace_complete` | Dispatched sub-agent. The orchestrator owns the sentinel (`execute-spec` Phase 0 step 2). This agent did not call `open_trace` or `close_trace`. | This session's own sentinel pair and `task_step` events | The orchestrator emits on return. A Tier-5 check against a missing file must not be treated as a pass. |

---

## Deviations from the Spec

- `.claude/agents/implementation-planner/AGENT.md` is not in this repo (the spec already says so). No `scripts/dev/expertise_state.py` dispatch-check was available here. The guide was written as operator documentation that follows ADR-0004 §4 through §9 and the routes in `INIT-004/SPEC-011` / `INIT-004/SPEC-012`. No `AGENT.md` was invented.
- Commit, push, and pull request were skipped because the dispatch said to skip them.
- The backup sentences ac-2 requires were already in the file from `INIT-004/SPEC-017`. This spec did not rewrite them. It added the same rule at the top of Pictures.
- Research index: `RSCH-005` is the source of the approval checks (wrong object, extra story, letters in the pixels). Those checks are already the rule in ADR-0004 §7. The guide states them in operator language. No new decision.

---

## Effort Tracking

**Agent session time:** not recorded here (orchestrator owns the timer)
**Spec effort estimate:** 3 hours
