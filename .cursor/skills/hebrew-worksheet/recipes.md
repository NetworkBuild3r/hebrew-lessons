# Recipe cards and tagging

Part of the Hebrew worksheet skill. The loop that calls these cards is [SKILL.md](SKILL.md). Provenance: `INIT-006/SPEC-019`.

The server enforces the validator. These cards are the order to write, not a second validator. Block order is guidance. N13 through N19 are ADR-0009 §7. Cite an id. Do not copy a class sheet, a transcript, or a profile into a plan.

The only sample account id is `example-learner`. It does not belong in a lesson body or a lexicon gloss.

## topic-conversation

Warm-up, then about fifteen words in three groups, sentence frames, a speaking task, and a personal question. Write original lines. A speaking task about a picture is not supported.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `spaced-review-checkpoint` of `very_weak_words`, then `weak_words` | Spacing over massing is High (`RSCH-003` §1.1). N15 warns when `buildsOn` is non-empty and this block is not first. `RSCH-004` §3 rule 5. Due dates are not in the profile. |
| 2 | `introduction`, about fifteen words in three groups (`group` 1 to 3). The sentence frames are its one pattern. | N18 is the cap for this recipe: warn above eight new words or a group over five; block above fifteen. Product choice. No source sets either number. Pictures stay on introduction words only. |
| 3 | `short-dialogue` as a role-play or a this-or-that choice | Product choice, taken from the speaking task in this lesson type. |
| 4 | `personal-question` | Output is a strand condition, graded Low (`RSCH-004` §1.4). N16 informs when this recipe has no `personal-question`. The block stores no answer. |

## review-worksheet

A review sheet: a word-bank match, two readings to compare, a short dialogue, and a short writing task. "Pick the right sentence" has no block in schema `1.1.0`. Do not invent a block for it.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `spaced-review-checkpoint` as the word-bank match | Same spacing note as `topic-conversation` step 1. N15 when `buildsOn` is set. |
| 2 | `graded-story` with two readings to compare | N3 applies. Run `check_text_coverage` and revise until `coverage` is at least 0.95 (`RSCH-004` §3 rule 1, Low). |
| 3 | `short-dialogue` | Product choice. The sheet's shape includes a short exchange after the readings. |
| 4 | `personal-question` with `answer` set to `type` | The short writing task. Output is a strand condition, Low (`RSCH-004` §1.4). The answer is not sent to the server. |

## verse-reading

One `verse-reading` block per verse or short range. Three to six new words on that block. Pointed text. Simple questions. One personal question.

| Step | What to write | Research note |
| --- | --- | --- |
| 1 | `ref` with `book`, `chapter`, `from`, `to`, and a non-empty `source` naming a public-domain pointed edition | N19 blocks when `source` is empty. Product choice. The reader must be able to find the text. Do not store a translation of the verse. |
| 2 | `newWords`, three to six, each pointed, each occurring again in the verse or its questions | N6 warns when a new word appears only once. N1 counts the new words. New words stay pointed. |
| 3 | `verse`, then `questions` (at most four) | First-language glosses beat target-language glosses (`RSCH-003` §1.2, Moderate). No other claim. Product choice. N3 does not apply to this block. The loop in SKILL.md still runs `check_text_coverage` and revises until `coverage` is at least 0.95 (`RSCH-004` §3 rule 1, Low). |
| 4 | `personalQuestion` on the block, or a `personal-question` block after it | N16 informs when neither is present. Output is a strand condition, Low (`RSCH-004` §1.4). |

Whether the verse body itself is pointed is not a validator check. Drop points on that body only as `journey.stage` rises, and only in text you author. Do not change the learner's toggle (ADR-0001 #5).

## verb-workshop

Use this recipe when a grammar concept is `weak` or `very_weak`. Target the weakest of those. Present and past only. Future-tense and imperative cells are not in the registry.

| Step | Block | Research note |
| --- | --- | --- |
| 1 | `explainer` for the rule | Focused instruction on form helps (`RSCH-003` §1.3, Moderate). Explicit versus implicit is unresolved (Low). |
| 2 | `conjugation-table` in `study` mode for three or four verbs. One tense per table. `sentences` at most three; the shape to follow is one sentence and two questions per verb. | Product choice, from this lesson type. N13 blocks a `formKey` that is not a stored form of `lemmaKey` in that tense and cell. |
| 3 | One `conjugation-table` in `drill` mode | Retrieval beats rereading (`RSCH-003` §1.1, Moderate). Corrective feedback works (`RSCH-003` §1.3, Moderate). Untimed. Only `verified` forms are drilled. An unverified form warns N14 and is not scored. You cannot mark it verified. No published comparison shows that a morphology drill speeds acquisition (`RSCH-001`, Low, an absence). |
| 4 | `personal-question` | N16 informs when it is missing. Output is a strand condition, Low (`RSCH-004` §1.4). |

Do not add a timed root or binyan speed drill.

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

`tense` is `infinitive`, `present`, `past`, `future`, or `imperative`. This skill's workshops and starter forms use `present`, `past`, and `infinitive` only. `cell` must belong to that tense.

| Tense | Cell codes |
| --- | --- |
| `present` | `ms`, `fs`, `mp`, `fp` |
| `past` | `1s`, `2ms`, `2fs`, `3ms`, `3fs`, `1p`, `2mp`, `2fp`, `3p` |
| `infinitive` | `inf` |

`ms` `fs` `mp` `fp` are masculine singular, feminine singular, masculine plural, feminine plural. `1s` is אני, `2ms` אתה, `2fs` את, `3ms` הוא, `3fs` היא, `1p` אנחנו, `2mp` אתם, `2fp` אתן, `3p` הם and הן. `inf` is the infinitive.

A `future` or `imperative` tense tag may sit on a lemma. Do not invent cells for those tenses. They are not in the registry.

### Plene and defective spellings

`accept[]` holds the unpointed spellings a learner may type. At most eight strings. Each is 1 to 60 characters. None may contain a character from U+0591 to U+05C7, so no vowel points and no cantillation. When a form is commonly typed both plene (with ו or י as a vowel letter) and defective (without that letter), list both. Do not list a pointed string. Do not list another lemma's spelling just because the letters overlap.

`gloss_en` is at most 120 characters. `translit` is at most 80 and is a pronunciation hint, never a scored answer. No text field contains `<` or `>`. A batch is at most 200 entries, all or nothing.
