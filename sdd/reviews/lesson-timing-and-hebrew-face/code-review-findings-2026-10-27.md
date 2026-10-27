# Code Review: Timed meaning questions, recall speed over MCP, and the Hebrew face of a lesson word

**Date:** 2026-10-27
**Reviewer:** `code-reviewer` persona (inline fallback), via `sdd-code-review`
**Target:** the uncommitted working tree on `main` after `1d697f1`: `migrations/018_lesson_asked_at.sql`, `internal/db/{lesson,coach}.go`, `internal/evidence/{grade,doc}.go`, `internal/web/lesson.go`, `internal/web/views/player.templ`, `internal/mcp/{coach,server,tool_learner_words}.go`, `internal/mcpstore/coach.go`, and their tests
**Review Depth:** standard
**Overall Assessment:** Request Changes

**Disclosures.** This repo has no `.claude/agents/code-reviewer`, no `scripts/dev/security_scan_indexer.py`, and no `sdd/security-reviews/` tree. Phase 0 step 3 and Phase 7 (scan id, ledgers, index row) were skipped, and the persona was applied inline. This file is the whole record. Security-class concerns were not reviewed. Phase 4 was done by reading; the test results quoted come from the docker runs made while building the change. One probe was run to read FSRS intervals (below) and was deleted afterwards. Phase 6 did not create an initiative; the next steps are listed at the end instead.

---

## Code Review Summary

The change does what was asked: a meaning question is timed from when it appears, the time is stored, a fast right answer waits longer and a slow one comes back sooner, the numbers reach the agent over MCP, and the face of a lesson word is pointed Hebrew. One problem blocks shipping. A word the learner met seconds earlier is still in short-term memory, so it is answered fast, rated Easy, and scheduled 16 days out on its first rating, where a normal right answer brings it back in 10 minutes. Three Major findings follow: a lesson resumed later measures the whole gap as answer time, the MCP speed field counts wrong answers and is attached to the wrong card, and the new behavior contradicts the written North Star without a recorded decision.

---

## Findings Summary

| ID     | Severity   | Category     | Title                                                                                  | Status |
| ------ | ---------- | ------------ | -------------------------------------------------------------------------------------- | ------ |
| CRIT-1 | Critical   | correctness  | A word answered fast seconds after it was taught is scheduled 16 days out              | Fixed  |
| MAJ-1  | Major      | correctness  | A resumed lesson counts the time away as answer time                                   | Fixed  |
| MAJ-2  | Major      | correctness  | `recallMs` / `recallSpeed` use wrong answers and attach to both of a word's cards      | Fixed  |
| MAJ-3  | Major      | architecture | The North Star still says Latin shows on every taught word, and has no slow rule       | Fixed  |
| MAJ-4  | Major      | test         | The slow path, `RecallSpeed`, and the MCP fields have no tests                         | Fixed  |
| MIN-1  | Minor      | correctness  | The meaning question offers the word's audio, so the timed answer mixes reading and hearing | Open   |
| MIN-2  | Minor      | architecture | Latin typed by hand in lesson prose and explainers has no rule                         | Open   |
| SUGG-1 | Suggestion | architecture | Give the speed baseline its own context, and extend timing to the other checks         | Open   |

**Totals:** 1 Critical, 4 Major, 2 Minor, 1 Suggestion

---

## Critical Issues — Must Fix

### CRIT-1: A word answered fast seconds after it was taught is scheduled 16 days out

**Location:** `internal/evidence/grade.go:39`, `internal/web/lesson.go:476`, `internal/evidence/recorder.go:186-191`

**Problem:** A lesson teaches a word, then asks what it means a few steps later. The answer is fast because the word was just on screen. Once ten timed right answers exist, about half of all right answers beat the median and are rated Easy. For a word with no card yet, `ScheduleNext(nil, …, Easy)` creates a card in review state due in 384 hours. Measured with the shipped parameters (go-fsrs v3.3.1, fuzz off), a first rating gives: Again 1 minute, Hard 5 minutes, Good 10 minutes, Easy 16 days. The lesson-end `INSERT OR IGNORE` then leaves that card alone.

The baseline has the same bias. Only the lesson player writes activity `recall`, so the median is built entirely from answers given right after teaching.

**Impact:** The words the learner knows least are the ones sent furthest away. This is the opposite of the intent ("words that take time need to come back more"), and it hits a common input: every new word in every lesson after the third or so.

**Recommendation:** Give no speed bonus on a card's first ratings. Keep the time; just do not let it lift the rating to Easy until the card has survived a review on a later day.

```text
# Before (recorder.go, default step):
card, log, err := r.review.ScheduleNext(prev, now, rating)

# After:
if rating == fsrs.Easy && (prev == nil || prev.State == "new" || prev.State == "learning") {
	rating = fsrs.Good
}
card, log, err := r.review.ScheduleNext(prev, now, rating)
```

Doing it in the recorder covers every path that grades by speed, and `Grade` stays a pure function. Return the clamped rating so the stored attempt and the review log agree.

**Rationale:** Speed means automatic recall only when the item had time to be forgotten. Ten minutes later, FSRS asks again, and that later answer is the one worth trusting.

---

## Major Issues — Should Fix

### MAJ-1: A resumed lesson counts the time away as answer time

**Location:** `internal/db/lesson.go:390-421` (an existing run is reused without touching `asked_at`), `internal/web/lesson.go:481`

**Problem:** `asked_at` is set when a run starts and when it advances. A learner who opens a question, closes the tab, and comes back tomorrow answers with a latency of about a day. `answerLatency` has no ceiling, so the answer is rated Hard once a median exists, and `recallMs` reports a figure in the tens of millions.

**Impact:** One stale answer pulls a word forward for no reason and puts a false "slow" in front of the agent. Leaving mid-lesson is normal on a phone.

**Recommendation:** Treat a long answer as untimed. A ceiling also covers a learner who walks away mid-question without closing anything.

```go
// maxAnswer is the longest an answer can take and still count as timed.
const maxAnswer = 60 * time.Second

func answerLatency(clk clock.Clock, asked time.Time) time.Duration {
	if clk == nil || asked.IsZero() {
		return 0
	}
	d := clk.Now().Sub(asked)
	if d <= 0 || d > maxAnswer {
		return 0
	}
	return d
}
```

Optionally also reset `asked_at` when the ask screen is served again by a GET on an existing run.

### MAJ-2: `recallMs` / `recallSpeed` use wrong answers and attach to both of a word's cards

**Location:** `internal/db/coach.go:133-141`

**Problem:** The subquery takes the latest timed `recall` attempt for the lexeme. It does not filter on `correct`, so a wrong answer given in one second is reported as `fast`. It does not filter on skill, so the English-to-Hebrew `recall` card shows the speed of the Hebrew-to-English meaning question, which is a `read` attempt.

**Impact:** The agent is told to bring slow words back sooner. With these two gaps it will deprioritize a word the learner just got wrong quickly, and it will judge production by recognition speed.

**Recommendation:**

```sql
SELECT a.latency_ms FROM attempts AS a
WHERE a.user_id = c.user_id AND a.lexeme_id = c.lexeme_id
  AND a.skill = c.skill AND a.activity = 'recall'
  AND a.correct = 1 AND a.latency_ms > 0
ORDER BY a.created_at DESC, a.id DESC
LIMIT 1
```

The index `attempts_user_lexeme_skill_id (user_id, lexeme_id, skill, created_at, id, correct)` then serves the lookup directly. Update the MCP description to say "the latest timed right answer".

### MAJ-3: The North Star still says Latin shows on every taught word, and has no slow rule

**Location:** `docs/north-star.md` section 2 ("Teach with help, test without it … Fading transliteration is an opt-in setting, never the default"), section 4.2 ("Right is Good. Right and faster than my own median … is Easy"), section 6.1 table ("Shown on every word")

**Problem:** The code now keeps the Latin reading on the back of the card by default and rates a slow right answer Hard. Both are reasonable and both were asked for, but the document every agent and author is pointed at still says the opposite. `ADR-0018` is the grading record and is already dirty in the working tree with unrelated edits.

**Impact:** The next agent that follows the North Star will "fix" the card face back, or write lessons assuming Latin is visible. This is the same drift the 2026-10-23 word-presentation review found.

**Recommendation:** Record the decision in a new `ADR-0020` (lesson word face and speed grading), and update North Star sections 2, 4.2, and 6.1 to point at it. State the rule as: the face of every lesson word is pointed Hebrew; the Latin reading and meaning are one tap away; a tested word shows neither until the answer.

### MAJ-4: The slow path, `RecallSpeed`, and the MCP fields have no tests

**Location:** `internal/evidence/grade_test.go`, `internal/web/lesson_attempts_test.go`, `internal/mcpstore/` (no coach test touches the new fields), `internal/db/` (no test for the new subquery)

**Problem:** Grade's slow boundary is tested. Nothing tests `RecallSpeed`, a lesson answer that is rated Hard for being slow, the mapping of `recallMs`/`recallSpeed` in `mcpstore.LearnerCards`, or the coach subquery. `TestLessonRecallStoresResponseTime` covers the stored latency and a Good rating with no median.

**Impact:** CRIT-1 and MAJ-2 are exactly the kind of mistake those tests would have caught.

**Recommendation:** See Testing Recommendations.

---

## Minor Issues — Consider Fixing

### MIN-1: The meaning question offers the word's audio

**Location:** `internal/web/views/player.templ` `wordClipRow` and the orb under the tested word; `TestMultiTokenRecallNamesTheTarget` asserts the target has a play bar during the ask

**Problem:** The North Star's rule is one skill per question: "A reading question has no audio and no transliteration", and a replay counts as help. A learner can play the tested word before answering. The answer is then partly a listening answer, the time includes the clip, and the replay is not recorded.

**Recommendation:** Hide the tested word's clip until the answer, or count a play as `HintUsed` (Hard) and leave the answer untimed.

### MIN-2: Latin typed by hand in lesson prose and explainers has no rule

**Location:** unit concept and situation text (for example unit 4: "im ata shome'a"), `internal/web/views/rich.templ:37-39`

**Problem:** The card face is now Hebrew, but an author can still spell a Hebrew phrase in Latin inside English prose, and an explainer always prints Latin beside its Hebrew.

**Recommendation:** Add a `validate_unit` warning when concept or situation text contains a lexicon word's transliteration, suggesting the pointed word instead. Leave explainers as they are until the fade rule in MAJ-3 is decided.

---

## Suggestions — Optional

### SUGG-1: Give the speed baseline its own context, and extend timing to the other checks

**Location:** `internal/mcpstore/coach.go:36`, `internal/web/lesson.go:476`

**Suggestion:** The median mixes all lesson meaning answers. A separate baseline for answers on a later day than the word was taught would make "fast" mean "automatic". Cloze, listen-and-pick, and produce already store their time; once CRIT-1 is fixed, they can schedule the same way, each against its own median, which `CorrectLatencies` already supports.

---

## Positive Observations

- ✅ The migration is one nullable column. Old rows and explicit-column inserts keep working, and an untimed answer is never treated as fast.
- ✅ The answer time is captured before the run is updated, and a repeated post is not a second answer, so a double tap cannot double-count.
- ✅ The lesson-end card creation is `INSERT OR IGNORE`, so a card the meaning question already scheduled is not reset.
- ✅ The lesson card face now matches the review flashcard, which already showed pointed Hebrew alone on the front (`flashTranslit` is empty until the answer side). That closes one row of the 2026-10-23 consistency table.
- ✅ The MCP wording is the same in the server instructions, `get_context`, and `get_learner_words`, and the pinned authoring phrases were left untouched.
- ✅ Every test that encoded the old card face was updated to assert the new one, including the front/back split in `flip_cards_test.go`.

---

## Testing Recommendations

- `TestRecallSpeed`: zero latency, zero median, just under the median, exactly the median, exactly twice, just over twice.
- A lesson test that seeds ten timed right answers, then answers a new word fast and asserts the card is due in minutes (CRIT-1), and answers slowly and asserts Hard.
- A lesson test that sets `asked_at` two hours back and asserts the stored latency is 0 (MAJ-1).
- A db test for `CoachCards`: a wrong fast answer is ignored, and the `recall` card of the same word has no `RecallMs` (MAJ-2).
- An `mcpstore` test that `recallSpeed` stays null below ten timed answers and is `slow` above twice the median.

---

## Documentation Needs

- `ADR-0020` and the North Star updates in MAJ-3.
- `internal/evidence/doc.go` should say Easy is withheld on a card's first ratings once CRIT-1 lands.

---

## Next Steps

1. **Before merge:** CRIT-1.
2. **Should fix soon:** MAJ-1, MAJ-2, MAJ-3, MAJ-4.
3. **Track for later:** MIN-1, MIN-2.
4. **Optional:** SUGG-1.

---

## Beyond this diff: consistency and learning

Asked: how consistent is the app, and how can it better help people learn Hebrew and the ways of the Jews.

**Consistency today.** The word face is now consistent between lessons and review. The remaining drift is between the written rules and the code (MAJ-3), and between the author's prose and the card (MIN-2). The pattern repeats from 2026-10-23: each change fixes the surface it touches and the governing document is left behind. A rule of thumb that would stop it: a change to what the learner sees or how a word is graded updates the North Star or an ADR in the same commit.

**Learning, in order of payoff.**

1. **Fade by mastery, then fade the vowels.** The North Star already plans a `fade_translit` setting and unpointed reading as a fifth skill. With word states and timing in place, the rule can be per word: the Latin reading is on the back while a word is new, gone once it is `known` for reading and answered fast, and the vowel points follow the same path once band 1 is `solid`. That is the road to reading a siddur, a Chumash, or a newspaper without help.
2. **Use speed for fluency.** North Star section 4.5 already says "Right but slow: a fluency pass using known words only". `get_lesson_brief` can list slow words so the agent writes that pass. Today the speed only reaches the agent on individual cards.
3. **Turn on the Isaiah reader.** Its fourth layer is exactly "the manner of the Jews": parallel lines, wordplay, idioms, and how Jewish tradition reads the verse. Isaiah 40:1 is drafted and waits for one check at `/admin/isaiah/40/1`. The reader stays hidden until then. Linking each verse word to its root page connects the verse to everyday words that share the root.
4. **Teach living practice through the speaking path.** Short units for Shabbat greetings, the common blessings, and the next holiday on the calendar use high-frequency, culturally central words, and they give the learner something to say at a real table. They fit the existing unit and brief model.
5. **Mark the register of a word.** Biblical, liturgical, and modern Hebrew share roots and differ in use. A register field on a lexeme lets the brief keep a modern speaking lesson modern, and lets the Isaiah bridge layer show the shift on purpose.
6. **Hold cultural claims to the Isaiah standard.** Any note on how tradition reads, prays, or observes something stays a draft until a person checks it, and names its source. The Isaiah tools already work this way; reuse them for culture notes in units.
