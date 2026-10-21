# Assessment Evaluation Report — ASMT-002

- **Target:** `sdd/assessments/ASMT-002-hebrew-web-app/ASMT-002-hebrew-web-app-assessment-report.md`
- **Oracle:** `.claude/skills/sdd-assess/assets/eval/assessment-oracle.md`
- **Oracle provenance:** authored_by: human · generator_coupled: false
- **Judge:** independent read-only general-purpose sub-agent (fresh context, did not author the report), run twice: a first pass, then a re-score after the follow-ups were applied
- **Evaluated at:** 2026-10-17 (local)
- **Verdict:** PASS (after one MITIGATE round)

## Metrics

| Signal | First pass | Final | Notes |
| --- | :---: | :---: | --- |
| `llm_judge_score` (1–4 → [0,1]) | 0.67 (3/4) | 1.00 (4/4) | First pass was MITIGATE: precision gaps, nothing invented |
| `ragas.faithfulness` | 0.94 (15/16) | 1.00 (16/16) | Two "partly" claims (C1 widths, C7 390px evidence) fixed |
| `ragas.answer_relevance` | n/a | 0.95 | Answers "how good is the design and usability" against research, WCAG, and Nielsen; ranked list and skeleton answer "how to do better" |
| `ragas.context_precision` | n/a | 0.90 | Two citation nits after re-score (a `dock360` flag and an evidence-file list), both since fixed |
| `property_checks` | 16 / 0 | 16 passed / 0 failed | `eval/ASMT-002-hebrew-web-app-property-checks.json`, re-run after the final edits |
| `ac_pass_rate` | 0.875 (9 + 3 partly) | 1.00 (12/12) | AC1, AC8, and AC11 moved from partly to met |

## LLM-judge rationale (summary of the judge's evaluation)

The adapted framing is coherent. The source is the running app; the standard is RSCH-006, -007, -008, and -010 plus WCAG 2.2 AA and Nielsen's heuristics. Capability means usability and readiness means accessibility mechanics, and the report uses those meanings throughout.

**Exact matches the judge confirmed:**
- Every count: 28 measured page states, 28 full-page and 9 probe screenshots, 7 learner destinations, and 2 primary buttons on returning desktop Today.
- Every cited line range, and the LOC column (the non-blank line count of each file).
- Both averages: capability 24/9 ≈ 2.7 and, after the dark-mode row was lowered, readiness 36/9 = 4.0.

**The four main findings, confirmed in the viewport-only screenshots:**
- the desktop Today staircase;
- the misplaced colon;
- the in-progress lesson sorted last;
- the `test` and private lessons visible to a learner.

**The falsification pass is real.** F1 and F3 lowered severities, F3 and F4 narrowed scope, F5 lowered confidence, and F2 tested the capture-artifact hypothesis.

**First-pass shortfalls, all on precision:**
- C1 claimed no overflow at 360px without measuring it, and implied all routes were checked at 1024 and 1280px.
- C7 cited a probe flag that contradicted the 390px truncation, which only the screenshot showed.
- The BLUF said focus was "always visible" from an eight-Tab sample.
- "Dark mode parity" rested on two Today states.
- The summary dropped F1's severity caveat.
- One phone measurement ("about a third") was never measured.

**Re-score:** the C1 and phone-measurement gaps were closed with a new measurement script (`evidence/probe2.js`, all 21 route/width pairs with no overflow; step heading at 265 of 844px), not softer wording. The other items were reworded or rescored.

**Total rating: 4** (final)

## Acceptance-criteria checks (final)

| # | Oracle acceptance criterion | Pass? | Evidence / gap |
| --- | --- | :---: | --- |
| AC1 | Grounded, not invented | yes | All load-bearing ledger rows trace to code lines, `ux-metrics.json`, `probe.json`, `probe2.json`, or screenshots |
| AC2 | Both sides read for parity (adapted) | yes | "Already exists" claims cite the in-house implementation (`bi()`, `ICON_NAMES`, `verbs-page.js:47`, "Show raw") |
| AC3 | Reuse-first inventory present | yes | "Existing support" column; first-run Today and Verbs empty state named as patterns to copy |
| AC4 | Capability ≠ readiness | yes | Separate matrix sections and averages (2.7 against 4.0), discussed separately |
| AC5 | Multi-perspective (lens conflicts surfaced) | yes | Seven lenses; Security vs migration intent on the visibility seed handed to the operator |
| AC6 | Falsified (headline survived) | yes | F1–F5; headline survived with two severities lowered and confidence set at 0.80 |
| AC7 | Justified recommendation + bounded confidence | yes | Refactor follows from the matrix; Tier-C items (production visibility, learner testing) become spikes |
| AC8 | Executive-legible (CIO/CTO framing) | yes | Cost (two to three weeks of front-end work), risk table, and sampled claims now qualified |
| AC9 | BLUF-first, no separate TL;DR | yes | Bold recommendation opens the report |
| AC10 | Objectivity surfaced, not buried | yes | Same-author caveat, measurement method, artifact caught and dropped, limits listed |
| AC11 | Precise, not absolute | yes | Focus, dark mode, and reflow claims scoped to what was measured |
| AC12 | Table density | yes | One unified Technical Component Analysis table |

## Notes / follow-ups

- **First-pass follow-ups:** all 11 were applied. Two were closed by new measurement (`evidence/probe2.js` → `probe2.json`). The Dark mode row was lowered from 5 to 4, which moved readiness from 4.1 to 4.0.
- **Re-score nits:** all four were applied. The `dock360` flag covers only "Progress", so the "Lessons" truncation is cited to the screenshot. `probe2.json` was added to the evidence-file list. F3 now gives the list position as "ninth of ten". F2 now states the empty-mix condition precisely (`today-page.js:367–380`, `:546`).
- **Not run:** the branded PDF render (`pandoc` is not installed), a screen-reader pass, and real-learner testing. The report lists the last two as open unknowns, and SPEC-009 covers them.
