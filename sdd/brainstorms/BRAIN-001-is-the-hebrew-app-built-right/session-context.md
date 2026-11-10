## Session Context

**Topic:** Is the Hebrew learning app built right, and what could we do better?
**Session Type:** Quick Cross-Domain EPIC — **Depth:** Quick
**Session ID:** BRAIN-001 · **Date:** 2026-11-09 · **Decision-maker:** Brian Nelson (owner, and learner 1)
**Repository:** `c:\Users\BrianNelson\Projects\Hebrew` (Go module `github.com/NetworkBuild3r/hebrew-lessons`). Live at https://hebrew.ibhacked.us. Authoring MCP at https://hebrew-mcp.ibhacked.us/mcp.

**Goal:** Decide whether the app's overall design (lessons, review, progress, placement, audio, and the way lessons are produced) is sound, and name the few changes that would do the most good. Scope is the whole app.

### Context

**Trigger.** On 2026-11-08 Brian opened lesson 12 and found buttons for words he was never taught ("this lesson is shit. I didn't even have some of the option i just failed on"). An assessment (ASMT-004) found 23 learner units (12 to 34) cloned from one broken structure: 0 new words, a lesson that repeats another, 128 one-word clozes, an authoring note ("Correct form first; incorrect form second.") in the learner's English, and wrong answer buttons drawn from untaught words and from the wrong side of tutor-mistake contrasts. Two initiatives fixed it and shipped today (INIT-036 "known means remembered on a later day", INIT-037 "a lesson asks only about words you were taught"). Brian now asks whether the app is built right at all.

**How the app is built.**
- One Go binary (`cmd/hebrew`) serves the learner web app (server-rendered `templ` HTML plus small vanilla JS) and an MCP authoring server on a second port. SQLite with one writer connection; 22 migrations. FSRS spaced-repetition scheduler. ElevenLabs text-to-speech clips stored on disk. A Tanakh corpus (Isaiah verse by verse). About 67,000 lines of Go and 92,000 lines of tests in 37 packages.
- Deploy: GitHub Actions on self-hosted runners builds `ghcr.io/networkbuild3r/hebrew:sha-<commit>` after a three-shard test run of about 20 to 30 minutes. Production is pinned in a separate GitOps repo (`home_k3`) and synced by Argo CD to a home k3s cluster. The image auto-updater is currently out of sync, so today's deploy needed a manual pin commit. Today's push also failed CI twice (gofmt on 42 files, then a test import cycle) because the Windows dev machine has no Go on PATH; tests run through Docker. Generated `templ` code has been hand-edited when the generator was not available.
- **Content model.** A lexicon of about 1,000 pointed Hebrew entries. A unit is a JSON document with lessons of up to 10 activity types (story, dialogue, shadow, say, listen_pick, recall, produce, cloze, pattern, order). There are 34 units: 1 shared, 33 scoped to learner 1.
- **How lessons are produced.** An AI agent writes each new unit for one learner over MCP. Code builds a "lesson brief" (which new words, which struggle words, which known words, pace, tier). The agent follows a 364-line skill file and a 352-line authoring guide, calls `validate_unit` (61 rule ids), generates audio, and calls `put_unit` with `publish: true`. A learner-scoped unit that passes the validator with its brief goes straight to the learner's Today with no human review. A coach note is left for the next session. A reviser skill rewrites untouched units (now capped at two per session).
- **Learner model.** Per word and per skill (listen, read, recall = English to Hebrew, spell): unseen, seen, learning, known, solid, struggling, regressed. As of today "known" needs checked right answers on two different study days (rule version 4). There is a placement test, daily snapshots, weekly reports, coach notes, and a "tutor data loop" for an outside tutor to record taught words and corrections.
- **How the app is developed.** Almost entirely by AI coding agents through a spec-driven process: 34 initiatives, 291 specs, 31 decision records (ADRs), 258 commits since the first commit on 2026-10-07. Early initiatives (1 to 15) built a Node app, an admin plan builder, several redesigns, then a Go port and "v3" (placement, learner model, AI-built lessons). Initiatives 27 to 37 are mostly corrections to review and to the AI lesson pipeline (review as a test, practice that teaches, lessons that build, one right answer, known means remembered, taught words only).
- A separate, untracked folder "Aharon's code" holds simple static HTML worksheets for a course called דרך עברית: one shared player, each lesson a small JS data file (vocab cards, word-bank fill-ins, grammar multiple choice, reading, interview questions), browser speech at 85% speed, progress in localStorage.

**Outcomes so far (learner 1, the only active learner).**
- 11 units finished in about a month. 64 minutes studied last week, 45 minutes in app this week so far.
- Words known in at least one skill: 36 (strict rule). Known or solid by skill: listening 10, reading 32, English to Hebrew 3, spelling 4. About 88 words are "learning".
- 161 review cards are due now. Retention last week was 80% against an 85% target; English-to-Hebrew recall retention is 59%.
- No tutor corrections recorded in the last 30 days.
- The brief for the next lesson today says "review first": one consolidation unit is already queued and 161 cards are due, so no new lesson was written.
- Under today's rules the 11 finished published units each have 11 to 29 blocking issues. They were written under older rules.
- A sound-contrast feature (hearing the difference between similar Hebrew sounds) is blocked until a native speaker or tutor approves the pairs.

**Constraints.** One developer-owner who is also the learner. Built and changed mostly by AI agents. Runs on a home cluster. No hard budget limit stated, but ElevenLabs audio and agent time both cost money. Brian wants lessons he can trust without checking each one.

**Success criteria.** A short list of changes, ranked, that would make the app teach Hebrew better and stop bad lessons reaching Today, with an honest view of what to keep, change, or stop.

### User Answers (Phase 2 Inquiry) — ground truth from Brian

| Specialist | Question | Brian's answer |
| --- | --- | --- |
| Technical Architect + Product Strategist | Who should the app serve over the next year? | **Only me.** A personal tool; anything built for other learners can go. |
| Product Strategist + Technical Architect | What is your main Hebrew course? | (own words) **"I have a tutor. I want a tool to help me relearn our lessons but actually retain what I learned through science backed research."** |
| Product Strategist + Data Analyst | Which result in 6 to 12 months proves the app worked? | **Hold a simple spoken conversation:** understand Hebrew at normal speed and answer. |
| QA Advocate + Data Analyst | What can a real Hebrew speaker do for you? | **Nobody.** Plan as if no person ever checks the Hebrew or tests him. |
| Data Analyst | 161 due, about 9 min/day: which plan? | **"I can give 20 to 30 minutes a day if the sessions are worth it. Plan for that."** |
| Devil's Advocate | What stops you studying more? | **Trust.** When a lesson looks wrong he skips it or stops, and lately that happens often. |
| Devil's Advocate | Study vs build time split? | (own words) "AI builds. But app sucks. yeah i can tell the ai fix this, fix that. but most the time the lessons are shit. Asking me questions like pick one, i____, eat, sleep, walk. Only one is right but technically all are right. I do a review, spell words. go to progress and says you don't know any words. It's like I know how to spell words, let alone read them. so yeah it's frustrating to not be able to know where I really am. The app doesn't evaluate my progression and every lesson is the same shit. I wanted more, things like create a sentence, typing. More reading interesting stories. Easy to understand what is going on." |
| Devil's Advocate | Alternative to AI-written lessons? | (own words) **"AI is to be my supplemental Tutor. knowing where I am, where I struggle and helping me tune my lessons."** |
| Technical Architect | How much would you throw away? | (own words) "Whatever is crap and not helping me learn. So none to all of it." |
| QA Advocate | Bad lesson tolerance? | "Other" (no detail given). Read together with the Trust answer: a bad lesson costs him trust and study time. |
| QA Advocate | A deploy breaks a study morning? | (own words) "I keep going. it sucks but whatever." Deploy risk is tolerated. |

**What these answers mean together (parent's reading, for the panel to test, not to accept blindly):**
- The app's job is a **retention tool for what the tutor teaches**, plus an AI that acts as a supplementary tutor: it knows where he is, where he struggles, and tunes practice. It is not meant to be a whole course that invents a syllabus.
- The goal skill is **conversation** (listening plus speaking). He also wants production (create a sentence, typing) and **interesting stories to read**.
- **No human will check content.** Every guarantee of correctness has to come from code, trusted sources, or the tutor's own material.
- **Trust is the binding constraint**, not time. Fix trust and he will give 20 to 30 minutes a day.
- Concrete complaints: multiple-choice blanks where several answers are grammatical and only one is marked right; "every lesson is the same"; **Progress says he knows almost no words when he can spell them**, so he cannot tell where he really stands; the app does not evaluate his progression.
- Nothing is sacred: keep only what helps him learn.

### Main Questions
1. Is "an AI writes each learner's units and a rule validator gates them" the right way to produce lessons, or should the course be built differently?
2. Is the app measuring and driving real learning (words he can recall and use), or optimizing proxies?
3. Where is the system fragile (content quality, the learner model, testing, deploy, the way it is developed), and what would make it robust?
4. What should we stop building or simplify?
