package placement

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

var when = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type moveClock struct{ t time.Time }

func (c *moveClock) Now() time.Time { return c.t }

func TestBandStopAtExactlyHalf(t *testing.T) {
	if cardCap != 100 || timeCap != 5*time.Minute {
		t.Fatalf("caps cards=%d time=%s", cardCap, timeCap)
	}
	svc, store := openSvc(t, fixedClock{when})
	cat := mustCatalog(t)
	ctx := context.Background()
	userID := insertLearner(t, store, "half@example.com", "sub-half")
	runID := start(t, svc, userID)
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Seed != when.UnixNano() || !run.StartedAt.Equal(when) {
		t.Fatalf("run seed %d started %s", run.Seed, run.StartedAt)
	}
	deck := blocks(run.Seed, cat)
	if len(deck[0]) != 16 || len(deck[1]) != 16 {
		t.Fatalf("block lengths %d %d", len(deck[0]), len(deck[1]))
	}
	band0 := idSet(deck[0])
	band1 := idSet(deck[1])

	rateBand(t, svc, runID, band0, 6)
	card, done, err := svc.Next(ctx, runID)
	if err != nil || done || card == nil {
		t.Fatalf("next after half: card=%v done=%v err=%v", card, done, err)
	}
	if _, inBand := band1[card.ID]; !inBand {
		t.Fatalf("card %s is not in band 1", card.ID)
	}
	body, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "is_pseudo") || strings.Contains(string(body), "\"band\"") {
		t.Fatalf("card leaked: %s", body)
	}

	// A second sitting that misses the line stops after band 0.
	stopID := insertLearner(t, store, "stop@example.com", "sub-stop")
	stopRun := start(t, svc, stopID)
	stop, err := store.GetPlacementRun(ctx, stopRun)
	if err != nil {
		t.Fatal(err)
	}
	stopDeck := blocks(stop.Seed, cat)
	rateBand(t, svc, stopRun, idSet(stopDeck[0]), 5)
	if _, done, err := svc.Next(ctx, stopRun); err != nil || !done {
		t.Fatalf("next after a miss: done=%v err=%v", done, err)
	}
	result, err := svc.Finish(ctx, stopRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cap != capBand || result.Level != -1 || result.Bands[0].Good != 5 || result.Bands[0].Shown != 12 {
		t.Fatalf("stop result cap=%s level=%d band0=%+v", result.Cap, result.Level, result.Bands[0])
	}
	if _, _, err := svc.Next(ctx, stopRun); err == nil {
		t.Fatal("next on a finished run succeeded")
	}
	stored, err := store.GetPlacementRun(ctx, stopRun)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Cap != capBand {
		t.Fatalf("stored cap %q", stored.Cap)
	}
}

func TestFalseAlarmSeedsNothing(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = []fixtureItem{
		real("r1", "אֶחָד", 0),
		real("r2", "בַּיִת", 0),
		pseudo("p1", "פֶּסֶא"),
		pseudo("p2", "פֶּסֶב"),
		pseudo("p3", "פֶּסֶג"),
		pseudo("p4", "פֶּסֶד"),
	}
	ctx := context.Background()
	userID := insertLearner(t, store, "false@example.com", "sub-false")
	runID := start(t, svc, userID)
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		it := mustFind(t, svc, card.ID)
		rating := Again
		if it.Pseudo {
			rating = Good
		}
		rated, err := svc.Rate(ctx, runID, card.ID, rating)
		if err != nil {
			t.Fatal(err)
		}
		if rated.IsPseudo != it.Pseudo {
			t.Fatalf("is_pseudo=%v for %s", rated.IsPseudo, card.ID)
		}
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reliable || result.FalseAlarmRate != 1 || result.Cap != capBand {
		t.Fatalf("result reliable=%v f=%v cap=%s", result.Reliable, result.FalseAlarmRate, result.Cap)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("cards = %d", n)
	}
	for _, it := range svc.fixture {
		if _, _, _, _, ok, err := store.LexemeByText(ctx, it.TextNFC); err != nil || ok {
			t.Fatalf("lexeme %s ok=%v err=%v", it.TextNFC, ok, err)
		}
	}
}

func TestQuarterFalseAlarmStillSeeds(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = append(reals(0, "א", 2), pseudo("p1", "פֶּא"), pseudo("p2", "פֶּב"), pseudo("p3", "פֶּג"), pseudo("p4", "פֶּד"))
	ctx := context.Background()
	userID := insertLearner(t, store, "quarter@example.com", "sub-quarter")
	runID := start(t, svc, userID)
	var pseudoGood int
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		it := mustFind(t, svc, card.ID)
		rating := Good
		if it.Pseudo {
			rating = Again
			if pseudoGood == 0 {
				rating = Easy
				pseudoGood++
			}
		}
		if _, err := svc.Rate(ctx, runID, card.ID, rating); err != nil {
			t.Fatal(err)
		}
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reliable || result.FalseAlarmRate != 0.25 || result.Level != 0 {
		t.Fatalf("result %+v", result)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil || n != 2 {
		t.Fatalf("cards = %d err=%v", n, err)
	}
	for _, it := range svc.fixture {
		_, _, _, _, ok, err := store.LexemeByText(ctx, it.TextNFC)
		if err != nil {
			t.Fatal(err)
		}
		if ok == it.Pseudo {
			t.Fatalf("lexeme %s pseudo=%v present=%v", it.ID, it.Pseudo, ok)
		}
	}
}

func TestFirstRatingSeedsCardAndLog(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	words := []fixtureItem{
		real("g", "טוֹב", 0),
		real("e", "קַל", 0),
		real("h", "קָשֶׁה", 0),
		real("d", "לֹא", 0),
	}
	svc.fixture = words
	// A verified lexeme keeps verified and takes the fixture gloss.
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO lexemes (text_nfc, gloss_en, pos, verified) VALUES ('טוֹב', 'old', 'noun', 1)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := insertLearner(t, store, "grade@example.com", "sub-grade")
	runID := start(t, svc, userID)

	want := []int{Good, Easy, Hard, DontKnow}
	gotRating := map[string]int{}
	var dontID string
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
		if _, err := svc.Rate(ctx, runID, card.ID, rating); err != nil {
			t.Fatal(err)
		}
		gotRating[card.TextNFC] = rating
		if rating == DontKnow {
			dontID = card.ID
		}
	}
	if n != 4 {
		t.Fatalf("dealt %d", n)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reliable || result.Level != 0 || result.Cap != "" || result.SuggestedStartPosition != 0 {
		t.Fatalf("result %+v", result)
	}
	if result.Bands[0].Good != 1 || result.Bands[0].Easy != 1 || result.Bands[0].Hard != 1 || result.Bands[0].DontKnow != 1 {
		t.Fatalf("band %+v", result.Bands[0])
	}

	var known, learning []string
	for text, rating := range gotRating {
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
		if card.Dir != fsrs.DirHeEn {
			t.Fatalf("dir %s", card.Dir)
		}
		logs, err := store.ReviewLogs(ctx, cardID)
		if err != nil {
			t.Fatal(err)
		}
		wantLog := rating
		if rating == DontKnow {
			wantLog = Again
		}
		if len(logs) != 1 || logs[0].Rating != wantLog || logs[0].Peeked {
			t.Fatalf("%s logs %+v, want rating %d", text, logs, wantLog)
		}
		if rating == Good || rating == Easy {
			known = append(known, text)
		} else {
			learning = append(learning, text)
		}
		if rating == DontKnow {
			rows, err := store.ListPlacementAnswers(ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			var stored int64 = -1
			for _, row := range rows {
				if row.ItemID == dontID && row.Rating.Valid {
					stored = row.Rating.Int64
				}
			}
			if stored != DontKnow {
				t.Fatalf("stored don't-know rating %d", stored)
			}
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

	// A lesson word and an unseeded row stay out of both lists.
	lessonID, err := testharness.InsertLexeme(ctx, store, "שִׁעוּר")
	if err != nil {
		t.Fatal(err)
	}
	unitID, err := testharness.InsertUnit(ctx, store, "unit", "published")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, store, unitID, 0, lessonID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	finished := when
	if _, err := testharness.InsertLessonRun(ctx, store, userID, unitID, 0, when, &finished); err != nil {
		t.Fatal(err)
	}
	unseeded, err := testharness.InsertLexeme(ctx, store, "יָד")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLearnerLexeme(ctx, userID, unseeded, "known", false, "manual", when); err != nil {
		t.Fatal(err)
	}
	gotKnown, err = store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	gotLearning, err = store.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotKnown, known) || !slices.Equal(gotLearning, learning) {
		t.Fatalf("after extras known %v learning %v", gotKnown, gotLearning)
	}

	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	// The inserted unit has an empty newWords list, so it is covered.
	if place.Level != 0 || place.SuggestedStartPosition != 2 || !place.TakenAt.Equal(when) {
		t.Fatalf("latest %+v", place)
	}
}

func TestRetakeDoesNotDoubleGrade(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = []fixtureItem{real("a", "אֶחָד", 0), real("b", "בַּיִת", 0)}
	ctx := context.Background()
	userID := insertLearner(t, store, "retake@example.com", "sub-retake")
	first := start(t, svc, userID)
	rateAll(t, svc, first, Good)
	result, err := svc.Finish(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != 0 {
		t.Fatalf("first level %d", result.Level)
	}
	ids := cardIDs(t, store, userID, svc.fixture)
	for _, id := range ids {
		logs, err := store.ReviewLogs(ctx, id)
		if err != nil || len(logs) != 1 || logs[0].Rating != Good {
			t.Fatalf("first logs %+v %v", logs, err)
		}
	}

	second := start(t, svc, userID)
	rateAll(t, svc, second, Again)
	again, err := svc.Finish(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if again.Level != -1 {
		t.Fatalf("retake level %d", again.Level)
	}
	for _, id := range ids {
		logs, err := store.ReviewLogs(ctx, id)
		if err != nil || len(logs) != 1 || logs[0].Rating != Good {
			t.Fatalf("retake logs %+v %v", logs, err)
		}
	}
	known, err := store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	learning, err := store.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(learning) != 0 || len(known) != 2 {
		t.Fatalf("known %v learning %v", known, learning)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || place.Level != -1 || !place.Reliable {
		t.Fatalf("latest %+v ok=%v err=%v", place, ok, err)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil || n != 2 {
		t.Fatalf("cards %d %v", n, err)
	}
}

func TestCardAndTimeCaps(t *testing.T) {
	if cardCap != 100 || timeCap != 5*time.Minute {
		t.Fatalf("caps cards=%d time=%s", cardCap, timeCap)
	}
	svc, store := openSvc(t, fixedClock{when})
	var fixture []fixtureItem
	for band := 0; band < bands; band++ {
		fixture = append(fixture, reals(band, fmt.Sprintf("ב%d", band), realsPerBlock)...)
	}
	for i := 0; i < pseudosPerBlock*bands; i++ {
		fixture = append(fixture, pseudo(fmt.Sprintf("p%d", i), fmt.Sprintf("פ%d", i)))
	}
	svc.fixture = fixture
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
		it := mustFind(t, svc, card.ID)
		rating := Again
		if !it.Pseudo {
			rating = Good
		}
		if _, err := svc.Rate(ctx, runID, card.ID, rating); err != nil {
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
	if result.Cap != capCards || !result.Reliable {
		t.Fatalf("cap result %+v", result)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if n != realsPerBlock*bands {
		t.Fatalf("cards = %d", n)
	}

	clk := &moveClock{t: when}
	timed, _ := openSvc(t, clk)
	timed.fixture = []fixtureItem{real("a", "א", 0), real("b", "ב", 0), real("c", "ג", 0)}
	timedUser := insertLearner(t, timed.store, "time@example.com", "sub-time")
	timedRun := start(t, timed, timedUser)
	first, done, err := timed.Next(ctx, timedRun)
	if err != nil || done || first == nil {
		t.Fatalf("first card done=%v err=%v", done, err)
	}
	clk.t = when.Add(timeCap - time.Nanosecond)
	again, done, err := timed.Next(ctx, timedRun)
	if err != nil || done || again == nil || again.ID != first.ID {
		t.Fatalf("outstanding card %+v done=%v err=%v", again, done, err)
	}
	if _, err := timed.Rate(ctx, timedRun, again.ID, Good); err != nil {
		t.Fatal(err)
	}
	next, done, err := timed.Next(ctx, timedRun)
	if err != nil || done || next == nil || next.ID == first.ID {
		t.Fatalf("card under the cap %+v done=%v err=%v", next, done, err)
	}
	if _, err := timed.Rate(ctx, timedRun, next.ID, Good); err != nil {
		t.Fatal(err)
	}
	clk.t = when.Add(timeCap)
	if card, done, err := timed.Next(ctx, timedRun); err != nil || !done || card != nil {
		t.Fatalf("at the cap card=%v done=%v err=%v", card, done, err)
	}
	timedResult, err := timed.Finish(ctx, timedRun)
	if err != nil {
		t.Fatal(err)
	}
	if timedResult.Cap != capTime || timedResult.Level != 0 {
		t.Fatalf("time result cap=%s level=%d", timedResult.Cap, timedResult.Level)
	}
}

func TestAudioURLAndOpenCard(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	const text = "שָׁלוֹם"
	oldID := strings.Repeat("a", 64)
	newID := strings.Repeat("b", 64)
	ctx := context.Background()
	if _, err := store.ExecContext(ctx, `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, ?, 'voice', 'model', 1, ?)`,
		oldID, text, "2020-01-01T00:00:00.000000000Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, ?, 'voice', 'model', 1, ?)`,
		newID, text, "2026-01-01T00:00:00.000000000Z"); err != nil {
		t.Fatal(err)
	}
	svc.fixture = []fixtureItem{real("a", text, 0), real("b", "קָפֶה", 0), pseudo("p", "פֶּסֶא")}
	userID := insertLearner(t, store, "audio@example.com", "sub-audio")
	runID := start(t, svc, userID)
	card, done, err := svc.Next(ctx, runID)
	if err != nil || done {
		t.Fatal(err)
	}
	if _, err := svc.Rate(ctx, runID, "nope", Good); err == nil {
		t.Fatal("wrong card was rated")
	}
	if _, err := svc.Rate(ctx, runID, card.ID, 9); err == nil {
		t.Fatal("rating 9 was accepted")
	}
	rated, err := svc.Rate(ctx, runID, card.ID, Good)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rate(ctx, runID, card.ID, Good); err == nil {
		t.Fatal("second rating was accepted")
	}
	it := mustFind(t, svc, card.ID)
	if rated.IsPseudo != it.Pseudo {
		t.Fatalf("pseudo flag %v", rated.IsPseudo)
	}
	wantURL := ""
	if card.TextNFC == text {
		wantURL = "/audio/" + newID + ".mp3"
	}
	if card.AudioURL != wantURL || card.GlossEN == "" {
		t.Fatalf("card %+v", card)
	}

	// Same seed, same first card.
	other := New(store, nil, fixedClock{when})
	other.fixture = svc.fixture
	otherUser := insertLearner(t, store, "same@example.com", "sub-same")
	otherRun := start(t, other, otherUser)
	again, _, err := other.Next(ctx, otherRun)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != card.ID {
		t.Fatalf("first card %s then %s", card.ID, again.ID)
	}

	// Finish before the block ends stores no cap. The known word moves the
	// suggestion once a covered unit exists.
	early, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if early.Cap != "" || early.SuggestedStartPosition != 0 {
		t.Fatalf("early %+v", early)
	}
	if _, err := testharness.InsertUnit(ctx, store, "covered", "draft"); err != nil {
		t.Fatal(err)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || place.SuggestedStartPosition != 2 {
		t.Fatalf("start %+v ok=%v err=%v", place, ok, err)
	}
}

func TestManualFinishHasNoCap(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = []fixtureItem{real("a", "א", 0), real("b", "ב", 0), real("c", "ג", 0), real("d", "ד", 0)}
	ctx := context.Background()
	userID := insertLearner(t, store, "manual@example.com", "sub-manual")
	runID := start(t, svc, userID)
	rateNext(t, svc, runID, Good)
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cap != "" || result.Level != 0 || result.Bands[0].Shown != 1 {
		t.Fatalf("manual %+v", result)
	}
}

func rateBand(t *testing.T, svc *Service, runID int64, ids map[string]struct{}, goodReals int) {
	t.Helper()
	ctx := context.Background()
	var goods int
	for range ids {
		card, done, err := svc.Next(ctx, runID)
		if err != nil || done || card == nil {
			t.Fatalf("next: card=%v done=%v err=%v", card, done, err)
		}
		if _, ok := ids[card.ID]; !ok {
			t.Fatalf("card %s is outside the band", card.ID)
		}
		it := mustFind(t, svc, card.ID)
		rating := Again
		if !it.Pseudo && goods < goodReals {
			rating = Good
			goods++
		}
		rated, err := svc.Rate(ctx, runID, card.ID, rating)
		if err != nil {
			t.Fatal(err)
		}
		if rated.IsPseudo != it.Pseudo {
			t.Fatalf("is_pseudo=%v item %s", rated.IsPseudo, it.ID)
		}
	}
	if goods != goodReals {
		t.Fatalf("good reals %d, want %d", goods, goodReals)
	}
}

func rateAll(t *testing.T, svc *Service, runID int64, rating int) {
	t.Helper()
	ctx := context.Background()
	for {
		card, done, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			return
		}
		if _, err := svc.Rate(ctx, runID, card.ID, rating); err != nil {
			t.Fatal(err)
		}
	}
}

func rateNext(t *testing.T, svc *Service, runID int64, rating int) Card {
	t.Helper()
	card, done, err := svc.Next(context.Background(), runID)
	if err != nil || done || card == nil {
		t.Fatalf("next: card=%v done=%v err=%v", card, done, err)
	}
	if _, err := svc.Rate(context.Background(), runID, card.ID, rating); err != nil {
		t.Fatal(err)
	}
	return *card
}

func cardIDs(t *testing.T, store *db.Store, userID int64, items []fixtureItem) []int64 {
	t.Helper()
	var ids []int64
	for _, it := range items {
		lexemeID, _, _, _, ok, err := store.LexemeByText(context.Background(), it.TextNFC)
		if err != nil || !ok {
			t.Fatalf("lexeme %s", it.TextNFC)
		}
		id, has, err := store.CardID(context.Background(), userID, lexemeID, fsrs.DirHeEn)
		if err != nil || !has {
			t.Fatalf("card %s", it.TextNFC)
		}
		ids = append(ids, id)
	}
	return ids
}

func idSet(items []fixtureItem) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, it := range items {
		out[it.ID] = struct{}{}
	}
	return out
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
	return fixtureItem{ID: id, TextNFC: text, Band: bandPtr(band), GlossEN: "gloss", POS: "noun", Source: "tutor"}
}

func reals(band int, prefix string, n int) []fixtureItem {
	out := make([]fixtureItem, n)
	for i := range out {
		out[i] = real(fmt.Sprintf("%s-%02d", prefix, i), fmt.Sprintf("%s-%02d", prefix, i), band)
	}
	return out
}

func pseudo(id, text string) fixtureItem {
	return fixtureItem{ID: id, TextNFC: text, Pseudo: true, GlossEN: "no", POS: "noun", Source: "pseudo"}
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
