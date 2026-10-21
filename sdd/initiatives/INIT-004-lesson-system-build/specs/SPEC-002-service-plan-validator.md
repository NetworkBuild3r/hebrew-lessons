# SPEC-002 · Service: Plan validator

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-004
title: "Plan validator: JSON Schema and the research-rule registry"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: []
blocks: [SPEC-003]
```

---

## Description

**Context:** ADR-0003 §6 is the only copy of the numbers. Later code cites N1–N12. The spike validator in `spikes/skeleton/validate.js` is the base. It must not be imported at runtime, and its four-id `MEDIA_IDS` set must not be copied (`spikes/skeleton/RESULTS.md`).

**Scope (in):**

- `server/plans/validate.js` reads `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (draft-07, unknown properties fail, unknown `schemaVersion` fails).
- It reports each Decision 6 rule by id, with the severity in that table: block, warn, or inform.
- `migration.legacy: true` makes N1 a warning. Absent or false, N1 blocks publish.
- N3 coverage uses the tokenizer and the closed function list in ADR-0003 §6. Prefixes are not stripped.
- Block-severity failures refuse publish. Warnings are returned and do not by themselves refuse, including on a non-legacy plan, except where the severity is block.
- The four sample plans under `docs/decisions/ADR-0003-assets/samples/` pass schema validation. The two legacy lessons over 8 new words produce N1 warnings and do not block.

**Scope (out):** persisting a plan (SPEC-001); applying an edit (SPEC-003); resolving a media id against stored bytes (SPEC-008 and the renderer). Slug shape of `mediaId` stays the schema pattern.

---

## Understood (agent's read of the intent)

- [x] und-1: Cite rules as N1, N3, and so on. Do not restate a cap as a second number in a comment that could drift.
- [x] und-2: HTML tag starts in a text field are a block, using the ADR-0003 Security rule (`<` then a name, `/`, or `!`).

## Acceptance Criteria

- [x] ac-1 (REQ-003): `my-morning`, `anal-sex-vocab`, `shai-date-indirect-speech`, and `beginner-unit-1` validate. `anal-sex-vocab` and `shai-date-indirect-speech` return N1 as warn because `migration.legacy` is true. A copy of `my-morning` with `migration.legacy` removed and a ninth `newWords` entry returns N1 as block.
- [x] ac-2 (REQ-003): the broken cases the spike named are rejected for that reason: a tag-like string, a missing alt, text over 4000 characters, a media id that is a URL, a missing required field. A well-formed media id that is not one of the four spike pictures is not rejected by this module.
- [x] ac-3 (REQ-003): N12 blocks a word list on `fluency-pass` and warns when that block sits in a lesson whose `newWords` is non-empty. The café unit does not trip the warn. Each legacy lesson does.
- [x] ac-4 (REQ-015): `server/plans/validate.js` does not contain the strings `coffee-cup`, `water-glass`, `milk`, or `sugar` as an allow-list.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Schema implementation | Port the spike's keyword walker. Do not add a JSON Schema package. | resolved | yes — ADR-0005 Option A, no new direct dependency for this module |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/plans/validate.js`
- [x] Tests: `test/plan-validate.test.js`

---

## Technical Requirements (domain-specific)

- [x] Pure function of a document. No database, no filesystem except reading the schema file, no network.
- [x] Return value separates `blocks`, `warnings`, and `informs`, each item naming the rule id and a path.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-validate.test.js` and `npm test`.
- **Rollback plan:** delete the module and the test. No data change.

---

## Security

- **Sensitivity:** standard. Triggers matched: untrusted plan text, HTML rejection, size limits N9–N11.
- **Security acceptance criteria:**
  - [x] A string that starts an HTML tag is a block failure.
  - [x] An absolute or scheme-bearing `audioPath` is a block failure.
- **Review depth:** standard · **focus areas:** injection in plan text, size limits.

---

## Verification Strategy

- **Claim:** every sample matches the schema, each named broken plan fails for the named rule, and the spike picture ids are not an allow-list.
- **Check + executor:** `mechanized` — `node --test test/plan-validate.test.js`.
- **Pass condition:** the command exits 0, including the assertion that a non-catalog slug of the right shape is not a schema failure.
- **Test levels:** unit, fixtures are the four sample files plus in-memory mutations. No property-based suite: the registry is a finite table and each row has a direct assertion. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-003 calls `validate` on write and on publish.
- Produces **Change:** `server/plans/validate.js`.
- **Provenance tag:** `INIT-004/SPEC-002`
