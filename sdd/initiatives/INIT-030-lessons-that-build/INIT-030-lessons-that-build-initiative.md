# INIT-030: Lessons that build on each other

**Initiative ID:** INIT-030 · **Status:** complete · **Created:** 2026-11-05 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian, 2026-11-05: "Our lesson plans are the same over and over. They kind of suck. They aren't progressive. Maybe we need a way for the agents to look at undone lessons and enhance them based on previous lessons." Evidence: [RSCH-004](../../research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md) (about three quarters of a unit on meaning, a quarter on deliberate study, studied words coming back in later meaning-focused work, and a fluency pass with no new language), [RSCH-003](../../research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) (spacing High, and no fixed count of encounters), and [RSCH-011](../../research/RSCH-011-practice-layouts-that-teach/RSCH-011-practice-layouts-that-teach-report.md) (checked questions on known words, and sentence build).

Closed 2026-11-05. Specs 1 to 6 are in this checkout. SPEC-007, the rewrite of learner 1's untouched units, is left for the next agent, after this commit is on the server.

## What we found

Learner 1 has 33 learner units, at positions 1 to 33. Units 8, 20, 30, and 34 were read through `get_unit`, and they share one skeleton:

- Each lesson opens with a "story" of one to three short sentences. Unit 30's first story is three words (אוּלַי אֲנִי רָעֵב). The beginner tier allows 20.
- Then each new word gets the same three steps: `shadow`, `listen_pick`, and `recall` of the bare word. About half of every lesson is single-word drill.
- Then one `pattern` line, one `listen_pick`, one `cloze`, one `produce`, and one `say`, mostly on the same sentence as the story.
- Two checkpoints repeat that sentence.
- Then a final checkpoint with the same three fix-the-mistake pairs: קוֹנֶה / לִקְנוֹת, שְׁנֵי סְטֵייקִים, and אֶת הַנַּעֲלַיִים. They are word for word the same in units 8, 20, 30, and 34. `fixMistakes` in `internal/brief/store.go` returns the three most repeated mistakes, and nothing retires one.

Each unit teaches a different grammar pattern, and none comes back. Possession (unit 20), "maybe" (unit 30), comparison, age, going-to, and roots each get one unit. After that, later units use the same few pronoun-and-verb frames.

The words are not sticking. Learner 1's own model shows about 9 known words for listening and 15 for reading. There are 53 reading words in learning. 23 lessons were finished in the last 7 days. The last 50 reviews were 72% right, below the 85% target. Every unit still brings up to 8 new words. `NewWordCount` in `internal/brief/build.go` uses only the tier and the number of struggle words. It ignores recent accuracy and the size of the learning backlog.

The brief is a word list: new words, struggle words, comfortable words, a skill focus, a tier, and the five newest coach notes. It does not name a grammar step. `list_patterns` already has 48 patterns with prerequisites, and the brief ignores them. It does not show the previous unit's story, patterns, or words. Its theme defaults to "general". The writer has nothing to build on, so it repeats the shape it used last time.

`get_context` lists every unit as published. It does not say which units the learner has started or finished. `db.FinishedLessons` already reads that from `lesson_runs`. A writer cannot find the untouched units, so it cannot improve them.

## What you get

- **Every lesson builds on the last.** The brief names one next grammar pattern, chosen from the patterns whose prerequisites are already taught. It names two earlier patterns to use again in the text. It includes the previous units' stories and new words, so the next story can continue from them.
- **The pace follows what you keep.** New words per unit go down when review accuracy is under target or when too many words are still in learning. When the backlog is large, the brief asks for a consolidation unit: no new words, a longer story made of words you are learning, and checked questions on them.
- **Lessons are mostly reading and listening.** Each lesson has a real story or dialogue that is long enough for the tier and gets longer as you know more. Single-word drills are capped. Most questions are on sentences: listen, cloze, build the sentence, and say it.
- **Mistakes rotate.** A fix-the-mistake pair that has been in the last two units, or that you have since answered right, gives way to a newer one.
- **Agents fix the lessons you have not started.** `get_context` marks each unit untouched, started, or finished. An agent can take an untouched unit, get a revision brief for it as of today, and rewrite it against everything before it. A unit you have started or finished is not changed.
- **No piling up.** A new brief waits while three or more untouched units are already on Today. The agent revises the next one instead.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-progressive-lessons.md) | docs | ADR: lessons that build on each other | — | 2h | complete |
| [SPEC-002](specs/SPEC-002-data-unit-progress.md) | data | Untouched, started, or finished, per learner unit | SPEC-001 | 2h | complete |
| [SPEC-003](specs/SPEC-003-service-progression-brief.md) | service | The brief plans the next step: pattern, pace, previous units, mistakes | SPEC-001 | 5h | complete |
| [SPEC-004](specs/SPEC-004-content-shape-rules.md) | content | Validator rules for story length, drill share, and recycling | SPEC-003 | 5h | complete |
| [SPEC-005](specs/SPEC-005-mcp-revise-untouched.md) | mcp | Unit progress in `get_context`, revision briefs, and the backlog cap | SPEC-002, SPEC-003 | 4h | complete |
| [SPEC-006](specs/SPEC-006-mcp-authoring-guide.md) | mcp | The authoring guide and skills teach the new shape and the revise loop | SPEC-004, SPEC-005 | 3h | complete |
| [SPEC-007](specs/SPEC-007-content-revise-learner-units.md) | content | Revise learner 1's untouched units | SPEC-006 | 3h | deferred |

**Total:** 24h. **Critical path:** SPEC-001 → SPEC-003 → SPEC-004 → SPEC-006 → SPEC-007 = 18h.

**Order.** SPEC-002 and SPEC-003 can run side by side after SPEC-001. SPEC-004 and SPEC-005 can run side by side after SPEC-003, because one owns `internal/units` and the other owns `internal/mcp`. SPEC-007 is the only spec that writes to the production course.

## Decisions

- **The code plans the progression. The AI writes it.** Choosing the next pattern, the pace, and what to recycle is counting, so it lives in the brief and is testable. The writer turns that plan into a story. This follows north-star principle 1.
- **One new pattern per unit, two old ones reused.** The next pattern is the first, in a fixed order, whose prerequisites have all been taught in this learner's earlier units and which has not been taught yet. The two most recent patterns are named for reuse in the story. RSCH-004 says studied language should come back in later meaning-focused work. The fixed order is a product choice. No trial ranks grammar orders (RSCH-004, Low).
- **Pace from accuracy and backlog.** New words per unit use the same accuracy bands as review (ADR-0023): 2 below 80% on the last 50 reviews, 5 from 80% to 90%, and the tier's count above 90%. A consolidation unit is asked for when learning words exceed 40 or accuracy is under 75%. Those two thresholds are starting values, not findings.
- **About three quarters meaning.** At most 40% of a unit's activities may be single-word drills, meaning a bare-word `shadow`, `listen_pick`, `recall`, or `produce`. Each lesson needs a story or dialogue of at least the brief's `minStoryTokens`: 12 Hebrew words for beginner, 30 for developing, and 50 for established, capped by `maxTextTokens`. RSCH-004 gives the three-quarters split as a planning rule, Low for the exact share, so 40% is a product choice.
- **Recycle the previous unit.** At least four of the previous two units' new words appear in this unit's stories, dialogues, sentence questions, or `order` steps. This is the validator's `brief.recycle_previous`.
- **Mistakes rotate.** A mistake pair that appeared in each of the last two units is skipped for one unit. A pair answered right three times since it was recorded is retired. The brief still caps fixMistakes at three.
- **Only untouched units are revised.** A unit with any `lesson_runs` row for that learner is started, and revision refuses it. That keeps every attempt pointing at the lesson it was made on. An untouched unit has no attempts, so a new revision loses nothing. The old revision stays in `unit_revisions` and can be republished.
- **Revise before you add.** `get_lesson_brief` for a new unit refuses when three or more untouched units exist. The agent revises the earliest one instead. The cap of three is a product choice.

## Threat model

- **No new trust boundary.** The new brief fields and the revision path run through the existing admin-only MCP tools. `get_context` progress fields use the same learner scoping as its cards.
- **Integrity.** Revision checks the learner's runs again inside the publish transaction. If a run started between the brief and the publish, it refuses. A revision brief is tied to one unit id and one learner, and a brief for another unit cannot publish it.
- **Data.** No learner text goes into a brief. Previous-unit stories are the course's own Hebrew. Mistake rows already exist and are not widened.
- **Reversibility.** A revision adds a unit revision and changes the published pointer. Republishing the earlier revision undoes it.
- **Output.** Screens are unchanged. Unit text stays templ-escaped.

## Depends on / unblocks

- Builds on the lesson brief (INIT-015/SPEC-019), the pattern catalog and `prereqs` (INIT-017), the review accuracy bands (ADR-0023), checked review (ADR-0024), sentence build and later-day results (ADR-0025), and the attempt log.
- Out of scope: new activity types, a new Today layout, FSRS weights, changing shared units, and rewriting units the learner has started.

## Done when

- A new brief for learner 1 names a next pattern, the patterns to reuse, the previous units' stories and words, a pace, and at most three mistakes that are not the same three as the last two units.
- `validate_unit` blocks a unit whose lessons have a three-word story or are mostly bare-word drills, and one that does not reuse the previous units' words.
- `get_context` shows which of learner 1's units are untouched, started, and finished.
- An agent can revise an untouched unit through MCP, and it is refused for a started one.
- Learner 1's untouched units are revised, each one passes the new rules, and a before-and-after table shows story length, drill share, and recycled words per unit. Deferred to the next agent (SPEC-007). The revise path is in place. The rewrite itself waits until this commit is what `/version` shows.
