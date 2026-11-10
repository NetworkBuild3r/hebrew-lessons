package evidence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// A void replays the card the recorder would have stored without that answer,
// including the study-day cutoff. The voided answer sits first, in the middle,
// and last. INIT-038/SPEC-004.

func newZonedEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	if err := e.store.SetUserTimeZone(context.Background(), e.user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	e.clk.t = time.Date(2026, 10, 21, 16, 0, 0, 0, loc)
	return e
}

func (e *env) latestAttemptID() int64 {
	e.t.Helper()
	var id int64
	if err := e.conn().QueryRow(`SELECT id FROM attempts ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func scheduleEqual(a, b db.Card) bool {
	return a.State == b.State &&
		a.Stability == b.Stability &&
		a.Difficulty == b.Difficulty &&
		a.Reps == b.Reps &&
		a.Lapses == b.Lapses &&
		a.Due.Equal(b.Due) &&
		a.LastReview.Equal(b.LastReview)
}

func formatSchedule(c db.Card) string {
	return fmt.Sprintf("%s stab=%g diff=%g due=%s reps=%d lapses=%d last=%s",
		c.State, c.Stability, c.Difficulty, c.Due.UTC().Format(time.RFC3339Nano),
		c.Reps, c.Lapses, c.LastReview.UTC().Format(time.RFC3339Nano))
}

func TestVoidMatchesTheCardWithoutThatAnswer(t *testing.T) {
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 21, 16, 0, 0, 0, loc)
	times := []time.Time{start, start.Add(48 * time.Hour), start.Add(10 * 24 * time.Hour)}
	right := []bool{true, true, false}
	positions := []struct {
		name  string
		index int
	}{
		{"first", 0},
		{"middle", 1},
		{"last", 2},
	}
	for _, pos := range positions {
		t.Run(pos.name, func(t *testing.T) {
			full := newZonedEnv(t)
			twin := newZonedEnv(t)
			lexF := full.lexeme(kelev)
			lexT := twin.lexeme(kelev)
			var before db.Card
			var ids [3]int64
			for i, tm := range times {
				full.clk.t = tm
				twin.clk.t = tm
				if pos.index == 2 && i == 2 {
					card, ok := full.card(lexF, fsrs.SkillRead)
					if !ok {
						t.Fatal("no card before the last answer")
					}
					before = card
				}
				if i != pos.index {
					a := twin.attempt(lexT)
					a.Correct = right[i]
					if _, err := twin.rec.Attempt(context.Background(), a); err != nil {
						t.Fatal(err)
					}
				}
				a := full.attempt(lexF)
				a.Correct = right[i]
				if _, err := full.rec.Attempt(context.Background(), a); err != nil {
					t.Fatal(err)
				}
				ids[i] = full.latestAttemptID()
			}
			batch, err := full.rec.VoidAnswers(context.Background(), "", []AnswerVoid{{
				AttemptID: ids[pos.index],
				Reason:    db.VoidReasonAlsoRight,
				Actor:     "learner",
			}})
			if err != nil || batch == "" {
				t.Fatalf("void batch = %q, err = %v", batch, err)
			}
			got, ok := full.card(lexF, fsrs.SkillRead)
			want, wantOK := twin.card(lexT, fsrs.SkillRead)
			if !ok || !wantOK {
				t.Fatalf("card after void ok=%v, twin ok=%v", ok, wantOK)
			}
			if !scheduleEqual(got, want) {
				t.Fatalf("card\n got %s\nwant %s", formatSchedule(got), formatSchedule(want))
			}
			if pos.index == 2 && !scheduleEqual(got, before) {
				t.Fatalf("last answer did not return to the card before it\n got %s\nbefore %s", formatSchedule(got), formatSchedule(before))
			}
			if n := full.count(`SELECT COUNT(*) FROM attempts`); n != 3 {
				t.Fatalf("attempts = %d, want 3; a void does not delete the row", n)
			}
			if n := full.count(`SELECT COUNT(*) FROM review_log`); n != 3 {
				t.Fatalf("review_log = %d, want 3; a void does not delete the row", n)
			}
		})
	}
}

func TestVoidReplayKeepsTheStudyDayCutoff(t *testing.T) {
	e := newZonedEnv(t)
	ctx := context.Background()
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	zone := studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
	lex := e.lexeme(kelev)
	first := e.clk.t
	if _, err := e.rec.Attempt(ctx, e.attempt(lex)); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(48 * time.Hour)
	if _, err := e.rec.Attempt(ctx, e.attempt(lex)); err != nil {
		t.Fatal(err)
	}
	second := e.latestAttemptID()
	if _, err := e.rec.VoidAnswers(ctx, "", []AnswerVoid{{
		AttemptID: second, Reason: db.VoidReasonAlsoRight, Actor: "learner",
	}}); err != nil {
		t.Fatal(err)
	}
	card, ok := e.card(lex, fsrs.SkillRead)
	if !ok {
		t.Fatal("no card")
	}
	cutoff := studyday.DueCutoff(first, zone)
	raw, _, err := fsrs.New(nil, nil).ScheduleNext(nil, first, fsrs.Good)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Due.Equal(cutoff) {
		t.Fatal("fixture: the raw learning step is already the cutoff, so the test proves nothing")
	}
	if !card.Due.Equal(cutoff) {
		t.Fatalf("due = %s, want cutoff %s (raw %s)", card.Due, cutoff, raw.Due)
	}
}

func TestVoidThenUndoRestoresTheCard(t *testing.T) {
	e := newZonedEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	var ids [3]int64
	for i := 0; i < 3; i++ {
		a := e.attempt(lex)
		a.Correct = i != 1
		if _, err := e.rec.Attempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		ids[i] = e.latestAttemptID()
		e.clk.Advance(48 * time.Hour)
	}
	before, ok := e.card(lex, fsrs.SkillRead)
	if !ok {
		t.Fatal("no card")
	}
	batch, err := e.rec.VoidAnswers(ctx, "batch-undo", []AnswerVoid{{
		AttemptID: ids[1], Reason: db.VoidReasonAlsoRight, Actor: "learner",
	}})
	if err != nil || batch != "batch-undo" {
		t.Fatalf("batch = %q, err = %v", batch, err)
	}
	changed, ok := e.card(lex, fsrs.SkillRead)
	if !ok || scheduleEqual(changed, before) {
		t.Fatalf("void left the card unchanged: %+v", changed)
	}
	if err := e.rec.UndoVoid(ctx, batch); err != nil {
		t.Fatal(err)
	}
	got, ok := e.card(lex, fsrs.SkillRead)
	if !ok || !scheduleEqual(got, before) {
		t.Fatalf("after undo\n got %s\nwant %s", formatSchedule(got), formatSchedule(before))
	}
	var undone int
	if err := e.conn().QueryRow(`SELECT COUNT(*) FROM answer_voids WHERE batch_id = ? AND undone_at IS NOT NULL`, batch).Scan(&undone); err != nil {
		t.Fatal(err)
	}
	if undone != 1 {
		t.Fatalf("undone rows = %d, want 1", undone)
	}
}

func TestVoidedConfusionNoLongerCounts(t *testing.T) {
	e := newZonedEnv(t)
	ctx := context.Background()
	asked := e.lexeme(kelev)
	picked := e.lexeme(kala)
	a := e.attempt(asked)
	a.Correct = false
	a.ChosenLexemeID = picked
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	id := e.latestAttemptID()
	var kind, key string
	if err := e.conn().QueryRow(`SELECT kind, key FROM confusion_misses`).Scan(&kind, &key); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses`); n != 1 {
		t.Fatalf("confusion rows = %d, want 1", n)
	}
	if _, err := e.rec.VoidAnswers(ctx, "", []AnswerVoid{{
		AttemptID: id, Reason: db.VoidReasonAlsoRight, Actor: "learner",
	}}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses`); n != 1 {
		t.Fatalf("void deleted the confusion row, count = %d", n)
	}
	if _, ok, err := e.store.LastConfusionMiss(ctx, e.user, kind, key); err != nil || ok {
		t.Fatalf("voided confusion still counts: ok=%v err=%v", ok, err)
	}
	if _, ok, err := e.store.LastConfusionMissOpts(ctx, e.user, kind, key, db.EvidenceOpts{IncludeVoided: true}); err != nil || !ok {
		t.Fatalf("include-voided missed the row: ok=%v err=%v", ok, err)
	}
	var named int
	if err := e.conn().QueryRow(`SELECT COUNT(*) FROM answer_voids WHERE confusion_id IS NOT NULL AND undone_at IS NULL`).Scan(&named); err != nil || named != 1 {
		t.Fatalf("voids naming a confusion = %d, err = %v", named, err)
	}
}

func TestVoidUnitWrongCoversOnlyThatLearnersWrongs(t *testing.T) {
	e := newZonedEnv(t)
	ctx := context.Background()
	other, err := e.store.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetUserTimeZone(ctx, other, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	lex := e.lexeme(kelev)
	otherWord := e.lexeme(shalom)
	unit1, err := e.store.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (1, 'one', 's', 'c', 'published', 1, '{"title":"one"}', 'shared')`)
	if err != nil {
		t.Fatal(err)
	}
	unit1ID, _ := unit1.LastInsertId()
	unit2, err := e.store.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (2, 'two', 's', 'c', 'published', 1, '{"title":"two"}', 'shared')`)
	if err != nil {
		t.Fatal(err)
	}
	unit2ID, _ := unit2.LastInsertId()

	record := func(user, word, unit int64, correct bool) {
		t.Helper()
		e.clk.Advance(time.Hour)
		a := e.attempt(word)
		a.UserID = user
		a.UnitID = unit
		a.Correct = correct
		rec := e.rec
		if user == other {
			rec = New(e.store, nil, e.clk)
		}
		if _, err := rec.Attempt(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	record(e.user, lex, unit1ID, true)
	beforeWrong, ok := e.card(lex, fsrs.SkillRead)
	if !ok {
		t.Fatal("no card after the right answer")
	}
	record(e.user, lex, unit1ID, false)
	record(e.user, otherWord, unit2ID, false)
	if _, exists := e.cardFor(other, lex, fsrs.SkillRead); exists {
		t.Fatal("the other learner had a card before answering")
	}
	record(other, lex, unit1ID, false)

	batch, err := e.rec.VoidUnitWrong(ctx, e.user, unit1ID, "report")
	if err != nil || batch == "" {
		t.Fatalf("batch = %q, err = %v", batch, err)
	}
	got, ok := e.card(lex, fsrs.SkillRead)
	if !ok || !scheduleEqual(got, beforeWrong) {
		t.Fatalf("unit void\n got %s\nwant %s", formatSchedule(got), formatSchedule(beforeWrong))
	}
	unit2Card, ok := e.card(otherWord, fsrs.SkillRead)
	if !ok || unit2Card.Reps != 1 {
		t.Fatalf("unit 2 card = %+v, want the wrong answer still counted", unit2Card)
	}
	otherCard, ok := e.cardFor(other, lex, fsrs.SkillRead)
	if !ok || otherCard.Reps != 1 {
		t.Fatalf("other learner card = %+v, %v", otherCard, ok)
	}
	if n := e.count(`SELECT COUNT(*) FROM answer_voids WHERE undone_at IS NULL AND reason = ? AND learner_id = ?`, db.VoidReasonReport, e.user); n != 1 {
		t.Fatalf("active report voids = %d, want 1", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM answer_voids WHERE learner_id = ?`, other); n != 0 {
		t.Fatalf("other learner voids = %d", n)
	}
	if e.count(`SELECT COUNT(*) FROM attempts WHERE user_id = ? AND unit_id = ? AND correct = 1`, e.user, unit1ID) != 1 {
		t.Fatal("the right answer was removed")
	}
	again, err := e.rec.VoidUnitWrong(ctx, e.user, unit1ID, "report")
	if err != nil || again != "" {
		t.Fatalf("second unit void = %q, err = %v; an active void is left as it is", again, err)
	}
}

func (e *env) cardFor(user, lexeme int64, skill string) (db.Card, bool) {
	e.t.Helper()
	id, ok, err := e.store.CardID(context.Background(), user, lexeme, skill)
	if err != nil {
		e.t.Fatal(err)
	}
	if !ok {
		return db.Card{}, false
	}
	card, err := e.store.GetCard(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return card, true
}
