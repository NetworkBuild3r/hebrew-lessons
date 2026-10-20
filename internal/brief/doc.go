// Package brief builds the lesson brief: the word plan for the next lesson, chosen by
// code from the learner model. It implements INIT-015/SPEC-019 and docs/north-star.md
// section 7.1.
//
// The code picks the words and the AI writes the lesson. A Builder reads the learner
// model and returns a Brief with:
//
//   - new words: unseen words from the lowest band that still has any, themed words first,
//     never a struggling word. 5 to 8 words, or 3 to 4 while the learner knows fewer than
//     BeginnerKnownBelow words.
//   - struggle words: the top MaxStruggleWords struggling or regressed words by StruggleScore.
//     Each carries the skill that is failing, the likely cause, and a required use count of 3
//     or more.
//   - comfortable words: up to MaxComfortableWords known or solid words that carry the story,
//     themed ones first, then the most recently seen.
//   - skill focus: the weakest unlocked skill overall, and the activity types that train it
//     and each struggle word's failing skill.
//   - targets, the staged coverage tier for the learner's vocabulary size, and the newest
//     coach notes.
//
// # Payload
//
// The brief is stored in lesson_briefs.payload_json as a versioned document. "v" is
// Version, so a validator or client can check the shape it is reading. Plan is that
// document. Brief adds what the table keeps in its own columns: the id, the creation and
// expiry times, and the unit that used it. A change to the shape of Plan must raise Version.
//
// # Ids, scope, expiry, use
//
// A brief id is 128 random bits from crypto/rand, written as 22 base64url characters. It is
// never a counter, so an id cannot be guessed or counted. Get, MarkUsed and every other read
// is scoped by the learner's user id in the query itself, so one learner's request for another
// learner's id finds nothing, the same as an id that does not exist.
//
// A brief expires TTL after it was made (7 days). Get refuses an expired brief. MarkUsed
// records the unit that was built from a brief and works once: a second call returns ErrUsed,
// and the first unit stays. Get still returns a used brief, with UsedByUnitID set, so a
// validator can tell the brief was already used.
//
// # Determinism
//
// Selection never draws from a random source. Ties and shuffles are ordered by a SHA-256 of
// the seed, the user id, the UTC day from the injected clock, and the lexeme id. Two calls on
// the same day with no new evidence choose the same word sets, and a new day or new evidence
// can change them. All times come from the injected clock.
//
// # Limits
//
// The word counts, the theme length, the gloss and text lengths, and MaxPayloadBytes are
// constants. A payload over MaxPayloadBytes drops comfortable words from the least preferred
// end until it fits, and fails with ErrTooLarge if it still does not.
//
// Nothing in this package writes to any table except lesson_briefs (a new row by Build, and
// used_by_unit_id by MarkUsed). It reads coach_notes and the learner model.
package brief
