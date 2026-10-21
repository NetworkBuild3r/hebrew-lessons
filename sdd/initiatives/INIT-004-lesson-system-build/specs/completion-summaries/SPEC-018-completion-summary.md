# SPEC-018 Completion Summary — Rewrite the hebrew-worksheet skill for plans

**Traceability:** INIT-004/SPEC-018
**Status:** complete (orchestrator closed 2026-10-09 — re-verified ACs against `.cursor/skills/hebrew-worksheet/` by search; all four PASS; no secrets/learner-profile/approve tools)
**Domain:** docs
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `.cursor/skills/hebrew-worksheet/SKILL.md` now teaches the plan tools as the default path.
- [x] `.cursor/skills/hebrew-worksheet/schema.md` documents id paths, revision, images, audio, and N1–N12 by pointer, and keeps the worksheet field list for `upsert_lesson`.
- [x] Tests: none. The spec's mechanized search was run (see Tests).

## Acceptance Criteria

Implementer evidence is below. These boxes are the author's reading of the files, not an independent verdict.

- [x] ac-1 (REQ-012): the seven plan tools plus `upload_media`, `plan_image`, and `generate_image` are named, and approval is stated as not an MCP success.
- [x] ac-2 (REQ-012): writes send `expectedRevision`, `revision_conflict` means stop and read again, and N1 is a warn on `migration.legacy` plans rather than a reason to delete words.
- [x] ac-3 (REQ-014): account names, answer text, and secrets are forbidden in plan files, and inventing a `.mp3` path is forbidden.
- [x] ac-4 (REQ-004): `upsert_lesson` is the worksheet tool, not the plan editor.

## AC Verification (independent)

Independent verification was **not run**. This session is a dispatched subagent. The Task tool schema available here has no `readonly` flag, so a nested verifier that could edit files was not launched. The parent orchestrator still owns the fresh-verifier loop.

Overall: **not verified** · remediation loops: 0.
**Evidence pack:** [`SPEC-018-eval-report.md`](SPEC-018-eval-report.md)

**Verification tier:** `full` · **pre-flight:** the dispatch prompt did not include an `AC verification tier` line, so this agent did not classify the spec as light · gate_actions=`[]` · security_review=`light` · domain=`docs` · autonomy_level=`plan_only` · light_tier_eligible=`not declared` ⇒ treated as `full` · **checks skipped:** `None`

| AC | Implementer reading | Evidence (file:line · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | met on the file | `SKILL.md` lines 26–32 name `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, `publish_plan`. Lines 78–82 name `plan_image`, `generate_image`, and `upload_media`, and line 80 says approval is not an MCP success. |
| ac-2 | met on the file | `SKILL.md` lines 34–36 (`expectedRevision`, stop on `revision_conflict`, do not merge). Line 62: N1 warns when `migration.legacy` is true, and words are not deleted from `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech`. |
| ac-3 | met on the file | `SKILL.md` lines 92–99 forbid account names, answer text, transcripts, and secrets. The only sample account id is `example-learner`. Line 88 forbids inventing a `.mp3` path. |
| ac-4 | met on the file | `SKILL.md` lines 105–107: `upsert_lesson` is the worksheet tool, not the plan editor, and does not accept a plan document. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-018`
- **Title:** Teach the hebrew-worksheet skill to edit lesson plans with the plan MCP tools
- **Files changed:** 3 skill files · +172 · −108 (spec status and this summary are separate)
- **Change status:** proposed (not committed)
- **Reversibility:** revertible — restore the previous skill files from git

---

## Tests

No test file. The spec's mechanized check was a string search over `.cursor/skills/hebrew-worksheet/`.

| Check | Result |
| ------------- | --------- |
| Seven plan tool names, `upload_media`, `plan_image`, `generate_image`, `revision_conflict`, `expectedRevision`, `migration.legacy`, `example-learner`, `upsert_lesson` each occur | found |
| `OPENAI_API_KEY=` and `ELEVENLABS_API_KEY=` | absent |
| `get_learner_profile`, `get_learner_snapshot`, `approve_media` | absent |

**All tests passing:** the search printed `SEARCH_EXIT: 0` and the process exited 0. **Coverage (new code):** not applicable. This spec names no test file and no coverage tool is configured for a documentation change.

---

## Gates & Controls

None — no `gate_action`. The change is revertible documentation. Commit, push, and PR were not performed.

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | revertible | n/a | string search exit 0 | Rollback is restoring the skill files from git. |

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline read of the three skill files · **self_reviewed:** true (a security-review subagent was not dispatched; same readonly-Task gap as AC verification)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| none | — | — | — | — |

- **Positive observations:** the skill forbids secrets, account names, answer text, and transcripts in plan files. The only sample account id is `example-learner`. It does not assign `OPENAI_API_KEY` or `ELEVENLABS_API_KEY`. It states that image approval is a human action and that a successful media tool call is not approval. It does not document learner-profile tools.

---

## Files Changed

Provenance is `INIT-004/SPEC-018` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `.cursor/skills/hebrew-worksheet/SKILL.md` | Modified — plan tools are the default path; short `upsert_lesson` section kept | `INIT-004/SPEC-018` |
| `.cursor/skills/hebrew-worksheet/schema.md` | Modified — plan id paths, revision, images, audio, N1 pointer; worksheet fields kept as legacy | `INIT-004/SPEC-018` |
| `.cursor/skills/hebrew-worksheet/example.md` | Modified — labeled as the legacy worksheet sketch | `INIT-004/SPEC-018` |

`scripts/validate.js` was not changed. No application code was changed.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 blocked | Cursor Task in this session has no `readonly` parameter. A writable nested verifier was not launched. Evidence pack only. Spec left `in_review`. |
| `phase_4_security_review` | `subagent_dispatch` | T3 | Light review run inline by the author. `self_reviewed: true`. No Critical or High finding. |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 / Phase 5 | `trace_open`, `dispatch`, `trace_complete`, and intermediate emits | Dispatched subagent: the orchestrator owns the trace. `scripts/dev/` is not in this repo, so this agent could not call `open_trace` / `emit` / `close_trace`. | This spec's own live `.sdd-trace/SPEC-018.jsonl` was not written here | Tier-5 invariants and model-dispatch conformance cannot pass on a file this agent did not create. Spec stays `in_review`. |
| Phase 4.5 | `harness_fallback` | Same missing trace library | The fallback row above is only in this summary | Disclosed here. |

---

## Deviations from the Spec

- Commit, push, and PR were skipped because the dispatch said to skip them. The git identity guard was not the reason (the configured email is not recorded here as a match against an empty library guard list).
- `.claude/agents/implementation-planner/AGENT.md` is not in this repo. The spec already notes that degradation. The skill was written in the existing skill's instructional voice, against ADR-0003 §4 and §6 and the tool names in SPEC-005 and SPEC-012.
- Spec status is `in_review`, not `complete`, because independent AC verification and the Tier-5 trace check have not passed.
- `example.md` was given a legacy banner so the worksheet sketch is not the default lesson shape. The spec's deliverable names `SKILL.md` and the schema doc; the banner is so those two files do not point at a contradictory default.

---

## Effort Tracking

**Agent session time:** not captured — the orchestrator owns the timer for this dispatch
**Spec effort estimate:** 4 hours
