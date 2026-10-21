# SPEC-008 · Service: Validator nudges — recycling, coverage, glosses

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-001
title: "Validator nudges: recycling, coverage, glosses"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-009]
```

---

## Description

**Context:** lessons are authored by an agent through the `hebrew-worksheet` skill and checked by `.cursor/skills/hebrew-worksheet/scripts/validate.js`. `RSCH-003`: more encounters help (Moderate), input alone adds words slowly so deliberate study runs alongside it (Low), and stories read best at high known-word coverage (Low). The validator can nudge authors toward these without blocking publication.

**Scope (in):** add **warnings** (never errors) to the validator:
- Recycling: fewer than 3 vocab words recycled from earlier published lessons (via the word-audio catalog or the vocab index when present).
- Encounters: a new vocab word that appears fewer than 2 times across the story, sentences, and activities.
- Coverage: estimated share of story tokens that are this lesson's vocab or earlier lessons' vocab below 90% (labelled an estimate; prefixes not stripped).
- Glosses: validate `reading.glosses` shape when present (object of string → string, keys found in the story text); warn on keys not in the story.
- Surface the warnings in `upsert_lesson`'s reply text next to the existing validation summary.

**Scope (out):** blocking publication on these rules; rewriting existing lessons; client code.

---

## Understood (agent's read of the intent)

- [x] und-1: Warnings guide the authoring agent; the operator can still publish anything.

## Acceptance Criteria

- [x] ac-1 (REQ-008): a fixture lesson with no recycled words gets a recycling warning; one with 3 or more does not — covered: yes
- [x] ac-2 (REQ-008): a new word that appears only in the vocab list gets an encounters warning — covered: yes
- [x] ac-3 (REQ-007/008): a story with many unknown tokens gets a coverage warning with the estimated percentage — covered: yes
- [x] ac-4 (REQ-006): malformed `reading.glosses` is an error (schema), keys missing from the story are warnings — covered: yes
- [x] ac-5: existing published lessons still pass validation with zero new errors — covered: yes
- [x] ac-6: `upsert_lesson` reply includes the warnings — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Thresholds | recycle ≥ 3, encounters ≥ 2, coverage ≥ 90% (warning only) | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `.cursor/skills/hebrew-worksheet/scripts/validate.js`
- [x] `mcp/tools.js` — include warnings in the reply
- [x] Tests: `test/validate-activities.test.js` (or new `test/validate-nudges.test.js`)

---

## Technical Requirements (domain-specific)

- [x] Reuse the tokenizer from SPEC-007 if it has landed; otherwise a local copy with a TODO to consolidate.
- [x] No new dependencies.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`; validator run over every lesson in `worksheets/` shows no new errors.
- **Rollback plan:** revert the two files and the test.

---

## Security

- **Sensitivity:** standard — triggers matched: reads lesson files and catalogs on disk.
- **Security acceptance criteria:**
  - [x] Reads only `LESSONS_DIR` content via existing path helpers.
- **Review depth:** standard · **focus areas:** data.

---

## Verification Strategy

- **Claim:** the validator warns on low recycling, low encounters, low estimated coverage, and stray gloss keys, errors on malformed glosses, and adds no new errors to existing lessons.
- **Check + executor:** `mechanized` — `npm test` with fixture lessons; a script run of the validator across `worksheets/*`.
- **Pass condition:** each fixture yields exactly the expected warning or error codes; the run across existing lessons reports zero new errors; `npm test` exits 0.

---

## Integration Points

- Rules documented in SPEC-009; glosses consumed by SPEC-007.
- Produces **Change:** validator warnings and their display in the MCP reply.
- **Provenance tag:** `INIT-001/SPEC-008`
