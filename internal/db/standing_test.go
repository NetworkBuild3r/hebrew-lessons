package db

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestAttemptTallyCountsCheckedSourcesAndMisses(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "tally@example.com", "sub-tally", "learner")
	other := insertUser(t, s, "other-tally@example.com", "sub-tally-other", "learner")
	a := insertLexeme(t, s, "אָלֶף")
	b := insertLexeme(t, s, "בֵּית")

	insertAttempt(t, s, user, a, "recall", "review", true, false, when.Add(-2*time.Hour))
	insertAttempt(t, s, user, a, "recall", "lesson", false, false, when.Add(-90*time.Minute))
	insertAttempt(t, s, user, a, "recall", "practice", false, true, when.Add(-80*time.Minute))   // dont_know
	insertAttempt(t, s, user, a, "recall", "placement", false, false, when.Add(-70*time.Minute)) // excluded
	insertAttempt(t, s, user, a, "recall", "review", false, false, when.Add(-40*24*time.Hour))   // outside window
	insertAttempt(t, s, user, b, "read", "review", true, false, when.Add(-time.Hour))
	insertAttempt(t, s, other, a, "recall", "review", false, false, when.Add(-time.Hour)) // other learner

	got, err := s.AttemptTally(ctx, user, when.AddDate(0, 0, -14), when)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]AttemptTallyRow{}
	for _, row := range got {
		by[row.Skill+":"+itoa(row.LexemeID)] = row
	}
	recall := by["recall:"+itoa(a)]
	if recall.Answers != 3 || recall.Misses != 2 {
		t.Fatalf("recall tally = %+v, want 3 answers 2 misses", recall)
	}
	if recall.LastMiss.IsZero() || !recall.LastMiss.Equal(when.Add(-80*time.Minute)) {
		t.Fatalf("recall last miss = %v, want %v", recall.LastMiss, when.Add(-80*time.Minute))
	}
	read := by["read:"+itoa(b)]
	if read.Answers != 1 || read.Misses != 0 || !read.LastMiss.IsZero() {
		t.Fatalf("read tally = %+v, want 1 answer 0 misses", read)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (placement and other learner excluded)", len(got))
	}
}

func TestCountLexemesWithAudio(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	n, err := s.CountLexemesWithAudio(ctx)
	if err != nil || n != 0 {
		t.Fatalf("empty = %d, %v", n, err)
	}
	clip := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	mustExec(t, s, `INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, 'בַּיִת', 'v', 'm', 4, ?)`,
		clip, FormatTime(when))
	mustExec(t, s, `INSERT INTO lexemes (text_nfc) VALUES ('שָׁלוֹם')`)                   // NULL audio_id
	mustExec(t, s, `INSERT INTO lexemes (text_nfc, audio_id) VALUES ('בַּיִת', ?)`, clip) // has clip
	mustExec(t, s, `INSERT INTO lexemes (text_nfc) VALUES ('מַיִם')`)
	n, err = s.CountLexemesWithAudio(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count = %d, %v; want 1", n, err)
	}
}

func insertAttempt(t *testing.T, s *Store, user, lexeme int64, skill, source string, correct, dontKnow bool, at time.Time) {
	t.Helper()
	c, d := 0, 0
	if correct {
		c = 1
	}
	if dontKnow {
		d = 1
	}
	mustExec(t, s, `
		INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, 'test', ?, '', '', ?, ?, 0, 0, ?)`,
		user, lexeme, skill, source, c, d, FormatTime(at),
	)
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
