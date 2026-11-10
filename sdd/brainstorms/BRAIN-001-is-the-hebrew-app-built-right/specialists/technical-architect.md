# Technical Architect — Brainstorm Analysis

**Session:** Is the Hebrew app built right? (BRAIN-001)
**Date:** 2026-11-09
**Domain:** Engineering
**Primary Lens:** Fit and maintainability for one learner who has a tutor. Where does the truth about "what to learn" live, where does the trust boundary sit when no person checks the Hebrew, and does each part's complexity pay for itself?

---

## Phase 3: Independent Analysis

### Chain of Thought

#### 1. Primary lens

I judge the system by its shape, not its code quality. The code is careful: pure functions, named constants, append-only logs, deterministic seeds, and tests that outnumber the source. The question I am asking is whether the parts are arranged to do the job Brian now says he wants: a retention tool for what his tutor teaches, plus an AI that acts as a supplementary tutor. For a single learner, "scalability" means one thing here: whether the design converges as content and rules grow, or whether every new lesson and rule creates new work.

#### 2. Initial assessment

The learning core (evidence logging, the FSRS scheduler, the derived learner model, the lexicon, the audio cache) is sound and worth keeping. The content side is the wrong shape for this learner. It is a general course that invents its own syllabus from word-frequency bands. Its content is free-form AI JSON documents, gated by a validator that can only list known bad patterns, and auto-published ahead of time in batches. That combination cannot converge, and it is the structural reason initiatives 27 to 37 are mostly corrections.

#### 3. Main considerations

**C1. The app was built for a learner without a tutor, and the tutor channel is idle.**

- The design doc that set the current shape says the tool "works on its own for someone without a tutor, and alongside a tutor for someone who has one. ... Without a tutor, the default path stands in" (`docs/design/learner-without-a-tutor.md`, lines 5 and 17).
- The lesson brief chooses new words as "unseen words from the lowest band that still has any", ordered by a seeded hash (`internal/brief/build.go:913-961`, `internal/brief/doc.go:8-10`). The brief package never reads `taught_words`. A search of `internal/brief` for "taught" finds only pattern bookkeeping.
- The tutor channel exists: `taught_words` (migration 020), `Recorder.Taught`, `record_taught_words`, and dated `record_mistakes` (`internal/evidence/doc.go:34-40`). Today's live digest for learner 1 shows `taught: []` for the last 14 days, `corrections: []` for the last 30, and `closedLoop.total: 0` since 2026-10-11.
- So the "default path" is what actually runs. The app teaches its own frequency-ordered syllabus, and the tutor's lessons never reach it. That is the opposite of Brian's stated goal: "I want a tool to help me relearn our lessons."
- The static worksheets in `Aharon's code/` show what tutor-course material looks like. The one lesson there (`worksheets/shai-date-indirect-speech/worksheet.js`) is labelled "Level A1+ / A2". It teaches indirect speech, with vocabulary cards that carry example sentences, fill-ins with an explicit answer and word bank, and grammar multiple choice with an explicit answer index and a "why". *Assumption, not verified:* this is Brian's tutor's course (the session context names the course, but not whose it is).

**C2. Generate-then-validate over an open document is a blacklist, and blacklists do not converge.**

- A unit is a free-form JSON document with lessons of up to 10 activity types. An AI writes it, and 61 rule ids try to catch what is wrong with it (`internal/units/rules.go:46-108`).
- Every rule names a bad pattern that has already been seen. The space of bad documents is unbounded, so the learner keeps finding the next one. ASMT-004 says it directly: "The AI author does what its rules reward." It satisfied "reuse struggle words" with bare shadow and cloze pairs, and it satisfied `brief.fix_contrast` by pasting the tutor's whole sentence.
- The initiative sources show the same loop. INIT-027 is sourced to Brian on 2026-11-01 ("review is the same shit over and over"). INIT-028 comes from a live review sitting on 11-02. INIT-030 comes from 11-05 ("Our lesson plans are the same over and over"). INIT-032 comes from lesson 12 on 11-06, and INIT-037 from lesson 12 again on 11-08. Today, in Phase 2, Brian says "every lesson is the same shit." The same complaint has come back three times across a week of fixes.
- The learner is the test oracle. Nothing checks how a lesson reads to a learner before it reaches Today. A learner-scoped unit that passes the validator publishes with no human step (author skill, line 18).

**C3. One contract is written in five prose copies, and the copies drift.**

- The rule contract lives in `internal/units/rules.go` (the catalog), the doc comment in `internal/units/doc.go:24-56`, the rule table in `internal/mcp/resources/authoring.md` (352 lines), the rule table in `.cursor/skills/hebrew-lesson-author/SKILL.md` (now about 420 lines), and the MCP tool descriptions in `internal/mcp/tools.go`.
- ASMT-004 found `unit.same_shape` inverted against its spec. The test pinned the inversion, and the guide told the author the opposite of what the code did.
- I found a live example today. The `get_context` description (`internal/mcp/tools.go:628`) still says `knownLexemes` is "read cards whose latest rating is Good or Easy". The code calls `LearnerKnownLexemes` (`internal/mcp/tools.go:316`), which is the strict later-day set under ADR-0031 D3. The author skill says "the tool descriptions are the contract", so the agent is told something false.
- The digest description says history points carry "ruleVersion (3 under current rules)", but the current rule version is 4.
- Since 2026-10-20, `authoring.md` and the author skill each changed 15 times, and `rules.go` changed 11 times (git log). Each rule change has to land in four or five places by hand.

**C4. Tightening a rule creates legacy content by design.**

- Most newer rules carry the same clause: "On a new save or publish this is a block ... Checking the current published revision reports it as a warning" (`internal/units/rules.go`, for example `unit.cloze_context`, `unit.lesson_repeat`, `unit.meta_text`, and `unit.target_untaught`).
- That grandfathering keeps old units playing, so every new rule opens a cleanup backlog. The 11 finished units now carry 11 to 29 blocking issues each (session context). Units 12 to 34 had to be unpublished (INIT-037 SPEC-001).
- Content was also generated far ahead. The outline has 34 units, 22 of them untouched, revised from one learner snapshot (ASMT-004, claims 13 to 15). For an adaptive system, pre-generating weeks of lessons is the wrong timing, because each one is stale by the time it is opened.
- Today's page now runs the full write-mode validator over every published unit on each render, to skip bad ones (`internal/web/shell.go:95-136`). That is a reasonable patch. It also shows the layering bending: the web handler now imports the MCP store and the validator to defend itself against content.

**C5. The solid core is already decoupled from units, and that makes migration cheap.**

- `attempts` and `exposures` are append-only, and their `unit_id` is nullable with `ON DELETE SET NULL` (`migrations/006_evidence.sql:8-38`). Cards are keyed by learner, lexeme, and skill, and `review_log` hangs off cards, not units (`migrations/006_evidence.sql:50-63`, `migrations/001_init.sql:107-117`).
- Learner state is "computed from evidence, never stored" by pure functions (`internal/learner/doc.go:6-9`). Rule changes rebuild derived history from `review_log` under a stamped rule version, and the log itself is never edited (ADR-0031 D5).
- `internal/evidence` is "the one place a checked answer is written" (`internal/evidence/doc.go:1`).
- This is event sourcing done properly. Brian's progress lives in data that does not depend on the unit model, so the content model can change without migrating his history.
- One caveat: `lesson_runs` has `unit_id ... ON DELETE CASCADE` (`migrations/001_init.sql:120-128`), and so does `lesson_items`. Deleting a unit row would delete his lesson completion history. Units must be archived, never deleted.

**C6. There are two engines for checked questions, and the bugs cluster in one of them.**

- Review uses `internal/activity`, with its own distractor picker, gloss-overlap checks, HMAC item tokens, and typed grading (`internal/activity/doc.go`). Lessons use a second path: `internal/web/lesson.go` (1,967 lines) with `wideChoices`, `lessonChoices`, and `choicePool`, plus `internal/choices`.
- `internal/web/lesson.go` is the most-changed file since 2026-10-20 (20 commits), and the player template is next (18). The untaught-button fault and the cloze ambiguity both lived on the lesson side (ASMT-004 Fault 1, INIT-032).
- Runtime distractors drawn by part of speech into a sentence frame are ambiguous by construction. "אֲנִי __" with eat, sleep, and walk is grammatical for every button. INIT-032 patched this with an English cue. Aharon's worksheet avoids it structurally: a person writes the right answer and the wrong answers, with a reason.

**C7. The definitions behind the numbers keep changing, and the signals conflict.**

- The meaning of "known" changed under ADR-0027 (2026-11-06), ADR-0030 (rule version 3, 11-07), and ADR-0031 (rule version 4, 11-08).
- The digest history jumps from 20 to 35 on 11-06, the day typed-spelling credit and a spelling back-fill shipped (git log). It then shows 38 under rule 3 on 11-08 and 36 under rule 4 today, on the same chart. *Inference, not verified:* part of each jump is a change of definition, not of learning. The chart mixes rule versions.
- Today's `get_context` model for listening has an `estimatedKnown` of about 503 from placement, 10 words known or solid, 96% retention, and a standing verdict of `not_askable`, "No audio yet". For spelling, it says "97% right over 33 answers" with a verdict of `strong`, and 4 words known.
- Each of those numbers has its own definition, and every one was added by a separate fix. Brian's complaint, "go to progress and says you don't know any words", is what a learner sees when five measures disagree on one screen. The Data Analyst should own the fix. Structurally, the problem is that the numbers have no single owner and no frozen definition.

**C8. The process and the dev loop amplify the churn, but they are not the root cause.**

- The dev machine has no Go on PATH (verified: `Get-Command go` finds nothing). At least 8 of the last 60 commits only fix what CI rejected: gofmt (5), a race-shard test, an import cycle, and doc drift that blocked the image. Test shards have a 45-minute timeout (`.github/workflows/ci.yml:62`). CI checks gofmt and templ drift (`ci.yml:532-574`), but only after a push.
- Generated templ code has been hand-edited (session context). The image auto-updater is out of sync, so deploys need a manual pin.
- The output for one learner in one month is 34 initiatives, 291 specs, 31 ADRs, and 258 commits. ADRs amend ADRs: ADR-0032 amends ADR-0026 D3 and restores an INIT-030 spec line, and ADR-0031 amends ADR-0027 D1 and supersedes an INIT-020 spec line.
- A slow loop pushes fixes into big batches, and a big batch is where an inverted rule gets pinned by its own test. The loop slows the fixes and lowers their quality at the edges. The content shape (C1 to C4) is what keeps generating new things to fix.

#### 4. Opportunities

**O1. Make the tutor's lesson the source of truth for "what to learn".** The plumbing exists: `taught_words`, `Recorder.Taught` (which creates cards due at the next study-day cutoff), dated `learner_mistakes`, and the MCP tools. What is missing is a place to store the tutor's sentences and grammar point, and an ingest flow quick enough to use after every class. Aharon's worksheet data format (vocab with an example sentence, a fill-in with an answer and a bank, multiple choice with an answer index and a "why") is very close to the item schema this needs.

**O2. Replace units with an item bank and a session that code assembles on the day.** `internal/session` already builds a deterministic, research-shaped review queue (struggling and known shares, a target success rate, interleaving, and requeue gaps; `internal/session/doc.go`). `internal/activity` already grades typed answers with near-miss handling and logs confusions. Adding sentence items as a new kind in that same engine gives one player instead of two. It also makes clones impossible, because code sequences the session and no AI writes a five-lesson skeleton to copy.

**O3. Shrink validation to a small, complete per-item check.** A sentence item is small enough to check completely. Every token must be an exact lexicon entry, every token except the target must be a word the learner was taught (the ADR-0032 D1 definition), there must be one target, an English translation with no authoring vocabulary, length bounds, and no duplicate. A dozen rules can be complete for an item in a way that 61 can never be complete for a free-form unit.

**O4. Keep one machine-readable contract.** `units.Rules` is already served as `hebrew://rules`. Skills and guides should point at it and stop reprinting the table. A test should fail when a skill or guide names a rule id that is not in the catalog. Tool descriptions should be checked against the behavior they describe.

**O5. Collect the deletion dividend.** `internal/units` (about 9,000 lines with tests), `internal/brief` (about 4,200), and `internal/web/lesson.go` (about 2,000) come to roughly 15,000 lines of clearly unit-pipeline code. That is before counting the unit parts of `internal/mcp`, `internal/mcpstore`, `internal/db` (`put_unit.go`, `learner_publish.go`, `lesson.go`), and the player templates. Retiring the pipeline removes the part of the code that changes most often.

**O6. A local check in one command.** A Docker-based `check` that runs `gofmt -l`, `templ generate` with a diff check, `go vet`, and the affected tests, wired as a pre-push hook, would catch most of the 8 CI-only fix commits in seconds instead of 20 to 30 minutes.

#### 5. Risks and concerns

- **R1. High: the tutor channel stays empty, so the new shape starves.** `record_taught_words` has been available since INIT-026 (2026-11-01) and has never been used in the window the digest covers. If ingest is not close to effortless, the AI will fill the bank with invented sentences, and the trust problem comes back at a smaller grain.
- **R2. High: a rebuild by AI agents repeats the treadmill.** This codebase has already been through a Node app, a Go port, v2, and v3. A new shape built with the same initiative-per-complaint process could grow its own 61 rules. Mitigations: build it beside the current code (strangler pattern), keep the new surface small, and delete the old path only after Brian has used the new one for a few weeks.
- **R3. Medium: AI-written sentences contain Hebrew errors nobody catches.** Per-item checks prove that the words are taught lexicon entries. They do not prove that the combination is grammatical (agreement, construct state, prepositions). *Not verified:* how many lexicon entries, verb forms, and noun forms carry `verified = 1`. Agents upsert them, and the skill forbids agents from setting the flag. Mitigations: prefer tutor sentences, tag every item's source, cap the share of AI items in a session, and give one-tap retirement of a single item.
- **R4. Medium: migration damages history.** Deleting unit rows cascades into `lesson_runs` and `lesson_items`. Mitigation: archive units, never delete them, take a backup before every migration (the repo already has a backup-refusing back-fill pattern), and keep migrations additive.
- **R5. Medium: another change of definition moves Brian's numbers again.** Every rule-version bump changes the headline he is trying to trust. Mitigation: freeze the "known" definition for a fixed period, and rebuild the whole history chart under one rule version before showing it.
- **R6. Low: deploy fragility.** Brian tolerates a broken morning ("I keep going"). It still slows every fix. Repair the auto-updater or make the pin part of the release step, but do not let it outrank the content work.

#### 6. Recommended approach

**Target shape: tutor lesson in, practice out.**

1. **Sources (trusted inputs).** Add tutor lessons as a record: date, title, and grammar note. Add a sentence bank: pointed Hebrew, English, token-to-lexeme links, a target, an accept list, a source (`tutor`, `worksheet`, `ai`, or `legacy_unit`), and a retired time. Words keep going through `Recorder.Taught`, and corrections through dated mistakes. The lexicon and the exact-text audio cache stay as they are.
2. **Learning core (keep unchanged).** Keep `internal/evidence`, FSRS cards and `review_log`, `internal/learner`, `internal/studyday`, and rule-versioned rebuilds.
3. **Session assembler (code, run on the day).** Extend `internal/session` to build one 20 to 30 minute daily session. It has three parts:
   - due cards, capped and mixed as it does now;
   - sentence items for words that are learning or struggling, drawn only from sentences whose other words he was taught;
   - new items from the latest tutor lesson, plus one short reading passage.

   Prefer formats that code grades without ambiguity: typed English-to-Hebrew sentences with an accept list, sentence order, dictation, listen and pick the meaning, and cloze with a typed answer instead of buttons. Brian asked for typing and building sentences, and these formats are also the easiest to trust.
4. **AI as supplementary tutor (narrow tools, each item checked).** The AI should get four narrow tools:
   - Ingest a tutor lesson from Brian's notes or worksheet: extract the words, sentences, and corrections.
   - Propose sentences for one word, using only taught words.
   - Write one short story whose coverage is computed.
   - Set the next session's focus or theme as a parameter, and leave a coach note.

   The AI never writes a multi-lesson document, never orders practice, and never publishes a batch ahead of time. A bad item costs one tap to retire, not a whole unit.
5. **One contract.** Make the rule catalog the only copy. Skills shrink to "call the tool, fix by rule id, read `hebrew://rules`". Add a test that ties the rule ids in the docs to the catalog.

**Keep:** the evidence recorder, FSRS, the learner model and its rule-versioned rebuild, the lexicon with verb and noun forms, the audio cache, the review session and `internal/activity`, coach notes, `taught_words` and mistakes, the MCP server and auth, and a simplified Progress page.

**Change:** make the session the thing Today offers instead of "the next unit". Move to one player for checked questions. Give the AI narrow, item-level tools. Have one owner and one frozen definition for each learner number.

**Stop:** AI-written per-learner units, the reviser, auto-publishing units, choosing new words by frequency band, consolidation units, the shared-versus-learner unit duality and pattern path-order checks, worksheet-shape templates, and hand-editing generated templ code. Freeze placement, Isaiah, the roots hub, and weekly reports: keep them running, but do no new work on them unless Brian asks. Brian said anything built for other learners can go, but ripping out working multi-user code has no learning payoff. Stop extending it, and remove it only where it is in the way.

**Migration path (each step ships alone and can be undone):**

- **Step 0, now.** Today offers review only, which it effectively does already ("review first"). Take a database backup. Confirm the deploy pin.
- **Step 1.** Add an additive migration for the tutor lesson and sentence tables. Add ingest and add-sentence MCP tools with per-item checks. Run a dry-run harvest of sentences from finished units 1 to 11 and from the worksheet data files; only items that pass the per-item check enter, tagged `legacy_unit` or `worksheet`.
- **Step 2.** Add sentence activities to `internal/activity` and serve them from the session mix. Today's main button becomes "Today's session". Keep units 1 to 11 readable as past lessons, off Today.
- **Step 3.** After two to three weeks of real use, retire the unit authoring path in code, the brief, the reviser skill, and the lesson player. Leave the `units`, `unit_revisions`, and `lesson_runs` rows in place for history. History is safe because attempts set `unit_id` to null instead of cascading, `review_log` belongs to cards, and derived state rebuilds from the log.

**Dev loop.** Make a one-command Docker check the pre-push hook. Ban hand-edits to `*_templ.go`: always regenerate, and CI already rejects drift. Fix the image auto-updater or script the pin. For a one-learner app, keep ADRs for shape decisions like this one, and handle small fixes with a short change log instead of an initiative, specs, and an ADR each.

**Optimistic scenario.** Brian pastes the tutor's worksheet or notes after each class, and ingest takes a few minutes. Each day's session is review, then typing and building sentences from the tutor's own material, then one short story. Per-item checks hold because items are small. He stops meeting words he never had, because the bank only uses taught words. The code shrinks by a fifth or more, and agent changes become small and local.

**Pessimistic scenario.** Tutor input is sporadic, so the AI fills the bank and subtle grammar errors slip through with no person to catch them. The rebuild also drags on while Brian gets nothing new. Mitigations: Step 0 gives a usable session at once. Tutor items are ranked first. AI items are tagged, capped, and retired with one tap. Each step ships on its own, so stopping halfway still leaves a working app.

#### 7. Questions for other specialists

- **For the Product Strategist:** Does Brian's tutor produce material after each class (a worksheet, notes, or a word list) that he could hand to an AI in under five minutes? Is the דרך עברית worksheet in `Aharon's code/` from his tutor's course, and how far along is he? If tutor input stays irregular, what should the app practice between classes?
- **For the QA Advocate:** With no person ever checking the Hebrew, which answer formats can code grade with no false "wrong" (typed with an accept list, order, dictation), and which should be banned outright (multiple choice drawn from a same-part-of-speech pool inside a sentence frame)? What is the smallest per-item check set you would accept before an AI-written sentence reaches him?
- **For the Data Analyst:** Can one definition of "known" per skill be frozen for at least three months, with the whole history rebuilt under it? Which single number should Today and Progress lead with, so that "97% right in spelling" and "4 words known" stop contradicting each other? Is the 59% English-to-Hebrew retention a scheduling problem, a format problem, or a sign that recall cards are created too early?
- **For the Devil's Advocate:** Does narrowing the app to review plus tutor-sentence practice remove the "interesting stories" and comprehensible input Brian asked for? Would a plain spaced-repetition deck of his tutor's sentences, in an off-the-shelf tool, beat this app at the job he described?

### Summary

**Position:** The learning core is well built and should be kept, but the content side is the wrong shape. It is a self-directed course in which an AI writes free-form units ahead of time and a blacklist validator gates them. For a learner whose real course is his tutor, that shape cannot converge, and it is the structural cause of the correction treadmill.

**Recommendation:** Rebuild the content side as "tutor lesson in, practice out". Ingest each tutor lesson as words, sentences, and corrections, keep a sentence bank with a complete per-item check, and have code assemble each day's 20 to 30 minute session from FSRS due cards and taught-word sentences. Narrow the AI to ingesting, proposing sentences, writing a story, and coaching. Migrate beside the current code with additive tables, never delete unit rows, and retire the unit pipeline after a few weeks of real use.

**Top risk:** The tutor channel stays empty (it has been idle since it shipped), so the AI goes back to inventing content and the trust problem returns at sentence grain (High).

**Confidence:** Medium. The diagnosis rests on code and live data read this session (High). The recommendation depends on one unverified assumption: that tutor material can reach the app regularly and with little effort.

---

## Phase 4: Dialogue Contributions

### Questions Posed
- To the Product Strategist: can the tutor's material reach the app in under five minutes per class?
- To the QA Advocate: which formats can code grade with no false "wrong"?
- To the Data Analyst: can one definition of "known" be frozen and the history rebuilt under it?
- To the Devil's Advocate: does narrowing the app lose the stories Brian asked for?

### Questions Answered
- **From the Devil's Advocate:** Can review, the learner model and the MCP read tools stand alone without units, and is the listening trap real?
  **Response:** Yes, they stand alone. Evidence is append-only and sets `unit_id` to null, and learner state is computed from logs. The listening trap is real (`internal/session/mix.go:263-281`): 56 of 161 due cards are listening cards with no word clip. Fix it with word clips, or by never creating a listening card without one. Archive units and never delete them, because `lesson_runs` cascades.
- **From the QA Advocate:** Can the buttons be frozen per revision?
  **Response:** Yes, through a small additive change to `lesson_items`. Cheaper still: stop grading word-choice blanks whose wrong buttons share the target's part of speech.
- **From the Data Analyst:** Can reading and spelling be credited from production answers?
  **Response:** It's feasible. INIT-027's move-up logic already does this. Stop creating reading and spelling cards for new words.

### Tensions Involved In
- Build now or freeze first: build now, but accepted Stage 1 trust fixes plus an audit gate before the import.
- How much the AI writes: AI proposes sentences and a story. Accepted that only items with a trusted key are graded.
- The process itself: agreed it is his second-highest risk, and supports the guardrails.

### Synergies Identified
- Keep the core: shared with all.
- The syllabus comes from the tutor: shared with Product, QA and the Devil's Advocate.
- Retire AI units, the reviser and auto-publish: shared with QA and the Devil's Advocate.
- One source of truth for rules: shared with QA and the Devil's Advocate.

---

## Phase 5: Consensus Position

### Option Scores
| Dimension | A: Patch in place | B: Staged | C: Re-center now | D: Freeze and go simpler |
|-----------|---|---|---|---|
| Learning value | 2 | 4 | 3 | 3 |
| Trust and content correctness | 1 | 4 | 3 | 3 |
| Effort and time to value | 2 | 4 | 2 | 3 |
| Treadmill risk (higher = lower risk) | 1 | 4 | 2 | 4 |
| Preserves progress and history | 3 | 5 | 3 | 3 |
| Reversibility | 4 | 5 | 2 | 5 |
| **Total** | **13** | **26** | **15** | **21** |

### Score notes
- **A, patch in place.** This keeps the shape that causes the problem: free-form AI units checked by a list of known bad patterns. That list can never be complete, so trust and treadmill risk both score 1. Each fix looks cheap, but it has to be copied by hand into four or five places, and each new rule turns old units into a cleanup backlog. Total cost keeps growing, and the payoff never arrives. Each patch can be undone on its own, which is why reversibility scores 4. History is kept, but the contaminated wrong answers stay in it, and every further change to "known" moves his numbers again.
- **B, staged.** This is the only option that removes the cause and keeps the core. Stage 1 is mostly switches and deletions, so it pays off within days and can be undone. Stage 2 is gated and built beside the old path. History scores 5 because his answers and reviews live in logs that do not depend on units. Units can be archived instead of deleted, and Progress can be rebuilt once from the logs. Trust scores 4, not 5, because the pointed lexicon was written by agents, and how much of it is verified is unknown. A judge model lowers the chance of a bad AI sentence, but it does not prove the sentence is right.
- **C, re-center now.** The end state is the same as B. The difference is that it is built as one big push through the same process, with no gate, while today's untrustworthy app keeps running. Retiring the unit pipeline while the import is still being built leaves nothing to fall back on if the tutor material turns out to be thin. A rushed retirement is also exactly when someone deletes a unit row, and deleting a unit row deletes his lesson history with it.
- **D, freeze.** It costs nothing in code and can be fully undone. For 14 days it stops the treadmill, but it does not remove the cause. When building resumes, the same validator and the same five copies of the rules are still there. The app side keeps its false wrong answers, its switched-off listening, and a Progress page he can't trust. Practice done in Anki never reaches the app's learner model, so his history splits in two. It also relies on Brian loading tutor sentences himself, and he has not done that in a month.

### Final Position
**Preferred Option:** B, stabilize and then re-center on the tutor, in stages.

**Rationale:** B is the only option that takes out the structural cause of the treadmill (AI-written units behind a blacklist validator) while keeping the solid core: the answer log, FSRS, the learner model, the lexicon, the audio cache, and the MCP server. It also keeps Brian's history, because his evidence never depended on units. Stage 1 is small and reversible and fixes trust within days, with no new subsystem. The audit gate turns my top risk, an empty tutor channel, into a measured decision instead of an assumption.

**Conditions:**
1. **Archive, never delete.** No unit row is deleted at any stage, because lesson runs and lesson items are deleted along with their unit. Take a database backup before every migration, and make migrations additive only.
2. **Void before archive, and void without erasing.** Voiding is a marker with a reason, stored beside the answer. No attempt or review row is ever removed. The learner model skips voided rows when it rebuilds. The void must run before any unit is archived, because it needs the unit id. Then rebuild once under one frozen rule version, and draw the whole history chart under that single version.
3. **Stage 1 uses switches, not rewrites.** Stopping auto-publish, stopping graded word-choice blanks, and freezing side features are each a setting or a removed code path. Each must be undoable in one commit. "Freeze" means no new work, not ripping out working code such as multi-user support.
4. **Listening is fixed in two places.** Generate a word clip for every taught word, and add a guard so a listening card is never created without a clip.
5. **Stage 2 has one engine.** Sentence items become a new kind inside the existing review and session code, not a third player. The old lesson player and the unit pipeline are deleted only after Brian has used the new daily session for two to three weeks.
6. **One contract, with a budget.** The rule catalog is the only copy of the rules. A test fails when a skill, guide, or tool description names a rule id that is not in the catalog. The per-item check for a sentence stays small and complete, about a dozen rules, and a new rule retires an old one.
7. **Fix the dev loop first.** Before Stage 1 lands, add a one-command local check run through Docker as a pre-push hook: formatting, templ regeneration with a drift check, vet, and the affected tests. Hand edits to generated templ files are banned.

**Dissent:** Aligned with consensus, with one narrowing. I would not build the judge model in Stage 2 at first. Any sentence the AI proposes ships ungraded, as practice or as part of a story, and only tutor lines and exact lexicon forms are graded. A judge from another model family is added later only if the trusted bank proves too thin to fill a 20 to 30 minute session. That keeps Stage 2's surface, cost, and rule count smaller. It is the same "delete first, build least" guardrail the panel already adopted.
