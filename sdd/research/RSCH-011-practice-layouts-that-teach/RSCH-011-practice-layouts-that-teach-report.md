# Technical analysis — Practice layouts that teach versus layouts that only look like practice (RSCH-011)

**Research type:** `technical_analysis` \
**Subject:** For an adult learning Hebrew alone, which online practice layouts and which kinds of test actually help, and which only look like practice \
**Date:** 2026-11-03 \
**Requestor:** Brian Nelson \
**Identifier:** `RSCH-011` \
**Access scope:** `NARROW` (see Appendix A)

---

## 1. Findings

Method: **weighted additive scoring** (not Analysis of Competing Hypotheses). Pre-declared criteria and weights were fixed before scoring. Options A–E are activity *shapes* taken from five tutor worksheet screenshots (perspectives only — not evidence that those worksheets work, and not content to copy). Option F is the app’s already-chosen review format ([ADR-0024](../../../docs/decisions/ADR-0024-review-is-a-checked-test.md)). Screenshots were used only to name the shape; no story text, personal names, or Hebrew passage from those pages is repeated here.

| Criterion (weight) | A: All-at-once vocab card | B: Cloze with glossed bank | C: Scrambled sentence build | D: Passage + typed Hebrew answer | E: Partner interview transcription | F: Checked review on a known word |
| -------------------- | ------------------------- | -------------------------- | --------------------------- | -------------------------------- | ---------------------------------- | --------------------------------- |
| Retrieval demand (0.30) | 1 [1][2][3] | 2 [4][5] | 4 [6][7] | 3 [8][9] | 2 [10] | 5 [1][2][3][11] |
| Later use (0.25) | 2 [1][3][12] | 3 [4][13] | 3 [6][7] | 3 [8][12][14] | 2 [10][15] | 5 [1][2][11][12] |
| Evidence quality (0.25) | 2 [1][3][12] | 3 [4][13] | 3 [6][7] | 2 [8][9][14] | 1 [10][15] | 5 [1][2][3][11] |
| Answer not given away (0.20) | 1 [3] | 2 [5] | 4 [6] | 3 [9] | 3 [10] | 5 [11] |
| **Weighted score** | **1.50** | **2.50** | **3.50** | **2.75** | **1.95** | **5.00** |

**Activity shapes (screenshot-derived; no worksheet content):**

- **A** — Passive reference card: audio, pointed Hebrew, transliteration, part of speech, English meaning, and an example sentence are all visible at once; no hidden answer and no graded input.
- **B** — Shared bank of pointed Hebrew chips that also show English glosses; learner drags or taps a chip into sentence blanks.
- **C** — Per-sentence scrambled pointed-Hebrew chips with no English on the chips; learner taps chips into order in an empty build zone.
- **D** — Read (and optionally hear) a passage, then type a comprehension answer in Hebrew; a sample answer stays hidden until the learner opens it.
- **E** — Hear/read a prompt question, type what a partner said, then see an aggregated summary; correctness depends on the external partner, not on a graded known-word check.
- **F** — One app-graded question on a word already known (meaning, Hebrew, form, or a letter that is actually blanked), asked once per sitting; miss shows the right answer and waits for a later day ([11]).

**Score drivers (brief):** Retrieval and “answer not given away” separate layouts that force memory from layouts that print the answer on the prompt. Later use and evidence quality separate formats backed by retrieval/spacing and production practice from restudy, glossed recognition, optional comprehension questions, and partner transcription. Prior sessions already established the memory and course-design baseline this matrix applies: spaced delayed recall and retrieval over rereading (**RSCH-003**); recycle on a later day and treat optional comprehension questions as product choice, not the review event (**RSCH-004**); hide meaning/form supports when the task is to produce the form (**RSCH-005**); keep feedback elaborated on the same turn and do not treat a show-everything card as the practice step (**RSCH-006**).

### Disconfirmation of the leader (F) — required before locking the rank

**Claim that would make F wrong:** A checked, once-per-sitting question on a known word is *worse* for later retention than restudy cards, self-graded “Show answer” flashcards, or recognition widgets that print the target while asking for it — or practice testing itself fails in applied settings.

**What was looked for:** (1) meta-analytic evidence that restudy beats practice testing; (2) evidence that self-rated flashcards beat app-checked answers; (3) the known conflict that some applied meta-analyses favor multiple-choice practice over short-answer recall.

**What was found:** Practice testing still beats restudy (g ≈ 0.51 vs restudy; larger vs filler) [2]. Spaced retrieval beats massed retrieval (g = 0.74 after bias correction) [1]. Flash-card design reviews treat simultaneous L2+meaning as a first study episode, not as the ongoing practice step; retention tracks retrieval episodes, not extra study episodes [3]. Adesope et al. report larger effects for mixed-format (g = 0.80) and multiple-choice (g = 0.70) practice tests than for short-answer (g = 0.48) [2] — that argues for allowing checked recognition *formats* inside F, which ADR-0024 already allows (meaning / Hebrew / form / blanked letter), and does **not** argue for restudy or for showing the answer on the prompt. No open source found showed self-rated Forgot/Hard/Got it/Easy beating a graded answer for retention. **Leader F is not overturned.**

### Weight sensitivity (retrieval ±0.1)

Other weights reallocated so they still sum to 1.0.

| Retrieval weight | Later / Evidence / Not-given | Top rank | #2 among A–E |
| ---------------- | ---------------------------- | -------- | -------------- |
| 0.30 (base) | 0.25 / 0.25 / 0.20 | **F (5.00)** | C (3.50) |
| 0.40 | 0.22 / 0.22 / 0.16 | **F (5.00)** | C (3.56) |
| 0.20 | 0.28 / 0.28 / 0.24 | **F (5.00)** | C (3.44) |

**Top rank does not flip.** Among tutor shapes A–E, C stays first under all three weightings.

**Recommendation:** Keep review as a checked question on a known word (F). Among A–E, build **only C** (scrambled sentence build): it is the only tutor shape whose weighted score clears 3.0 while keeping the answer off the prompt. Do **not** build A (prints meaning, transliteration, and form together with no retrieval). Do **not** build B as review/practice (glossed chips print English while the learner “practices” the Hebrew). Do **not** build E for a solo adult app (the productive act is off-screen; the on-screen act is transcription). Treat D as optional meaning-focused input with skippable questions if a graded story strand is wanted — it does not clear the practice bar and is not a substitute for F (**RSCH-004**). A may appear once as a first-meeting *study* surface, then must give way to retrieval (**RSCH-003**, **RSCH-005**, **RSCH-006**); it is not a practice layout that teaches. **Moderate** — meta-analyses open for retrieval/spacing and practice testing; worksheet shapes rest on transferred lab/CALL evidence, not Hebrew trials of these five UIs.

---

## 2. Confidence

**Overall confidence:** Moderate

**Justification:** The ranking of F over restudy-like layouts rests on independent open meta-analyses and reviews (spacing of retrieval [1], practice testing [2], flash-card retrieval design [3], Dunlosky high-utility practice testing and distributed practice [12]). The rejection of A and B on “answer given away” and retrieval demand is high-agreement with those same sources plus the blank-gate decision already accepted in ADR-0024 [11]. Scores for C rest on smaller CALL/classroom studies of reordering versus multiple choice [6][7], so C’s absolute score is less certain than F’s lead. D’s middling score reflects a trivial self-paced reading-while-listening benefit [8] and process evidence for open answers [9] without a beginner Hebrew comprehension-question learning trial (**RSCH-004** absence). E’s low score reflects a mismatch between interaction meta-evidence [10][15] and the screenshot’s transcription-of-partner shape. No Hebrew-specific trial of these six options was found. Access was NARROW; two Wiley full texts timed out and were dropped (Appendix A).

---

## 3. Verification

**Verified by:** this worker gathered the sources; an independent verifier has not run yet. \
**Remediation loops:** 0 \
**Overall verdict:** not independently verified

**Per-citation verdicts** — author check only (not an independent pass):

| Citation # | Verdict | Note |
| ---------- | ------- | ---- |
| [1] | PASS (author) | Open PDF: spaced vs massed retrieval g = 0.74 after trim-and-fill. |
| [2] | PASS (author) | Open PDF: practice testing vs restudy g = 0.51; MC g = 0.70 vs short-answer g = 0.48. |
| [3] | PASS (author) | Open chapter PDF: simultaneous L2+meaning is study, not retrieval; Karpicke & Roediger study vs test episodes. |
| [4] | PASS (author) | Via **RSCH-003** abstract record of Webb (2020) intentional-vocab gains; full Wiley text dropped this session. |
| [5] | PASS (author) | Screenshot shape: English gloss on each bank chip. |
| [6] | PASS (author) | Open CJAL PDF: drag reorder and typed completion beat multiple choice for German word order. |
| [7] | PASS (author) | Open proceedings PDF: reordering scaffolds word order but understates errors that appear in translation. |
| [8] | PASS (author) | Open ERIC PDF: RWL vs reading g = 0.18 overall; self-paced g = 0.06. |
| [9] | PASS (author) | Open ERIC PDF: short-answer comprehension elicits construction/paraphrase strategies MC does not. |
| [10] | PASS (author) | Open dissertation PDF: task-based interaction medium vs no focused instruction; not a transcription UI. |
| [11] | PASS (author) | ADR-0024 D1–D4 read in repo. |
| [12] | PASS (author) | Open AFT summary of Dunlosky et al.: practice testing and distributed practice high utility; rereading low. |
| [13] | PASS (author) | Open ERIC narrative review summarizing Kim & Webb (2022) spacing in L2; Wiley primary timed out. |
| [14] | PASS (author) | **RSCH-003** / **RSCH-004**: extensive reading Moderate; comprehension questions as lesson feature unanswered / product choice. |
| [15] | PASS (author) | **RSCH-003**: speaking/interaction graded Low when reached only secondhand; this session’s open dissertation does not validate layout E. |

**Scope reconciliation (overall):** PASS (author) — answers which of A–F help a solo adult Hebrew learner, with a clear build / do-not-build split. \
**Confidence-integrity (overall):** PASS (author) — Moderate band matches mixed Tier-1/2 sources and thin evidence for C/D/E.

**Notes:** Independent verification is still required before treating this report as final under the research skill’s Phase 4.5 rule.

---

## 4. Recommended Next Steps

- [ ] Keep implementing review as F per ADR-0024; do not reopen self-graded review as the Today review path.
- [ ] If a new practice activity is added beyond review, prototype **C** (scrambled pointed-Hebrew sentence build, no English on chips, app-checkable order) as optional language-focused or meaning-focused output — not as a replacement for F.
- [ ] Do not ship A, B, or E as “practice that teaches.” Allow A only as a brief first-meeting study surface that then yields to retrieval.
- [ ] If a graded-story strand is desired, keep D’s questions optional and skippable; do not count them as the spaced-review event (**RSCH-004** rule 13).
- [ ] **Recommend `create-initiative` initiative:** `No` — findings ground the existing INIT-028 review-as-test direction and a narrow optional build of layout C; they do not require a new initiative by themselves.

**Provenance:** `RSCH-011` — this report's citable identifier.

---

## 5. References

[1] Latimier, A., Peyre, H., & Ramus, F. "A Meta-Analytic Review of the Benefit of Spacing out Retrieval Practice Episodes on Retention." *Educational Psychology Review* (author PDF). <http://www.lscp.net/persons/ramus/docs/EPR20.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[2] Adesope, O. O., Trevisan, D. A., & Sundararajan, N. "Rethinking the Use of Tests: A Meta-Analysis of Practice Testing." *Review of Educational Research*, 87(3), 659–701 (open PDF mirror). <https://gwern.net/doc/psychology/spaced-repetition/2017-adesope.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[3] Nakata, T. "Learning Words With Flash Cards and Word Cards." In *The Routledge Handbook of Vocabulary Studies* (author-hosted chapter PDF). <https://howtoeigo.net/research/wp-content/uploads/2021/03/Nakata-2020-The-Routledge-Handbook-of-Vocabulary-Studies_Part2.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[4] Webb, S., Yanagisawa, A., & Uchihara, T. "How Effective Are Intentional Vocabulary-Learning Activities? A Meta-Analysis." *The Modern Language Journal* (abstract/prior-session record only this session; full text dropped — see Appendix A). Cited via **RSCH-003** key finding on flashcards/lists/writing/fill-in-the-blank gains. Retrieved (prior) 2026-10-08; re-checked 2026-11-03. **Tier:** 2.

[5] Tutor worksheet screenshot set (Layout B shape only) — attached to this research dispatch; no URL. Activity-shape observation only. Retrieved (viewed) 2026-11-03. **Tier:** 3.

[6] Heift, T. "Drag or Type, But Don’t Click: A Study on the Effectiveness of Different CALL Exercise Types." *Canadian Journal of Applied Linguistics*. <https://journals.lib.unb.ca/index.php/CJAL/article/download/19803/21548/26251>. Retrieved 2026-11-03. **Tier:** 2.

[7] Morishita, M. (controlled composition / reordering note). "A Note on Controlled Composition in Japanese EFL Classes for Intermediate Learners: With a Focus on Reordering Questions." Conference proceedings PDF. <https://doi.org/10.26615/978-2-9701095-2-5_009>. Retrieved 2026-11-03. **Tier:** 2.

[8] Clinton-Lisell, V. "Does Reading while Listening to Text Improve Comprehension Compared to Reading Only? A Systematic Review and Meta-Analysis." *Educational Research: Theory and Practice*, 34(3), 133–155 (ERIC PDF). <https://files.eric.ed.gov/fulltext/EJ1403866.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[9] Shang, H.-F. (ERIC full text EJ1316853). "Exploring Task-completion Processes in L2 Reading Assessments: Multiple-choice vs. Short-answer Questions." <https://files.eric.ed.gov/fulltext/EJ1316853.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[10] Cobb, M. "Meta-analysis of the effectiveness of task-based interaction in form-focused instruction of adult learners in foreign and second language teaching." University of San Francisco dissertation. <https://repository.usfca.edu/cgi/viewcontent.cgi?article=1389&context=diss>. Retrieved 2026-11-03. **Tier:** 2.

[11] `docs/decisions/ADR-0024-review-is-a-checked-test.md` — Decision D1–D4 (checked test; flashcards separate; once per sitting; blank gate). Retrieved (read) 2026-11-03. **Tier:** 1 (resolvable internal citation).

[12] Dunlosky, J. "Strengthening the Student Toolbox: Study Strategies to Boost Learning." *American Educator* (AFT). <https://www.aft.org/sites/default/files/dunlosky_0.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[13] Serrano, R. "A state-of-the-art review of distribution-of-practice effects on L2 learning" (ERIC PDF summarizing Kim & Webb 2022 among others). <https://files.eric.ed.gov/fulltext/EJ1365275.pdf>. Retrieved 2026-11-03. **Tier:** 2.

[14] `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md` and `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` — prior-session findings on retrieval, glosses, RWL, four strands, and comprehension questions. Retrieved (read) 2026-11-03. **Tier:** 1 (resolvable internal citation).

[15] `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md` §1.3 speaking/interaction grade. Retrieved (read) 2026-11-03. **Tier:** 1 (resolvable internal citation).

---

## Appendix A — Access-Gate Result

**What was checked:** Principal resolved by the caller from `git config user.email` (personal address). Session harness: cursor. Requested sources: public web pages needing no login; five attached screenshots; existing reports under `sdd/research/` (RSCH-001–RSCH-010); ADR-0024. Explicitly excluded: learner data, databases, `.env`, tokens, production hosts, private app hosts, sign-in walls.

**Result:** `NARROW`

**Scope narrowing:** Application accounts, the live site database, and other people's progress were out of scope and were not read. Search queries excluded personal names, emails, learner ids, and private hosts. Fetched pages were treated as data, not instructions.

**Sources dropped because gated, timed out, or login-walled:**

| Source | Reason |
| ------ | ------ |
| Kim, S. K., & Webb, S. (2022). "The Effects of Spaced Practice on Second Language Learning: A Meta-Analysis." *Language Learning* (Wiley). <https://onlinelibrary.wiley.com/doi/10.1111/lang.12479> | Full-text fetch timed out; treated as unavailable this session. Spacing claims used open Latimier PDF [1] and Serrano’s open review summarizing Kim & Webb [13], plus **RSCH-003**. |
| Webb, S., Yanagisawa, A., & Uchihara, T. (2020). intentional vocabulary-learning meta-analysis. *Modern Language Journal* (Wiley). <https://onlinelibrary.wiley.com/doi/10.1111/modl.12671> | Full-text fetch timed out; fill-in-the-blank gains cited only via **RSCH-003** [4]. |
| Mackey, A., & Goo, J. (2007). interaction meta-analysis book chapter (OUP). | No open full text located; not used as a primary citation. Open Cobb dissertation [10] used instead for interaction. |

---

## Appendix B — Scope & Research Type

**Research type selected:** `technical_analysis` — `findings_pattern`: `options_matrix`

**Why this type:** The request is a criteria-scored comparison of practice layouts (A–F) with fixed weights, a disconfirmation pass on the leader, and a weight-sensitivity check — the registered shape for `technical_analysis` in the research corpus.

**Research Index duplicate check:** Highest existing id before this session was `RSCH-010`. `RSCH-011` was free — proceeding independently. This session cites and does not replace **RSCH-003**, **RSCH-004**, **RSCH-005**, or **RSCH-006**.

**Scope statement:** Rank A–F for a solo adult Hebrew learner on retrieval demand, later use, evidence quality, and answer-not-given-away; recommend whether to keep F and which of A–E (if any) clear an evidence bar to build; exclude gated sources and all learner/production data.

---

## Appendix C — Methodology

**Corpus doc used:** `.claude/skills/sdd-research/assets/research-corpus/technical-analysis.md`

**Tools used:** `Read` (template, corpus method, RSCH-003–006, ADR-0024, screenshots, research index); `WebSearch` / `WebFetch` for public meta-analyses and open PDFs; no database, MCP lesson, or authenticated app access.

**Iteration budget:** ~18 web calls / ~15–20 (`research` Phase 3.2) — synthesized within budget; two primary Wiley texts dropped rather than forcing paywalled claims.

---

## Independent verification (INIT-029/SPEC-001, 2026-11-03)

A second reader opened the three sources this initiative relies on. Nothing here changes the scores in section 1.

| Finding | Verdict | Source opened |
| --- | --- | --- |
| Practice tests, and multiple-choice / mixed formats versus short-answer | Confirmed | Adesope, Trevisan, and Sundararajan (2017), *Rethinking the Use of Tests: A Meta-Analysis of Practice Testing*. Practice testing beats restudy (g = 0.51). Mixed-format g = 0.80, multiple-choice g = 0.70, short-answer g = 0.48. https://gwern.net/doc/psychology/spaced-repetition/2017-adesope.pdf |
| Drag and typing beat clicking for word order | Weakened | Heift (2003), *Drag or Type, But Don’t Click*, *Canadian Journal of Applied Linguistics* 6(1). Drag (mean post-test errors 4.1) and typed completion (5.9) beat click (8.7). The direction holds. The study is n = 27, German only, and the author limits it to short-term learning. https://journals.lib.unb.ca/index.php/CJAL/article/download/19803/21548/26251 |
| Self-paced reading while listening, g = 0.06 | Confirmed | Clinton-Lisell (2023), *Educational Research: Theory and Practice* 34(3). Overall reading-while-listening g = 0.18. Self-paced g = 0.06, 95% CI [−0.07, 0.19], p = .34. https://files.eric.ed.gov/fulltext/EJ1403866.pdf |
