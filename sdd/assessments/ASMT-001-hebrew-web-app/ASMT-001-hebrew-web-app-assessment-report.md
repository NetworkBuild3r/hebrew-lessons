# Codebase Assessment: Hebrew web app → its own research base (RSCH-001 to RSCH-010)

**Assessment ID:** ASMT-001  ·  **Status:** Complete
**Source:** `c:\Users\BrianNelson\Projects\Hebrew` (working tree at `86e8d50`, plus uncommitted files present on 2026-10-17)
**Assessed:** 2026-10-17  ·  **Analyst:** Cursor agent running `sdd-assess`
**Recommendation:** Refactor  ·  **Confidence:** 0.85
**Pipeline mode:** brownfield (targeted in-place changes)

> **How this assessment is adapted.** `sdd-assess` normally compares an outside codebase with a target platform. Here the question was different: *does the web app do what the project's own research says, and can it do better?* So the **standard** is the ten research reports in `sdd/research/` and the **source** is the app. "Capability" means how closely the learning loop follows the research. "Readiness" means the interface, first-run, navigation, and data qualities the research also asks for. "Refactor" means targeted changes inside the existing code: no rewrite and no new agent package.

## Bottom Line Up Front (BLUF)

**Recommendation: Refactor. Keep the architecture, fix about ten specific gaps, and turn on code that is already written.** Where the research is strongest (spacing, English meanings, no points), the app already conforms. The shortfalls are in *how* recall is practised (mostly "show answer, then grade yourself"), uneven feedback after a mistake, a 30-day review ceiling that will not scale, and two unbuilt practice types the research supports: hearing hard Hebrew sounds apart, and pronunciation with feedback. About half the fixes reuse code already in the repo, so the work stays small and there is still one stack to maintain.

## Objectivity & Independent Validation

The app and the research come from the same project, so the obvious objection is that the assessment grades its own homework. The mitigations:

- **Evidence discipline:** every load-bearing claim below was verified by reading the implementing file and lines, on both sides. The research claims come from the report text in `sdd/research/`, and the app claims from the code. Four read-only explorer agents surfaced leads first. Their summaries were treated as unverified until the cited lines were re-read; see the Claims-Evidence Ledger.
- **Falsification pass:** four ways the headline could be wrong were tested against the code. Two of them changed the findings. Self-graded review was downgraded from "contradicts" to "partial", and the scheduler finding was narrowed from "wrong algorithm" to "wrong ceiling". See Evidence & Falsification.
- **Independent LLM-judge eval:** a fresh read-only sub-agent that did not write this report scored it against the human-authored oracle. The verdict is recorded in `eval/`.

## Executive Summary

**We recommend one of the following paths:**

1. **No action.** The app already follows the strongest research findings.
2. **Targeted refactor.** Close about ten specific gaps inside the existing code, reusing what is already built.
3. **Refactor plus an AI judgment layer now.** Also add AI grading of spoken and written Hebrew output.

**Based on these findings, the recommendation is Option 2.** Option 3 starts as a time-boxed spike inside Option 2, not as a commitment.

- **Where the app is today:** a mature learning loop. Of the 21 research-backed behaviours scored in the matrix below, 7 largely conform (score 4–5), 11 partly conform (2–3), and 3 are absent (1). The conforming ones are mostly the strongest-evidence items: spacing (rated High in RSCH-003), English meanings, interleaving, and WCAG-level type and target sizes.
- **What it is missing:**
  1. Recall is mostly covert and self-graded.
  2. Three activities skip the "one retry, then show the answer" pattern the other five follow.
  3. The review ladder never schedules anything further out than 30 days, so maintenance load grows with every word learned.
  4. There is no listening-discrimination practice and no pronunciation feedback. Both are rated Moderate in RSCH-003.
  5. Verb-practice progress lives only in the browser, so it does not follow the learner between desktop and phone.
  6. A new visitor sees nothing before signing in (RSCH-007's top lever).
- **Capability versus readiness:** the learning loop scores about 2.8 out of 5 against the research and the interface and first-run qualities about 3.0. The averages hide the pattern that matters: the low scores cluster where the research is Low (habit, session goals) or where whole features are missing (perception, pronunciation). The core memory loop scores 3 to 4. The fixes can therefore be added on top; nothing needs redoing.
- **Harvestable deltas:** three places where the app goes beyond what the research asked for, worth keeping and copying elsewhere:
  1. The plan validator turns RSCH-004's design rules into named warnings. Examples are `N1` (more than 8 new words), `N7` (language-focused work above a quarter of the unit), `later-context`, and `pointed-word`.
  2. The same-day guard stops a lesson from inflating the review schedule.
  3. An FSRS-6 scheduler is fully written and tested, waiting behind one environment variable.

  In the other direction, one research item the app reasonably declines is a forced fade of vowel points. The research says it is untested, and the app offers a toggle instead.

## Assessment

- **Domain summary:** a Hebrew-learning single-page web app for adult English speakers. It has lessons built from AI-authored plans (through MCP) or an admin editor, a cross-lesson review queue, a daily mix, verb practice, a word list, and progress. It runs in a browser on desktop and phone, behind a login.
- **Tech stack:** Node.js with no framework and no build step, `node:sqlite`, and vanilla JS in the browser. ElevenLabs `eleven_v3` provides batch text-to-speech (`server/audio.js:17`), with browser `speechSynthesis` as fallback. OpenAI provides images. It deploys to Kubernetes.
- **Complexity:** 1,235 files, about 230 thousand lines in total (134 thousand of them JavaScript). Discovery snapshot run on 2026-10-17.
- **Opportunities:** turn on FSRS, which is already built. Make recall overt. Unify miss feedback through the existing `HebrewFeedback.showMiss` helper. Add perception drills using ElevenLabs' multiple voices. Extend the existing recorder into model-versus-learner comparison.
- **Risk areas:**
  - Review load under the 30-day ceiling (`shared/player/review.js:8`, `:105`).
  - Verb progress that never reaches the server (`shared/verb-practice.js:313–338`).
  - The picture on the "Hebrew → English" recall card gives the answer away (`shared/activities/words.js:230–245`).
  - Infinite pulse animations (`shared/worksheet.css:403`, `:3294`, `:3516`).

## Persona-Lens Findings

| Lens | Headline finding | Evidence (Tier A) |
| --- | --- | --- |
| Software Architect | The seams are clean: the scheduler, activities, feedback helper, and validator are separate modules. Every fix lands in one or two files, so a rewrite is not justified. | `shared/player/review.js`, `shared/player/fsrs.js`, `shared/player/feedback.js`, `server/plans/validate.js` |
| Data Architect | The main review state is server-owned, but verb-practice progress is saved to `localStorage` only and has no server route. Nothing counts how many times or in how many sentences a word has been met (RSCH-003 rates the encounter link Moderate). | `shared/verb-practice.js:313–338`; grep for `saveLadder` and verb routes in `server/` returned nothing; grep for `encounter` and `times_seen` returned nothing |
| Security & Compliance | Every signed-out HTML request redirects to `/login/`. A public sample lesson (RSCH-007) would be the first unauthenticated content route and needs its own security review. | `server/web.js:411–417` |
| Platform / Cloud | Production pins `SRS_SCHEDULER=ladder`. The FSRS rollout runbook exists and requires written operator approval to flip. Audio uses one ElevenLabs voice through an env variable, with no slow variant. | `deploy/k8s-database.md:111–112`; `docs/operations/language-program-rollout.md:16`; `server/audio.js:420–421`; `shared/player/speech.js:127` |
| Day-2 Operations | With a 30-day ceiling and a 30-card daily cap, every known word costs about 12 reviews a year. Roughly 2,500 known words would fill the cap every day. FSRS (up to 365 days at 0.9 retention) removes that growth. | `review.js:8–9`, `:105`; `fsrs.js:36–47` |
| Capability-Parity / Reuse | About half the gaps are reuse, not build: FSRS is written, `showMiss` already has a retry option, and the recorder already captures audio. Perception drills and an attempt-before-reveal review card are true builds. | `fsrs.js`; `feedback.js:144–193`; `core.js:849–912` |
| Agentic / Judgment | Almost everything is deterministic: scheduling, grading typed Hebrew, and UI. The only judgment-heavy candidate is grading free speech or writing (retell, ask-friend, pronunciation). The research grade is Moderate for pronunciation feedback and Low for output feedback, so spike it before committing. | `shared/activities/retell.js:11–28`; `ask-friend.js:17–36` (output is collected and never assessed) |

Conflict surfaced, not averaged: the Security lens would keep login gating as-is, while RSCH-007 calls a public sample lesson the biggest ease lever. That is an operator decision, recorded in Risks and placed last in the skeleton.

## Capability & Readiness Scoring Matrix

The score is the app against the research target (5 means it fully does what the research says). The grade is the research's own confidence for that item.

| Dimension | Research grade | App | Notes (Tier-A evidence) |
| --- | :---: | :---: | --- |
| *Capability: learning loop* | | | |
| Spaced recall on later days | High | 4 | Ladder `[1,3,7,14,30]` and same-day pass is a no-op (`review.js:8`, `:99–103`). Ceiling: `Math.min(step+1,4)` caps at 30 days (`:105`). RSCH-003: longer gaps help delayed retention (Moderate). |
| Active, honest retrieval | Moderate | 3 | Review is "Show answer" then a self-grade from 1 to 4 (`review-page.js:605–615`, `:767–772`). Typing is optional and only offered on English→Hebrew (`:591–593`). Lessons do require typed recall (`type-it.js`). |
| Both directions; typed and spoken recall | Moderate / Low | 3 | Both directions scheduled. Typed recall is optional in review. No spoken recall. No move from recognition to recall as words mature. |
| Interleaving | Moderate | 4 | Due queue avoids runs from one lesson; verb practice mixes tenses (`verb-practice.js` interleave). |
| Feedback: one retry, then the answer | Moderate / Low | 3 | Fill, listen-pick, pattern, exercise, and verb practice conform. Type-it reveals on the first miss (`type-it.js:203–209`). Build-sentence passes `showRetry: false` (`build-sentence.js:173–176`). Match allows endless retries with no reveal (`match.js:81–91`). |
| English glosses and picture rules | Moderate | 4 | Meaning always shown at introduction (`words.js:186`). Tap-to-gloss in stories. **But** the Hebrew→English recall card shows the picture while hiding the meaning (`words.js:230–245`), which hands over the answer (RSCH-005 rule 4, and [4] on overconfidence). |
| Nikud and script support | Low | 3 | Points on by default with a learner toggle, no forced fade. No look-alike letter drills (bet/kaf, dalet/resh, final forms; RSCH-005 §1.5). |
| Perception (listening discrimination) training | Moderate | 1 | Absent. Grep for minimal-pair and contrast drills returned nothing; listen-pick is word-to-meaning, not sound contrast. |
| Pronunciation practice with feedback | Moderate | 2 | Record and playback only (`core.js:849–912`); no `SpeechRecognition` anywhere; no model-versus-learner comparison outside ask-friend. |
| Speaking toward a message, with feedback | Low | 2 | Retell, ask-friend, and personal-question collect output and give no feedback. |
| Course design rules (RSCH-004 §3) | Moderate / Low | 4 | `N1` caps at 8 new words, `N5` at one pattern, `N7` keeps language-focused work to a quarter, `N12` covers fluency, plus `later-context` and `pointed-word` (`validate.js:40–44`, `:62–76`, `:97–100`, `:158–160`, `:183–185`). Not checked: frequency band, semantic set. |
| Coverage-graded input (about 95% known) | Low | 3 | The daily mix filters reading at `MIN_COVERAGE = 0.95` (`mix.js:37`, `:211`); `N3` warns at authoring time. Per-learner coverage is not used to flag lesson stories. |
| Learner control of audio pace | Moderate | 2 | Replay exists; speech fallback rate is fixed at 0.85 (`speech.js:127`); no slow control on clips (no `playbackRate`); one voice. |
| Habit cue and study-time log | Low | 1 | Absent (grep: no reminder, goal, or time-spent). Low evidence, so low priority. |
| **Capability (average)** | | **≈ 2.8** | The high-evidence rows score 3–4; the 1–2 rows are missing features or Low-evidence items. |
| *Readiness: interface, first run, data* | | | |
| Accessibility (WCAG 2.2 AA, Hebrew size, targets) | High | 4 | Hebrew word is 48px with line-height 1.5 (`worksheet.css:1920`, `:1927`); primary target 48px (`:19`); light theme default. Some glosses are not wrapped in `dir="ltr" lang="en"` (`words.js:186`). |
| Motion and sound (RSCH-008) | Moderate | 4 | Sound off by default with a visible toggle, a 440 Hz miss tone, and no overlap with speech (`feedback.js:10–11`, `:40–47`, `:89–104`); shake removed. Three `infinite` pulses remain (`worksheet.css:403`, `:3294`, `:3516`). |
| No points or streaks; progress shown as ability | Low (operator rule) | 5 | No XP, leaderboard, or streak UI. Progress shows words by memory stage. |
| First run: value before signup | Moderate | 1 | Every signed-out HTML request redirects to login (`web.js:411–417`). |
| One next action; 3–5 nav destinations with icon and label | Low / Moderate | 2 | Today stacks up to six cards (`today-page.js:977–983`). Seven learner nav items, and Guides and Verbs have no icon (`app-shell.js:20–30`, `:36`). The phone bottom dock is done. RSCH-007 (2026-10-09) rated Today as conforming. The mix, review, and verb cards and the Guides and Verbs nav items were added later, so this is drift since that report. |
| Program list: you are here, what's next, more than one way to find a lesson | Low (WCAG AA/AAA) | 2 | No `aria-current`, no named next lesson, no search on the lessons page (grep returned nothing). |
| Progress follows the learner across devices | n/a (product) | 3 | Review is on the server; verb ladder is `localStorage` only. |
| **Readiness (average)** | | **≈ 3.0** | |

## Technical Component Analysis

| Source unit | LOC | Nature | Target home | Existing support | Status | Rationale (cite behaviour) |
| --- | ---: | --- | --- | --- | --- | --- |
| `shared/player/review.js` (ladder) | 441 | Deterministic | Keep as fallback | FSRS-6 in `fsrs.js` (353 LOC), runbook in `docs/operations/language-program-rollout.md` | Provided (flip) | 30-day ceiling conflicts with "lengthen intervals"; FSRS goes to 365 days |
| `shared/review-page.js` (reveal and self-grade) | 1,163 | Presentation | Frontend | `flashcard-answer.js` `suggestGrade` already scores typed Hebrew | Adapt | Require an attempt before reveal, auto-grade typed answers, add typed English for Hebrew→English |
| `shared/activities/words.js` (recall cards) | 270 | Presentation | Frontend | `vocabImage()` | Adapt | Hide the picture on Hebrew→English recall (`:230–245`) |
| `type-it.js`, `build-sentence.js`, `match.js` | 607 | Presentation | Frontend | `HebrewFeedback.showMiss({showRetry})` | Adapt | Align with fill and listen-pick: one retry, then the pointed answer |
| `shared/verb-practice.js` ladder storage | 1,227 | Persistence | Data and API (server) | review store and `/api/me` pattern | Adapt | Saved to `localStorage` only (`:313–338`); progress is lost across devices |
| Perception drills | n/a | Deterministic and content | Frontend activity plus audio batch | `listen-pick.js`, ElevenLabs batch (`server/audio.js`) | Build | RSCH-003 HVPT (high-variability phonetic training): Moderate, gains retained |
| Pronunciation and output feedback | n/a | Judgment | Spike, then possibly an AI grader | recorder `core.js:849–912` | Build (spike) | Moderate for pronunciation feedback; Hebrew speech-recognition quality unknown |
| `shared/player/speech.js` pace | 402 | Presentation | Frontend | `<audio>` element | Adapt | Add a 0.75× toggle through `playbackRate`; RSCH-003 says the learner controls the pace |
| `today-page.js`, `app-shell.js`, `lessons-page.js` | 2,095 | Presentation | Frontend | dock, sprite icons | Adapt | One primary action, at most 5 destinations each with an icon, `aria-current`, named next lesson, search |
| Public sample lesson | n/a | Integration and security | Server route plus frontend | `isPublicStatic` pattern (`web.js:388–397`) | Build (operator call) | RSCH-007's top lever; first unauthenticated content |
| `server/plans/validate.js` | n/a | Deterministic | Keep | n/a | Provided | Already encodes RSCH-004 rules; frequency and semantic-set stay open because the research has no Hebrew list |
| Encounter and context counter | n/a | Persistence | Data | `practice_events` | Build (later) | Moderate association, Low threshold; useful for recycling decisions |

## Recommendation

Refactor, confidence 0.85.

### Recommendation Options Key

- **Option 1: No action.** Not recommended. Real gaps sit on Moderate-evidence items: honest retrieval, feedback consistency, perception, pronunciation, and the review ceiling.
- **Option 2: Targeted refactor.** **Recommended.** The seams are clean, about half the fixes reuse existing code, and every change is one domain.
- **Option 3: Refactor plus an AI judgment layer now.** Not recommended yet. Hebrew speech-recognition and grading quality are unknown; spike first, then upgrade to Hybrid if the spike passes.
- **Option 4: Greenfield rewrite.** Not recommended. The architecture already follows the high-evidence research, and a rewrite risks the parts that work.

**Why:** the scoring matrix shows the core memory loop at 3–4 on the strongest-evidence rows. The gaps are local to named files: `review.js:105`, `review-page.js:591–615`, `type-it.js:203–209`, `build-sentence.js:173–176`, `match.js:81–91`, `words.js:230–245`, and `verb-practice.js:313–338`. Or they are new activities that plug into the existing player. Capability and readiness are kept separate: the interface already meets WCAG-level sizing (readiness 4), while first-run and navigation lag (readiness 1–2). Both can be improved independently of the learning loop.

**Ranked improvements: can we do it better?** Ranked by research grade, then cost.

1. **Raise the review ceiling: flip to FSRS.** Moderate evidence for longer gaps; the code exists; operator-gated. Cost: one env change plus a backup, following the runbook.
2. **Make review recall overt and honest.** Ask for an attempt before reveal: typing, or "say it aloud" then reveal. Grade typed answers automatically. Move each word from recognition (choose) to recall (type) as it matures. Hide the picture on Hebrew→English recall. Moderate evidence (RSCH-003 retrieval and format; RSCH-005 overconfidence).
3. **One feedback pattern everywhere.** Type-it, build-sentence, and match get one retry, then the pointed answer. Moderate for the elaborated correction, Low for the exact one-retry sequence. Low cost, because the helper exists.
4. **Hear the hard sounds apart.** Minimal-pair listening drills for ח/כ/ה, א/ע, ט/ת, כ/ק, ש/ס, and stress pairs, with instant right/wrong feedback and two or more voices. Moderate (RSCH-003 HVPT).
5. **Pronunciation with feedback.** Everywhere the recorder exists, play the model and the learner's recording side by side. Spike browser `he-IL` speech recognition as an optional check. Moderate for feedback; tech unknown.
6. **Audio pace.** A 0.75× toggle on clips. Moderate (RSCH-003 learner-paced audio).
7. **Server-side verb progress**, so desktop and phone agree, which INIT-013's own goal requires.
8. **Simplify the front door:**
   1. Today shows one primary action.
   2. Nav drops to five or fewer destinations, each with an icon.
   3. The lessons page marks "you are here", names the next lesson, and adds search.

   Low to Moderate (RSCH-006 rule 16, RSCH-007 §2 and §4, RSCH-010 and WCAG).
9. **Small accessibility fixes.** Wrap English glosses in `dir="ltr" lang="en"`, rated High (W3C). Cap or remove the three infinite pulses (WCAG 2.2.2 when longer than 5 seconds).
10. **Public sample lesson.** Moderate (RSCH-007), but an operator and security decision.
11. *Later, Low evidence:* an encounter and context counter; an optional first-run goal and study-time log.

**Open unknowns (confidence < 0.9):**

- How much production content exists. The learner database and lesson volume are out of scope (application data was not read); the repo holds 3 worksheets plus seed scripts. This affects how fast the 30-day ceiling bites. *Spike:* run `npm run stage:counts` on production.
- Hebrew speech-recognition quality in the browser. *Spike:* a two-hour test of `webkitSpeechRecognition` with `lang='he-IL'` on 20 lesson words.
- Whether learners use the optional typed field today. *Spike:* count `practice_events` with a typed value.

## Evidence & Falsification

- **Load-bearing claims:** all Tier A (ledger below). Explorer-agent summaries were Tier C until the cited lines were re-read; two of them were corrected on re-read. The ladder is the production default, not FSRS. The miss tone is 440 Hz, not 220 Hz.
- **Falsification pass:**

**Hypothesis:** the app should be refactored in place to close specific research gaps; no rewrite and no agent package.

| # | How it could be wrong | Tested against (Tier-A evidence) | Survived? |
| --- | --- | --- | :---: |
| F1 | "Show answer, then self-grade" *is* retrieval practice (the learner recalls silently), so the retrieval gap is overstated. | `review-page.js:605–615`; RSCH-003 §1.1 (retrieval before reveal counts); RSCH-003 "Do" list asks for typed or spoken recall *in addition*; RSCH-005 [4] says learners overrate easy cues. | Partly. Covert recall counts, so this was **downgraded from CONTRADICTS to PARTIAL**. The gap that remains is overt production and honest grading. |
| F2 | The ladder already conforms (equal intervals are as good as expanding ones), so FSRS adds nothing. | `review.js:8`, `:105` (cap at step 4, 30 days); RSCH-003 §1.1 (equal ≈ expanding, Moderate; longer gaps help delayed retention, Moderate); `DEFAULT_CAPS.due = 30` (`:9`). | Partly. The interval *shape* conforms; the *ceiling* does not and creates growing load. **The finding was narrowed** to the ceiling. |
| F3 | Login gating is a deliberate choice for a private app, so "value before signup" is not a gap. | `web.js:411–417`; RSCH-007 §1 calls it "a genuine gap, not a decided tradeoff"; commit `59d3367` lets Google sign-in create new learners. | Partly. **Reclassified** as an operator decision, not a defect, and moved to the last phase. |
| F4 | The validator only warns, so content conformance is unenforced and the "4" is inflated. | ADR-0013 §Summary (warnings by operator decision D1–D8); `validate.js` blocks on `N1`, `N5`, and `N8`–`N13` (plus integrity codes) and warns on the research-shaping codes (`N2`, `N3`, `N6`, `N7`, `later-context`, `pointed-word`). | No. Warnings are a recorded design choice, and the hard caps do block. The score stands. |

- **Independent evaluation:** **PASS**. Judge rating 4/4 (score 1.00), property checks 16 passed and 0 failed, 12 of 12 acceptance criteria, faithfulness 1.0. The judge's five follow-ups were applied: the F4 wording, C17 re-read directly, a line-number fix, the RSCH-007 drift note, and a shorter BLUF. See `eval/ASMT-001-hebrew-web-app-eval-report.md`.

**Result:** the recommendation holds. F1 and F2 lowered the severity of two findings without changing the recommendation, and F3 moved one item to an operator decision. Confidence is 0.85: all load-bearing claims are Tier A, and one unknown (production content volume) affects priority but not direction.

### Claims-Evidence Ledger

| # | Claim | Type | Tier | Source(s) | Method | Load-bearing? | Status |
| --- | --- | --- | :---: | --- | --- | :---: | --- |
| C1 | Production uses the ladder, not FSRS | readiness | A | `server/progress.js:53–63`; `deploy/k8s-database.md:111–112` | read | yes | verified |
| C2 | Ladder caps at 30 days | capability | A | `shared/player/review.js:8`, `:105` | read | yes | verified |
| C3 | Same-day pass does not advance the schedule | capability | A | `review.js:99–103` | read | yes | verified |
| C4 | Review is reveal-then-self-grade; typing optional, English→Hebrew only | capability | A | `shared/review-page.js:591–615`, `:755–772` | read | yes | verified |
| C5 | Type-it reveals on first miss with no retry | capability | A | `shared/activities/type-it.js:7–9`, `:203–209` | read | yes | verified |
| C6 | Build-sentence disables retry | capability | A | `build-sentence.js:173–176` | read | yes | verified |
| C7 | Match allows endless retries, no reveal | capability | A | `match.js:81–91` | read | yes | verified |
| C8 | Hebrew→English recall card shows the picture with the meaning hidden | capability | A | `words.js:135–143`, `:230–245` | read | yes | verified |
| C9 | No perception drills and no speech recognition | capability | A | grep `SpeechRecognition\|minimal.?pair\|playbackRate\|encounter\|times_seen` over `shared/`, `server/`, `mcp/`: 0 hits | searched | yes | verified (absence) |
| C10 | Verb progress is `localStorage` only | readiness | A | `verb-practice.js:313–338`; grep for `saveLadder` and verb routes: shared files only | read and searched | yes | verified |
| C11 | Signed-out HTML redirects to login | readiness | A | `server/web.js:411–417` | read | yes | verified |
| C12 | Today renders up to six cards; seven learner nav items, two without icons | readiness | A | `today-page.js:977–983`; `app-shell.js:20–36` | read | no | verified |
| C13 | Lessons page has no `aria-current`, next lesson, or search | readiness | A | grep `lessons-page.js`: 0 hits | searched | no | verified (absence) |
| C14 | Sound off by default, 440 Hz miss, suppressed during speech | readiness | A | `feedback.js:10–11`, `:40–47`, `:89–104` | read | no | verified |
| C15 | Hebrew word 48px, line-height 1.5, primary target 48px | readiness | A | `worksheet.css:19`, `:1920`, `:1927` | read | no | verified |
| C16 | Validator encodes RSCH-004 rules | capability | A | `server/plans/validate.js` registry (`N1`, `N5`, `N7`, `N12`, `later-context`, `pointed-word`) | read (explorer-cited lines re-checked by grep) | no | verified |
| C17 | FSRS-6 exists with 0.9 retention, 1–365 days | capability | A | `shared/player/fsrs.js:36–47` | read | yes | verified |
| C18 | Audio is one ElevenLabs voice, no slow variant | capability | A | `server/audio.js:17`, `:420–421`; `speech.js:127`; grep `playbackRate`: 0 | read and searched | no | verified |
| C19 | Infinite pulses remain in CSS | readiness | A | `worksheet.css:403`, `:3294`, `:3516` | searched | no | verified |
| C20 | Production content volume | effort | C | not inspected (application data out of scope) | n/a | no | open (spike) |

## Spec Skeleton

Grouped by domain in dependency order. One domain per spec.

### Phase 1: Foundation (reuse)
- SPEC-001 · Infrastructure: flip `SRS_SCHEDULER` to `fsrs` per `docs/operations/language-program-rollout.md` steps 1–5 (backup, stage counts, flip, verify, rollback ready). Gate: written operator approval.
- SPEC-002 · Data: server-side verb-practice ladder table and repository; migrate the existing `localStorage` entries on first sign-in.
- SPEC-003 · API: `GET/POST /api/me/verbs` for the ladder; include verb due counts in the Today model.

### Phase 2: Learning loop (frontend, mostly adapt)
- SPEC-004 · Frontend: review card asks for an attempt before reveal (typed Hebrew for English→Hebrew; typed English or choice for Hebrew→English), auto-grades typed answers from `suggestGrade`, moves each word from choice to typing by memory stage, and hides the picture on Hebrew→English recall.
- SPEC-005 · Frontend: one miss pattern for type-it, build-sentence, and match through `HebrewFeedback.showMiss` (one retry, then the pointed answer).
- SPEC-006 · Frontend: audio pace toggle (0.75× through `playbackRate`); cap the three infinite pulses; wrap glosses in `dir="ltr" lang="en"`.

### Phase 3: New practice types
- SPEC-007 · Service (content and audio): minimal-pair word list for ח/כ/ה, א/ע, ט/ת, כ/ק, ש/ס, and stress, generated in two or more ElevenLabs voices into `worksheets/_audio/`.
- SPEC-008 · Frontend: "Hear the difference" activity (play, pick one of two, instant feedback), added to the daily mix.
- SPEC-009 · Spike (time-boxed, 1 day): model-versus-learner comparison wherever the recorder exists, plus a test of browser `he-IL` speech recognition on 20 words. Decide whether an AI output grader is warranted. That decision is the Hybrid trigger.

### Phase 4: Front door and navigation
- SPEC-010 · Frontend: Today shows one primary action; nav consolidated to five or fewer destinations, each with icon and label; lessons page with `aria-current`, a named next lesson, and search.
- SPEC-011 · Frontend + server (`security_review: required`, operator decision): public read-only sample lesson with "save your progress" signup.

### Later (Low evidence)
- SPEC-012 · Data: encounter and distinct-sentence counter from `practice_events`, exposed to the plan builder for recycling.

## Risks

| Risk | Probability | Impact | Mitigation |
| --- | --- | --- | --- |
| FSRS flip mis-schedules existing cards | L | M | Runbook backup and stage counts first; ladder fields keep updating, so rollback is one env change |
| Required typing makes review feel slower, and learners skip it | M | M | Keep "say it aloud, then reveal" as the low-effort path; auto-grade so the learner taps less, not more |
| Hebrew speech recognition is too weak for pronunciation checks | M | L | SPEC-009 is a spike; side-by-side playback ships regardless |
| Public lesson widens the attack surface | M | M | `security_review: required`; read-only, no API writes, static content only |
| Minimal-pair audio from TTS blurs the very contrasts being trained | M | M | Native-speaker spot-check before release (RSCH-001 recommends a human model for sounds; RSCH-002 option E) |
| Nav consolidation hides Guides or Verbs from current users | L | L | Fold them into Lessons and Review as tabs, not removals |

## Next Step

Proceed to SDD `create-initiative` with the spec skeleton above as the pre-analysed decomposition, setting `source: ASMT-001`. SPEC-001 needs the operator's written approval before it runs, and SPEC-011 needs an operator decision.
