// Package session builds the post-placement review queue.
//
// This is the only study scheduler. It is not the login row in package db.
// It interleaves struggling cards with stable known cards, then grades each
// answer through fsrs.Review.GradeWithChoice. Letter and pattern drills are
// card kinds in that same mix: a confusion with at least MinMissesForDrill
// misses in the struggle window adds DrillSteps visits of that kind. Those
// visits do not take a known or struggle slot, and they do not change PlanFor.
// DrillCap is 20, the old queue's per-visit bound. Drill steps past that
// cap are left for a later session. Study cards are not dropped to make room.
// A drill is not an FSRS card. Record grades it in the queue only.
//
// The mix itself does not touch the database. BuildSession reads the clock
// once and passes that instant to ListStudyCards, RecentReviewRatings, and
// StruggleStatsAt, then mix turns that snapshot into a queue. Record is the
// only write: it grades the card that Next just served. BuildSession returns
// a pointer. The queue stays in that Session. A restart drops it.
//
// Handlers are not in this package. The handler that calls BuildSession or
// Record checks Session.UserID against the signed-in learner, and it checks
// the CSRF token on every mutating request. Those checks need the HTTP
// request, which this package does not see.
//
// # Constants
//
// SessionCap is 20. TargetSuccess is 0.85, the sessionlimits value the
// coach also reports. MaxNewPerSession is 5 and MaxNewShare is 0.25.
// BuildSession applies both to cards that have no review. RecentReviews is 50.
// LowAccuracy is 0.80 and HighAccuracy is 0.90. A recent Good/Easy share below
// LowAccuracy uses LowAccuracyKnownShare. A share above HighAccuracy uses
// HighAccuracyKnownShare. A share from LowAccuracy through HighAccuracy, and
// a learner with no reviews, uses DefaultKnownShare.
//
// 85% (TargetSuccess) is the target success rate. The known share that
// lands on that rate is about 7/9 (0.778). DefaultKnownShare is that
// fraction, from a known card at KnownSuccess (0.95) and a struggling card
// at StruggleSuccess (0.50):
//
//	(TargetSuccess - StruggleSuccess) / (KnownSuccess - StruggleSuccess)
//
// AccuracyShareShift (0.15) is how far the known-card share moves outside
// that band. MaxStruggleRun is 2. RequeueGap is 3. StruggleLapses is 2.
// ConfusionWindow is 7 days, the same span StruggleStats counts.
//
// # Algorithm
//
// A due he_en or en_he card is struggling when its lapse count is at least
// StruggleLapses, or its latest rating is Again or Hard, or StruggleStats
// lists a confusion key for its lexeme. StruggleStatsAt reads
// confusion_misses over ConfusionWindow ending at the same instant as the
// rest of the build. A card is stable known when its latest rating is Good
// or Easy and it has no Again inside that same window.
// A he_en card also has to sit in KnownLexemeIDs, which is the latest-Good
// or Easy he_en set plus new words of lessons this learner has finished.
// Struggling wins when a card matches both. A not-yet-due card is never
// struggling. Suspended cards, and words whose intro lesson is unfinished,
// stay out.
//
// Due cards are chosen before not-yet-due cards. The known quota is filled
// from due known cards first. Not-yet-due known cards fill only the known
// quota that is still short. They are not used to pad a shortfall of
// struggling cards. When one side cannot fill its quota, remaining due cards
// of the other side fill the cap. Each side is ordered by due time, then by
// a pure mix of the injected seed and the card id. There is no math/rand
// draw. The same clock, the same seed, and the same rows always build the
// same queue.
//
// The two lists are then interleaved. While a known card is still waiting,
// a run of struggling cards stops at MaxStruggleRun. Within the known list,
// every due card stands before every not-yet-due card, so the due known
// cards are emitted first. Letter steps and pattern steps are woven in
// after that, one of each after each study card until the drill plan runs
// out or DrillCap steps have been woven. Study-card order and the known
// and struggle counts stay as planned.
//
// A due he_en or en_he card with no review is a new card. It does not take
// a known slot, a struggle slot, or a drill step. New cards are appended
// after the woven queue. At most MaxNewPerSession of them are taken. When
// the planned queue already has a reviewed card, new cards are also at most
// MaxNewShare of the session those reviewed cards and the new cards make.
// A planned queue with no reviewed cards still takes up to MaxNewPerSession,
// so the words of a finished lesson are not an empty session. New cards are
// ordered by the intro unit's position, then that lesson's index, then
// frequency rank when the card has one, then card id. A card with no intro
// lesson sorts after every card that has one. A card with no frequency rank
// sorts after every card that has one. The lexicon does not store a rank,
// so ListStudyCards leaves that key unset.
//
// Next returns the card at the cursor. Reveal marks that card peeked.
// Record grades it through GradeWithChoice with an empty chosen option.
// peeked is true only after Reveal on this serve. A grade with no Reveal
// is stored with peeked false and is not rejected. Again inserts that same card
// RequeueGap places after the cursor, at most once. The inserted visit does
// not take a second SessionCap slot. When fewer than RequeueGap cards remain,
// the miss is placed at the end of the queue.
package session
