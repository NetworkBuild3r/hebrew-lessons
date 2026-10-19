package placement

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

var when = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func TestStruggleStopOnARun(t *testing.T) {
	if stopAt != 10 || cardCap != 100 {
		t.Fatalf("stopAt=%d cardCap=%d", stopAt, cardCap)
	}
	svc, store := openSvc(t, fixedClock{when})
	ctx := context.Background()
	userID := insertLearner(t, store, "stop@example.com", "sub-stop")
	runID := start(t, svc, userID)
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Seed != when.UnixNano() || !run.StartedAt.Equal(when) {
		t.Fatalf("run seed %d started %s", run.Seed, run.StartedAt)
	}
	order := deck(run.Seed, mustCatalog(t))
	if order[0].Band != 0 {
		t.Fatalf("first card band %d", order[0].Band)
	}
	ratePrefix(t, svc, runID, 10, func(i int) int {
		if i < 6 {
			return Again
		}
		return Good
	})
	if card, done, err := svc.Next(ctx, runID); err != nil || !done || card != nil {
		t.Fatalf("after 6/10: card=%v done=%v err=%v", card, done, err)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != stopBand || result.Level != -1 || result.CardsRated != 10 || result.Bands[0].StruggleRate != 0.6 {
		t.Fatalf("stop result %+v band0=%+v", result, result.Bands[0])
	}
	stored, err := store.GetPlacementRun(ctx, runID)
	if err != nil || stored.StopReason != stopBand {
		t.Fatalf("stored %+v %v", stored, err)
	}

	again, _ := openSvc(t, fixedClock{when})
	againUser := insertLearner(t, again.store, "below@example.com", "sub-below")
	againRun := start(t, again, againUser)
	ratePrefix(t, again, againRun, 10, func(i int) int {
		if i < 5 {
			return Again
		}
		return Good
	})
	card, done, err := again.Next(ctx, againRun)
	if err != nil || done || card == nil {
		t.Fatalf("after 5/10: card=%v done=%v err=%v", card, done, err)
	}
	if mustFind(t, again, card.ID).Band != 0 {
		t.Fatalf("11th card band %d", mustFind(t, again, card.ID).Band)
	}

	// Ten Hards stay under the Again rate, so the band continues.
	hardSvc, _ := openSvc(t, fixedClock{when})
	hardUser := insertLearner(t, hardSvc.store, "hard@example.com", "sub-hard")
	hardRun := start(t, hardSvc, hardUser)
	ratePrefix(t, hardSvc, hardRun, 10, func(int) int { return Hard })
	card, done, err = hardSvc.Next(ctx, hardRun)
	if err != nil || done || card == nil || mustFind(t, hardSvc, card.ID).Band != 0 {
		t.Fatalf("after 10 hard: card=%v done=%v err=%v", card, done, err)
	}
}

func TestCardCap(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = many(25)
	ctx := context.Background()
	userID := insertLearner(t, store, "cap@example.com", "sub-cap")
	runID := start(t, svc, userID)
	var dealt int
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		dealt++
		if dealt > cardCap {
			t.Fatalf("dealt %d", dealt)
		}
		if err := svc.Rate(ctx, runID, card.ID, Good); err != nil {
			t.Fatal(err)
		}
	}
	if dealt != cardCap {
		t.Fatalf("dealt %d, want %d", dealt, cardCap)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != stopCap || result.Level != 3 || result.CardsRated != cardCap {
		t.Fatalf("cap result stop=%s level=%d rated=%d", result.StopReason, result.Level, result.CardsRated)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil || n != cardCap {
		t.Fatalf("cards = %d %v", n, err)
	}
}

func TestResumeAndIdempotentFinish(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = manyBand(0, 6)
	ctx := context.Background()
	userID := insertLearner(t, store, "resume@example.com", "sub-resume")
	runID := start(t, svc, userID)
	first := rateNext(t, svc, runID, Good)
	second := rateNext(t, svc, runID, Easy)
	if again := start(t, svc, userID); again != runID {
		t.Fatalf("resume run %d, want %d", again, runID)
	}
	third, done, err := svc.Next(ctx, runID)
	if err != nil || done || third == nil {
		t.Fatal(err)
	}
	if third.ID == first.ID || third.ID == second.ID {
		t.Fatalf("resume replayed %s", third.ID)
	}
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	order := deck(run.Seed, svc.fixture)
	if third.ID != order[2].ID {
		t.Fatalf("third card %s, want %s", third.ID, order[2].ID)
	}

	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != "" || result.Level != -1 || result.CardsRated != 2 || result.SuggestedStartPosition != 0 {
		t.Fatalf("partial %+v", result)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Level != result.Level || again.StopReason != result.StopReason || again.CardsRated != 2 || again.EstimatedKnownWords != result.EstimatedKnownWords {
		t.Fatalf("second finish %+v, first %+v", again, result)
	}
	if got, err := store.CountCards(ctx, userID); err != nil || got != n {
		t.Fatalf("cards after second finish = %d, want %d (%v)", got, n, err)
	}
	if _, _, err := svc.Next(ctx, runID); err == nil {
		t.Fatal("next on a finished run succeeded")
	}

	if _, err := testharness.InsertUnit(ctx, store, "covered", "draft"); err != nil {
		t.Fatal(err)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || !place.Finished || place.StopReason != "" || place.SuggestedStartPosition != 2 || place.CardsRated != 2 {
		t.Fatalf("latest %+v ok=%v err=%v", place, ok, err)
	}
}

func TestRatingIsTheFirstReview(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	words := []fixtureItem{
		real("g", "טוֹב", 0),
		real("e", "קַל", 0),
		real("h", "קָשֶׁה", 0),
		real("a", "שׁוּב", 0),
	}
	svc.fixture = words
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, verified) VALUES ('טוֹב', 'old', 'noun', 1)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := insertLearner(t, store, "grade@example.com", "sub-grade")
	runID := start(t, svc, userID)
	want := []int{Good, Easy, Hard, Again}
	got := map[string]int{}
	var n int
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		rating := want[n]
		n++
		if err := svc.Rate(ctx, runID, card.ID, rating); err != nil {
			t.Fatal(err)
		}
		got[card.TextNFC] = rating
	}
	if n != 4 {
		t.Fatalf("dealt %d", n)
	}
	param := gofsrs.DefaultParam()
	param.EnableFuzz = false
	sched := gofsrs.NewFSRS(param)
	var known, learning []string
	for text, rating := range got {
		id, gloss, _, verified, ok, err := store.LexemeByText(ctx, text)
		if err != nil || !ok {
			t.Fatalf("lexeme %s ok=%v err=%v", text, ok, err)
		}
		if text == "טוֹב" && (!verified || gloss != "gloss") {
			t.Fatalf("verified lexeme gloss=%q verified=%v", gloss, verified)
		}
		cardID, has, err := store.CardID(ctx, userID, id, fsrs.DirHeEn)
		if err != nil || !has {
			t.Fatalf("card %s has=%v err=%v", text, has, err)
		}
		card, err := store.GetCard(ctx, cardID)
		if err != nil {
			t.Fatal(err)
		}
		first := sched.Next(gofsrs.NewCard(), when, gofsrs.Rating(rating))
		if card.Stability != first.Card.Stability {
			t.Fatalf("%s stability %v, want %v", text, card.Stability, first.Card.Stability)
		}
		logs, err := store.ReviewLogs(ctx, cardID)
		if err != nil {
			t.Fatal(err)
		}
		if len(logs) != 1 || logs[0].Rating != rating {
			t.Fatalf("%s logs %+v", text, logs)
		}
		if rating == Good || rating == Easy {
			known = append(known, text)
		} else {
			learning = append(learning, text)
		}
	}
	slices.Sort(known)
	slices.Sort(learning)
	gotKnown, err := store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	gotLearning, err := store.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotKnown, known) || !slices.Equal(gotLearning, learning) {
		t.Fatalf("known %v want %v; learning %v want %v", gotKnown, known, gotLearning, learning)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != stopExhausted || result.Level != 0 {
		t.Fatalf("finish %+v", result)
	}

	// A retake skips every word that already has a card.
	second := start(t, svc, userID)
	if card, done, err := svc.Next(ctx, second); err != nil || !done || card != nil {
		t.Fatalf("retake card=%v done=%v err=%v", card, done, err)
	}
	for text := range got {
		id, _, _, _, ok, err := store.LexemeByText(ctx, text)
		if err != nil || !ok {
			t.Fatal(err)
		}
		cardID, has, err := store.CardID(ctx, userID, id, fsrs.DirHeEn)
		if err != nil || !has {
			t.Fatal(err)
		}
		logs, err := store.ReviewLogs(ctx, cardID)
		if err != nil || len(logs) != 1 {
			t.Fatalf("%s logs %+v %v", text, logs, err)
		}
	}
	if n, err := store.CountCards(ctx, userID); err != nil || n != 4 {
		t.Fatalf("cards after retake = %d %v", n, err)
	}
}

func TestSkipExistingCard(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = []fixtureItem{real("a", "אֶחָד", 0), real("b", "בַּיִת", 0), real("c", "גֶּשֶׁם", 0)}
	ctx := context.Background()
	userID := insertLearner(t, store, "skip@example.com", "sub-skip")
	lexemeID, err := testharness.InsertLexeme(ctx, store, "אֶחָד")
	if err != nil {
		t.Fatal(err)
	}
	cardID, err := store.InsertCard(ctx, db.Card{
		UserID:   userID,
		LexemeID: lexemeID,
		Dir:      fsrs.DirHeEn,
		State:    fsrs.StateNew,
		Due:      when,
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := start(t, svc, userID)
	var seen []string
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		seen = append(seen, card.TextNFC)
		if err := svc.Rate(ctx, runID, card.ID, Good); err != nil {
			t.Fatal(err)
		}
	}
	if slices.Contains(seen, "אֶחָד") || len(seen) != 2 {
		t.Fatalf("dealt %v", seen)
	}
	logs, err := store.ReviewLogs(ctx, cardID)
	if err != nil || len(logs) != 0 {
		t.Fatalf("existing card logs %+v %v", logs, err)
	}
	if n, err := store.CountCards(ctx, userID); err != nil || n != 3 {
		t.Fatalf("cards = %d %v", n, err)
	}
}

func TestAudioURL(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	const text = "שָׁלוֹם"
	clip := stringsRepeat("ab", 32)
	ctx := context.Background()
	if _, err := store.ExecContext(ctx, `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'model', 1, ?)`,
		clip, text, "2026-01-01T00:00:00.000000000Z"); err != nil {
		t.Fatal(err)
	}
	svc.fixture = []fixtureItem{real("a", text, 0), real("b", "קָפֶה", 0)}
	userID := insertLearner(t, store, "audio@example.com", "sub-audio")
	runID := start(t, svc, userID)
	card, done, err := svc.Next(ctx, runID)
	if err != nil || done {
		t.Fatal(err)
	}
	if err := svc.Rate(ctx, runID, "nope", Good); err == nil {
		t.Fatal("wrong card was rated")
	}
	if err := svc.Rate(ctx, runID, card.ID, 0); err == nil {
		t.Fatal("rating 0 was accepted")
	}
	want := ""
	if card.TextNFC == text {
		want = "/audio/" + clip + ".mp3"
	}
	if card.AudioURL != want || card.GlossEN == "" || card.Translit == "" {
		t.Fatalf("card %+v", card)
	}
	if err := svc.Rate(ctx, runID, card.ID, Good); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rate(ctx, runID, card.ID, Good); err == nil {
		t.Fatal("second rating was accepted")
	}
}

func ratePrefix(t *testing.T, svc *Service, runID int64, n int, rating func(int) int) {
	t.Helper()
	for i := 0; i < n; i++ {
		rateNext(t, svc, runID, rating(i))
	}
}

func rateNext(t *testing.T, svc *Service, runID int64, rating int) Card {
	t.Helper()
	card, done, err := svc.Next(context.Background(), runID)
	if err != nil || done || card == nil {
		t.Fatalf("next: card=%v done=%v err=%v", card, done, err)
	}
	if err := svc.Rate(context.Background(), runID, card.ID, rating); err != nil {
		t.Fatal(err)
	}
	return *card
}

func mustFind(t *testing.T, svc *Service, id string) fixtureItem {
	t.Helper()
	it, err := svc.find(id)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func mustCatalog(t *testing.T) []fixtureItem {
	t.Helper()
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func real(id, text string, band int) fixtureItem {
	return fixtureItem{ID: id, TextNFC: text, Band: band, GlossEN: "gloss", Translit: id, Gender: "m", POS: "noun", Source: "tutor"}
}

func many(n int) []fixtureItem {
	var out []fixtureItem
	for band := 0; band < bands; band++ {
		out = append(out, manyBand(band, n)...)
	}
	return out
}

func manyBand(band, n int) []fixtureItem {
	out := make([]fixtureItem, n)
	for i := range out {
		id := fmt.Sprintf("b%d-%02d", band, i)
		out[i] = real(id, id, band)
	}
	return out
}

func openSvc(t *testing.T, clk clock.Clock) (*Service, *db.Store) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, fsrs.New(store, clk), clk), store
}

func insertLearner(t *testing.T, store *db.Store, email, sub string) int64 {
	t.Helper()
	id, err := store.InsertUser(context.Background(), email, sub, "learner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func start(t *testing.T, svc *Service, userID int64) int64 {
	t.Helper()
	id, err := svc.Start(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("run id is 0")
	}
	return id
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
