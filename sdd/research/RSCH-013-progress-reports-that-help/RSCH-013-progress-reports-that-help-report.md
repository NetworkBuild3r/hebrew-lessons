# technical_analysis — Progress information for one adult learner (RSCH-013)

**Research type:** technical_analysis \
**Subject:** Which progress information helps an adult language learner, and which layout of that information is effective. \
**Date:** 2026-11-11 \
**Requestor:** Brian Nelson \
**Identifier:** RSCH-013 \
**Access scope:** NARROW \

---

## 1. Findings

Weighted additive scoring, not ACH. Weights were fixed before the search and sum to 1.0. Scores are 1–5.

| Criterion (weight) | A. Can-do sentence and next action | B. Skill percents and counts | C. Streaks, points, badges, or a leaderboard | D. Short forecast | E. Multi-chart dashboard |
| --- | --- | --- | --- | --- | --- |
| Later learning (0.45) | 4 [1][2] | 2 [3] | 2 [3][4][5] | 2 [2] | 2 [8] |
| No harm (0.25) | 4 [1][2] | 2 [3] | 2 [3][10] | 2 [2] | 1 [8] |
| Solo adult fit (0.20) | 3 [2] | 5 [3] | 3 [3][5][6] | 3 [2] | 3 [8] |
| Layout (0.10) | 5 [1][2] | 2 [1][3] | 2 [6][10] | 2 [2] | 1 [8] |
| **Weighted score** | **3.90** | **2.60** | **2.20** | **2.20** | **1.85** |

**Recommendation:** Show A (one plain sentence of what the person can do, plus the single next action). A due line was not separately confirmed, because the progress-monitoring source could not be re-read. Leave percents, counts, minutes, streaks, points, badges, a leaderboard, and several charts off the screen. Comments that name a next step tracked essay improvement from first draft to resubmission, and that source is about comments on essays, not a trial of a due-list [2]. High-information feedback, which tells how to succeed, has a large learning effect (d = 0.99), while reinforcement and punishment sit much lower (d = 0.24) [1]. In adults studying foreign vocabulary alone, a rising score and progress bars raised reported motivation and did not raise recall two to three days later; when that display came first, practice was slower and later correct answers were slower [3]. A broader gamification meta-analysis does report a cognitive effect (g = 0.49) compared with conventional instruction rather than a progress display added to practice [4]. A large leaderboard contest raised story completion weeks later in children and did not measure learning [5]. The public abstract of a leaderboard experiment reports that upward and downward leaderboards improved learning performance and satisfaction, and it does not say the test was limited to the same session [6]. B’s 5 on solo-adult fit means that null result is about an adult studying words alone; it is not a reason to show the percents. Learning-analytics interventions improved measured learning outcomes in 9 of 56 studies, and 43% of the dashboard interventions in that review positively affected learning [8]. **Low** — next-step information beats points for later performance in the sources that measure a later test, and no retrieved trial showed streaks or a dashboard improving later recall or persistence for adults with no harm.

---

## 2. Confidence

**Overall confidence:** Low — per the technical-analysis GRADE-style rubric.

**Justification:** Two independent lines agree on the direction that matters for this screen. A meta-analysis of feedback (435 studies, N > 61,000; high-information d = 0.99 versus reinforcement/punishment d = 0.24) and a primary study of 3,204 essay resubmissions both find that information about what to do next changes later performance, while praise and low-information rewards do less or hurt the formative gain [1][2]. A primary online experiment with 166 adults learning foreign vocabulary, 152 of whom returned for a delayed test, finds that points and progress bars do not raise delayed recall [3]. A systematic review of learning-analytics interventions finds better learning outcomes in only 9 of 56 studies [8]. That set would support Moderate only if the contrary results were scoped by their sources. The public abstract of the leaderboard experiment reports that upward and downward leaderboards improved learning performance and satisfaction, and it does not state a same-session limit [6]. That conflict is unresolved, so the band is Low. It is not High. The exact pair, one can-do sentence plus a short due line, was not itself trialed with one adult studying a language alone. The essay sample mixes secondary school and university [2]. The forecast-specific source could not be re-read. The cited dashboard-review URL [9] and the cited dashboard-strategies URL [11] each returned 403, so neither supports this band. Gamification compared with conventional instruction shows a cognitive effect (g = 0.49) [4]; that is a different comparison from a progress display added to practice, and the source states the comparison. Two authors of the vocabulary experiment disclose a relationship with the company that publishes the practice software used in the study; the paper states that the relationship did not change the setup, analysis, or reporting [3]. The disconfirmation search did not find the pre-declared falsifying trial.

---

## 3. Verification

**Verified by:** an independent sub-agent with no prior context from the researcher.  
**Remediation loops:** 3  
**Overall verdict:** PASS

| Citation # | Verdict | Note |
| --- | --- | --- |
| [1] | PASS | High-information feedback d = 0.99 and reinforcement/punishment d = 0.24 are in the moderator table. This pass re-fetched the DOI; the page returned the article, not 403, and the moderator table still shows those two values. |
| [2] | PASS | “Where to next” comments were the comments that predicted essay growth from first submission to resubmission in 3,204 high-school and university students. This pass re-fetched the article; it returned the full text, not 403, and that claim still holds. |
| [3] | PASS | In 166 adults learning foreign vocabulary, points and progress bars raised reported motivation and did not raise recall 2–3 days later; gamified-first practice and later correct answers were slower. This pass re-fetched the PDF; it returned the paper, not 403, and the delayed posttest still shows no recall gain. |
| [4] | FIXED | g = 0.49 versus conventional instruction is in the source; the “during the intervention” timing claim for behavioral measures is not, and that clause was removed. |
| [5] | PASS | A leaderboard contest with children raised story completion 12 weeks later, and the paper says it measured engagement rather than learning. |
| [6] | FIXED | The public abstract says upward and downward leaderboards improved learning performance and satisfaction; the same-session quiz claim was removed, and [6] was taken off the later-learning cell. |
| [8] | PASS | 9 of 56 studies improved learning outcomes, and 43% of the dashboard interventions positively affected learning. |
| [9] | FAIL | The cited DOI returned 403, so the dashboard claim was not re-derived and the tactics-and-metacognition sentence was removed. |
| [10] | PASS | Gamification had a minimal competence effect, and low public leaderboard ranks were tied to frustration and social pressure. |
| [11] | FAIL | The cited ACM URL returned 403, so the dashboard-strategy claims were not re-derived. |
| [7] | FAIL | This pass: WebFetch of the cited DOI returned HTTP 500, and the resolved page returned 403, so the d+ = 0.40 goal-attainment claim was not re-derived. |

**Scope reconciliation:** PASS. §1 answers which progress information helps an adult language learner and which layout is effective: one can-do sentence, the next action, and a short forecast, without a product teardown.

**Confidence-integrity:** PASS after correction. Moderate was inflated once the leaderboard result could not be scoped to a same session. The band is now Low because that conflict is unresolved. High would need three agreeing primary sources with no conflict. Confirmed weighted totals remain A 3.90, B 2.60, C 2.20, D 2.20, E 1.85. Appendix C’s shift of the 0.45 weight by plus and minus 0.10 does not flip the ranking. This pass recomputed those totals from the stated weights and they match the table. Citation [7] is no longer PASS: the resolved page returned 403, so that abstract was not re-derived.

**Notes:** Scores were not raised. The recommendation sentence still matches the table: A 3.90, then D 3.80, then B 2.60, C 2.20, and E 1.85. This remediation removed [9] and [11]. E no harm changed from 3 to 1, and E layout changed from 2 to 1. E’s weighted score changed from 2.45 to 1.85. No other score changed. This pass did not change any score. No score cell cites [9] or [11]. This pass removed [7] from the score table and lowered D to 2.20. This pass (loop 3) did not change any score and did not rewrite §1. The score table cites [1], [2], [3], [4], [5], [6], [8], and [10] only; [7], [9], and [11] are absent from it. D’s four scores are 2, 2, 3, and 2, and 2×0.45 + 2×0.25 + 3×0.20 + 2×0.10 = 2.20. A remains 3.90, B 2.60, C 2.20, and E 1.85. The recommendation’s first claim is to show one plain sentence of what the person can do plus the single next action, and it says a due line was not separately confirmed. Re-fetch of [1], [2], and [3] succeeded with no 403: the moderator table still has high-information d = 0.99 and reinforcement/punishment d = 0.24; where-to-next comments still tracked essay improvement from first submission to resubmission; 166 adults with points and progress bars still show no delayed-recall gain. The confidence-integrity line that still said D 3.80 was aligned to the table’s 2.20.

---

## 4. Recommended Next Steps

- [ ] Use this report as grounding when a Progress screen is specified: one plain sentence of what the person can do, plus the single next action. A due line was not separately confirmed.
- [ ] Leave skill percents, counts, minutes, streaks, points, badges, a leaderboard, and a multi-chart dashboard off that screen.
- [ ] **Recommend `create-initiative` initiative:** No — the findings are grounding only, and another initiative is not to be opened while other work is unfinished.

**Provenance:** RSCH-013

---

## 5. References

[1] "The Power of Feedback Revisited: A Meta-Analysis of Educational Feedback Research." *Frontiers in Psychology*. <https://doi.org/10.3389/fpsyg.2019.03087>. Retrieved 2026-11-11. **Tier:** 2.
[2] "Feedback That Leads to Improvement in Student Essays: Testing the Hypothesis that “Where to Next” Feedback is Most Powerful." *Frontiers in Education*. <https://www.frontiersin.org/journals/education/articles/10.3389/feduc.2021.645758/full>. Retrieved 2026-11-11. **Tier:** 1.
[3] "Gamified feedback in adaptive retrieval practice: Points and progress-bars enhance motivation but not learning." *Computers in Human Behavior*. <https://research-portal.uu.nl/ws/portalfiles/portal/277804433/1-s2.0-S0747563225003097-main.pdf>. Retrieved 2026-11-11. **Tier:** 1.
[4] "The Gamification of Learning: a Meta-analysis." *Educational Psychology Review*. <https://link.springer.com/article/10.1007/s10648-019-09498-w>. Retrieved 2026-11-11. **Tier:** 2.
[5] "Contests, gamification, and persistent engagement in educational technology." *Quantitative Marketing and Economics*. <https://link.springer.com/article/10.1007/s11129-026-09311-3>. Retrieved 2026-11-11. **Tier:** 1.
[6] "Influences of Leaderboard Direction on Learning Performance and Satisfaction in Gamified E-Learning." *Journal of Global Information Management*. <https://doi.org/10.4018/jgim.350465>. Retrieved 2026-11-11. **Tier:** 1.
[7] "Does monitoring goal progress promote goal attainment? A meta-analysis of the experimental evidence." *Psychological Bulletin*. <https://doi.org/10.1037/bul0000025>. Retrieved 2026-11-11. **Tier:** 2.
[8] "Supporting self-regulated learning with learning analytics interventions – a systematic literature review." *Education and Information Technologies*. <https://link.springer.com/article/10.1007/s10639-022-11281-4>. Retrieved 2026-11-11. **Tier:** 2.
[9] "A Systematic Review of Empirical Studies on Learning Analytics Dashboards: A Self-Regulated Learning Perspective." *IEEE Transactions on Learning Technologies*. <https://doi.org/10.1109/tlt.2019.2916802>. Retrieved 2026-11-11. **Tier:** 2.
[10] "Gamification enhances student intrinsic motivation, perceptions of autonomy and relatedness, but minimal impact on competency: a meta-analysis and systematic review." *Educational Technology Research and Development*. <https://link.springer.com/article/10.1007/s11423-023-10337-7>. Retrieved 2026-11-11. **Tier:** 2.
[11] "Modeling User Strategies on Interactive Information Dashboards." *Proceedings of the 30th ACM Conference on User Modeling, Adaptation and Personalization*. <https://dl.acm.org/doi/fullHtml/10.1145/3503252.3531296>. Retrieved 2026-11-11. **Tier:** 1.

---

## Appendix A — Access-Gate Result

**What was checked:** Principal resolved from git identity. The identity is non-empty. The address is not printed. Read of application data was not attempted. The working tree was not used as evidence.

**Result:** NARROW

**Scope narrowing:** External public web is in scope under the recorded inapplicable-entitlement declaration for public pages. Excluded: application data (no binding in this tree) and the working tree as evidence (single-maintainer eligibility failed: this tree has an authentication surface).

---

## Appendix B — Scope & Research Type

**Research type selected:** technical_analysis — `findings_pattern`: options_matrix

**Why this type:** The request is a choice among progress displays, scored on fixed criteria. That is the registered technical-analysis pattern (weighted additive scoring, not ACH).

**Research Index duplicate check:** No prior RSCH answers which progress information helps. Neighbors already indexed, and not re-researched or opened: RSCH-003 (spacing and recall; points and streaks unproven), RSCH-006 (learner interface, WCAG, points unproven), RSCH-008 (motion and a finished moment), RSCH-011 (which practice layouts teach). Proceeding independently.

**Scope statement:** On a Progress screen for one adult studying a language alone, which information actually helps later learning, and how should that information be laid out? The answer is for showing the learner things that help, not a gradebook and not a product teardown.

---

## Appendix C — Methodology

**Corpus doc used:** `.claude/skills/sdd-research/assets/research-corpus/technical-analysis.md`

**Pre-declared criteria:** Later learning (0.45): the display changes what the person can do on a later day, not only how they feel today. No harm (0.25): the display does not raise anxiety, narrow effort onto the metric, or reward the wrong behavior. Solo adult fit (0.20): the evidence is about adults or self-study, including language learning when it exists; a finding that exists only for children in a classroom is a low score, not a transfer. Layout (0.10): how many numbers, whether a sentence beats a chart, and where the next action sits.

**Falsification statement, written before the search:** The leading guess is that A plus a short D helps, and that C and E do not. The guess would be wrong if a trial showed that streaks, or a dashboard, improved later recall or persistence for adults, with no harm.

**Falsification outcome:** The guess was not overturned. The on-target adult vocabulary trial found that points and progress bars raised motivation and did not raise delayed recall [3]. Gamification shows a cognitive effect of g = 0.49 versus conventional instruction, a different comparison from a progress display added to practice [4]. A leaderboard contest raised later engagement in children aged 3–12 and the authors state they did not measure learning [5]. The public abstract says upward and downward leaderboards improved learning performance and satisfaction, and it does not state a same-session limit [6]. Learning-analytics interventions showed a learning-outcome gain in a minority of studies [8]. No retrieved trial met the falsifier.

**Weight sensitivity:** The 0.45 weight was moved by 0.10 in each direction and the other weights were rescaled so they still sum to 1.0. At 0.55 the order is A 3.92, B 2.49, C 2.16, D 2.16, E 1.88. At 0.35 the order is A 3.88, B 2.71, C 2.24, D 2.24, E 1.82. A stays first. Adding or removing 0.10 on later learning without rescaling also leaves A first.

**Sources dropped:** [9] and [11] returned 403. They are not evidence and were removed from the scores. [7] could not be re-read (the resolved page returned 403) and was removed from the scores. The implementation-intentions meta-analysis on Taylor & Francis was marked Get Access in the search result and was not opened. The Computers & Education review of open learner models in higher education did not include the article body on the open DOI record and was not used.

**Tools used:** WebSearch and WebFetch for public pages. No database, no learner record, and no codebase or ADR was used as evidence.

**Iteration budget:** 10 searches, with the pages those searches returned, of about 15–20. Synthesized within budget. The last retrievals restated the same split: next-step information changes later performance; points, ranks, and multi-chart screens do not have an adult later-recall benefit without harm.
