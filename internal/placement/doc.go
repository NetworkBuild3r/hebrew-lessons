// Package placement deals an adaptive vocabulary deck.
//
// The learner sees a pointed Hebrew word, recalls it, flips it, and rates
// Again, Hard, Good, or Easy. Before the flip, every card also offers
// "I don't know this", including invented words, so a pseudo-word is not
// given away by the absence of that button. Gloss and audio travel on the
// card. The client shows them after the flip. AudioURL is /audio/<id>.mp3
// when a clip exists for the exact pointed text, and empty otherwise.
// Whether the word was invented is returned only from Rate.
//
// Times come from the clock.Clock passed to New. The same clock drives the
// FSRS review that a reliable finish writes.
//
// Bands 0–4 are real words: 12 from the tutor's lessons (band 0) and 12
// from each frequency band (bands 1–4). Twenty items are pseudo-words.
// The deck starts at band 0. Each band deals up to 16 real words and 4
// pseudo-words, shuffled from the seed stored on the run. After a band's
// block, a Good-or-better rate under 50% on its real words stops the deck.
// Exactly 50% continues. The deck also stops at 100 cards, or 5 minutes
// after started_at. The next card after a cap returns done. An unanswered
// card already dealt is still returned so it can be rated.
//
// Scoring lives in Score(answers, items) Result, so a UI and MCP apply the
// same rule. A pseudo-word rated Good or Easy is a false alarm. Hard on a
// pseudo-word is only recorded. f is false alarms divided by pseudo-words
// shown. Above 0.25 the run is unreliable: it is stored, and it seeds
// nothing. Level is the highest band whose Good-or-better rate is at least
// 0.5, or −1.
//
// EstimatedKnownWords sums, over bands 1–4, the Good-or-better rate times
// the band's width in ranks (500, 500, 1000, 2000), then adds the count of
// band 0 words rated Good or Easy. SuggestedStartPosition is filled by
// Finish, not by Score: the lowest units.position whose doc newWords are
// less than 80% covered by the learner's known set, one past the highest
// position when every unit is covered, and 0 when there are no units. The
// known set is the KnownLexemeIDs union. An empty newWords list is covered.
// Learning rows do not count toward coverage.
//
// A reliable finish, for each rated real word, upserts the lexeme with
// verified 0 and writes a he_en card. That card is graded through
// fsrs.Review.Grade, so the placement rating is the card's first review
// and a review_log row. "I don't know" is graded Again. learner_lexemes
// stores Good or Easy as known, and Hard, Again, or "I don't know" as
// learning, with seeded 1 and source placement. A word that already has a
// he_en card is skipped, so a retake does not grade it twice. The latest
// finished run sets the level. Invented words are never written to lexemes
// or cards.
//
// The deck API on *Service, reached from app.App.Placement, is:
//
//	Start(ctx, userID) (runID, error)
//	Next(ctx, runID) (*Card, done bool, error)
//	Rate(ctx, runID, itemID, rating) (Rated, error)
//	Finish(ctx, runID) (Result, error)
//
// MCP Builder, for get_context, reads these store methods.
//
//	(*db.Store).LatestPlacement(ctx, userID) (db.Placement, bool, error)
//	  The newest finished run, ordered by finished_at then id.
//	  ok is false when the learner has not finished a run.
//	  db.Placement carries TakenAt, Reliable, FalseAlarmRate, Level,
//	  BandScores (shown, again, hard, good, easy, dontknow, goodPlusRate),
//	  EstimatedKnownWords, and SuggestedStartPosition.
//	  SuggestedStartPosition is computed from the units and known words now.
//
//	(*db.Store).PlacementKnownLexemes(ctx, userID) ([]string, error)
//	(*db.Store).PlacementLearningLexemes(ctx, userID) ([]string, error)
//	  Pointed NFC text of seeded learner_lexemes in that status, sorted.
//	  A word taught only by a finished lesson is not included.
//	  units.Lookup.KnownLexemes(ctx, beforePosition) is the global authoring
//	  view: words introduced as new in an earlier unit. It is a different set,
//	  and this package does not implement Lookup.
package placement
