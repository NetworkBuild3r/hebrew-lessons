// Package placement is a yes/no vocabulary test in the style of Paul Meara.
//
// The learner sees pointed Hebrew words, some of them invented, and says
// whether they know each one. Review history starts empty. A finished run
// can record real words the learner already knows, so a later unit does not
// treat those words as new. Invented words are never written to lexemes,
// and this package does not create FSRS cards.
//
// Times come from the clock.Clock passed to New.
//
// Bands 0–4 are real words: 12 from the tutor's lessons (band 0) and 12
// from each frequency band (bands 1–4). Twenty items are pseudo-words.
// Start deals all 80 in an order fixed by a seed stored on the run. The
// Item a caller receives has the id and the pointed text only. Band,
// pseudo, gloss, and part of speech stay on the server.
//
// Scoring lives in Score(answers, items) Result, so a UI and MCP apply the
// same rule. For each band, h is the yes-rate on that band's real words, f
// is the yes-rate on the pseudo-words, and corrected is (h − f) / (1 − f).
// A band passes when corrected is at least 0.8. When f is 1, no band
// passes and each corrected score is 0. When f is above 0.25 the run is
// unreliable and seeds nothing. Level is the highest passed band, or −1
// when none passed.
//
// EstimatedKnownWords sums, over bands 1–4, clamp(corrected, 0, 1) times
// the band's width in ranks (500, 500, 1000, 2000), then adds the count of
// band 0 words answered yes. SuggestedStartPosition is filled by Finish and
// Latest, not by Score: the lowest units.position whose doc newWords are
// less than 80% covered by the learner's known set, one past the highest
// position when every unit is covered, and 0 when there are no units. The
// known set is the KnownLexemeIDs union. An empty newWords list is covered.
//
// Seeding writes each yes-answered real word in a passed band into lexemes
// (text_nfc, gloss_en, pos, verified 0) and into learner_lexemes as status
// known, seeded 1, source placement. A retake's latest finished run sets
// the level. Seeds from an earlier run stay, because the learner row is
// inserted with INSERT OR IGNORE.
//
// MCP Builder, for get_context, reads these.
//
//	Latest(ctx, userID) (*Result, error)
//	  The most recently finished run, ordered by finished_at then id.
//	  (nil, nil) when the learner has not finished a run.
//	  EstimatedKnownWords is the value stored with that run.
//	  SuggestedStartPosition is computed from the units and known words now.
//
//	KnownLexemeIDs(ctx, userID) ([]int64, error)
//	  The union of learner_lexemes rows with status known and seeded 1,
//	  and lexemes taught as lesson_items.role new in a lesson this learner
//	  has finished (lesson_runs.finished_at is set for that user, unit, and
//	  lesson). Ids are sorted and unique.
//
//	(*db.Store).LatestPlacement(ctx, userID) (db.Placement, bool, error)
//	  The same finished run as Latest, with ok false when there is none.
//	  db.Placement carries TakenAt, Reliable, FalseAlarmRate, Level,
//	  BandScores, EstimatedKnownWords, and SuggestedStartPosition.
//
//	(*db.Store).PlacementKnownLexemes(ctx, userID) ([]string, error)
//	  The pointed NFC text of the seeded-known learner_lexemes, sorted.
//	  This is the seeded half of the union, not KnownLexemeIDs.
//	  units.Lookup.KnownLexemes(ctx, beforePosition) is the global authoring
//	  view: words introduced as new in an earlier unit. It is a different set,
//	  and this package does not implement Lookup.
package placement
