---
name: hebrew-worksheet
description: >-
  Builds and edits a Hebrew lesson plan through the plan MCP tools (create_plan
  first, then the six operations, list_plan_issues, publish_plan), or edits a
  worksheet that has not moved yet through upsert_lesson. Use when the user
  asks to create, add, edit, or publish a Hebrew lesson, plan, worksheet, or
  דף עבודה, to turn class notes into a lesson, or to draft the next lesson
  from the learner profile.
---

# Hebrew lesson plans

The server validates every write. This skill names the tools and the rules you follow. It is not a second validator.

## Default path

Build and edit lesson plans with the MCP tools on the `hebrew-lessons` server. These tools require the server auth token. A missing or wrong token returns no plan body.

**New lessons are plans.** Start one with `create_plan`. Do not write a new lesson with `upsert_lesson`; that tool is for a worksheet that has not moved onto a plan (see Legacy worksheets). This is a recommendation, not a refusal: the server still accepts `upsert_lesson`.

Imported plan ids include `beginner-unit-1` and the three legacy plans `my-morning`, `anal-sex-vocab`, and `shai-date-indirect-speech`. To change one of those, open it with `read_part`.

### The AI-first loop

1. `list_plans` — see what exists: id, title, status, revision, whether a snapshot is published, and how many fields a person last wrote.
2. `create_plan` for a new lesson (or `read_part` with an empty `path` to open an existing plan).
3. Build with the six operations (`insert_block`, `replace_part`, `set_field`, `move_block`, `remove_block`, and `read_part` before each edit, below).
4. `list_plan_issues` — every finding the validator has for the plan, hard errors and warnings, each with an id path and, where one is known, a fix.
5. Fix what you can with `set_field` or `replace_part`. Use `fix.suggested` where the finding has one.
6. `publish_plan` when `list_plan_issues` shows no `block` finding. No person approves a publish. See "Publishing" below.

Call these tools by these names:

| Tool | Call |
| --- | --- |
| `list_plans` | List plans: id, title, audience, status, revision, updated time, whether published, whether it has unpublished changes, and `humanEditedFields` (how many fields a person last wrote). Optional `limit`. No assignee name is returned. |
| `create_plan` | Make a new draft: `title`, optional `recipe`, `unitId`, `lessonId`. Omit `id`. The server assigns a UUID. Revision 1, audience `shared`, lesson `hidden`. Passing an id that exists returns `already_exists` and changes nothing. |
| `read_part` | Read the part at `path`. An empty `path` is the whole plan. Returns the part, the current revision, and `editState` (below). Does not change the plan. |
| `replace_part` | Replace the part at `path` with `value`. The value's `id` must equal the id in the path, or the write returns `id_mismatch`. |
| `insert_block` | Insert `block` on a lesson path. Pass `index` to place it; omit `index` to append. |
| `move_block` | Move the block at `path` to `index` inside its lesson. |
| `remove_block` | Remove the block at `path`. |
| `set_field` | Set `field` to `value` on the part at `path`. `audience` and `assigneeId` are not settable here: that is `assign_required`. |
| `list_plan_issues` | List the plan's findings. Optional `severity` (`block`, `warn`, `inform`, `all`, or a list), `code`, `pathPrefix`, `limit`. Returns `counts`, and for each issue `code`, `severity`, `hard`, `path`, `field`, `message`, and `fix`. |
| `make_word_audio` | Make (or reuse) the spoken clip for up to 20 Hebrew words, pointed as in the plan. Pass `words`. Returns each word's `audioPath` and whether it was `created`, or an `error`. Does not change the plan: save each path with `set_field` (field `audioPath`). See "Audio already in a plan". |
| `suggest_glosses` | Draft an English meaning for each word of a `graded-story` block that has no gloss yet. Pass `id` and the block's `path`. Returns `suggestions` (`word`, `meaning`, `form`, `senses`), `unmatched`, and `revision`. Read-only. See "Word meanings for a story". |
| `list_plan_revisions` | List the history, newest first: revision, operation, path, field, who (`ai`, `human`, `system`), tool, time. Optional `sinceRevision`, `beforeRevision`, `actorKind`, `pathPrefix`, `limit`. A person appears as `human`, never by name. |
| `get_plan_revision` | Read the whole document as it was at one revision. |
| `diff_plan` | Compare two states field by field. `from` and `to` are a revision number, `working`, or `published`. Optional `path`. `no_snapshot` means nothing is published yet. |
| `restore_plan_revision` | Put an earlier value back as a **new** revision: `fromRevision`, optional `path` and `field` (the narrowest that fits), and `expectedRevision`. History is not edited. It cannot change `audience` or `assigneeId`. |
| `assign_plan` | The only way to change `audience` or `assigneeId`: `audience` `shared` or `personal`, `assigneeId` for `personal` (an active account), and `expectedRevision`. `assignee_refused` means that account cannot be used. |
| `publish_plan` | Publish the working copy: `id` and `expectedRevision`, nothing else. Learners keep the previous snapshot until this succeeds. |

Every write sends `expectedRevision`: the integer from the last read of that plan. A successful mutation increments the revision by 1.

If the tool returns `revision_conflict`, stop. The stored revision is in the error, and the document is unchanged. Read the part again and apply your edit to that revision. Do not merge.

### A new plan starts with a placeholder block

`create_plan` writes one placeholder block (`fluency-pass`, id `placeholder`) because a lesson must hold a block. Insert your real block first, then `remove_block` the placeholder; removing it first is refused. Omit `id`: the server assigns a UUID, and that id is how you address the plan afterwards. The unit and lesson ids default to `unit-1` and `lesson-1`. A part id cannot be changed afterwards (`id_mismatch`), so pass `unitId` and `lessonId` to `create_plan` when you want other names. `recipe` sets the lesson's recipe and `schemaVersion` `1.1.0` (or `1.2.0` when the first block needs `exercise`, `assignment`, or `tense: future` on a table); it writes no Hebrew and no block shells.

### Read before you edit

Call `read_part` on a part before you change it. Its `editState` lists the fields under that part that **a person last wrote**: each entry has `path`, `field`, `revision`, `previousValue` (the value the person replaced) and `previousWriter` (`ai`, `human`, `system`, or `unknown`). `lastHumanEdit` is the newest person edit anywhere in the plan.

This is advice, not a refusal. Nothing stops you editing a field a person fixed, and nothing is refused because of who wrote last. If you write over one, your result lists it under `overwrote` with the person's value. Read that list. If the person's value was right, call `restore_plan_revision` for that field (`fromRevision` is the `revision` in `editState` or in `overwrote`, `humanRevision`). Prefer to keep a person's fix unless you have a reason, and say what the reason is in your reply.

### Issues

`list_plan_issues` returns hard errors (`severity` `block`; they stop a publish) and warnings. Five checks report as `warn` and never block a write or a publish:

- `pointing-form` — a vowel point with no letter before it, two vowel points on one letter, or text that is not in canonical (NFC) order. The fix is the canonical value.
- `field-language` — Hebrew in a field meant for English or Latin letters, or the reverse.
- `lexicon-form` — a word pointed differently from its one lexicon entry, or a meaning or transliteration that differs from a **verified** entry. Treat the lexicon form as the authority only when the entry is `verified`, and only a person verifies a form. Otherwise it is a lead.
- `pointing-consistency` — one word pointed two ways in one lesson. `fix.candidates` lists the forms; both may be right (a homograph).
- `audio-stale` — text changed since the last publish while its clip path stayed. See "Audio already in a plan".

A `fix` is `{ action, path, field, suggested }` (or `candidates`, or a `note`). `action` is `set_field` when the field is one property of the node at `path`, and `replace_part` when it is nested; then `suggested` is the new value of that sub-field.

### Word meanings for a story

A reader taps a word in a `graded-story` and sees the block's `glosses` meaning for it. Research (RSCH-003, glosses in the reader's first language, Moderate) supports a short English meaning on every word the learner does not yet know, so write one for each.

1. Call `suggest_glosses` with the plan `id` and the block `path`. It reads the dictionary and writes nothing.
2. Read every suggestion against the story. The dictionary has no context. In a story it can offer `לחם` as "to fight" where the text means "bread", `אוכלים` as "food" where it means "eat", or `לבית` as "to domesticate" where it means "to the house". `form` names the base word a suggestion came from, and `senses` lists the other meanings the dictionary has. Pick the sense the story uses and write it short. Words in `unmatched` have no entry: write those yourself.
3. Save the whole map with one `set_field` on the block, field `glosses`: keys are the words as written in the paragraphs, values are the English meanings. At most 80 words. `set_field` replaces the map, so send the words that already have a gloss as well.
4. A wrong meaning teaches the wrong thing. Do not save a suggestion you have not checked.

### Publishing

The server publishes any plan with no `block` finding, with no approval step. You may publish. Two things the server still refuses: a `block` finding (`validation`) and a picture still pending on a lesson visible to `everyone` or `selected` (`pending_media`).

**Guidance, not enforcement:** keep a new lesson `hidden` unless the operator asked you to expose it. `visibility` is a field you can set with `set_field`, so you could publish a lesson and show it to learners with no person involved. History and `restore_plan_revision` undo that, but a learner may have seen it first. After you publish, say in your reply what you published, its revision, and which warnings remain.

### Id paths

Address a part by id path, not by array index. The paths are ADR-0003 §4:

```text
units/{unitId}
units/{unitId}/lessons/{lessonId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/words/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/paragraphs/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/lines/{id}
units/{unitId}/lessons/{lessonId}/blocks/{blockId}/questions/{id}
```

Example: `units/unit-1/lessons/cafe-order/blocks/introduction/words/2`.

`set_field` names one field under that path, such as `title`, `meaning`, `alt`, or `newWords`.

The document is JSON with `schemaVersion` `1.0.0`, unless the loop below is writing a recipe or a `1.1.0` block. The field list is [schema.md](schema.md). The machine schema is `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`.

### Validator ids N1–N12

Rules N1 through N12 live in ADR-0003 §6. Cite an id when a write is refused or warned. Do not restate the caps, and do not keep a second copy of them here.

N1 warns when `migration.legacy` is true. N1 blocks publish when `migration.legacy` is absent or false. Do not delete words from `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech` to satisfy that cap.

Publish still runs the block rules. A warning is not a request to rewrite a legacy plan down to the cap.

### Images

A picture is allowed only on an introduction word:

```json
{ "mediaId": "coffee-cup", "alt": "coffee" }
```

`alt` is required. It states the word's meaning in the gloss language. Story, listening, dialogue, fluency, and checkpoint blocks have no image field.

Request a picture in this order:

1. `plan_image` — dry-run for one prompt. It returns an image count and a dollar estimate. It does not call the provider.
2. `generate_image` — call it only with `confirm: true`, the `estimate` from that dry-run, an `id` for the media record, and an alt-text draft. Without `confirm: true` the provider is not called. The image stays pending.
3. A human approves the pixels and the alt text. You cannot approve. Approval is not an MCP success. `upload_media`, `plan_image`, and `generate_image` do not approve, and no approve tool is registered.

`upload_media` stores one PNG, JPEG, or WebP as pending media. Pass `id` and base64 `bytes`, plus an alt-text draft. A file path is not accepted. The upload stays pending until a human approves it.

After a media id exists, set the introduction word's `image` with `set_field` (or replace that word). The plan stores `mediaId` and `alt`. It does not store image bytes or a generator prompt.

### Audio already in a plan

Copy `audioPath` and `sentenceAudioPath` unchanged, including the same fields on a paragraph, question, pattern item, or line. Do not invent a `.mp3` path. Where the plan has no clip, leave the field absent.

**Sound for a new word or dictation cue.** A word's own `audioPath` comes from `make_word_audio`, never from a guess. Pass the words exactly as the plan spells them, with vowel points (up to 20 at a time): the clip is for that exact spelling, and the points are what make the voice say it right. A word that already has a clip is reused for free; a new one is made once with ElevenLabs and kept for every lesson. The tool returns each word's `audioPath` (or an `error`: `bad_word` for anything that is not a short Hebrew word, `not_configured` when the server has no key, `failed` otherwise). It does not change the plan, so save each path with `set_field` on the word entry (field `audioPath`), or on a dictation item (`exercise` with `kind: dictation`, field `audioPath` on the item). A teacher can do the same in the editor: the word's Audio file field is a list of existing sounds with Hear and Make sound buttons. Dictation clips use the same tool when the heard text is a short word or phrase you can pass as one `words` entry. Sentence, story-line, and question clips still have no dedicated tool beyond reusing an existing path.

A person's text fix leaves the old clip attached: when a person changes a sentence, passage, or word, its `audioPath` or `sentenceAudioPath` still points at the clip for the old text. `audio-stale` names the text field and the clip. `generate_lesson_audio` works on legacy worksheets only, so a changed sentence has no tool. For a changed word, call `make_word_audio` with the new spelling and save the path it returns. For a sentence the fix is to leave the stale clip path out (`replace_part` without it) so the old clip stops playing, or to set a path to a new `.mp3` that already exists. Do not invent a path.

Before any regeneration, run `npm run audio:inventory`. After it, run `npm run audio:verify`.

### What never goes in a plan file

Do not write any of these into a plan, a worksheet, a gloss, or an example in this skill:

- An account name. The only sample account id is `example-learner`.
- A learner profile, or any field copied out of one.
- Learner data, including answer text, typed text, and progress.
- A class transcript or chat.
- A secret, a token, or an API key.

The profile loop is below. Reading a profile is not permission to store it.

Also kept from the course decision: do not add a root or binyan speed drill, do not force vowel fading, and do not edit a lesson's `index.html`.

## Legacy worksheets

`upsert_lesson` is the tool for a worksheet that has not moved onto a plan (today, the three existing worksheets). It is not the plan editor. It does not accept a plan document. For a new lesson, prefer `create_plan`; that is a recommendation, not a refusal, and the server still accepts `upsert_lesson`.

For that worksheet only:

1. `list_activity_types`, `list_words`, `list_lessons`, and `get_lesson` still describe the worksheet catalog.
2. `upsert_lesson` writes `window.WORKSHEET` and validates it. Omit audio fields. The server fills clips. Do not invent a `.mp3` path on this path either.
3. `set_lesson_visibility` shares or hides the worksheet after the write. It does not change who can read a published plan for the same slug. To restrict a lesson that has one, first `set_field` the plan lesson's `visibility` and `publish_plan`, then call `set_lesson_visibility`. A new worksheet stays hidden until it is assigned. If you need a sample account id, use `example-learner`. `list_users` can list accounts for that assignment. Do not copy a real account name into the worksheet or into a plan.
4. `generate_lesson_audio` backfills clips for a worksheet. Run `npm run audio:inventory` before regenerating and `npm run audio:verify` after.

Without MCP, copy `worksheets/_template` to `worksheets/<slug>/`, edit only `worksheet.js`, add a catalog row, and run:

```text
node .cursor/skills/hebrew-worksheet/scripts/validate.js <slug>
```

The worksheet field list is the legacy section of [schema.md](schema.md). [example.md](example.md) is a worksheet sketch, not a plan.

## From class notes

Take the lesson out of the notes: one pattern, known words brought back, pointed Hebrew. Do not paste the transcript, the learner's name, or the learner's answers into the file.

## Build the next lesson

Provenance: `INIT-006/SPEC-019`. This section is guidance. The server still validates every write and still refuses a profile when that learner's switch is off. Read [recipes.md](recipes.md) before you choose a recipe or tag a word. The cards and the tagging tables live there.

The only username in an example is `example-learner`. Pass it as a tool argument when the operator is working that sample account. Do not write it into a plan, a block, a gloss, or a lexicon entry.

### The six steps

1. Call `get_learner_profile` with `{ "username": "example-learner" }` only when that is the account the operator named. If the tool returns `planning_off`, stop. Tell the operator the switch is off. Do not retry, do not invent a profile, and do not draft from memory of an earlier profile. `rate_limited` is the same kind of stop: tell the operator and wait. A profile has no glosses. Join a `word_id` to a meaning with `get_lexicon_entry`. The field list and the never-returned list are ADR-0009 §1. Do not paste the profile, or any part of it, into a plan or a lexicon entry.
2. Choose a recipe. Rotate among `topic-conversation`, `review-worksheet`, `verse-reading`, and `verb-workshop`. Use `letters-and-vowels` when `journey.stage` is `script`. `guide` is not part of that rotation; use it when the operator asks for a reference page. Do not write three lessons in a row with the same recipe (N17, inform). Read the unit you are appending to and pick a different recipe when the previous two lessons already share one. Open with review of weak words. Due dates are not in the profile; do not invent them. End most lessons with a personal question.
3. Pick words from the profile, then from the lexicon. Recycle `very_weak_words`, then `weak_words`, then words whose stage is `learning` or whose strength is `fragile` or `shaky`. Introduce new words only inside that recipe's cap: N18 for `topic-conversation`, N1 for every other recipe. Cite the id. Do not copy the numbers into the lesson. When any `grammar[]` entry has status `weak` or `very_weak`, the next `verb-workshop` targets the weakest of those (lowest `recent`, then lowest `accuracy`). `list_grammar_concepts` supplies the concept id. It is not learner data.
4. Write with the plan tools in the default path: `create_plan` for a new lesson, then `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, and `list_plan_issues`. A recipe or any of the five blocks (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`) belongs in schema `1.1.0`. `exercise`, `assignment`, and `conjugation-table` with `tense: future` belong in schema `1.2.0`. A part operation that adds a block above the plan's current version raises `schemaVersion` in that same revision (never down). Set the lesson field `recipe` with `set_field`. Tag every new word with `upsert_lexicon_entries` before you teach it: `pos`, `root`, `binyan`, `tense`, `cell`, and `accept[]`, as [recipes.md](recipes.md) defines. Pass `expectedRev` 0 for a new key, or the current rev when you edit. A `verified` field on the input is ignored. If `add_library_words` returns `unknown_keys`, write the entries first and retry that call. The cap is 50 new library words per learner per local day. `planning_off` on that call writes nothing.
5. Call `check_text_coverage` on every reading you write (`graded-story`, `listening`, and the `verse` text of a `verse-reading`). `text` is at most 4000 characters. Revise until `coverage` is at least 0.95. That bar is `RSCH-004` §3 rule 1 (Low). The validator's N3 enforces it for `graded-story` and `listening` only. A verse is not blocked by N3. Revise the verse anyway until the tool reports at least 0.95. `planning_off` means nothing was scored; stop and tell the operator.
6. Check and publish. Call `list_plan_issues`, fix what you can, and call `publish_plan` when there is no `block` finding (see "Publishing" above: you may publish without asking). Keep the lesson `hidden` unless the operator asked you to expose it: that is guidance, not something the server enforces. A plan's lesson is exposed through its own `visibility` field, not through `set_lesson_visibility`. Report what you published and which warnings remain. Do not call `set_lexicon_verified` or `set_ai_planning`. Both always return `forbidden` and write nothing. Marking a form verified is a person's act. The operator ticks the forms checklist; you do not.

### INIT-006 tools

These names are the ones registered in `mcp/tools.js`. Call them by these names. Every one requires the server auth token. This table does not assign a token.

| Tool | Call |
| --- | --- |
| `get_learner_profile` | Read one learner's profile. `planning_off` means stop. |
| `check_text_coverage` | Score Hebrew text for that learner. Returns `coverage`, `known_count`, `running_words`, and `unknown`. |
| `list_grammar_concepts` | List concept ids. Not learner data. |
| `list_lexicon` | Search entries before you cite a word or add it to a library. |
| `get_lexicon_entry` | Read one entry by the same key as a profile `word_id`, with its lemma and paradigm. |
| `upsert_lexicon_entries` | Write entries. Stored as source `mcp`. Cannot set `verified`. |
| `add_library_words` | Add lexicon keys to that learner's library. Keys must already exist. |
| `set_lexicon_verified` | Do not call. Always `forbidden`. |
| `set_ai_planning` | Do not call. Always `forbidden`. An admin turns the switch on in the admin Users view. |

Rules N13 through N23 live in ADR-0009 §7 and ADR-0014. Cite an id when a write is refused or warned. Do not keep a second copy of the caps here.

### Hard rules

- Never copy the profile, a username, a typed answer, or a transcript into a plan, a worksheet, a gloss, or a lexicon entry.
- Never mark a form verified, and never tell the operator that an unverified form has been checked.
- Reduce English, and reduce vowel points on text the learner has already met, only as `journey.stage` rises, and only in text you author. `script` and `foundations` keep a gloss and an English line under the Hebrew, and they keep vowel points. `building` shortens the English. `expanding` and `independent` lead with Hebrew and keep English only where the frame needs it. New words stay pointed. A recipe never turns the learner's vowel-point toggle off. That toggle stays theirs (ADR-0001 #5). `journey.vowels_off_share` is information, not an instruction to strip points.
- Do not add a timed root or binyan speed drill. An untimed `conjugation-table` in `drill` mode is allowed.
- Do not edit a lesson's `index.html`.

## Activity names

Author-facing labels on the plan builder menu map to stored block types in `shared/admin/activity-map.js` (`blockTypeFor`). Use these names in worksheet-to-lesson mapping ([recipes.md](recipes.md)).

| Activity | Block type |
| --- | --- |
| Words | `introduction` |
| Story | `graded-story` |
| Listening | `listening` |
| Dialogue | `short-dialogue` |
| Writing | `personal-question` |
| Verse | `verse-reading` |
| Pattern | `conjugation-table` |
| Type it | `type-it` |
| Exercise | `exercise` |
| Assignment | `assignment` |
| Fluency | `fluency-pass` |
| Review | `spaced-review-checkpoint` |
| Notes | `explainer` |

Match column A to B stays INIT-010's `match` block; it is not on this menu until INIT-010 SPEC-004 lands.

## Recipe cards

One card per recipe in ADR-0009 §7 and the five teacher lesson types in ADR-0014. The block order is guidance for you. The validator does not enforce that order except through the rule ids on the card. The shapes follow the operator's summary of lesson types. They are not quotations from a class sheet. Write original sentences.

The cards, including the research note on each step, are in [recipes.md](recipes.md):

| Recipe | Blocks, in order |
| --- | --- |
| `topic-conversation` | `spaced-review-checkpoint`, `introduction`, `short-dialogue`, `personal-question`, `assignment` |
| `review-worksheet` | `introduction`, `exercise` (choose / unscramble / translate), `graded-story`, `personal-question` or `assignment`; type E adds `true-false` and choose after the story |
| `verse-reading` | `verse-reading`, grammar pivot (`explainer` or `conjugation-table`), optional `exercise` translate, `personal-question` |
| `verb-workshop` | `explainer`, `conjugation-table` (`present`, `past`, `future`, one tense per table), `conjugation-table` in `drill`, `personal-question` |
| `letters-and-vowels` | `letter-intro` per letter, then `spaced-review-checkpoint` |
| `guide` | `explainer` only |
