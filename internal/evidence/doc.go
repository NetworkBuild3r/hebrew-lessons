// Package evidence is the one place a checked answer is written.
//
// Every screen that grades an answer calls Recorder.Attempt. It writes the
// attempts row, turns the attempt into an FSRS rating with Grade, moves the
// learner's card for that word and skill forward, and logs a confusion when the
// learner picked another word. All of that is one transaction: an attempt has
// its attempts row and its review_log row, or neither.
//
// Grade is the one grading rule (north-star section 4.2):
//
//   - wrong, or "I don't know": Again
//   - right after a hint: Hard
//   - right and faster than the learner's own median for that activity: Easy,
//     once 10 or more timed correct attempts exist
//   - right and slower than twice that median: Hard, so the word comes back sooner
//   - right: Good
//
// Placement (source = placement) follows ADR-0018 D2. A right answer seeds that
// skill's card as Good. A wrong answer or "I don't know" is stored as an attempt
// and creates no card. A word that already has a card is not reviewed again.
//
// Review and practice attempts create the card if the learner has none. An
// attempt with SkipSchedule set is stored and touches no card. A lesson's meaning
// question is timed and does schedule. The other lesson checks still wait, and
// the lesson still gives its new words cards when it ends. When Grade says Easy
// and the card is missing, new, or still in its first steps, the recorder stores
// Good instead, so a word just taught is not sent weeks away. Easy waits until
// the card is in review or relearning. ADR-0020.
//
// Recorder.SelfGrade keeps the flashcard path: the learner picks the rating, and
// the attempt is stored with activity flashcard.
//
// Recorder.Expose writes exposures: words shown with no question asked.
//
// The package depends on db, fsrs, and clock. It does not import session.
// Time comes from the clock, so tests are deterministic. The learner id is a
// field of Attempt that handlers fill from the signed-in session, never from a
// form. Response text is cut to 200 characters before it is stored.
//
// Provenance: INIT-015/SPEC-005.
package evidence
