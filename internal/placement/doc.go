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
// A word that already has a he_en card is skipped. Good or Easy is stored
// in learner_lexemes as known. Again or Hard is learning. The source is
// placement. Times come from the clock.Clock passed to New.
//
// Bands 0–4 are real words, at least ten in each band. The deck plays band
// 0 first, then bands 1–4. Each band is shuffled from the seed stored on
// the run. Once a band has 10 ratings, a struggle rate (Again + Hard) /
// Rated of 0.60 or more stops that band and every band above it, and the
// run ends. Exactly 0.60 stops. The run also stops after 80 rated cards.
// Start returns the learner's unfinished run when there is one, and Next
// continues after the last answered card. Finish is idempotent.
//
// Level is the highest band whose dealable words were all rated without
// tripping the struggle rule, or −1. EstimatedKnownWords sums, over bands
// 1–4, (Good + Easy) / Rated times the band's width in ranks (500, 500,
// 1000, 2000), then adds the count of band 0 words rated Good or Easy.
// SuggestedStartPosition is filled by Finish, not by Score: the lowest
// units.position whose doc newWords are less than 80% covered by the
// learner's known set, one past the highest position when every unit is
// covered, and 0 when there are no units. The known set is the
// KnownLexemeIDs union. An empty newWords list is covered. Learning rows
// do not count toward coverage. The position is recomputed on read.
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
//	  Pointed NFC text of seeded learner_lexemes in that status, sorted.
//
//	(*db.Store).LexemeStruggles(ctx, userID, topN) ([]db.LexemeStruggle, error)
//	  Reviewed lexemes sorted by misses in the last 7 days, lapses, the
//	  Again rate, then text. topN ≤ 0 returns all of them. The 7 days are
//	  measured on the store clock.
//
//	(*db.Store).ConfusionCounts(ctx, userID, window) ([]db.ConfusionCount, error)
//	  confusion_misses inside the window, with Kind, Key, Count, and LastAt.
//
// units.Lookup.KnownLexemes(ctx, beforePosition) is the global authoring
// view: words introduced as new in an earlier unit. It is a different set,
// and this package does not implement Lookup.
package placement
