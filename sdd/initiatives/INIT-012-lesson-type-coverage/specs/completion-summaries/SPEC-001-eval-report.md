# SPEC-001 eval report

**Spec:** `INIT-012/SPEC-001` (`SPEC-001-docs-lesson-type-decision.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the files).

## Per-AC verdicts (Loop 2 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | Lesson-type table at `docs/decisions/ADR-0014-lesson-type-coverage.md:49-56` matches `INIT-012-lesson-type-coverage-initiative.md:127-133`. Mechanized check over all 12 block `type` names against `docs/decisions/ADR-0003-assets/lesson-plan.schema.json:212-226`: `Mechanized orphan blocks: none` (`match` marked `INIT-010` at `:52`; `exercise`, `assignment`, `tense: future` marked `new 1.2.0` at `:51-56`). |
| ac-2 | PASS | Five `kind` values at `ADR-0014-lesson-type-coverage.md:87`; field and limit tables at `:97-151`; pointed Hebrew + English gloss examples for all five kinds at `:105-153` (`choose` `:105-109`, `true-false` `:120`, `unscramble` `:131`, `translate` `:142`, `dictation` `:153`). `docs/decisions/ADR-0014-assets/exercise-examples.json:5-76` parses cleanly (`JSON parse OK`) and has one block per kind. |
| ac-3 | PASS | `N20`–`N22` (`block`) and `N23` (`warn`, >2 `exercise` blocks in `topic-conversation`, product choice citing `RSCH-004` §1.2) at `ADR-0014-lesson-type-coverage.md:191-199`, each with a one-sentence trigger and level. |
| ac-4 | PASS | `1.2.0` recorded at D10 (`ADR-0014-lesson-type-coverage.md:72`) and §7 (`:225-231`); quotes `ADR-0003` §3 (`:217-219`, matching `ADR-0003-course-and-lesson-plan-language.md:176`) and `ADR-0009` Decision 7 / §7 Schema (`:221-223`, matching `ADR-0009-learner-model-and-language-program.md:400`); notes the `type-it` version-guard gap for `INIT-010/SPEC-004` at `:233` vs `lesson-plan.schema.json:225`. |
| ac-5 | PASS | D6 (`ADR-0014-lesson-type-coverage.md:68`) and §9 Answer disclosure (`:250-254`) state plainly that answers sit in the published plan JSON, are checked in the browser, and are readable by the learner. |

## Remediation-loop log

- **Loop 1:** `ac-1`, `ac-3`, `ac-4`, `ac-5` PASS; `ac-2` FAIL because `dictation` in `ADR-0014-lesson-type-coverage.md:153` had a generic description instead of a concrete pointed Hebrew string with an English gloss.
- **Remediation 1:** Fix sub-agent updated `ADR-0014-lesson-type-coverage.md:153` with concrete `audioPath`, pointed `answer` (`מַיִם`), English gloss ("water"), and `accept` (`מיים`), and aligned `docs/decisions/ADR-0014-assets/exercise-examples.json:73`.
- **Loop 2 (fresh independent verifier):** all-PASS (`ac-1` through `ac-5` and Verification Strategy).

**Overall:** all-PASS

**Tier:** `light` · pre-flight: `gate_actions` `[]` PASS · `security_review=light` PASS · `domain=docs` PASS · `autonomy_level=plan_only` PASS => `light_tier_eligible=true`. Test-suite rerun skipped: Deliverables name no test file.
