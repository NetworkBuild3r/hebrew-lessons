package db

import (
	"context"
	"testing"
	"time"
)

func TestActivityPoolResolvesClipFromEitherSource(t *testing.T) {
	when := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "pool@example.com", "sub-pool", "learner")

	assetOnly := insertLexeme(t, s, "שָׁלוֹם")
	idOnly := insertLexeme(t, s, "מַיִם")
	neither := insertLexeme(t, s, "אֵין")
	assetID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lexemeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := s.InsertAudio(ctx, Asset{
		ID: assetID, TextNFC: "שָׁלוֹם", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	// audio_id is a foreign key. This row's text is not the word, so the clip is the id alone.
	if _, err := s.InsertAudio(ctx, Asset{
		ID: lexemeID, TextNFC: "unused-id-clip", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE lexemes SET gloss_en = 'hello' WHERE id = ?`, assetOnly)
	mustExec(t, s, `UPDATE lexemes SET gloss_en = 'water', audio_id = ? WHERE id = ?`, lexemeID, idOnly)
	mustExec(t, s, `UPDATE lexemes SET gloss_en = 'none' WHERE id = ?`, neither)

	got, err := s.ActivityPool(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]PoolWord{}
	for _, w := range got {
		byID[w.ID] = w
	}
	if byID[assetOnly].AudioID != assetID {
		t.Fatalf("audio_assets-only clip = %q, want %s", byID[assetOnly].AudioID, assetID)
	}
	if byID[idOnly].AudioID != lexemeID {
		t.Fatalf("audio_id-only clip = %q, want %s", byID[idOnly].AudioID, lexemeID)
	}
	if byID[neither].AudioID != "" {
		t.Fatalf("clipless word clip = %q", byID[neither].AudioID)
	}
}

func TestDueCountOmitsCliplessListen(t *testing.T) {
	when := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "due@example.com", "sub-due", "learner")

	assetWord := insertLexeme(t, s, "שָׁלוֹם")
	idWord := insertLexeme(t, s, "מַיִם")
	bareWord := insertLexeme(t, s, "אֵין")
	readWord := insertLexeme(t, s, "בַּיִת")
	assetID := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	lexemeAudio := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	if _, err := s.InsertAudio(ctx, Asset{
		ID: assetID, TextNFC: "שָׁלוֹם", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertAudio(ctx, Asset{
		ID: lexemeAudio, TextNFC: "unused-id-clip", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE lexemes SET audio_id = ? WHERE id = ?`, lexemeAudio, idWord)

	insert := func(lex int64, skill string, due time.Time) {
		t.Helper()
		if _, err := s.InsertCard(ctx, Card{
			UserID: user, LexemeID: lex, Skill: skill, State: "new", Due: due,
		}); err != nil {
			t.Fatal(err)
		}
	}
	insert(assetWord, "listen", when.Add(-time.Hour))
	insert(idWord, "listen", when.Add(-time.Hour))
	insert(bareWord, "listen", when.Add(-time.Hour))
	insert(readWord, "read", when.Add(-time.Hour))

	due, err := s.ListDueCards(ctx, user, when)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range due {
		got[c.Skill]++
		if c.LexemeID == bareWord {
			t.Fatal("clipless listen card is due")
		}
	}
	if got["listen"] != 2 || got["read"] != 1 || len(due) != 3 {
		t.Fatalf("due = %+v (%d cards), want two listen cards and one read card", got, len(due))
	}
	n, err := s.CountDueNewCards(ctx, user, when)
	if err != nil || n != 3 {
		t.Fatalf("due new = %d, %v; want 3", n, err)
	}
}

func TestCreateNewCardsListenRequiresClip(t *testing.T) {
	when := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "new@example.com", "sub-new", "learner")
	bare := insertLexeme(t, s, "אֵין")
	assetWord := insertLexeme(t, s, "שָׁלוֹם")
	idWord := insertLexeme(t, s, "מַיִם")
	if _, err := s.InsertAudio(ctx, Asset{
		ID:      "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		TextNFC: "שָׁלוֹם", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	idClip := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, err := s.InsertAudio(ctx, Asset{
		ID: idClip, TextNFC: "unused-id-clip", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE lexemes SET audio_id = ? WHERE id = ?`, idClip, idWord)

	always := func(bool) []string { return []string{"listen", "read"} }
	out, err := s.CreateNewCards(ctx, user, []int64{bare, assetWord, idWord}, when, always)
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]string{}
	for _, w := range out.Words {
		by[w.LexemeID] = joinSkills(w.Created)
	}
	if by[bare] != "read" {
		t.Fatalf("clipless created %q, want read only", by[bare])
	}
	if by[assetWord] != "listen,read" || by[idWord] != "listen,read" {
		t.Fatalf("clipped created %#v", by)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards WHERE lexeme_id = ? AND skill = 'listen'`, bare); n != 0 {
		t.Fatalf("clipless listen cards = %d", n)
	}
}

func TestRecordedListenCardRequiresClip(t *testing.T) {
	when := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "ev@example.com", "sub-ev", "learner")
	bare := insertLexeme(t, s, "אֵין")
	clipped := insertLexeme(t, s, "שָׁלוֹם")
	if _, err := s.InsertAudio(ctx, Asset{
		ID:      "1111111111111111111111111111111111111111111111111111111111111111",
		TextNFC: "שָׁלוֹם", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: when,
	}); err != nil {
		t.Fatal(err)
	}

	schedule := func(lex int64) (AttemptResult, error) {
		return s.WriteAttempt(ctx, AttemptWrite{
			Attempt: Attempt{
				UserID: user, LexemeID: lex, Skill: "listen", Activity: "listen_pick",
				Source: "review", Prompt: "p", Response: "r", Correct: true, CreatedAt: when,
			},
			Step: func(prev *Card) (Card, ReviewLog, bool, error) {
				return Card{State: "new", Due: when}, ReviewLog{Rating: 3, ReviewedAt: when}, true, nil
			},
		})
	}
	res, err := schedule(bare)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scheduled || res.CardID != 0 {
		t.Fatalf("clipless listen write scheduled a card: %+v", res)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards WHERE lexeme_id = ? AND skill = 'listen'`, bare); n != 0 {
		t.Fatalf("clipless listen cards = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM attempts WHERE lexeme_id = ?`, bare); n != 1 {
		t.Fatalf("attempts = %d, want the answer kept", n)
	}

	res, err = schedule(clipped)
	if err != nil || !res.Scheduled || res.CardID == 0 {
		t.Fatalf("clipped listen write = %+v, %v", res, err)
	}

	replayed, err := s.WriteReplayedCard(ctx, Card{
		UserID: user, LexemeID: bare, Skill: "listen", State: "review", Due: when, LastReview: when, Reps: 1,
	}, []ReviewLog{{Rating: 3, ReviewedAt: when}})
	if err != nil || replayed {
		t.Fatalf("replay clipless = %v, %v", replayed, err)
	}
}

func joinSkills(skills []string) string {
	out := ""
	for i, s := range skills {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
