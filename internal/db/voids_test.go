package db

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// INIT-038/SPEC-002: a void hides the named answer from the evidence readers,
// an undone void counts again, and an empty answer_voids table leaves the
// readers on the numbers they had before this spec.

type threeDayFixture struct {
	user, lex, card int64
	days            [3]time.Time
	attempts        [3]int64
	reviews         [3]int64
	confusion       int64
}

func seedThreeDayEvidence(t *testing.T, s *Store) threeDayFixture {
	t.Helper()
	ctx := context.Background()
	user := insertUser(t, s, "voids@example.com", "sub-voids", "learner")
	if err := s.SetUserTimeZone(ctx, user, "UTC"); err != nil {
		t.Fatal(err)
	}
	lex := mustLexemeText(t, s, "סֵפֶר")
	days := [3]time.Time{
		time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC),
	}
	card, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: days[2], Stability: 10, Difficulty: 5, Reps: 3, LastReview: days[2],
	})
	if err != nil {
		t.Fatal(err)
	}
	var f threeDayFixture
	f.user, f.lex, f.card, f.days = user, lex, card, days
	latencies := []int64{1000, 1200, 800}
	for i, at := range days {
		res := mustExec(t, s, `INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, 'read', 'read_pick', 'review', 'book', 'book', 1, 0, 0, ?, ?)`,
			user, lex, latencies[i], FormatTime(at))
		f.attempts[i], err = res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		res = mustExec(t, s, `INSERT INTO review_log (
			card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied
		) VALUES (?, 3, ?, 1, 2, 0, 0)`, card, FormatTime(at))
		f.reviews[i], err = res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
	}
	res := mustExec(t, s, `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
		VALUES (?, 'letter_pair', 'ב/כ', ?, ?)`, user, lex, FormatTime(days[1]))
	f.confusion, err = res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

type readerSnap struct {
	outcomes, successes, days int
	first, last               string
	confusions                int
	attemptIDs                []int64
	latencies                 []int64
	recentCorrect, recentN    int
	history                   int
	retentionDue              int
	pairs                     int
	miss                      bool
}

func snapReaders(t *testing.T, s *Store, f threeDayFixture, opts EvidenceOpts) readerSnap {
	t.Helper()
	ctx := context.Background()
	q := testLearnerQuery()
	q.IncludeVoided = opts.IncludeVoided
	ev, err := s.LearnerEvidence(ctx, f.user, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(ev.Skills))
	}
	sk := ev.Skills[0]
	attempts, err := s.ListAttemptsOpts(ctx, f.user, f.lex, opts)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(attempts))
	for i, a := range attempts {
		ids[i] = a.ID
	}
	lat, err := s.CorrectLatenciesOpts(ctx, f.user, "read_pick", 10, opts)
	if err != nil {
		t.Fatal(err)
	}
	correct, total, err := s.RecentCheckedOpts(ctx, f.user, 10, opts)
	if err != nil {
		t.Fatal(err)
	}
	hist, err := s.CardHistoriesOpts(ctx, f.user, time.Time{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	reviews := 0
	if len(hist) == 1 {
		reviews = len(hist[0].Reviews)
	}
	ret, err := s.DueRetentionOpts(ctx, f.user, f.days[0].Add(-time.Hour), time.Time{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	due := 0
	for _, row := range ret {
		if row.Skill == "read" {
			due = row.Due
		}
	}
	pairs, err := s.RecentConfusionPairsOpts(ctx, f.user, f.days[0].Add(-time.Hour), opts)
	if err != nil {
		t.Fatal(err)
	}
	_, miss, err := s.LastConfusionMissOpts(ctx, f.user, "letter_pair", "ב/כ", opts)
	if err != nil {
		t.Fatal(err)
	}
	bySkill, err := s.ListAttemptsBySkillOpts(ctx, f.user, f.lex, "read", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(bySkill) != len(attempts) {
		t.Fatalf("by skill = %d, list = %d", len(bySkill), len(attempts))
	}
	return readerSnap{
		outcomes: sk.Outcomes, successes: sk.ReviewSuccesses, days: sk.SuccessDays,
		first: sk.FirstSuccessDay, last: sk.LastSuccessDay,
		confusions: len(ev.Confusions), attemptIDs: ids, latencies: lat,
		recentCorrect: correct, recentN: total, history: reviews, retentionDue: due,
		pairs: len(pairs), miss: miss,
	}
}

func evidenceRowCounts(t *testing.T, s *Store) (attempts, reviews, confusions int) {
	t.Helper()
	return count(t, s, `SELECT COUNT(*) FROM attempts`),
		count(t, s, `SELECT COUNT(*) FROM review_log`),
		count(t, s, `SELECT COUNT(*) FROM confusion_misses`)
}

func TestNoVoidRowsMatchTheFixture(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	f := seedThreeDayEvidence(t, s)
	got := snapReaders(t, s, f, EvidenceOpts{})
	want := readerSnap{
		outcomes: 3, successes: 3, days: 3,
		first: "2026-11-01", last: "2026-11-10",
		confusions: 1, attemptIDs: []int64{f.attempts[0], f.attempts[1], f.attempts[2]},
		latencies:     []int64{800, 1200, 1000},
		recentCorrect: 3, recentN: 3, history: 3, retentionDue: 3,
		pairs: 1, miss: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no voids:\n got %+v\nwant %+v", got, want)
	}
	again := snapReaders(t, s, f, EvidenceOpts{IncludeVoided: true})
	if !reflect.DeepEqual(again, got) {
		t.Fatalf("include-voided with no rows differs:\n include %+v\n default %+v", again, got)
	}
}

func TestVoidHidesAnswerAndUndoCountsItAgain(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	f := seedThreeDayEvidence(t, s)
	before := snapReaders(t, s, f, EvidenceOpts{})
	attemptsBefore, reviewsBefore, confusionsBefore := evidenceRowCounts(t, s)

	voidedAt := when.Add(time.Minute)
	rows, err := s.InsertAnswerVoids(ctx, "batch-middle", voidedAt, []AnswerVoidWrite{{
		AttemptID: f.attempts[1], ReviewLogID: f.reviews[1], ConfusionID: f.confusion,
		LearnerID: f.user, Reason: VoidReasonAlsoRight, Actor: "learner",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].BatchID != "batch-middle" || !rows[0].UndoneAt.IsZero() {
		t.Fatalf("inserted = %+v", rows)
	}
	if a, r, c := evidenceRowCounts(t, s); a != attemptsBefore || r != reviewsBefore || c != confusionsBefore {
		t.Fatalf("void edited evidence rows: attempts %d review %d confusion %d", a, r, c)
	}

	hidden := snapReaders(t, s, f, EvidenceOpts{})
	if hidden.outcomes != 2 || hidden.successes != 2 || hidden.days != 2 || hidden.confusions != 0 {
		t.Fatalf("hidden counts = %+v", hidden)
	}
	if hidden.first != "2026-11-01" || hidden.last != "2026-11-10" {
		t.Fatalf("span after the middle void = %s..%s", hidden.first, hidden.last)
	}
	if len(hidden.attemptIDs) != 2 || hidden.attemptIDs[0] != f.attempts[0] || hidden.attemptIDs[1] != f.attempts[2] {
		t.Fatalf("attempts = %v", hidden.attemptIDs)
	}
	if len(hidden.latencies) != 2 || hidden.latencies[0] != 800 || hidden.latencies[1] != 1000 {
		t.Fatalf("latencies = %v", hidden.latencies)
	}
	if hidden.recentN != 2 || hidden.history != 2 || hidden.retentionDue != 2 || hidden.pairs != 0 || hidden.miss {
		t.Fatalf("other readers = %+v", hidden)
	}

	shown := snapReaders(t, s, f, EvidenceOpts{IncludeVoided: true})
	if !reflect.DeepEqual(shown, before) {
		t.Fatalf("include-voided:\n got %+v\nwant %+v", shown, before)
	}

	listed, err := s.ListAnswerVoidsByLearner(ctx, f.user)
	if err != nil {
		t.Fatal(err)
	}
	byBatch, err := s.ListAnswerVoidsByBatch(ctx, "batch-middle")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || len(byBatch) != 1 || listed[0].ID != byBatch[0].ID || listed[0].AttemptID != f.attempts[1] {
		t.Fatalf("list learner %+v batch %+v", listed, byBatch)
	}

	undoneAt := voidedAt.Add(time.Hour)
	if err := s.UndoAnswerVoidBatch(ctx, "batch-middle", undoneAt); err != nil {
		t.Fatal(err)
	}
	if a, r, c := evidenceRowCounts(t, s); a != attemptsBefore || r != reviewsBefore || c != confusionsBefore {
		t.Fatalf("undo edited evidence rows: attempts %d review %d confusion %d", a, r, c)
	}
	restored := snapReaders(t, s, f, EvidenceOpts{})
	if !reflect.DeepEqual(restored, before) {
		t.Fatalf("after undo:\n got %+v\nwant %+v", restored, before)
	}
	undone, err := s.ListAnswerVoidsByBatch(ctx, "batch-middle")
	if err != nil {
		t.Fatal(err)
	}
	if len(undone) != 1 || !undone[0].UndoneAt.Equal(undoneAt.UTC()) {
		t.Fatalf("undone_at = %+v", undone)
	}
	if err := s.UndoAnswerVoidBatch(ctx, "batch-middle", undoneAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, err := s.ListAnswerVoidsByBatch(ctx, "batch-middle")
	if err != nil {
		t.Fatal(err)
	}
	if !again[0].UndoneAt.Equal(undoneAt.UTC()) {
		t.Fatalf("second undo rewrote undone_at to %s", again[0].UndoneAt)
	}
}

func TestVoidOfEarliestDayMovesTheSpan(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	f := seedThreeDayEvidence(t, s)
	if _, err := s.InsertAnswerVoids(ctx, "batch-first", when, []AnswerVoidWrite{{
		AttemptID: f.attempts[0], ReviewLogID: f.reviews[0],
		LearnerID: f.user, Reason: VoidReasonUnit1234, Actor: "hebrew void-evidence",
	}}); err != nil {
		t.Fatal(err)
	}
	got := snapReaders(t, s, f, EvidenceOpts{})
	if got.days != 2 || got.first != "2026-11-05" || got.last != "2026-11-10" {
		t.Fatalf("span = %d %s..%s", got.days, got.first, got.last)
	}
	full := snapReaders(t, s, f, EvidenceOpts{IncludeVoided: true})
	if full.days != 3 || full.first != "2026-11-01" || full.last != "2026-11-10" {
		t.Fatalf("include span = %d %s..%s", full.days, full.first, full.last)
	}
}

func TestAttemptOnlyVoidLeavesTheReview(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	f := seedThreeDayEvidence(t, s)
	if _, err := s.InsertAnswerVoids(ctx, "batch-attempt", when, []AnswerVoidWrite{{
		AttemptID: f.attempts[2], LearnerID: f.user, Reason: VoidReasonReport, Actor: "learner",
	}}); err != nil {
		t.Fatal(err)
	}
	got := snapReaders(t, s, f, EvidenceOpts{})
	if got.days != 3 || got.successes != 3 || got.outcomes != 2 || len(got.attemptIDs) != 2 {
		t.Fatalf("attempt-only void = %+v", got)
	}
}

func TestVoidInsertRejectsAndIsolatesLearners(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	f := seedThreeDayEvidence(t, s)
	other := insertUser(t, s, "other-void@example.com", "sub-other-void", "learner")
	row := AnswerVoidWrite{
		AttemptID: f.attempts[0], LearnerID: f.user, Reason: VoidReasonNoCueBlank, Actor: "learner",
	}
	if _, err := s.InsertAnswerVoids(ctx, "", when, []AnswerVoidWrite{row}); err == nil {
		t.Fatal("empty batch id was accepted")
	}
	if _, err := s.InsertAnswerVoids(ctx, "batch", time.Time{}, []AnswerVoidWrite{row}); err == nil {
		t.Fatal("zero time was accepted")
	}
	if _, err := s.InsertAnswerVoids(ctx, "batch", when, nil); err == nil {
		t.Fatal("empty batch was accepted")
	}
	bad := row
	bad.Reason = "bored"
	if _, err := s.InsertAnswerVoids(ctx, "batch", when, []AnswerVoidWrite{bad}); err == nil {
		t.Fatal("unknown reason was accepted")
	}
	stolen := row
	stolen.LearnerID = other
	if _, err := s.InsertAnswerVoids(ctx, "batch", when, []AnswerVoidWrite{stolen}); err == nil {
		t.Fatal("another learner's attempt was voided")
	}
	missing := row
	missing.AttemptID = 999999
	if _, err := s.InsertAnswerVoids(ctx, "batch", when, []AnswerVoidWrite{missing}); err == nil {
		t.Fatal("missing attempt was voided")
	}
	if err := s.UndoAnswerVoidBatch(ctx, "no-such-batch", when); err == nil {
		t.Fatal("unknown batch was undone")
	}
	if _, err := s.InsertAnswerVoids(ctx, "kept", when, []AnswerVoidWrite{row}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListAnswerVoidsByLearner(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("other learner sees %d voids", len(listed))
	}
	own, err := s.ListAnswerVoidsByLearner(ctx, f.user)
	if err != nil || len(own) != 1 || own[0].Reason != VoidReasonNoCueBlank {
		t.Fatalf("own voids = %+v %v", own, err)
	}
}

func TestListenReplaySkipsAVoidedAttempt(t *testing.T) {
	when := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "listen-void@example.com", "sub-listen-void", "learner")
	lex := mustLexemeText(t, s, "שׁוֹמֵעַ")
	res := mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response,
		correct, latency_ms, created_at
	) VALUES (?, ?, 'listen', 'listen_pick', 'review', 'p', 'r', 1, 500, ?)`,
		user, lex, FormatTime(when))
	attemptID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	targets, err := s.ListenReplayTargets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTarget(targets, user, lex) {
		t.Fatalf("targets = %+v, want the listen attempt", targets)
	}
	if _, err := s.InsertAnswerVoids(ctx, "batch-listen", when, []AnswerVoidWrite{{
		AttemptID: attemptID, LearnerID: user, Reason: VoidReasonReport, Actor: "learner",
	}}); err != nil {
		t.Fatal(err)
	}
	targets, err = s.ListenReplayTargets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hasTarget(targets, user, lex) {
		t.Fatal("voided listen attempt is still a replay target")
	}
	targets, err = s.ListenReplayTargetsOpts(ctx, EvidenceOpts{IncludeVoided: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTarget(targets, user, lex) {
		t.Fatal("include-voided dropped the listen attempt")
	}
}

func hasTarget(targets []LexemeSkill, user, lex int64) bool {
	for _, t := range targets {
		if t.UserID == user && t.LexemeID == lex {
			return true
		}
	}
	return false
}
