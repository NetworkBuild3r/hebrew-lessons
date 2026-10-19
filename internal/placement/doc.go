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
// Each rating is that word's first real FSRS review. Rate writes the he_en
// card, its review log, and the answer in one transaction. The schedule is
// the one FSRS-5 assigns a new card. Rating 0 is stored as
// DontKnow and graded as Again, so the card's latest rating is Again and
// the word is learning. The initial stability is the one FSRS-5 assigns a
// new card. There is no separate seeding step. A word that already has a
// he_en card is skipped. Known and learning are read from that card: the
// latest rating is Good or Easy when the word is known, and Again or Hard
// when it is learning. Times come from the clock.Clock passed to New.
//
// Bands 0–4 are real words. There are no pseudo-words. Each band deals at
// most BandMax words. The deck plays band 0 first, then bands 1–4. Each
// band is shuffled from the seed stored on the run. Once StopMinRated
// cards in a band are rated, an Again share of StopAgainRate or more stops
// that band and every band above it, and the run ends. "I don't know"
// counts as Again for that rule. Hard does not. Exactly StopAgainRate stops.
// Fewer than StopMinRated ratings do not stop. The run also stops after
// CardCap rated cards. There is no time cap.
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
// of BandMax Good answers in every band.
//
// Level is the highest band whose block was fully rated without the Again
// rate reaching StopAgainRate, or −1. A band with nothing dealable is skipped.
// EstimatedKnownWords sums, over bands 1–4, GoodPlusRate times the band's
// width in ranks (500, 500, 1000, 2000), then adds the count of band 0
// words rated Good or Easy. SuggestedStartPosition is filled by Finish,
// not by Score: the lowest units.position whose doc newWords are less than
// 80% covered by the learner's known set, one past the highest position
// when every unit is covered, and 0 when there are no units. The known set
// is KnownLexemeIDs: he_en cards whose latest rating is Good or Easy.
// A placement Good or Easy is that review. Finishing a lesson does not
// add the lesson's new words. An empty newWords list
// is covered. A latest Again or Hard does not count. The position is
// recomputed on read.
//
// The tuning numbers are BandMax, StopMinRated, StopAgainRate,
// SittingSize, and CardCap.
//
// The deck API on *Service, reached from app.App.Placement, is:
//
//	Start(ctx, userID) (runID, error)
//	Next(ctx, runID) (NextResult, error)
//	ContinueSitting(ctx, userID) error
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
//	(*db.Store).LearnerKnownLexemes(ctx, userID) ([]string, error)
//	  The pointed NFC text of that set, sorted.
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
//	(*db.Store).PutUnit(ctx, id *int64, doc, scope, actor) (db.PutResult, error)
//	  A nil id creates a draft. A missing id returns db.ErrUnitNotFound.
//	  An identical document, position, and scope returns Unchanged and
//	  writes nothing, so a published unit stays published. Any change,
//	  including scope only, appends a revision, sets status to draft,
//	  and replaces lesson_items. actor is the caller, not a fixed name.
//	  A position change updates that unit only.
//
// Handlers that call Next, Rate, or Finish must check run ownership and
// CSRF. They load the run, require run.UserID to be the session user, and
// take that user id only from the session. A POST uses the existing CSRF check.
//
// units.Lookup.KnownLexemes(ctx, beforePosition) is the shared-unit authoring
// view: words introduced as new in an earlier shared unit. It is a different set,
// and this package does not implement Lookup. There is no learner_lexemes
// table.
package placement
