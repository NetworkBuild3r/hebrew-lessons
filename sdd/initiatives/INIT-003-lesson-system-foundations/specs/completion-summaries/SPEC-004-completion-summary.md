# SPEC-004 Completion Summary — ADR-0003 course and lesson-plan language

**Traceability:** INIT-003/SPEC-004
**Status:** Complete (ADR, schema, four sample plans, and spec `status: complete` written in this session)
**Domain:** docs
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `docs/decisions/ADR-0003-course-and-lesson-plan-language.md`
- [x] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`
- [x] `docs/decisions/ADR-0003-assets/samples/my-morning.plan.json`
- [x] `docs/decisions/ADR-0003-assets/samples/anal-sex-vocab.plan.json`
- [x] `docs/decisions/ADR-0003-assets/samples/shai-date-indirect-speech.plan.json`
- [x] `docs/decisions/ADR-0003-assets/samples/beginner-unit-1.plan.json`
- [x] Tests: none in this spec. SPEC-009 adds `node spikes/skeleton/validate.js`.

## Acceptance Criteria

- [x] ac-1: Decision sections 1–10, each with options, the choice, and the rationale.
- [x] ac-2: Six operations, each with an MCP column and a builder column.
- [x] ac-3: Six block types labeled from `RSCH-004` / `RSCH-005`. All eleven activity types mapped or retired.
- [x] ac-4: Draft-07 schema at schemaVersion `1.0.0`. Four samples passed a Node walk of that schema. The ADR states `node spikes/skeleton/validate.js`.
- [x] ac-5: `beginner-unit-1.plan.json` declares `newWords`, `buildsOn`, and `recycles` on each of the three lessons.
- [x] ac-6: Decision 8 maps every `window.WORKSHEET` field or retires it, and states that audio paths do not change.
- [x] ac-7: Decision 9 lists each carried constraint once as kept, changed, or superseded.
- [x] ac-8: The ADR has no secret values, no account names, and no transcript text.

## AC Verification (independent)

An independent verifier was not dispatched. This session was instructed not to spawn sub-agents. The table below is the author's structural check against the files, plus a Node draft-07 walk of the four samples (no npm dependency). The spec's human check — the operator reading the ten decisions — is not claimed as done. The ADR status stays **Proposed**.

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL · domain=`docs` PASS · autonomy_level=`plan_only` PASS ⇒ light_tier_eligible=`false` · **checks skipped:** None

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (author) | ADR Decisions 1–10, lines 93–401 |
| ac-2 | PASS (author) | Operations table, line 219: read, replace, insert, move, remove, set field; MCP and Builder columns both filled |
| ac-3 | PASS (author) | Block table, line 129; activity map, line 145. Spelling ladder and keyboard practice deferred per `RSCH-005` |
| ac-4 | PASS (author) | `lesson-plan.schema.json` `$schema` draft-07, `schemaVersion` const `1.0.0`. Node walk: all four `*.plan.json` files passed. Command in the ADR, line 193: `node spikes/skeleton/validate.js` |
| ac-5 | PASS (author) | `beginner-unit-1.plan.json`: `cafe-order` 8 new / builds on none; `cafe-again` 5 new / builds on `cafe-order` / 8 recycles; `cafe-recall` 0 new / builds on both / 13 recycles; checkpoint lists the 13 words |
| ac-6 | PASS (author) | Decision 8 field table, line 317. Wording check: every source string except the retired icon class is present in the migrated JSON. Audio paths copied character for character |
| ac-7 | PASS (author) | Decision 9 table, line 376 |
| ac-8 | PASS (author) | ADR text has no key material, no account name, and does not paste lesson passages. Samples carry existing lesson wording, including names that are already part of that wording |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-004`
- **Title:** Decide the course and lesson-plan language and publish its schema and samples
- **Files changed:** 8 (ADR, schema, four samples, this summary, SPEC-004 status)
- **Change status:** proposed
- **Reversibility:** revertible (delete the ADR, `docs/decisions/ADR-0003-assets/`, and this summary; set status back to `ready`)

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| —         | 0         | 0     |

**All tests passing:** N/A (docs; SPEC-009 owns `node spikes/skeleton/validate.js`) · **Coverage (new code):** N/A

Session check, not a committed test: a Node walk of JSON Schema draft-07 accepted all four samples. The café unit's stories and listening passages measured 100% known-word coverage under the ADR's tokenizer. Introduction blocks are 2 of 11. No npm packages were added.

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `plan_only`. No one-way action taken.

aud-1 and aud-2 were already resolved on the spec (operator 2026-10-08, "go") and are the choices in Decisions 1 and 3. The ADR itself remains Proposed until the operator reads it.

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** inline review of the ADR security section (`self_reviewed: true` — documentation, no executable code)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —            | —        | no findings |

- **Positive observations:** Text fields are plain and escaped; images are media ids with required alt; audio cannot be a remote URL; size caps are N9–N11; `assigneeId` is an opaque id. The intimate-vocabulary sample is existing lesson wording, with visibility left at `everyone` to match the ADR-0002 seed. The new unit is `draft` and `hidden`.

---

## Files Changed

Provenance is `INIT-003/SPEC-004` for every row.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `docs/decisions/ADR-0003-course-and-lesson-plan-language.md` | Created | `INIT-003/SPEC-004` |
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | Created | `INIT-003/SPEC-004` |
| `docs/decisions/ADR-0003-assets/samples/my-morning.plan.json` | Created | `INIT-003/SPEC-004` |
| `docs/decisions/ADR-0003-assets/samples/anal-sex-vocab.plan.json` | Created | `INIT-003/SPEC-004` |
| `docs/decisions/ADR-0003-assets/samples/shai-date-indirect-speech.plan.json` | Created | `INIT-003/SPEC-004` |
| `docs/decisions/ADR-0003-assets/samples/beginner-unit-1.plan.json` | Created | `INIT-003/SPEC-004` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-004-docs-adr-0003-course-and-lesson-plan-language.md` | `status: complete` | `INIT-003/SPEC-004` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-004-completion-summary.md` | This file | `INIT-003/SPEC-004` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `trace_complete` | Dispatched sub-agent; trace ownership stays with the orchestrator | This session did not open or close the run trace | A Tier-5 reader must use the orchestrator's sentinel, not a sentinel from this session |

---

## Deviations from the Spec

- Primary prompt `.claude/agents/implementation-planner/AGENT.md` is not in this repo. The ADR is written in the same section shape as ADR-0002 (metadata, context, options, numbered decisions, consequences, downstream plan).
- SPEC-009 owns the committed validator. This session checked the samples with a Node draft-07 walk and did not add that script or any npm dependency. The walk's source was not kept.
- No independent AC verifier was dispatched.
- Samples keep character names that are already in the worksheet wording. No account names were added. The ADR does not quote those passages.
- Git operations were not requested and were not performed.

---

## Effort Tracking

**Agent session time:** one session; wall clock not instrumented
**Spec effort estimate:** 5 hours
