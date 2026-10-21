# ADR-0009: Learner model and language program

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-09 |
| **Accepted** | 2026-10-09 by the operator, as written. The two open decisions (aud-1, aud-3) were answered "proposed". |
| **Authored by** | `INIT-006/SPEC-001` |
| **Initiative** | INIT-006 · language-program |
| **Research grounding** | `RSCH-001` (pointed Hebrew, no morphology-drill trial); `RSCH-003` §1.1 (spacing, recall direction, interleaving), §1.2 (first-language glosses), §1.3 (focused grammar work, feedback); `RSCH-004` §1.4, §1.6, §3 rules 1, 4, 5, 14; `RSCH-005` (no image on a task about the written form); `RSCH-006` §1.4 (no points or streaks); operator decisions 2026-10-09 |
| **Supersedes** | [ADR-0004](ADR-0004-learner-data-media-and-secrets.md) in part: §1 (the snapshot), the "what the operator is accepting" paragraph in §2, the disclosure sentence in §3, the snapshot rate-limit and matrix row in §4, the `learner_snapshot_read` audit row in §10, and the MCP wording in §11. [ADR-0001](ADR-0001-learner-review-model.md) in part: constraint 4 (changed, not removed), and the ladder in §3 and the schedule effects in §5 while `SRS_SCHEDULER=fsrs`. The full list, with line numbers, is Decision 9. |
| **Extends** | [ADR-0003](ADR-0003-course-and-lesson-plan-language.md): schema `1.1.0`, five blocks, a lesson `recipe`, rules N13 to N19, changes to N1, N2, and N7, and N6 applied to `verse-reading.newWords`. [ADR-0002](ADR-0002-accounts-server-progress-site-map.md) §10: the stage thresholds under FSRS. |
| **Superseded by** | — |

## Summary

The operator's AI needs to see where one learner is: which words are fragile, which grammar is weak, how far along the program they are. This ADR lets the MCP read a profile of **aggregates** for a learner whose AI-planning switch is on. It never lets the MCP read answer history, typed text, or timestamps. The learner can always see the exact object the AI would get. Review scheduling moves from the five-step ladder to FSRS-6 with four grades, and the old ladder stays as a fallback. Words get grammar tags (part of speech, root, binyan, tense, person, root class) from a shared lexicon, and form practice (conjugation) feeds a per-concept accuracy score. Untimed conjugation practice with feedback is allowed. Timed root and binyan speed drills stay out. The plan language gains five blocks and a `recipe` field, so lessons can take the shapes the operator's teacher uses: topic conversation, review worksheet, verse reading, verb workshop, letters and vowels, and standalone guides.

---

## Context

### Problem statement

ADR-0004 gave the AI a minimum snapshot: the words a learner has passed, a stage per direction, and lessons completed. The operator asked on 2026-10-09 for more. The AI should be told which words are weak, which are very weak, which grammar the learner gets wrong, and where the learner is on the journey, so it can build the next lesson from that. The operator also asked for a personal word library, Anki-style flashcards, lessons that bring known words back, and tracking of tense and verb type, with verb conjugation lessons. Four accepted decisions stand in the way, and each is changed here: the snapshot contract (ADR-0004 §1), the ladder (ADR-0001 §3), the ban on root and binyan drills (ADR-0001 constraint 4), and the six-block plan vocabulary (ADR-0003 §2).

### Current state

- `review_items` stores, per word and direction: `due`, `step` (0 to 4), `streak`, `lapses`, `last_result`, `last_at`, `sources` (`server/db/migrations/001-initial.js:42`–`:58`). The ladder is `[1, 3, 7, 14, 30]` days (`shared/player/review.js:8`). A `pass` climbs one rung, once per local day. A `miss` resets to rung 0 and adds a lapse (`:74`–`:114`).
- Stage is `new`, `learning` (step 0 to 2), or `longTerm` (step 3 or more) (`server/progress.js:229`–`:234`; ADR-0002 §10, line 340).
- `practice_events` stores word, direction, result, time, lesson slug, and activity. It has no grammar attribute (`001-initial.js:60`–`:73`).
- A word exists only inside lesson files, merged by `server/vocab-index.js`. There is no lexicon and no part of speech, root, binyan, or tense anywhere.
- The MCP registers eight tools and none reads progress (`mcp/tools.js`; the list is in ADR-0005, line 47). INIT-004 plans a `get_learner_snapshot` tool under ADR-0004 §1.
- ADR-0001 constraint 4 says: "No root/binyan speed drills added by this initiative" (`ADR-0001:64`, `:308`). `RSCH-001` found no published comparison showing that a script, sound, or morphology drill speeds acquisition for English-speaking adults learning Modern Hebrew. That is an absence claim, graded Low. It is not evidence that such drills fail.
- The operator's class notes for the verb lessons contain pointing errors in the conjugation tables. A Hebrew form cannot be trusted because an AI or a note wrote it.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | One named list of what the AI may read about a learner, and one closed list of what it may not. A test can check both. |
| R2 | The learner sees exactly what the AI would see. |
| R3 | Scheduling with four grades, per direction, migrated from the ladder without changing any learner's stage counts, with a fallback that needs no down migration. |
| R4 | Words carry grammar tags. Practice on forms is attributed to concepts through those tags. |
| R5 | No Hebrew form is scored until a person has checked it. |
| R6 | The plan language covers the teacher's lesson shapes without a second plan writer. |
| R7 | Every number is either cited to a report section or labeled "product choice". |
| R8 | Each carried constraint is marked kept, changed, or superseded. |

### Constraints

- Plan-only: this record. No application code, no edit to `worksheets/`.
- No secrets, no account names, no class-transcript text, and no learner data in this record. Samples use `example-learner`.
- MCP authentication stays `MCP_AUTH_TOKEN`. There is no anonymous MCP mode.
- No new npm dependency and no build step (ADR-0005 Option A).
- The operator decided on 2026-10-09: aggregates may leave and raw history may not; FSRS replaces the ladder; INIT-006 depends on INIT-004's plan core; the word library and flashcards ship first.

---

## Options considered

Each decision records its own options. The three comparisons that shaped the rest:

| Area | Chosen | Set aside |
|------|--------|-----------|
| What the AI reads | Aggregate profile (Decision 1) | Minimum snapshot (ADR-0004); full history with typed text |
| Scheduler | FSRS-6, default parameters, four grades (Decision 2) | Keep the ladder; SM-2 style ease factors |
| Grammar practice | Untimed form practice with feedback, scored only on verified forms (Decisions 4, 6, 7) | Keep the ban; timed speed drills |

---

## Decision

### Chosen options (summary)

- **Profile:** `get_learner_profile`, switch-gated, aggregates only. The learner sees the same object at `GET /api/me/profile`.
- **Scheduler:** FSRS-6 with its published default parameters, desired retention 0.90, grades 1 to 4. `SRS_SCHEDULER=ladder` restores the old behavior.
- **Weak words:** `weak` and `very_weak`, from lapses and last grade, cleared when a word reaches long-term stability.
- **Grammar concepts:** a closed registry of 32 keys. Only form practice updates them.
- **Journey stage:** five labels from the count of long-term words. A label, not a reward.
- **Lexicon:** shared content, written by an admin or the MCP, with a `verified` flag set only by a person. Learners never write it.
- **Plan language:** schema `1.1.0`, five new blocks, six recipes, rules N13 to N19.
- **Daily mix:** due cards, weak words, one concept drill, one reading at 95% known words or more.

### 1. What the AI may read

**Options considered:**

1. **Keep ADR-0004's minimum snapshot.** Rejected (operator 2026-10-09). It names passed words and a stage. It cannot say which words keep slipping or which grammar is weak, and that is what the operator wants the AI to act on.
2. **An aggregate profile.** Per-word stage, strength bucket, capped lapse count, and weak flag. Per-concept accuracy. A journey label. **Chosen.** Operator 2026-10-09.
3. **Everything: recent wrong answers, typed text, per-event times.** Rejected. A next-lesson draft does not need them. Weak flags and per-concept accuracy carry the same decision, and typed text can hold personal content.

Support for an adaptive profile at all: individualized tutors that model the learner beat ordinary group instruction and non-adaptive software in two pooled studies (`RSCH-004` §1.6, Moderate). `RSCH-004` §1.6 also found no trial in which the next lesson, chosen from a learner's known words, beat a fixed frequency-and-theme order (Low, an absence). The profile therefore lets the AI recycle weak words and aim practice at weak grammar. It does not license reordering unseen lessons (Decision 8).

**Chosen profile.** One JSON object. For `audience: mcp` it is returned only when the learner's switch is on. `word_id` is the ADR-0001 identity key (NFC pointed Hebrew).

| Field | Returned |
|-------|----------|
| `username` | The account username. Present on every profile that is returned. |
| `profile_version` | The integer `1`. |
| `journey.stage` | One of `script`, `foundations`, `building`, `expanding`, `independent` (Decision 5). |
| `journey.counts.he_en`, `journey.counts.en_he` | `{ new, learning, longTerm }`: how many words are in each stage in that direction. |
| `journey.lessons_completed` | Lesson slugs whose stored `completedRounds` is greater than zero. Present even when empty. |
| `journey.recipes_completed` | Map of recipe id to the number of completed plan lessons with that recipe. `{}` until plans exist. |
| `journey.tenses_practiced`, `journey.binyanim_practiced` | Registry suffixes (`past`, `paal`, …) with at least one form attempt. Present even when empty. |
| `journey.vowels_off_share` | Number from 0 to 1 in steps of 0.05: the share of the learner's lesson-progress records whose `vowelsOn` is false. `null` when there are no records. |
| `words_total` | How many words have at least one graded review in either direction. |
| `words[]` | At most 500 entries, in the order of Decision 3. |
| `words[].word_id` | Identity key. |
| `words[].he_en`, `words[].en_he` | `{ stage, strength }` for a direction with at least one graded review. Omitted otherwise. `stage` is `new` (graded, never passed), `learning`, or `longTerm`. `strength` is `fragile`, `shaky`, `steady`, or `solid` (Decision 3). |
| `words[].weak` | `weak` or `very_weak`. Omitted when neither. |
| `words[].lapses` | Integer 0 to 9, the larger of the two directions. 9 means nine or more. |
| `weak_words`, `very_weak_words` | Arrays of `word_id`, at most 50 each, same order. Present even when empty. |
| `grammar[]` | One entry for every key in the Decision 4 registry: `concept`, `status`, `attempts`. Plus `accuracy` and `recent` (steps of 0.05) when `attempts` is above 0. |

| Never returned |
|----------------|
| Display name, numeric user id, role, password hash, session token, auth provider fields, or any other account secret |
| `practice_events` rows and every field of one: result, time, lesson slug, activity, client event id, `kind`, `lemma`, the grade of one event |
| Review-row schedule and memory fields: `due`, `step`, `streak`, `last_result`, `last_at`, `last_grade`, `stability`, `difficulty`, `reps`, `fsrs_state`, `sources` |
| Any timestamp or date: of an answer, a lesson completion, a word being added, or a switch change. Any time-of-day or session-length figure. |
| Typed text, free text, answers given (right or wrong), wrong choices. Answers to a `personal-question` are never stored, so they cannot be sent. |
| English glosses, transliteration, or any other meaning text. The AI joins `word_id` to the lexicon through `get_lexicon_entry`. |
| Lesson-record fields other than the completed-slug list and the vowels-off share: `wordMarks`, `missedLast`, `activityState`, `completedSteps`, `stepIndex`, `roundStartedAt` |
| Exact accuracy, exact stability, exact retrievability |
| Library fields: `user_words` source, added date, and the learner's own note; the `suspended` flag on a review row |
| Any other learner's data. Any `audit_log` content. |

**Rounding.** Accuracy and share values are rounded to the nearest 0.05. Strength is a bucket. Lapses are capped at 9. These steps are a product choice made so that no field is a near-copy of a stored schedule value.

This pair of tables is the contract. A profile response that contains a name from the "Never returned" table, an ISO timestamp string, or a numeric user id fails the INIT-006 SPEC-007 test. The test copies the names from this table once.

**Switch, rate, audit.** The switch is ADR-0004 §3's `users.ai_planning`, default off. Rules from ADR-0004 §3, §4, and §10 apply to the new tools as follows.

| Rule | Decision |
|------|----------|
| Authentication | Every MCP tool refuses without `MCP_AUTH_TOKEN`. |
| Switch off | `get_learner_profile`, `check_text_coverage`, and `add_library_words` return `{ error: "planning_off" }` and no other field. An empty `words` array is only for a learner whose switch is on and who has no graded review. |
| Rate | 60 reads per clock hour for the MCP credential, **shared** by `get_learner_profile` and `check_text_coverage`. Over the limit: `{ error: "rate_limited" }` and no body. |
| Library write cap | `add_library_words` adds at most 50 words per learner per local day. Words must already exist as lexicon keys. |
| Audit | One `audit_log` row per call, including refusals. See the table at the end of this decision. |
| Who sets the switch | An admin, one account at a time. Unchanged. The MCP receives 403. |

**The learner sees the same object.** `GET /api/me/profile` returns the same object the MCP would receive, plus `ai_planning` (true or false), whatever the switch says. It writes no audit row and counts against no rate limit. The Progress screen shows it (INIT-006 SPEC-017). This is the mitigation for sending more than ADR-0004 allowed: nothing leaves that the learner cannot read first.

**Coverage tool.** `check_text_coverage({ username, text })` tokenizes Hebrew text (whitespace and punctuation, NFC), and returns `{ coverage, known_count, running_words, unknown }`. `text` is at most 4000 characters. `unknown` lists at most 100 distinct tokens. A token is known when one of these holds, and no other rule applies:

1. Its `exactKey`, or its unique vowel-stripped form (ADR-0001 A1), matches a word the learner has passed or has in the library.
2. Its key is a lexicon `form` entry whose `lemma_key` is a word the learner has passed or has in the library.

There is no prefix stripping. ADR-0001 A1 and ADR-0003 §1 say not to strip proclitics, and ADR-0001 A2 was rejected for false merges. A token with a prefix (for example a verb with a leading ו) therefore counts as unknown until the AI or an admin records it as a `form` of its lemma. That makes coverage a conservative figure. It under-reports, and it never over-reports. A prefix-aware match is a follow-up and would amend ADR-0001 A1. The tool supports `RSCH-004` §3 rule 1, which is Low: a reading counts as input when about 95% of its running words are treated as known. It reveals which of the AI's own words the learner lacks, so it is learner data and is gated like the profile.

**What the operator is accepting.** For a learner whose switch is on, a call to `get_learner_profile` puts that learner's username, every reviewed word's identity key with its stage, strength, and weak flag, a capped lapse count, accuracy per grammar concept, a journey label, counts, and completed lesson slugs into the MCP client's context. The client's model provider therefore receives them. The Hebrew forms of weak words are included, because they are the identity keys, and so the provider learns which words this learner is struggling with. Data already sent is not recalled when an admin later turns the switch off. Further reads stop at the next call. The operator accepts this handoff on 2026-10-09 for learners whose switch is on. A coverage check sends the AI's own text to this server and returns the unknown tokens from it.

**Disclosure sentence.** This replaces the sentence in ADR-0004 §3. It appears in the same two places (the sign-in page and the account menu), on the Progress panel, and in the admin's confirm step for the switch. It does not replace the sentence that admins can see all activity. That sentence is shown in Hebrew with the English in brackets (`shared/app-shell.js:9`, `shared/login.js:11`, `login/index.html:129`). The new sentence is shown the same way. The English below is the contract. INIT-006 SPEC-017 writes the Hebrew line, and the operator approves it before it ships.

> If an admin turns on AI planning for your account, a summary of your progress and your username are sent to the operator's AI provider so it can draft your next lesson. The summary covers the words you have practiced and how well you know them, the grammar you have practiced and how accurate you are, the lessons you have finished, and how often you read with vowel points off. It does not include your individual answers, anything you typed, or when you practiced.

**Role by route.** Roles are the signed-out browser, `user`, `admin`, and the `mcp` actor. Browser routes use the session. MCP tools use the token. "Own" means the signed-in account. A blank cell is a defect. `404` is the response when the caller must not learn whether a key exists.

| Route group | Representative route | Signed out | `user` | `admin` | `mcp` |
|-------------|----------------------|------------|--------|---------|-------|
| Profile read | MCP `get_learner_profile`; `GET /api/me/profile` | MCP: refuse (no token). Browser: 401 | Own, any switch state | Own. No browser route reads another learner's profile. The MCP route takes the token only. | Allow with token when that learner's switch is on. Off: `planning_off` |
| Coverage | MCP `check_text_coverage`; `POST /api/me/coverage` | Refuse / 401 | Own | Own | Allow with token when the switch is on |
| Library read and cards | `GET /api/me/library`, `GET /api/me/cards`, `GET /api/me/mix` | 401 | Own | Own | No tool |
| Library add | `POST /api/me/library`; MCP `add_library_words` | 401 | Own | Own | Allow with token when the switch is on; keys must exist; 50 per day |
| Library suspend | `PATCH /api/me/library/:key` | 401 | Own | Own | No tool |
| Lexicon read | `GET /api/lexicon`, `GET /api/lexicon/:key`; MCP `list_lexicon`, `get_lexicon_entry` | 401 | 200 | 200 | Allow with token |
| Grammar concepts | `GET /api/grammar/concepts`; MCP `list_grammar_concepts` | 401 | 200 | 200 | Allow with token |
| Lexicon write | `PUT /api/admin/lexicon`; MCP `upsert_lexicon_entries` | 401 | 403 | Allow | Allow with token. Rows are marked `source: mcp`. |
| Lexicon verify | `PATCH /api/admin/lexicon/:key/verified` | 401 | 403 | Allow | 403. No success tool. |
| AI-planning switch | `PATCH /api/admin/users/:id/ai-planning` | 401 | 403 | Allow | 403. No success tool. |

**Audit.** Same `audit_log` as ADR-0004 §10.

| Action | Actor | Row records |
|--------|-------|-------------|
| `learner_profile_read` | `mcp` | actor, learner username, time, the field names from this decision that were included, `word_count`, `concept_count`, `lesson_count`. No word ids. A `planning_off` or `rate_limited` refusal records the learner and `fields: none`. |
| `learner_coverage_read` | `mcp` | actor, learner username, time, `running_words`, `unknown_count`. No text and no tokens. Refusals record `fields: none`. |
| `library_add` | `mcp` | actor, learner username, time, count of words added. No words. |
| `lexicon_upsert` | `admin:<username>` or `mcp` | actor, time, count of entries. No text. |
| `lexicon_verify` | `admin:<username>` | actor, time, count of keys, new value |
| `ai_planning_switch` | `admin:<username>` | Unchanged from ADR-0004 §10. |

### 2. Scheduler

**Options considered:**

1. **Keep the ladder** (`[1, 3, 7, 14, 30]` days, two grades). It works and it is tested. It cannot express "hard" or "easy", and it gives the AI no strength signal beyond the step. Rejected for the flashcard screen; kept as the fallback.
2. **FSRS-6 with its published default parameters.** **Chosen.** Operator 2026-10-09, who asked for an "Anki built in". FSRS-6 is the version in Anki 25.07 and later and in the `ts-fsrs` package.
3. **SM-2 style ease factors.** Rejected. ADR-0001 (line 298) accepted that the ladder is coarser than SM-2-style ease factors, on the strength of equal gaps matching expanding gaps in `RSCH-003`. Nothing here argues for ease factors specifically, and FSRS meets the same need with a published reference.

**What the research does and does not say.** `RSCH-003` §1.1 grades spacing over massing High, and rates recall in both directions Moderate. It found equal gaps as good as expanding gaps (Moderate) and longer gaps better for delayed retention (Moderate). It did not compare FSRS with a fixed ladder, and neither did `RSCH-004`. The choice of FSRS is a product choice. The accuracy claims made for FSRS come from its maintainers' benchmarks, which this record did not read. The two pages read for this record are the maintainers' algorithm page and a third party's explanation of it, and both are listed in the References.

**Pinned version.** FSRS-6, 21 parameters, as published on the open-spaced-repetition wiki page "The Algorithm" (URL in References, retrieved 2026-10-09). Default parameters:

```text
[0.212, 1.2931, 2.3065, 8.2956, 6.4133, 0.8334, 3.0194, 0.001, 1.8722, 0.1666, 0.796, 1.4835, 0.0614, 0.2629, 1.6483, 0.6014, 1.8729, 0.5425, 0.0912, 0.0658, 0.1542]
```

- Forgetting curve: `R(t, S) = (1 + F · t / S) ^ (−w20)` with `F = 0.9 ^ (−1 / w20) − 1`, so `R(S, S) = 0.9`.
- Interval for desired retention `r`: `I = (S / F) · (r ^ (−1 / w20) − 1)`. When `r = 0.9`, `I = S`.
- Desired retention: **0.90**. Product choice, and FSRS's default.
- Same-day review: `S' = S · e^(w17 · (G − 3 + w18)) · S ^ (−w19)`.
- The remaining transitions (initial stability and difficulty, difficulty update, stability after recall, stability after a lapse) are the ones on that page and in the `ts-fsrs` reference. This record does not restate them, so that there is one copy to check. The golden fixtures in INIT-006 SPEC-004 come from an independent reference implementation run outside this repository, so no dependency is added. Each fixture file records the implementation's name, its version, and the page version it matched.
- Parameters are never trained on this learner's history. One learner's history is too short to optimize 21 parameters. Out of scope.

**Grades.** 1 Again, 2 Hard, 3 Good, 4 Easy. A flashcard sends a grade. A lesson activity keeps sending the ADR-0001 §5 signal, and the server maps it:

| Signal | Grade | Schedule effect |
|--------|-------|-----------------|
| `pass` | 3 (Good) | FSRS transition |
| `miss` | 1 (Again) | FSRS transition; one lapse added |
| `not_yet` | none | No schedule change; `last_result` set as ADR-0001 §5 says. No lapse. |

**Same-day rule.** ADR-0001 §3 stands in spirit: a `pass` from a lesson activity on a card already reviewed on the same local calendar day is recorded and does not change the schedule. An explicit flashcard grade on the same day uses the FSRS same-day stability formula above. A miss always counts.

**Interval rules.** Whole days, minimum 1, maximum 365. There are no sub-day learning steps, so an Again card is due the next day. In a session the flashcard screen shows an Again card once more before the session ends, and the server treats that second grade as a same-day review. The preview on the four buttons shows Hard ≤ Good ≤ Easy. Where rounding makes two equal, Good is at least Hard + 1 and Easy is at least Good + 1. That ordering is a product choice.

**Card state.** `fsrs_state` is `new` (never graded), `learning` (graded, no success yet), `review` (a success, and the last grade was not Again), or `relearning` (the last grade was Again after a success). It is informational. The math does not depend on it.

**Stage under FSRS.** The three stage strings stay, with the same meaning ADR-0002 §10 gave them and a new threshold:

| Stage | Rule |
|-------|------|
| `new` | The direction has never passed. A pass is a `practice_events` row with result `pass`, a review row with `last_result` of `pass`, or any grade of 2 or more. |
| `learning` | It has passed, and stability is below 14 days. |
| `longTerm` | It has passed, and stability is 14 days or more. |

**Migration.** The additive migration (INIT-006 SPEC-002) sets, for each existing review row that has never passed, `stability = NULL` and `fsrs_state = new` (no lapses) or `learning` (lapses above 0). The first FSRS review of such a row uses FSRS's initial stability for the grade given. For a row that has passed, `stability = [1, 3, 7, 14, 30][step]` days, `difficulty = 5 + 0.5 · min(lapses, 5)`, `reps = streak + lapses`, and `fsrs_state = review`. This formula puts step 3 and step 4 at 14 and 30 days, which are `longTerm`, and steps 0 to 2 at 1, 3, and 7 days, which are `learning`. The stage counts per direction are therefore equal before and after the migration. The ladder columns, `due`, and every other existing value are not rewritten. After the first FSRS review the counts may diverge, because FSRS decides stability. Only the migration moment is promised.

**Fallback.** `SRS_SCHEDULER` is `fsrs` or `ladder`, read once at process start. With `ladder`, the schedule is the ADR-0001 ladder as before, signals still write `grade`, and for strength buckets the stability is `[1, 3, 7, 14, 30][step]` days and retrievability is `R = 0.9 ^ (t / S)`, so `R = 0.9` on the due day. That curve is a product choice for ladder mode only. The default is `ladder` until INIT-006 SPEC-020 sets `fsrs` in each deployment.

**Caps.** ADR-0001 §4 stands: 30 due and 10 new per Review session, oldest due first, directions and source lessons interleaved. Suspended cards are never due.

### 3. Weak words and strength

**Options considered:** (1) flags from stored review fields (lapses, last grade, stage) — **chosen**; (2) flags from the last five events per word — rejected, a window query per word for little gain and a second definition of "recent"; (3) no flags, only a strength number — rejected, the operator asked for "words they are weak, words they suck" as lists.

**Strength** is a bucket of retrievability now, `R(t, S)` with `t` the days since the last review:

| Bucket | Retrievability |
|--------|----------------|
| `fragile` | below 0.50 |
| `shaky` | 0.50 to below 0.80 |
| `steady` | 0.80 to below 0.95 |
| `solid` | 0.95 or more |

A word that has not been reviewed for a long time falls in strength. It does not become weak by waiting. That is why strength and the weak flag are separate fields.

**Weak flags**, per direction, first matching row wins. A word takes the worse of its directions that have at least one graded review. `very_weak` is worse than `weak`.

| Row | Condition | Flag |
|-----|-----------|------|
| W1 | stage is `longTerm` and lapses is 4 or more | `weak` |
| W2 | stage is `longTerm` | none |
| W3 | lapses is 3 or more | `very_weak` |
| W4 | lapses is 2, or lapses is 1 and the last grade is Again | `weak` |
| W5 | `fsrs` only: difficulty is 8 or more and reps is 3 or more | `weak` |
| W6 | otherwise | none |

A word that was failed three times and has since reached 14-day stability stops being flagged. A word that keeps slipping after it is long-term (4 or more lapses) is flagged `weak`. All numbers here are a product choice. They live in one module, `server/learner/thresholds.js`. No source sets them.

**Order in the profile.** `very_weak`, then `weak`, then no flag. Inside each, `fragile` to `solid`, then lapses (high first), then the key in code-point order.

### 4. Grammar concepts and mastery

**Options considered:** (1) a closed registry of concept keys, updated by form practice only — **chosen**; (2) free-text tags written by the AI — rejected, the profile would not be comparable from one lesson to the next; (3) updating concepts from every flashcard grade — rejected, recalling the meaning of a verb says little about conjugating it, and it would blur the two signals.

**Registry.** 32 keys. The migration seeds exactly these.

| Family | Keys |
|--------|------|
| `binyan` | `paal` פָּעַל, `nifal` נִפְעַל, `piel` פִּעֵל, `pual` פֻּעַל, `hifil` הִפְעִיל, `hufal` הֻפְעַל, `hitpael` הִתְפַּעֵל |
| `tense` | `infinitive` שם פועל, `present` זמן הווה, `past` זמן עבר, `future` זמן עתיד, `imperative` ציווי |
| `root-class` | `regular` שלם (green), `lamed-he` ל״ה (yellow), `peh-yod` פ״י (red), `peh-nun` פ״נ (red), `ayin-vav` ע״ו (red), `irregular` חריג (red) |
| `cell` | `present.ms`, `present.fs`, `present.mp`, `present.fp`, `past.1s`, `past.2ms`, `past.2fs`, `past.3ms`, `past.3fs`, `past.1p`, `past.2mp`, `past.2fp`, `past.3p`, `infinitive.inf` |

Concept keys are written `family:key`, for example `binyan:paal` and `cell:past.2ms`. The traffic light is a label for the learner's guide: green regular, yellow `lamed-he`, red the rest. Future-tense and imperative cells are not in the registry. The tense keys are, so that a lemma can be tagged. A later change adds the cells with an insert and an ADR-0009 changelog row.

| Cell code | Meaning | Pronoun |
|-----------|---------|---------|
| `ms`, `fs`, `mp`, `fp` | masculine singular, feminine singular, masculine plural, feminine plural (present tense) | — |
| `1s` | first person singular | אני |
| `2ms`, `2fs` | second person masculine, feminine singular | אתה, את |
| `3ms`, `3fs` | third person masculine, feminine singular | הוא, היא |
| `1p` | first person plural | אנחנו |
| `2mp`, `2fp` | second person masculine, feminine plural | אתם, אתן |
| `3p` | third person plural, both genders | הם, הן |
| `inf` | infinitive | — |

**What updates a concept.** Only a `kind: "form"` signal with result `pass` or `miss`. A word-card grade does not. A form signal counts for the concept keys found on that form and on its lemma, once each: `tense` and `cell` from the form, `binyan` and `root-class` from the form or, when the form has none, the lemma. A fully tagged form therefore touches four concepts. An untagged form touches none. A `not_yet` touches none. This is an attribution rule. A miss on a past-tense second-person form counts against `past`, `past.2ms`, the binyan, and the root class together. The profile calls the result "accuracy on items tagged X", not "skill in X".

**Mastery.** Per learner and concept: `attempts`, `correct`, and a recent score.

- Recent score `e`: the first attempt sets `e` to 1 or 0. After that, `e' = e + 0.2 · (x − e)` with `x` of 1 for a pass and 0 for a miss. The 0.2 is a product choice.
- `accuracy` is `correct / attempts`. `recent` is `e`. Both are rounded to 0.05 in the profile.
- Status:

| Status | Rule |
|--------|------|
| `unseen` | 0 attempts |
| `emerging` | 1 to 4 attempts. No judgment. |
| `very_weak` | 5 or more attempts and `e` below 0.50 |
| `weak` | 5 or more attempts and `e` from 0.50 to below 0.70 |
| `ok` | 5 or more attempts and `e` from 0.70 to below 0.90 |
| `strong` | 5 or more attempts and `e` of 0.90 or more |

These thresholds are a product choice. They live in `server/learner/thresholds.js`. Mastery can be rebuilt from `practice_events` (form rows) when tags change. It is not rebuilt on every lexicon write.

**Evidence.** `RSCH-003` §1.3: focused instruction on language forms helps second-language learners (two meta-analyses, Moderate); corrective feedback works (Moderate), and which feedback type is best is unresolved (Low); whether explicit rules beat implicit exposure is unresolved (Low). `RSCH-003` §1.1: interleaving different patterns in one practice set helps delayed retention (Moderate). The weighting of attempts and every threshold above are product choices.

### 5. Journey stage

**Options considered:** (1) a label from the count of long-term words — **chosen**; (2) a proficiency level such as A1 to C2 — rejected, nothing here tests that, and the profile would claim more than it measures; (3) points or a streak — rejected, `RSCH-006` §1.4 keeps them out (Low, unchanged from `RSCH-003`).

`secure` is the number of words whose `he_en` direction is `longTerm`.

| Stage | Rule |
|-------|------|
| `script` | `secure` below 50 |
| `foundations` | 50 to 149 |
| `building` | 150 to 499 |
| `expanding` | 500 to 1499 |
| `independent` | 1500 or more |

The numbers are a product choice. No Hebrew frequency list was validated (`RSCH-004` §2), so no number here claims to match a published band. The label depends on a count only. An earlier proposal also let a completed letters-and-vowels unit end `script`. That clause is dropped because it would depend on a `recipe` tag that exists only in `1.1.0` plans, while a count works the same for legacy lessons. The label gates nothing. It tells the AI roughly how much English and how many vowel points to expect the learner to need, and the AI still reads the counts. See Open decisions.

### 6. The lexicon and the personal library

**Options considered:** (1) a shared lexicon table written only by an admin or the MCP, with learners' own words kept on their own rows — **chosen**; (2) learners write lexicon entries — rejected, one learner's gloss would be rendered to every other learner, which makes the lexicon a content-injection channel; (3) a file under the lessons folder — rejected, plans live in `hebrew.sqlite` (ADR-0005 Decision 5, "Where state lives"), and a file gives no all-or-nothing batch, no per-entry `verified` clear on edit, and no audit row per write.

**Entries.** A lexicon entry is a `lemma`, a `form`, or a `phrase`. A form names its lemma. The key is `exactKey` of the pointed word (ADR-0001 A1). Fields and limits:

| Field | Rule |
|-------|------|
| `key` | NFC, 1 to 60 characters, at least one Hebrew letter (U+05D0 to U+05EA). A key that contains a space is a `phrase`. |
| `lemma_key` | Forms only. Names a lemma already stored or in the same batch. |
| `gloss_en` | At most 120 characters |
| `translit` | At most 80 characters. A pronunciation hint, never a scored answer (ADR-0003 §2). |
| `pos` | `noun`, `verb`, `adjective`, `adverb`, `preposition`, `pronoun`, `conjunction`, `particle`, `numeral`, `other` |
| `root` | Two to four Hebrew letters joined by `.`, for example `ח.ש.ב` |
| `binyan`, `root_class`, `tense` | Registry keys from Decision 4 |
| `cell` | A cell code that belongs to the entry's `tense`: present takes `ms fs mp fp`; past takes `1s 2ms 2fs 3ms 3fs 1p 2mp 2fp 3p`; infinitive takes `inf` |
| `accept[]` | At most 8 accepted unpointed spellings, 1 to 60 characters each, none containing U+0591 to U+05C7. Used to score typed answers. |
| Any text field | No `<` or `>`. No control characters. |
| `verified` | `0` or `1`. Set to 1 only by an admin (`lexicon_verify`). |
| Batch | At most 200 entries per call. All or nothing. |

**Verified.** An unverified form is shown with "not yet checked" and is never scored. An edit to a verified form clears its flag. The MCP cannot set the flag. That is deliberate. The class notes that prompted this feature contain pointing errors, and an AI can make the same errors with more confidence.

**Seed.** At startup the server adds one `lemma` or `phrase` entry for every key in the published lessons' vocab (gloss from `meaning`, hint from `sound`, `source: seed`, `verified: 0`). It never overwrites an entry whose source is not `seed`. It does not guess part of speech, root, binyan, or tense. Those come from the AI or an admin.

**The personal library.** A learner's library is every word with a review row for that learner, plus every word in their `user_words` table. A word a learner adds enters as `new` in both directions, under the new-card cap. A learner can suspend a card. A word that has no lexicon entry keeps its meaning on the learner's own `user_words` row: `gloss_en` at most 100 characters, same character rule, visible to that learner only. When an admin or the MCP later adds a lexicon entry for the key, the lexicon gloss is the one shown, and the learner's note is shown as "your note". Learners never write the lexicon. `add_library_words` (MCP) accepts only keys that already exist in the lexicon, so the AI writes the entry first.

**Forms do not become cards.** Form practice creates no review row. Only words in the library are cards. Without that rule one verb with thirteen forms per tense would put 26 cards into the deck.

### 7. Plan language 1.1.0

**Options considered:** (1) five new blocks and a lesson `recipe`, additive to `1.0.0` — **chosen**; (2) express every shape with the six ADR-0003 blocks — rejected for `explainer` and `conjugation-table`, which have no learner action in the six and need lexicon references, and for `verse-reading`, which needs a source reference and a per-verse structure; (3) a second plan type for guides — rejected, ADR-0003 §1 chose one plan, unit, lesson, and block shape for shared and personal plans, and a second type would need its own writer, which ADR-0005 Decision 5 ("One implementation") rules out.

ADR-0003 §2 (lines 121–146) took its six block types from the `RSCH-004` lesson-type table and listed what is not a block. None of the five new blocks is in that table. Each is added here as a product choice taken from the operator's request and the teacher's formats, with the grade for each stated. One row of ADR-0003's "not block types" list is reversed. It said "Script and sound support" is fields on the introduction word (line 140). `letter-intro` is a separate block for the lesson whose subject is the letter itself, which an introduction (a word with an example sentence) cannot express. Fields on introduction words stay as they are.

| Block | Purpose | Learner action | Evidence |
|-------|---------|----------------|----------|
| `explainer` | A reference page: a rule, a term, a table, callouts. A guide is a lesson of these. | Reads. Taps an example to hear it when a clip exists. | Focused instruction on form helps (`RSCH-003` §1.3, Moderate). Explicit versus implicit is unresolved (Low). A reference page is a product choice. |
| `conjugation-table` | The forms of one verb in one tense, by cell. A sentence and two questions per verb. | In `study` mode: reads, hears. In `drill` mode: produces each form, one at a time, untimed, with feedback. | Retrieval beats rereading (`RSCH-003` §1.1, Moderate). Corrective feedback works (`RSCH-003` §1.3, Moderate); "try again once, then show" follows the prompts-first finding, whose advantage over recasts is Low. No published comparison shows a morphology drill speeds acquisition (`RSCH-001`, Low, an absence). |
| `personal-question` | One to six prompts about the learner's own life. | Says or types an answer. Never scored. Never sent to the server. | Output is a strand condition, graded Low (`RSCH-004` §1.4). The teacher asks one after almost every task. Product choice. |
| `verse-reading` | One verse or a short range: new words, the pointed text, simple questions, one personal question. | Reads, taps for a gloss, answers. | First-language glosses beat target-language glosses (`RSCH-003` §1.2, Moderate). No other claim. Product choice. |
| `letter-intro` | One letter: its sound with each vowel, and recognition in words. | Reads, hears, chooses which words contain the letter. Writing the letter is a paper prompt, not scored. | Script-first versus words-first is not settled for Hebrew (`RSCH-004` §1.5, Very Low). Product choice, taken from the teacher's letter sheets. |

**Fields.** Every list item has an `id` (ADR-0003 §1). Every string is plain text, and the ADR-0003 HTML rule (line 290, Block) applies to all of it. The block size ceiling is N9 and the plain-text field ceiling is N11. The limits below are tighter and are product choices.

- `explainer`: `title` (120); `sections[]` (at most 12), each with `heading` (120), `paragraphs[]` (at most 8, 1500 each), `examples[]` (at most 8, each `{ he, en, translit?, root? }`, limits 200, 300, 200, and a root in the pattern from Decision 6), `table?` (at most 8 columns and 12 rows, 200 per cell), and `callouts[]` (at most 6, each `{ tone: green | yellow | red | info, title (80), text (600) }`). A callout shows its tone in words or an icon as well as in color.
- `conjugation-table`: `lemmaKey`, `tense` (`present` or `past`), `mode` (`study` or `drill`), `cells[]` each `{ cell, formKey }`, `sentences[]` (at most 3, `{ he, en? }`), `questions[]` (at most 4, `{ he, en? }`).
- `personal-question`: `prompts[]` (at most 6, each `{ he, en? }`), `answer` (`speak`, `type`, or `either`). The block has no field that holds an answer.
- `verse-reading`: `ref` (`{ book, chapter, from, to, source }`, where `source` names a public-domain pointed edition), `newWords[]` (pointed keys), `verse` (pointed text), `questions[]` (at most 4, `{ he, en? }`), `personalQuestion` (`{ he, en? }`). No translation of the verse is stored.
- `letter-intro`: `letter`, `name`, `sounds[]` (each `{ vowel, pointed, sound }`), `findIn[]` (each `{ word, contains }`). The `findIn` words are recognition items. They are not `newWords`, create no review rows, and are exempt from N1 and N6. There is no picture field (see "Pictures" below).
- Introduction words gain an optional `group` (1 to 3), so the teacher's three groups of five can be written.

**Drill scoring.** In `drill` mode a typed answer is compared after stripping U+0591 to U+05C7 from both sides, against the form's `accept[]` and its stripped key. A wrong answer shows "try again" once, then shows the form. A learner may press "I was right". That counts as a pass. Each attempt sends one signal with `kind: "form"`, the form key as `word`, and the lemma. `activity` is `conjugation-drill` or `conjugation-override`, so an override can be told apart in the record. Only `verified` forms are drilled. There is no timer.

**Recipes.** `recipe` is a lesson field, optional, and only in `1.1.0` documents. Its values and the block order each recommends are below. The order is guidance for the author and the AI. The validator does not enforce it, except through the rules.

| Recipe | Recommended blocks | Teacher's shape it follows |
|--------|--------------------|----------------------------|
| `topic-conversation` | `spaced-review-checkpoint`, `introduction` (about 15 words in three groups, with the sentence frames as its one pattern), `short-dialogue` (role-play or a "this or that" choice), `personal-question` | Warm-up or review; three groups of five; sentence frames; personal questions; a speaking task |
| `review-worksheet` | `spaced-review-checkpoint`, `graded-story` (two readings to compare), `short-dialogue`, `personal-question` with `answer: type` for the short writing task | Exercises 1 to 7. Matching to a word bank is the checkpoint. "Pick the right sentence" has no block in this version. |
| `verse-reading` | one `verse-reading` per verse or short range | Verse by verse. Three to six new words per verse. |
| `verb-workshop` | `explainer` (the rule), `conjugation-table` in `study` mode for three or four verbs, one `conjugation-table` in `drill` mode, `personal-question` | Rule in English; three or four verbs; present and past tables; one sentence and two questions per verb |
| `letters-and-vowels` | `letter-intro` per letter, then a `spaced-review-checkpoint` of earlier letters | The letter and vowel sheets |
| `guide` | `explainer` blocks only | A standalone guide such as "Decoding Hebrew Verbs" |

A speaking task about a picture is not supported. ADR-0003 §5 limits images to introduction words, and that stays.

**Pictures.** `letter-intro` has no picture field. The teacher's sheets include "match the words with this sound to their pictures". Supporting it needs an amendment to ADR-0003 §5 and a check against `RSCH-005`. Follow-up.

**Schema.** ADR-0003 §3 (lines 163–176) lets a later minor version add optional fields by publishing a schema that names that version, and has a `1.0.0` reader reject fields it does not know. `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` therefore takes `schemaVersion` `1.0.0` or `1.1.0`. A `1.0.0` document cannot contain a new block, a `recipe`, or a `group`. The validator rejects it, and does not guess. A part operation that adds a `1.1.0` construct to a `1.0.0` plan raises `schemaVersion` to `1.1.0` in the same revision. That is the only field an operation changes that it was not given. A plan never returns to `1.0.0`. The four ADR-0003 sample plans stay `1.0.0` and validate unchanged.

**New rules.** Same registry and severities as ADR-0003 §6 (rules N1 to N12, lines 260–271). Cite by id. `server/plans/validate.js` implements them with N1 to N12.

| Id | Rule | Severity | Label |
|----|------|----------|-------|
| N13 | Each `formKey` in a `conjugation-table` names a stored form whose lemma is the table's `lemmaKey`, whose `tense` is the table's `tense`, and whose `cell` is the entry's cell. A cell appears at most once. | **Block** | Product choice. The check keeps a table honest to the lexicon. |
| N14 | A `conjugation-table` lists a form that is not `verified`. | **Warn** | Product choice. Decision 6. |
| N15 | A lesson with a non-empty `buildsOn` and a recipe other than `guide` starts with a `spaced-review-checkpoint`. | **Warn** | Product choice, taken from the teacher's practice. Spacing itself is High (`RSCH-003` §1.1). `RSCH-004` §3 rule 5. |
| N16 | A lesson with recipe `topic-conversation`, `verb-workshop`, or `verse-reading` contains a `personal-question`, or for `verse-reading` a `personalQuestion`. | **Inform** | Product choice. Output is a strand condition, Low (`RSCH-004` §1.4). |
| N17 | Three lessons in a row in one unit have the same recipe. | **Inform** | Product choice. `RSCH-003` §1.1 rates interleaving Moderate for practice sets. That finding is about practice items, not lesson formats, so no grade is claimed. |
| N18 | A `topic-conversation` lesson has more than 8 new words, or an introduction group of more than 5 words. | **Warn** for 9 to 15 words or a group over 5. **Block** above 15 words. | Product choice. The teacher's lesson has about 15. No source sets either number. See Open decisions. |
| N19 | A `verse-reading` carries a non-empty `ref.source`. | **Block** when it does not | Product choice. The reader must be able to find the text. |

**Changes to ADR-0003.**

- **N1** is unchanged for every recipe except `topic-conversation`. For that recipe the block threshold is 15 and the warn threshold is 8, as N18 states. N1's `migration.legacy` rule is unchanged.
- **N7** counts `introduction`, `explainer`, `conjugation-table`, and `letter-intro` as language-focused. A lesson whose recipe is `guide` or `letters-and-vowels` is left out of both sides of the count, because those lessons are deliberate study by definition. A `verb-workshop` is counted, and a unit made only of workshops will warn. That is intended.
- **N2** does not apply to a `guide`.
- **N3** (95% known) is written for `graded-story` and `listening` only, so it does not reach `verse-reading`. A verse is not coverage-checked. The block lists its new words, N1 counts them, and the learner can tap for a gloss. This is a product choice, and `RSCH-004` §3 rule 1 is not applied to it.
- **N6** (each new word at least twice in the introducing lesson) applies to `verse-reading.newWords`. A verse and its questions count as occurrences. It stays a Warn.
- The ADR-0003 rule that a new word is shown with vowel points (line 283, Warn) applies to `verse-reading.newWords` and `introduction` words. Whether the verse itself is pointed is not a check (ADR-0001 constraint 5: no forced vowel fading).

### 8. Daily mix

**Options considered:** (1) a read-time composition of the learner's own cards, weak words, one concept drill, and one reading — **chosen**; (2) a stored daily plan — rejected, it goes stale as soon as the learner reviews anything; (3) reordering unseen lessons by known words — rejected, `RSCH-004` §3 rule 14 (Low) allows a personal queue to insert a due review or skip a known word, and says it may not reorder unseen words off the spine. No trial found that reorder winning.

The mix is computed on request and never stored. Sections, in order:

1. `review`: due cards, not suspended, at most 30, with new cards (never passed, from the learner's own library) at most 10 (ADR-0001 §4). Oldest due first. Directions and source lessons interleaved.
2. `weak`: up to 8 words flagged `very_weak` then `weak`, not already in `review`.
3. `drill`: the concept with the lowest recent score among those with status `weak` or `very_weak`, that has at least one verified form whose lemma the learner has in their library. Up to 6 verified forms with that tag, interleaved across at least two lemmas when two qualify, never the same lemma twice in a row when avoidable. Omitted when no concept qualifies.
4. `reading`: up to 3 sentences from the published lessons' vocab (and, once INIT-004 plans exist, their published stories) whose coverage for this learner is 0.95 or more by the coverage rule in Decision 1. Omitted when none qualifies.

Every card and weak word in the mix comes from the learner's own review rows or library. A drill form belongs to a lemma in the library. A reading sentence is the one place an unknown word can appear, and at most 5% of its running words are unknown. Its shuffle is seeded by the user id and the local date, so two requests over the same data return the same order. Interleaving rests on `RSCH-003` §1.1 (Moderate).

### 9. Changes to earlier decisions and carried constraints

ADR-0001, ADR-0002, ADR-0003, and ADR-0004 are not rewritten. Their metadata or changelog point here. The sentences below are the wording this ADR puts in force. "Line" is the line in the file as of 2026-10-09.

**New wording for the MCP progress rule.** It replaces the blockquote in ADR-0004 §11, line 263.

> MCP reads only the profile in ADR-0009 §1, for learners whose switch is on. That profile is aggregates: word ids with stage, strength, weak flag and a capped lapse count; accuracy per grammar concept; a journey label; and lessons completed. The MCP still never reads answer history, typed text, timestamps, passwords, sessions, or admin-only views.

| Source | Line | Wording in force before this ADR | Wording this ADR puts in force |
|--------|------|----------------------------------|--------------------------------|
| ADR-0004 §1, options and "Chosen snapshot" | 67–75 | "(2) Minimum snapshot … Chosen." and "(3) Richer snapshot. Answer history, timestamps, wrong answers, typed text, accuracy, and trouble words. Rejected." | The chosen option is Decision 1 here. The rejection of answer history, timestamps, wrong answers, and typed text stands. It yields for accuracy and trouble words only as rounded per-concept accuracy, capped lapse counts, and weak flags. |
| ADR-0004 §1, field table and "Never returned" table | 77–95 | The snapshot fields and exclusions | The two tables in Decision 1 here. |
| ADR-0004 §2, "What the operator is accepting" | 105 | The username, passed-word keys, two stage strings, and completed slugs reach the provider | The paragraph "What the operator is accepting" in Decision 1 here. |
| ADR-0004 §3, disclosure sentence | 123 | "…a short summary of words you have passed and lessons you have finished is sent…" | The disclosure sentence in Decision 1 here. |
| ADR-0004 §4, "Snapshot rate limit" | 133 | "60 reads per clock hour for the MCP credential." for the snapshot | 60 reads per clock hour, shared by `get_learner_profile` and `check_text_coverage`. |
| ADR-0004 §4, matrix row "Snapshot read" | 143 | `get_learner_snapshot`: allow with token when the switch is on | The rows for profile, coverage, and library in Decision 1 here. `get_learner_snapshot` is not built. |
| ADR-0004 §10, audit row `learner_snapshot_read` | 254 | Actor, learner, time, field names, `word_count`, `lesson_count` | The rows `learner_profile_read` and `learner_coverage_read` in Decision 1 here. |
| ADR-0004 §11, the MCP progress rule | 263 | The blockquote beginning "MCP reads only the snapshot in item 1" | The blockquote above. |
| ADR-0004 §11 table rows for ADR-0002 R9 (line 55), the matrix note (265), §10 options (336), §10 MCP row (345), consequences (433), security (494), and INIT-002 REQ-016 | as listed | Each reads "the ADR-0004 §1 snapshot" | Each reads "the ADR-0009 §1 profile". Nothing else in those rows changes. |
| ADR-0001 constraint 4 | 64, 308 | "No root/binyan speed drills added by this initiative." | **Changed.** Untimed conjugation practice with feedback is allowed. It is retrieval with corrective feedback, which `RSCH-003` §1.1 and §1.3 grade Moderate. Timed root or binyan speed drills stay out. `RSCH-001` found no published comparison showing that a morphology drill speeds acquisition (Low, an absence), so the product keeps the cautious part of the rule. |
| ADR-0001 §3, ladder, miss rule, same-day rule | 183–199 | The five-step ladder | Superseded while `SRS_SCHEDULER=fsrs` (Decision 2). Kept when it is `ladder`. |
| ADR-0001 §5, schedule-effect column and the "not yet" row | 216–228 | "Advance one step", "reset to step 1" | While `fsrs`, the signal maps to a grade (Decision 2). The signals themselves, and every activity's mapping to `pass`, `miss`, or `not_yet`, are unchanged. |
| ADR-0001 §4, caps | 203–210 | 30 due, 10 new | Kept. |
| ADR-0002 §10, stage definition | 340 | "**learning** = step 0–2; **long-term** = step ≥ 3" | Under `fsrs`, `longTerm` is stability of 14 days or more (Decision 2). Under `ladder`, unchanged. |
| ADR-0003 §2, six block types, and the "not block types" row for script and sound support | 121–146, 140 | Six types from the `RSCH-004` table. Script and sound support is fields on the introduction word. | Five more block types, as product choices with stated grades. `letter-intro` reverses the script-and-sound row for a lesson whose subject is the letter (Decision 7). |
| ADR-0003 §3, `schemaVersion` | 163–176 (field at 170) | `1.0.0` only | `1.0.0` and `1.1.0` (Decision 7). |
| ADR-0003 N1 | 260 | At most 8 new words | Unchanged except for recipe `topic-conversation` (N18). |
| ADR-0003 N2, N3, N7 | 261, 262, 266 | As written | N2 does not apply to a `guide`. N3 is written for `graded-story` and `listening` and does not reach `verse-reading`. N7's list and exclusions as in Decision 7. |

**Carried constraints.**

| Source | Constraint | In force for this build |
|--------|------------|-------------------------|
| ADR-0001 A1 | `exactKey` identity, no prefix stripping | Kept. Lexicon keys, library keys, and `word_id` are that key. Coverage uses no prefix stripping (Decision 1). |
| ADR-0001 #3 | No edits to existing lesson `index.html` | Kept |
| ADR-0001 #4 | No root/binyan speed drills | **Changed**, above |
| ADR-0001 #5 | No forced vowel fading | Kept. The profile reports the share of lesson records with points off. A recipe never turns points off for the learner. |
| ADR-0001 #6 | Bilingual UI labels | Kept on new screens |
| ADR-0001 #7 | No names, transcripts, or secrets in lesson files | Kept, and extended to lexicon entries and plans |
| ADR-0002 | Server owns the review schedule | Kept. The browser module for FSRS is for previews only. |
| ADR-0002 | Admin full answer history is admin-only | Kept |
| ADR-0003 | One plan shape, six part operations, revisions, publish | Kept. Schema `1.1.0` is additive. |
| ADR-0004 | Switch default off, token, rate, audit | Kept and applied to the profile and coverage tools |
| ADR-0005 | Option A, no build step, no new dependency, one process | Kept |
| ADR-0006 | Tokens and interface rules | Kept on new screens |
| ADR-0007 | Route table, view contract, ready signal | Kept. Routes `words` and `guides` follow it. |
| `RSCH-006` | No points, streaks, or leaderboards | Kept. The journey label is not a reward. |

---

## Consequences

### Positive

- The AI can pick weak words and weak grammar to build the next lesson from, and a test checks the list of what it may and may not see.
- The learner can read the same object first.
- Review gets four grades and a per-card strength without changing anyone's stage counts at migration.
- An unchecked Hebrew form is never scored. Only an admin can mark one verified, and the check is audited.
- The teacher's lesson shapes have a name and a recommended block order.

### Negative / trade-offs

- For opted-in learners, the operator's model provider learns which Hebrew words this learner is weak on and how accurate they are by grammar concept. The provider's retention of that content is not known to this record. ADR-0004 §7 cites a 30-day abuse-monitoring window, but for one provider's image endpoint, not for the MCP client's provider.
- FSRS has no row in `RSCH-003` or `RSCH-004`. It is a product choice that rests on the operator's request and on the maintainers' claims, which this record did not read.
- Every threshold in Decisions 3, 4, and 5 is a product choice. A bad choice mislabels a learner. The thresholds are in one module so a change is one edit.
- Concept accuracy is attribution to tags, not a diagnosis. A miss counts against four tags at once.
- `1.1.0` plans cannot be read by a `1.0.0` reader. INIT-004's reader must accept both.
- Form data takes human time. Until the operator or the teacher ticks forms, the drill has nothing to ask.

### Follow-ups (out of scope here)

- Pictures on `letter-intro` (amend ADR-0003 §5 and check `RSCH-005`).
- Future-tense and imperative cells and tables.
- FSRS parameter training, if one learner's history ever becomes long enough.
- A "pick the right sentence" exercise for the review worksheet.
- An admin lexicon editor. Writes are HTTP and MCP only in INIT-006.

### Rollback

Delete this file. Revert the metadata and changelog rows added to ADR-0001, ADR-0002, ADR-0003, and ADR-0004, and the INIT-004 amendments listed below. No code lands in this spec.

---

## Implementation plan (downstream)

| Consumer | What it takes from this ADR |
|----------|----------------------------|
| INIT-006 SPEC-002 | Decision 2: columns, grade, backfill formula, stage rule |
| INIT-006 SPEC-003 | Decisions 4 and 6: registry keys, lexicon fields and limits, `user_words` with the learner's note |
| INIT-006 SPEC-004 | Decision 2: parameters, grade mapping, same-day rule, interval rules, fallback |
| INIT-006 SPEC-005 | Decision 6: validation, verified, seed |
| INIT-006 SPEC-006 | Decisions 3, 4, 5: weak flags, attribution, mastery, journey |
| INIT-006 SPEC-007 | Decision 1: the profile, the closed list, rate, audit, coverage |
| INIT-006 SPEC-008 | Decision 8 |
| INIT-006 SPEC-009, SPEC-010 | Decision 1 matrix and the capability table in the initiative §12 |
| INIT-006 SPEC-011 | Decision 7: schema, rules N13 to N19, changes to N1, N2, N3, N7 |
| INIT-006 SPEC-012 to SPEC-017 | The screens named in each Decision |
| INIT-006 SPEC-018 | Decisions 6 and 7: starter content |
| INIT-006 SPEC-019 | The recipe table and the tagging rules |
| INIT-004 | SPEC-010 loses the snapshot, SPEC-011 loses the switch route, SPEC-012 loses `get_learner_snapshot`, and SPEC-014 and SPEC-019 lose the disclosure and guide sentences. The new disclosure sentence and the guide sentence are written by INIT-006 SPEC-017 and SPEC-019 |

**Amendments made to INIT-004 by this spec.** INIT-004 is not approved, so editing is allowed. `get_learner_snapshot` and the snapshot service are removed from INIT-004 and replaced by INIT-006 SPEC-007 and SPEC-010. The switch route moves to INIT-006 SPEC-010. The `users.ai_planning` column and its repository functions stay in INIT-004 SPEC-006. INIT-006 depends on that spec. The files edited are listed in the INIT-006 SPEC-001 completion summary.

---

## Security considerations

### Authentication and authorization

The role-by-route table in Decision 1 is the authorization contract for the new routes. Profile and coverage reads by the MCP require the token and the learner's switch. Verification and the switch are admin-only. Lexicon writes are admin or token-authenticated MCP. Learner routes are scoped to the signed-in account and take no user id parameter.

### Data protection

The profile is the only new learner data that leaves the server, and only when the switch is on. The "Never returned" list is closed. Rows, text, timestamps, and names other than the username stay on the server. Learners never write shared content: their own words live on their own rows. All lexicon and plan text is validated for size and markup on the server and escaped again on render. Answers to `personal-question` blocks are never stored or sent.

### Residual risks (accepted)

1. **Provider receipt of the profile.** The operator accepted it on 2026-10-09 for learners whose switch is on. The provider learns the Hebrew keys of weak words and accuracy by grammar concept.
2. **Provider retention.** What the MCP client's model provider keeps, and for how long, depends on that provider's terms and the operator's account. This record did not read them. ADR-0004 §7's 30-day figure is for an image endpoint and is not evidence for this case. Before the first switch is turned on, the operator checks the provider's data-retention terms for the client they use.
3. **Attribution error.** One miss counts against four tags. The profile labels the number as accuracy on tagged items.
4. **Threshold error.** The weak flags, strength buckets, concept statuses, and journey labels are product choices.
5. **Wrong Hebrew in unverified forms.** They are visible and marked "not yet checked", and they are never scored.
6. **Single-admin verification.** The same person writes and verifies forms. The verification is audited and closed to the MCP.

---

## Open decisions (answered at acceptance, 2026-10-09)

| Id | Item | Value accepted | Why it was open |
|----|------|----------------------|----------------|
| aud-1 | New-word limit for recipe `topic-conversation` | Warn from 9, block above 15, groups of at most 5 (N18) | The teacher's lessons have about 15. No source sets any number (`RSCH-004` §3 rule 7). |
| aud-3 | Journey stage thresholds | `secure` below 50, 50 to 149, 150 to 499, 500 to 1499, 1500 or more (Decision 5) | No Hebrew frequency list was validated (`RSCH-004` §2). |

The operator accepted both proposals as written on 2026-10-09. Changing either number later is an amendment with a changelog row here.

---

## References

- `sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md` — pointed Hebrew; no morphology-drill trial.
- `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md` — §1.1, §1.2, §1.3.
- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` — §1.4, §1.5, §1.6, §2, §3 rules 1, 4, 5, 7, 14.
- `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` — pictures.
- `sdd/research/RSCH-006-learner-interface-design/RSCH-006-learner-interface-design-report.md` — §1.4.
- [ADR-0001: Learner review model](ADR-0001-learner-review-model.md)
- [ADR-0002: Accounts, server-side progress, and site map](ADR-0002-accounts-server-progress-site-map.md)
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md)
- [ADR-0004: Learner data, media, and secrets](ADR-0004-learner-data-media-and-secrets.md)
- open-spaced-repetition, "The Algorithm" (FSRS-6 parameters and formulas): <https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm>. Retrieved 2026-10-09. The page's earlier home at `fsrs4anki/wiki/The-Algorithm` redirects there.
- Expertium, "A technical explanation of FSRS" (states FSRS-6 is in Anki since 25.07): <https://expertium.github.io/Algorithm.html>. Retrieved 2026-10-09.
- open-spaced-repetition, `ts-fsrs` README (FSRS v6 badge): <https://github.com/open-spaced-repetition/ts-fsrs>. Retrieved 2026-10-09.
- `sdd/initiatives/INIT-006-language-program/INIT-006-language-program-initiative.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-09 | Initial Proposed record — `INIT-006/SPEC-001`. Open decisions aud-1 and aud-3 carry proposed values. |
| 2026-10-09 | Accepted by the operator as written. aud-1 (topic-conversation limit: warn from 9, block above 15, groups of at most 5) and aud-3 (journey thresholds 50, 150, 500, 1500) confirmed as proposed. The earlier ADRs' "once Accepted" notes are now in force. Still open from Residual risk 2: the operator checks the model provider's retention terms before the first learner's switch is turned on. |
