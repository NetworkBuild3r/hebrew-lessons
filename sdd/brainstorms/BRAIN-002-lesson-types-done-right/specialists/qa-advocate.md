# QA Advocate — Brainstorm Analysis

**Session:** Lesson types done right
**Date:** 2026-11-10
**Domain:** Engineering
**Primary Lens:** Quality and trust. A lesson step must never mark a right answer wrong, never show a word Brian was not taught, and never show wrong Hebrew. With no human reviewer, code has to prove each of these before the step reaches Brian.

---

## Phase 3: Independent Analysis

### Chain of Thought

#### Primary Lens

I ask four questions about every step:

1. How will we test it?
2. What edge cases exist?
3. What could go wrong?
4. How do we know it is still right after the next change?

Brian's trust is the product. He skips or quits when a lesson looks wrong (session-context, "The learner"). So I treat each bad step as a release blocker, not a polish item.

There is no reviewer. The tutor will never open the app. Brian said "do as much as we can programmatically." So my rule is this: **a step may be graded only if code can prove its answer key. If code cannot prove it, the step is ungraded.** That is already ADR-0033 D3. I want to extend it from "AI-written sentences" to "any key we cannot prove."

#### Initial Assessment

What exists and helps:

- **Typed grading is plain and testable.** `GradeTyped` strips niqqud and cantillation, keeps letters, and counts edit distance on runes. Distance 0 is Good, 1 is Hard, more is Again (`internal/fsrs/typed.go:10-27`, `31-53`).
- **A second copy of the same rule lives in `activity.Normalize` and `Align`** (`internal/activity/align.go:10-34`, `59-104`). The comment says it is the same rule. Two copies can drift apart. Nothing I read proves they match.
- **An escape hatch exists.** `postAlsoRight` voids a missed attempt with reason `also_right` (`internal/web/also_right.go:21`, `53-60`). It does not mark the answer right (`also_right.go:15-20`). It refuses a correct attempt (`also_right.go:49-52`).
- **Today has a pre-serve gate.** `nextTodayLesson` skips a learner unit that has write-mode blocks (`internal/web/shell.go:100-107`, `142`, `153`). That is a natural hook for a "does this lesson pass its checks" gate.

What is missing:

- **The button does not flag the item.** The void is per attempt. Nothing marks the key, the word, or the line as suspect, so the same bad item can come back.
- **The only item-level control is heavy.** `postReport` hides the whole unit for the learner (`shell.go:397-401`). That is too coarse for "this one question is wrong."
- **Lesson `produce` is three Hebrew buttons, not typing** (ASMT-006 A6, `lesson.go` 1244, 1428–1430). The typed-spelling problem is not live in lessons yet. It is live in review's `type_it`. It will go live the moment lessons use `GradeTyped`.
- **The skill name for `produce` disagrees** between the validator and the player (ASMT-006 A7: `brief_rules.go` 123 says spell, `lesson.go` 658 says recall). A mislabeled skill means a wrong card gets scheduled. That is a silent wrong-answer bug.
- **Facts that raise the risk:**
  - 742 of 974 class tokens have no lexicon entry.
  - Only 158 of 350 tutor lines are pointed.
  - No accepted voice exists.

The central problem in one line: **code will soon build graded steps from material it has never verified.** The checks have to sit between that material and the answer key.

#### Main Considerations

**1. Separate "taught" from "keyed."**

These are two different permissions. Mixing them is how an untaught word or a wrong key slips through.

- **Taught:** the word may appear on screen. A word counts as taught if it is in a class document Brian attended, or was recorded with `record_taught_words`. It need not be in the lexicon.
- **Keyed:** the word may be an answer key, a distractor, or a blank. It needs a verified lexicon entry.

So the 742 unmatched tokens can still show up inside the tutor's own line, as Brian saw it. They cannot be graded. Some of the 742 are prefixed or inflected forms of known words (session-context). The first job is to measure how many resolve after stripping prefixes. That number is not known yet.

**2. Lexicon entries get a trust tier before they can be a key.**

Many entries will be AI-proposed. I suggest three tiers:

| Tier | Meaning | May be a graded key? |
| --- | --- | --- |
| T0 Tutor-attested | The word and its gloss both appear in a class document. | Yes |
| T1 Verified | AI-proposed, passed every code check below, and a second independent source agrees. | Yes |
| T2 Proposed | AI-proposed, passed code checks only, or one source. | No. Story tap-for-meaning and ungraded steps only. |

Code checks on any new entry (all deterministic, all can run on import):

- **Pointing is well formed.**
  - No two vowels on one letter.
  - A shin has a dot only on shin.
  - Final letters only at word end, and no final letter mid-word.
  - Dagesh only on letters that take it.
  - Cantillation marks are absent.
- **The skeleton matches the source.** The consonants of the pointed form must equal the unpointed token in the class document. The AI may add points. It may not change letters.
- **The gloss is plain.** One short English string, no Hebrew, no empty value. Its words must not overlap another taught word's gloss. The recall step already drops overlapping glosses (ASMT-006, `lesson.go` 1422–1435). A key that collides with a neighbour makes two answers right.
- **Collision on the unpointed form.** If two entries share one unpointed spelling, flag the pair. Typed grading cannot tell them apart (`typed.go:31-53` strips the points).
- **Verb forms are derivable.** A verb's stored forms must agree with its binyan template. Mismatches demote the entry to T2.
- **A second source agrees.** The best available is a different AI answering blind (see consideration 6). A word list or dictionary dump, if the license allows, is stronger. That is a question for the Technical Architect.

An entry that fails any check is held with a reason code. It is never silently fixed.

**3. The typed-spelling problem.**

`GradeTyped` compares letters exactly after stripping points. Hebrew has two legitimate ways to spell many words: full spelling (extra vav or yod) and defective spelling. A learner who types the full form against a defective key is off by one letter, or by two if both vav and yod differ. By the doc comment, distance 1 is Hard and anything more is Again (`typed.go:10-27`). Distance 2 marks a valid spelling wrong. That is exactly the failure Brian hates. I did not check whether Hard counts as right in the caller, so that needs confirming.

The reverse risk is just as real. Distance 1 can accept a *different word*. `בית` and `בת` differ by one letter and mean different things.

Two tempting fixes are bad:

- **Fold all vav and yod away.** Then `בית` equals `בת`. A wrong word is accepted.
- **Leave it and rely on the button.** Then the button fires on every full-spelling answer, and the key is never fixed.

My fix:

- **Give each lexeme an accept-set.** It holds the canonical spelling plus known variants (full and defective).
- **Allow a folded match only if it is safe.** Fold vav and yod on both sides. Accept the typed answer only if the fold equals this word's fold **and no other taught lexeme has the same fold.** This is checkable offline across the whole lexicon in one script.
- **Tell Brian when a variant was accepted.** "Also correct: full spelling." Never silently pass it.

The test set (a plain table file, one row per case: expected, typed, wanted grade, category):

- Exact match, and the same with points added or removed.
- Final letters: `ך` vs `כ`, `ם` vs `מ`. These must stay different (`typed.go:10-17`).
- Maqaf vs hyphen, extra spaces, tabs, and non-breaking spaces.
- Invisible marks from pasted text, such as U+200E and U+200F. They sit outside the stripped range U+0591–U+05C7 (`typed.go:36`), so I expect them to count as letters. I have not run it.
- Geresh and gershayim (U+05F3, U+05F4), also outside the range.
- Presentation-form letters from a Hebrew keyboard or paste.
- Full vs defective spelling, with vav only, yod only, and both.
- Minimal pairs from the lexicon: every pair at edit distance 1 that means something different. These must not be accepted as each other.
- Prefixed forms (`ו`, `ה`, `ב`, `ל`, `מ`, `ש`).
- Mistakes to catch: a wrong sofit, a swapped `ס`/`שׂ`, and `ח` vs `כ`.

Where the rows come from:

- Every lexicon entry run against itself (must be Good).
- The same entry with points stripped (Good).
- Every vav/yod variant (accepted, unless the collision check says no).
- Every row of Brian's past typed misses that were later voided as `also_right`. Those are labeled data: Brian said these were right.

Also add one property test: `activity.Align` distance must equal `GradeTyped` distance on random strings. That keeps the two copies in step.

**4. One-right-answer checks for gap and order steps.**

*Gap (cloze).* A gap is safe only when exactly one option fits. Code can check a strict version:

- The blank is one token with a T0 or T1 entry.
- The sentence has an English line fixing the meaning.
- Each distractor must differ from the key in part of speech, or in a stored feature (tense, gender, number). Swap each distractor into the sentence. If the swapped sentence is a tutor line or matches a stored form, reject it.
- If a distractor could fit, drop it. If none are left, the step becomes an ungraded reveal.

*Order.* Hebrew word order is flexible. Time words and adverbs move. A pronoun subject can come before or after the verb. Code cannot prove one order without a parser. So:

- **Order stays an ungraded reveal in the first release.** That is what D3 already says (ASMT-006 A8).
- If order is graded later, use only chunked sentences of four chunks or fewer (a noun plus its adjective counts as one chunk), and take the key from a tutor line.

I would rather grade less and be right.

**5. Fixed sample lessons that rerun on every change.**

Build a **golden set** of at least six frozen lessons:

- One word lesson, one conjugation, one listening, one easy, one spelling, and one built around a held tutor line.
- Each has a frozen DB snapshot, a frozen clock, and a frozen random seed.
- The test assembles the lesson and writes out every step as plain text: stem, options, key, accept-set, and the taught set at that point. It compares that to a saved file.
- Edge inputs go in on purpose: an unpointed tutor line, a token with no lexicon entry, an irregular verb, a final letter, a maqaf, and an AI story with one unknown word.

On top of the snapshots, run **invariants over every assembled lesson**, not only the golden ones:

- **I1 Taught only.** Every Hebrew token on screen is taught.
- **I2 Key present once.** The key appears in the options exactly once.
- **I3 One right answer.** No distractor is also right.
- **I4 Pointing valid.** Every pointed string passes the well-formed check.
- **I5 Keys are tier T0 or T1.**
- **I6 Perfect student.** A bot answers every graded step with the key and every accepted variant. All must grade right.
- **I7 Wrong student.** The bot answers each distractor. All must grade wrong.

The perfect-student run is the most useful of these. It tests the exact complaint: a right answer marked wrong. It costs almost nothing, because grading is local code.

Run these two ways:

- **On every `go test`**, against the golden set and a copy of the real DB.
- **At serve time**, in the gate that `nextTodayLesson` already has (`shell.go:100-107`, `142`). A lesson that fails its own perfect-student run is not offered.

When one step fails, drop that step to an ungraded reveal. Do not drop the whole lesson.

**6. Does a second AI reviewer help?**

It helps if it **finds problems and never grades.** I would use it in three narrow ways, offline and in batch, never in the serve path:

- **Blind solve.** A different model gets the stem and options, without the key, and answers. If it picks another option, or says two fit, hold the step.
- **Independent gloss and pointing.** A different model proposes the gloss and pointing for a new entry. A mismatch demotes it to T2.
- **Read the story.** A model flags a sentence that reads wrong or uses a word outside the taught set.

Limits, stated plainly:

- Two language models can share the same mistake. Agreement is not proof.
- Models are weak at Hebrew pointing. Never trust a model where code can check.
- The reviewer may only **demote or hold.** It can never promote an entry on its own, and it never decides whether Brian was right.
- Measure it. Compare what the reviewer flags with what Brian flags over four weeks. If it catches little, delete it (D13: delete first).

**7. The flag-to-fix loop.**

Brian wants a button that voids the miss and flags the item. Today the void exists (`also_right.go:53-60`) and the flag does not. Here is the loop, with what code does at each step.

1. **Tap.** Show "I was right" on a miss and "This question is wrong" on any graded step, including a right answer. A right answer on a wrong key is bad evidence too. Today the handler refuses a correct attempt (`also_right.go:49-52`), so the second path needs its own handler.
2. **Void and suspend in one call.** Void the attempt. Then write an `item_flags` row: the key's lexeme and skill, the step type, a hash of the stem and key, what Brian actually typed or picked, and the time. **Suspend the item at once.** Suspending is harmless. At worst a good item sits out for a while.
3. **Code triages.** Re-run the checks on that item with Brian's answer.
   - If Brian's answer is a safe variant (same lexeme, passes the collision check), add it to the accept-set automatically.
   - Otherwise do **not** accept his answer automatically. He may be wrong, and the button is a way to avoid a hard miss. The item stays suspended with a diagnosis.
4. **A fix queue.** The flagged items go to a queue an outside AI reads over MCP (a `get_flagged_items` tool). It proposes a fix. The fix goes through the **same code gates**. The item returns to use only if the gates pass.
5. **Every fix becomes a test.** A fixed flag adds a row to the typed test set or a golden lesson. A bug that was found once cannot come back.
6. **Escalate.** A second flag on one lexeme demotes it to T2. Many flags in one lesson type in one day suspends that type and falls back to review.

Watch two things: the **flag rate** (flags per 100 graded answers), and the **share of flags that code confirmed as real key errors.** A high rate with few confirmed errors means the button is being used to skip hard items. That is useful to know. It also needs the Data Analyst: voided attempts must not count toward "proved" (D4) or S2.

**8. What happens to a class-document line that fails a check (Brian delegated this).**

My decision, by failure kind:

| Failure | What happens |
| --- | --- |
| **Not pointed** (190 of 350 lines) | This is not a failure. Show it unpointed. Do not invent points. Typed grading already ignores points. Word taps show the lexicon's pointed form. |
| **Unknown tokens** (no lexicon entry) | The line may show verbatim. The unknown tokens are not blanked or keyed. A gap may blank only a T0 or T1 token. |
| **Garbled** (mixed scripts, broken order, stray marks, cut off) | Hold the line. Keep it with a reason code. Never show it. Never delete it. |
| **Fails a safety check** (e.g. contains a word that is not taught) | Hold for graded steps. It may appear in an ungraded read only if every word is taught. |

A held line is **retried automatically** whenever the lexicon grows or a new import lands. No person reviews it. A small count of held lines by reason sits on an admin page, so a rising number is visible.

I would not drop failing lines. With 286 usable lines, a line that is "unknown tokens only" is the most likely to become usable after the next import.

**9. Minimum quality bar for the first release.**

Every item is a pass/fail check that code can run. Numbers are my proposals.

- **Q1** No graded step has a key below T1.
- **Q2** Every assembled lesson passes the perfect-student and wrong-student runs (I6, I7) at 100%.
- **Q3** The taught-only invariant (I1) has zero violations across the golden set and a synthetic 30-day run.
- **Q4** The typed test set has at least 150 rows. Zero failures on "every spelling the lexicon holds must be accepted." Known gaps are written in a list in the test file, not hidden.
- **Q5** Every pointed string on screen passes the well-formed check. Unpointed text is shown unpointed.
- **Q6** The button works on every graded step. A browser-level test proves the void, the flag, and the suspension all happen in the same request.
- **Q7** The golden lessons run on every change and fail the build on any diff.
- **Q8** Any lesson type can be switched off, with review as the fallback.
- **Q9** After release: if flags exceed 10% of graded answers for three days, that lesson type suspends itself.

First-release scope, to make the bar reachable: **graded** are the meaning check (single-word `recall`) and typed write on T0/T1 words. Gap is graded only when it passes the one-right-answer proof. Order, multi-word listening, and sentence use stay ungraded reveals. Everything blocked on audio stays blocked.

#### Opportunities

1. **The perfect-student bot is cheap and strong.** Grading is local code, so the bot runs in milliseconds. It tests the exact complaint (right marked wrong) for every lesson ever built, not just the ones someone remembered to check.
2. **The flag becomes a growing test suite.** Every real mistake Brian hits turns into a permanent test row. In a few weeks the typed test set is based on his actual typing, not guesses.
3. **Existing hooks make this small.** The Today gate (`shell.go:100-107`, `142`), the void path (`also_right.go:53-60`), and the lesson run lookup (`also_right.go:200-213`) already exist. The new work is mostly an invariant checker, an `item_flags` table, and a test file.
4. **The collision script is a one-off with high value.** One run over the lexicon lists every pair of words that typed grading cannot tell apart, and every pair that distance-1 grading would accept as each other. That shows the real size of the spelling problem before any design choice.
5. **Replace rules with tests.** The invariants live in the assembler and `go test`, not as new validator rules. ADR-0033 D13 says a new rule retires an old one. Tests do not cost rule budget. This avoids the treadmill from BRAIN-001.
6. **A free trust signal for Brian.** Flag rate and held-line counts on one page tell him if quality is rising, without a reviewer.

#### Risks and Concerns

| # | Risk | Severity | Why it matters | Mitigation |
| --- | --- | --- | --- | --- |
| R1 | **Typed spelling marks a valid answer wrong** (full vs defective spelling, distance 2). | **High** | This is Brian's top complaint class. It will start the moment lesson `produce` becomes typed (ASMT-006 A6), and review `type_it` is exposed now (`typed.go:10-27`). | Accept-set per lexeme, collision-checked fold, and the test set. Run the collision script first. |
| R2 | **An AI-proposed entry becomes an answer key with a wrong gloss or wrong pointing.** | **High** | 742 of 974 tokens are unmatched, so many keys will be AI-proposed. A wrong key is wrong every time it comes up, and looks like a bug to Brian. | Trust tiers. Only T0 and T1 may be keys. Code checks on every entry. Two flags demote a word. |
| R3 | **A word Brian was never taught shows up** (a distractor, a story word, or a gap option). | **High** | Past complaint: buttons for words he was never taught. Distractors are the easy place to miss. | Taught-only invariant I1 on every on-screen token, including distractors. Tutor-doc words count as taught. |
| R4 | **Distance 1 accepts a different word** (`בית`/`בת`). Hard may count as correct. | **Medium** | A wrong word counts as nearly right. It teaches the wrong thing quietly. I did not verify how the caller treats Hard. | List the minimal pairs from the lexicon. For each, require exact match. Check the caller. |
| R5 | **The button becomes a shortcut around hard items.** | **Medium** | It erases the miss. Over-use would inflate "known" without learning. | Only auto-accept safe variants. Log the share confirmed by code. Keep the held-out probe (S3) as an independent measure. |
| R6 | **Two copies of the normalizer drift apart** (`typed.go:31-53`, `align.go:14-34`). | **Medium** | Grading and the letter-confusion feedback would disagree about what a typed answer is. | Property test that the two agree. Better: have one call the other. |
| R7 | **Gap steps with two valid answers.** | **Medium** | Past complaint: "several answers right but only one counts." | Strict one-fit proof. Fall back to an ungraded reveal. |
| R8 | **The second AI reviewer gives false comfort.** | **Medium** | Two models can agree and both be wrong. | Veto-only. Measure its catch rate against Brian's flags. Delete it if weak. |
| R9 | **A held-line pile grows and nobody notices.** | **Low** | Lines held for unknown tokens never recover. | Auto-retry on lexicon growth. A count by reason on an admin page. |
| R10 | **Skill name mismatch silently schedules a wrong card** (spell vs recall, ASMT-006 A7). | **Low** | Small data error, but it feeds the "proved" ladder. | Fix before typed `produce` ships. Add one test that both layers agree. |
| R11 | **Checks slow the build or serve path.** | **Low** | Brian wants lessons within about two weeks. | Checks are local, in-process, and cheap. The AI reviewer runs in batch, never at serve time. |

#### Recommended Approach

1. **Decide the permission model first.** Two words: *taught* (may appear) and *keyed* (may be graded). Write it down in one short page. Everything else follows.
2. **Run the two one-off scripts** before building. One lists lexicon collisions on the unpointed form. The other lists distance-1 pairs. Their output sets the real size of R1 and R4.
3. **Fix typed spelling next.**
   - Add the accept-set and the collision-checked fold.
   - Unify the two normalizers.
   - Write the 150-row test file.
   - Do this before lesson `produce` goes typed.
4. **Add the trust tiers** and the entry checks. Everything not T0/T1 is ungraded.
5. **Build the invariant checker** (I1–I7) as plain Go tests plus the Today gate. Build the six golden lessons.
6. **Build the flag loop.** An `item_flags` table, a suspend flag on the item, the two buttons, and the `get_flagged_items` tool. Each fixed flag becomes a test row.
7. **Add the AI reviewer last**, as veto-only and batch, if the first four weeks of flags show a gap that code cannot catch.
8. **Ship the first release at the quality bar above (Q1–Q8).** Run Q9 as a self-suspending health check.
9. **Hold the first graded scope small:** meaning check, typed write on keyed words, and gap only with proof. Add order grading only after the flag rate is low.

I would not add a new validator rule for any of this. Use tests and the assembler. That keeps within ADR-0033 D13.

#### Questions for Other Specialists

1. **Product Strategist:** Are you willing to ship the first release with *most graded steps limited to the meaning check and typed write*, and with order, sentence use, and multi-word listening ungraded? That is the scope my quality bar assumes. If you need more graded types, which one comes first?
2. **Product Strategist:** The story is allowed about 5 to 10% unknown words (90–95% coverage). That conflicts with "never show a word he was not taught." Do you accept that "taught" includes anything in a class document he attended, with tap-for-meaning as the fallback? Or should unknown words in a story be cut?
3. **UX Designer:** Where do the two buttons ("I was right" and "This question is wrong") go, and what does Brian see after the tap? I need them on right answers too, and I need the item to vanish quietly for a while. How do we keep the button from becoming a way to skip hard questions without making it hard to find?
4. **UX Designer:** When code accepts a spelling variant ("also correct: full spelling"), what is the lightest message that teaches without a lecture?
5. **Technical Architect:** Can the assembler run the perfect-student and wrong-student checks in-process in the serve path, and can the existing Today gate (`shell.go:100-107`, `142`) carry it? Where does the per-item suspend flag live?
6. **Technical Architect:** Is there a licensed Hebrew word list or morphology source we can bundle for the "second source" check on new entries? What is the cheapest way to strip prefixes and resolve inflected forms, so we learn how many of the 742 unmatched tokens are really gaps?
7. **Data Analyst:** Should a voided attempt, and any attempt on a flagged item, be excluded from "proved" (D4) and from S2? How do we tell a real key error from Brian dodging a hard word, using only the flag log and the S3 probe?
8. **Data Analyst:** What flag rate per 100 graded answers is "healthy" for week one, and what rate should auto-suspend a lesson type? I proposed 10% for three days. Does the data support a better number?
9. **Devil's Advocate:** I claim a code-only quality gate plus a veto-only AI reviewer is enough with no human. What is the strongest case that Brian himself should do a short, one-time review of the first batch of AI-proposed entries? What is the cheapest version, such as 20 entries?
10. **Devil's Advocate:** Is a six-lesson golden set plus a 150-row typed test set too much test infrastructure for a one-learner app? What is the smallest set you would accept that still catches "right answer marked wrong"?

---

### Summary

**Position:** No lesson step may be graded unless code can prove its answer key. Words may appear if they were taught, but only verified lexicon entries (T0 tutor-attested, T1 two-source) may be keys. Typed spelling needs an accept-set per word, a collision-checked fold, and a test file. A perfect-student bot, golden lessons, and an item-level flag loop turn each failure into a test, so quality improves with no human reviewer.

**Recommendation:** Grade only provable keys (meaning check, typed write, and strict-proof gap), prove it with a perfect-student run on every lesson and a 150-row typed test set, and make the "I was right / this question is wrong" button void the miss, suspend the item, and add a permanent test row.

**Top Risk:** Typed spelling marks a valid answer wrong. Full vs defective spelling can differ by two letters, which `GradeTyped` grades Again (`internal/fsrs/typed.go:10-27`), and the risk starts when lesson `produce` becomes typed. The reverse risk (a distance-1 match accepting a different word) is nearly as serious.

**Confidence:** 0.70. The code facts are read directly. Three points are unverified: how callers treat Hard, how many of the 742 unmatched tokens resolve after prefix stripping, and whether the stored verb forms themselves are reliable. The thresholds (150 rows, 10% flag rate, six golden lessons) are my proposals and should be tuned with the Data Analyst.

---

## Phase 4: Dialogue Contributions

### Questions Posed

1. **Product Strategist:** Will you ship with most graded steps limited to the meaning check and typed write, and with order, sentence use, and multi-word listening ungraded? If you need more graded types, which comes first?
2. **Product Strategist:** Does "taught" include anything in a class document he attended, with tap-for-meaning as the fallback? Or should unknown words in a story be cut?
3. **UX Designer:** Where do the "I was right" and "This question is wrong" buttons go, and what does Brian see after the tap? How do we keep the button from becoming a way to skip hard items?
4. **UX Designer:** What is the lightest message when code accepts a spelling variant, so it teaches without a lecture?
5. **Technical Architect:** Can the assembler run the perfect-student and wrong-student checks in-process at serve time, and can the Today gate carry them? Where does the per-item suspend flag live?
6. **Technical Architect:** Is there a licensed Hebrew word list or morphology source for a second-source check? What is the cheapest way to strip prefixes and resolve inflected forms, so we know how many of the 742 unmatched tokens are real gaps?
7. **Data Analyst:** Should voided attempts and attempts on flagged items be excluded from "proved" (D4) and from S2? How do we tell a real key error from Brian dodging a hard word?
8. **Data Analyst:** What flag rate per 100 graded answers is healthy in week one, and what should auto-suspend a lesson type? Does the data support 10% for three days?
9. **Devil's Advocate:** What is the strongest case that Brian should review the first batch of AI-proposed entries himself, for example 20 entries?
10. **Devil's Advocate:** Is a six-lesson golden set plus a 150-row typed test set too much for a one-learner app? What is the smallest set that still catches "right answer marked wrong"?

### Questions Answered

- **To Devil's Advocate (who checks agent-written entries with no human reviewer):** A T0 entry is one where the tutor's own document uses the word with a gloss beside it, so it may be a key. A T1 entry passed every code check and a second independent source agrees, so it may also be a key. A T2 entry may appear only in story taps and ungraded steps. The AI that writes an entry cannot pass its own check.
- **To Technical Architect (buttons after every graded answer, or only after a miss?):** After every graded step. On a miss it reads "I was right". On a right answer it reads "This question is wrong". One request must void the attempt, flag the item, and suspend it. A suspended item is not asked again until it is fixed.
- **To Technical Architect, replying to Product Strategist (can graded word order ship?):** No. Hebrew word order is often free, so one tutor line is not a single right answer. Order stays an ungraded reveal in the first release.

### Tensions Involved In

- **Tension 1, Study first or build first:** Study first. A week of real review on backfilled words catches wrong glosses before any graded step depends on them.
- **Tension 2, How many lesson types in the first release:** Keep the graded set as small as possible. Word lesson only. Typed write only on T0 or T1 words. Gap only with a strict one-fit proof. Order stays ungraded.
- **Tension 3, Does the AI story earn its place:** Keep it optional and ungraded. It is held if any word in it is not taught. The tutor's own lines carry the passage.
- **Tension 4, How much measurement:** The flag rate is a useful quality signal, but only if voided attempts are kept out of "proved". The perfect-student run is the cheap proof and needs no extra logging.
- **Tension 5, Who certifies the lexicon:** A hard gate. The first release grades only T0 words. T1 waits for a licensed second source, and none has been identified yet.

### Synergies Identified

- **Lexicon backfill is the critical path:** shared with Devil's Advocate, Technical Architect, Data Analyst, and Product Strategist. Every type, card, and key reads the lexicon.
- **Typed spelling fix (accept-set, collision-checked fold, fixed test table):** shared with UX Designer and Technical Architect. All three found the extra-ו and extra-י problem on their own.
- **One void, flag, and suspend control on the existing void path:** shared with UX Designer, Technical Architect, and Data Analyst.
- **Tutor lines as the reading passage:** shared with Product Strategist. My T0 tier and their passage rely on the same class documents.
- **Perfect-student run in the Today gate:** shared with Technical Architect, using the existing gate in the shell code.
- **Tests replace new validator rules, and old validator paths are deleted:** shared with Devil's Advocate (the treadmill point) and Technical Architect.

---

## Phase 5: Consensus Position

### Option Scores

Scale: 1 (poor) to 5 (excellent). Scores are for each option as written in dialogue.md, from the QA lens.

| Dimension | A | B | C | D | Reason |
| --- | --- | --- | --- | --- | --- |
| Learning effect (backed by research) | 3 | 3 | 4 | 4 | The core word lesson uses the best-backed parts: spaced recall (RSCH-003, High), retrieval (Moderate), and first-language gloss with coverage-graded input (Moderate). Typed write is Low confidence (RSCH-005) in every option. A adds types with no evidence yet (RSCH-004 has no count of new words per lesson). B adds an easy lesson with no cited evidence. |
| Trust (no right answer marked wrong, no untaught words) | 2 | 3 | 3 | 4 | A ships all five types with no gates named. B names accept-sets but grades from an unverified lexicon. C backfills with tiers and a week of real use, but names no accept-set or flag control. D adds tiers, accept-sets, and a one-item void and suspend. |
| Time to new study (about 2 weeks) | 1 | 2 | 3 | 3 | New words reach Brian in review at once under C and D. The lesson waits for the Stage 1 gate on 2026-11-30 and a build of about 3 weeks. A is five builds in sequence. B tries to build a lesson before the gate, which the Stage 2 rule forbids. |
| Build cost and treadmill risk (fewest specs and rules) | 1 | 2 | 4 | 3 | C is one build with no new surfaces. D adds the flag control and typed accept-sets to slice 1, but also deletes the old validator paths for these lessons. A is five types. B is two types plus the flag loop. |
| Fixes "feels the same" | 4 | 3 | 2 | 2 | A varies the shape most, but only after five builds. B adds a second shape now. C and D add one new shape, with per-class tutor passages and a stated tripwire if Brian says "same again". That is a bet, not proof. |
| Measurable (can we tell if it works) | 2 | 3 | 3 | 4 | A changes five things at once, so cause is hard to separate from effect. C has a clean review-versus-lesson baseline but no retire rule. D sets a retire rule and a "same again" tripwire before the build, plus the flag rate and the perfect-student run. |
| **Total (of 30)** | **13** | **16** | **19** | **20** | |

### Final Position

**Preferred Option:** D. Count and backfill the newest classes now, study them in review, then build one word lesson as the only first slice.

**Rationale:** D is the only option that puts the key-proof gates (T0 tiers, accept-sets for typed spelling, and the perfect-student run) in front of the first graded step. It also starts with one build and agrees the retire rule before building. This holds only if the three release tests agreed in Round 4 are the bar.

**Dissent:** The item-flag table and the fix-queue tool should wait until flag data exists. Slice 1 needs only the void, flag, and suspend on the existing void path. One new void reason in a small migration is enough to store the suspension.

---
## Information Injection R1: Lessons not tied to a recent class
**Injected at:** After Phase 5, before Phase 6
### Re-Evaluation
**Impact Assessment:** High
**Position Change:** Changed
**Analysis:** The gates do not depend on class age. A T0 key is still a word whose tutor line shows the word and its gloss, wherever that line sits in the folder, so the tier rule applies to older sheets once each one passes the same per-sheet check. But the audit covered only 10 of about 85 sheets (dated 14 Sep to 3 Nov), and it did not count gloss adjacency, so nothing yet shows older sheets are T0-eligible. Words beyond the classes have no tier and fail the taught-only rule (I1), so "always something new" must be met from older tutor sheets; and a lesson with no graded keys passes the release tests trivially, so "always a lesson" needs a minimum graded floor or it ships weak lessons on heavy days.
**Prior Recommendation:** Option D, amended: word lesson only in slice 1, heavy-day Today shows review only, backfill the newest three classes, release bar of three tests.
**Revised Recommendation:** Keep D, but make the T0 rule per sheet and not per date, replace review-only on heavy days with a minimum-lesson floor (easy lesson in slice 1 for that use), and extend the release bar to cover older-sheet lessons, held lines, and zero-key lessons.
**New Risks or Opportunities Surfaced:**
- **Vacuous pass:** a lesson with zero graded keys passes the perfect-student and wrong-student runs trivially, so the tests cannot catch a weak heavy-day lesson. It needs a graded floor, proposed at 2 or more new T0 words and 4 or more graded items on T0 keys, to be tuned with the Data Analyst.
- **Unverified sheets:** 75 of about 85 sheets have never been loaded. Older sheets may use a different layout, so the 100% loadability and the 286-line count from the newest 10 do not carry over.
- **Unmeasured T0 pool:** SPEC-012 counted lexicon misses, not gloss adjacency or tutor corrections. The 742 misses do not tell us the T0 pool size. The 6 tutor corrections in the audited sheets must be checked against any key they touch.
- **Non-class files:** study notes, the zip, and the html were excluded from the audit. They must stay out of the taught set and out of the backfill.
- **Opportunity:** older tutor sheets meet the "something new" need with tutor-attested words, not AI-proposed ones. That lowers R2 (wrong AI-proposed keys) rather than raising it.
- **Provenance gap:** `record_taught_words` with no sheet reference gives a word taught status with no source. Every recorded word needs its sheet.
**Confidence Adjustment:** Decreased. The gate logic is unchanged, but the supply and heavy-day assumptions rest on 10 of about 85 sheets, and the gloss check that defines T0 has not been measured.
