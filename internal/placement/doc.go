// Package placement is the placement test. Engine is the v3 test (INIT-015/SPEC-007). Service is the
// v2 self-graded deck, kept until the placement screens move to Engine (SPEC-012).
//
// # The v3 test
//
// It measures, and it does not teach. The learner takes four parts. Part 0 is six letters: hear a
// syllable, pick the letter with its vowel. Five or more right lets reading be tested. Fewer means
// RunResult.ReadingReady is false and the reading part is skipped. Part 1 is listening: the clip plays,
// no Hebrew is shown, and the learner picks the meaning from four options or says "I don't know". Part 2
// is reading: pointed Hebrew, no audio, no transliteration, the same four options. Part 3 is typing, and
// optional: the English meaning, and the learner types the Hebrew. It asks only words that were right in
// Parts 1 and 2, at most TypingMax of them, and grades with fsrs.GradeTyped. Only an exact answer is right.
//
// Bands are numbered 1 to BandCount here. Band N is the deck's band N−1 (items.json numbers its bands 0
// to 4), so band 1 is the tutor words. Result levels and estimates use these numbers. The level stored in
// placement_runs.level is the deck number, the reading level less one, because that column has meant "the
// highest deck band completed" since v2.
//
// # Adaptive rule
//
// Each skill (listening, then reading) starts in band 1 and is dealt FirstSize items. Five or six right
// moves up a band. Three or four right deals ExtraSize more in the same band, and the decision is then on
// all twelve: eight or more right moves up, and fewer stops. Fewer than three right stops the skill. A
// skill that clears band 5 is done. "Right" is the only thing that counts toward moving up, and a band
// that runs out of words ends the skill. Decide and replay are that rule, and TestScoreAdaptiveRule pins it.
//
// A word is never asked twice in listening and reading in one run: both walk the band's seeded order and
// skip a word already asked. A word with no clip is not dealt in listening. Distractors come from the same
// band and part of speech, and never from a word whose gloss shares a content word with the answer's.
// Where a band holds too few adverbs, an adverb borrows adjectives and the reverse (the "modifier" class).
// Nothing else crosses a part of speech.
//
// # Scoring
//
// The known share of a band is max(0, (right − wrong ÷ 3) ÷ asked). "I don't know" counts in asked and
// is neither right nor wrong, so it is never penalised. Estimated words known is the sum over bands of
// share × BandWidth. The level is the highest band with a share of at least 0.5. Listening and reading
// are scored apart.
//
// # What a run writes
//
// Every word answer goes through evidence.Recorder.Attempt with source placement, so a right answer
// seeds that skill's card as Good (only when the word has none) and nothing else is scheduled. A wrong
// answer or "I don't know" seeds no card. When the run finishes, each of those words gets an exposure with
// context placement_review, because that is when the review shows it. A letters item names no word, so it
// writes no attempt: its outcome is kept on the item row. A new lexicon row is made for a word when it is
// first answered, as v2 did.
//
// The result lists each miss with its lexeme, skill, what was picked, the right gloss, the pointed form,
// and the audio id. It also carries StartUnit, the same value as db.Placement.SuggestedStartPosition.
// The stored placement_runs.result keeps the v2 keys (bands, estimated_known_words) so that the learner
// model and the MCP read still work: its bands are the deck's bands, pooled over the skills tested, with
// good_plus_rate holding the guess-corrected share.
//
// # No feedback
//
// The reply to an answer is a Step: the next Item, or TypingOffer, or Done with the Result. It has no
// field that says whether the answer was right, and a test compares the replies to a right answer, a wrong
// one, and "I don't know" byte for byte. Item ids are opaque tokens ("p7"), option ids are positions
// ("o0" to "o3"), and the right option is kept on the server.
//
// # Calling it
//
// Every method takes the learner's id from the session, not from the request, and treats another
// learner's run, a v2 run, and an unknown run alike as ErrNotYourRun. A handler still needs CSRF on POST.
//
//	NewEngine(store, rec, clk, ...EngineOption) *Engine      // WithSource(rand.Source) sets run seeds
//	(*Engine).Start(ctx, userID) (runID, error)               // resumes an open run; closes an open v2 run
//	(*Engine).Next(ctx, userID, runID) (Step, error)          // the open item, or deals one; repeatable
//	(*Engine).Answer(ctx, userID, runID, itemID, AnswerIn) (Step, error)
//	(*Engine).StartTyping(ctx, userID, runID) (Step, error)   // take Part 3 at the offer
//	(*Engine).Finish(ctx, userID, runID) (RunResult, error)   // skips typing; idempotent
//	(*Engine).Result(ctx, userID, runID) (RunResult, error)
//	(*Engine).Latest(ctx, userID) (RunResult, bool, error)    // the learner's current picture
//	(*Engine).NewRecheck(ctx, userID, skill) (runID, error)   // skill is "listen" or "read"
//	(*Engine).RecheckDue(ctx, userID) (RecheckStatus, error)
//
// # Re-check
//
// NewRecheck opens a short sitting of one skill in one band: the first band above the skill's level in
// the learner's latest finished v3 result. It is the same item, answer, and write path, so Next, Answer,
// and Finish drive it. Its result is the whole current picture, with that band replaced, and Recheck says
// whether the skill moved up, stayed, or went down. RecheckDue is true from 28 days after the last finished
// v3 placement or re-check, and Late is set from 42 days. It has no end: the window is soft.
//
// # State
//
// An Engine keeps no run state. A run is its placement_runs row, its placement_v3_runs row, and its
// placement_items rows (migration 011). The deal is a pure function of the run's seed, drawn from the
// injected rand.Source, and the answers so far, so any process can serve the next request. Two requests
// that answer one item at once count once: the answer is claimed with an update that matches an open item.
//
// # The v2 deck
//
// The rest of this comment describes Service. It is unchanged.
//
// The learner sees a pointed Hebrew word and its audio, recalls it, and
// may say "I don't know this" before the flip. That answer is rating 0.
// After the flip the ratings are Again, Hard, Good, or Easy. The back of
// the card is the gloss, the transliteration, and the gender. TextNFC and
// AudioURL are the front. GlossEN, Translit, and Gender are the back.
// They are returned together. AudioURL is /audio/<id>.mp3 when a clip
// exists for the exact pointed text, and empty otherwise.
//
// Each rating is that word's first real FSRS review. Rate writes the read
// card, its review log, and the answer in one transaction. The schedule is
// the one FSRS-5 assigns a new card. Rating 0 is stored as
// DontKnow and graded as Again, so the card's latest rating is Again and
// the word is learning. The initial stability is the one FSRS-5 assigns a
// new card. There is no separate seeding step. A word that already has a
// read card is skipped. Known and learning are read from that card: the
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
// is KnownLexemeIDs: read cards whose latest rating is Good or Easy.
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
//	  Pointed NFC text from the newest run, sorted. A v2 run reads
//	  placement answers. A v3 run reads answered placement items: right is
//	  Good, wrong is Again, and dont_know is "I don't know".
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
