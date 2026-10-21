# Assessment Evaluation Report — ASMT-001

- **Target:** `sdd/assessments/ASMT-001-hebrew-web-app/ASMT-001-hebrew-web-app-assessment-report.md`
- **Oracle:** `.claude/skills/sdd-assess/assets/eval/assessment-oracle.md`
- **Oracle provenance:** authored_by: human · generator_coupled: false
- **Judge:** independent read-only general-purpose sub-agent (fresh context, did not author the report)
- **Evaluated at:** 2026-10-18T03:05Z (2026-10-17 local)
- **Verdict:** PASS

## Metrics

| Signal | Score | Notes |
| --- | :---: | --- |
| `llm_judge_score` (1–4 → [0,1]) | 1.00 | rating 4/4 |
| `ragas.faithfulness` | 1.00 | 12 of 12 load-bearing claims traced to the cited lines |
| `ragas.answer_relevance` | 0.95 | answers both "does it conform" (matrix) and "can we do better" (ranked list and skeleton) |
| `ragas.context_precision` | 0.88 | one off-by-one line and one F4 misstatement, both since fixed |
| `property_checks` | 16 passed / 0 failed | `eval/ASMT-001-hebrew-web-app-property-checks.json` |
| `ac_pass_rate` | 1.00 | 12/12 oracle acceptance criteria satisfied |

## LLM-judge rationale (summary of the judge's evaluation)

The adaptation is coherent: the header redefines source, standard, capability, and readiness, and every section uses those meanings. The judge spot-checked 16 code citations and every load-bearing ledger row (C1–C11, C17): progress.js and k8s-database.md (ladder default), review.js:8 and :105 (30-day cap), review.js:99–103 (same-day guard), review-page.js:591–615 and :767–772 (optional typing, reveal then grade), type-it.js:203–209, build-sentence.js:173–176, match.js:81–91, words.js:230–245 (picture on Hebrew→English recall), the greps for missing features, verb-practice.js:313–338 (localStorage only), web.js:411–417 (login redirect), and fsrs.js:36–47. All matched. It also checked six research claims against RSCH-003, -005, -006, and -007, and all grades matched. The arithmetic is consistent: capability 39/14 ≈ 2.8, readiness 21/7 = 3.0, and the 7/11/3 band split. No unverified load-bearing claim.

The judge found minor issues, none load-bearing:

1. F4 said only N1 and N5 block, but N8–N13 also block.
2. C17's provenance said "via explorer quote", which contradicted the Objectivity section.
3. One citation was off by a line (`words.js:185` should be `:186`).
4. The report scored Today at 2 without noting that RSCH-007 had rated it as conforming.
5. The BLUF was longer than 2–3 sentences.

**Total rating: 4**

## Acceptance-criteria checks

| # | Oracle acceptance criterion | Pass? | Evidence / gap |
| --- | --- | :---: | --- |
| AC1 | Grounded, not invented | yes | All load-bearing claims trace to the cited lines |
| AC2 | Both sides read for parity | yes | Each matrix row pairs an RSCH grade with code lines; absences backed by greps |
| AC3 | Reuse-first inventory present | yes | Reuse lens plus an "Existing support" column (FSRS, `showMiss`, recorder, `suggestGrade`) |
| AC4 | Capability ≠ readiness | yes | Separate matrix sections, separate averages, discussed independently |
| AC5 | Multi-perspective (lens conflicts surfaced) | yes | Seven lenses; the Security vs RSCH-007 conflict is surfaced and handed to the operator |
| AC6 | Falsified (headline survived) | yes | F1–F4 tested; two findings narrowed; headline survived |
| AC7 | Justified recommendation + bounded confidence | yes | Refactor follows from the matrix; confidence 0.85; unknowns plus spikes listed |
| AC8 | Executive-legible (CIO/CTO framing) | yes | Cost, risk, and one-stack framing |
| AC9 | BLUF-first, no separate TL;DR | yes | Bold recommendation opens; BLUF trimmed after review |
| AC10 | Objectivity surfaced, not buried | yes | Executive-facing section; C17 wording fixed |
| AC11 | Precise, not absolute | yes | "About ten gaps", "about half reuse"; three harvestable deltas plus one declined item |
| AC12 | Table density | yes | One unified Technical Component Analysis table |

## Notes / follow-ups

- All five judge follow-ups were applied to the report after evaluation: the F4 wording, C17 re-read directly (`fsrs.js:36–47` confirmed FSRS-6, 0.9 retention, 1–365 days), `words.js:186`, the RSCH-007 drift note, and the shortened BLUF.
- Not run: the branded PDF render. `pandoc` is not installed on this machine, so the Markdown report is the deliverable.
