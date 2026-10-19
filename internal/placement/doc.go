// Package placement deals an adaptive vocabulary deck.
//
// The learner sees a pointed Hebrew word and its audio, recalls it, and
// may say "I don't know this" before the flip. That answer is rating 0.
// After the flip the ratings are Again, Hard, Good, or Easy. The back of
// the card is the gloss, the transliteration, and the gender. TextNFC and
// AudioURL are the front. GlossEN, Translit, and Gender are the back.
// They are returned together. AudioURL is /audio/<id>.mp3 when a clip
// exists for the exact pointed text, and empty otherwise.
//
// Each rating is that word's first real FSRS review. Rate creates a he_en
// card and grades it through fsrs.Review.Grade. Rating 0 is stored as
// DontKnow and graded as Again, so the card's latest rating is Again and
// the word is learning. The initial stability is the one FSRS-5 assigns a
// new card. There is no separate seeding step. A word that already has a
// he_en card is skipped. Known and learning are read from that card: the
// latest rating is Good or Easy when the word is known, and Again or Hard
// when it is learning. Times come from the clock.Clock passed to New.
//
// Bands 0–4 are real words. There are no pseudo-words. Each band deals at
// most BandSize words. The deck plays band 0 first, then bands 1–4. Each
// band is shuffled from the seed stored on the run. Once MinRatedForStop
// cards in a band are rated, an Again share of StopAgainMin or more stops
// that band and every band above it, and the run ends. "I don't know"
// counts as Again for that rule. Hard does not. Exactly StopAgainMin stops.
// Fewer than MinRatedForStop ratings do not stop. The run also stops after
// MaxCards rated cards. There is no time cap.
//
// A sitting is SittingSize answers. Next then returns a break instead of a
// card. The client shows "Done for today" as the primary action and "Keep
// going" as the secondary one. ContinueSitting starts the next sitting.
// Start resumes an unfinished run on a later day and opens a new sitting
// when the last one was full. "Done for today" does not finish the run.
// Finish stays explicit and idempotent. The run scores itself when a band
// stops, the card cap is hit, or nothing dealable remains.
//
// StopReason is band_stop, cap_cards, exhausted, or empty while the run is
// in progress or when the learner finished it first. band_stop wins over
// the card cap. exhausted wins when nothing remains, including a full run
// of BandSize Good answers in every band.
//
// Level is the highest band whose block was fully rated without the Again
// rate reaching StopAgainMin, or −1. A band with nothing dealable is skipped.
// EstimatedKnownWords sums, over bands 1–4, GoodPlusRate times the band's
// width in ranks (500, 500, 1000, 2000), then adds the count of band 0
// words rated Good or Easy. SuggestedStartPosition is filled by Finish,
// not by Score: the lowest units.position whose doc newWords are less than
// 80% covered by the learner's known set, one past the highest position
// when every unit is covered, and 0 when there are no units. The known set
// is KnownLexemeIDs: he_en cards whose latest rating is Good or Easy, plus
// new words of lessons this learner has finished. An empty newWords list
// is covered. A latest Again or Hard does not count. The position is
// recomputed on read.
//
// The tuning numbers are BandSize, MinRatedForStop, StopAgainMin,
// SittingSize, and MaxCards.
//
// The deck API on *Service, reached from app.App.Placement, is:
//
//	Start(ctx, userID) (runID, error)
//	Next(ctx, runID) (NextResult, error)
//	ContinueSitting(ctx, runID) error
//	Rate(ctx, runID, itemID, rating) error
//	Finish(ctx, runID) (Result, error)
//
// NextResult carries Card, Break, and Done. Score(tallies) Result is the
// band rule.
//
// MCP Builder, for get_context, reads these store methods.
//
//	(*db.Store).LatestPlacement(ctx, userID) (db.Placement, bool, error)
//	  The newest run, ordered by id. ok is false when the learner has none.
//	  db.Placement carries TakenAt, Finished, Level, CardsRated, StopReason
//	  (band_stop, cap_cards, exhausted, or empty), BandScores (band, rated,
//	  again, hard, good, easy, dont_know, good_plus_rate),
//	  EstimatedKnownWords, SuggestedStartPosition, and Sittings.
//
//	(*db.Store).PlacementKnownLexemes(ctx, userID) ([]string, error)
//	(*db.Store).PlacementLearningLexemes(ctx, userID) ([]string, error)
//	  Pointed NFC text from the newest run's placement answers, sorted.
//	  Good or Easy is known. "I don't know", Again, or Hard is learning.
//
//	(*db.Store).KnownLexemeIDs(ctx, userID) ([]int64, error)
//	  Sorted card-based known set described above.
//
//	(*db.Store).StruggleStats(ctx, userID, topN) (db.Struggle, error)
//	  Lexemes is every reviewed lexeme sorted by text, with ConfusionKeys.
//	  Top is the first topN by misses in the last 7 days, lapses, then the
//	  Again rate. topN ≤ 0 leaves Top empty. Confusions are ConfusionPairs
//	  from the last 7 days on the store clock. There is no LexemeStruggles
//	  and no ConfusionCounts.
//
//	(*db.Store).CompletedSharedUnits(ctx, userID) ([]db.UnitCompletion, error)
//	(*db.Store).UnitsInScope(ctx, learnerID) ([]db.UnitRow, error)
//	(*db.Store).PutUnit(ctx, doc, scope) (int64, error)
//	  Writes the units row's scope and a unit_revisions row in one
//	  transaction. scope is shared or learner:<digits>. A scope-only
//	  change still writes a revision.
//
// units.Lookup.KnownLexemes(ctx, beforePosition) is the global authoring
// view: words introduced as new in an earlier unit. It is a different set,
// and this package does not implement Lookup. There is no learner_lexemes
// table.
package placement
