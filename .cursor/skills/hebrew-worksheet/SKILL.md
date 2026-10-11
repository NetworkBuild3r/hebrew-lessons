---
name: hebrew-worksheet
description: >-
  Edits a Hebrew lesson plan through the plan MCP tools, or a worksheet that
  has not moved yet through upsert_lesson. Use when the user asks to create,
  add, edit, or publish a Hebrew lesson, plan, worksheet, or דף עבודה, to
  turn class notes into a lesson, or to draft the next lesson from the
  learner profile.
---

# Hebrew lesson plans

The server validates every write. This skill names the tools and the rules you follow. It is not a second validator.

## Default path

Edit an existing plan with the MCP tools on the `hebrew-lessons` server. These tools require the server auth token. A missing or wrong token returns no plan body.

There is no tool that creates a plan id. Edit a plan that is already imported. Ask the operator before expecting a create tool.

Imported plan ids include `beginner-unit-1` and the three legacy plans `my-morning`, `anal-sex-vocab`, and `shai-date-indirect-speech`.

Call these tools by these names:

| Tool | Call |
| --- | --- |
| `read_part` | Read the part at `path`. An empty `path` is the whole plan. Returns the part and the current revision. Does not change the plan. |
| `replace_part` | Replace the part at `path` with `value`. |
| `insert_block` | Insert `block` on a lesson path. Pass `index` to place it; omit `index` to append. |
| `move_block` | Move the block at `path` to `index` inside its lesson. |
| `remove_block` | Remove the block at `path`. |
| `set_field` | Set `field` to `value` on the part at `path`. |
| `publish_plan` | Publish the working copy. Learners keep the previous snapshot until this succeeds. |

Every write sends `expectedRevision`: the integer from the last `read_part` of that plan. A successful mutation increments the revision by 1.

If the tool returns `revision_conflict`, stop. The stored revision is in the error, and the document is unchanged. Read the part again and apply your edit to that revision. Do not merge.

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

`upsert_lesson` is the tool for a worksheet that has not moved onto a plan. It is not the plan editor. It does not accept a plan document.

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
4. Write with the plan tools in the default path (`read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`). A recipe or any of the five blocks (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`) belongs in schema `1.1.0`. A part operation that adds one of those to a `1.0.0` plan raises `schemaVersion` to `1.1.0` in that same revision. Set the lesson field `recipe` with `set_field`. Tag every new word with `upsert_lexicon_entries` before you teach it: `pos`, `root`, `binyan`, `tense`, `cell`, and `accept[]`, as [recipes.md](recipes.md) defines. Pass `expectedRev` 0 for a new key, or the current rev when you edit. A `verified` field on the input is ignored. If `add_library_words` returns `unknown_keys`, write the entries first and retry that call. The cap is 50 new library words per learner per local day. `planning_off` on that call writes nothing.
5. Call `check_text_coverage` on every reading you write (`graded-story`, `listening`, and the `verse` text of a `verse-reading`). `text` is at most 4000 characters. Revise until `coverage` is at least 0.95. That bar is `RSCH-004` §3 rule 1 (Low). The validator's N3 enforces it for `graded-story` and `listening` only. A verse is not blocked by N3. Revise the verse anyway until the tool reports at least 0.95. `planning_off` means nothing was scored; stop and tell the operator.
6. Save as a draft. Do not call `publish_plan` unless the operator asks in this conversation. Do not call `set_lesson_visibility` to share it. Leave the lesson hidden. Do not call `set_lexicon_verified` or `set_ai_planning`. Both always return `forbidden` and write nothing. Marking a form verified is a person's act. The operator ticks the forms checklist; you do not.

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

Rules N13 through N19 live in ADR-0009 §7. Cite an id when a write is refused or warned. Do not keep a second copy of the caps here.

### Hard rules

- Never copy the profile, a username, a typed answer, or a transcript into a plan, a worksheet, a gloss, or a lexicon entry.
- Never mark a form verified, and never tell the operator that an unverified form has been checked.
- Reduce English, and reduce vowel points on text the learner has already met, only as `journey.stage` rises, and only in text you author. `script` and `foundations` keep a gloss and an English line under the Hebrew, and they keep vowel points. `building` shortens the English. `expanding` and `independent` lead with Hebrew and keep English only where the frame needs it. New words stay pointed. A recipe never turns the learner's vowel-point toggle off. That toggle stays theirs (ADR-0001 #5). `journey.vowels_off_share` is information, not an instruction to strip points.
- Do not add a timed root or binyan speed drill. An untimed `conjugation-table` in `drill` mode is allowed.
- Do not edit a lesson's `index.html`.

## Recipe cards

One card per recipe in ADR-0009 §7. The block order is guidance for you. The validator does not enforce that order except through the rule ids on the card. The shapes follow the operator's summary of lesson types. They are not quotations from a class sheet. Write original sentences.

The cards, including the research note on each step, are in [recipes.md](recipes.md):

| Recipe | Blocks, in order |
| --- | --- |
| `topic-conversation` | `spaced-review-checkpoint`, `introduction`, `short-dialogue`, `personal-question` |
| `review-worksheet` | `spaced-review-checkpoint`, `graded-story`, `short-dialogue`, `personal-question` |
| `verse-reading` | one `verse-reading` per verse or short range |
| `verb-workshop` | `explainer`, `conjugation-table` in `study`, one `conjugation-table` in `drill`, `personal-question` |
| `letters-and-vowels` | `letter-intro` per letter, then `spaced-review-checkpoint` |
| `guide` | `explainer` only |
