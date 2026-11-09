# Assessment Evaluation Report — ASMT-004

- **Target:** `sdd/assessments/ASMT-004-hebrew-web-app/ASMT-004-hebrew-web-app-assessment-report.md`
- **Oracle:** `.claude/skills/sdd-assess/assets/eval/assessment-oracle.md`
- **Oracle provenance:** authored_by: human · generator_coupled: false
- **Judge:** independent read-only general-purpose sub-agent with a fresh context, which did not write the report. A different fresh judge ran each of three rounds; each round after the first scored the report with the previous round's fixes applied. harness_source: cursor. Agent ids: round 1 `decb856c-53b0-445f-8e4c-c78c4fc22018`, round 2 `b7464909-173b-4148-8646-5a8dbcfccff4`, round 3 `bf475042-2cd9-4378-a8cd-266806fe4efd`.
- **Evaluated at:** 2026-11-08 (local)
- **Verdict:** MITIGATE (3 of 4 in each round; every flag fixed; no fourth round run)

## Metrics

| Signal | First pass | Final | Notes |
| --- | :---: | :---: | --- |
| `llm_judge_score` (1–4 → [0,1]) | 0.67 (3/4) | 0.67 (3/4) | All three rounds rated 3. No round found a fabricated claim or a wrong code citation |
| `ragas.faithfulness` | 0.86 (31/36) | 0.93 (43/46) | Round 2 was 0.90 (36/40). The "partly" claims were wording or counts, not mechanisms |
| `ragas.answer_relevance` | n/a | 0.95 | Answers "why was lesson 12 bad, and what cleans up old and new units" with a diagnosis, a sweep, and a spec skeleton |
| `ragas.context_precision` | n/a | 0.90 | Round 3 flagged one card label ("placement") that the saved evidence did not show; since reworded |
| `property_checks` | 16 / 0 | 16 passed / 0 failed | Re-run after the final edits: `eval/ASMT-004-hebrew-web-app-property-checks.json` |
| `ac_pass_rate` | 0.75 (9/12) | 0.92 (11/12), then fixed | Round 2 passed 12/12. Round 3 failed AC1 narrowly on the INIT-036 file count, which was then corrected |

## LLM-judge rationale (summary of the judges' evaluations)

The adapted framing is coherent. The "source" is the app's lesson pipeline plus learner 1's 34 published units. Capability means the pipeline has the parts a good lesson needs. Readiness means what it ships passes its own rules and matches what this learner was taught.

**Exact matches all three judges confirmed:**
- **Word list and buttons.**
  - The word list tokenizes both sides of a contrast (`db/put_unit.go:214-252`, `units/doc.go:241-250`).
  - The button tiers and pool queries (`web/lesson.go:1339-1352`, `:1391-1441`; `db/lesson.go:209-251`) apply no learner filter.
- **Generator and rules.**
  - The `unit.same_shape` inversion holds across the spec, code, test, catalog row and guide.
  - The published-mode downgrade (`units/validate_cloze.go:12-20`, `:46-52`).
  - The `PaceFor` thresholds (`brief/build.go:113-116`, `:413-429`).
  - The mistake skip rule (`brief/progression.go:213-263`).
- **Sweep counts.** Every count in `scan.json` is exact:
  - 23 units, each with 0 new words and 5 lessons;
  - 128 one-word clozes;
  - one repeated lesson and the authoring note in each unit;
  - four wrong-side-only dictionary words;
  - set B at every third unit.
- **Sweep script.** It is read-only and asserts an allowlist of read tools.

**Shortfalls by round:**
- **Round 1.**
  - An absolute "INIT-036 doesn't touch these files."
  - Two off-by-one SPEC-008 line numbers.
  - The dry-run limit: it counts shared units only.
  - A five-versus-six fault count.
  - One Tier B inference stated as fact.
  - A long BLUF and a premature status line.
- **Round 2.**
  - INIT-036 shares at least three files, and its stricter Learning state (SPEC-003 line 28) raises the count `PaceFor` reads (`build.go:573-575`). Shipping it before the pace fix makes the consolidation fault worse.
  - The same-shape text also sits in the author skill and `doc.go:48`.
  - Report a problem hides a whole unit (`web/shell.go:330-343`).
  - Unit 1 was validated as `shared`.
  - Retiring a started unit already works (`db/learner_publish.go:239-251`).
  - The card and version reads were not saved.
- **Round 3.**
  - INIT-036 shares at least four files, since SPEC-004 lines 31 and 42 also edit the author skill.
  - Two wording overreaches, in the BLUF and in option 1.
  - A card labelled "placement" without saved evidence.

**Fixes:** every flag was checked against the files before fixing, and all held. The pace-sequencing finding from round 2 changed the plan: SPEC-G now ships before INIT-036 SPEC-003, and a risk row records it.

**Total rating: 3** (final)

## Acceptance-criteria checks (final)

| # | Oracle acceptance criterion | Pass? | Evidence / gap |
| --- | --- | :---: | --- |
| AC1 | Grounded, not invented | yes, after fix | Round 3 failed it only on the INIT-036 file count; now "at least four", naming the files and SPEC-004 lines 31 and 42 |
| AC2 | Both sides read for parity (adapted) | yes | "Provided" rows cite implementing code; gaps cite code plus the INIT-031, INIT-032 and INIT-036 docs |
| AC3 | Reuse-first inventory present | yes | Harvestable deltas, Provided / Adapt / Build column, reused INIT-031 and INIT-032 specs |
| AC4 | Capability ≠ readiness | yes | Separate matrices: 2.7 and 1.5 |
| AC5 | Multi-perspective (lens conflicts surfaced) | yes | Seven lenses; pace vs progress and fix-now vs fix-right surfaced and resolved |
| AC6 | Falsified (headline survived) | yes | Five tests; the coverage findings narrowed to Tier B; none reversed Refactor |
| AC7 | Justified recommendation + bounded confidence | yes | Refactor from a four-option key; Agentify, Hybrid and No action ruled out; 0.85 with the unobserved items named |
| AC8 | Executive-legible (CIO/CTO framing) | yes | Cost (about 26h, 13h critical path), risk, and the decisions needed from Brian |
| AC9 | BLUF-first, no separate TL;DR | yes | Bold recommendation, then three sentences |
| AC10 | Objectivity surfaced, not buried | yes | Same-author conflict, re-verification, dry-run limit, saved evidence, judge rounds |
| AC11 | Precise, not absolute | yes | "Share one structure", "did not flag", "at least four files" |
| AC12 | Table density | yes | One unified Technical Component Analysis table |

## Notes / follow-ups

- **Why it stays at MITIGATE.** Each fresh judge found a new, smaller gap, but none touched the diagnosis, the recommendation or a code mechanism. A fourth round was not run. The verdict is recorded as the last judge gave it, not raised on the strength of the fixes.
- **Saved live-read evidence** (agent store, not the source tree): `scan.json`, `cards_check.out.txt`, `shape_units.out.txt`, `version.out.txt`. Not saved: the study digest read, so the 21:27 UTC struggle times, the 158 due cards, the 59% recall retention and גָּר's 21 reviews remain unverifiable by a later reader. None of them carries the recommendation.
- **Not run:**
  - the branded PDF render (`pandoc` is missing, as for ASMT-003);
  - a brief-based validation of units 1 to 11;
  - a live capture of the four buttons on the failed step;
  - any MCP write.
