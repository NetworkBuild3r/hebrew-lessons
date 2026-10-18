# Recipe cards and tagging

Part of the Hebrew worksheet skill. The loop that calls these cards is [SKILL.md](SKILL.md). Provenance: `INIT-006/SPEC-019`, `INIT-011/SPEC-001`, `INIT-012/SPEC-008`.

The server enforces the validator. These cards are the order to write, not a second validator. Block order is guidance. N13 through N23 are ADR-0009 §7 and ADR-0014. Cite an id. Do not copy a class sheet, a transcript, or a profile into a plan.

The only sample account id is `example-learner`. It does not belong in a lesson body or a lexicon gloss.

## Lesson types

The teacher runs five lesson shapes. Each maps to one `recipe` value and a usual block order. Order is guidance; only the cited rule ids are enforced.

| Teacher type | `recipe` | Blocks in teaching order |
| --- | --- | --- |
| A. Topic conversation | `topic-conversation` | `spaced-review-checkpoint`, `introduction` (three groups of five, sentence frames, fill-in), `short-dialogue`, `personal-question`, `assignment` |
| B. Review worksheet | `review-worksheet` | `introduction` (with fill-in), `match` (**INIT-010, not held yet**), `exercise` (`kind: choose`), `exercise` (`kind: unscramble`), `exercise` (`kind: translate`), `graded-story` (two readings), `personal-question` or `assignment` |
| C. Bible verse | `verse-reading` | `verse-reading`, `explainer` or `conjugation-table` (grammar pivot), `exercise` (`kind: translate`) for numbers, `personal-question` |
| D. Verb workshop | `verb-workshop` | `explainer`, `conjugation-table` (`tense: present`), `conjugation-table` (`tense: past`), `conjugation-table` (`tense: future`), `conjugation-table` (`mode: drill`) |
| E. Short story and true or false | `review-worksheet` | `graded-story`, `exercise` (`kind: true-false`), `exercise` (`kind: choose`) |
| Any lesson (dictation after listening) | any recipe | `listening`, then `exercise` (`kind: dictation`) |

Type E uses the same `review-worksheet` recipe as B (D2). Do not add a sixth recipe for the story sheet.

## Using the teacher's class docs

Read class docs for structure and topics only. Write original Hebrew and English lines in the plan. Take every pointed form from the dictionary (`lookup_lexicon` / verified lexicon entries). Never copy a learner's typed answer or class-doc text into a plan: live class docs hold real learner errors and unreliable pointing (D8).

## topic-conversation

Warm-up, then about fifteen words in three groups, sentence frames, a speaking task, a personal question, and a writing task with optional homework. Write original lines. A speaking task about a picture is not supported.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `spaced-review-checkpoint` of `very_weak_words`, then `weak_words` | Spacing over massing is High (`RSCH-003` §1.1). N15 warns when `buildsOn` is non-empty and this block is not first. `RSCH-004` §3 rule 5. Due dates are not in the profile. |
| 2 | `introduction`, about fifteen words in three groups (`group` 1 to 3). The sentence frames are its one pattern. | N18 is the cap for this recipe: warn above eight new words or a group over five; block above fifteen. Product choice. No source sets either number. Pictures stay on introduction words only. |
| 3 | `short-dialogue` as a role-play or a this-or-that choice | Product choice, taken from the speaking task in this lesson type. |
| 4 | `personal-question` | Output is a strand condition, graded Low (`RSCH-004` §1.4). N16 is satisfied by either `personal-question` or `assignment`. The block stores no answer. |
| 5 | `assignment` as the in-class writing task; put optional homework lines in the same block (`homework` array, D4) | Same privacy model as `personal-question`. Requires `schemaVersion` `1.2.0`. |

N23 warns when a `topic-conversation` lesson has more than two `exercise` blocks. Extra drills belong on another recipe.

## review-worksheet

A review sheet: introduction with fill-in, optional column match (INIT-010), choose / unscramble / translate checks, two readings to compare, and a short writing task. Pick the right sentence is an `exercise` with `kind: choose`.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `introduction` with fill-in (word bank and cloze) | Same spacing note as `topic-conversation` when you open with `spaced-review-checkpoint` instead; N15 when `buildsOn` is set. |
| 2 | `match` | **INIT-010, not held yet.** Until INIT-010 SPEC-004 lands, leave column match for the teacher. |
| 3 | `exercise`, `kind: choose` | N20 blocks when `answer` is not one of `choices`. |
| 4 | `exercise`, `kind: unscramble` | N21 blocks fewer than three words or a bad `alt` order. |
| 5 | `exercise`, `kind: translate` | Unpointed `answer` and optional `accept[]`. |
| 6 | `graded-story` with two readings to compare | N3 applies. Run `check_text_coverage` and revise until `coverage` is at least 0.95 (`RSCH-004` §3 rule 1, Low). |
| 7 | `personal-question` or `assignment` | The short writing task. N16 is satisfied by either block. `assignment` holds optional homework lines (D4). Requires `schemaVersion` `1.2.0` for `exercise` and `assignment`. |

### Short story with true or false (type E)

Same `review-worksheet` recipe (D2), different block order:

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `graded-story` | N3 applies. Coverage at least 0.95. |
| 2 | `exercise`, `kind: true-false` | One statement per item; `answer` is boolean. |
| 3 | `exercise`, `kind: choose` | Follow-up recognition check. N20 applies. |

## verse-reading

One `verse-reading` block per verse or short range. Three to six new words on that block. Pointed text. Simple questions. One personal question.

| Step | What to write | Research note |
| --- | --- | --- |
| 1 | `ref` with `book`, `chapter`, `from`, `to`, and a non-empty `source` naming a public-domain pointed edition | N19 blocks when `source` is empty. Product choice. The reader must be able to find the text. Do not store a translation of the verse. |
| 2 | `newWords`, three to six, each pointed, each occurring again in the verse or its questions | N6 warns when a new word appears only once. N1 counts the new words. New words stay pointed. |
| 3 | `verse`, then `questions` (at most four) | First-language glosses beat target-language glosses (`RSCH-003` §1.2, Moderate). No other claim. Product choice. N3 does not apply to this block. The loop in SKILL.md still runs `check_text_coverage` and revises until `coverage` is at least 0.95 (`RSCH-004` §3 rule 1, Low). |
| 4 | `explainer` or `conjugation-table` after the verse — the modern-Hebrew grammar pivot is the core teaching | Focused instruction on form helps (`RSCH-003` §1.3, Moderate). |
| 5 | `exercise`, `kind: translate`, for number / date / time drills when the teacher asks (D9) | Author-written cues; no generator yet. |
| 6 | `personalQuestion` on the block, or a `personal-question` block after it | N16 informs when neither is present. Output is a strand condition, Low (`RSCH-004` §1.4). |

Whether the verse body itself is pointed is not a validator check. Drop points on that body only as `journey.stage` rises, and only in text you author. Do not change the learner's toggle (ADR-0001 #5).

## verb-workshop

Use this recipe when a grammar concept is `weak` or `very_weak`. Target the weakest of those. Use `present`, `past`, and `future` — one tense per table, same limits. `imperative` is still not in the `conjugation-table` registry.

A published `verb-workshop` lesson opens as a Look / Practise / Use studio for the learner (not a long scroll). Build the blocks below; the studio wires them:

| Studio tab | What the learner sees | Which blocks feed it |
| --- | --- | --- |
| Look | One verb at a time; past, present, and future side by side; root letters marked | `explainer` (collapsed under "How the patterns work") plus every `conjugation-table` in `study` mode. The live tables also pull the full paradigm from the lexicon for each `lemmaKey`. |
| Practise | 12-card interleaved rounds: pick, then type; one retry; table hidden until answered; spacing across days | Built automatically from those `lemmaKey`s and the lexicon forms. You do **not** author a separate practice screen. Draft (unverified) forms may be practised with a visible draft label; they do not send form signals until a person marks them verified. |
| Use | Sentence work once the forms come easily | `exercise`, `assignment`, `personal-question`, `compare` |

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `explainer` for the rule | Focused instruction on form helps (`RSCH-003` §1.3, Moderate). Explicit versus implicit is unresolved (Low). |
| 2 | `conjugation-table` in `study` mode for three or four verbs. One tense per table (`present`, then `past`, then `future`). `sentences` at most three; the shape to follow is one sentence and two questions per verb. Upsert every form into the lexicon first (`upsert_lexicon_entries`: `lemma_key`, `tense`, `cell`, `accept[]`, `root`, `binyan`). The studio Practise tab needs those rows — table cells alone are not enough. | Product choice, from this lesson type. N13 blocks a `formKey` that is not a stored form of `lemmaKey` in that tense and cell. N13 and N14 apply to `future` forms the same as `past`. `tense: future` requires `schemaVersion` `1.2.0`. |
| 3 | Optional: one `conjugation-table` in `drill` mode | Retrieval beats rereading (`RSCH-003` §1.1, Moderate). Corrective feedback works (`RSCH-003` §1.3, Moderate). Untimed. Only `verified` forms are drilled. An unverified form warns N14 and is not scored. You cannot mark it verified. The studio Practise tab already runs interleaved recall, so a drill table is optional; keep it when you want an in-lesson drill station under Use or on the fallback long scroll. No published comparison shows that a morphology drill speeds acquisition (`RSCH-001`, Low, an absence). |
| 4 | `personal-question` (and optional `exercise` / `assignment`) | N16 informs when the personal question is missing. Output is a strand condition, Low (`RSCH-004` §1.4). These blocks appear under Use. |

Do not add a timed root or binyan speed drill. After publish, the lesson appears on Lessons (plan rows) and on the Verbs practice screen (`/verbs/`) once `visibility` is open to the learner.

## letters-and-vowels

Use this recipe only when `journey.stage` is `script`. One `letter-intro` per letter, then a `spaced-review-checkpoint` of earlier letters.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `letter-intro`: `letter`, `name`, `sounds` (vowel, pointed form, sound), `findIn` (word and whether it contains the letter) | Script-first versus words-first is not settled for Hebrew (`RSCH-004` §1.5, Very Low). Product choice. There is no picture field. `findIn` words are not `newWords` and create no review rows. |
| 2 | `spaced-review-checkpoint` of letters already taught | Spacing over massing is High (`RSCH-003` §1.1). |

N7 leaves a `letters-and-vowels` lesson out of the language-focus count. N2 still applies when `buildsOn` is set. Follow the card. Do not add a picture.

## guide

`explainer` blocks only. Use it for a standalone reference, not as one turn of the four-recipe rotation. N2 does not apply. N7 leaves a `guide` out of the language-focus count.

A verb-decoding guide can cover the root, the binyan, the infinitive, present and past, and the traffic-light labels below. Write that page in your own words. Do not copy a class sheet.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `explainer` sections: heading, paragraphs, examples, an optional table, callouts | Focused instruction on form helps (`RSCH-003` §1.3, Moderate). A reference page is a product choice. Explicit versus implicit is unresolved (Low). |
| 2 | Further `explainer` blocks for the next part of the same reference | Same note. No `personal-question` is required. N16 does not name `guide`. |

Callout `tone` is `green`, `yellow`, `red`, or `info`. Show the tone in words or an icon as well as in color.

## worksheet-to-lesson

Turn a teacher's Google Doc worksheet into one lesson on a plan. Provenance: `INIT-011/SPEC-001`.

The AI reads the Doc with its own Drive tool. The app does not get a Drive integration. Do not invent a block the schema does not hold.

### Worksheet → activity

Teacher activity names map through `shared/admin/activity-map.js` (`blockTypeFor`). Use those names.

| What the teacher makes | Example | Activity (block via activity-map) |
| --- | --- | --- |
| Word bank, then complete the sentences | `02` Ex. 1, `03` Ex. 1 | Words (`introduction`, including its cloze) |
| Match column A to column B | `03` Ex. 2 | Match (`match`) — **not held until INIT-010 SPEC-004 lands.** Do not invent a match block. Leave the exercise for the teacher. |
| Choose the correct sentence | `02` Ex. 2 | Exercise (`exercise`, `kind: choose`) |
| True or false | story sheet | Exercise (`exercise`, `kind: true-false`) |
| Put words in order (unscramble) | review sheet | Exercise (`exercise`, `kind: unscramble`) |
| Translate into Hebrew | numbers, cues | Exercise (`exercise`, `kind: translate`) |
| Type what you hear (dictation) | after listening | Exercise (`exercise`, `kind: dictation`) |
| Write N sentences / homework | end of lesson | Assignment (`assignment`) |
| A short reading and questions | `02` Ex. 3–4, `03` Ex. 3 | Story (`graded-story` with questions) |
| Write about yourself / discussion prompts | `02` Ex. 5, `03` Ex. 5 | Writing (`personal-question`) or Assignment (`assignment`) when homework lines belong in the plan |
| A verse with glosses | Genesis series, `ישעיהו מ` | Verse (`verse-reading`) |
| Verb rows to chant | Drill sheet Day 2 | Pattern (`conjugation-table`) |
| English cue, say the Hebrew | Drill sheet Days 1, 3, 4, 6; CSV deck | Type it (`type-it`); Fluency (`fluency-pass`) for the timed round |
| A teacher note | Drill sheet rules | Notes (`explainer`) |
| A pasted list of Hebrew–English lines | CSV deck, `אוצר מילים` tables | Paste into the word bank (Words / plan word bank) |

### How to build

1. `create_plan` for a new plan, or `insert_lesson` (`insertLesson` behind `POST /api/plans/:id/lessons`) when adding a lesson to an open plan. Reuse is the same operation with a copy.
2. For each held exercise, `insert_block` with the activity's block type from the table above.
3. Remove the placeholder block after the first real block is in.
4. `list_plan_issues`, fix what you can, then stop. Leave publish for the teacher unless they asked you to publish.

### Worked example: `03 המשפחה שלי`

Drive file id `1P6GWVOUBdGf3Z4E7jS8li6yQlZ8uECJlHPT_8rJCHJs`. Map exercise by exercise:

| Exercise | What it is | What you write |
| --- | --- | --- |
| 1 | Complete the sentences with a word bank (אח, אחות, דוד, סבא) | Words (`introduction`, including its cloze) |
| 2 | Match column A to column B | **Leave for the teacher.** The schema has no `match` block until INIT-010 SPEC-004 lands. Do not invent one. |
| 3 | A short reading and questions | Story (`graded-story` with questions) |
| 4 | Read and draw a family tree | **Not held.** Do not invent a drawing block. Name this as the exercise the plan cannot hold. |
| 5 | Write about your family | Writing (`personal-question`). The block stores no answer. |

### What you leave for the teacher

- Match exercises, until INIT-010 SPEC-004 lands.
- Drawing tasks (for example `03` Ex. 4).
- Anything that needs a picture approval, a verified lexicon form, or a publish they did not ask for.
- After you build, they open the plan on the two teacher screens (plan `19:2304`, lesson `19:2592`) and fix what is wrong.

## How to write each exercise

Each `exercise` block needs `schemaVersion` `1.2.0`. One block, one `kind`, one to twelve items. Every item has an `id` slug.

### `choose`

| Field | Required |
| --- | --- |
| `prompt` | yes (`text200`) |
| `choices` | yes, 2–5 strings (`text120`) |
| `answer` | yes, must equal one of `choices` (N20) |
| `why` | no |

Typical N20 mistake: `answer` is not in `choices`.

```json
{
  "id": "ex-choose-shulchan",
  "type": "exercise",
  "kind": "choose",
  "heading": "Pick the correct sentence",
  "items": [
    {
      "id": "ch-1",
      "prompt": "Which sentence means \"There is a table in the kitchen\"?",
      "choices": [
        "בַּמִטְבָּח יֵשׁ שֻׁלְחָן.",
        "בַּמִטְבָּח יֵשׁ כֶּלֶב.",
        "בַּגִּנָּה יֵשׁ עֵץ."
      ],
      "answer": "בַּמִטְבָּח יֵשׁ שֻׁלְחָן.",
      "why": "שֻׁלְחָן is table; כֶּלֶב is dog; עֵץ is tree."
    }
  ]
}
```

### `true-false`

| Field | Required |
| --- | --- |
| `statement` | yes |
| `answer` | yes, boolean |
| `why` | no |

```json
{
  "id": "ex-tf-shemesh",
  "type": "exercise",
  "kind": "true-false",
  "items": [
    {
      "id": "tf-1",
      "statement": "הַשֶּׁמֶשׁ זוֹרַחַת בַּבֹּקֶר.",
      "answer": true,
      "why": "שֶׁמֶשׁ is the sun; בֹּקֶר is morning."
    }
  ]
}
```

### `unscramble`

| Field | Required |
| --- | --- |
| `words` | yes, 3–10 tokens in correct order |
| `english` | no |
| `alt` | no, up to 3 alternate orders; each must be a permutation of `words` and not identical to `words` (N21) |

Typical N21 mistakes: fewer than three words; an `alt` row that is not a permutation or matches `words`.

```json
{
  "id": "ex-unscramble-cafe",
  "type": "exercise",
  "kind": "unscramble",
  "items": [
    {
      "id": "us-1",
      "english": "We drink coffee in the café.",
      "words": ["אֲנַחְנוּ", "שׁוֹתִים", "קָפֶה", "בַּבֵּית", "קָפֶה"],
      "alt": [
        ["בַּבֵּית", "קָפֶה", "אֲנַחְנוּ", "שׁוֹתִים", "קָפֶה"]
      ]
    }
  ]
}
```

### `translate`

| Field | Required |
| --- | --- |
| `english` | yes |
| `answer` | yes, canonical unpointed Hebrew |
| `accept` | no, up to 8 unpointed alternates |

```json
{
  "id": "ex-translate-shalosh",
  "type": "exercise",
  "kind": "translate",
  "items": [
    {
      "id": "tr-1",
      "english": "three",
      "answer": "שלוש",
      "accept": ["שלש"]
    }
  ]
}
```

### `dictation`

| Field | Required |
| --- | --- |
| `audioPath` | yes (N22) |
| `answer` | yes, unpointed |
| `accept` | no |

Typical N22 mistake: missing `audioPath`. Use `make_word_audio` or an existing clip path; do not invent a path.

```json
{
  "id": "ex-dictation-mayim",
  "type": "exercise",
  "kind": "dictation",
  "items": [
    {
      "id": "di-1",
      "audioPath": "audio/sentences/vocab-1.mp3",
      "answer": "מַיִם",
      "accept": ["מיים"]
    }
  ]
}
```

## Tagging

Call `upsert_lexicon_entries` for every new word before `add_library_words`. A form names its lemma with `lemma_key`. The key is the pointed `exactKey`. Rows are stored with source `mcp`. The `verified` flag is ignored on this call.

`list_grammar_concepts` is the registry. Concept keys are `family:key`, for example `binyan:paal` and `cell:past.2ms`. On an entry, store the suffix in `binyan`, `root_class`, `tense`, and `cell`, not the `family:key` string.

### Part of speech

`pos` is one of `noun`, `verb`, `adjective`, `adverb`, `preposition`, `pronoun`, `conjunction`, `particle`, `numeral`, `other`.

### Root

`root` is two to four Hebrew letters joined by `.`, for example `ח.ש.ב`. No vowel points in the root.

### Binyan

`paal`, `nifal`, `piel`, `pual`, `hifil`, `hufal`, `hitpael`.

### Traffic-light root classes

The traffic light is a label on the guide, not a field of its own. Store `root_class` as the key.

| Class | Key | Light |
| --- | --- | --- |
| Regular | `regular` | green |
| Final he | `lamed-he` | yellow |
| Initial yod | `peh-yod` | red |
| Initial nun | `peh-nun` | red |
| Middle vav | `ayin-vav` | red |
| Other | `irregular` | red |

### Tense and cell

`tense` is `infinitive`, `present`, `past`, `future`, or `imperative`. Verb workshops and `conjugation-table` blocks use `present`, `past`, and `future` (one tense per table). `cell` must belong to that tense.

| Tense | Cell codes |
| --- | --- |
| `present` | `ms`, `fs`, `mp`, `fp` |
| `past` | `1s`, `2ms`, `2fs`, `3ms`, `3fs`, `1p`, `2mp`, `2fp`, `3p` |
| `future` | `1s`, `2ms`, `2fs`, `3ms`, `3fs`, `1p`, `2mp`, `2fp`, `3p` |
| `infinitive` | `inf` |

`ms` `fs` `mp` `fp` are masculine singular, feminine singular, masculine plural, feminine plural. `1s` is אני, `2ms` אתה, `2fs` את, `3ms` הוא, `3fs` היא, `1p` אנחנו, `2mp` אתם, `2fp` אתן, `3p` הם and הן. `inf` is the infinitive.

Only `imperative` has no cells in the `conjugation-table` registry. An `imperative` tag may sit on a lemma for tagging; do not invent imperative table cells.

### Plene and defective spellings

`accept[]` holds the unpointed spellings a learner may type. At most eight strings. Each is 1 to 60 characters. None may contain a character from U+0591 to U+05C7, so no vowel points and no cantillation. When a form is commonly typed both plene (with ו or י as a vowel letter) and defective (without that letter), list both. Do not list a pointed string. Do not list another lemma's spelling just because the letters overlap.

`gloss_en` is at most 120 characters. `translit` is at most 80 and is a pronunciation hint, never a scored answer. No text field contains `<` or `>`. A batch is at most 200 entries, all or nothing.
