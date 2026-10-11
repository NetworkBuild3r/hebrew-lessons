# ADR-0003: Course and lesson-plan language

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Proposed |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-003/SPEC-004` |
| **Initiative** | INIT-003 · lesson-system-foundations |
| **Research grounding** | `RSCH-004` (course shape, lesson types, design rules); `RSCH-005` (pictures, typing, spelling) |
| **Supersedes** | [ADR-0001](ADR-0001-learner-review-model.md) constraint 2 and [ADR-0002](ADR-0002-accounts-server-progress-site-map.md)'s carried form of that constraint, for the **plan document only**, at the API cutover described in Decision 3. Also changes ADR-0002's "hidden is the draft" rule, as described in Decision 7. |
| **Superseded by** | — Extended by [ADR-0009](ADR-0009-learner-model-and-language-program.md), Accepted 2026-10-09 (schema `1.1.0`, five blocks, a lesson `recipe`, rules N13–N19, and changes to N1, N2, N7 and to the "script and sound support" row). |

## Summary

A course is one JSON plan: an ordered set of units, each an ordered set of lessons, each an ordered set of blocks. The same shape is a shared plan or a personal plan. Lessons declare the words they introduce, the lessons they build on, and the earlier words they recycle. Six block types carry the research lesson types. Images are media ids with alt text. Recorded audio paths stay as they are. Part-level edits share one operation list between the MCP and the builder, guarded by a revision. Publish rules live in one numbered registry so later specs cite them once.

---

## Context

### Problem statement

A lesson today is one `window.WORKSHEET` object. `upsert_lesson` replaces it whole. Parts have no stable ids, lessons do not point at each other, and the player offers eleven activity types that do not match the lesson types in `RSCH-004`. The operator wants a human or an AI to author one plan or many, edit one part, attach a checked picture, and have lessons build on each other. The player, the builder, the MCP, the validator, and the personal-plan service are all projections of that language, so it is fixed here before any of them are built.

### Current state

- Three lessons on disk: `worksheets/my-morning/worksheet.js`, `worksheets/anal-sex-vocab/worksheet.js`, `worksheets/shai-date-indirect-speech/worksheet.js`. Each assigns `window.WORKSHEET`.
- Activity types in use: `words`, `match`, `listen-pick`, `story`, `speed-quiz`, `build-sentence`, `pattern`, `ask-friend`. The authoring schema also names `fill`, `retell`, and `type-it`.
- Word identity, the spacing ladder, and gloss lookup are ADR-0001. Accounts, visibility, and server-side progress are ADR-0002.
- Lesson and vocab data still load by script tag. ADR-0002 allows `fetch` only for `/api/` JSON.
- Clip paths under `worksheets/` are stable. INIT-002 forbids changing them.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | One plan shape for a shared course and a personal plan (aud-1, operator 2026-10-08). |
| R2 | Entities, ids, and what a lesson declares: new words, lessons it builds on, words it recycles. |
| R3 | Block types taken from `RSCH-004`, with pictures limited as `RSCH-005` requires. Every existing activity type mapped or retired. |
| R4 | A published serialization, a version, and a migration rule. The script-tag constraint is kept or superseded by name. |
| R5 | Part-level operations with the same list for MCP and builder, addressed by id, with a revision conflict rule. |
| R6 | Images by media id, alt text required. Audio paths unchanged. |
| R7 | Publish rules from the research, each labeled, each block, warn, or inform. |
| R8 | Draft and publish, including how a learner comes to see a shared or personal plan. |
| R9 | A field-by-field map of `window.WORKSHEET`, and samples of the three existing lessons plus one new three-lesson unit. |
| R10 | Carried constraints from ADR-0001, ADR-0002, and INIT-002 REQ-014 each marked kept, changed, or superseded. |
| R11 | Plan text is untrusted. No HTML in plan fields. Size limits for a plan, a lesson, and a block. |

### Constraints

- Plan-only: this record, the schema, and the samples. No application code, no edit to `worksheets/` or lesson `index.html`.
- No secrets, no account names, and no class-transcript text in this record.
- Word identity stays the ADR-0001 key, so review rows already stored still apply.
- Learner answers and progress are not part of a plan.
- Admins author in the builder. The operator's AI authors through the MCP. There is no instructor role (aud-3). The server does not call a text model to draft lessons (aud-4). Image generation is ADR-0004.
- Upload checks, image-byte review, and what an AI may read about a learner are ADR-0004. The stack is ADR-0005. The look is ADR-0006.

---

## Options considered

The choice for each area is recorded in that decision. The alternatives that were set aside:

| Area | Chosen | Set aside |
|------|--------|-----------|
| 1. Entities | One plan document; personal differs by audience and assignee | A second schema for personal plans; a flat list of lessons with no unit |
| 2. Blocks | The six `RSCH-004` types | Keeping the eleven activity types; adding a spelling ladder or a keyboard course |
| 3. Serialization | JSON, schemaVersion `1.0.0`, JSON Schema draft-07; script-tag load superseded at the API cutover | Script-tag as the only runtime; YAML or unmarked prose |
| 4. Operations | Six part operations, one revision, same list on MCP and builder | Whole-document replace as the edit path; automatic merge |
| 5. Media | Image by media id plus alt; audio stays a relative path | URLs in the plan; renumbering audio into media ids |
| 6. Validation | One registry, mixed block / warn / inform | Blocking on every figure, including legacy lessons that predate it |
| 7. Draft | `status` draft or published, plus ADR-0002 visibility | Using `hidden` as the only draft, which cannot hold a working copy beside a published snapshot |
| 8. Migration | Copy wording; keep paths; soften the new-word cap on legacy plans | Cutting existing lessons down to the cap; leaving `window.WORKSHEET` as the source of truth |
| 9. Carried constraints | Supersede script-tag load for the plan document; keep the rest | Keeping script-tag load as the only way a plan can reach the browser |
| 10. Examples | The three existing lessons, plus one new unit that declares recycling | Examples that invent a second course around the old lessons |

---

## Decision

### Chosen options (summary)

- **Plan:** one JSON document for shared and personal. Personal sets `audience` to `personal` and `assigneeId` to an opaque account id.
- **Blocks:** `introduction`, `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, `spaced-review-checkpoint`.
- **File:** `schemaVersion` `1.0.0`. Schema: `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (JSON Schema draft-07). Unknown properties fail.
- **Edits:** `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`. Both MCP and builder. Mismatched revision rejects the write.
- **Pictures:** `{ mediaId, alt }` on an introduction word only. **Audio:** the existing relative path, unchanged.
- **Rules:** the registry in Decision 6. Cite it by id (N1, N3, …). Do not restate the figures elsewhere.
- **Samples:** `docs/decisions/ADR-0003-assets/samples/`.

### 1. Entities

**Options considered:** (1) one plan, units, lessons, blocks, shared and personal — **chosen**; (2) a different schema for a personal plan — rejected, aud-1 and und-4 say the difference is who it is for, not its shape; (3) lessons with no unit — rejected, "build on each other" needs an ordered container.

A **plan** is the document. A **unit** is an ordered theme inside it. A **lesson** is an ordered list of blocks. A personal plan is a plan with `audience: "personal"` and an `assigneeId`. A shared plan has `audience: "shared"` and `assigneeId: null`.

| Entity | Stable id | Rule |
|--------|-----------|------|
| Plan | `id` slug | Immutable. Unique in the catalog. |
| Unit | `id` slug | Unique inside the plan. Immutable. Order is array order. |
| Lesson | `id` slug | Unique in the catalog, not only inside the plan. Migrated lessons keep the worksheet folder slug. Immutable. `buildsOn` may name a lesson in another plan. |
| Block | `id` | Unique inside the lesson. Immutable. Moving a block changes order and does not change the id. |
| Word, paragraph, line, question, cloze item, pattern item, span | `id` | Unique inside its parent list. The old numeric worksheet ids are kept as decimal strings. |

The review identity of a word is **not** that local `id`. It is ADR-0001 `exactKey` of the pointed `word` string: NFC, whitespace collapsed, trimmed. Do not strip proclitic prefixes. `newWords`, `recycles[].word`, and checkpoint `wordKeys` store that pointed string. A story token that differs by a prefix does not automatically join the vocab row.

Every lesson carries these three declarations, even when the arrays are empty:

| Field | Meaning |
|-------|---------|
| `newWords` | Pointed forms introduced here. The cap is N1. |
| `buildsOn` | Lesson ids whose words this lesson already treats as known. |
| `recycles` | `{ word, fromLessonId }` for earlier words brought back. The minimum is N2. |

Plans hold content and structure only. They do not hold answers, schedules, or progress.

`frequencyBand` on a unit is optional. Nothing in this version checks it. See Decision 6, rule 9.

### 2. Block vocabulary

**Options considered:** (1) the six types below — **chosen**, from the `RSCH-004` lesson-type table; (2) the eleven existing activity types as the vocabulary — rejected, they are player drills over one worksheet, not the course types; (3) a copy-then-recall spelling ladder and a keyboard course — rejected, `RSCH-005` does not support them as beginner activities.

`introduction` is the meet-the-words block. That is its only type name in the schema.

| Type | Purpose | Learner action | Evidence |
|------|---------|----------------|----------|
| `introduction` | First meeting of the new words and at most one pattern, each word with a first-language meaning. Optional picture on a word, here only. | Reads the pointed word, the meaning, and the example sentence. Hears the clip when `audioPath` is set. | Adapted from the language-focused strand (`RSCH-004` §4). Glosses stay at the `RSCH-003` grade already cited there. The picture limit is `RSCH-005` rules 1–3 (Moderate for the meaning line and for hiding the picture during form recall; Low / adapted for "first meeting"). |
| `graded-story` | Meaning-focused reading on the theme. | Reads the paragraphs. May open a gloss. Optional questions are skippable. | Coverage rule is Low, as graded in `RSCH-003` and restated in `RSCH-004` §4. |
| `listening` | A short passage on the same theme, mostly known words. | Listens, or reads the passage when no clip exists yet. | Low, and only for a short informal narrative (`RSCH-004` §4, study cited there as [7]). |
| `short-dialogue` | Meaning-focused output with mostly known language. | Produces the line, or rebuilds story sentences when `source` is `story`. | Low. Output is a strand condition. Task-based program effects stay Low (`RSCH-004` §4). |
| `fluency-pass` | Faster use of language already in the lesson or in `recycles`. The block has no word list of its own. | Speaks or answers against a clock (`seconds`) and optional cues. | Adapted. Empty new-item list is Nation's definition (`RSCH-004` §3 rule 3, §4). |
| `spaced-review-checkpoint` | Recall of earlier words on a later session. | Produces the word from the meaning, with the spelling not left on screen as the method of study. | Evidence-backed. High via `RSCH-003` spacing, not re-argued (`RSCH-004` §4). |

These are not block types:

| Candidate | Decision | Reason |
|-----------|----------|--------|
| Script and sound support | Fields on the introduction word (`word` pointed, `sound`, `translit`) | `RSCH-004` places it inside the introduction. Product choice. `translit` and `sound` are pronunciation hints. They are never the scored answer. |
| Literal comprehension check | Optional `questions` on `graded-story` or `listening` | Product choice (`RSCH-004` §4). Not a recall event. |
| Personal review | Not in the plan | It reads the learner's review record. A plan does not. The checkpoint is the authored recall session. The due queue stays the ADR-0001 scheduler beside the spine (`RSCH-004` §3 rule 14). |
| Copy-then-recall spelling ladder | Deferred. Not in schemaVersion `1.0.0` | `RSCH-005`: no such protocol was tested. Copying a word while it stays visible scored worse than not writing. |
| Keyboard / touch-typing practice | Deferred. Not in schemaVersion `1.0.0` | `RSCH-005`: adults can learn key positions on a rearranged familiar alphabet. No vocabulary gain was found. |
| Picture-only cue, or a picture left up while the learner types the spelling | Out | `RSCH-005` activity list: a picture does not replace the first-language meaning, and picture glosses gave the worst written-form recall. |

#### Existing activity types

| Activity type | Lands as | Reason |
|---------------|----------|--------|
| `words` | `introduction` | First meeting of the vocab rows. |
| `story` | `graded-story` | The reading. |
| `pattern` | `introduction.pattern` | One pattern, on the introduction. Not a second block. `rules[]` are lines of that one pattern. |
| `retell` | `fluency-pass` with `source: "story"` | Timed recall of known lines. `reading.retell` is this block even when `retell` is absent from `activities`. |
| `ask-friend` | `short-dialogue` lines, `speaker: "prompt"` | The learner produces a reply. |
| `fill` | `introduction.cloze` | Deliberate practice of the new words. Not its own block. The sentences are kept. |
| `speed-quiz` | `fluency-pass` with `source: "vocab"` | The `seconds` value is kept. Same-lesson placement next to new words is a legacy warning under N12. |
| `build-sentence` | `short-dialogue` with `source: "story"` and `count` | No lines of its own; the sentences are the story. |
| `match` | Retired as a block | No authored payload. Pairing a word with its meaning is a player behavior of the introduction, and it is not a `RSCH-004` lesson type. |
| `listen-pick` | Retired as a block | The bare activity is a drill over vocab audio. It is not the short listening passage. Word clips stay on the word. A real passage is a `listening` block, which these three lessons do not contain. |
| `type-it` | Retired | Not used in the three lessons. `RSCH-005` does not support copying-while-visible, a spelling ladder, or keyboard practice as the way a beginner learns the word. |

### 3. Serialization

**Options considered:** (1) JSON, `schemaVersion` `1.0.0`, JSON Schema **draft-07**, strict properties — **chosen**; (2) keep `<script>` assignment of `window.WORKSHEET` as the only form — rejected, a static script cannot carry a revision or a part edit; (3) a prose or YAML plan with no schema — rejected, an AI patch needs a validator that fails closed on a typo.

| Rule | Value |
|------|-------|
| Document | One JSON object, UTF-8 |
| Version field | `schemaVersion` = `1.0.0` |
| Schema file | `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` |
| Schema dialect | JSON Schema draft-07 (`http://json-schema.org/draft-07/schema#`) |
| Unknown properties | Fail validation |
| Unknown `schemaVersion` | Reject the document. Do not best-effort parse it. |
| Compatible change | A later minor version may add optional fields only by publishing a schema that names that version. A `1.0.0` reader rejects fields it does not know. |
| Breaking change | A new major version ships a named transform. The new reader loads only documents that transform has already rewritten. |

**Script-tag constraint — superseded for the plan, at the API cutover.** ADR-0001 constraint 2, carried by ADR-0002 and by INIT-002 REQ-014, says lesson and vocab data load by script tag and do not use `fetch`. That stays true for the legacy `window.WORKSHEET` files until those files are retired. It does not stay true for this plan.

Reason (aud-2, operator 2026-10-08, plan default): part edits, the revision check, draft versus published, and a personal assignee cannot be a script the browser loads with no conflict token. The operator allowed a ground-up load path.

Until ADR-0005's stack serves plans from the authenticated API:

- Existing worksheet scripts and their `index.html` files stay as they are.
- A publisher may emit `window.LESSON_PLAN = <json>` as a build artifact of the **published snapshot**. That file is replaced whole. It is not the source of truth and it is not what part operations edit.

After that cutover:

- A signed-in client loads the published snapshot with `fetch` of an `/api/` route. ADR-0002 already allows `fetch` for `/api/` JSON. This decision extends that allowance to the plan document and to no other lesson URL.
- Script-tag loading of plan JSON stops.
- Clients do not `fetch` arbitrary lesson URLs, and the plan does not contain those URLs.

**Mechanized check (SPEC-009):** `node spikes/skeleton/validate.js`

That command reads `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` and every `*.plan.json` under `docs/decisions/ADR-0003-assets/samples/`. Every sample exits clean. Each deliberately broken plan exits non-zero for the reason SPEC-009 names. This spec does not add that script and does not add an npm dependency.

### 4. Part-level operations

**Options considered:** (1) the six operations below, on both the MCP and the builder, one plan revision — **chosen**; (2) keep `upsert_lesson` whole-document replace as the way authors edit — rejected, that is the limit this language exists to remove; (3) automatic merge of concurrent edits — rejected, two authors can rewrite the same sentence and a merge would publish a blend nobody checked.

`upsert_lesson` is not an editing operation for this language. A later bulk import, if one is added, is added to both columns of this table in the same change.

Address by id path, not by array index:

```text
units/{unitId}
units/{unitId}/lessons/{lessonId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/words/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/paragraphs/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/lines/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/questions/{id}
```

`set_field` names one field under that path (`title`, `meaning`, `alt`, `newWords`, …).

The revision is an integer on the plan, starting at 1. Every successful mutation increments it by 1. The caller sends `expectedRevision`. If it does not equal the stored working-copy revision, the write is rejected, the document is unchanged, and the caller reads again. There is no merge. The transport status code is ADR-0005's; the outcome is fixed here.

| Operation | Id path | On conflict | MCP | Builder |
|-----------|---------|-------------|-----|---------|
| Read a part | Any path above | None (read) | `read_part` | The form loads that part |
| Replace a part | Any path above | `expectedRevision` mismatch rejects the write | `replace_part` | Save replaces that part |
| Insert a block | Lesson path, plus index and the new block | Same | `insert_block` | Insert in the lesson outline |
| Move a block | Block path, plus the new index | Same | `move_block` | Move up, move down, or drag |
| Remove a block | Block path | Same | `remove_block` | Remove on that block |
| Set a field | Path plus field name and value | Same | `set_field` | The field control saves that field |

No operation in this table exists on only one side. Both sides send the same path and the same `expectedRevision`, and both receive the same reject-and-refetch outcome.

### 5. Media references

**Options considered:** (1) images by media id with required alt; audio paths left unchanged — **chosen**; (2) `http` or `https` URLs inside plan text — rejected, the plan would be able to pull a remote resource into every learner's page; (3) move existing clips onto media ids — rejected, INIT-002 forbids path and URL changes.

An image is allowed only on an introduction word:

```json
{ "mediaId": "coffee-cup", "alt": "coffee" }
```

| Rule | Decision |
|------|----------|
| `mediaId` | Opaque slug, `[a-z0-9][a-z0-9-]{0,63}`. Not a URL, path, or filename. |
| `alt` | Required. Length is N8. States the word's meaning in the gloss language so the meaning survives if the image fails. |
| Meaning line | `meaning` is required on every word whether or not an image is present. The picture does not replace it (`RSCH-005` rule 1). |
| Where | Introduction words only. Story, listening, dialogue, fluency, and checkpoint blocks have no image field. |
| Form recall | No block in this version asks the learner to type the spelling. A later block that does must not carry an image (`RSCH-005` rule 3). |
| Who may see the bytes | Only while rendering a lesson that learner is already allowed to open. No standalone public media URL. A personal plan's images follow that plan's assignee. A shared plan's images follow the lesson's visibility. Two lessons may share one `mediaId`; the learner still only receives the bytes inside a lesson they can open. |
| Approval | The plan stores an id. It does not store bytes, a generator prompt, or a review checklist. Human approval before a learner sees the picture is ADR-0004. |

Audio is a different reference. `audioPath`, `sentenceAudioPath`, and the same field on a paragraph, question, pattern item, or line are relative `.mp3` paths. They must not be absolute, scheme-bearing, or protocol-relative. Migrated values are copied character for character. New clips, when they exist, use that same relative convention. Archive-before-overwrite and the inventory gate stay as ADR-0002 stated them.

### 6. Validation rules

**Options considered:** (1) one registry, severities mixed by evidence — **chosen**; (2) block publication on every figure in the research, including the ones marked product choice or Low — rejected, `RSCH-004` says the numeric caps are not findings, and two existing lessons would have to lose words; (3) warnings only, with no block — rejected, HTML, a missing alt, a second pattern, and an oversized document are not soft.

Every figure below is stated here and only here. Later specs cite the id.

| Id | Rule | Severity | Label |
|----|------|----------|-------|
| N1 | A lesson introduces at most **8** new words (`newWords`). | **Block** on publish when `migration.legacy` is absent or false. **Warn** when `migration.legacy` is true. | Product choice. `RSCH-004` §3 rule 7. No source sets this cap. |
| N2 | A lesson whose `buildsOn` is non-empty lists at least **3** `recycles`. | **Warn** | Product choice. The current worksheet validator used 3. `RSCH-004` does not set a count of old words. There is no upper warn. |
| N3 | A `graded-story` and a `listening` passage treat at least **95%** of running words as known. | **Warn** below 95%. | Evidence-backed, Low. `RSCH-004` §3 rules 1 and 2. |
| N4 | A listening passage has no second coverage bar at **98%**. N3 is the only coverage warn. | The validator does not fail a passage solely for being under this bar. | Evidence-backed, Low. `RSCH-004` §3 rule 2. |
| N5 | A lesson has at most **one** pattern object, on its single introduction. | **Block** a second introduction or a second pattern. | Product choice. `RSCH-004` §3 rule 10. `rules[]` are lines of that pattern, not further patterns. |
| N6 | Each new word occurs at least **twice** in the introducing lesson (example sentence, story, listening, dialogue, or cloze). | **Warn** | Adapted. The current validator used 2. `RSCH-004` says no count guarantees that a word is known. |
| N7 | Introduction blocks are at most **one quarter** of the unit's blocks (`introductions × 4 ≤ block count`). | **Warn** | Adapted. `RSCH-004` §3 rule 4. The equal split is arbitrary in the source. Only `introduction` counts as language-focused. |
| N8 | Every image `alt` is **1 to 300** characters. | **Block** | Product choice. `RSCH-005` rule 7. No vocabulary study on alt text; the line keeps the meaning if the image fails. |
| N9 | UTF-8 JSON size: a plan at most **512 KiB**, a lesson at most **128 KiB**, a block at most **32 KiB**. | **Block** | Product choice. Security size limit. |
| N10 | At most **24** units in a plan, **12** lessons in a unit, **12** blocks in a lesson, **24** words on an introduction. | **Block** | Product choice. Structural ceiling, above the pedagogy cap in N1. |
| N11 | A plain text field is at most **4000** characters. | **Block** | Product choice. Security size limit. |
| N12 | A `fluency-pass` block has an empty new-item list (the schema gives it no word list). | **Block** the presence of a word list. **Warn** when that block sits in a lesson whose `newWords` is non-empty. Legacy lessons trip the warn. The new unit does not. | Adapted. `RSCH-004` §3 rule 3. |

Further rules, with no second copy of a number:

| Rule | Severity | Label |
|------|----------|-------|
| Each new word appears in a later lesson in a different sentence than the introducing lesson used. Inside a multi-lesson plan this is checked. A one-lesson legacy plan is exempt. | Warn | Adapted. `RSCH-004` §3 rule 6. Low for any required count of contexts. |
| Every new word in a multi-lesson plan appears in a later `spaced-review-checkpoint`. | Warn | Evidence-backed, High via `RSCH-003`, not re-argued (`RSCH-004` §3 rule 5). |
| That recall falls on a later calendar day. | Inform | The plan has no dates. The scheduler is ADR-0001, on the server clock after ADR-0002. |
| The new words of a lesson are not one semantic category (colors, body parts, days, near-synonyms). A theme that mixes kinds of words is allowed. | Warn, by the author or reviewer. Not a token counter. | Adapted, Low. `RSCH-004` §3 rule 8. |
| New words come from the current frequency band before the next band opens, except words the theme cannot avoid. | Inform | Adapted. `RSCH-004` §3 rule 9. No Hebrew list was validated. Do not block on this. |
| The pattern's focus also appears in a story or dialogue in the lesson or later in the unit. | Warn | Product choice. `RSCH-004` §3 rule 10. Which pattern comes first is not fixed by a trial. |
| A new word is shown with vowel points. Removing the points later is not a check. | Warn when the pointed `word` has no Hebrew point. | Product choice. `RSCH-004` §3 rule 11, consistent with `RSCH-001`. |
| The learner is not asked to produce a romanized form as the correct answer. | Block when a cloze `answer` or the correct pattern `choice` contains no Hebrew letter. | Product choice. `RSCH-004` §3 rule 12. |
| Comprehension questions are optional. If present, the answer is in the text. They may be in English. They are not the recall event. | Inform | Product choice. `RSCH-004` §3 rule 13. |
| A personal queue may insert a due review. It may not reorder unseen lessons off this plan's order. | Inform | Adapted, Low. `RSCH-004` §3 rule 14. Enforced by the scheduler, not by rewriting the plan. |
| First-language `meaning` is present on every new word. | Block | Evidence-backed, Moderate. `RSCH-005` rule 1. |
| An image, if any, is on the introduction word only. | Block | Adapted, Low. `RSCH-005` rule 2, plus the decision that pictures are not repeated on later blocks. |
| No image on a task whose job is the written form. | Block | Evidence-backed, Moderate. `RSCH-005` rule 3. |
| HTML in a text field: a `<` that starts a tag name, a slash, or `!`. | Block | Security. See Security below. |
| `audioPath` matches the relative `.mp3` pattern. | Block | INIT-002 path invariant, plus no remote URL. |

**Coverage estimate (N3).** Tokenize on whitespace. Strip leading and trailing punctuation. A token is known if it is in that lesson's `newWords`, `recycles`, or this closed function list, or if ADR-0001's unique stripped-key fallback matches exactly one of those:

`אֲנִי`, `אַתָּה`, `אַתְּ`, `הוּא`, `הִיא`, `אֲנַחְנוּ`, `זֶה`, `לֹא`, `כֵּן`

Prefixes are not stripped. A token that the paragraph lists in `glosses`, `support`, or a `spans[].text` counts as treated-as-known for this estimate only. The function list is a product choice so the estimate has a denominator. It is not a claim that those words have been taught.

Publish runs the block rules and refuses the snapshot on any block failure. Warn rules are returned to the MCP and shown in the builder. They do not by themselves refuse a legacy plan. A non-legacy plan still publishes with warnings, except where the severity is block. Inform rules are reported and do not refuse.

### 7. Draft and publish

**Options considered:** (1) `status` of `draft` or `published` on the plan, and ADR-0002 visibility on each lesson — **chosen**; (2) `hidden` as the only draft, which is what ADR-0002 chose — **changed** here, because a part edit needs a working copy that learners do not see while the last published snapshot stays up; (3) a second file beside the live lesson — rejected in ADR-0002 and still rejected. One document holds both, distinguished by status and by the snapshot.

| State | Who can open it |
|-------|-----------------|
| `status: "draft"` | Authors (admin builder and MCP). No learner. |
| `status: "published"`, lesson `visibility: "hidden"` | Authors. No learner. This is the fail-closed default for a **new** lesson. |
| `status: "published"`, `visibility: "everyone"` | Signed-in learners, shared catalog. |
| `status: "published"`, `visibility: "selected"` | Only accounts granted in the ADR-0002 grant table. Those ids are not copied into the plan. |
| `audience: "personal"`, `status: "published"` | Only `assigneeId`. The plan is absent from the shared catalog. `visibility` is `hidden` so a catalog pass cannot treat it as shared. |

A new plan starts `draft`, revision 1. Publish copies the working document onto the published snapshot, sets `status` to `published`, and increments `revision`. A later part edit changes the working copy, sets `status` back to `draft`, and increments `revision`. Learners keep reading the snapshot until the next publish. The conflict check uses the working copy's revision.

The three migrated plans are `published` and their lesson `visibility` is `everyone`, matching ADR-0002's one-time seed of lessons already on disk. The new unit is `draft`, and its lessons are `hidden`.

### 8. Migration of what exists

**Options considered:** (1) map every worksheet field, copy audio paths unchanged, mark the three plans `migration.legacy: true` — **chosen**; (2) delete words until each lesson satisfies N1 — rejected, the spec requires the wording carried over unchanged; (3) leave the worksheets as the authoring source — rejected for the new tools. The worksheet files themselves are not edited by this spec, and existing `index.html` files are not edited.

No audio path changes. A path that ends in `.mp3` in the worksheet appears character for character in the sample. Where the worksheet has no clip (the advice lesson), the plan invents none.

Local numeric ids become decimal strings. That is a type change. The digits are unchanged. They are part ids, not review keys.

`{word:…}` and `{strong:…}` markup is retired as presentation. The characters are joined into a plain string. Marked words are kept on `highlights` (a paragraph) or `focus` (a pattern rule). `{reported:…}` and `{direct:…}` become `spans` of `kind` `reported` or `direct`, with the same inner text. The advice lesson uses those spans for indirect speech. They are data, not HTML.

`grammar.icon` (`fa-puzzle-piece` on the advice lesson) is retired. It is a font-class name, not lesson content. It is listed on that plan's `migration.retiredFields`.

`migration.activityOrder` copies the old activity list. An empty list means the worksheet had no `activities` array; the blocks were taken from the sections that were present. `migration.retiredActivityTypes` lists `match` and `listen-pick` when that file contained them.

Pattern content sits on the introduction even when the old activity list showed `pattern` later. The text is unchanged. The old screen order is not the block order.

| `window.WORKSHEET` field | In the plan |
|--------------------------|-------------|
| `id` | `plan.id` and `lesson.id` |
| `level` | `lesson.level` |
| `title` | `plan.title`, `unit.title`, `lesson.title` |
| `subtitle` | `plan.subtitle`, `lesson.subtitle` |
| `footer` | `lesson.footer` |
| `legacyStorageKey` | `lesson.legacyStorageKey` when present. Not a review key. |
| `activities` | `migration.activityOrder`. Each `type` is mapped in Decision 2. |
| `activities[].seconds` | `fluency-pass.seconds` for `speed-quiz` |
| `activities[].from`, `activities[].count` | `short-dialogue.source`, `short-dialogue.count` for `build-sentence` |
| `vocab.heading`, `vocab.note` | `introduction.heading`, `introduction.note` |
| `vocab.items[]` | `introduction.words[]` |
| `vocab.items[].id` | `word.id` (string) |
| `vocab.items[].word` | `word.word`, and either `newWords` or `recycles` |
| `vocab.items[].meaning` | `word.meaning` |
| `vocab.items[].sound`, `translit`, `pos`, `trap`, `infinitive`, `sentence` | The same fields on the word |
| `vocab.items[].audio` | `word.audioPath` (unchanged) |
| `vocab.items[].sentenceAudio` | `word.sentenceAudioPath` (unchanged) |
| `vocab.items[].from` | `word.fromLessonId`, `recycles[]`, and `buildsOn` |
| `fill.bank`, `fill.sentences[]` | `introduction.cloze` |
| `fill.sentences[].id`, `textPre`, `textPost`, `prefix`, `answer`, `english` | The cloze item |
| `fill.sentences[].gloss[]` | Cloze `gloss[]` (`he`, `en` unchanged; `id` `g1`… assigned) |
| `grammar.tabLabel`, `heading`, `lede`, `prompt` | `introduction.pattern` |
| `grammar.icon` | Retired. Recorded on `migration.retiredFields`. |
| `grammar.rules[]` | `pattern.rules[]`. `example` flattened to plain text plus `focus`. |
| `grammar.reminders[]` | `pattern.reminders` |
| `grammar.items[]` | `pattern.items[]`, including `answer` index, `choices`, `pointed`, `why` |
| `grammar.items[].audio` | `pattern.items[].audioPath` (unchanged) |
| `reading.notice` | `graded-story.notice` |
| `reading.support[]` | `graded-story.support[]` (`id` `s1`… assigned; `he` and `en` unchanged) |
| `reading.paragraphs[]` | `graded-story.paragraphs[]` with ids `p1`… in source order |
| `reading.glosses` | `graded-story.glosses` when present. Absent on these three lessons. |
| `reading.questions[]` | `graded-story.questions[]` |
| `reading.questions[].audio` | `question.audioPath` (unchanged) |
| `reading.audio[]` | `paragraph.audioPath` at the same index (unchanged) |
| `reading.retell.seconds`, `reading.retell.cues` | `fluency-pass` `fluency-retell` |
| `askFriend.intro`, `askFriend.introEn` | `short-dialogue.intro`, `introEn` |
| `askFriend.questions[]` | `short-dialogue.lines[]`, including `question`, `pointed`, `youFeminine`, `speak`, `gloss`, `followUp` |
| `askFriend.questions[].audio` | `line.audioPath` (unchanged) |

Legacy publish notes, from the samples: `my-morning` has 8 new words (on the N1 line). `anal-sex-vocab` has 9 new words and 3 recycles from `my-morning` (`buildsOn: ["my-morning"]`). `shai-date-indirect-speech` has 10 new words and does not declare a prior lesson. N1 warns on the latter two and does not block them, because `migration.legacy` is true. Each legacy lesson's fluency block shares the introducing lesson, so N12 warns on all three.

### 9. Carried constraints

**Options considered:** (1) supersede script-tag loading for the plan document only, and keep every other carried constraint — **chosen**; (2) keep script-tag loading for plans as well — rejected in Decision 3.

Each constraint is listed once.

| Source | Constraint | Here |
|--------|------------|------|
| ADR-0001 #1, already superseded by ADR-0002 | Progress only in `localStorage`; no accounts | **Kept** as ADR-0002 left it. Not reopened. Plans do not store progress. |
| ADR-0001 #2; ADR-0002 carried row; INIT-002 REQ-014 | No `fetch` for lesson or vocab data; script tags only | **Superseded** for the plan document at the API cutover (Decision 3). **Kept** for legacy `window.WORKSHEET` files until they are removed. |
| ADR-0001 #3; REQ-014 | No edits to existing lesson `index.html` | **Kept**. Samples are new files under `docs/decisions/`. |
| ADR-0001 #4; REQ-014 | No root or binyan speed drills | **Kept**. A fluency pass is not that drill. |
| ADR-0001 #5; REQ-014 | No forced vowel fading; points on by default | **Kept**. Points on the new word are a warn (Decision 6). Taking them off is not a publish check. |
| ADR-0001 #6; REQ-014 | Bilingual UI labels | **Kept** for player chrome. Not a plan field. |
| ADR-0001 #7; REQ-014 | No account names, transcripts, or secrets in lesson files, glosses, the vocab index, or ADRs | **Kept**. |
| ADR-0001 | `exactKey` identity | **Kept**. Decision 1. |
| ADR-0002 | Server is authoritative for the review schedule | **Kept**. The checkpoint names words. It does not store due dates. |
| ADR-0002 | `fetch` only for `/api/` | **Changed** in scope: plan GET is one of those `/api/` calls after the cutover. Still no `fetch` of a worksheet URL. |
| ADR-0002 | New lessons default to `hidden`; `hidden` was the draft | **Changed**. Draft is `status: "draft"`. `hidden` remains visibility of a published lesson. Default for a new lesson is still `hidden`. |
| ADR-0002 | Visibility `hidden` / `everyone` / `selected`, grants in the account database | **Kept**. Decision 7. |
| ADR-0002; REQ-014 | Audio paths and URLs unchanged; archive before overwrite; inventory and verify | **Kept**. Decision 5 and Decision 8. |
| ADR-0002; REQ-014 | ElevenLabs key only in server env | **Kept**. This record does not copy it. |
| ADR-0002 | No instructor role; MCP does not read learner progress | **Kept** until ADR-0004. This language does not put progress in the plan. |
| ADR-0002 | One replica; `DATA_DIR` outside served roots | **Kept**. Not changed here. |

### 10. Worked examples

**Options considered:** (1) the three existing lessons as three plans, plus one new unit of three lessons — **chosen**; (2) one plan that pretends the three old lessons were written as a single course — rejected, only the intimate lesson declares a `from` link, and inventing the others would falsify the sources.

| File | What it shows |
|------|----------------|
| `docs/decisions/ADR-0003-assets/samples/my-morning.plan.json` | The morning lesson. 8 new words, `buildsOn` empty, `recycles` empty. Published, `everyone`, `migration.legacy`. |
| `docs/decisions/ADR-0003-assets/samples/anal-sex-vocab.plan.json` | The intimate-vocabulary lesson. 9 new words. `buildsOn` `my-morning`. Three `recycles` copied from `vocab.items[].from`. Audio paths unchanged. |
| `docs/decisions/ADR-0003-assets/samples/shai-date-indirect-speech.plan.json` | The advice lesson. 10 new words. No prior lesson in the source. Indirect-speech spans kept. No audio paths invented. |
| `docs/decisions/ADR-0003-assets/samples/beginner-unit-1.plan.json` | New unit `unit-1`, theme "ordering at a café". Draft, lessons `hidden`. |

The new unit, in the plan file itself:

| Lesson | `buildsOn` | `newWords` | `recycles` |
|--------|------------|------------|------------|
| `cafe-order` | none | 8, mixed parts of speech, one pattern `אֲנִי רוֹצֶה` | none |
| `cafe-again` | `cafe-order` | 5, one pattern `בְּלִי` | the 8 words from `cafe-order` |
| `cafe-recall` | `cafe-order`, `cafe-again` | none (fluency and checkpoint) | all 13 earlier words, each with `fromLessonId` |

`cafe-order` and `cafe-again` each have an introduction, a graded story, a listening passage, and a short dialogue. Pictures sit only on introduction words that name a depictable thing (`coffee`, `water`, `milk`, `sugar`), each with `alt` equal to the English meaning. `cafe-recall` is a fluency pass, a short dialogue, and a `spaced-review-checkpoint` whose `wordKeys` are the 13 earlier words and whose `fromLessons` are the two earlier lessons. Introduction blocks are 2 of 11 in the unit. The checkpoint is its own lesson, not the last screen of an introduction. No clip paths are invented; those fields are omitted until a recording exists.

---

## Security

Plan strings are untrusted input. They are rendered as text and escaped. The player does not assign a plan string to HTML. The schema rejects a string that contains an HTML tag start (`<` followed by a name, `/`, or `!`). There is no markdown in schemaVersion `1.0.0`.

Media is an id plus alt (N8). The plan has no image URL, no `src`, and no embedded markup. Audio paths are relative `.mp3` paths with no scheme.

Size limits are N9, N10, and N11.

`assigneeId` is an opaque account id of at least 8 characters from `[a-z0-9-]`. It is not a display name. Samples of shared plans set it to `null`. Personal plans are not listed in the shared catalog.

Adult lesson wording that already exists is carried into that lesson's sample. This record does not add account names, secrets, or transcript text. Visibility of that published lesson stays `everyone`, matching the seed ADR-0002 already describes. New-unit lessons stay `hidden` until an author publishes and assigns them.

| Threat | Mitigation |
|--------|------------|
| HTML or script in a plan field | Plain text, escaped at render; schema rejects tag-like strings |
| Remote image or audio URL in the plan | Media id only for images; relative `.mp3` pattern for audio |
| Oversized document | N9, N10, N11 |
| Personal plan leaked through the shared catalog | `audience` plus `assigneeId`; `visibility` `hidden`; bytes only inside a lesson the account can open |
| Lost update between MCP and builder | One revision; mismatch rejects the write |
| Picture that teaches the wrong meaning | Alt required here; human approval of the bytes is ADR-0004 |

---

## Consequences

### Positive

- Later specs implement one language. SPEC-006 can draw a real block. SPEC-007 can store and serve it. SPEC-009 can load the café unit. INIT-004 can cite N1–N12 instead of picking new caps.
- An AI can patch one field by id path and get a hard failure on a typo or a stale revision.
- Existing review rows still match, because the word key did not change.
- Existing clips keep their paths.

### Negative / trade-offs

- Two existing lessons sit over N1. They warn, and they are not rewritten to fit the cap.
- Legacy fluency blocks share a lesson with new words. N12 warns. The new unit puts fluency on a lesson with an empty `newWords`.
- `file://` loading of a plan is not the runtime after the cutover. Legacy worksheets still load as they do today until they are removed.
- Coverage is an estimate. Prefixes are not stripped, so a prefixed form needs a gloss or it counts as unknown.
- Rule 9 cannot be enforced until a Hebrew frequency list exists. It is inform only.
- The spelling ladder and the keyboard course are absent. A later version has to earn them with a new decision, not a silent schema addition.

### Rollback

Delete `docs/decisions/ADR-0003-course-and-lesson-plan-language.md` and `docs/decisions/ADR-0003-assets/`. Set SPEC-004 `status` back to `ready`. No worksheet, audio file, or `index.html` is changed by this spec, so nothing else reverts.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|------------------------|
| SPEC-006 | The six block types, as the lesson screen it mocks |
| SPEC-007 | The document, the part operations, the revision, and the load path |
| SPEC-009 | `node spikes/skeleton/validate.js` over the samples and the schema; renders `beginner-unit-1.plan.json` |
| INIT-004 | Entities, N1–N12, MCP/builder parity, draft and snapshot, media ids, unchanged audio paths |

---

## References

- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` — §3 design rules, §4 lesson types
- `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` — picture rules, activity list
- [ADR-0001: Learner review model](ADR-0001-learner-review-model.md) — `exactKey`
- [ADR-0002: Accounts, server-side progress, and site map](ADR-0002-accounts-server-progress-site-map.md) — visibility, audio invariants, `/api/` fetch
- `.cursor/skills/hebrew-worksheet/schema.md` — current `window.WORKSHEET` fields and the eleven activity types
- `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-004-docs-adr-0003-course-and-lesson-plan-language.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Initial Proposed record — `INIT-003/SPEC-004`. aud-1 (one plan for shared and personal) and aud-2 (script-tag load may be superseded) recorded as chosen. |
| 2026-10-09 | ADR-0009 (`INIT-006/SPEC-001`, Proposed) would extend this record: schema `1.1.0` adds five blocks (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`), a lesson `recipe`, and an introduction-word `group`; adds rules N13–N19; changes N1 for recipe `topic-conversation`, N2 for `guide`, and N7's language-focused set; reverses the "script and sound support is fields on the introduction word" row for `letter-intro`. `1.0.0` plans stay valid. Metadata note and this row only; the sentences in this file are left as written. Takes effect only when ADR-0009 is Accepted. |
| 2026-10-09 | ADR-0009 Accepted by the operator. The extension in the row above is in force. Metadata note and this row only. |
