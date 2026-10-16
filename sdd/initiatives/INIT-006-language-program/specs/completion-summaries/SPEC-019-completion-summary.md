# SPEC-019 Completion Summary — Skill and operator guide for the profile loop

**Traceability:** INIT-006/SPEC-019
**Status:** complete
**Domain:** docs
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `.cursor/skills/hebrew-worksheet/SKILL.md` — six-step profile loop, the nine registered INIT-006 tools, hard rules, recipe index
- [x] `.cursor/skills/hebrew-worksheet/recipes.md` — one card per ADR-0009 §7 recipe, with a research note on each step, plus tagging rules
- [x] `.cursor/skills/hebrew-worksheet/schema.md` — one line so the field list no longer says profile tools are undocumented
- [x] `docs/operations/accounts-and-admin.md` — AI-planning switch, Progress, forms checklist, ladder fallback; the old unqualified privacy sentence removed

## Acceptance Criteria

- [x] ac-1 (REQ-015): the skill names every INIT-006 MCP tool exactly as registered and the six-step loop.
- [x] ac-2 (REQ-015): the skill has one recipe card per recipe in ADR-0009 §7.
- [x] ac-3 (REQ-015): the operator guide points at the ADR-0009 §1 tables, quotes the disclosure sentence, and no longer states the old privacy rule unqualified.
- [x] ac-4: no key assignment, no real username, no teacher text in the new prose.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did not author the files.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-019-eval-report.md`](SPEC-019-eval-report.md)

**Verification tier:** `full` · **pre-flight:** the dispatch did not classify this spec as light-tier AC verification · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | Nine tools in `mcp/tools.js` `registerLearnerTools` all appear in `SKILL.md`. Steps 1–6 are under "The six steps". |
| ac-2 | PASS | Six recipe headings in `recipes.md`, matching ADR-0009 §7, each with a research note. |
| ac-3 | PASS | Guide points at the two §1 tables, quotes the disclosure sentence and the §9 rule, and does not contain the old unqualified sentence. |
| ac-4 | PASS | No `_API_KEY=`, `MCP_AUTH_TOKEN=`, or `ADMIN_PASSWORD=` in the skill, the cards, or the guide. Sample account is `example-learner`. |

The human check in the verification strategy (the operator reads the loop and accepts it) was not performed in this session. The blocking check named in Gates is the verification search, which passed.

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-019`
- **Title:** Teach the profile → recipe → write → coverage → draft loop, and document the AI-planning switch
- **Files changed:** worksheet skill, recipe cards, schema pointer, operator guide, this spec's status
- **Change status:** proposed
- **Reversibility:** revertible. Restore the four docs files. No application code, no migration, no secret.

The skill tells the agent to call `get_learner_profile`, stop on `planning_off`, rotate recipes, tag new words, hold readings at 0.95 coverage, and save a draft. It forbids pasting a profile, a username, a typed answer, or a transcript into a plan or a lexicon entry. `set_lexicon_verified` and `set_ai_planning` are documented as always forbidden.

The operator guide replaces the old "cannot read progress or answer history" bullet with ADR-0009 §9. It points at the §1 field list and the never-returned list, quotes the disclosure sentence, and says how to read the profile on Progress, turn the switch on from Admin → Users, tick the SPEC-018 forms checklist, and set `SRS_SCHEDULER` to `ladder`.

---

## Tests

No test file. This spec's deliverables are documentation.

| Check | Result |
| ------------- | ----- |
| Nine registered tool names present in `SKILL.md` | pass |
| Six recipe cards in `recipes.md` | pass |
| Disclosure sentence and §9 rule match ADR-0009 | pass |
| Unqualified privacy sentence absent | pass |
| `_API_KEY=`, `MCP_AUTH_TOKEN=`, `ADMIN_PASSWORD=` absent | pass |

**Coverage:** not applicable. No new application code.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | revertible | n/a | verification search | `plan_only`. No `gate_actions`. Commit, push, and PR were not requested and were not done. |

---

## Security Review

- **Sensitivity / depth:** light · focus: no secrets, no real learner data, no profile pasted into lesson files · **method:** author review of the changed docs, plus the independent verifier's search for assignment literals. A separate security-review subagent was not dispatched. SAST and secret scan: not run — none configured for this repo.
- **self_reviewed:** true for the author's judgment. The secret-assignment search was repeated by the AC verifier.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

The new prose names environment variables and does not assign them. The only account id in examples is `example-learner`. The skill repeats that a profile must not be written into a plan or a lexicon entry. The guide states that the MCP cannot flip the switch and cannot mark a form verified.

---

## Trace Emission Degradation

None from this process. This spec was orchestrator-dispatched. Timer start/complete and `open_trace` / `close_trace` belong to the orchestrator. This process did not call them. Tier-5 and model-dispatch checks against the live trace are the orchestrator's, before dehydration.

## Harness Adaptation

AC verification ran as a fresh read-only sub-agent on Cursor (model `composer-2.5-fast`). The dedicated security-review subagent was not dispatched; see Security Review.

## Deviations

- `primary_prompt` `.claude/agents/implementation-planner/AGENT.md` is not in this repo. The spec already names that degradation. Work followed the spec, ADR-0009, and the existing skill. No persona was manufactured.
- Discipline profile is `software` (initiative field). Domain is `docs`; the docs pattern was used. No application code.
- `schema.md` gained one sentence so it does not contradict the new loop. The spec's deliverable list names `SKILL.md` and `recipes.md`; the schema line is that consistency fix.
- Commit, push, and PR skipped: the dispatch said not to commit.
- The operator has not yet read the loop. That human check is recorded above and was not treated as a `gate_action`.

## Files Changed

| File | Change | Provenance |
| ---- | ------ | ---------- |
| `.cursor/skills/hebrew-worksheet/SKILL.md` | Profile loop, tool table, hard rules, recipe index | `INIT-006/SPEC-019` |
| `.cursor/skills/hebrew-worksheet/recipes.md` | Created. Six recipe cards and tagging rules | `INIT-006/SPEC-019` |
| `.cursor/skills/hebrew-worksheet/schema.md` | Profile tools are documented in `SKILL.md` | `INIT-006/SPEC-019` |
| `docs/operations/accounts-and-admin.md` | AI planning, Progress, checklist, ladder fallback | `INIT-006/SPEC-019` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-019-docs-skill-and-operator-guide.md` | Status `complete`; AC checked | `INIT-006/SPEC-019` |
