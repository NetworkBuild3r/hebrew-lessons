// Package jobs is the scheduled work of the app (INIT-015/SPEC-014). Today that is one job,
// the daily snapshot: it records, for every learner and every day, how many words are in each
// state, and it writes down the words that fell out of known.
//
// # What a run writes
//
// For one learner and one day, in one transaction:
//
//   - 28 daily_snapshots rows, one per skill and state: the count of words, the estimated
//     words known in that skill, and the retention. The rows are written even when the learner
//     has no evidence (every word is then unseen), so a day with no study still has a row.
//   - a regression_events row (kind word) for each word and skill that was known or solid in
//     the learner's previous snapshot and is struggling or regressed now. The cause is the
//     word's learner.Diagnose result, or "unexplained" when no pattern fits.
//   - a regression_events row (kind band_slip, no word) for a placement band whose share of
//     known words fell 15 points or more since the snapshot 7 days earlier.
//   - the learner's word states and band shares as of the day, in job_state, so the next
//     run has something to compare with.
//
// # Why the previous state is kept in job_state
//
// daily_snapshots holds counts, not words. A regression is a word that moved between two
// states, so the job keeps each learner's per-word states from their newest snapshot
// (job_state key daily.words.<user id>) and the band shares of the last 14 days (daily.bands.<user id>).
// A regression is found as a transition between those stored states. It is not found by
// reading the state "regressed", because the model says struggling before regressed: a word
// that is both is struggling.
//
// # Idempotence, resume, and catch-up
//
// A learner-day that already has snapshot rows is skipped, so a second run for the same day
// writes nothing. Each learner-day is its own transaction, so a run that stops half way keeps
// the learners it finished and the next run does the rest. No transaction spans learners.
// Run takes a lock (job_state key daily.lock) so two processes do not run at once. A lock
// whose holder has not written for 30 minutes is taken over.
//
// CatchUp fills every day since each learner's newest snapshot, up to yesterday and at most
// 14 days. A learner with no snapshot gets only yesterday, so history starts with a real day.
//
// # A limit that is disclosed, not hidden
//
// The model reads the evidence as it is now. Cards keep their current stability and nothing
// keeps old card states, so a day cannot be recomputed after it has passed. A snapshot is
// therefore the state at the time the job ran, filed under the day it was run for. For a
// day caught up late, that is the state at catch-up time. When a gap of several days is
// caught up, the learner is evaluated once, every missed day gets that state, and the
// regression events of the whole gap land on the first missed day (later days have nothing
// to compare). A band_slip is not repeated for the same band within 7 days.
//
// All times come from internal/clock. The day boundary is the server time zone.
package jobs
