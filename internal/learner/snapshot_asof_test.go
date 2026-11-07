package learner

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func snapshotsEqual(t *testing.T, live, atNow Snapshot) {
	t.Helper()
	if live.ReplayMismatch != atNow.ReplayMismatch {
		t.Fatalf("ReplayMismatch live=%d atNow=%d", live.ReplayMismatch, atNow.ReplayMismatch)
	}
	if !reflect.DeepEqual(live.Summary, atNow.Summary) {
		t.Fatalf("Summary differs\nlive=%+v\natNow=%+v", live.Summary, atNow.Summary)
	}
	if live.Bands != atNow.Bands {
		t.Fatalf("Bands live=%+v atNow=%+v", live.Bands, atNow.Bands)
	}
	if len(live.Words) != len(atNow.Words) {
		t.Fatalf("Words len live=%d atNow=%d", len(live.Words), len(atNow.Words))
	}
	for id, w := range live.Words {
		got, ok := atNow.Words[id]
		if !ok {
			t.Fatalf("word %d missing from SnapshotAt(now)", id)
		}
		if !reflect.DeepEqual(w, got) {
			t.Fatalf("word %d differs\nlive=%+v\natNow=%+v", id, w, got)
		}
	}
}

// AC1: three fixtures — SnapshotAt(now) equals Snapshot with ReplayMismatch 0.
func TestSnapshotAtNowEqualsSnapshotThreeFixtures(t *testing.T) {
	ctx := context.Background()

	t.Run("reading_only", func(t *testing.T) {
		e := newEnv(t)
		a, b := e.lexeme(), e.lexeme()
		e.gradeCard(a, fsrs.SkillRead, 24*time.Hour, fsrs.Good, fsrs.Good, fsrs.Good)
		e.gradeCard(b, fsrs.SkillRead, 12*time.Hour, fsrs.Good, fsrs.Hard, fsrs.Good)
		assertLiveEqualsAtNow(t, e, ctx)
	})

	t.Run("reading_recall_with_lapses", func(t *testing.T) {
		e := newEnv(t)
		w := e.lexeme()
		e.gradeCard(w, fsrs.SkillRead, 24*time.Hour, fsrs.Good, fsrs.Good, fsrs.Good, fsrs.Again, fsrs.Good)
		e.gradeCard(w, fsrs.SkillRecall, 24*time.Hour, fsrs.Good, fsrs.Again, fsrs.Good, fsrs.Good)
		assertLiveEqualsAtNow(t, e, ctx)
	})

	t.Run("placement_and_typed_spelling", func(t *testing.T) {
		e := newEnv(t)
		lex := e.lexeme()
		e.seedListeningRun4(e.clk.Now().Add(-time.Hour))
		// Typed recall with letters gate open creates a spelling card + implied review.
		recallID, err := e.store.InsertCard(ctx, db.Card{
			UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall,
			State: fsrs.StateNew, Due: e.clk.Now(),
		})
		if err != nil {
			t.Fatal(err)
		}
		rev := fsrs.New(e.store, e.clk)
		if err := rev.Grade(ctx, recallID, fsrs.Good, false); err != nil {
			t.Fatal(err)
		}
		e.clk.t = e.clk.t.Add(time.Hour)
		rec := evidence.New(e.store, rev, e.clk)
		_, err = rec.Attempt(ctx, evidence.Attempt{
			UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall,
			Activity: "type_it", Source: evidence.SourceReview,
			Correct: true, Latency: 3 * time.Second, LettersPassed: true,
			Prompt: "word", Response: "מילה",
		})
		if err != nil {
			t.Fatal(err)
		}
		assertLiveEqualsAtNow(t, e, ctx)
	})
}

func assertLiveEqualsAtNow(t *testing.T, e *env, ctx context.Context) {
	t.Helper()
	live, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	atNow, err := e.model.SnapshotAt(ctx, e.user, e.clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if live.ReplayMismatch != 0 {
		t.Fatalf("ReplayMismatch = %d, want 0", live.ReplayMismatch)
	}
	snapshotsEqual(t, live, atNow)
}

// AC2: learning at end of day 2, known at end of day 3.
func TestSnapshotAtLearningThenKnownAcrossDays(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	day1 := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	endDay2 := time.Date(2026, 10, 19, 23, 59, 0, 0, time.UTC)
	day3Noon := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	endDay3 := time.Date(2026, 10, 20, 23, 59, 0, 0, time.UTC)

	e.clk.t = day1
	lex := e.lexeme()
	id, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRead,
		State: fsrs.StateNew, Due: e.clk.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rev := fsrs.New(e.store, e.clk)
	if err := rev.Grade(ctx, id, fsrs.Good, false); err != nil {
		t.Fatal(err)
	}
	e.clk.t = day1.Add(24 * time.Hour) // day 2 noon
	if err := rev.Grade(ctx, id, fsrs.Good, false); err != nil {
		t.Fatal(err)
	}
	// Two successes → still learning at end of day 2.
	e.clk.t = endDay3 // clock ahead so live reads see everything; SnapshotAt uses at
	snap2, err := e.model.SnapshotAt(ctx, e.user, endDay2)
	if err != nil {
		t.Fatal(err)
	}
	w2, ok := snap2.Words[lex]
	if !ok {
		t.Fatal("word missing at end of day 2")
	}
	if st := skillOf(w2, fsrs.SkillRead).State; st != Learning {
		t.Fatalf("day2 state = %s, want learning (word=%+v)", st, skillOf(w2, fsrs.SkillRead))
	}

	// Third success on day 3 → known by end of day 3.
	e.clk.t = day3Noon
	if err := rev.Grade(ctx, id, fsrs.Good, false); err != nil {
		t.Fatal(err)
	}
	e.clk.t = endDay3
	snap3, err := e.model.SnapshotAt(ctx, e.user, endDay3)
	if err != nil {
		t.Fatal(err)
	}
	w3, ok := snap3.Words[lex]
	if !ok {
		t.Fatal("word missing at end of day 3")
	}
	if st := skillOf(w3, fsrs.SkillRead).State; st != Known && st != Solid {
		t.Fatalf("day3 state = %s, want known/solid (skill=%+v)", st, skillOf(w3, fsrs.SkillRead))
	}
	// Past day 2 view must still be learning after the day-3 grade.
	snap2b, err := e.model.SnapshotAt(ctx, e.user, endDay2)
	if err != nil {
		t.Fatal(err)
	}
	if st := skillOf(snap2b.Words[lex], fsrs.SkillRead).State; st != Learning {
		t.Fatalf("day2 after later grade = %s, want learning", st)
	}
}

// AC3: an attempt after at does not change SnapshotAt(at).
func TestSnapshotAtIgnoresLaterAttempt(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	at := e.clk.Now().Add(-time.Hour)
	lex := e.lexeme()
	e.clk.t = at.Add(-30 * time.Minute)
	cardID := e.gradeCard(lex, fsrs.SkillRead, 0, fsrs.Good)

	before, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}

	// Later wrong attempt and another review, both after `at`.
	e.clk.t = at.Add(2 * time.Hour)
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'test', 'review', '', '', 0, 0, 0, 0, ?)`,
		e.user, lex, fsrs.SkillRead, db.FormatTime(e.clk.Now()))
	if err := fsrs.New(e.store, e.clk).Grade(ctx, cardID, fsrs.Again, false); err != nil {
		t.Fatal(err)
	}

	after, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Summary, after.Summary) {
		t.Fatalf("Summary changed after later evidence\nbefore=%+v\nafter=%+v", before.Summary, after.Summary)
	}
	if !reflect.DeepEqual(before.Words[lex], after.Words[lex]) {
		t.Fatalf("word changed\nbefore=%+v\nafter=%+v", before.Words[lex], after.Words[lex])
	}
}

// The daily sweep lands moments after midnight. A past moment that close to the
// clock is still the past: answers after it are not counted and cards are replayed.
func TestSnapshotAtSecondsAgoIsStillThePast(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	at := e.clk.Now().Add(-time.Hour)
	lex := e.lexeme()
	e.clk.t = at.Add(-30 * time.Minute)
	cardID := e.gradeCard(lex, fsrs.SkillRead, 0, fsrs.Good)

	e.clk.t = at
	before, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}

	e.clk.t = at.Add(20 * time.Second)
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'test', 'review', '', '', 0, 0, 0, 0, ?)`,
		e.user, lex, fsrs.SkillRead, db.FormatTime(e.clk.Now()))
	if err := fsrs.New(e.store, e.clk).Grade(ctx, cardID, fsrs.Again, false); err != nil {
		t.Fatal(err)
	}

	e.clk.t = at.Add(30 * time.Second)
	after, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Summary, after.Summary) {
		t.Fatalf("Summary changed by answers after at\nbefore=%+v\nafter=%+v", before.Summary, after.Summary)
	}
	if !reflect.DeepEqual(before.Words[lex], after.Words[lex]) {
		t.Fatalf("word changed by answers after at\nbefore=%+v\nafter=%+v", before.Words[lex], after.Words[lex])
	}
	if after.ReplayMismatch != 0 {
		t.Fatalf("ReplayMismatch = %d on a past snapshot, want 0", after.ReplayMismatch)
	}
}

// AC4: reading PlacementRunAt comes from the older reading run.
func TestSnapshotPlacementRunAtFromOlderReading(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	when := e.clk.Now().Add(-time.Hour)
	lex, err := e.store.UpsertPlacementLexeme(ctx, "סֵפֶר", "book", "noun", "sefer", "")
	if err != nil {
		t.Fatal(err)
	}

	readFinish := when.Add(time.Minute)
	readRun, err := e.store.StartPlacementV3(ctx, e.user, when, 2, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimPlacement(e, readRun, 1, db.PartReading, "read", 2, "r-1", "right", lex, when)
	claimPlacement(e, readRun, 2, db.PartReading, "read", 2, "r-2", "wrong", lex, when)
	body := `{"bands":[{"band":0,"rated":0,"good_plus_rate":0},{"band":1,"rated":0,"good_plus_rate":0},{"band":2,"rated":2,"good":1,"again":1,"good_plus_rate":0.5}]}`
	if err := e.store.FinishPlacement(ctx, readRun, readFinish, 1, "band_stop", body); err != nil {
		t.Fatal(err)
	}

	listenFinish := when.Add(3 * time.Minute)
	listenRun, err := e.store.StartPlacementV3(ctx, e.user, when.Add(2*time.Minute), 3, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimPlacement(e, listenRun, 1, db.PartListening, "listen", 1, "l-1", "right", lex, when.Add(2*time.Minute))
	if err := e.store.FinishPlacement(ctx, listenRun, listenFinish, 1, "band_stop",
		`{"bands":[{"band":0,"rated":1,"good":1,"good_plus_rate":1}]}`); err != nil {
		t.Fatal(err)
	}

	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]SkillSummary{}
	for _, s := range snap.Summary.Skills {
		byName[s.Skill] = s
	}
	if !byName[fsrs.SkillRead].PlacementRunAt.Equal(readFinish) {
		t.Fatalf("read PlacementRunAt = %v, want older reading %v", byName[fsrs.SkillRead].PlacementRunAt, readFinish)
	}
	if !byName[fsrs.SkillListen].PlacementRunAt.Equal(listenFinish) {
		t.Fatalf("listen PlacementRunAt = %v, want %v", byName[fsrs.SkillListen].PlacementRunAt, listenFinish)
	}
}

// AC5: edited stored stability → ReplayMismatch 1; Snapshot still uses stored state.
func TestSnapshotReplayMismatchEditedStability(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme()
	id := e.gradeCard(lex, fsrs.SkillRead, 24*time.Hour, fsrs.Good, fsrs.Good, fsrs.Good)

	agree, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if agree.ReplayMismatch != 0 {
		t.Fatalf("before edit ReplayMismatch = %d", agree.ReplayMismatch)
	}
	storedBefore := skillOf(agree.Words[lex], fsrs.SkillRead).Stability

	e.exec(`UPDATE cards SET stability = stability + 10 WHERE id = ?`, id)

	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ReplayMismatch != 1 {
		t.Fatalf("ReplayMismatch = %d, want 1", snap.ReplayMismatch)
	}
	got := skillOf(snap.Words[lex], fsrs.SkillRead).Stability
	if got != storedBefore+10 {
		t.Fatalf("Snapshot stability = %v, want stored %v (not replay)", got, storedBefore+10)
	}
}

// Introduced but never reviewed: learning after creation, absent before creation.
func TestSnapshotAtUnreviewedCardCreationWindow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	before := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	created := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 10, 18, 14, 0, 0, 0, time.UTC)

	e.clk.t = created
	lex := e.lexeme()
	if _, err := e.store.CreateNewCards(ctx, e.user, []int64{lex}, created, func(bool) []string {
		return []string{fsrs.SkillRead}
	}); err != nil {
		t.Fatal(err)
	}
	e.clk.t = after

	past, err := e.model.SnapshotAt(ctx, e.user, before)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := past.Words[lex]; ok {
		t.Fatalf("word present before creation: %+v", past.Words[lex])
	}

	mid, err := e.model.SnapshotAt(ctx, e.user, after)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := mid.Words[lex]
	if !ok {
		t.Fatal("word missing after creation")
	}
	if st := skillOf(w, fsrs.SkillRead).State; st != Learning {
		t.Fatalf("state = %s, want learning (skill=%+v)", st, skillOf(w, fsrs.SkillRead))
	}
}

// A card created after at with no earlier evidence must not appear in Words.
func TestSnapshotAtOmitsGhostWordForLaterCard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	e.clk.t = at.Add(2 * time.Hour)
	lex := e.lexeme()
	if _, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRead,
		State: fsrs.StateNew, Due: e.clk.Now(), CreatedAt: e.clk.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	e.clk.t = at.Add(3 * time.Hour)

	snap, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Words[lex]; ok {
		t.Fatalf("ghost word in SnapshotAt: %+v", snap.Words[lex])
	}
}

// LastReview-only edit still counts as ReplayMismatch.
func TestSnapshotReplayMismatchEditedLastReview(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme()
	id := e.gradeCard(lex, fsrs.SkillRead, 24*time.Hour, fsrs.Good, fsrs.Good, fsrs.Good)

	agree, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if agree.ReplayMismatch != 0 {
		t.Fatalf("before edit ReplayMismatch = %d", agree.ReplayMismatch)
	}

	e.exec(`UPDATE cards SET last_review = ? WHERE id = ?`,
		db.FormatTime(e.clk.Now().Add(2*time.Hour)), id)

	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ReplayMismatch != 1 {
		t.Fatalf("ReplayMismatch = %d, want 1", snap.ReplayMismatch)
	}
}

// An exposure after at does not mark the word seen as of at.
func TestSnapshotAtIgnoresLaterExposure(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	at := e.clk.Now().Add(-time.Hour)
	lex := e.lexeme()
	e.exec(`INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (?, ?, 'story', ?)`,
		e.user, lex, db.FormatTime(at.Add(2*time.Hour)))

	snap, err := e.model.SnapshotAt(ctx, e.user, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Words[lex]; ok {
		t.Fatalf("later exposure leaked into SnapshotAt: %+v", snap.Words[lex])
	}
	live, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if w, ok := live.Words[lex]; !ok || w.State != Seen {
		t.Fatalf("live snapshot = %+v, want seen from exposure", live.Words[lex])
	}
}
