# Review of the first 500 course words

Reviewer: grok-4.7, on 2026-11-12. This session did not build the course list.

The course has 507 rows with list rank 1 to 500. Rank 1 is the seven pronouns, and rank 3 is two rows, so the row count is higher than 500.

## Counts

| action | lines | signed |
| --- | ---: | ---: |
| keep | 257 | 254 |
| fix | 206 | 206 |
| drop | 28 | 0 |
| prefix | 7 | 7 |
| split | 9 | 8 |

The one unsigned split is אַתְּ "you (f.)". Three keep lines are unsigned. Drops are not signed.

In the generated course, those lines come out as:

- ready: 475
- dropped: 28
- unsigned: 4

475 ready is the 254 signed keeps, 206 fixes, 7 prefixes, and 8 signed splits.

## Unsigned

| rank | lemma | why it stays unsigned |
| --- | --- | --- |
| 3 | את | אַתְּ "you (f.)" stays `course.duplicate`. It is the same spelling as the object marker. |
| 59 | נראה | Everyday sense is "to seem". The verb entry has no present form, so it cannot pass `entry.verb`. |
| 132 | אי | Island, the prefix "non-", and the interjection "oh" are all plausible. The conjunction "if" is archaic. |
| 468 | שה | Wiktionary only has the biblical lamb. This token may be a fragment. |

## Not in Wiktionary senses

The right everyday meaning was not among the senses for that lemma. Each of these is a signed fix.

| rank | lemma | gloss | reason |
| --- | --- | --- | --- |
| 93 | עצמו | himself | not in Wiktionary senses: reflexive himself |
| 122 | כש | when | not in Wiktionary senses: the extract lists the conjunction only as כש־ |
| 154 | אה | uh | not in Wiktionary senses: filler uh |
| 197 | הו | oh | not in Wiktionary senses: interjection oh |
| 224 | אוקיי | okay | not in Wiktionary senses: loanword okay |
| 229 | מעולם | ever | not in Wiktionary senses: used as ever, mostly in מעולם לא |
| 378 | לשם | to there | not in Wiktionary senses: to that place; the listed stone is biblical |
| 386 | הכול | everything | not in Wiktionary senses: everything, the article plus כל |
| 495 | פשע | crime | not in Wiktionary senses: the noun is glossed felony; speech means a crime |

## Check

`go test ./cmd/coursegen/ ./internal/speech/` passed in Docker `golang:1.26` after `coursegen` regenerated `course.tsv`.
