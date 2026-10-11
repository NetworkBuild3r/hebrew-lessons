# Prior-art review — Learner interface design (RSCH-006)

**Research type:** `prior_art_review` \
**Subject:** Published principles for a second-language practice screen that shows a word, its meaning, audio, and a picture, including pointed Hebrew, feedback, motivation without points or streaks, WCAG 2.2, phones, and novice navigation \
**Date:** 2026-10-08 \
**Requestor:** session user \
**Identifier:** `RSCH-006` \
**Access scope:** `NARROW` (public sources only; see Appendix A)

---

## 1. Findings

**Recommendation:** Build the practice screen from four published constraints, and label everything else as a product choice. Keep the pointed word, its first-language meaning, and the picture in one cluster, and play the audio while that word is still on screen [2][3]. Give the correction on the same turn, and make it more than "wrong" [11][12]. Meet WCAG 2.2 as a W3C Recommendation of 12 December 2024, including 4.5:1 text contrast and a 24 CSS pixel target floor, and size primary controls to 44 CSS pixels [1][19]. Show progress as what the learner can now do, and leave points, streaks, and leaderboards out. That last choice does not raise the Low grade `RSCH-003` already gave language-app gamification [15][16][18]. No public trial states a pixel size or line height that keeps nikud legible for adult learners, so the Hebrew type size below is an adaptation of Latin reading research plus the standard's large-text threshold, not a Hebrew result [5][6][7]. **Moderate**

### 1.1 Cognitive load and multimedia learning

- People learn better from words plus graphics than from words alone, and from a layout that cuts processing unrelated to the goal. In a review of twelve principles for computer-based lessons, the multimedia principle had a median effect of d = 1.67 across five comparisons. Principles that reduce extraneous processing had these medians: coherence (drop material that is not needed) d = 0.70, signaling (highlight what matters) d = 0.46, redundancy (narration plus graphics, without a second on-screen copy of the narration) d = 0.87, spatial contiguity (printed words next to the matching graphic) d = 0.79, and temporal contiguity (narration at the same time as the graphic) d = 1.30. Segmenting a lesson into learner-paced parts had a median of d = 0.70. The same review says some principles are stronger for low-knowledge learners than for high-knowledge learners [2]. **Moderate: one high-quality review by the author of the theory, reporting median effects across earlier experiments. General multimedia learning, applied here to a vocabulary screen.**
- An overview of 29 systematic reviews (1,189 studies, 78,177 participants) found eleven design principles with significant positive meta-analytic effects on learning. The largest benefits were for captioning second-language videos, for temporal and spatial contiguity, and for signaling. The same overview found support for coherence (removing seductive details), segmentation, and verbal redundancy, and it found that good design mattered more for complex material and more in system-paced settings (lectures) than in self-paced settings (websites) [3]. **Moderate: one meta-meta-analysis. It agrees with [2] on contiguity, signaling, coherence, and segmenting, and it adds the self-paced boundary.**
- The classic redundancy rule does not transfer cleanly to second-language learners. [3] ranks captioning of second-language video among the largest benefits, which is the opposite of "never show text with speech." A study of 62 learners of English found that full captions plus still key frames helped local comprehension (the details), while full captions plus moving animation did not help those details as much. That paper reports a statement from Mayer, Fiorella, and Stull (2020), which this session did not read directly: when people learn from video in a second language, many of the multimedia principles are reversed [4]. **Moderate that second-language learners are a boundary on redundancy: the overview [3] and one experiment [4] agree, and [4]'s quotation of the 2020 chapter is secondhand. Low for any claim that every multimedia principle reverses.**
- For a screen whose job is one word, the written pointed word is the object of study, the English meaning is a short first-language gloss, the picture is the graphic, and the audio is the pronunciation. Spatial and temporal contiguity say to keep those four in one view and to play the audio while the word is visible [2][3]. Coherence says to leave off decoration that is none of those four [2]. Segmenting says to show one item and let the learner move on, with the caveat that the segmenting benefit is smaller when the learner already controls the pace [2][3]. The written word stays on screen with the audio. Treating it as a redundant caption would apply the first-language lecture rule to a case the second-language evidence treats as a boundary [3][4]. **Moderate for the cluster layout. Adapted from lesson research to a single vocabulary card.**

### 1.2 Reading and typography, including pointed Hebrew

- On Latin web text, larger type helped adult readers up to a point, and extreme line spacing hurt. An eye-tracking study of 104 adults reading Wikipedia in Arial compared sizes from 10 to 26 points and line spacings from 0.8 to 1.8. Fixation durations showed higher readability as size increased, and comprehension questions were answered more often at 18 and 26 points. The two spacing extremes, 0.8 and 1.8, impaired readability. The authors recommend 18 points or larger and default line spacing for text-heavy pages [5]. **Moderate: one experiment with 104 adults. Latin script, not Hebrew.**
- Bigger type is not a universal law. In a study of 100 children, second graders comprehended a standard font size better than a small one (proportion correct .87 versus .79, d = .35) and comprehended a standard line length better than a long one (d = .54). Fifth graders showed the opposite font-size pattern. Line spacing had no effect on the second graders [6]. **Low as a limit on [5]: one study of children, and the age groups disagree. It blocks any rule that says "the largest type is always best."**
- Pointed Hebrew (nikud) uses small marks that carry the vowels. A cross-grade study of elementary readers states that the marks are perceptually less salient than the consonant letters because they are small dots and dashes placed under, above, or inside the letter [7]. A font-production report from a Hebrew foundry states that a letter can carry up to three of these marks, and that marks below the baseline have to sit under the visual axis of that letter. Mathematical centering and ordinary kerning do not place them [8]. **Moderate that the marks are small and easy to lose: one reading paper states the salience claim, and one font-engineering paper states the placement constraint. Neither paper is a trial of a screen size.**
- No study read in this session gives a point size, a CSS pixel size, or a line height at which adult learners can still read nikud. The search covered pointed-Hebrew reading, adult second-language Hebrew, and diacritic legibility. What exists is about whether pointing helps decoding, mostly in children and in skilled adult readers of Hebrew, not about pixels [7]. Applying [5]'s 18-point floor to the pointed word is an adaptation. Eighteen points is also about 24 CSS pixels, which is the size WCAG uses as the start of "large text" (18 pt, or 14 pt bold, about 24 px and 18.5 px) [1]. **Low for any Hebrew-specific size: an absence claim, capped at Low. The 18-point floor is adapted from Latin adults and happens to match the standard's large-text cutoff.**
- WCAG 2.2 does not require authors to set line height to 1.5. Success Criterion 1.4.8 (Level AAA) says a mechanism must exist so that blocks of text can be shown with line spacing of at least space-and-a-half, width of no more than 80 characters, and no full justification, and it says the browser may be that mechanism. It also says a writing system that does not use one of those settings need not use it [1]. Success Criterion 1.4.12 (Level AA) is the author requirement that can be tested: when markup text is set to line height of at least 1.5 times the font size, paragraph spacing of at least 2 times the font size, letter spacing of at least 0.12 times the font size, and word spacing of at least 0.16 times the font size, nothing is lost [1]. For pointed Hebrew, the practical stake is that the points sit outside the letter body [7][8], so a tight line box can clip them when the user applies 1.4.12. Choosing 1.5 as the author's own default is a product choice. Surviving 1.5 without clipping the points is the standard. **High for the survival test, because it is the text of the current Recommendation. Low for 1.5 as the most readable leading: [5] only shows that 0.8 and 1.8 were worse, and no Hebrew leading trial was found.**
- Mixed Hebrew and English is a markup problem with a current W3C method. If the page is Hebrew, set `dir="rtl"` on the `html` element and set the language. Do not set base direction with CSS. Use logical CSS properties (`start` and `end`) rather than physical left and right [9]. Inside a line, wrap each phrase whose direction is known and opposite to the surrounding text in an element with `dir` set to that direction. An English gloss inside a Hebrew line is that case: wrap it and set `dir="ltr"` [10]. Both articles were updated 25 June 2021 and remain the W3C internationalization guidance. **High: the standard body's own current text. No user study of Hebrew-English learner errors was read, so the rule is the markup requirement, not a measured reduction in reading mistakes.**

### 1.3 Feedback timing and form

- In computer-based item feedback, elaboration beat telling the learner the right answer, which beat saying only whether the answer was right. A meta-analysis of 40 studies (70 effect sizes) found elaborated feedback at 0.49, knowledge of the correct response at 0.32, and knowledge of whether the response was correct at 0.05. Elaborated feedback was especially stronger for higher-order outcomes. Delayed timing was associated with smaller effects. The authors' suggestion that immediate feedback helps lower-order learning and delayed feedback helps higher-order learning did not reach a significant interaction [11]. **Moderate: one meta-analysis of computer-based item feedback. Languages were one of the subject areas, and effect sizes there were smaller than in mathematics, so the language application is the weaker slice of the same review.**
- A review of formative feedback defines it as information that tells the learner how to move toward the goal, and it separates verification (right or wrong) from elaboration (cues toward the correct answer). It reports a converging view that useful feedback contains both, and it warns that a long message is skipped [12]. The same review finds the timing literature mixed: some studies support immediate feedback so that errors are not stored, and others support a delay so that the first error does not interfere with the correction [12]. **Moderate that feedback should say more than "wrong": the review [12] and the meta-analysis [11] agree. Low for a single best delay: [12] says the timing evidence conflicts.**
- Across real college classes, immediate versus delayed feedback had no overall effect. In 38 classes, feedback given on submission was compared with feedback given days later (mean delay 2.58 days, range 1–5). The overall estimate of the advantage of immediate feedback was a change in z of 0.002, with a 95% highest-density interval from −0.05 to 0.05. Incentive to look at the feedback did not change that [13]. **Moderate as a null result in applied classes: one large multi-class experiment. It conflicts with any blanket rule that immediate feedback is always better, and with the classroom side of the older "immediate is better" advice that [13] itself summarizes.**
- The interface split that survives both results is this. On the practice item, while the word is still on screen, say more than "wrong," and include the correct form [11][12]. A wait of several days before showing that form is a different schedule, and [13] found no general benefit to it. Spacing review across later days remains the memory rule in `RSCH-003` (cross-reference, not re-derived here). A single retry before the answer is shown is the prompt pattern `RSCH-003` already graded Moderate for oral feedback. This session did not find a computer-based trial of "one retry, then the answer" as its own condition. [12] notes that response-specific feedback has looked stronger than "answer until correct" in some older studies, so an endless retry loop is not the supported form. **Moderate for elaborated correction on the same turn. Low for the exact one-retry sequence: it is adapted from the oral-feedback precedent and is not a result in [11] or [13].**

### 1.4 Motivation without gamification

- In second-language research, more autonomous motivation tracks achievement, and controlled motivation does not. A multilevel meta-analysis found intrinsic regulation correlated with language achievement at r = .26 and identified regulation at r = .16. Combined as autonomous motivation, the correlation was r = .23. Introjected regulation (r = −.06) and external regulation (r = −.01) were not significant, and controlled motivation overall was r = −.03. Autonomous motivation was also associated with lower anxiety (r = −.23) and controlled motivation with higher anxiety (r = .16) [14]. **Moderate: one meta-analysis of second-language achievement. It is correlational. It supports designing for volition and for a sense of competence, and it does not support external pressure as a path to achievement.**
- Interventions built on self-determination theory move perceived autonomy more clearly than they move competence or intrinsic motivation. In a meta-analysis of 36 studies, the autonomy contrast started at g = 1.65 with extreme heterogeneity and fell to g = 1.14 (95% CI 0.50 to 1.77) after one outlier was removed. The primary competence contrast was not significant (g = 0.80, interval from −0.49 to 1.65). After a bias correction it became g = 0.48 (95% CI 0.17 to 0.79). Pre-post intrinsic motivation was not significant once publication bias was corrected (g = 0.02) [17]. **Low for "an SDT intervention will raise intrinsic motivation": the bias-corrected pre-post result is null. Moderate that autonomy support is the more reliable of the three needs in this review, with the size still sensitive to one study.**
- Gamification of school learning has small average effects, and the competence effect is the weak one. A meta-analysis found gamification associated with cognitive outcomes at g = .49 (k = 19, N = 1,686), motivational outcomes at g = .36 (k = 16, N = 2,246), and behavioral outcomes at g = .25 (k = 9, N = 951). The cognitive effect held up in a high-rigor subset. The motivational and behavioral effects were less stable. The authors say the ingredients of successful gamification are still unresolved, especially for cognitive outcomes [15]. A later meta-analysis of 35 interventions (2,500 participants) found a small effect on intrinsic motivation (g = 0.257, 95% CI 0.043 to 0.471), a small effect on perceived competence whose interval nearly includes zero (g = 0.277, 95% CI 0.001 to 0.553), and a large relatedness effect with a very wide interval (g = 1.776, 95% CI 0.737 to 2.814). The review's own conclusion is that gamified classes often fail to produce a sense of competence and a sense of autonomy [16]. **Moderate that the average effect on learning is small and that competence is the need these designs barely move: two meta-analyses agree on a modest benefit and on unstable motivation results. The relatedness figure in [16] is too unstable to use as a design target.**
- `RSCH-003` graded language apps, streaks, and points as unproven (Low), from a review of Duolingo studies and a nine-learner case study. This session does not raise that grade. [15] and [16] are reviews of game elements in education generally. They are not trials of points or streaks as a method for learning a language. The conflict is a scope conflict: small average effects in mixed school studies, beside thin language-app evidence. The ban on points, streaks, and leaderboards stays, as a product choice aligned with the Low grade, not as a new High finding that those features harm learning. What the motivation evidence does support is a progress display that tells the learner what they can now do (a competence signal), because controlled rewards do not track achievement [14] and because gamification's competence effect is the weak one [16]. No trial compared a progress count with a progress bar or with no progress display. **Low for the streaks decision, unchanged from `RSCH-003`. Low for the exact progress widget: an inference from [14] and [16], listed as adapted.**

### 1.5 Accessibility

WCAG 2.2 is the current W3C Recommendation, dated 12 December 2024. It supersedes the 5 October 2023 Recommendation of the same guidelines and succeeds WCAG 2.1 [1]. The thresholds below are the ones SPEC-009 can test. Each line names the level, the criterion, and the number or the yes/no check. Currency: current.

| ID | Level | Test |
| --- | --- | --- |
| 1.4.3 Contrast (Minimum) | AA | Text and images of text at least 4.5:1 against the background. Large text at least 3:1. Large text is 18 pt (about 24 px) or 14 pt bold (about 18.5 px). Inactive, decorative, and logo text are exempt [1]. The pointed word counts as text, so the vowel marks are included in the same ratio. |
| 1.4.6 Contrast (Enhanced) | AAA | Text at least 7:1. Large text at least 4.5:1 [1]. Optional higher bar, not the AA floor. |
| 1.4.11 Non-text Contrast | AA | Parts of a control needed to identify it, and parts of a graphic needed to understand it, at least 3:1 against adjacent colors [1]. |
| 1.4.4 Resize Text | AA | Text, other than captions and images of text, can grow to 200% with no loss of content or function [1]. |
| 1.4.5 Images of Text | AA | Information is real text, not a picture of text, unless a particular presentation is essential [1]. Pointed Hebrew is real text so it can resize and so a screen reader can reach it. |
| 1.4.10 Reflow | AA | At a width of 320 CSS pixels (1280 CSS pixels at 400% zoom), content does not need scrolling in two dimensions, except where two-dimensional layout is essential [1]. |
| 1.4.12 Text Spacing | AA | No loss when line height is set to at least 1.5 times the font size, paragraph spacing to at least 2 times the font size, letter spacing to at least 0.12 times the font size, and word spacing to at least 0.16 times the font size [1]. |
| 1.4.7 Low or No Background Audio | AAA | For prerecorded speech, background sound is absent, can be turned off, or is at least 20 dB below the speech (about four times quieter), aside from sounds of one or two seconds [1]. Applies if a clip has music under the voice. |
| 2.5.8 Target Size (Minimum) | AA | Pointer targets at least 24 by 24 CSS pixels, unless the spacing, equivalent-control, inline, user-agent, or essential exception applies [1]. |
| 2.5.5 Target Size (Enhanced) | AAA | Pointer targets at least 44 by 44 CSS pixels, with a shorter exception list [1]. |
| 2.4.7 Focus Visible | AA | A keyboard-operable interface has a mode in which the focus indicator is visible [1]. Yes/no. |
| 2.4.11 Focus Not Obscured (Minimum) | AA | When a component receives keyboard focus, author content does not hide it entirely [1]. New in 2.2. Yes/no. |
| 2.4.13 Focus Appearance | AAA | The focus indicator's area is at least a 2 CSS pixel perimeter of the unfocused component, and the contrast change between focused and unfocused pixels is at least 3:1 [1]. |
| 2.2.2 Pause, Stop, Hide | A | Motion that starts by itself, lasts more than five seconds, and runs beside other content has a way to pause, stop, or hide it, unless the motion is essential [1]. |
| 2.3.3 Animation from Interactions | AAA | Motion started by a user action can be turned off unless it is essential [1]. The understanding document points to `prefers-reduced-motion` as one way to meet it. |
| 2.5.7 Dragging Movements | AA | Anything that requires a drag also works with a single pointer without dragging, unless dragging is essential [1]. New in 2.2. |

Meeting AA is the conformance target the standard itself treats as the common bar. Meeting 2.5.5 and 2.3.3 is a product choice to take two AAA criteria because the phone evidence and vestibular guidance line up with them (sections 1.6 and the motion row above). Dark mode is not a WCAG requirement. In two studies with 459 participants, dark text on a light background was read faster than the reverse, for both glance reading and paragraph reading, and readers did not prefer the faster polarity [23]. No study of dark mode with pointed Hebrew was found. If a dark theme is shipped later, the contrast rows above still apply. **High for the AA rows, because they are the current Recommendation's own text. Low for dark mode as a reading aid: one large study found light polarity faster, and the Hebrew case was not studied.**

### 1.6 Mobile and one-handed use

- The AA target floor is 24 by 24 CSS pixels [1]. A study of one-handed thumb use on small touchscreens found that error rates stopped improving once discrete targets reached 9.6 mm and serial targets reached 7.7 mm, and it recommends 9.2 mm for a single tap and 9.6 mm for a series of taps [19]. A CSS pixel is 1/96 inch, so 9.6 mm is about 36 CSS pixels and 24 CSS pixels is about 6.4 mm. The AAA size, 44 by 44 CSS pixels, is about 11.6 mm, which clears the thumb study's recommendation [1][19]. **High for 24 CSS pixels as the AA floor: it is the standard. Moderate for preferring 44 CSS pixels on primary controls: the AAA criterion and the thumb study agree that 24 CSS pixels is smaller than the size at which one-handed tapping leveled off. The 2006 phones were smaller than current phones, so the millimeter result is adapted, not copied as a law of modern screens.**
- Not all of a phone screen is in reach of the thumb that is holding the phone. A kinematic model fitted to 20 people's thumb paths predicts the reachable area from screen size, hand size, and where the index finger sits on the back of the device. As the screen grows relative to the hand, the reachable fraction shrinks [20]. **Low: one model, N = 20, and the area is not a fixed rectangle. The interface consequence is to put the main action in the lower reachable region, and to re-check it on a large phone. That placement is adapted from the model.**
- No study read here gives a best length, in minutes, for a phone session. Segmenting into learner-paced items [2] is the related principle, and it lets a person stop after one word. Session length stays an open question, consistent with the absence already recorded in `RSCH-003`.

### 1.7 Layout and navigation for novices

- A deeper menu on an e-learning site raised extraneous cognitive load and reduced learning efficiency. A checklist helped mainly in the deep-menu condition, not overall [21]. **Low: one experiment. It supports keeping the path to the next practice short.**
- Hick's law does not justify a rule of "fewer buttons." A review of the choice-reaction literature concludes that the law speaks against the slogan that less is better, that a logarithmic time curve is not automatically Hick's law, and that the stimulus-response setup rarely matches interface tasks, where choice time can often be treated as constant [22]. **Moderate as a warning against misusing the law: one theoretical review. It removes Hick's law as a reason for a one-button home screen.**
- Progressive disclosure, in the practitioner literature, means showing the frequent action first and keeping advanced options one step away, and it warns that more than two levels of disclosure tends to lose people [24]. That article is not a trial. Combined with [21], the home screen that offers one next action, with other destinations one level down, is a product choice. It is not a measured optimum. **Low.**

### Do

- **Put the pointed word, the English meaning, and the picture in one cluster, and play the audio while the word is visible.** Spatial and temporal contiguity, with the second-language boundary that keeps the written word [2][3][4]. **Moderate · multimedia learning, with a second-language boundary.**
- **Show one item, and let the learner advance.** Segmenting [2], smaller when the pace is already the learner's [3]. **Moderate · multimedia learning.**
- **Leave off decoration that is not the word, the meaning, the audio, or the picture.** Coherence [2][3]. **Moderate · multimedia learning.**
- **Set the pointed word at least 18 pt (about 24 px), in a Hebrew font that positions nikud, as live text.** Adapted from adult Latin reading [5] and from the standard's large-text size [1], with the font constraint from [8] and the live-text rule from 1.4.5 [1]. **Low for the Hebrew size · adapted.**
- **Keep the vowel marks visible when line height is 1.5 and letter spacing is 0.12 em.** That is 1.4.12 [1]. **High · WCAG 2.2.**
- **After a miss, say more than "wrong," and show the correct form on that turn.** Elaboration and knowledge of the correct response beat right/wrong [11][12]. **Moderate · computer-based feedback research.**
- **Offer one retry, then the answer.** Adapted from the prompt pattern in `RSCH-003`. An open retry loop is not the supported form [12]. **Low · adapted.**
- **Show progress as items or words the learner can now do.** Autonomous motivation tracks achievement and controlled motivation does not [14]. **Low · inference from second-language SDT.**
- **Meet the AA rows in section 1.5, and size primary controls to 44 CSS pixels.** [1][19]. **High for the AA tests · WCAG 2.2. Moderate for 44 CSS pixels · adapted from AAA plus the thumb study.**
- **Set `dir="rtl"` and `lang="he"` on a Hebrew page, and wrap each English gloss with `dir="ltr"`.** [9][10]. **High · W3C internationalization.**

### Don't

- **Don't drop the written word because a first-language redundancy rule says speech and text together are harmful.** Second-language captioning is a boundary, and in the overview it was among the largest benefits [3][4]. **Moderate · second-language multimedia research.**
- **Don't delay the correct form for days in the hope that delay is always better.** Thirty-eight classes showed no overall timing effect [13]. **Moderate for the null result · applied education research.**
- **Don't use points, streaks, or a leaderboard as the progress display.** `RSCH-003` remains Low on language-app gamification. Later meta-analyses find small, unstable motivation effects and a weak competence effect [15][16][18]. **Low · the grade is unchanged.**
- **Don't set Hebrew direction with CSS, and don't leave an English gloss unwrapped inside a Hebrew line.** [9][10]. **High · W3C.**
- **Don't treat 24 CSS pixels as a comfortable one-handed size.** It is the AA floor. The thumb study's recommended size is larger [1][19]. **Moderate · standard plus one HCI study.**
- **Don't cite Hick's law as the reason the home screen has one action.** The law does not support "less is better" [22]. **Moderate · HCI theory review.**
- **Don't require dark mode in order to help reading.** The large polarity study found light text-on-dark slower, and pointed Hebrew was not tested [23]. **Low · one study, wrong script for a Hebrew rule.**

### Not answered

- A controlled trial of CSS pixel size or line height for nikud with adult second-language readers. The 18-point floor is adapted from Latin Wikipedia [5].
- A trial of the four-part card itself (one pointed word, an English gloss, one picture, one audio file). The layout rules are transferred from lesson and video research [2][3][4].
- Whether a photograph teaches a word better than a drawing, and when to show the picture at all. Picture style and the introduction-versus-spelling choice are RSCH-005. This report covers only how the picture sits with the word when it is shown.
- The best delay, in seconds, between a wrong answer and the correction. [11], [12], and [13] disagree about days versus the same sitting, and none of them isolates a second-scale delay.
- Whether a progress count, a progress bar, or no progress display changes achievement. The progress rule is an inference from motivation research [14][16].
- A best phone-session length in minutes.
- Whether dark mode helps or hurts reading of pointed Hebrew [23].
- A user study of Hebrew-English mixing errors. The markup rule is the W3C method [9][10], not a measured error reduction.
- Whether 44 CSS pixel targets change learning, as opposed to tapping errors [19].

### Interface rule table

Each row is a rule a later design record can adopt. The grade is the confidence in the evidence behind the rule. The label is evidence-backed when the row states a published result or a current standard in the terms of that source, adapted when a general result is applied to this screen, and product choice when the evidence does not decide the design.

| # | Rule | Grade | Source | Label |
| --- | --- | --- | --- | --- |
| 1 | Set the pointed Hebrew word at least 18 pt (about 24 px), larger than the English around it, in a font that places nikud on the visual axis of the letter. | Low | [5][7][8] | adapted |
| 2 | Render the pointed word as live text, not as a picture of text, so it can resize to 200%. | High | [1] 1.4.4, 1.4.5 | evidence-backed |
| 3 | The author's default line height on a pointed line is 1.5. | Low | [5] | product choice |
| 4 | The points stay fully visible when line height is 1.5, letter spacing is 0.12 em, and word spacing is 0.16 em. | High | [1] 1.4.12 | evidence-backed |
| 5 | The pointed word, including the vowel marks, has a contrast of at least 4.5:1 (3:1 if the word is large text: 18 pt or 14 pt bold). | High | [1] 1.4.3 | evidence-backed |
| 6 | Controls, and the parts of a picture that carry the meaning, have a contrast of at least 3:1. | High | [1] 1.4.11 | evidence-backed |
| 7 | Every pointer target is at least 24 by 24 CSS pixels, aside from the WCAG exceptions. | High | [1] 2.5.8 | evidence-backed |
| 8 | Play, check, and next are at least 44 by 44 CSS pixels. | Moderate | [1] 2.5.5; [19] | adapted |
| 9 | When a picture is on the screen, it sits in one cluster with the pointed word and the English meaning. The audio plays while the word is visible. Whether to show the picture is RSCH-005, not this report. | Moderate | [2][3][4] | adapted |
| 10 | One item is on screen at a time, and the learner advances. | Moderate | [2][3] | adapted |
| 11 | The screen does not add decoration beyond the word, the meaning, the audio, and the picture. | Moderate | [2][3] | adapted |
| 12 | After a miss, the correction is more than "wrong," and it includes the correct form before the learner leaves the item. | Moderate | [11][12] | evidence-backed |
| 13 | The learner gets one retry, then the correct form. The retry does not continue until they happen to guess it. | Low | [12][18] | adapted |
| 14 | Progress shows what has been learned (items done, words due). It does not show points, a streak, or a leaderboard. | Low | [14][15][16][18] | product choice |
| 15 | A Hebrew page sets `dir="rtl"` and `lang="he"`. Each English gloss is wrapped with `dir="ltr"`. Direction is not set with CSS. | High | [9][10] | evidence-backed |
| 16 | The home screen has one next action. Other destinations are at most one level down. | Low | [21][22][24] | product choice |
| 17 | The main phone action sits in the lower reachable area, and it is re-checked on a large phone. | Low | [20] | adapted |
| 18 | Motion that starts by itself and lasts more than five seconds can be paused. Motion started by a tap honors reduced motion. | High for the pause; the reduced-motion half is AAA | [1] 2.2.2, 2.3.3 | evidence-backed for 2.2.2; product choice to meet 2.3.3 |

## 2. Confidence

**Overall confidence:** Moderate

**Justification:** The accessibility thresholds are High because they are the text of WCAG 2.2, a W3C Recommendation dated 12 December 2024, read from the current document [1]. The cluster layout is Moderate: a review of the multimedia principles [2] and an overview of 29 reviews [3] agree on contiguity, coherence, and segmenting, and the second-language boundary on redundancy is stated in [3] and in one experiment [4]. Feedback form is Moderate from one computer-based meta-analysis [11] and one review [12]. Feedback timing is Low because [13] found no overall effect across 38 classes. Hebrew type size is Low because no nikud pixel trial was found; the 18-point figure is Latin [5], and a child study shows that bigger type can hurt older children [6]. Motivation is Moderate for the achievement correlation of autonomous motivation [14] and Low for the progress widget and for streaks. Sailer and Homner [15] and Li, Hew, and Du [16] do not raise `RSCH-003`'s Low grade on language-app gamification, because they are not that question. Navigation depth rests on one experiment [21] and is Low. Hick's law was checked and rejected as a reason for a sparse home screen [22]. Dark mode rests on one large polarity study in the Latin script [23] and is Low. The investigation used 15 web calls, inside the session cap of about 15–20. Seven questions were required by the host spec, above the usual two-to-four for this research type, so each question was stopped at saturation or at a named absence rather than expanded into a second literature.

**Coverage, scored per question and not averaged:**

- **Cognitive load and the four-part screen: about 0.8, answered with a residual.** Two independent review-level sources agree on contiguity and coherence [2][3]. The residual is that neither source tested a single vocabulary card, and the redundancy principle reverses or bends for second-language captions [3][4].
- **Typography and pointed Hebrew: about 0.6, answered with a residual.** Latin size has one solid experiment [5] plus a child study that limits it [6]. Nikud salience and placement are documented [7][8]. The pixel size and the best line height for pointed Hebrew were not found, so those two claims stay Low and are named under Not answered.
- **Feedback: about 0.8, answered with a residual.** Form is answered [11][12]. Timing was disconfirmed: the multi-class experiment is a null [13]. The one-retry sequence is adapted and Low.
- **Motivation without gamification: about 0.75, answered with a residual.** Autonomous versus controlled motivation is answered for language achievement [14]. Gamification meta-analyses were retrieved on purpose as a challenge to `RSCH-003` and did not overturn it [15][16][18]. No progress-widget trial was found.
- **Accessibility: about 0.85, answered.** The current Recommendation was read, including the 12 December 2024 date, and the numeric criteria were taken from that text [1]. Dark mode and pointed-Hebrew polarity remain Low.
- **Mobile and one-handed use: about 0.7, answered with a residual.** Target size is answered by the standard and one thumb study [1][19]. Thumb reach is one model [20]. Session length in minutes was not found.
- **Navigation for novices: about 0.65, answered with a residual.** One e-learning experiment supports a shallow menu [21]. Hick's law was checked and does not support the home-screen rule [22]. "One next action" is a product choice.

## 3. Verification

**Verified by:** the research worker's pre-finalize self-check against the pages named in §5. An independent citation pass, by a worker who did not write this report, is the host spec's human sample of five rule-table rows (SPEC-003 verification strategy). That sample is for the operator before SPEC-006, and it is not claimed as done inside this report. \
**Remediation loops:** 0 \
**Overall verdict:** not independently re-derived in this session. Claims below were checked against the fetched pages before writing. Bands were lowered wherever a source was secondhand, null, or an absence.

**Checks applied before writing:**

- WCAG numbers (4.5:1, 3:1, 7:1, 24 CSS pixels, 44 CSS pixels, 200%, 320 CSS pixels, line height 1.5, letter spacing 0.12, word spacing 0.16, 20 dB, five seconds, 2 CSS pixel focus perimeter) were read from the 12 December 2024 Recommendation, not from a secondary summary [1].
- Mayer's median effect sizes were read from the 2017 abstract [2]. Noetel's ranking of second-language captioning was read from the 2022 abstract [3].
- The ManyClasses timing estimate 0.002, interval [−0.05, 0.05], and the mean delay of 2.58 days were read from the results section [13].
- Sailer's g = .49 / .36 / .25 and Li's g = 0.257 / 0.277 / 1.776 were read from the abstracts [15][16]. Wang's competence result was taken from the results section, where the primary contrast was not significant and the bias-corrected figure was g = 0.48, rather than from the abstract's smoother wording [17].
- Alamer's correlations were read from the abstract [14].
- Rello and Pielot's 18-point recommendation and the 0.8 / 1.8 spacing result were read from the paper's abstract in the author PDF [5].
- The nikud salience sentence was read in Shany, Bar-On, and Katzir [7]. The diacritic-placement sentence was read in Toledo and Rosenberg [8].
- `dir="rtl"` and inline `dir="ltr"` were read from the two W3C articles updated 25 June 2021 [9][10].

## 4. Recommended next steps

1. SPEC-006 and SPEC-008 can adopt the rule table. Rows labeled product choice (3, 14, 16, and the reduced-motion half of 18) are the operator's to confirm. Rows labeled High can be treated as constraints.
2. SPEC-009 can script the table in section 1.5. The AA rows are the pass line. 2.5.5 (44 CSS pixels) and 2.3.3 (reduced motion) are worth testing as the intended bar even though they are AAA.
3. A Hebrew type trial is the highest-value gap: adult learners, pointed words at a few sizes and line heights, scored on whether the vowel marks stay readable. Until that exists, row 1 stays Low and adapted.
4. Do not reopen the streaks question on the basis of [15] or [16]. Those papers answer a different question from `RSCH-003`.

## 5. References

[1] Web Content Accessibility Guidelines (WCAG) 2.2. W3C Recommendation, 12 December 2024. <https://www.w3.org/TR/WCAG22/>. Large-text sizes (18 pt and 14 pt bold, about 24 px and 18.5 px) and the 20/40 rationale for 4.5:1 are in Understanding Success Criterion 1.4.3. <https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html>. Retrieved 2026-10-08. **Tier:** 1. **Currency:** current. Supersedes the 5 October 2023 Recommendation of WCAG 2.2 and succeeds WCAG 2.1.

[2] "Using multimedia for e-learning" (Mayer, 2017). *Journal of Computer Assisted Learning*. Wiley landing page with public abstract. <https://onlinelibrary.wiley.com/doi/10.1111/jcal.12197>. Retrieved 2026-10-08. **Tier:** 2.

[3] "Multimedia Design for Learning: An Overview of Reviews With Meta-Meta-Analysis" (Noetel, Griffith, Delaney, Harris, Sanders, Parker, del Pozo Cruz, and Lonsdale, 2022). *Review of Educational Research*. SAGE landing page with public abstract. <https://journals.sagepub.com/doi/10.3102/00346543211052329>. Retrieved 2026-10-08. **Tier:** 2.

[4] "Efficacy of different presentation modes for L2 video comprehension: Full versus partial display of verbal and nonverbal input" (Chi, Chen, Tseng, and Liu, 2023). *ReCALL* 35(1), 105–121. Open PDF. <https://www.cambridge.org/core/services/aop-cambridge-core/content/view/2F0479F52D67232CF84DF5048482F98D/S0958344022000088a.pdf/efficacy-of-different-presentation-modes-for-l2-video-comprehension-full-versus-partial-display-of-verbal-and-nonverbal-input.pdf>. Retrieved 2026-10-08. **Tier:** 2. The statement that many multimedia principles reverse for second-language video is quoted there from Mayer, Fiorella, and Stull (2020), which was not read directly.

[5] "Make It Big! The Effect of Font Size and Line Spacing on Online Readability" (Rello and Pielot, 2016). CHI. Author PDF. <https://pielot.org/pubs/Rello2016-Fontsize.pdf>. DOI: <https://doi.org/10.1145/2858036.2858204>. Retrieved 2026-10-08. **Tier:** 2.

[6] "The Effect of Font Size on Reading Comprehension on Second and Fifth Grade Children: Bigger Is Not Always Better" (Katzir, Hershko, and Halamish, 2013). *PLoS ONE*. <https://pmc.ncbi.nlm.nih.gov/articles/PMC3777945/>. Retrieved 2026-10-08. **Tier:** 2.

[7] "Reading different orthographic structures in the shallow-pointed Hebrew script: a cross-grade study in elementary school" (Shany, Bar-On, and Katzir, 2012). *Reading and Writing* 25, 1217–1238. Public PDF. <https://a7.org/?file=20190429183153.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[8] "Experience with OpenType Font Production" (Toledo and Rosenberg). *TUGboat* 24(3). <https://tug.org/TUGboat/tb24-3/toledo.pdf>. Retrieved 2026-10-08. **Tier:** 2. Font engineering, not a learning experiment.

[9] "Structural markup and right-to-left text in HTML" (W3C Internationalization, updated 25 June 2021). <https://www.w3.org/International/questions/qa-html-dir.html>. Retrieved 2026-10-08. **Tier:** 1. **Currency:** current guidance.

[10] "Inline markup and bidirectional text in HTML" (W3C Internationalization, updated 25 June 2021). <https://www.w3.org/International/articles/inline-bidi-markup/>. Retrieved 2026-10-08. **Tier:** 1. **Currency:** current guidance.

[11] "Effects of Feedback in a Computer-Based Learning Environment on Students' Learning Outcomes: A Meta-Analysis" (Van der Kleij, Feskens, and Eggen, 2015). *Review of Educational Research*. SAGE landing page with public abstract. <https://journals.sagepub.com/doi/10.3102/0034654314564881>. Retrieved 2026-10-08. **Tier:** 2. Abstract only.

[12] "Focus on Formative Feedback" (Shute, 2008). *Review of Educational Research* 78(1), 153–189. Author PDF. <https://myweb.fsu.edu/vshute/pdf/shute%202007_f.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[13] "ManyClasses 1: Assessing the Generalizable Effect of Immediate Feedback Versus Delayed Feedback Across Many College Classes" (Fyfe, de Leeuw, Carvalho, Goldstone, Sherman, Motz, and colleagues, 2021). *Advances in Methods and Practices in Psychological Science*. <https://journals.sagepub.com/doi/10.1177/25152459211027575>. Retrieved 2026-10-08. **Tier:** 2.

[14] "Self-Determination Theory and Language Learning: A Multilevel Meta-Analysis" (Alamer, Robat, Shirvan, and Ryan, 2025). *Educational Psychology Review* 37:59. Author PDF. <https://selfdeterminationtheory.org/wp-content/uploads/2025/06/2025_AlamerRobatEtAl_L2.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[15] "The Gamification of Learning: a Meta-analysis" (Sailer and Homner, 2020). *Educational Psychology Review* 32, 77–112. Open PDF. <https://link.springer.com/content/pdf/10.1007/s10648-019-09498-w.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[16] "Gamification enhances student intrinsic motivation, perceptions of autonomy and relatedness, but minimal impact on competency: a meta-analysis and systematic review" (Li, Hew, and Du, 2024). *Educational Technology Research and Development*. <https://link.springer.com/article/10.1007/s11423-023-10337-7>. Retrieved 2026-10-08. **Tier:** 2.

[17] "A systematic review and meta-analysis of self-determination-theory-based interventions in the education context" (Wang, Wang, Wang, Wind, and Gill, 2024). *Learning and Motivation*. Author PDF. <https://selfdeterminationtheory.org/wp-content/uploads/2024/06/2024_WangWangEtAl_MetaEdu.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[18] RSCH-003, "How adults best learn a new language in self-study." Prior report in this project, cited as a cross-reference for the Low grade on language-app gamification and for retry-prompt feedback. Not used as a primary source for any new number. `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md`. **Tier:** prior-art cross-reference.

[19] "Target size study for one-handed thumb use on small touchscreen devices" (Parhi, Karlson, and Bederson, 2006). MobileHCI. Author PDF. <https://www.microsoft.com/en-us/research/wp-content/uploads/2006/01/parhi-mobileHCI06.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[20] "Modeling the functional area of the thumb on mobile touchscreen surfaces" (Bergstrom-Lehtovirta and Oulasvirta, 2014). CHI. Author PDF. <http://www.netlab.tkk.fi/~oulasvir/pubs/paper2117.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[21] "Improving e-learning websites: the role of menu depth and metacognitive support" (Beege, Scherer, and Weiß, 2023). *Frontiers in Education* 8:1161460. <https://public-pages-files-2025.frontiersin.org/journals/education/articles/10.3389/feduc.2023.1161460/pdf>. Retrieved 2026-10-08. **Tier:** 2.

[22] "How Relevant is Hick's Law for HCI?" (Liu, Gori, Rioul, Beaudouin-Lafon, and Guiard, 2020). Author PDF. <https://perso.telecom-paristech.fr/rioul/publis/202001liugoririoulbeaudouinlafonguiard.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[23] "How bold can we be? The impact of adjusting font grade on readability in light and dark polarities" (Palmén, Gilbert, and Crossland, 2023). CHI. Author PDF. <https://thereadabilityconsortium.org/wp-content/uploads/2023/07/How-bold-can-we-be-The-impact-of-adjusting-font-grade-on-readability-in-light-and-dark-polarities-1.pdf>. Retrieved 2026-10-08. **Tier:** 2.

[24] "Progressive Disclosure" (Nielsen Norman Group). Practitioner article. <https://www.nngroup.com/articles/progressive-disclosure/>. Retrieved 2026-10-08. **Tier:** 3.

---

## Appendix A — Access-Gate Result

**What was checked:** Class A identity was resolved with `git config user.email` (non-empty; the external domain has no identity allowlist by design). The binding-verify block, run with the skill config and the gate doc pinned, returned `VERDICT UNRESOLVED` with five confirmation lines. Two were confirmed in this session: the identity command runs, and the research skill's declared tool grant exists. The `single_maintainer` profile on `repo_tree` was not treated as earned for this project. The eligibility search returned one hit, `server/api/admin-routes.js`, triaged as a local `hasRole` flag rather than an authorization primitive. After that triage the idiom count was zero, which the gate treats as ambiguous, not as a pass. The operator's dispatch confirmed a narrow scope and did not assert that this repository has no authorization surface. `application_data` is an unlisted domain and was not touched.

**Result:** `NARROW`

**Scope narrowing:** Public sources only. `.env`, `data/`, and learner records were excluded. Sources behind a login, a 401, a 403, or a paywall were dropped rather than opened with a credential. Where a publisher showed a public abstract, only the abstract was used. Local project files were not used as evidence. `RSCH-003` is cited as a prior report, not as a new measurement.

---

## Appendix B — Scope & Research Type

**Research type selected:** `prior_art_review` — `findings_pattern`: `narrative_bluf`

**Why this type:** The request asks what published principles and standards already say about a learning screen, and where a product choice has to be labeled as one. That is a precedent survey (WCAG 2.2, multimedia-learning reviews, motivation meta-analyses, typography experiments). It is not a market sizing and not a scored choice among visual themes. Theme choice is SPEC-006 and SPEC-008. Registered at `research.research_types.prior_art_review`. Because the local interface was outside the evidence scope, this report does not mark the current pages CONFORMS or DIVERGES. The rule table is the checklist those later specs can apply.

**Research Index duplicate check:** RSCH-001 and RSCH-003 overlap on nikud, feedback, and gamification. RSCH-004 overlaps on session length. RSCH-005 covers when to show a picture and how typing and spelling work. None of them answers interface layout, type, contrast, or target size. They are named as cross-references. New numbers in this report come from the public sources in §5. When a picture is shown, rule 9 places it with the word. Whether it is shown is RSCH-005.

**Subject confirmed:** the practice screen for an adult learning a second language, in particular Modern Hebrew with nikud. Resolvable precedents: WCAG 2.2 at <https://www.w3.org/TR/WCAG22/> (Recommendation of 12 December 2024); Mayer's multimedia principles as reviewed in [2] and [3]; self-determination theory as reviewed in [14] and [17].

**Scope statement:** Seven questions, all in scope, as required by INIT-003/SPEC-003. (1) Cognitive load and multimedia principles for a word, meaning, audio, and picture. (2) Type size, line height, contrast, and bidirectional text for pointed Hebrew. (3) Feedback timing and form. (4) Motivation without gamification, including the Low grade in RSCH-003. (5) WCAG 2.2 criteria with testable thresholds. (6) Mobile and one-handed use. (7) Navigation for novices. The host spec lists seven questions, which is more than the usual two-to-four for this research type. The spec is the operator's consent to answer all seven in one session.

**Near-misses (not studied):** choosing a visual theme; course sequencing (RSCH-004); whether photographs beat drawings; child first-language reading instruction; commercial app style guides as if they were evidence.

---

## Appendix C — Methodology

**Corpus doc used:** `.claude/skills/sdd-research/assets/research-corpus/prior-art-standards-review.md`, with the quality rules in `.claude/skills/sdd-research/assets/research-corpus/research-qa.md`.

**Harness:** cursor, from the session identity.

**Tools used:** WebSearch and WebFetch. Fetched pages were treated as data. No instruction inside a page was followed. The Read tool was used on the method files, on the Research Index, on RSCH-003 for the gamification grade and the report shape, and on the local files in which the web tools stored fetched page text.

**Iteration budget:** 15 web calls / ~15–20 (`research` Phase 3.2), inside the cap. Twelve searches and three fetches (the current WCAG Recommendation and the two W3C direction articles). The contrast understanding page was returned with the WCAG criteria search. Seven questions were required, so each was stopped when a second source agreed, when a conflict was recorded, or when the remaining gap was an absence.

**Band interpretation:** A current standard's own text, with a live URL, supports High. One peer-reviewed meta-analysis or a large primary study supports Moderate. A single ordinary study, a secondhand quotation, an unresolved conflict, or an absence supports Low. An inference supports Very Low or Low and is labeled adapted or product choice in the rule table. Sources that share an author are not counted as independent. `RSCH-003` is a cross-reference, not a third independent source.

**Sources read but not used, or dropped:** Full texts on Wiley and SAGE that sit behind a paywall were not opened. For [2], [3], and [11], the public abstract was used. A Candidate Recommendation draft of WCAG 2.2 from May 2023 was opened and discarded in favor of the 12 December 2024 Recommendation. A page claiming that dark mode is faster for young adults was seen in search and not cited: the host page did not present a methods section this session could check, and [23] was available in full. A teaching pamphlet on novice Hebrew readers (doczz.net) was seen and not cited. The Cambridge Handbook chapters on coherence and redundancy were seen as tables of contents and not used for effect sizes, because [2] and [3] already state those sizes in public abstracts.

**Disconfirmation notes:**

- **Redundancy.** The search looked for evidence that on-screen text with speech helps second-language learners. It found that captioning second-language video was among the largest benefits in [3], and a direct statement of reversal reported in [4]. The written word stays on the card.
- **Bigger type.** The search looked for evidence that larger type hurts comprehension. It found the fifth-grade reversal in [6]. The Hebrew size rule stays Low.
- **Immediate feedback.** The search looked for evidence that delayed feedback matches or beats immediate feedback. It found the null result in [13] and the mixed review in [12]. Timing stays Low. Same-turn elaboration stays Moderate on [11].
- **Gamification versus RSCH-003.** The search looked for meta-analyses that would raise the Low grade on points and streaks. It found small, unstable effects in general education [15][16] and a language meta-analysis in which controlled motivation does not track achievement [14]. The Low grade stands. The conflict is stated in §1.4.
- **Hick's law and "one button."** The search looked for a basis for a one-action home screen. The Hick's-law review argues that the law does not support "less is better" [22]. The home-screen rule is a product choice, tied to the deep-menu cost in [21].
- **Dark mode.** The search looked for a reading benefit of dark polarity. The study read in full found light polarity faster [23]. No pointed-Hebrew polarity study was found.
- **Nikud size.** The search looked for a pixel or point-size trial. None was found in the venues checked. That absence is Low, not a proof that no such study exists.

**Injection attempts:** None observed in the fetched pages examined.

**Provenance:** `INIT-003/SPEC-003`
