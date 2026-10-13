# Prior-art review — Motion, sound, icons and picture style in second-language practice (RSCH-008)

**Research type:** `prior_art_review` \
**Subject:** Whether animated or sound feedback, icon-plus-label controls, vocabulary-picture style, and a points-free "finished" moment help adult second-language learners, and which effects the feedback kit (INIT-005/SPEC-009) may build \
**Date:** 2026-10-12 \
**Requestor:** session user (INIT-005/SPEC-005) \
**Identifier:** `RSCH-008` (the spec and initiative text call this "RSCH-007"; that number is already taken, see Appendix B) \
**Access scope:** `NARROW` (public sources only; see Appendix A)

---

## 1. Findings

**Recommendation:** Treat motion and sound as a product choice, not a learning aid. No study found shows that a feedback flash, a shake, a chime, or a celebration animation improves second-language vocabulary learning. Evidence that irrelevant extras cost learning is Moderate in general lessons and Low for this screen [4][5][7][10]. Build only effects that carry information: the correct form after a miss, a signal that audio is loading or playing, a short press state, and a calm static summary at the end [14][20]. Keep UI sounds off by default, and drop the low-pitched "miss" tone [7][10]. Show a visible word next to every navigation and primary-action icon (Moderate) [15][16][17]. There is no direct evidence on flat versus realistic versus cartoon pictures for adult second-language recall, so pick one clear, single-object style as a product choice [20][21][22]. A "finished" moment without points has no direct evidence on return to study. The best-supported mechanism is that a session's ending shapes how it is remembered and whether people choose to study again, in lab tasks [27][28][30][31]. The only large trial found on return rates tested leaderboards and prizes, which the operator excluded [33]. **Low**

### 1.1 Question 1 — Does animated or sound feedback help or distract in second-language vocabulary practice?

**Short answer:** We found no evidence of benefit for feedback animation or UI sound. We found Moderate general evidence that irrelevant extras cost learning, and Low evidence that the cost applies to this screen. Feedback *content* is what has evidence behind it.

- **Irrelevant but interesting material lowers learning.** A meta-analysis of "seductive details" (interesting but irrelevant content) found that learning with them was worse than without, with a mean effect of −0.33 (range −0.48 to −0.18). It covered 58 studies and over 7,500 students. The effect was larger when the detail sat beside a relevant diagram or stayed constantly on screen, and smaller in digital than in print [4][5][6]. Most studies were 6–10 minute lab lessons, and the authors measured cognitive outcomes only [5][6]. This is the coherence principle. **Moderate for general lessons. Low for a vocabulary card, because none of the studies was a vocabulary practice screen.**
- **Sound added to a narrated lesson hurt.** In two experiments, adding background music, sound effects, or both to a narrated animation lowered retention and transfer. Music hurt in both. Sound effects hurt only in the second experiment [7]. The authors' account is overload of auditory working memory [7]. The word audio here is narration-like, so a UI sound that overlaps it is the risky case. **Moderate that continuous or overlapping extra sound hurts, from one pair of experiments with college students. Low for a 50–100 ms chime that never overlaps speech: no study tested one.**
- **Feedback sounds in games had no measurable effect.** A between-participants study (N = 317, five conditions, plus a 49-person pre-study) added sound effects to point rewards in an image-labelling task. Performance, affect, immersion, and enjoyment did not change. Only perceived pressure differed, and the authors conclude that low-valence sounds should be avoided beside rewards [10]. **Moderate that UI feedback sounds do little, from one large study. It is not a language task, and it used points. Low for transfer here.**
- **One L2 vocabulary study favours meaning-related sound effects, not feedback tones.** 216 Polish speakers learned English words in four conditions, and word-related sound effects beat no audio at the immediate and seven-day tests. Pronunciation alone, and pronunciation plus sound effects, did not differ from no audio [8]. The study is about sound that depicts the word's meaning, not correct/wrong tones. Only the abstract was read. **Low: one study, abstract only, and a different manipulation. It also shows that adding sound to the pronunciation gave no gain.**
- **Environmental sounds were mixed.** In 36 Spanish speakers learning foreign words, a sound matching the word's meaning, a meaningless tone, and silence were compared. Both sound conditions gave a marginal benefit on a semantic task and lower accuracy on a lexical decision task than silence [9]. The authors warn about added load when exact form matters, such as spelling or written recall [9]. **Low: one small study.**
- **Animation helps when it depicts the content, which a feedback flash does not.** A meta-analysis of 61 studies (N = 7,036) found animation beat static graphics, g = 0.226. It was larger when system-paced (g = 0.309), with narration (g = 0.336), or with no accompanying text (g = 0.883) [11]. An earlier meta-analysis of 26 studies reported d = 0.37, and d = 0.40 when the animation was representational, not decorational [12]. These are instructional animations of dynamic content. A vocabulary word is static, and a tap flash or shake represents nothing. A second-language experiment cited in RSCH-006 found full captions plus still key frames helped local comprehension more than moving animation [R6 4]. **Moderate that animation can help for dynamic content. No evidence that decorative feedback motion helps vocabulary.**
- **Feeling good is not learning.** In an online experiment with 166 adults practising French vocabulary, points and progress bars raised felt competence, enjoyment, and task value. They did not change learning behaviour during practice or recall on a delayed test [13]. Points are excluded here. The result is the closest L2-vocabulary trial of "feels better, learns the same", and it supports not claiming a learning benefit for satisfying effects. **Low: one experiment.**
- **Feedback content has the evidence.** Elaborated feedback beat right/wrong only in computer-based learning (RSCH-006 §1.3 [R6 11][R6 12]). In an arcade-style game where every condition got correct and incorrect tones and points, adding explanation of the principle raised transfer (d = 0.68) [14]. That study held the tones constant, so it does not test them. **Moderate for showing the correct form on the same turn (inherited from RSCH-006).**
- **Standards.** Motion started by a user action must be disableable unless essential (WCAG 2.3.3, Level AAA), and `prefers-reduced-motion` is a sufficient technique [1][2]. Audio that plays automatically for more than three seconds needs a stop or independent volume control. Sound started by the user's own action is outside that criterion, and a sound under three seconds is exempt [3]. Automatically started motion over five seconds needs pause, stop, or hide (WCAG 2.2.2, read in RSCH-006 [R6 1]). **High for these thresholds, read in the W3C pages.** Meeting 2.3.3 is a product choice already taken in ADR-0006 rule 18.

### 1.2 Question 2 — Icon plus label versus icon alone for adult novices

**Short answer:** For a first-time user, a visible label beside the icon is better than the icon alone (Moderate). Icon-only catches up with practice. No study covers labels written in the language the learner is still learning.

- **Practitioner rule.** Nielsen Norman Group says an icon needs a text label because most icons are ambiguous. Labels should be visible at all times, not revealed on hover, and are especially critical for navigation [15]. They also recommend a label on every icon, and suggest asking whether the icon adds anything once a label is always there [19]. **Tier 3 source. Moderate overall because controlled studies below agree.**
- **Novice computer users.** Learners of an application with text-label buttons, icon buttons, or icon-plus-label buttons did best in session one with label-only and icon-label. Icon-only was much slower and needed more help, approached the others in session two, and kept a short-lived retention gap. Ease-of-use ratings were best for icon-plus-label [16]. **Moderate: one experiment, abstract only (the sample size was not read).**
- **Initial use on mobile icons.** In an experiment with 18 younger and 18 older (65+) adults, labels improved how correctly people interpreted icons at first use. Labels helped both groups about equally. Icons whose meaning was close to the depicted object, and concrete icons, also helped [17][18]. **Moderate: one controlled experiment, with a second source summarising the same work.**
- **What was not found.** None of these tests used labels in a language the user cannot yet read. Our primary labels are Hebrew, with an English gloss, and a label only disambiguates if it can be read. REQ-016 (one language per label, English only as gloss) is a product choice. **No evidence on target-language-only labels. We would keep the English gloss visible on primary navigation until a user can read the Hebrew, as a product choice.**
- **Icon-only for universal controls** (speaker, close) with an accessible name and a 44 px target has no direct test here. **Low, product choice.**

### 1.3 Question 3 — Vocabulary picture style (flat / realistic / cartoon) and recall

**Short answer:** We found no study that compares flat, realistic, and cartoon pictures for adult second-language recall. The nearest evidence says line drawings did at least as well as colour pictures in one large experiment, that simple pictures did not help abstract words, and that the pooled picture effect fades on delayed tests.

- **Line drawings did best in one large experiment.** 238 Polish B2 learners of English used one of four purpose-built online dictionary versions: colour pictures, greyscale pictures, line drawings, or none. Line drawings gave the best comprehension, shortest reception time, and best immediate and two-week retention. Colour pictures were second, similar to line drawings except for weaker long-run retention. Greyscale was least recommended [20]. The entries were monolingual English definitions, not first-language glosses, so this is not a flashcard. **Moderate for the result in that setting. Low for transfer to a Hebrew vocabulary card.**
- **Simple pictures did not help abstract words.** Japanese university EFL learners learned words with no, simple, or complex illustrations. With word lists there was no picture effect. With flashcards, both simple and complex illustrations helped concrete words, and simple illustrations hindered abstract words [21]. **Low: one study, partly read.**
- **Pictures plus a first-language gloss help at first, delayed gains are weak, and style was not reported.** A meta-analysis of 2,056 participants found first-language words plus pictures beat words alone (g = 0.334) and pictures alone (g = 0.350) on immediate tests. On delayed tests, it only marginally beat pictures alone (g = 0.271, p = 0.067). No significant moderator was detected [22]. The abstract does not report a picture-style moderator. **Moderate (the same review is graded in RSCH-005 [R5 2]).**
- **Realism.** A 1997 meta-analysis of Dwyer's nine studies on secondary and college students found that pictorial realism and colour mattered for a vocabulary factor, not for text comprehension, and it did not confirm that realistic pictures were less effective. The authors limit how far it generalises [23]. **Low: old, not language vocabulary in the modern sense.**
- **Pleasant colours and faces on the core graphics.** Meta-analyses of adding human-like faces and pleasant colours to multimedia graphics found small gains in retention (d+ = 0.387; g+ = 0.35 in the replication) and intrinsic motivation (d+ = 0.255; g+ = 0.15) [24][25]. One experiment redrew graphics with expressive faces and colours and reported d = 0.69 and 0.65 on a posttest [26]. These are science lessons, not vocabulary pictures, and the faces conflict with the operator's "no mascot" rule. The result is a counterweight to coherence: appealing colour built into the picture itself did not hurt. **Moderate for general multimedia lessons. Low for vocabulary pictures.**
- **Risk of a wrong picture.** RSCH-005 already records that a realistic-looking AI image can hide a consequential error [R5 8]. That is a reason to prefer a clear schematic style that is easy to check, adapted from that finding. **Low, adapted.**
- **Not used.** A study of children with autism found colour photographs beat black-and-white cartoons for retaining object names after five minutes. It is a different population and age, so it is not applied here.

### 1.4 Question 4 — Does a "finished" moment without points affect return to study?

**Short answer:** No direct evidence. We found no trial of a points-free completion screen against none, measuring return to study. The mechanism evidence is that endings are weighted heavily in memory and in later choices, in lab tasks.

- **How an episode ends shapes how it is remembered and chosen again.** Participants preferred an extremely effortful study episode extended with a more moderate stretch over the shorter one, even though the shorter one gave better test scores, and their later study choices followed that preference [27]. The same pattern held for difficult math tests [28]. In a study of primary-school children learning English–Dutch word pairs, an experienced peak at the end made study feel easier and shaped immediate and delayed study behaviour [29]. In a 401-person working-memory task, discomfort at the end, but not effort, predicted willingness to do the task again [31]. **Moderate that the ending matters in lab tasks (several agreeing studies, none an app trial). Low that a completion screen changes return in this app.**
- **A word-learning app experiment is consistent but small.** In 56 participants, making the completion page and the end check-in page more appealing (colour and anthropomorphic graphics) improved retrospective emotional valence. Learning accuracy improved only when both were optimised. The authors call it preliminary [30]. **Low: N = 56, abstract only.**
- **What this implies for design.** The evidence is about the *last items and how they felt*, so ending a session on a success or an easier item has more support than a screen does [27][28][31]. A summary that shows relevant content (words met, what is due next) is not a "seductive detail". An irrelevant extra at the end can be especially harmful, per the meta-analysis author's summary [6]. **Low: inference from lab studies and one practitioner article.**
- **Closure for its own sake is not supported.** A 2025 meta-analysis found no memory advantage for interrupted over completed tasks (interrupted-to-completed recall ratio 0.99). It did find a general tendency to resume an interrupted task: a weighted resumption rate of 67% across 21 publications, counting the original 1928 data [32]. That is not evidence that a finished screen brings people back, and it argues against designing a cliffhanger. **Moderate for the null on the memory effect: one meta-analysis. No evidence on return after a finished screen.**
- **The honest counter-signal.** A randomised trial of 10,000 students on an English-learning app tested a reading contest with leaderboards and prizes. During the contest, treated students completed 45% more stories. Twelve weeks after the incentives were removed they completed 76% more, and the share of active users rose from 6.5% to 9.6% [33]. Only the abstract was read. The mechanics are banned here, so SPEC-005 does not reopen that decision. It does mean the project accepts a points-free return design with weaker evidence than the mechanics it excludes. **Moderate that contest mechanics can lift engagement (one large RCT, abstract only). Not applicable to a points-free design.**
- **Other points evidence.** In the same Utrecht experiment, participants who practised with gamified feedback first and then without responded more slowly, scored lower, and practised fewer words than those who had the control condition first [13]. That bears on why points are risky to remove later. It does not answer the points-free question.

### Effects table

**Allowed** means SPEC-009 may build it. **Product choice** means it ships off by default or behind a toggle, or needs the operator's confirmation. **Banned** means it must not ship. The grade is the confidence in the evidence behind the rule. "Operator" means the rule rests on the operator's constraint, not on research. The ADR-0010 column says what already ships (see the reconciliation below).

| # | Effect | Class | Grade | Basis | ADR-0010 / shipped |
| --- | --- | --- | --- | --- | --- |
| A1 | Show the correct pointed form on the same turn after a miss, with one retry | **Allowed** | Moderate | [R6 11][R6 12][14] | Ships (`showMiss`, `.correction`). Keep. |
| A2 | Static success mark plus a short word ("Correct"), no applause | **Allowed** | product choice (informational) | no evidence it teaches; it confirms an answer | Ships (`showCorrect`). Keep. |
| A3 | Audio state on the speak control: loading and playing, ending on `ended` or `error`, static under reduced motion | **Allowed**, with conditions | product choice (informational) | system status; no learning evidence [1][2] | Ships. Conditions below. |
| A4 | Press state of 150 ms or less (scale 0.98) on buttons and cards | **Allowed** | product choice (informational) | touch acknowledgement; [1][2] for reduced motion | Ships. Keep. |
| A5 | Step or route transition of 200 ms or less (fade or small shift), none under reduced motion | **Allowed** | product choice | [1][2]; coherence says keep it short [4] | Ships (150–160 ms). Keep. |
| A6 | A visible word beside every navigation icon and primary-action icon | **Allowed** | Moderate | [15][16][17][19] | Ships in the shell (RSCH-007 §4). Keep. |
| A7 | Calm static completion summary: words met, steps done, next review due, one primary action | **Allowed** | product choice (assumption A1) | relevant content, not decoration [6]; effect on return not evidenced | Ships (`mastery-ceremony`). Keep static. |
| P1 | UI sounds for correct and tap: off by default, one visible mute control, never overlapping Hebrew audio | **Product choice** | Low | [7][10] | Engine ships off and silent during speech. **No visible control in committed code**; an uncommitted player-header button is in progress (see below). |
| P2 | The 220 Hz low "miss" tone | **Product choice, recommend drop** | Low | low-valence sound adds pressure [10] | Ships in `chime('miss')`. **Research downgrades it.** |
| P3 | Wrong-answer shake (`match-wrong-shake`, 0.12 s) | **Product choice, recommend static highlight** | no evidence | nothing supports it; the correction already carries the information | Ships in CSS. Not listed in ADR-0010. **Flag.** |
| P4 | Hover lift and elevation shadows | **Product choice** | no evidence | cosmetic | Ships. Harmless; not required. |
| P5 | Pleasant warm colour built into cards and pictures | **Product choice** | Moderate (general), Low (vocabulary) | [24][25][26] | Ships (warm paper palette). |
| P6 | Icon-only universal controls (speaker, close) with an accessible name and a 44 px target | **Product choice** | Low | [15][17] | Ships for the speak button. |
| P7 | One consistent picture style: clear, single concrete object, plain background, no picture on an abstract word | **Product choice** | Low (adapted) | [20][21][22][R5 8] | Decided in SPEC-006 (illustration guide). |
| P8 | Word-meaning sound effects (a sound that depicts the word) | **Product choice, defer** | Low | [8][9] | Not built. Needs its own test. |
| P9 | A short, static or fade-only mark on the completion summary | **Product choice** | no evidence | [32] | Not built. |
| B1 | Points, streaks, levels, badges, XP, leaderboards, score counters, "streak at risk" copy | **Banned** | Operator; evidence Low (RSCH-003, RSCH-006) | Q4 counter-signal [33] noted, not reopened | Excluded (ADR-0010 §5). |
| B2 | Mascot, characters with lines, jokes, funny error copy | **Banned** | Operator | none | Excluded. |
| B3 | Confetti, fireworks, particles, looping or bouncing celebration, full-screen effects | **Banned** | Moderate (general) | [4][5][6]; operator ("no patronizing applause") | Excluded (ADR-0010 §5). |
| B4 | Background music, ambient loops, any sound on page load | **Banned** | Moderate | [7][3] | Not present. |
| B5 | Any UI sound overlapping Hebrew word audio | **Banned** | Operator; Moderate | [7] | Guarded in `chime()`. |
| B6 | Self-running motion over 5 s with no pause, including an unbounded loading pulse | **Banned** | High (WCAG 2.2.2) and operator | [R6 1] | **Flag**: `.is-loading` pulse has no cap. |
| B7 | Motion that ignores `prefers-reduced-motion` | **Banned** | Operator; AAA taken as product choice | [1][2] | Honoured in the CSS blocks read. |
| B8 | An effect in place of, or ahead of, the correct form | **Banned** | Moderate | [R6 11][R6 12][14] | Not present. |
| B9 | Icon-only primary navigation or primary action; labels shown only on hover | **Banned** | Moderate | [15][16][17][19] | Not present. |
| B10 | Showing an unchecked photoreal AI picture | **Banned** | Moderate (RSCH-005) | [R5 8] | Governed by RSCH-005. |

**Conditions on A3.** The loading pulse must stop after a cap of 5 seconds or less and fall back to a static "loading" state. The playing pulse ends with the clip. For any clip longer than a few seconds, the learner must be able to stop the audio. The reduced-motion state is static, but the colour change stays.

### Reconciliation with ADR-0010 and the shipped feedback kit

ADR-0010 and INIT-007/SPEC-002 already built the kit (`shared/player/feedback.js`, `shared/player/speech.js`, `shared/worksheet.css`). The research does not remove any item the ADR *lists*. It downgrades or flags five things:

1. **No visible sound control shipped in committed code.** `feedback.js` queries `[data-sfx-toggle]` and the CSS styles it, but when this report was researched no markup rendered such an element. The SPEC-002 summary lists "shell may surface a visible control" as a follow-up. Without it, learners cannot turn UI sounds on, and REQ-014's "one visible mute control" is unmet. A later check of the working tree found an uncommitted edit to `shared/player/core.js` that adds a `[data-sfx-toggle]` button (`#btn-sfx`) to the player header, and an untracked `test/feedback-kit.test.js`. That looks like SPEC-009 work already under way. It covers the player only, so confirm a control also exists where a learner can reach it outside a lesson. Sound stays off by default (P1).
2. **The "miss" tone is downgraded (P2).** `chime('miss')` plays a 220 Hz sine. The only study found on sound valence says to avoid low-valence sounds [10]. Drop it, or make it a neutral short tone. The correct form on screen is the feedback.
3. **The wrong-answer shake is flagged (P3).** `match-wrong-shake` is not in ADR-0010's tactile list. Nothing supports it, and the correction already says what was wrong. Use a static highlight.
4. **The loading pulse is unbounded (B6).** `.is-loading` has `animation: hebrew-audio-pulse 1s infinite` with no timeout, so a stalled fetch pulses past 5 seconds. Cap it. `.is-playing` ends with the clip, which is fine if the learner can stop the audio.
5. **The ADR says "ducks and mutes"; the code only refuses new chimes while speech is active.** That is enough for "never overlap", and a chime is under 120 ms. The wording in ADR-0010 and in the SPEC-002 summary should say "suppresses" so SPEC-009 does not build ducking that was never needed.

Also: the `feedback.js` header cites "REQ-014 / RSCH-007". `RSCH-007` is the new-user ease report, which does not cover sound or motion. The sound and motion rules are this report, `RSCH-008`. I did not edit code.

Not assessed here: the speed-quiz countdown (`.quiz-timer-fill`, 1 s linear transition). It is a timed activity, not decoration. Check it against rule B6 in SPEC-013.

### Not answered

- Whether any feedback animation, flash, or chime changes adult second-language vocabulary recall. The nearest tests are about sound that depicts word meaning [8][9] or about points [10][13].
- Whether labels in the language being learned disambiguate an icon for an English-speaking novice.
- Whether flat, realistic, or cartoon pictures differ for adult second-language recall of concrete nouns.
- Whether a points-free completion screen changes next-day return, or whether ending on an easy item does.
- Whether WCAG 2.2.2 applies to a pulse that a learner's own tap started. The Understanding page says 2.3.3 covers user-initiated motion and 2.2.2 covers automatic motion [1]. The 5-second cap on the loading pulse rests on the operator's rule, not on a reading of 2.2.2.

---

## 2. Confidence

**Overall confidence:** Low

**Justification:** The standards thresholds are High because they are the text of the W3C pages [1][2][3]. The cost of irrelevant extras is Moderate in general lessons [4][5][7], but no study tested them on a vocabulary practice screen, so the transfer is Low. Not one study shows a benefit from feedback animation or UI sound for second-language vocabulary [8][9][10][13]. Icon plus label is Moderate: three sources agree at first use, but two are abstracts and one is a practitioner article [15][16][17]. Picture style has no direct comparison, and the nearest experiment is Moderate only for its own setting [20]. The "finished moment" question has mechanism evidence from lab tasks and no return trial, so it is Low [27][28][30][31]. The effects table is therefore mostly a set of product choices bounded by standards, operator constraints, and general multimedia evidence.

**Coverage, scored per question and not averaged:**

- **Q1 motion and sound: about 0.6, answered with a residual.** Cost evidence is general [4][5][7]. No benefit evidence exists. The L2 sound-effect studies test a different manipulation [8][9].
- **Q2 icon and label: about 0.6, answered with a residual.** Three agreeing sources at first use [15][16][17]. No test with target-language labels.
- **Q3 picture style: about 0.35, mostly an absence.** No direct comparison was found. Adjacent studies are in different settings [20][21][22].
- **Q4 finished moment: about 0.3, mostly an absence.** Mechanism evidence only [27][28][30][31][32]. The only return-rate trial found tests an excluded mechanic [33].

## 3. Verification

**Verified by:** the research worker's pre-finalize self-check against the pages and extracts named in §5. No independent re-derivation was run, and none is claimed. \
**Remediation loops:** 0 \
**Overall verdict:** not independently re-derived in this session. Bands were lowered wherever a source was abstract-only, a single study, or an absence.

**Checks applied before writing:**

- Seductive-details figures (−0.33; −0.48 to −0.18; 58 studies; over 7,500 students; the placement and format moderators) were read in the WSU release and the author's American Educator article [5][6]. The journal abstract confirms the direction and the moderators [4].
- Moreno and Mayer's group results were read in the abstract and the first pages of the paper [7].
- Altmeyer's N = 317 and the null result on performance, affect, immersion, and enjoyment were read in the abstract and opening of the paper [10].
- Berney and Bétrancourt's g values and moderators were read in the abstract [11]. Höffler and Leutner's d values were read in the abstract [12].
- Dziemianko's N = 238, the four conditions, the two-week delayed test, and the line-drawing result were read in the article page [20]. Zhang, Sala, and Gobet's g values were read in the open PDF [22].
- Wiedenbeck's and Leung's results were read in abstracts. Leung's 18 + 18 sample was read in a second source that summarises the same work [16][17][18].
- Finn's and Finn and Miele's results were read in abstracts [27][28]. Xie and Li's N = 32 and N = 56 and their effect sizes were read in the abstract [30]. The Zeigarnik meta-analysis's 0.99 ratio and its 67.00% weighted resumption rate (N = 21 publications) were read in its results text [32].
- The W3C pages were read as search extracts of the Understanding documents and the C39 technique [1][2][3].
- The code claims (no `data-sfx-toggle` producer at research time, the 220 Hz tone, the unbounded `.is-loading` pulse, the shake, the ceremony markup) were read in `shared/player/feedback.js`, `shared/worksheet.css`, `shared/player/core.js`, and `shared/review-page.js`. The first repository search found `data-sfx-toggle` only in `feedback.js` and one CSS rule. A re-check at the end found an uncommitted `#btn-sfx` button in `core.js` (another change in progress). The tone, shake, and loading-pulse findings were unchanged at that re-check.

## 4. Recommended next steps

1. **SPEC-007 and SPEC-009 can adopt the effects table.** Rows A1–A7 are buildable. Rows marked product choice are the operator's to confirm. Rows B1–B10 are not to ship.
2. **SPEC-009 should fix the five reconciliation items:** finish the visible sound control (a player-header button is already in progress), drop or neutralise the miss tone, replace the shake with a static highlight, cap the loading pulse, and align the ADR wording from "ducks" to "suppresses".
3. **SPEC-006 and SPEC-010 should keep the English gloss on primary navigation** until a test shows the Hebrew label alone works for a novice.
4. **If the operator wants return evidence, run a small test of their own:** one cohort with the calm summary and a scheduled next-review line, one without, and next-day return as the measure. None exists in the literature found.
5. **Fix the number in the specs.** SPEC-005, SPEC-007, SPEC-008, SPEC-009 and the INIT-005 initiative call this report "RSCH-007". It is RSCH-008. I did not edit those files.

## 5. References

Tier 1 is a current standard, Tier 2 peer-reviewed or archive-hosted research, Tier 3 practitioner. References marked `[R5 n]` and `[R6 n]` are numbered references in RSCH-005 and RSCH-006, cited as cross-references and not re-read for new numbers. All were retrieved 2026-10-12.

[1] "Understanding Success Criterion 2.3.3: Animation from Interactions." W3C WAI, WCAG 2.2. <https://www.w3.org/WAI/WCAG22/Understanding/animation-from-interactions>. **Tier:** 1. Read as a search extract.

[2] "C39: Using the CSS prefers-reduced-motion query to prevent motion." W3C WAI, WCAG 2.2 techniques. <https://www.w3.org/WAI/WCAG22/Techniques/css/C39>. **Tier:** 1. Read as a search extract.

[3] "Understanding Success Criterion 1.4.2: Audio Control." W3C WAI, WCAG 2.2. <https://www.w3.org/WAI/WCAG22/Understanding/audio-control>. **Tier:** 1. Read as a search extract. The technique G60 (sound that turns off within three seconds) is listed on the same page.

[4] "Keep it Coherent: A Meta-Analysis of the Seductive Details Effect" (Sundararajan and Adesope, 2020). *Educational Psychology Review* 32, 707–734. <https://link.springer.com/article/10.1007/s10648-020-09522-4>. **Tier:** 2. Abstract and references read.

[5] "Seductive details inhibit learning." Washington State University news release, 19 March 2020. <https://news.wsu.edu/press-release/2020/03/19/seductive-details-inhibit-learning/>. **Tier:** 3. Used for the 58-study count, the N, and the moderators; it reports the same meta-analysis as [4].

[6] "Sparking Interest, Reducing Learning?" (Sundar, 2020). *American Educator*, Fall 2020. <https://www.aft.org/ae/fall2020/sundar>. **Tier:** 3. The first author of [4] writing about it. Source for the mean −0.33 and its range, the typical 6–10 minute lab setting, and "especially harmful at the end". Not independent of [4].

[7] "A coherence effect in multimedia learning: The case for minimizing irrelevant sounds in the design of multimedia instructional messages" (Moreno and Mayer, 2000). *Journal of Educational Psychology* 92(1), 117–125. <https://psycnet.apa.org/doiLanding?doi=10.1037/0022-0663.92.1.117>. Full paper: <https://tecfa.unige.ch/tecfa/teaching/methodo/Moreno_Mayer00.pdf>. **Tier:** 2.

[8] "The impact of verbal and nonverbal auditory resources on explicit foreign language vocabulary learning" (Kaplan-Rakowski and Loranc-Paszylk, 2019). *System* 85, 102114. <https://www.sciencedirect.com/science/article/abs/pii/S0346251X18310388>. Preprint: <https://papers.ssrn.com/sol3/papers.cfm?abstract_id=3360419>. **Tier:** 2. Abstract only.

[9] "The impact of relevant environmental sounds on foreign language word learning" (Bellegarda and colleagues, 2025). *Bilingualism: Language and Cognition*. <https://www.cambridge.org/core/journals/bilingualism-language-and-cognition/article/impact-of-relevant-environmental-sounds-on-foreign-language-word-learning/9E7A3FB3FB78E611C36462AF469FE7B1>. **Tier:** 2. The page names Melodie Bellegarda as corresponding author; the full author list was not read.

[10] "Here Comes No Boom! The Lack of Sound Feedback Effects on Performance and User Experience in a Gamified Image Classification Task" (Altmeyer, Hnatovskiy, Rogers, Lessel, and colleagues, 2022). CHI '22. <https://dl.acm.org/doi/fullHtml/10.1145/3491102.3517581>. **Tier:** 2.

[11] "Does animation enhance learning? A meta-analysis" (Berney and Bétrancourt, 2016). Archive ouverte UNIGE. <https://access.archive-ouverte.unige.ch/access/metadata/75bf6876-cb91-4e2c-8ac6-9d0767449449/download>. **Tier:** 2.

[12] "Instructional animation versus static pictures: A meta-analysis" (Höffler and Leutner, 2007). *Learning and Instruction* 17(6), 722–738. <https://www.sciencedirect.com/science/article/abs/pii/S0959475207001077>. **Tier:** 2. Abstract only.

[13] "Gamified feedback in adaptive retrieval practice: Points and progress-bars enhance motivation but not learning" (van den Broek, Scholten, van Thuil, van Rijn, van Gog, and van der Velde, 2026). *Computers in Human Behavior* 177, 108862. <https://research-portal.uu.nl/ws/portalfiles/portal/277804433/1-s2.0-S0747563225003097-main.pdf>. **Tier:** 2.

[14] "Adding Instructional Features That Promote Learning in a Game-Like Environment" (Mayer and Johnson, 2010). *Journal of Educational Computing Research* 42(3). <https://journals.sagepub.com/doi/10.2190/EC.42.3.a>. **Tier:** 2. Abstract only.

[15] "Icon Usability" (Harley, Nielsen Norman Group, 27 July 2014). <https://www.nngroup.com/articles/icon-usability/>. **Tier:** 3.

[16] "The use of icons and labels in an end user application program: An empirical study of learning and retention" (Wiedenbeck, 1999). *Behaviour & Information Technology* 18(2), 68–82. <https://www.tandfonline.com/doi/abs/10.1080/014492999119129>. **Tier:** 2. Abstract only.

[17] "Age-related differences in the initial usability of mobile device icons" (Leung, McGrenere, and Graf, 2011). *Behaviour & Information Technology* 30(5), 629–642. <https://www.cs.ubc.ca/labs/edapt/papers/leung2011.pdf>. **Tier:** 2.

[18] "Improving the Learnability of Mobile Application Interfaces for Older Adults." CHI doctoral summary of the work in [17]. <https://dl.acm.org/doi/pdf/10.1145/1520340.1520443>. **Tier:** 2. The author line was not shown in the extract. Used only for the 18 + 18 sample. Not independent of [17].

[19] "Bad Icons: How to Identify and Improve Them" (Nielsen Norman Group). <https://www.nngroup.com/articles/bad-icons/>. **Tier:** 3. Not independent of [15].

[20] "The usefulness of graphic illustrations in online dictionaries" (Dziemianko, 2021). *ReCALL*. <https://www.cambridge.org/core/journals/recall/article/usefulness-of-graphic-illustrations-in-online-dictionaries/189AC0E82F5E1F66BF8896F00602DF0C>. **Tier:** 2.

[21] "What Types of Illustrations are Effective in Intentional Vocabulary Learning? Simple or Complex? Clear or Unclear?" (Ide and Yanagisawa). *TESL-EJ* 30(1), May 2026. <https://www.tesl-ej.org/pdf/ej117/a5.pdf>. **Tier:** 2. Abstract and parts of the paper read.

[22] "Effectiveness of L1 and pictures in multimedia conditions on learning second-language vocabulary: A meta-analysis" (Zhang, Sala, and Gobet, 2025). *Educational Research Review* 47. Open PDF: <https://researchonline.lse.ac.uk/id/eprint/127744/3/1-s2.0-S1747938X25000181-main.pdf>. **Tier:** 2. Same source as `RSCH-005` [R5 2].

[23] "A second look at Dwyer's studies by means of meta-analysis: the effects of pictorial realism on text comprehension and vocabulary" (Reinwein and Huberdeau, 1997). ERIC ED407671. <https://files.eric.ed.gov/fulltext/ED407671.pdf>. **Tier:** 2.

[24] "Meta-analysis of emotional design: adding anthropomorphic faces and pleasant colors to multimedia graphics" (Brom and colleagues, 2018, *Educational Research Review* 25, 100–119, as cited in [25]). Author manuscript: <https://artemis.ms.mff.cuni.cz/main/papers/emodesign_manuscript_1rev_180928_FINAL_norev.pdf>. **Tier:** 2. The manuscript title and full author list were not read.

[25] "Meta-Analysis of Emotional Designs in Multimedia Learning: A Replication and Extension Study" (2021). *Educational Psychology Review*. ERIC EJ1295978. <https://eric.ed.gov/?id=EJ1295978>. **Tier:** 2. Authors named as Wong and Adesope in a later review; the record read did not print the author line.

[26] "Benefits of emotional design in multimedia instruction" (Mayer and Estrella, 2014). *Learning and Instruction*. <https://tecfa.unige.ch/tecfa/teaching/methodo/MayerEstriella2014.pdf>. **Tier:** 2.

[27] "Ending on a high note: Adding a better end to effortful study" (Finn, 2010). *Journal of Experimental Psychology: Learning, Memory, and Cognition* 36(6), 1548–1553. <https://pmc.ncbi.nlm.nih.gov/articles/PMC2970645/> (direct fetch returned 403; abstract read through a search extract and <https://www.ovid.com/00004786-201012000-00017>). **Tier:** 2.

[28] "Hitting a high note on math tests: Remembered success influences test preferences" (Finn and Miele, 2015). *Journal of Experimental Psychology: Learning, Memory, and Cognition*. <https://pubmed.ncbi.nlm.nih.gov/26213835/>. **Tier:** 2. Abstract only.

[29] "Remembered Utility of Unpleasant and Pleasant Learning Experiences: Is All Well that Ends Well?" (2012). *Applied Cognitive Psychology*. <https://onlinelibrary.wiley.com/doi/10.1002/acp.2890>. **Tier:** 2. Abstract only; the authors were not shown in the extract.

[30] "The Peak–End Rule and Retrospective Emotional Valence in Digital Learning Tasks: Evidence from a Word-Learning App" (Xie and Li, 2026). *Behavioral Sciences* 16(5), 779. <https://pmc.ncbi.nlm.nih.gov/articles/PMC13203181/>. **Tier:** 2. Abstract only.

[31] "Mental effort and discomfort: Testing the peak-end effect during a cognitively demanding task" (Hsu, Propp, Panetta, and colleagues). Index page: <https://exa.ai/library/publication/5lgd0cmymh2>. **Tier:** 2. Abstract only; the venue and year were not shown.

[32] "Interruption, recall and resumption: a meta-analysis of the Zeigarnik and Ovsiankina effects" (Ghibellini and Meier, 2025). *Humanities and Social Sciences Communications*. <https://www.nature.com/articles/s41599-025-05000-w>. **Tier:** 2. Open access; the abstract and results text were read.

[33] "Contests, gamification, and persistent engagement in educational technology." Randomised trial, 10,000 students on an English-learning app in India. Index page: <https://exa.ai/library/publication/xgk70dpqskk>. **Tier:** 2. Abstract only; the authors and venue were not shown.

---

## Appendix A — Access-Gate Result

**What was checked:** The research skill's access-gate binding-verify block was not re-run in this session. This report was produced as the work of INIT-005/SPEC-005, whose security review is `light` and which handles no learner data. The scope below is the narrowest one that fits the work.

**Result:** `NARROW`

**Scope narrowing:** Public web sources only. `.env`, `data/`, and learner records were not opened. Sources behind a login, paywall, or a 403 were not opened with a credential. Where a publisher showed a public abstract, only the abstract was used. The local repository was read for exactly one purpose: to reconcile the effects table with what ADR-0010 and INIT-007/SPEC-002 already ship. Those local files were read as code and records, not as evidence for any research claim.

---

## Appendix B — Scope & Research Type

**Research type selected:** `prior_art_review` — `findings_pattern`: `narrative_bluf`

**Why this type:** The request asks what published precedent says about four interface questions, and where a product choice has to be labelled as one. That is a precedent survey with evidence grades, not a market study.

**Research Index duplicate check:** RSCH-003 (gamification Low, retry prompts), RSCH-005 (pictures in vocabulary learning, AI image checking), RSCH-006 (multimedia principles, feedback form, WCAG 2.2, points and streaks), and RSCH-007 (new-user ease, icon plus label in navigation) overlap. None answers the four questions here. They are cross-referenced and not re-derived. RSCH-007 already grades icon-plus-label navigation Moderate; this report adds the controlled studies behind that rule [16][17] and the language-of-the-label gap.

**Number resolution.** SPEC-005 (assumption aud-1), SPEC-007, SPEC-008, SPEC-009 and the INIT-005 initiative say "RSCH-007". The Research Index already lists RSCH-007 as the new-user ease report, and ADR-0010 and INIT-007 cite that number for it. The next free number is RSCH-008, which that report's own reference [19] also names for this session. This report is `RSCH-008`. The SPEC-005 file was updated. SPEC-007, SPEC-008, SPEC-009 and the initiative text were not edited, and their "RSCH-007" references should read RSCH-008.

**Scope statement:** Four questions, as required by INIT-005/SPEC-005: (1) animated or sound feedback in second-language vocabulary practice; (2) icon plus label versus icon alone for adult novices; (3) vocabulary picture style and recall; (4) a points-free "finished" moment and return to study. Operator constraints applied: no points, streaks, levels, or leaderboards; no jokes or mascot; UI sounds off by default with a mute control and never overlapping Hebrew word audio; `prefers-reduced-motion` honoured; nothing self-running over 5 seconds without a pause.

**Near-misses (not studied):** whether points or streaks work (excluded by the operator; RSCH-003 and RSCH-006 hold the Low grade); logo and icon-set design (SPEC-006); typography and contrast (RSCH-006); first-run onboarding (RSCH-007).

---

## Appendix C — Methodology

**Corpus doc used:** none loaded in this session. The report follows the structure and evidence-label style of `RSCH-006`.

**Harness:** cursor.

**Tools used:** WebSearch and WebFetch for public sources. Read, Grep, and file reads for the repository (spec, initiative, ADR-0010, RSCH-005, RSCH-006, RSCH-007, `shared/player/feedback.js`, `shared/worksheet.css`, `shared/player/core.js`, `shared/review-page.js`). Fetched pages were treated as data. No instruction inside a page was followed.

**Iteration budget:** 25 web calls: 22 searches and 3 page fetches (one timed out, one returned 403, one succeeded). That is above the 15–20 guideline used in RSCH-006, because there were four questions and two of them (picture style, return to study) needed a second and third pass before concluding that nothing direct exists.

**Band interpretation:** A current standard's own text supports High. A peer-reviewed meta-analysis or a large primary study supports Moderate for the setting it tested. A single ordinary study, an abstract-only source, an unresolved conflict, or an absence supports Low or "no evidence". An inference or a transfer to this screen is labelled adapted or product choice. Sources that share an author, or one that summarises another, are not counted as independent: [4][5][6] are one result, [17][18] are one result, and [15][19] are one source.

**Sources read but not used, or dropped:**

- A quasi-experimental study of sound feedback in Quizizz (62 Indonesian high-school students) was seen. It compared the platform with sound against teacher-led instruction, so sound is not isolated. Not used.
- A study of sound-effect-supported learning in a serious game (n = 23) rated user experience, not learning. Not used.
- A text-versus-graphic interface comparison for an art database (Dillon) compares text-only with icon-based navigation, not icon-only with icon-plus-label. Not used.
- A children-with-autism study of colour photographs versus black-and-white cartoons was seen and not applied (different population and age).
- Duolingo's own white paper describes rewards, streaks, and animated characters after each exercise. It is a vendor document with no data on a points-free design. Not used as evidence.
- Studies of points and gamification with delayed-effect or quasi-experimental designs in EFL classes were seen. They concern excluded mechanics and are not cited beyond [13] and [33].

**Disconfirmation notes:**

- **Sound and animation.** The search looked for evidence that feedback sound or animation helps vocabulary learning. It found word-meaning sound effects that helped in one study [8], a null for sound on performance [10], and animation benefits tied to dynamic content [11][12]. No feedback-effect benefit was found, so the grade stays at "no evidence".
- **Icon-only.** The search looked for evidence that icon-only is as good as icon-plus-label. It found that icon-only caught up after a second session [16]. First-use labels still win.
- **Picture style.** The search looked for a direct flat versus realistic versus cartoon test. None was found. The nearest result favours line drawings in one setting [20].
- **Finished moment.** The search looked for a trial of a completion screen and return. None was found. It found a large trial of contest mechanics that did raise lasting engagement [33], which is recorded as a counter-signal, not hidden.

**Injection attempts:** None observed in the fetched pages examined.

**Provenance:** `INIT-005/SPEC-005`
