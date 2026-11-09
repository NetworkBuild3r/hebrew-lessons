# Assessment Evaluation Report — ASMT-003

- **Target:** `sdd/assessments/ASMT-003-hebrew-web-app/ASMT-003-hebrew-web-app-assessment-report.md`
- **Oracle:** `.claude/skills/sdd-assess/assets/eval/assessment-oracle.md`
- **Oracle provenance:** authored_by: human · generator_coupled: false
- **Judge:** independent read-only general-purpose sub-agent with a fresh context, which did not write the report. A different fresh judge ran each round: a first pass, then a re-score after the follow-ups were applied. harness_source: cursor.
- **Evaluated at:** 2026-11-07 (local)
- **Verdict:** PASS (after one MITIGATE round)

## Metrics

| Signal | First pass | Final | Notes |
| --- | :---: | :---: | --- |
| `llm_judge_score` (1–4 → [0,1]) | 0.67 (3/4) | 1.00 (4/4) | First pass MITIGATE: three wrong research-grade labels and the method section ahead of the BLUF. Nothing invented |
| `ragas.faithfulness` | 0.93 (13/14) | 1.00 (30/30) | The one "partly" claim was ledger row 13 (research grades). The final pass checked all 17 grade labels |
| `ragas.answer_relevance` | n/a | 0.95 | Answers "how do we improve the learning experience further" with a ranked list and a spec skeleton, measured against the project's own research |
| `ragas.context_precision` | n/a | 0.90 | One non-load-bearing citation nit on re-score (RSCH-001 line 19 is about production errors), since reworded |
| `property_checks` | 15 / 1, then 16 / 0 | 16 passed / 0 failed | The options key was a table; the script reads a list. Fixed before round 1. Re-run after the final edits: `eval/ASMT-003-hebrew-web-app-property-checks.json` |
| `ac_pass_rate` | 0.83 (10/12) | 1.00 (12/12) | AC1 (grounded) and AC9 (BLUF-first) moved from fail to pass |

## LLM-judge rationale (summary of the judges' evaluations)

The adapted framing is coherent. The standard is the project's research (RSCH-001, -003, -004, -007) and the source is the Go app. Capability means the research-backed learning loop, and readiness means whether the learner numbers and content can be trusted. The report uses those meanings throughout.

**Exact matches both judges confirmed:**
- The discovery totals: 2,167 files, about 325,000 lines, and 613 Go files.
- Both averages: capability 56/17 ≈ 3.3 and readiness 18/6 = 3.0.
- Every code citation behind "known":
  - `learner/state.go:148-151` and `:167-192`, with no later-day rule;
  - `evidence/recorder.go:286-326`, where the say check is source self, Good on "Got it", and peeked;
  - `db/learner.go:319`, the success count;
  - `db/placement.go:614-632`, the read-Good set;
  - `web/lesson.go:610-692`, lesson scheduling.
- The absence search. Each judge ran its own search in `internal/` and reproduced the report's list of hits exactly: `placement.js:62`, its browser test, the guidance text, a schema hint, and two guidance tests.

**First-pass shortfalls:**
- **Three research-grade labels did not match the cited reports.**
  - Vowel-mark fading was labelled Moderate, but RSCH-001 lines 16 and 65 call it Low and untested.
  - The quarter share was labelled Moderate, but RSCH-004 line 39 says "Low for any exact percentage".
  - Audio pace was labelled Low, but RSCH-003 line 181 is an ungraded checklist item.
- **"How this assessment is adapted" came before the BLUF.**
- **Which code paths read which "known" set was unclear.** The tier (`brief/build.go:534`) and the coverage check (`mcpstore/learner.go:51-71`) read the learner model, while the read-Good set feeds `knownLexemes` and the suggested start.
- **Smaller issues:** a few line ranges, gap counts that didn't agree between the BLUF and the summary, and no stated basis for 0.85.

**Re-score:** every flag was checked against the files before fixing, and all held. The three labels were corrected, and every matrix row now cites a research line and code. The method note moved into the Assessment section, the BLUF was trimmed to the bold sentence plus three, the "known" consumers are named in the summary, lenses, table and SPEC-002/003, and a confidence-derivation paragraph was added. The second judge checked 30 load-bearing claims and all were supported.

**Total rating: 4** (final)

## Acceptance-criteria checks (final)

| # | Oracle acceptance criterion | Pass? | Evidence / gap |
| --- | --- | :---: | --- |
| AC1 | Grounded, not invented | yes | All 13 load-bearing ledger rows and all 17 research-grade labels verified at the cited lines; the one Tier C claim (phonetics) is non-load-bearing and gated on a native-speaker check |
| AC2 | Both sides read for parity (adapted) | yes | Every matrix row pairs a graded research line with code or the absence search |
| AC3 | Reuse-first inventory present | yes | Harvestable deltas, Capability-Parity lens, Provided / Adapt / Build column, ASMT-001 status table |
| AC4 | Capability ≠ readiness | yes | Separate matrices and averages (3.3 against 3.0); readiness held down by the "known" rule |
| AC5 | Multi-perspective (lens conflicts surfaced) | yes | Seven lenses; Security vs Capability (recording) and Data vs motivation (lower headline) surfaced and resolved or handed to the operator |
| AC6 | Falsified (headline survived) | yes | Five tests; each narrowed or reclassified a finding; none reversed Refactor |
| AC7 | Justified recommendation + bounded confidence | yes | Refactor follows from three Moderate rows at 1–2; Agentify, Hybrid and No action each ruled out; 0.85 derived and held at the floor because no data was read |
| AC8 | Executive-legible (CIO/CTO framing) | yes | Cost (17 specs, 35–100 hours, one-day spike), risk, value, and "no new stack to run or secure" |
| AC9 | BLUF-first, no separate TL;DR | yes | BLUF is the first section; recommendation in bold |
| AC10 | Objectivity surfaced, not buried | yes | Same-author caveat, re-verification, no data read, launcher disclosure, ID clash, judge rounds |
| AC11 | Precise, not absolute | yes | "Largely matches"; harvestable deltas named; pronunciation row stated as only partly closed by option 2 |
| AC12 | Table density | yes | One unified Technical Component Analysis table |

## Notes / follow-ups

- **First-pass follow-ups:** all were applied.
- **Re-score notes:** all were applied:
  - the BLUF now says three same-day right answers "may include" self-reported checks;
  - only a normal or fast right recall writes Good, since a slow one is Hard and is left out (`evidence/grade.go:41-42`, `db/placement.go:625`);
  - "matches" became "largely matches";
  - the corrective-feedback row cites `lesson.go:58`;
  - the unused RSCH-004 lines 52-53 were dropped from ledger row 13;
  - the INIT-032 ADR number is spelled out: it targets the taken ADR-0028, so it will likely take ADR-0029.
- **ID clash, found by the round 2 judge.** ADR-0030 line 24 (commit `6237152`) credits "ASMT-003" with a production measurement of typed-spelling credit. No such assessment was saved under `sdd/assessments/`. This report took ASMT-003 by the skill's folder-based numbering and discloses the clash under Objectivity. Renumbering is the operator's call.
- **Not run:**
  - the branded PDF render (see the registry note);
  - any read of application data;
  - a live run of the app;
  - real-learner testing.

  The report lists the data and learner gaps as open unknowns.
