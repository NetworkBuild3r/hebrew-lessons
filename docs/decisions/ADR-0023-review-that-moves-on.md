# ADR-0023: Review that moves on when you know it

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-01 |
| **Authored by** | INIT-027 SPEC-001 |
| **Initiative** | INIT-027 (adaptive-review), SPEC-001 |
| **Extends** | [ADR-0020](ADR-0020-lesson-word-face-and-speed.md) (speed rule), [ADR-0022](ADR-0022-learner-day-and-outside-teaching.md) (learner day) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A review sitting asks due cards and new cards only. Letter drills are timed picks that end after three right answers in a row. Typing slips log only look-alike or sound-alike pairs. A strong word moves up to a harder skill, and a right answer there schedules the due easier card as Good with an implied log row. New words follow the last fifty non-implied reviews, and at the retention target the sitting seeds the next placement-list words. The sitting follows the learner's study day.

## Context

ADR-0020 holds the speed rule for checked answers. ADR-0022 holds the learner day and the 4:00 study-day rollover. Neither covers letter drills that never end, typing slips that invent unlike pairs, filling a sitting with known cards that are not due, asking a due card only through its weakest skill, or capping new words without regard to accuracy. RSCH-003 supports spacing and longer gaps for delayed retention, and checklist line 173: count a same-session repeat as practice, not as the review. RSCH-003 has no evidence on answer speed.

## Decision

**D1 · Drills end.** A letter drill is a timed two-option pick of the letter in a real word. Pattern drills stay self-graded, and Good or Easy is right. Every drill answer is stored. Three right in a row since the pair's last miss, none slower than twice the learner's drill median once ten timed drill answers exist, clears the pair. The median is the middle of the last 50 timed right drill answers. Only misses after the latest clear count toward a new drill. A pair gets three steps; a sitting gets at most six drill steps. Rejected: a self-graded letter drill that never clears, and rejected: four steps per pair up to twenty per sitting.

**D2 · Typing slips.** A typed answer logs a letter pair only for look-alike or sound-alike pairs. Look-alike is the drill cue table in `internal/web/views/alefbet.go` with points removed, plus ו/י. Sound-alike is א/ע, ט/ת, כּ/ק, ח/כ, ב/ו, שׂ/ס. A letter-pair key stores base letters only (U+05D0 through U+05EA), so כּ/ק is stored as כ/ק and שׂ/ס as ס/ש. A wrong pick of a one-letter-off word still logs the pair. Rejected: logging every substituted letter within two edits, including unlike slips such as ו/מ.

**D3 · Due cards only.** A session queues due cards and new cards. A known card that is not due is never added to fill the known share. Same-session repeats are practice, not the review (RSCH-003 checklist line 173). Longer gaps help delayed retention (RSCH-003, Moderate). Rejected: padding the known quota with cards that are not due yet.

**D4 · Move up, with implied credit.** A due card is asked through its own skill, or through the skill that implies it (recall implies read, spell implies listen) when its own skill is known or solid and the implying card has no Again or Hard in seven days. A right implying answer schedules the due card Good, never Easy, with `review_log.implied = 1` and no attempts row. Accuracy counts skip implied rows. Rejected: always asking the weakest skill, and rejected: grading only the skill that was shown while the due card stays due.

**D5 · New words follow accuracy.** The new-card cap is 2 below 80%, 5 from 80% to 90%, and 8 above 90%, from the last fifty non-implied reviews. At or above the 85% retention target, when fewer `new` cards wait than the cap, review seeds verified placement-list words with a gloss, lowest band first, in list order, as read and recall cards plus listen with a clip. Rejected: a fixed share of the planned queue size with no accuracy tiers, and rejected: review that cannot introduce a word no lesson taught.

**D6 · The review sitting follows the learner's study day.** The queue and its answers last until the 4:00 rollover in the learner's zone ([ADR-0022](ADR-0022-learner-day-and-outside-teaching.md)), not the UTC date. Rejected: rebuilding the queue and refusing answers when the UTC date changes.

## Consequences

- North Star sections 4.2, 4.5, and 6 follow this record.
- Sessions can be shorter than twenty when due known cards run short; that is intended.
- ADR-0020's speed rule for checked answers is unchanged.
- RSCH-003 has no speed evidence, so D1's slow cut on drill answers is a product rule, as in ADR-0020.
- Later code specs (INIT-027) implement the drill store, mix, move-up, new-word tiers, and study-day queue against this record.
