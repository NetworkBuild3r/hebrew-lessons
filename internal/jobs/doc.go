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
//
// # The weekly report (INIT-015/SPEC-023)
//
// Weekly.Run writes one weekly_reports row for each learner and each finished week. A week is
// Monday to Sunday in the server time zone, the same zone the daily job files its days under,
// so a regression event and a snapshot land in the week their local day says. Minutes studied
// are counted over the same local days (Monday 00:00 to the next Monday 00:00 local), not over
// UTC days, so an evening session is not moved a day. The scheduler writes the report on Monday
// at 04:00, half an hour after the daily job wrote Sunday. A week that has not ended is refused.
//
// What the payload holds:
//
//   - lost: each regression event of the week (a word that fell from known or solid to
//     struggling or regressed in one skill), with the cause the daily job stored, and whether
//     the word is known again at the end of the week;
//   - gained: each word and skill that was not known at the start and is known or solid now;
//   - cleared: each word and skill that was struggling at the start and is neither struggling
//     nor regressed now;
//   - retention: the mean of the skills' retention on the newest snapshot of the week, against
//     the fixed 85% target (the learner has no per-learner target in the schema);
//   - minutes studied: the time spent answering, from attempts (see activeMinutes);
//   - band slips: the band_slip events of the week.
//
// The summary is three plain sentences from templates. No model is called.
//
// Gained and cleared words are a comparison, and nothing keeps old per-word states except
// daily.words.<user>, which holds only the newest. So the weekly job keeps its own copy of the
// states it ended each report at (job_state key weekly.words.<user>) and compares the next
// week with it. A first report has nothing to compare with, so it names no gains and says so.
// The states are those of the newest daily snapshot at the time the report is written, which
// is Sunday for the scheduled run and later for a catch-up or a back-filled week, and the
// payload names that day (stateDay). A week written late is therefore not a recomputation of
// that week. Lost words do not have this limit, because the events carry their own day.
//
// Idempotence: a learner with a report for the week is skipped, and the row is only ever
// inserted. The report, and the move of the baseline, are one transaction. The weekly job
// takes the daily job's lock, so the two never run at once.
package jobs
