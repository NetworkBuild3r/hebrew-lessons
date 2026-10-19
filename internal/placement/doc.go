// Package placement deals an adaptive vocabulary deck.
//
// The learner sees a pointed Hebrew word and its audio, recalls it, flips
// the card, and rates Again, Hard, Good, or Easy. The back of the card is
// the gloss, the transliteration, and the gender. TextNFC and AudioURL are
// the front. GlossEN, Translit, and Gender are the back. They are returned
// together. AudioURL is /audio/<id>.mp3 when a clip exists for the exact
// pointed text, and empty otherwise.
//
// Each rating is that word's first real FSRS review. Rate creates a he_en
// card and grades it through fsrs.Review.Grade, so the initial stability
// is the one FSRS-5 assigns a new card. There is no separate seeding step.
// A word that already has a he_en card is skipped. Known and learning are
// read from that card: the latest rating is Good or Easy when the word is
// known, and Again or Hard when it is learning. Times come from the
// clock.Clock passed to New.
//
// Bands 0–4 are real words, at least ten in each band. The deck plays band
// 0 first, then bands 1–4. Each band is shuffled from the seed stored on
// the run. Once a band has 10 ratings, an Again rate of 0.60 or more stops
// that band and every band above it, and the run ends. Hard does not count.
// Exactly 0.60 stops. The run also stops after 100 rated cards. Start
// returns the learner's unfinished run when there is one, and Next
// continues after the last answered card. Finish is idempotent. A partial
// run the learner finishes early is a valid placement.
//
// Level is the highest band whose dealable words were all rated without
// tripping the struggle rule, or −1. EstimatedKnownWords sums, over bands
// 1–4, (Good + Easy) / Rated times the band's width in ranks (500, 500,
// 1000, 2000), then adds the count of band 0 words rated Good or Easy.
// SuggestedStartPosition is filled by Finish, not by Score: the lowest
// units.position whose doc newWords are less than 80% covered by the
// learner's known set, one past the highest position when every unit is
// covered, and 0 when there are no units. The known set is KnownLexemeIDs:
// he_en cards whose latest rating is Good or Easy, plus new words of
// lessons this learner has finished. An empty newWords list is covered.
// A latest Again or Hard does not count. The position is recomputed on read.
//
// The deck API on *Service, reached from app.App.Placement, is:
//
//	Start(ctx, userID) (runID, error)
//	Next(ctx, runID) (*Card, done bool, error)
//	Rate(ctx, runID, itemID, rating) error
//	Finish(ctx, runID) (Result, error)
//
// Score(tallies) Result is the shared struggle rule.
//
// MCP Builder, for get_context, reads these store methods.
//
//	(*db.Store).LatestPlacement(ctx, userID) (db.Placement, bool, error)
//	  The newest run, ordered by id. ok is false when the learner has none.
//	  db.Placement carries TakenAt, Finished, Level, CardsRated, StopReason
//	  (band_stop, cap, exhausted, or empty), BandScores (rated, again, hard,
//	  good, easy, struggleRate), EstimatedKnownWords, and
//	  SuggestedStartPosition.
//
//	(*db.Store).PlacementKnownLexemes(ctx, userID) ([]string, error)
//	(*db.Store).PlacementLearningLexemes(ctx, userID) ([]string, error)
//	  Pointed NFC text from the newest run's placement answers, sorted.
//	  Good or Easy is known. Again or Hard is learning.
//
//	(*db.Store).KnownLexemeIDs(ctx, userID) ([]int64, error)
//	  Sorted card-based known set described above.
//
//	(*db.Store).StruggleStats(ctx, userID, topN) (db.Struggle, error)
//	  Lexemes is every reviewed lexeme sorted by text, with ConfusionKeys.
//	  Top is the first topN by misses in the last 7 days, lapses, then the
//	  Again rate. topN ≤ 0 leaves Top empty. Confusions are ConfusionPairs
//	  from the last 7 days on the store clock.
//
//	(*db.Store).CompletedSharedUnits(ctx, userID) ([]db.UnitCompletion, error)
//	(*db.Store).UnitsInScope(ctx, learnerID) ([]db.UnitRow, error)
//
// units.Lookup.KnownLexemes(ctx, beforePosition) is the global authoring
// view: words introduced as new in an earlier unit. It is a different set,
// and this package does not implement Lookup.
package placement
