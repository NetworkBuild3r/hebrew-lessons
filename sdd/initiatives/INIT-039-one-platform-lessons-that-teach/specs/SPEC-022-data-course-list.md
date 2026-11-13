# SPEC-022 · data: the course list, built from the frequency list and Wiktionary

## Metadata

```yaml
spec_id: SPEC-022
initiative_id: INIT-039
title: "The course list"
domain: data
status: complete
primary_prompt: none
complexity: high
complexity_reason: a deterministic builder over a 57 MB dictionary file, a fix list, and an embedded loader with its own entry check
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 6 hours
blocked_by: [SPEC-021]
blocks: [SPEC-023, SPEC-024]
```

## Description

`internal/speech/fdosh.tsv` holds the 5,000 most common lemmas in Hebrew subtitles (Pinto 2018). It has no English, no pointing, and no part of speech. It also has parser artifacts:

- **Pronouns folded into הוא.** הוא is rank 1 at 121 per 1,000 words. היא, אתה, and אנחנו are missing, and אני is rank 4791 at 10 per million.
- **One-letter prefixes listed as lemmas.** ה is rank 2, ל 4, ב 7, ש 9, ו 12, מ 17, and כ 66.
- **Split or wrong lemmas.** סדר at rank 27 comes from בסדר, which means okay. והיי at rank 55 is an artifact. כול at rank 14 is a spelling of כֹּל.

This spec writes a builder that turns the list into a course list, with one row per teachable word. A word's pointed spelling, English, part of speech, gender, and root and pattern for a verb come from the English Wiktionary Hebrew data, extracted by kaikki.org. Its inflected forms come from there too. A fix list, kept by hand, splits, drops, and corrects rows. No row is ready, meaning gradable, until a reviewer signs it. Signing is SPEC-023 and is not done here.

A check on 2026-11-12 found that the Wiktionary data has 98 of the top 100 list lemmas and 917 of the top 1,000. A pick by part-of-speech priority (below) gives a passing entry for about 75 of the top 100. Picking by machine gets some common words wrong, which is why the review exists. Under the priority rule, these came out wrong: ש as "Sh! Hush!", טוב as "well", איש as "anyone", and נכון as "correctly".

## Inputs

1. **The dictionary file. It is not committed.** Download https://kaikki.org/dictionary/Hebrew/kaikki.org-dictionary-Hebrew.jsonl (about 57 MB, one JSON object per line). The page says the file is deprecated. If it is gone, use the Hebrew file from https://kaikki.org/dictionary/rawdata.html, which has the same fields. Do not commit it. A copy may already exist at `%TEMP%\kaikki-he.jsonl` on Brian's machine. Record the download date in the license file.
2. `internal/speech/fdosh.tsv`, which is already in the repo.
3. `internal/speech/course_fixes.tsv`, which this spec creates.

### Fields the builder reads from each dictionary line

- `word`: the headword. It may be pointed.
- `pos`: one of noun, verb, adj, adv, prep, pron, conj, det, article, particle, intj, num, name, and others.
- `senses[]`: each sense has `glosses[]`, `tags[]`, and optionally `form_of` and `alt_of`.
- `head_templates[0].args`: `wv` (the pointed form), `head`, `g` (the gender, such as `m`, `f`, or `m-p`), `tr` (the transliteration), and `"1"` (the verb pattern code: `pa`, `nif`, `pi`, `pu`, `hif`, `huf`, `hit`).
- `forms[]`: each has `form` and `tags[]`. Tags include `canonical`, `romanization`, `present`, `past`, `future`, `table-tags`, `inflection-template`, and `error-*`.
- `etymology_templates[]`: the root is the entry named `he-rootbox`, at `args."1"`, such as `א־מ־ר`. About 811 of 3,720 verbs have one.

## What to build

### 1. `cmd/coursegen` (package main)

- `cmd/coursegen/main.go` has flags `-dict <path>` (required), `-list internal/speech/fdosh.tsv`, `-fixes internal/speech/course_fixes.tsv`, and `-out internal/speech/course.tsv`. It also has `-show <lemma>`, which prints every dictionary entry for that lemma, with pos, the pointed form, every sense's first gloss and tags, the root, and the pattern code, and writes nothing. SPEC-023 uses `-show`.
- `cmd/coursegen/build.go` holds pure functions. They take an `io.Reader` for the dictionary and structs for the list and fixes, so tests need no files except a fixture.
- Run with Docker as the repo already does, with a bind mount of the temp folder for the dictionary:
  `docker run --rm -v "${PWD}:/src" -v "${env:TEMP}:/dict" -v hebrew-gomod:/go/pkg/mod -v hebrew-gocache:/root/.cache/go-build -w /src golang:1.26 go run ./cmd/coursegen -dict /dict/kaikki-he.jsonl`

### 2. Rules, in this order. Each must have a test.

**The key.** `key(s)` is `units.Plain(units.Normalize(s))` with U+05C1 and U+05C2 (the shin and sin dots) also removed. A dictionary entry belongs to a list lemma when `key(word) == lemma`.

**Part of speech map.** noun→noun, verb→verb, adj→adjective, adv→adverb, prep→preposition, pron→pronoun, conj→conjunction, det→determiner, article→determiner, particle→particle, intj→interjection, num→numeral. Every other pos is skipped, including name, character, prefix, suffix, phrase, and root.

**Usable sense.** A sense is skipped when any of these holds:

- it has `form_of` or `alt_of`;
- its tags include `form-of`, `alt-of`, `obsolete`, `archaic`, or `misspelling`;
- its first gloss matches, ignoring case, `^(defective|alternative|archaic|obsolete|misspelling|excessive|plural of|feminine of|masculine of|construct|inflection of|form of|abbreviation of|initialism of|nonstandard|pausal|eye dialect|clipping of)`.

**Pick.** Among a lemma's entries with a mapped pos and at least one usable sense, take the first in this pos order: pronoun, preposition, conjunction, determiner, particle, adverb, verb, adjective, numeral, noun, interjection. Ties keep file order. Use the entry's first usable sense.

**Gloss.** Start from that sense's `glosses[0]`. Remove everything in parentheses. Cut at the first `;`, `:`, or `,`. Trim spaces and any trailing `.`. Collapse inner spaces. Lowercase the first letter, unless the gloss is `I` or starts with `I `.

**Pointed.** Use `args.wv`, else `args.head`, else the form tagged `canonical`, else `word`. Then apply `units.Normalize`.

**Transliteration.** Use `args.tr`, else the form tagged `romanization`.

**Gender, for nouns only.** The first letter of `args.g` when it is `m` or `f`. Otherwise leave it empty.

**Pattern, for verbs only.** pa→paal, nif→nifal, pi→piel, pu→pual, hif→hifil, huf→hufal, hit→hitpael. Any other value leaves it empty.

**Root, for verbs only.** Use the `he-rootbox` argument. If there is none, the pattern is paal, and `key(pointed)` has exactly 3 letters, the root is those 3 letters joined with U+05BE. Otherwise leave it empty.

**Present.** True when any form has the tag `present`.

**Forms.**

- Take `forms[].form` where no tag is `romanization`, `table-tags`, `inflection-template`, or `class`, and no tag starts with `error`.
- Keep only Hebrew letters, points, spaces, maqaf, and geresh.
- Store `key(form)`, deduplicated.
- Leave out the lemma itself.
- Keep at most 40, in file order.
- Join them with `|`.

**Fixes.** Read `course_fixes.tsv`. Its header is `rank	lemma	action	pointed	translit	gloss	pos	gender	root	binyan	reason	reviewed_by	reviewed_on`. A row matches a generated row by `rank` and `lemma`. The actions are:

- `keep`: no field change.
- `fix`: every non-empty column replaces the generated value. The source becomes `fix`.
- `drop`: the row is output with status `dropped`. A reason is required.
- `prefix`: the row is a prefix. The kind is `prefix` and the pos is `prefix`. The pointed, translit, and gloss come from the fix row.
- `split`: the generated row for that rank and lemma is replaced by one output row per `split` line, in file order. Each split line gives every field.

**Reviewed.** A row is reviewed when its fix row has an action other than `drop` and a non-empty `reviewed_by`. For a split, each split line carries its own `reviewed_by`.

**Status.**

- `dropped` for a drop.
- Otherwise run `CourseCheck` (section 4). A failure makes the row `draft`, with that check id.
- Otherwise a row that is not reviewed is `draft`, with check `review.missing`.
- Otherwise the row is `ready`.

After that, a ready row whose `key(pointed)` equals an earlier ready row's key becomes `draft`, with check `course.duplicate`.

**No dictionary entry.** A lemma with no pick is output as `draft`, with check `entry.nodict`, and empty fields. A fix row may still fill it.

**Order.** Rows are sorted by list rank. Split lines keep their file order inside their rank. `order` runs 1..N over every output row.

### 3. Files

**`internal/speech/course.tsv`** is generated and committed. Its header is `order	rank	lemma	pointed	translit	gloss	pos	gender	root	binyan	present	kind	forms	status	check	source	reviewed_by`. `present` is `1` or `0`. `kind` is `word` or `prefix`. `source` is `wiktionary`, `fix`, or `none`.

**`internal/speech/course_fixes.tsv`** is written by hand in this spec. Leave `reviewed_by` and `reviewed_on` empty on every row: the author of this spec does not sign. Write these rows:

- `prefix`:
  - rank 2 ה → `הַ`, `ha`, `the`
  - rank 4 ל → `לְ`, `le`, `to`
  - rank 7 ב → `בְּ`, `be`, `in`
  - rank 9 ש → `שֶׁ`, `she`, `that`
  - rank 12 ו → `וְ`, `ve`, `and`
  - rank 17 מ → `מִ`, `mi`, `from`
  - rank 66 כ → `כְּ`, `ke`, `like`
- `split` for rank 1 הוא, as seven lines. Each line gives pointed, translit, gloss, pos, and gender:
  - `אֲנִי`, `ani`, `I`, pronoun
  - `אַתָּה`, `ata`, `you (m.)`, pronoun
  - `הוּא`, `hu`, `he`, pronoun
  - `הִיא`, `hi`, `she`, pronoun
  - `אֲנַחְנוּ`, `anakhnu`, `we`, pronoun
  - `אַתֶּם`, `atem`, `you (pl.)`, pronoun
  - `הֵם`, `hem`, `they`, pronoun
  The reason on each line is `the list's parser folded personal pronouns into הוא`.
- `split` for rank 3 את, as two lines:
  - `אֶת`, `et`, `object marker`, particle
  - `אַתְּ`, `at`, `you (f.)`, pronoun
  The second line becomes `course.duplicate` by the rule above. That is expected, and its reason says so.
- `fix`:
  - rank 14 כול → `כֹּל`, `kol`, `all`, determiner
  - rank 21 טוב → `טוֹב`, `tov`, `good`, adjective
  - rank 27 סדר → `בְּסֵדֶר`, `beseder`, `okay`, interjection, with reason `the parser split בסדר`
  - rank 34 איש → `אִישׁ`, `ish`, `man`, noun, m
  - rank 46 זאת → `זֹאת`, `zot`, `this (f.)`, pronoun
  - rank 49 נכון → `נָכוֹן`, `nakhon`, `right`, adjective
- `drop` rank 55 והיי, reason `parser artifact`.

**`internal/speech/LICENSE-wiktionary.txt`**: the data comes from English Wiktionary, contributors, CC BY-SA 4.0, https://creativecommons.org/licenses/by-sa/4.0/. It was extracted by kaikki.org (Tatu Ylonen, wiktextract). Give the download URL and date, and say that `course.tsv` is a derived work under the same license.

**`internal/speech/doc.go`**: add one paragraph naming the course file, its two sources, the review rule, and the license file.

### 4. `internal/speech/course.go`

- `//go:embed course.tsv`
- `type CourseWord struct { Order, Rank int; Lemma, Pointed, Translit, Gloss, POS, Gender, Root, Binyan string; Present bool; Kind string; Forms []string; Status, Check, Source, ReviewedBy string }`
- `func Course() ([]CourseWord, error)` loads once, like `entries()` in `list.go`.
- `func CourseReady() ([]CourseWord, error)` returns only `Status == "ready"`, in order.
- `func CourseCheck(w CourseWord) string` returns the first failing check id, or "" when the word passes:
  - `entry.gloss`: the gloss is not 1 to 4 words after trimming, or it contains a Hebrew letter, `<`, or `>`.
  - `entry.pos`: the pos is not one of noun, verb, adjective, adverb, preposition, pronoun, conjunction, determiner, particle, interjection, numeral, prefix.
  - `entry.gender`: a noun's gender is not m or f.
  - `entry.verb`: a verb has no root, no pattern, or no present form.
  - `entry.points`: the pointed form has no point in U+05B0–U+05BC or U+05C7.
  - `entry.translit`: the transliteration is empty.

  The generator and the loader must share this one function. The generator imports `internal/speech`.

## Tests

In `cmd/coursegen/build_test.go`, with a fixture `cmd/coursegen/testdata/dict.jsonl` of about 12 entries copied from the real file (entries for אם, עם, הלך, ידע, טוב, ש, אמר, בית, ילד, היה, and one name), plus fixture list and fixes:

- The pos priority picks אִם "if" over אֵם "mother", and עִם "with" over עַם "nation".
- Gloss shaping turns "With: used to indicate togetherness" into "with", and "To know ." into "to know".
- A sense that is "defective spelling of …" is skipped.
- A paal verb with no root box gets its 3-letter root. A piel verb with no root box is `draft` with `entry.verb`.
- `split`, `prefix`, `fix`, and `drop` each produce the rows described. A split line with no `reviewed_by` is `draft` with `review.missing`.
- Two ready rows with the same key: the later one is `course.duplicate`.
- Running the build twice on the same input gives the same bytes.

In `internal/speech/course_test.go`, on the real embedded file:

- It has a row for every list rank, either one row or the split rows.
- `order` is 1..N with no gap.
- Every `ready` row passes `CourseCheck` and has `reviewed_by`.
- Every `dropped` row has a reason in the fixes file. Check it by reading `course_fixes.tsv` in the test with `os.ReadFile`.
- Rank 1 is the seven pronoun rows.
- No ready row has `Kind == "prefix"` without pos `prefix`.
- `LICENSE-wiktionary.txt` exists and names CC BY-SA 4.0.

## Acceptance criteria

- [ ] `go run ./cmd/coursegen -dict …` writes `internal/speech/course.tsv`, and a second run writes the same bytes.
- [ ] The fix rows listed in section 3 exist, unsigned.
- [ ] Before SPEC-023, every non-dropped row is `draft`, because nothing is signed yet. Every test above passes anyway.
- [ ] The license file and the doc paragraph exist.
- [ ] `go test ./cmd/coursegen/ ./internal/speech/` passes. `go vet ./...` is clean.

## Verification Strategy

- **Claim:** the builder is deterministic, picks by the stated rules, and marks nothing ready without a signature.
- **Check + executor:** mechanized. `go test ./cmd/coursegen/ ./internal/speech/` in Docker `golang:1.26`.
- **Pass condition:** the tests pass, and `git status` shows `course.tsv` unchanged after a second generator run.

## Out of scope

Writing to the database (SPEC-024). Signing rows (SPEC-023). Sentences or example lines.
