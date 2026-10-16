# SPEC-008 eval report

**Spec:** `INIT-012/SPEC-008` (`SPEC-008-docs-skill-and-recipes.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the files).

## Test run (blocking checks)

```text
node --test test/skill-recipes.test.js
-> tests 8 | pass 8 | fail 0 | exit 0
```

## Per-AC verdicts (Loop 1 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/skill-recipes.test.js:107-114` and `.cursor/skills/hebrew-worksheet/recipes.md:13-19` — `## Lesson types` maps A–E to `topic-conversation`, `review-worksheet`, `verse-reading`, `verb-workshop`, `review-worksheet` (all in `lesson-plan.schema.json:601-608`). |
| ac-2 | PASS | `test/skill-recipes.test.js:116-131` — every block `type` in `recipes.md` is in `block.oneOf` or is `match` marked `INIT-010, not held yet` (`recipes.md:16,49,131`). |
| ac-3 | PASS | `test/skill-recipes.test.js:133-148` — scans `SKILL.md`, `recipes.md`, and `schema.md` confirming no line says `choose`, `unscramble`, `true-false` / `true or false`, or `future` tense is unheld (`imperative` and `match` remain noted at `recipes.md:83,131`). |
| ac-4 | PASS | `test/skill-recipes.test.js:150-157` — extracts the five exercise JSON examples from `recipes.md:187-308` (`choose`, `true-false`, `unscramble`, `translate`, `dictation`) and verifies `validatePlan` returns `ok: true` with zero blocking findings in a `1.2.0` lesson. |
| ac-5 | PASS | `test/skill-recipes.test.js:159-166` — parses `.cursor/skills/hebrew-worksheet/SKILL.md:236-251` and confirms exact parity with `shared/admin/activity-map.js:12-43`. |
| ac-6 | PASS | `test/skill-recipes.test.js:168-174` and `recipes.md:24-26` — `## Using the teacher's class docs` instructs writing original lines, taking pointed forms from the dictionary, and never copying learner answers or class-doc text. |

## Edge cases

- Every `N1`–`N23` rule id cited in `recipes.md` exists in `server/plans/validate.js` `RULES` (`test/skill-recipes.test.js:176-183` PASS).
- JSON examples in `recipes.md` contain no `<` or `>` (`test/skill-recipes.test.js:185-190` PASS).

## Remediation-loop log

- **Loop 1 (fresh independent verifier):** all-PASS on first verification pass (8/8 tests pass, exit 0).

**Overall:** all-PASS

**Tier:** `light` · pre-flight: `gate_actions` `[]` PASS · `security_review=light` PASS · `domain=docs` PASS · `autonomy_level=plan_only` PASS => `light_tier_eligible=true` · skipped check: `None` (`test/skill-recipes.test.js` named in Deliverables was executed by both implementer and independent verifier).
