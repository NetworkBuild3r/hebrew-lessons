// Package session builds the post-placement review queue.
//
// This is the only study scheduler. It is not the login row in package db.
// It interleaves struggling cards with stable known cards, then grades each
// answer through evidence.Recorder.SelfGrade. Letter and pattern drills are
// card kinds in that same mix: a confusion with at least MinMissesForDrill
// misses in the struggle window adds DrillSteps visits of that kind. Those
// visits do not take a known or struggle slot, and they do not change PlanFor.
// DrillCap is 20, the old queue's per-visit bound. Drill steps past that
// cap are left for a later session. Study cards are not dropped to make room.
// A drill is not an FSRS card. Record grades it in the queue only.
//
// The mix itself does not touch the database. BuildSession reads the clock
// once and passes that instant to ListStudyCards, RecentReviewRatings, and
// the learner model (which supplies each word's state and confusions), then
// mix turns that snapshot into a queue. Record is the
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
// coach also reports. MaxNewPerSession is 5, MinNewPerSession is 2, and
// MaxNewShare is 0.25. BuildSession applies them to cards that have no
// review. With reviews due, the new-card count is
// min(5, max(2, floor(0.25 * sessionSize))), where sessionSize is the
// reviewed cards plus the new cards. A session with no reviews due takes
// up to 5. RecentReviews is 50.
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
// that band. MaxStruggleRun is 2. RequeueGap is 3, and a sitting's miss
// gap is 3, 4, or 5. StruggleLapses is 2.
// ConfusionWindow is 7 days, the same span StruggleStats counts.
//
// # Algorithm
//
// A due card (read, recall, listen or spell) is struggling when its lapse
// count is at least StruggleLapses, or its latest rating is Again or Hard, or
// the learner model lists a confusion key for its lexeme. The model
// (internal/learner) reads confusion_misses, of every kind including other,
// over ConfusionWindow ending at the same instant as the rest of the build.
// A card is stable known when its latest rating is Good or Easy and it has no
// Again inside that same window.
// A read card also has to sit in KnownLexemeIDs, which is the latest-Good
// or Easy read set. A placement Good or Easy is that review. Finishing a
// lesson does not add the lesson's new words.
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
// A due read or recall card with no review is a new card. It does not take
// a known slot, a struggle slot, or a drill step. New cards keep the order
// from the intro lesson and are seeded through the woven queue, so they are
// not a block at the end. When the woven queue is not empty, one of its
// cards stands after the new cards. The allowance counts sessionSize as the reviewed
// cards in the planned queue plus the new cards. Drill steps are not part
// of that count. With no reviewed cards, the allowance is MaxNewPerSession.
// With reviews due, it is min(MaxNewPerSession, max(MinNewPerSession,
// floor(MaxNewShare * sessionSize))). MinNewPerSession is 2, so one or two
// reviews still introduce two new cards when that many are waiting. New
// cards are ordered by the intro unit's position, then that lesson's index,
// then frequency rank when the card has one, then card id. A card with no
// intro lesson sorts after every card that has one. A card with no frequency
// rank sorts after every card that has one. The lexicon does not store a
// rank, so ListStudyCards leaves that key unset.
//
// A pass then walks the queue. Two consecutive study visits that share a
// lexeme are split when a later card can take the second visit's seat
// without sitting beside that lexeme and without a struggle run longer
// than MaxStruggleRun. A pair with no such seat stays. Drills are not
// lexeme visits.
//
// # Activities and skills (INIT-015/SPEC-015)
//
// Choosing cards is the mix above. Each study visit is then asked through the
// weakest skill of its word that has a checked activity: CardRef.Activity and
// CardRef.AskSkill. The skills come from the learner model, one
// learner.Model.Word per distinct lexeme in the build, read at the build's
// instant. activity.WeakestSkills ranks them by state, then by accuracy, and
// activity.Choose picks the activity: the skill's everyday one, and its match
// activity on every third visit through that skill. The rules, in order:
//
//   - a skill is asked only when its gate is open (listening always, the rest
//     once the letters check is passed: WithGate) and an activity that can ask
//     the word fits the lexicon (a gloss, a clip, enough fair distractors);
//   - the weakest state goes first (unseen, seen, struggling, regressed,
//     learning, known, solid), then the lower accuracy, then listen, read,
//     recall, spell;
//   - a skill with no evidence is opened on a word only while none of the
//     word's other skills is struggling or regressed;
//   - a word that comes up again in the same queue is asked through the next
//     skill, while one is left;
//   - a checked activity always beats a flashcard. Only when none fits is the
//     visit a flashcard through the card's own skill, graded with Record.
//
// The cards, their order and the known and struggle counts are those of the
// mix before activities: asking through a skill changes no card. The card in
// a CardRef is the one that was due, and for a checked activity the evidence
// lands on the card of AskSkill, which activity.Engine.Check creates at the
// first attempt. Answered closes a checked visit. A new card whose word has no
// exposure has Intro set: the screen records Engine.Expose before the first
// check.
//
// Next returns the card at the cursor. Reveal marks that card peeked.
// Record grades it through SelfGrade, which also writes an attempts row with
// activity flashcard. A grade is only accepted after Reveal on this serve, so
// the review_log row is peeked. A grade with no Reveal
// returns ErrNotRevealed, writes nothing, and leaves the serve in place.
// Again inserts that same card once. The gap is 3, 4, or 5 other visits,
// chosen from the sitting seed (RequeueGap plus the seed mixed modulo 3).
// The inserted visit does not take a second SessionCap slot. When fewer
// than that many cards remain, the miss is placed at the end of the queue.
// A queue whose study visits are all one lexeme is not requeued. The
// session ends instead of serving that word on the next Continue.
package session
