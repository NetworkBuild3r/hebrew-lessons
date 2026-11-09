# Assessment Report: Hebrew Web App, Next Gains in the Learning Experience

**Assessment ID:** ASMT-003
**Status:** Complete. Eval PASS on the second independent round (round 1 was MITIGATE); both are in Evidence & Falsification.
**Source:** `c:\Users\BrianNelson\Projects\Hebrew`, the Go app (hebrew v3), at `main` = `origin/main` = `d1e9490` after `git fetch` (0 ahead, 0 behind). The working tree has about 46 modified files whose diffs are line endings only (`git diff -w` is empty), plus untracked files outside the app (`Aharon's code/`, timer files). Every `file:line` below is from `d1e9490`.
**Assessed:** 2026-11-07
**Analyst:** AI agent (Cursor, Claude), read-only
**Recommendation:** Refactor
**Confidence:** 0.85
**Pipeline mode:** Assessment report and spec skeleton. Nothing in the app was changed.

---

## Bottom Line Up Front (BLUF)

**Recommendation: Refactor, meaning targeted changes inside the current app. Fix one number so "known" means remembered on a later day, then add the three kinds of practice no screen offers yet: hearing hard sounds apart, saying words with feedback, and reading and listening at volume.** Spacing, the one High-evidence finding, now scores 5 alongside checked recall, so the next gains sit in Moderate-evidence strands that still score 1–2. Today a word can count as "known" on the day it was taught, from three same-day right answers that may include self-reported "Got it" checks, which inflates "Words you know" and the known-word set that decides how hard lesson text may be. All of it fits the current Go app and AI lesson author, with no new stack to run or secure: about 17 small specs (roughly 35–100 hours of build) plus a one-day speech-recognition spike.

---

## Objectivity & Independent Validation

- **Who wrote this.** The same kind of AI agent that wrote ASMT-001 and ASMT-002 wrote this report. It has no stake in the result, but it shares their method and may share their blind spots. Two checks guard against that. First, every load-bearing claim was re-read in the cited file this session, including claims first raised by helper explorer agents, which were treated as leads only. Second, a fresh judge agent that did not write the report scored it against a fixed oracle. Its first pass found three wrong research-grade labels and a method section ahead of this summary; those were fixed and the report was re-scored (Evidence & Falsification).
- **What was not looked at.** No application data was read: not `data/`, not the SQLite database, and no learner records. So this report can show that a mechanism exists, for example that "known" can be reached on the first day, but not how often it happens. That limit is carried into Falsification and the open unknowns. The app was not run, and no learner was observed.
- **Tooling note.** The skill's Python launcher (`<KIT>/scripts/dev/run_py.sh`) is missing on this machine. The bundled scripts were run directly with `python -I` from their absolute kit paths, which keeps them isolated from the working directory. The discovery output was written to a temp file, not into the source tree.
- **Falsification.** Five attempts were made to overturn the findings. Each one narrowed a finding, and none reversed the recommendation (Evidence & Falsification).
- **ID clash.** [ADR-0030](../../../docs/decisions/ADR-0030-typed-spelling-counts.md) line 24, committed today in `6237152`, says "ASMT-003 measured the gap" for typed spelling, using production data ("40 of 44 successful spelling reviews were implied"). That is not this report. No ASMT-003 folder or registry row existed before this one, and no other Cursor chat mentions it. So that label belongs to an unsaved assessment from another session. This report took ASMT-003 by the skill's numbering rule (the next number after the folders on disk) and does not contain that measurement.
- **Phonetics.** Which Hebrew sounds an English speaker actually confuses is analyst domain knowledge, not something the repo or the research reports establish. This report keeps that claim out of the decision and makes a native-speaker check the first step of the hearing work.

---

## Executive Summary

**Options key**

1. **No action.** Keep building lessons as they are. This leaves three Moderate-evidence strands at 1–2 and the "known" inflation in place.
2. **Targeted refactor (recommended).** Five phases inside the current app: trustworthy "known", hearing, saying, reading and listening volume, and closing the measurement loop. It fixes the weakest rows and reuses the brief, validator, voices and drill machinery.
3. **Refactor plus an AI judge now.** Option 2, plus building automatic pronunciation scoring or AI conversation partners now. This is premature: Hebrew speech-recognition quality is unknown, so spike it first (SPEC-010).
4. **Rebuild.** Start again. Not justified, because the memory loop largely conforms and the seams are clean.

**Where the app is now.** The memory loop largely matches the research:

- **Scheduling and review:** FSRS-5 is the only scheduler (`internal/fsrs/review.go:23-36`). Review is a checked test, asked once per sitting ([ADR-0024](../../../docs/decisions/ADR-0024-review-is-a-checked-test.md) D1–D3).
- **Directions and formats:** words are practised in four directions (listen, read, recall, spell). There is typed spelling and dictation, and a sentence-build step from [ADR-0025](../../../docs/decisions/ADR-0025-practice-that-teaches.md).
- **The lesson brief:** each lesson is built from a brief that sets pace, consolidates when the learner is overloaded, brings back struggling words, and aims for 95% known-word coverage (`internal/brief/build.go:28-122`). The validator then requires each unit to follow a pattern sequence, reuse earlier words and cap bare drills at 40% (`internal/units/shape_rules.go`).
- **Nikud fade:** vowel marks fade on their own for well-known words (`internal/fsrs/types.go:76-97`).
- **Results by practice kind:** the app counts later-day results per kind of practice (`internal/evidence/later_day.go:14-49`).

**What it is missing.** Each gap is checked against code and research:

1. **"Known" has no later-day test.** A skill becomes known after three successes, with recall probability at least 0.85 and the newest three answers right (`internal/learner/state.go:148-151`, `:167-192`). Nothing requires one of those to come on a later day. Self-reported speaking checks count as successes (`internal/evidence/recorder.go:286-326`, `internal/db/learner.go:319`).
   - **Where it matters.** This "known" state sets the comfortable-text tier (`internal/brief/build.go:534`) and the validator's coverage set (`internal/mcpstore/learner.go:51-71`).
   - **A looser second set.** The AI author also receives `knownLexemes`: every word whose latest read review is Good (`internal/db/placement.go:614-632`; described in `internal/mcp/tools.go:617`). A normal or fast right lesson recall writes that Good, because lesson recall grades the read card (`internal/web/lesson.go:683-692`). A slow right answer is graded Hard (`internal/evidence/grade.go:41-42`) and is left out (`internal/db/placement.go:625`).
2. **No listening-perception training.** No screen trains hearing hard sounds apart. The project's own guidance describes such a drill (`internal/units/rules.go:234-235`), but nothing builds it.
3. **No pronunciation feedback.** The Go app has no recording at all. The speaking step is "I said it", then the model plays, then a self-check (`internal/web/lesson.go:52-55`).
4. **Little easy reading and listening.** Lesson stories have a floor of 12, 30 or 50 words by tier (`internal/brief/build.go:118-120`). Outside lessons there is nothing to read or listen to except Biblical Isaiah (`internal/web/shell.go:100-118`).
5. **Slow audio only in placement.** Only placement can slow audio down (`internal/web/static/js/placement.js:62`), against the project's own guidance A4 (`internal/units/rules.go:228`). This is part of the hearing strand, with item 2.

**Capability vs readiness.**

- **Capability averages 3.3 / 5,** up from 2.8 in ASMT-001.
- **Readiness averages 3.0 / 5.** It is held down by what "known" means, because that state feeds the headline, the comfortable-text tier and the coverage check.

**Harvestable deltas** are pieces the app already has that the fix can reuse:

- placement's 0.75× audio code;
- the letter-drill machinery, a separate drill kind that clears after three right answers (`internal/session/doc.go:5-12`);
- two ElevenLabs voices (`internal/audio/voices.go:11-23`);
- the coverage validator and tier rules a graded reader needs;
- an AI author skill that can be cloned for readers;
- the `rule_version` history rebuild used by [ADR-0030](../../../docs/decisions/ADR-0030-typed-spelling-counts.md), which can re-score past data when "known" changes.

**Decision needed from the operator.** Two calls belong to the operator:

- Should self-reported speaking checks keep counting toward "known"? Scheduling from them was a recorded choice; counting them toward "known" was not.
- Is a lower "Words you know" number acceptable once "known" needs a later day?

---

## Assessment

**How this assessment is adapted.** The sdd-assess method asks where a codebase should live on a platform. Here it is used the same way as [ASMT-001](../ASMT-001-hebrew-web-app/ASMT-001-hebrew-web-app-assessment-report.md) and [ASMT-002](../ASMT-002-hebrew-web-app/ASMT-002-hebrew-web-app-assessment-report.md): the "platform" is the project's own research base (`sdd/research/` RSCH-001 to RSCH-011), and the source is the app.

- **Capability** means how much of the research-backed learning loop the app delivers.
- **Readiness** means whether the app's numbers can be trusted to drive the next lesson, and whether its content and audio are safe to learn from.
- **Refactor** means targeted changes inside the current Go app and its AI authoring path. **Agentify** would mean moving behavior into a new AI agent. **Hybrid** would mean both. **No action** would mean the research is already met.

The question asked was "How do we improve the learning experience even further?" ASMT-001 asked the same question of the old app on 2026-10-17 and scored the learning loop 2.8 / 5. Since then the project rebuilt the app in Go ([ADR-0018](../../../docs/decisions/ADR-0018-v3-adaptive-learning.md)) and shipped most of ASMT-001's memory-loop items. This assessment measures the rebuilt app against the same research and asks what is left.

**Domain.** A Hebrew course for one adult English speaker at a time. AI agents write per-learner units over MCP from a lesson brief, and a validator is the only publish gate ([ADR-0018](../../../docs/decisions/ADR-0018-v3-adaptive-learning.md) D5, lines 107-118). The app runs review, lessons, placement, word pages, a roots page and a progress page.

**Stack.** Go, with server-rendered templ views and htmx, a little JavaScript, and SQLite (migrations 001–022). Discovery counted 2,167 files and about 325,000 lines, of which about 184,000 are Go in 613 files. The largest packages are `web` (about 32,500 non-test lines), `db` (12,600), `mcp` (7,300) and `units` (3,800).

**Mode.** Brownfield. The seams are clean: scheduling, evidence recording, the learner model, the brief, the validator and the views are separate packages. Each recommended change lands in one package.

**What changed since ASMT-001.** ASMT-001 ranked eleven improvements. The learning-loop items now stand as follows:

| ASMT-001 item | Status in the Go app | Evidence |
| --- | --- | --- |
| Raise the review ceiling (FSRS) | Done | `internal/fsrs/review.go:23-36` |
| Review asks, checks, then reveals | Done | ADR-0024 D1–D3; `internal/session/session.go:588-638` |
| One consistent feedback pattern | Changed: one try, then the answer is shown | ADR-0024 D3 (review); `internal/web/lesson.go:39-43`, `:56-59` (lesson questions reveal after one answer) |
| Hear hard sounds apart | Not built | Absence search below |
| Pronunciation with feedback | Not built. The old app's recorder was not ported | Absence search below; INIT-020 line 36 |
| Learner controls audio pace | Placement only | `internal/web/static/js/placement.js:62` |
| Public sample lesson | Still gated | `internal/web/web.go:33-40` |

**Absence searches.**

- **Recording, speech recognition and slow audio.** `internal/` was searched for `MediaRecorder`, `getUserMedia`, `SpeechRecognition`, `webkitSpeech`, `he-IL`, `playbackRate`, `minimal pair` and `perception`. The only hits are `placement.js:62` (`a.playbackRate = rate || 1`), its browser test, the guidance text at `internal/units/rules.go:234-235`, a schema hint at `internal/mcp/schema.go:343`, and two tests of that guidance text.
- **Reminders and reading.** `notification`, `remind`, `webpush`, `graded reader` and `extensive` found no product code.
- **Study time.** Minutes studied exist only in the coach digest (`internal/mcpstore/digest.go`, `internal/jobs/weekly.go`), not on any learner page. `internal/web` shows minutes only as a time estimate for today's reviews (`internal/web/views/today.go:114-136`).

**In flight, no overlap.** [INIT-031](../../initiatives/INIT-031-expose-revision-workflow/INIT-031-expose-revision-workflow-initiative.md) covers the revision workflow and [INIT-032](../../initiatives/INIT-032-one-right-answer/INIT-032-one-right-answer-initiative.md) one right answer per question. INIT-032 SPEC-001 targets `ADR-0028-one-right-answer.md`, but ADR-0028 is already taken (`ADR-0028-where-you-stand.md`), so INIT-032 will likely take ADR-0029, the lowest free number today. The ADR proposed here should take the next free number when it is written, likely ADR-0031.

---

## Persona-Lens Findings

Each lens's headline rests on Tier A evidence read this session.

- **Software Architect.** Every change fits an existing seam:
  - the known rule lives in one function (`internal/learner/state.go:167-192`), and the tier and coverage check read it through the learner model;
  - the looser `knownLexemes` set lives in one query (`internal/db/placement.go:614-632`);
  - a sound drill can be a new drill kind next to letter drills (`internal/session/doc.go:5-12`, `internal/session/drill.go`);
  - audio pace is a client control that already exists in placement.

  No new service is needed.
- **Data Architect.** "Known" is not yet a retention claim. A word can be "known" on its first day, and in `knownLexemes` after one right answer:
  - the success count includes self-reported rows (`internal/db/learner.go:319` excludes only implied rows that are not spelling);
  - `StateOf` has no later-day condition (`internal/learner/state.go:167-192`), and that state drives the tier (`internal/brief/build.go:534`) and the coverage check (`internal/mcpstore/learner.go:51-71`);
  - `knownLexemes` is "latest read review Good or Easy" (`internal/db/placement.go:614-632`). [ADR-0027](../../../docs/decisions/ADR-0027-numbers-you-can-trust.md) D1 (line 37) keeps that set for the brief and `knownLexemes`, and it also picks the suggested course start (`internal/db/placement.go:1060`).

  [ADR-0024](../../../docs/decisions/ADR-0024-review-is-a-checked-test.md) D3 (line 33) already says same-session repeats are practice, not the review.
- **Security & Compliance.** A recorder is allowed only if it never leaves the device:
  - [ADR-0022](../../../docs/decisions/ADR-0022-learner-day-and-outside-teaching.md) D3 (line 29) says the app does not store recordings;
  - INIT-020 deferred recording until there is a microphone-permission design ([initiative line 36](../../initiatives/INIT-020-speaking/INIT-020-speaking-initiative.md));
  - any speech-recognition spike sends learner audio to a vendor, so it needs a privacy review before it touches real learners;
  - the signed-out redirect (`internal/web/web.go:33-40`) stays an operator call.
- **Platform / Cloud.** The voice service has two voices, female and male (`internal/audio/voices.go:11-23`). Minimal pairs and reader audio add clip volume. [ADR-0019](../../../docs/decisions/ADR-0019-hybrid-learning-model.md) line 49 recorded low credits; the current balance was not checked, so budget is a risk to confirm, not a finding.
- **Day-2 Operations.** The app can already measure the effect of these changes:
  - later-day results by practice kind (`internal/evidence/later_day.go:14-49`, 60-day window, at least 20 answers before a share counts);
  - per-learner retention;
  - a `rule_version` history rebuild, already used once for typed spelling ([ADR-0030](../../../docs/decisions/ADR-0030-typed-spelling-counts.md) line 22 and its decisions).

  A change to "known" should bump the rule version and rebuild history the same way.
- **Capability-Parity / Reuse.** Most of what is missing is already described in the project's own guidance, which the validator does not enforce (`internal/units/rules.go:204-206`):
  - A4 is "every feedback sheet has a replay control and a 0.75× speed option" (line 228);
  - C1 and C2 are minimal-pair drills (lines 234-235).

  Placement already has the slow-audio code. The story activity, the coverage check and the tier rules (`internal/brief/build.go:372-379`) are most of a graded reader. What is net new: a recorder, a sound-contrast item bank, and somewhere to store and show readings.
- **Agentic / Judgment.** The AI author is already the judgment layer (`.cursor/skills/hebrew-lesson-author/SKILL.md`). A reader author is the same pattern behind the same validator gate. The only new judgment-heavy candidate is scoring the learner's pronunciation automatically, and its feasibility in Hebrew is unknown. That makes it a spike, not a build, and it is the one thing that could tip this to Hybrid later.

**Conflicts between lenses.**

- **Security vs Capability.** The research supports pronunciation practice with feedback (RSCH-003 line 41), but the project does not store recordings. Resolution: start with record-and-compare in the browser, discarded on leave. Feedback beyond that comes from the tutor or from a later, privacy-reviewed speech-recognition option.
- **Data vs motivation.** ADR-0027 D1 chose a broad "Words you know" so progress feels real. A later-day rule will lower that number. Resolution: show "learned today, check tomorrow" as its own state rather than hiding it, and let the operator decide in the ADR.

---

## Capability & Readiness Scoring Matrix

Each row is scored 1–5 against the research. The basis column gives the research grade (High, Moderate, Low, Very Low) of the claim being scored, with its line in the research report. "l." alone means RSCH-003. Code paths are under `internal/`.

**Capability: the learning loop**

| Dimension | Basis | Score | Evidence |
| --- | --- | --- | --- |
| Spaced, delayed recall | High (l.18) | 5 | FSRS-5 is the only scheduler (`fsrs/review.go:23-36`) |
| Checked recall before reveal | Moderate (l.68) | 5 | ADR-0024 D1; review never self-grades |
| Both directions and recall formats | Moderate for direction (l.22); Low for recall over recognition (l.23) | 4 | Four skills, typed spelling, dictation, sentence build. Lesson `produce` is still multiple choice (`web/lesson.go:44-45`) |
| Interleaving | Moderate (l.25) | 4 | Struggling words mixed with stable ones (`session/doc.go:4-5`) |
| Many encounters, recycling | Moderate (l.24) | 4 | A unit must reuse at least 4 earlier new words (`units/shape_rules.go:17-19`); each new word is used at least 3 times (`brief/build.go:61-62`) |
| Volume of easy reading and listening | Moderate (l.30) | 2 | Story floor 12, 30 or 50 words (`brief/build.go:118-120`). Nothing outside lessons but Isaiah (`web/shell.go:100-118`) |
| First-language glosses | Moderate (l.31) | 5 | Every story word is a tap card with its meaning behind it (`web/lesson.go:35-38`); word pages (`web/shell.go:100-118`) |
| Focused grammar | Moderate (l.37) | 4 | Each unit names its next pattern or reuses recent ones (`units/shape_rules.go:129-139`, `brief/progression.go:130-146`) |
| Corrective feedback | Moderate that feedback works; Low for which type (l.39) | 4 | The answer is revealed after the first pick (`web/lesson.go:39-43`); sentence build is explicitly one try (`:58`); review shows the right answer on a miss (ADR-0024 D3). No retry prompt |
| Perception of hard sounds | Moderate (l.42) | 1 | Not built. Guidance C1 is unbuilt (`units/rules.go:234`) |
| Pronunciation with feedback | Moderate (l.41) | 1 | No recording. Speaking is a self-check (`web/lesson.go:52-55`) |
| Message-focused speaking | Low (l.40) | 2 | Dialogue cards and "I said it" only (`web/lesson.go:52-55`, `:64-68`) |
| Learner controls audio pace | Ungraded checklist item (l.181; related finding l.33) | 2 | Placement only (`web/static/js/placement.js:62`), against guidance A4 |
| Habit cue and total time | Low (l.82); Moderate for the published hour figures (l.83) | 2 | No reminder. Total time is in the coach digest, not shown to the learner |
| Nikud: points on new words, then fade | Low, and fading is untested (RSCH-001 l.16, l.65) | 4 | Points shown until a read card is stable for 21 days with no recent miss (`fsrs/types.go:76-97`). The fade goes beyond the evidence. No learner toggle |
| Fixed spine of frequency and theme | Moderate (RSCH-004 l.15) | 3 | Frequency and patterns drive the brief. The theme is whatever the caller passes, default "general" (`brief/build.go:87-91`), not a fixed sequence |
| Deliberate study about a quarter of time | Moderate for the four strands (RSCH-004 l.38); Low for the exact quarter (l.39) | 4 | Bare drills capped at 40% of a unit (`units/shape_rules.go:11-13`); stories lead |
| **Average** | | **3.3** | 56 / 17. ASMT-001 was 2.8 |

**Readiness: can the numbers and content be trusted**

| Dimension | Score | Evidence |
| --- | --- | --- |
| "Known" means retained | 2 | No later-day condition; self-reported rows count (`learner/state.go:167-192`, `db/learner.go:319`) |
| Coverage set is valid | 3 | The coverage check reads that same "known" state (`mcpstore/learner.go:51-71`). `knownLexemes` is one latest Good read answer (`db/placement.go:614-632`) |
| Outcomes are measured | 4 | Later-day results by kind, retention, rule-versioned history |
| Data drives the next lesson | 3 | `practiceResults` reaches the author as advice only (`SKILL.md:142`); ADR-0025 D8 says measure first |
| Hebrew correctness | 3 | Lexicon lookup and validator; no native-speaker check before publish (ADR-0018 D5) |
| Audio model | 3 | Two TTS voices; RSCH-001 line 66 asks for a human model for sounds |
| **Average** | **3.0** | |

---

## Technical Component Analysis

One table covers every component this assessment would touch or keep. "Provided" means already done, "Adapt" means change in place, and "Build" means new. LOC counts non-blank lines, so it runs a little below the discovery snapshot's totals.

| Component | LOC (non-blank) | Nature | Home | Provided / Adapt / Build | Change |
| --- | --- | --- | --- | --- | --- |
| `internal/fsrs/review.go` scheduler | 280 | Deterministic | Service | Provided | Keep |
| `internal/session/session.go` checked review | 632 | Deterministic | Service | Provided | Keep |
| `internal/brief/build.go` lesson brief | 986 | Deterministic | Service | Adapt | Tier and coverage follow the new rule with no change here, because they read the learner model. Read `practiceResults` into the activity mix |
| `internal/units/shape_rules.go`, `rules.go` validator and guidance | 291 + 226 | Deterministic | Service | Adapt | Rewrite guidance C1 to train true sound contrasts only. Add reader checks |
| `internal/learner/state.go` known rule | 183 | Deterministic | Service | Adapt | Require one success on a later study day. New `rule_version`. The headline, tier and coverage check all follow |
| `internal/db/learner.go` success count | 761 | Deterministic (query) | Data | Adapt | Supply later-day success. Exclude self-reported rows from "known" but keep them for scheduling |
| `internal/db/placement.go` `knownLexemes` set | 1,125 | Deterministic (query) | Data | Adapt | `knownLexemes` for the author and the suggested course start (`suggestedStart`, line 1060) use the strict known rule |
| `internal/evidence/recorder.go` speaking self-check | 528 | Deterministic | Service | Adapt (operator call) | Keep scheduling. Stop counting toward "known" if the ADR says so |
| `internal/evidence/later_day.go` results by kind | 191 | Deterministic | Service | Provided | Feed into the brief |
| `internal/web/static/js/clips.js`, `activity.js` audio controls | 193 + 211 | Presentation | Frontend | Adapt | Add a 0.75× toggle everywhere, remembered per learner, reusing `placement.js` |
| `internal/session/drill.go` letter drills | 200 | Deterministic | Service | Adapt | Add a sound-contrast drill kind with audio |
| Sound-contrast item bank | new | Content + deterministic | Data + audio | Build | Real-word pairs from the lexicon, both voices, checked by a native speaker |
| Recorder on speaking, shadow and dialogue steps | new | Presentation | Frontend, browser only | Build | Record, play back next to the model, discard on leave |
| Automatic pronunciation check | new | Judgment | Spike | Build (spike) | Test Hebrew speech recognition before any commitment |
| Read-and-listen library | new | AI content + deterministic gate | MCP + data + frontend | Build (mostly reuse) | Reader brief, reader author skill, reader page with glosses and self-paced audio |
| `internal/web/progress.go` progress page | 902 | Presentation | Frontend | Adapt | Show total study time, from the digest's minutes |
| `.cursor/skills/hebrew-lesson-author` AI author | n/a | Judgment (agent) | Agent skill | Provided | Clone as a reader author |

---

## Recommendation

**Refactor**, option 2 in the options key. The memory loop largely conforms (three of its rows score 4, with named gaps), and every gap has a home in an existing package. Agentify is ruled out because the judgment layer, the AI author, already exists and needs no new runtime. Hybrid is not yet justified because the one new judgment candidate, automatic pronunciation scoring, has unknown feasibility in Hebrew; it is spiked in SPEC-010. No action is ruled out because three Moderate-evidence rows score 1–2.

**How confidence was set.** Every load-bearing claim is Tier A, read at the cited line, and the five falsification tests narrowed findings without reversing the recommendation. Under the skill's rule that puts confidence at 0.85 or above. It is held at the bottom of that band, 0.85, because no application data was read and the app was not run. Those unknowns change the order of the work, not its direction.

**Ranked improvements**, highest value first:

1. **Make "known" mean remembered on a later day** (SPEC-001 to SPEC-003). Every other number rests on it, including the comfortable-text tier and the coverage that keeps stories readable. It is the cheapest fix and it protects every other change.
2. **Slow audio everywhere** (SPEC-004). About two hours of work. It meets the project's own guidance A4 and RSCH-003 line 181.
3. **A "hear the difference" drill** (SPEC-005 to SPEC-008). Perception training is Moderate evidence (RSCH-003 line 42) and scores 1 today. It must start with a native-speaker check of which contrasts are real, because several Hebrew letter pairs sound the same in modern speech. A drill on those would train spelling, not hearing.
4. **Record and compare** (SPEC-009). The learner hears their own voice next to the model. It runs in the browser only, so nothing is stored. Pronunciation with feedback is Moderate evidence (line 41); this is the first step toward it.
5. **A graded read-and-listen library** (SPEC-011 to SPEC-015). Easy reading at volume is Moderate evidence (line 30). The AI author, the coverage validator and the tier rules already do most of the work.
6. **Close the loop** (SPEC-016, SPEC-017). Let later-day results shift the activity mix, and show total study time (line 83).

**Open unknowns**, which change priority but not direction:

- **How often "known" is reached on the first day.** Application data was not read. A one-query count on the database would settle it.
- **Hebrew speech-recognition quality.** This is what SPEC-010 tests.
- **Which contrasts English speakers actually miss.** RSCH-001 line 19 found no published list of English speakers' Hebrew errors (it looked for production errors), so a native speaker or the tutor must decide (SPEC-005).
- **Whether learners use optional reading.** Measure it after launch with exposures and later-day results.

---

## Evidence & Falsification

### Claims-Evidence Ledger

| # | Claim | Tier | Load-bearing | Evidence |
| --- | --- | --- | --- | --- |
| 1 | FSRS-5 is the only scheduler | A | Yes | `internal/fsrs/review.go:23-36` |
| 2 | Review is a checked test; a checked miss is not re-asked in the same sitting | A | Yes | ADR-0024 D1–D3 (D3 at line 33); `internal/session/session.go:588-638` |
| 3 | "Known" has no later-day condition, and that state drives the tier and coverage check | A | Yes | `internal/learner/state.go:57-70`, `:148-151`, `:167-192`; no later-day check in `internal/learner`; `internal/brief/build.go:534`; `internal/mcpstore/learner.go:51-71` |
| 4 | Self-reported speaking checks count as successes | A | Yes | `internal/evidence/recorder.go:286-326` (source self, Good on "Got it", peeked); `internal/db/learner.go:319` |
| 5 | `knownLexemes` and the suggested start use one latest Good read review | A | Yes | `internal/db/placement.go:614-632`, `:1060`, `:1086-1096`; ADR-0027 line 37; lesson recall grades the read card (`internal/web/lesson.go:683-692`) |
| 6 | Lesson recall and listen_pick schedule cards from the first day | A | Yes | `internal/web/lesson.go:610-692` |
| 7 | No perception drill, no recording, no speech recognition | A | Yes | Absence search over `internal/` (Assessment) |
| 8 | Slow audio exists only in placement | A | Yes | `internal/web/static/js/placement.js:62`; absence search |
| 9 | The project's own guidance asks for 0.75× and minimal pairs, unenforced | A | Yes | `internal/units/rules.go:204-206`, `:228`, `:234-235` |
| 10 | Story floors 12, 30, 50 words; nothing to read outside lessons but Isaiah | A | Yes | `internal/brief/build.go:118-120`; `internal/web/shell.go:100-118` |
| 11 | Recordings may not be stored; recording was deferred | A | Yes | ADR-0022 D3; INIT-020 line 36 |
| 12 | Speaking self-check feeding the scheduler is a recorded choice | A | Yes | INIT-020 SPEC-002 line 31 |
| 13 | Research grades for each scored row | A | Yes | RSCH-003 lines 18, 22-25, 29-31, 33, 37, 39-42, 68, 82-83 (line 181 is an ungraded checklist item and is labelled so); RSCH-001 lines 16, 19, 65-66; RSCH-004 lines 15, 38-39 |
| 14 | `practiceResults` is advice to the author only | A | Yes | `.cursor/skills/hebrew-lesson-author/SKILL.md:142`; ADR-0025 D8 |
| 15 | Two voices exist | A | No | `internal/audio/voices.go:11-23` |
| 16 | Total study time is not shown to the learner | A | No | `internal/mcpstore/digest.go`, `internal/jobs/weekly.go`; `internal/web` minutes are a today estimate only (`views/today.go:114-136`) |
| 17 | Several Hebrew letter pairs are homophones in modern speech (ט/ת, כּ/ק, ח/כ, ס/שׂ, א/ע, ב/ו); true listening targets are /χ/ vs /h/ vs /k/, ר, צ vs ס, stress and prefix boundaries | C | No | Analyst domain knowledge. SPEC-005 makes a native-speaker check the first step |
| 18 | How often "known" is reached on the first day in practice | n/a | No | Unknown. Application data not read |

### Falsification

Each test tried to overturn a finding.

1. **"The inflation of 'known' is theoretical."** The worked example in the AI author skill asks מַיִם in two listen_pick steps in the lesson and one in its checkpoint (`.cursor/skills/hebrew-lesson-author/SKILL.md` lines 291, 297, 308). Each schedules the listen card (`lesson.go:610-692`). Three right answers that day give three successes, recall probability near 1.0 and three newest right, so the listen skill is "known" (`state.go:148-151`, `:183-190`). For `knownLexemes` it takes less: one right lesson recall writes Good on the read card. **Survives, narrowed.** The mechanism is real. Its frequency in real data is unknown, and the inflation is temporary: once recall probability falls below 0.85, the state drops. It still affects the brief on the days it holds. ADR-0030 line 24 shows the project has measured a known-rule gap in production before, so the same kind of one-query count would settle the frequency.
2. **"Input volume is already enough."** Lessons do lead with a story, and drills are capped at 40%. **Survives, narrowed** to volume beyond lessons: a 12–50 word story per lesson is far below the reading volume the extensive-reading studies use, and nothing else is offered.
3. **"A sound drill would be useless, because the confusable letters sound the same."** **Survives, narrowed.** That is true of the letter pairs in guidance C1 as written. So this report recommends rewriting C1 and training only true contrasts, after a native-speaker check.
4. **"Hearing yourself is not feedback."** **Survives, narrowed.** Record-and-compare is self-monitoring, not corrective feedback. It is the private first step. Real feedback comes from the tutor or from a speech-recognition option if the spike supports one. So option 2 alone lifts the pronunciation row from 1 to about 2. Closing it depends on the tutor channel or on SPEC-010, which is also the trigger for Hybrid.
5. **"Counting self-reports toward 'known' is a decision already made."** INIT-020 SPEC-002 line 31 records that the speaking self-check feeds the scheduler. No record decides whether it counts toward "known". ADR-0030 line 22 shows the project asks exactly this kind of question in an ADR, but it was asked there for typed spelling, not for self-reports. **Reclassified** from a defect to an operator decision in SPEC-001.

### Independent evaluation

A fresh judge agent that did not write this report scored it against the skill's human-written oracle, using the cited files as grounding. Deterministic property checks ran as well. Details are in [`eval/ASMT-003-hebrew-web-app-eval-report.md`](eval/ASMT-003-hebrew-web-app-eval-report.md).

**Round 1: MITIGATE.** The judge rated it 3 of 4 (score 0.67). It found 13 of 14 load-bearing claims supported (faithfulness 0.93), 10 of 12 criteria passed (0.83), and all 16 property checks passed. It flagged:

- three research-grade labels that did not match the cited reports (vowel-mark fade, the quarter share, audio pace);
- the method section placed before the BLUF;
- an unclear statement of which code paths use which "known" set;
- a few line ranges;
- no stated basis for the 0.85 confidence.

Every flag was re-checked against the files, and all held. The fixes are in this version. No flag touched the recommendation.

**Round 2: PASS.** A second fresh judge rated it 4 of 4 (score 1.0). It checked 30 load-bearing claims, including all 17 research-grade labels, and all were supported (faithfulness 1.0). All 12 criteria passed (1.0), and all 16 property checks passed. Its remaining notes were wording and traceability; all were applied:

- the BLUF's "Got it" phrasing;
- which right recalls write Good;
- "matches" softened to "largely matches";
- one line range;
- the ADR-number plan for INIT-032;
- the ADR-0030 ID clash, now disclosed under Objectivity.

---

## Spec Skeleton

Each spec is one domain, sized 2–6 hours. They are ordered by phase.

**Phase 1: numbers you can trust**

- **SPEC-001 · docs.** An ADR, "Known means remembered on a later day." It decides three things:
  - a skill is known only after at least one right answer on a later study day;
  - whether self-reported speaking checks still count toward "known" (they keep scheduling either way);
  - that `knownLexemes` and the suggested course start use the same rule as the headline, tier and coverage check.

  It also records the expected drop in "Words you know" and how the page explains it.
- **SPEC-002 · data.** Queries that supply a later-day success flag per card and leave self-reported rows out of the known count (`internal/db/learner.go`). The `knownLexemeUnion` query (`internal/db/placement.go:614-632`) uses the strict rule, which moves `knownLexemes` and `suggestedStart` with it. Golden tests are written on fixed review histories.
- **SPEC-003 · service.** `StateOf` uses the later-day flag (`internal/learner/state.go`). The headline, the tier (`internal/brief/build.go:534`) and the coverage check (`internal/mcpstore/learner.go:51-71`) follow without their own change; tests pin that. Bump `rule_version` and rebuild history as ADR-0030 did. Add a "learned today, check tomorrow" state to the word list.

**Phase 2: hear it**

- **SPEC-004 · frontend.** A 0.75× toggle on every clip: lesson, review prompt and feedback, word page and dialogue. It is remembered per learner and reuses `placement.js`. This meets guidance A4.
- **SPEC-005 · docs and content.** A sound-contrast inventory for English speakers, checked by a native speaker or the tutor. It separates true contrasts from homophone letters, and guidance C1 is rewritten to match.
- **SPEC-006 · service.** A minimal-pair item bank built from real lexicon words, with clips in both voices via `generate_audio`.
- **SPEC-007 · service.** A "hear the difference" drill kind in the review mix. It has two choices and instant feedback, and clears after three right answers like letter drills. Listening misses trigger it, and new learners get it seeded.
- **SPEC-008 · frontend.** The drill screen: play, pick, then feedback with both clips replayable.

**Phase 3: say it**

- **SPEC-009 · frontend.** Record-and-compare on speaking, shadow and dialogue steps. It uses the browser's recorder, is never uploaded, and is discarded on leave. Includes the microphone-permission design INIT-020 asked for.
- **SPEC-010 · spike, one day.** Hebrew speech recognition on 20–30 lesson phrases, testing one browser option and one server option. Includes a privacy review. It decides whether an automatic pronunciation check is worth a spec, which is the Hybrid trigger.

**Phase 4: read and listen more**

- **SPEC-011 · docs.** An ADR for a graded read-and-listen library. Texts sit at 98% strict known-word coverage (Low basis, RSCH-003 line 29). Each has tap glosses and self-paced audio, logs exposures, and has no grade.
- **SPEC-012 · data.** A readings table, learner-scoped and revisioned like units, with exposures.
- **SPEC-013 · api (MCP).** `get_reader_brief` and `put_reading`. These reuse the validator's coverage, mapping, pointing and audio checks at the reader threshold.
- **SPEC-014 · agentic.** A `hebrew-reader-author` skill, sibling to `hebrew-lesson-author`.
- **SPEC-015 · frontend.** A "Read and listen" card on Today, after the main task. The reader page has tap glosses, replay and slow audio.

**Phase 5: close the loop**

- **SPEC-016 · service.** The brief reads later-day results by practice kind. When a kind trails by 10 points or more with enough answers, its share drops. This changes the mix only, never the scheduler (ADR-0025 D8).
- **SPEC-017 · frontend.** Progress shows total study time against a "hundreds of hours" expectation (RSCH-003 line 83).

**Later, Low evidence or operator calls**

- A sampled, non-blocking native-speaker or tutor spot-check of published units over MCP. It fits ADR-0018 D5.
- A within-learner comparison: randomize which new words get which practice kind, then compare later-day results.
- An optional daily reminder at a fixed time (RSCH-003 line 82, Low).
- A public sample lesson (RSCH-007 line 71; an operator and security call).

**Estimate.** 17 specs at 2–6 hours each is roughly 35–100 hours, plus the one-day spike.

---

## Risks

- **A lower headline may discourage the learner.** Mitigation: a visible "learned today, check tomorrow" state, a short note when the rule changes, and history rebuilt under a new rule version.
- **Synthetic voices may blur the very contrasts being trained.** Mitigation: a native-speaker check of the drill clips before release (RSCH-001 line 66).
- **Microphone permission and privacy.** Mitigation: browser-only recording, no upload, and a privacy review before the spike touches real learners.
- **Voice credits.** Minimal pairs and readers add clips. Mitigation: confirm the balance and cap readings per week.
- **More screens can dilute the one next action on Today.** Mitigation: reading sits after the main task, never in front of it.
- **AI-written readings may be unnatural Hebrew.** Mitigation: the same validator gate as lessons, plus the later spot-check.

---

## Next Step

Hand this decision to `create-initiative` with `source: ASMT-003`, using the spec skeleton above. The suggested first initiative is Phase 1 alone, because every later phase reads the "known" numbers it fixes. SPEC-010's privacy review and any public sample lesson should carry `security_review: required`.
