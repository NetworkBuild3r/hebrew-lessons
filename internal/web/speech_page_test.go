package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestCoursePage(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-course", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	home := getLearner(t, h, l, "/")
	if !strings.Contains(home, `href="/speech"`) || !strings.Contains(home, "Common words") {
		t.Fatal("idle Today does not offer the common words")
	}
	ctx := context.Background()
	rows := []db.CourseRow{
		{Order: 1, Rank: 1, Lemma: "אם", Pointed: "אִם", Translit: "im", Gloss: "if", POS: "conjunction"},
		{Order: 2, Rank: 2, Lemma: "עם", Pointed: "עִם", Translit: "im", Gloss: "with", POS: "preposition"},
		{Order: 3, Rank: 3, Lemma: "טוב", Pointed: "טוֹב", Translit: "tov", Gloss: "good", POS: "adjective"},
		{Order: 4, Rank: 4, Lemma: "לא", Pointed: "לֹא", Translit: "lo", Gloss: "not", POS: "adverb"},
	}
	if _, err := store.SyncCourse(ctx, rows); err != nil {
		t.Fatal(err)
	}
	course, err := store.CourseLexemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range course[:2] {
		card, err := store.InsertCard(ctx, db.Card{
			UserID: l.user.ID, LexemeID: row.LexemeID, Skill: "read", State: "review",
			Due: now.Add(-time.Hour), Stability: 1, Difficulty: 5, Reps: 5, Lapses: 1,
			LastReview: now.Add(-2 * time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			for n := 0; n < 5; n++ {
				if _, err := store.ExecContext(ctx, `
					INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 1, ?, 1, 1, 0, 0)`,
					card, db.FormatTime(now.Add(-time.Duration(n+10)*time.Hour))); err != nil {
					t.Fatal(err)
				}
			}
		}
		if i == 1 {
			for n := 0; n < 45; n++ {
				if _, err := store.ExecContext(ctx, `
					INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 3, ?, 1, 1, 0, 0)`,
					card, db.FormatTime(now.Add(-time.Duration(n+1)*time.Minute))); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	page := getLearner(t, h, l, "/speech")
	for _, want := range []string{"You have met 2", "data-next", "good", "not", "data-slipping", "if", "data-climb", "data-follow", "Learn 2 new words"} {
		if !strings.Contains(page, want) {
			t.Fatalf("course page missing %q", want)
		}
	}
	next := page[strings.Index(page, `data-next`):]
	if strings.Index(next, "good") > strings.Index(next, "not") {
		t.Fatal("next words are out of order")
	}

	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var rateCard int64
	if err := conn.QueryRow(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, l.user.ID, course[1].LexemeID).Scan(&rateCard); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 50; n++ {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
			VALUES (?, 1, ?, 0, 0, 0, 0)`,
			rateCard, db.FormatTime(now.Add(-time.Duration(n+1)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	low := getLearner(t, h, l, "/speech")
	if !strings.Contains(low, "under 85 out of 100") {
		t.Fatal("low accuracy does not say review only")
	}
}

func TestCourseReviewUsesServerIDs(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "b@example.com", "sub-slip", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	ctx := context.Background()
	if _, err := store.SyncCourse(ctx, []db.CourseRow{
		{Order: 1, Rank: 1, Lemma: "אם", Pointed: "אִם", Translit: "im", Gloss: "if", POS: "conjunction"},
	}); err != nil {
		t.Fatal(err)
	}
	course, err := store.CourseLexemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	card, err := store.InsertCard(ctx, db.Card{
		UserID: l.user.ID, LexemeID: course[0].LexemeID, Skill: "read", State: "review",
		Due: now.Add(-time.Hour), Stability: 1, Difficulty: 5, Reps: 5,
		LastReview: now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 5; n++ {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
			VALUES (?, 1, ?, 1, 1, 0, 0)`,
			card, db.FormatTime(now.Add(-time.Duration(n+3)*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	res, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, translit) VALUES ('פִּלְפֵּל', 'pepper', 'pilpel')`)
	if err != nil {
		t.Fatal(err)
	}
	decoy, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	decoyCard, err := store.InsertCard(ctx, db.Card{
		UserID: l.user.ID, LexemeID: decoy, Skill: "read", State: "review",
		Due: now.Add(-time.Hour), Stability: 5, Difficulty: 5, Reps: 5,
		LastReview: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := formPost(t, h, l, "/_hx/review/course", url.Values{"id": {"999"}, "_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("post = %d", rec.Code)
	}
	page := getLearner(t, h, l, "/review")
	got := mustInt(t, attr(t, cardAttrs(page), "data-card"))
	if int64(got) == decoyCard {
		t.Fatal("the posted id was added to the queue")
	}
}

func TestCoursePageLastWeek(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-week", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	payload := `{"version":1,"weekStart":"2026-11-02","minutesStudied":12,"course":{"place":3,"met":2,"metThisWeek":1,"gained":[],"lost":[{"text":"אִם","cause":"confusion","lexemeId":1,"skill":"read","from":"known","to":"learning","day":"2026-11-03","recovered":false}],"studyDays":2}}`
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at)
		VALUES (?, '2026-11-02', ?, 'A quiet week.', ?)`,
		l.user.ID, payload, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	page := getLearner(t, h, l, "/speech")
	if !strings.Contains(page, "data-last-week") || !strings.Contains(page, "mixed up with another word") {
		t.Fatal("last week does not show the lost word and its cause")
	}
}

func TestCourseReviewNeedsCSRF(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "b@example.com", "sub-csrf", now)
	if rec := formPost(t, h, l, "/_hx/review/course", url.Values{"id": {"1"}}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
}
