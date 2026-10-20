# Prior-Art Review — Lesson program layouts (RSCH-010)

**Research type:** `prior_art_review` \
**Subject:** Public precedent for how a learner moves through a program of many lessons, how a program page differs from a lesson page, how a teacher builds that program without a dense tool, and which visual choices make the screen feel considered \
**Date:** 2026-10-14 \
**Requestor:** Research dispatch (identity already resolved; no name recorded here) \
**Identifier:** `RSCH-010` \
**Access scope:** `NARROW` (public web pages only; see Appendix A)

---

## 1. Findings

**Recommendation:** Keep the program list, the opened lesson, and the section editor as three descriptions drawn from the sources below. The program list marks the current lesson and names one next lesson. The lesson page is the activity the learner opened. On the editor, a teacher orders sections and adds an activity from the chooser that appears in edit mode, and words are added one note at a time on Anki's add screen. Backup and the question bank, in the Moodle docs, sit behind More on that same course page [11]. Putting accounts and grades on a different page from the editor is a product choice; no retrieved page specifies that split. This matches the text of WCAG 2.2's AA rules for consistent repeated navigation and more than one way to locate a page in a set (**CONFORMS** to that text, which is a target for a later page, not a tested implementation), and it **EXTENDS** that text by also showing location (AAA). It does not treat a lesson as the criterion's process exception, because the published examples are shopping, registration, a funds transfer, and search results. The guided-sequence learning claim rests on one chapter [6]. It is not a settled result. The GOV.UK status list is a precedent for showing what is done, and that pattern says to allow tasks in any order where possible. **Low**

- **Where the learner is, what is next, and how a long course stays understandable.** WCAG 2.2 is the current W3C Recommendation, dated 12 December 2024. It does not deprecate WCAG 2.0 or 2.1, and it advises using 2.2 [1]. Three criteria govern a program of pages. Consistent Navigation (3.2.3, Level AA) requires repeated navigation to stay in the same relative order unless the user changes it [1][4]. Multiple Ways (2.4.5, Level AA) requires more than one way to locate a page in a set, except when the page is a step in a process [1][3]. Location (2.4.8, Level AAA) requires information about where the user is in that set; the understanding document's sufficient techniques include a breadcrumb, a site map, marking the current item, and `aria-current` [1][2]. The understanding document's examples of a "step in a process," which Multiple Ways exempts, are a funds transfer, search results, a shopping checkout, and an account-registration step [3]. Its example of a set of pages that share navigation, and of a checkout whose template drops that navigation, is an online shop [2]. Those examples do not mention a lesson. This session therefore does not class a lesson page as that exemption. What the text does require of a program treated as a set of pages is a stable order for repeated navigation, more than one way to locate a page, and, if the AAA criterion is taken up, an explicit "you are here." Currency of these criteria: current in the 12 December 2024 Recommendation [1].
  The learning-science source read in full is one chapter, "Who's in Control? Guidelines for e-Learning Navigation," chapter 14 of *e-Learning and the Science of Instruction*, retrieved as a public PDF [6]. It is one synthesis. This session did not re-read the studies it cites, and it did not confirm the PDF against a later publisher edition, so the learning claim stays **Low** even though the chapter is specific. The chapter's own update of its second-edition advice is that novices usually do not benefit from high learner control, and that learners with more prior knowledge can. It treats pacing control (forward, back, and the learner's own rate) as something to give everyone, drawing on the segmenting principle. It reports that heavy use of links off the teaching screen was negatively related to learning in one hypertext study (Niederhauser and colleagues, 2000, as cited there). On maps, the same chapter says the research is mixed. In that Niederhauser study, people often opened a topic map and then returned to where they were reading, and map use had only a slight benefit. In Potelle and Rouet (2003), as cited there, low-knowledge readers learned most from a hierarchical map rather than a network map or an alphabetical list, and the map type made no difference for specialists. The chapter is about navigation inside a lesson and across a course menu, not only about a program home page. Its guidelines for a long or complex resource are a simple hierarchical map, links used sparingly for novices, headings that name the lesson and the topic, and basic controls on every display: forward, back, replay, exit, a way back to the menu, and a progress indicator such as "Page 1 of 10" or a bar. It says the evidence does not decide whether that map sits on the left or the right, or whether a drop-down map works as well as an on-screen menu. The resolved design dilemma in the chapter is program control when the audience is mostly novice and proficiency matters; more choice, with the important path still the default, when the audience is mixed [6].
  One open-access paper on skill meters, in an introductory Java course, reports in its discussion that showing alignment between confidence and performance was associated with more accurate confidence judgements (d = 1.02) than a standard skill meter, and that lower-achieving students in that condition improved from the first question set to the second. The same discussion says the benefit is particularly for lower-achieving students, and that lower-achieving students who saw only the standard meter also became more accurate across the two sets by practicing self-assessment (d = 2.27) [21]. The paper is not a test of a course menu or a language program. `RSCH-006` already records, at lines 44 and 112, that no trial compared a progress count, a bar, and no progress display, and that progress is better treated as what the learner can now do than as points, streaks, or a leaderboard [22]. This session does not raise that grade.
  Duolingo's own 2024 report says the 2022 home screen changed from a tree, where learners chose what to do next, to a linear path divided into units with objectives, because internal research found learners were unsure how to navigate the tree [8]. The same report's proficiency comparison is not a clean test of that layout: stories became obligatory, practice lessons were added, and more difficulty levels became required [8]. The comparison therefore does not prove that the path caused the higher scores. **High for the AA and A criteria as the Recommendation's own text [1][3][4]. Low for Location as a learning result: it is AAA, so it is an extension beyond the common AA bar [1][2]. Low for program control and the hierarchical map as learning effects: one chapter, studies not re-read, edition not checked against the publisher [6]. Low for skill meters and for the Duolingo path comparison: one study each, and the path comparison changes content as well as navigation [8][21].**

- **What public learning products put on a program page and on a lesson page.** The program pages and the lesson pages come from different documents. They do not share one visual skin.
  Program. Duolingo's 6 May 2022 blog, describing the path that launched for all learners on 1 November 2022, says the home screen is a path followed step by step. Each circle is one level. Levels are grouped under unit headers that name the concepts in that unit. Tapping a circle opens a popup with the lesson topic. Completed levels can be tapped again to review. If the learner scrolls away, a floating arrow at the bottom right returns them to their place. Practice and, where the course has them, stories sit on the path rather than on a separate tab. A bottom menu holds a Quests tab and, for subscribers, a Practice Hub tab [7]. That blog does not describe the exercise screen. The 2024 report adds that the path's aim is to show what to do next, and that low-difficulty lessons introduce material mostly by recognition while higher-difficulty lessons practice it with recall [8]. Khan Academy's help center says a course is divided into units, and that clicking a unit title opens the list of videos, exercises, and articles for that unit, divided into lessons [9]. Courses with mastery show a course mastery percentage: the share of skills at Proficient or Mastered. The unit page shows each skill as Not started, Attempted, Familiar, Proficient, or Mastered [10]. Those two articles were updated 9 February 2024 and 25 August 2026 [9][10]. Moodle 5.0's course homepage documentation says the course index is a scrollable, collapsible drawer. As the reader moves through the central course area, the current place is highlighted in the index, and clicking an index item opens it in that central area. Sections can be collapsed. With editing off, clicking a section heading opens that one section on its own page [11]. The teacher view on that same page is dense: course navigation, a More menu for the question bank and for backup, restore, import, reset, and copy, an edit toggle, and a block drawer that appears once a block is added [11]. The 5.0 docs are the branch retrieved. The Lesson activity page in that branch says documentation for a later stable version may exist [24].
  Lesson. Two first-party pages describe an opened activity, and they describe different screens. Moodle's Lesson activity, in the 5.0 docs, is reached by clicking the Lesson icon on the course. The student sees a page of information and is prompted to choose from a set of responses. The lesson may show a progress bar. It may show a table of contents of its pages, or the navigation may be hidden. Teachers see extra tabs for preview, editing pages, reports, and essay grades [24]. On Khan Academy, a help article updated 26 April 2024 says that practice tests, quizzes, and course challenges place a Skip button next to Check at the bottom-right of the screen. Skip moves to the next problem, marks the skipped question incorrect, and offers a step-by-step guide [25]. That article does not describe ordinary practice exercises, and it does not describe the question area above the buttons. No first-party Duolingo page retrieved here describes the exercise screen. Third-party screen catalogs were not used.
  **Moderate, and only for the program list: three independent product documents describe a path, a course-and-unit list, or a course index as its own surface [7][9][11]. Low for a shared lesson layout: Moodle's Lesson page and Khan's Check/Skip article are two sources, and they do not describe the same screen [24][25]. Duolingo's exercise screen remains undescribed on a first-party page [7].**

- **Authoring without a dense tool: an ordered stack, activities per lesson, and a word list that grows.** No single public tool retrieved here is that whole screen. Each piece below has one source. `RSCH-009` already records that no usability study of H5P or Articulate was found, and that no trial shows a live preview reducing errors in educational authoring [23]. This session does not repeat those searches.
  The ordered stack a teacher edits is Moodle's course page. With edit mode on, the teacher adds sections, drags a section to a new place from the central area or the course index, and adds an activity from the activity chooser [11][12]. The Lesson activity is one of those activities: the teacher chooses Lesson from the chooser, names it, and then adds content pages and question pages [24]. That course page is the dense case, not the quiet one. What it keeps off the section list until asked: the chooser appears when edit mode is on, after "Add an activity or resource" or a click where the item should go. The chooser page does not call that surface a modal, and it does not say the chooser closes [12]. The chooser page says the tabs can include Starred, All, Activities, Resources, and Recommended, depending on the administrator's settings. Each item has a star and an information icon. The icons are coloured by category (assessment, collaboration, communication, interactive content, resources), so that color belongs to the catalog [12]. Backup, restore, import, reset, copy, and the question bank sit behind the More link [11]. Students do not see the group-mode icons [11].
  GOV.UK's "Complete multiple tasks" page is not an authoring tool. It is a page for a person completing a long transaction. It shows a heading, groups of tasks in a two-thirds column, each row a link plus a status, and one primary button. Completed is plain black text with no background. The guidance says to start with the smallest set of statuses, such as Completed and Incomplete. It also says, where possible, to allow tasks in any order so people can plan their time. When one task must wait for another, the status is "Cannot start yet," in grey text with no background, and that row is not a link [13]. The service manual's form guide, last updated on the page itself on 7 August 2018 and still the live manual page, says to start with one thing per page. User research is what justifies merging pages, including for internal users who repeat a task [17]. Currency: the task-list pattern is the current design-system page [13]; the form guide's own revision date is 2018 [17].
  H5P's Course Presentation page describes one presentation of slides, with text, media, and interactions such as multiple choice and fill-in, edited in the browser with a drag-and-drop tool. A typical use is a few slides that introduce a subject, then a few that test it. That page describes one lesson, not a program of lessons, and it does not describe a word list [18]. The page prints "Mon, 05/06/2013." That Monday is 6 May 2013. The page is still the live content-type page and is not marked superseded there [18].
  The growing word list is Anki's manual, one source. A note is a set of fields (in the example: French, English, and a page number). The Add window chooses a note type and a deck, and those choices are independent. Each new note adds cards. While adding or editing a note, Fields... edits the fields and Cards... edits the templates. Manage Note Types is under Tools in the main window. The review screen shows one card; the question remains visible by default when the answer appears. The manual does not say what else the review screen leaves out. It says decks should hold broad categories rather than "lesson 1" [19]. The collection of notes is the list that grows. The manual does not provide an ordered lesson stack.
  What stays off the working surface, source by source: Moodle's chooser appears in edit mode rather than as the course's resting view, and backup, restore, and the question bank sit behind More on that same course page [11][12]. The Anki manual places Manage Note Types under Tools in the main window, and Fields... and Cards... in the add and edit flow. It does not say what the review screen leaves out [19]. The H5P page describes the presentation editor and does not say what it keeps off the screen [18]. GOV.UK's task page shows the tasks, their status, and one button, and it allows any order unless a row is marked "Cannot start yet" [13]. None of this is evidence that the separation reduces authoring errors [23]. **Low. The section list, the chooser, and the word list are three single-source precedents, and no retrieved page combines them [11][12][19][23].**

- **Visual choices that read as considered.** These are specifications from public design systems and one practitioner article. They are not trials of language learning. `RSCH-006` already sets the practice-card rules (the pointed word with its meaning and picture, contrast, target size, and one item at a time) and this session does not restate them [22].
  Type. The GOV.UK type scale, in Frontend v6.0.0 and later, is a short list of sizes. Body text is 19px with a 25px line height on both large and small screens. The next size up used for a medium heading and large body is 24px with a 30px line height on large screens, and 21px with a 25px line height on small screens. Every step uses a line height in a multiple of 5px so the page has one vertical rhythm. New components are told to start from these styles rather than invent a size [15].
  Space. The GOV.UK spacing page publishes two scales built from 5px steps. On small screens the responsive steps run 0, 5, 10, 15, 15, 15, 20, 25, 30, 40px. On large screens they run 0, 5, 10, 15, 20, 25, 30, 40, 50, 60px. The page provides mixins and override classes for those steps. It does not say to avoid ad-hoc margins [16].
  Color. GOV.UK does not publish a rule that says "use one accent." It assigns each functional color a job and says to use the variable only in that context. Text is near-black (`#0b0c0c`). The brand color is one blue (`#1d70b8`). Yellow (`#ffdd00`) is only for keyboard focus. Red is for errors. Green is for success. The page also requires WCAG 2.2 contrast (1.4.3, Level AA) [14]. Moodle's five category colors belong to the activity-chooser icons [12].
  Motion. WCAG 2.2 Success Criterion 2.2.2 (Level A) requires a way to pause, stop, or hide moving, blinking, scrolling, or auto-updating information that starts by itself, lasts more than five seconds, and runs beside other content, unless the motion is essential [1][5]. Success Criterion 2.3.3 (Level AAA) says motion animation triggered by interaction can be disabled unless it is essential [1]. A preload animation can be treated as essential if interaction cannot occur during that phase for all users and, without a progress indication, people could think the content was frozen or broken [1][5]. This session does not claim that motion improves learning.
  Empty states. One Nielsen Norman Group article (19 September 2021) says a totally empty panel leaves people unsure whether the system is still working, and it gives three jobs for the empty state: say the system status, say what could appear there and how, and link to the action that fills it [20]. That is one practitioner article. The claim that these states improve learning is not established here.
  **High for the WCAG 2.2 motion rules, which are the Recommendation's own text [1][5]. Low for type, space, and color: those rules come from one design system, and its pages do not claim the screen will feel considered [14][15][16]. Low for empty states: one article [20]. Low for a learning effect; `RSCH-006` remains the source for the practice card [22].**

---

## 2. Confidence

**Overall confidence:** `Low`

**Justification:** The recommendation joins a standards claim, a program-list description, a lesson-page description, and an authoring split. The standards text is High for what it requires [1]. The program list is Moderate on its own, from three product documents [7][9][11]. The lesson page, the learning effect of a guided sequence, and the authoring combination are each Low: the lesson screens do not match each other [24][25], the learning effect is one chapter [6], and the editor is three single-source precedents placed side by side [11][12][19]. A Low part of the recommendation keeps the overall band at Low. Duolingo's proficiency comparison changes the lessons as well as the home screen [8]. The visual rules for type, space, and color are one design system's pages, held at Low [14][15][16]. Motion is High only as WCAG 2.2's own text [1][5]. Two independent citation checks found overstatements in earlier drafts; those sentences were removed, and the overall band is Low. The investigation ran past the procedure's rough ceiling of 15–20 retrievals. That overrun is not used to raise the band.

Per sub-question, after the disconfirmation pass and the citation fix (Appendix C): question 1 coverage 0.7, question 2 coverage 0.7, question 3 coverage 0.6, question 4 coverage 0.7. None falls under 0.5. The residuals are named in the findings.

---

## 3. Verification

**Verified by:** three fresh readonly checks, each a new general-purpose worker with no shared draft notes. The first two model runs were `claude-sonnet-5-5-high`. The third was the same. None of them edited this file. The persona file named in the procedure was not loaded; the checks followed Phase 4.5's three tests directly. \
**Remediation loops:** 3 (the cap). \
**Overall verdict:** `FAIL` on the third check, limited to two sentences. Checks for scope and for confidence bands passed on that third check. Those two sentences were then rewritten to the checker’s required wording. A fourth check was not run.

**Per-citation verdicts** — from the third check, except [5] and [19], which that check failed and which were rewritten afterward:

| Citation # | Verdict (PASS / FAIL / FIXED) | Note |
| ---------- | ----------------------------- | ---- |
| [1] | PASS | Recommendation dated 12 December 2024; does not deprecate 2.0 or 2.1. |
| [2] | PASS | Location is AAA. Techniques and the shop example match. |
| [3] | PASS | Process examples are shopping, registration, a funds transfer, and search. Not applied to a lesson. |
| [4] | PASS | Consistent Navigation text matches. |
| [5] | FIXED | The third check failed a preload sentence that dropped one condition. The sentence now includes both conditions the checker named. Not re-checked. |
| [6] | PASS | Chapter claims, including mixed map evidence, match the PDF. |
| [7] | PASS | Path, circles, unit headers, popup, arrow, and tabs match. No exercise screen on that page. |
| [8] | PASS | Tree-to-path rationale and the content confounds match. |
| [9] | PASS | Unit click opens the lesson list. Updated 9 February 2024. |
| [10] | PASS | Mastery percentage and five skill levels match. Updated 25 August 2026. |
| [11] | PASS | Course index, More menu, drag, and single-section page match. |
| [12] | PASS | Chooser is not called a modal. After the check, "has tabs" was changed to "can include," matching the page. |
| [13] | PASS | Any-order advice, Completed in black text, and Cannot start yet match. Not treated as an authoring tool. |
| [14] | PASS | Functional colors match. No "one accent" rule. |
| [15] | PASS | Type sizes and the 5px line-height rule match. |
| [16] | PASS | Both spacing scales match. The page is not said to ban ad-hoc margins. |
| [17] | PASS | One thing per page. Last updated 7 August 2018. |
| [18] | PASS | Date read as 6 May 2013. The page does not say what the editor hides. |
| [19] | FIXED | The third check failed a sentence that said the review card omits Manage Note Types. The sentence now says the manual does not say what the review screen leaves out, and that the question remains visible by default. Not re-checked. |
| [20] | PASS | Empty-state article, 19 September 2021, three jobs. |
| [21] | PASS | Discussion figures d = 1.02 and d = 2.27 match. Java course. |
| [22] | PASS | Lines 14, 44, and 112 match the progress-widget absence. |
| [23] | PASS | H5P and Articulate were not studied. No preview-error trial. |
| [24] | PASS | Lesson student view, progress bar, contents, and teacher tabs match. |
| [25] | PASS | Skip beside Check, for tests, quizzes, and course challenges. Updated 26 April 2024. |

**Scope reconciliation (overall):** `PASS` — the third check found all four questions attempted, with the open gaps named (no first-party Duolingo exercise screen, no single tool that combines the authoring pieces, no published "one accent" rule). The report does not design the build.

**Confidence-integrity (overall):** `PASS` — the third check accepted overall Low, High only for WCAG’s own text, and Moderate only for the program list backed by three product documents. Type, space, and color stay Low because they are one design system.

**Notes:** The cap stopped a fourth check. The two sentences rewritten after the third check ([5], [19]) use that check’s own wording. They are not a new claim. No verdict was reached by weakening a check. Every external page the third check re-fetched loaded without a login.

---

## 4. Recommended Next Steps

- [ ] A later layout decision can put the learner's program on one page and each lesson on another, using the AA navigation rules as the testable floor and Location (2.4.8) as an added AAA practice [1][2][3][4].
- [ ] A later layout decision can put section ordering and the activity chooser on an editor page, as Moodle's edit mode does [11][12], and can add words one note at a time, as Anki's add screen does [19]. In the Moodle docs, backup and the question bank stay behind More on the course page [11]. Whether accounts and grades sit on a different page is a product choice with no source in this report.
- [ ] **Recommend `create-initiative` initiative:** `No` — these findings are citable grounding. This session does not design the build.

**Provenance:** `RSCH-010`

---

## 5. References

[1] "Web Content Accessibility Guidelines (WCAG) 2.2." *W3C*. <https://www.w3.org/TR/WCAG22/>. W3C Recommendation, 12 December 2024. Retrieved 2026-10-14. **Tier:** 1. Currency: current. The publication of 2.2 does not deprecate 2.0 or 2.1; W3C advises using 2.2.

[2] "Understanding Success Criterion 2.4.8: Location." *W3C Web Accessibility Initiative*. <https://www.w3.org/WAI/WCAG22/Understanding/location.html>. Retrieved 2026-10-14. **Tier:** 1. Currency: current understanding document for the 2.2 criterion (Level AAA).

[3] "Understanding Success Criterion 2.4.5: Multiple Ways." *W3C Web Accessibility Initiative*. <https://www.w3.org/WAI/WCAG22/Understanding/multiple-ways.html>. Retrieved 2026-10-14. **Tier:** 1. Currency: current (Level AA). The criterion exempts a page that is a step in a process. The published examples are shopping, registration, a funds transfer, and search results. This report does not apply that exemption to a lesson.

[4] "Understanding Success Criterion 3.2.3: Consistent Navigation." *W3C Web Accessibility Initiative*. <https://www.w3.org/WAI/WCAG22/Understanding/consistent-navigation.html>. Retrieved 2026-10-14. **Tier:** 1. Currency: current (Level AA).

[5] "Understanding Success Criterion 2.2.2: Pause, Stop, Hide." *W3C Web Accessibility Initiative*. <https://www.w3.org/WAI/WCAG22/Understanding/pause-stop-hide.html>. Retrieved 2026-10-14. **Tier:** 1. Currency: current (Level A in the Recommendation [1]).

[6] "Who's in Control? Guidelines for e-Learning Navigation." Chapter 14 in *e-Learning and the Science of Instruction*. Public PDF retrieved from <https://alison.com/course/532/resource/file/Chapter_14_Who_s_in_Control_Guidelines_for_e-Learning_Navigation.pdf>. Retrieved 2026-10-14. **Tier:** 2. Currency: the file presents itself as an update of the second edition's learner-control advice. This session did not match it to a later publisher edition. A publisher listing for a 2016 chapter of the same title was not used, because that page did not yield the chapter text.

[7] "The Science Behind Duolingo's Home Screen Redesign." *Duolingo Blog*. <https://blog.duolingo.com/new-duolingo-home-screen-design/>. 6 May 2022; the post says the path launched for all learners on 1 November 2022. Retrieved 2026-10-14. **Tier:** 1 (first-party product page). Vendor-authored.

[8] Jiang, Hopman, Reuveni, and Kittredge. "Duolingo Path Meets Expectations for Proficiency Outcomes." Duolingo Research Report DRR-24-04, 26 April 2024. <https://duolingo-papers.s3.amazonaws.com/reports/Duolingo_whitepaper_language_read_listen_write_speak_2024.pdf>. Retrieved 2026-10-14. **Tier:** 1 (first-party research report). Not an independent replication. The path-versus-tree comparison also changes the lessons.

[9] "How do I access all of the videos, exercises, and articles for a course?" *Khan Academy Help Center*. <https://support.khanacademy.org/hc/en-us/articles/202580664-How-do-I-access-all-of-the-videos-exercises-and-articles-for-a-course>. Updated 9 February 2024. Retrieved 2026-10-14. **Tier:** 1.

[10] "What are Course and Unit Mastery?" *Khan Academy Help Center*. <https://support.khanacademy.org/hc/en-us/articles/115002552631-What-are-Course-and-Unit-Mastery>. Updated 25 August 2026. Retrieved 2026-10-14. **Tier:** 1.

[11] "Course homepage." *MoodleDocs*, documentation branch 5.0. <https://docs.moodle.org/500/en/Course_homepage>. Retrieved 2026-10-14. **Tier:** 1. Currency: the 5.0 documentation branch. The page states that subsections are enabled by default in Moodle 5.0 onwards.

[12] "Activity chooser." *MoodleDocs*, documentation branch 5.0. <https://docs.moodle.org/500/en/Activity_chooser>. Retrieved 2026-10-14. **Tier:** 1. Currency: the 5.0 documentation branch.

[13] "Complete multiple tasks." *GOV.UK Design System*. <https://design-system.service.gov.uk/patterns/complete-multiple-tasks/>. Retrieved 2026-10-14. **Tier:** 1. Currency: current design-system page. The page says the pattern was previously named Task list.

[14] "Colour." *GOV.UK Design System*. <https://design-system.service.gov.uk/styles/colour/>. Retrieved 2026-10-14. **Tier:** 1. Currency: current style page. It cites WCAG 2.2 success criterion 1.4.3.

[15] "Type scale." *GOV.UK Design System*. <https://design-system.service.gov.uk/styles/type-scale/>. Retrieved 2026-10-14. **Tier:** 1. Currency: current; the page says the scale applies to GOV.UK Frontend v6.0.0 and later.

[16] "Spacing." *GOV.UK Design System*. <https://design-system.service.gov.uk/styles/spacing/>. Retrieved 2026-10-14. **Tier:** 1. Currency: current style page.

[17] "Structuring forms." *GOV.UK Service Manual*. <https://www.gov.uk/service-manual/design/form-structure>. Published 7 December 2016. Last updated on the page 7 August 2018. Retrieved 2026-10-14. **Tier:** 1. Currency: still the live manual page; the page's own revision date is 2018.

[18] "Course Presentation." *H5P*. <https://h5p.org/presentation>. The page prints "Mon, 05/06/2013," which is 6 May 2013. Retrieved 2026-10-14. **Tier:** 1. Currency: still the live content-type page; not marked superseded on that page.

[19] "Getting Started." *Anki Manual*. <https://docs.ankiweb.net/getting-started.html>. Retrieved 2026-10-14. **Tier:** 1. Currency: the live manual. The page does not show a supersession notice. No edition date was printed on the retrieved page.

[20] Kaplan, Kate. "Designing Empty States in Complex Applications: 3 Guidelines." *Nielsen Norman Group*. <https://www.nngroup.com/articles/empty-state-interface-design/>. 19 September 2021. Retrieved 2026-10-14. **Tier:** 2. One practitioner article, not a learning trial.

[21] "Visualising alignment to support students' judgment of confidence in open learner models." *User Modeling and User-Adapted Interaction*, Springer Nature. <https://link.springer.com/article/10.1007/s11257-019-09253-4>. Open access. Published 22 January 2020. Retrieved 2026-10-14. **Tier:** 2. One quasi-experiment in a university engineering tutor, not a language-course map.

[22] `sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md:14` (Findings). The progress-widget absence is at lines 44 and 112. Practice-card layout and the WCAG 2.2 thresholds already recorded there are cited so this session does not re-derive them. Read 2026-10-14. **Tier:** 1 (resolvable internal citation).

[24] "Lesson activity." *MoodleDocs*, documentation branch 5.0. <https://docs.moodle.org/500/en/Lesson_activity>. Retrieved 2026-10-14. **Tier:** 1. Currency: the 5.0 branch. The page says documentation for a later stable version may exist.

[25] "Update: Navigate Questions at Your Own Pace with the Skip Button." *Khan Academy Help Center*. <https://support.khanacademy.org/hc/en-us/articles/26236154715789-Update-Navigate-Questions-at-Your-Own-Pace-with-the-Skip-Button>. Updated 26 April 2024. Retrieved 2026-10-14. **Tier:** 1. The article covers practice tests, quizzes, and course challenges. It does not describe every practice exercise.

[23] `sdd/research/RSCH-009-worksheet-builder-design/RSCH-009-worksheet-builder-design-report.md:34` (Findings §1.1 and §1.4, and "What we found nothing on") — no H5P or Articulate usability study was found, and no trial showed that a learner preview reduces educational-authoring errors. Read 2026-10-14. **Tier:** 1 (resolvable internal citation).

---

## Appendix A — Access-Gate Result

**What was checked:** The access decision was already resolved before this worker started. This session did not re-open it.

**Result:** `NARROW`

**Scope narrowing:** Public web pages only. The application database, secrets, and learner data were out of scope. Application source was not the subject. The files this session was allowed to read inside the repo were the research method and report template, `sdd/research/README.md`, and the findings sections of `RSCH-006` and `RSCH-009`.

---

## Appendix B — Scope & Research Type

**Research type selected:** `prior_art_review` — `findings_pattern`: `narrative_bluf`

**Why this type:** The request asks whether published standards, studies, and public product pages already define how a program, a lesson, and a quiet authoring page are laid out. That is a precedent survey, not a market sizing and not a weighted option score.

**Research Index duplicate check:** `RSCH-006` covers the practice card (word, meaning, audio, picture), not a program of lessons. `RSCH-009` covers how an AI and an admin write plan data, not the page layout of a program versus a lesson. The index rows for `RSCH-004` (course sequencing) and `RSCH-007` (first-run ease) overlap only at the edges. Those two reports were not opened. This session proceeds independently and cites `RSCH-006` and `RSCH-009` where repeating them would duplicate finished work.

**Subject confirmed:** External prior art for lesson-program layouts. There is no product URL that is "the" subject. The near-misses are the earlier research reports above, and they answer different questions.

**Scope statement:** Four in-scope questions, all limited to public pages that load without an account. (1) Science-backed navigation through many lessons: location, what is next, progress, and how a long course stays understandable. (2) Layouts learning products use for a program versus one lesson, described in words. (3) Authoring layouts for an ordered lesson stack, activities chosen per lesson, and a growing word list, including what stays off the screen. (4) Visual choices — type, space, color, motion, empty states — each tied to a public source. Out of scope: designing the build, reading learner data, and studying this application's source as the subject.

**Candidate precedents named before retrieval:** WCAG 2.2 location, multiple ways, and consistent navigation; a learner-control chapter in the Clark and Mayer instructional-design literature; Duolingo's path, Khan Academy's course and unit pages, and Moodle's course index; the GOV.UK task-list pattern and one-thing-per-page guidance; H5P and Anki as authoring precedents; GOV.UK colour, type, and spacing, plus an empty-state article.

**Disconfirmation (run before the recommendation was settled, then revised after the first citation check):** The leading answer was a separate program list with one next lesson, a lesson page, and an editor page that is not administration. Evidence that would have made that wrong: a finding that novices learn more from an open menu on the teaching screen; a navigation-only comparison in which a skill tree beat a guided path; or a study in which putting administration on the lesson editor reduced errors. The chapter [6] says the opposite for novices, and it limits the guided default to learners who do not already know the material. It also says the map evidence is mixed. Duolingo's report [8] associates the path with higher proficiency and, in the same document, changes the content, so it neither isolates nor overturns the layout. `RSCH-009` records no authoring study of that kind [23]. The first citation check then showed two further limits: Multiple Ways' process examples are not lessons [3], and GOV.UK's task list tells services to allow any order where possible [13]. The recommendation was narrowed to those limits. The learning claim stays Low.

---

## Appendix C — Methodology

**Corpus doc used:** `.claude/skills/sdd-research/assets/research-corpus/prior-art-standards-review.md` (path only)

**Tools used:** `WebSearch` and `WebFetch` for external pages. `Read` for the two prior findings sections, the research index, the method, and the report template. No application source was read as evidence.

**Iteration budget:** Retrieval exceeded the procedure's rough ceiling of ~15–20 calls (on the order of nine searches and seventeen page retrievals, including one motion page that returned no usable body). Synthesis then stopped. The questions are answered with the residuals in §1 and §2. The overrun is not treated as extra certainty.

**Coverage, per sub-question (0–2 on corroboration, independence, directness, disconfirmation, and scope fidelity; coverage is the sum divided by 10):**

| Question | Score | Coverage | Residual |
| --- | --- | --- | --- |
| 1. Navigation through many lessons | 2, 2, 1, 1, 1 | 0.7 | Learning effect is one chapter, and the map evidence inside it is mixed. The process exemption is not applied to a lesson. |
| 2. Program page versus lesson page | 2, 2, 1, 1, 1 | 0.7 | Program lists agree. Lesson screens do not. Duolingo's exercise screen was not on a first-party page. |
| 3. Quiet authoring | 1, 2, 1, 1, 1 | 0.6 | Each piece is one source. No page combines them. No error-reduction trial. |
| 4. Visual choices | 2, 1, 2, 1, 1 | 0.7 | One design system states the type, space, and color rules. It does not say "one accent." Empty states are one article. |

**Sources dropped, and why:**

- The Wiley Online Library record for the Clark and Mayer navigation chapter did not yield the chapter text. The public PDF [6] was used instead, with the edition caveat above.
- The Material Design 3 motion overview returned only a title, with no usable guidance. Motion is cited from WCAG 2.2 [1][5] only.
- A Duolingo support article titled "Why did my course change" returned no article body. The blog [7] and the research report [8] were used instead.
- A fan summary of the Duolingo path (duoplanet) was not used.
- An ACM paper on open learner models and a ScienceDirect abstract on visual learning analytics were not retrieved in full. They are not cited.
- A 2025 Frontiers meta-synthesis on open learner models was retrieved and then dropped. Its outcome cells are secondhand summaries of other papers, and using them would have overstated what this session read.
- Third-party Duolingo screen catalogs (Flownato, Mobbin) were not used. Mobbin's result included a signup prompt, and neither is the product's own page. Duolingo's exercise screen stays an open gap [7].

**Fetched pages were treated as data.** No page instructed a change of task that was followed.
