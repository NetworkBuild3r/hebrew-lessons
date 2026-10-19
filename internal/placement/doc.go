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
// Bands 0–4 are real words. There are no pseudo-words. BlockSize words are
// dealt from each band, or fewer when the band has fewer dealable words.
// The deck plays band 0 first, then bands 1–4. Each band is shuffled from
// the seed stored on the run. After a band's block, a Good-or-Easy share
// below StopGoodPlusMin stops the run. Exactly StopGoodPlusMin does not.
// DontKnow counts in the rated total and in DontKnow, not in Again, and
// it is not a Good or Easy answer. Hard is not either. The run also stops
// after MaxCards rated cards, or MaxDuration after the start, measured
// with the service clock. A card already dealt is still returned after the
// clock runs out, so the learner can rate it. Start returns the learner's
// unfinished run when there is one, and Next continues after the last
// answered card. Finish is idempotent. A partial run the learner finishes
// early is a valid placement: its StopReason is empty.
//
// StopReason is band_stop, cap_cards, cap_time, exhausted, or empty while
// the run is in progress or when the learner finished it first. band_stop
// wins over the caps. cap_cards wins over cap_time. exhausted wins over
// the clock. Score does not see the clock; Finish sets cap_time only when
// Score left the reason empty.
//
// Level is the highest band whose block was fully rated at or above
// StopGoodPlusMin, or −1. A band with nothing dealable is skipped.
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
// The tuning numbers are BlockSize, StopGoodPlusMin, MaxCards, and
// MaxDuration.
//
// The deck API on *Service, reached from app.App.Placement, is:
//
//	Start(ctx, userID) (runID, error)
//	Next(ctx, runID) (*Card, done bool, error)
//	Rate(ctx, runID, itemID, rating) error
//	Finish(ctx, runID) (Result, error)
//
// Score(tallies) Result is the block rule. It does not read the clock.
//
// MCP Builder, for get_context, reads these store methods.
//
//	(*db.Store).LatestPlacement(ctx, userID) (db.Placement, bool, error)
//	  The newest run, ordered by id. ok is false when the learner has none.
//	  db.Placement carries TakenAt, Finished, Level, CardsRated, StopReason
//	  (band_stop, cap_cards, cap_time, exhausted, or empty), BandScores
//	  (band, rated, again, hard, good, easy, dont_know, good_plus_rate),
//	  EstimatedKnownWords, and SuggestedStartPosition.
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
