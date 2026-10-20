package placement

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
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

type movingClock struct{ t *time.Time }

func (c movingClock) Now() time.Time { return *c.t }

func TestAgainStop(t *testing.T) {
	if BandMax != 20 || StopMinRated != 10 || StopAgainRate != 0.60 || SittingSize != 20 || CardCap != 100 {
		t.Fatalf("BandMax=%d minRated=%d again=%v sitting=%d cards=%d", BandMax, StopMinRated, StopAgainRate, SittingSize, CardCap)
	}
	svc, store := openSvc(t, fixedClock{when})
	ctx := context.Background()
	userID := insertLearner(t, store, "stop@example.com", "sub-stop")
	runID := start(t, svc, userID)
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Seed != when.UnixNano() || !run.StartedAt.Equal(when) || run.SittingNo != 1 {
		t.Fatalf("run seed %d started %s sitting %d", run.Seed, run.StartedAt, run.SittingNo)
	}
	order := deck(run.Seed, mustCatalog(t))
	if order[0].Band != 0 {
		t.Fatalf("first card band %d", order[0].Band)
	}

	// Nine Agains do not stop, and the tenth card is still dealt.
	nineSvc, _ := openSvc(t, fixedClock{when})
	nineUser := insertLearner(t, nineSvc.store, "nine@example.com", "sub-nine")
	nineRun := start(t, nineSvc, nineUser)
	ratePrefix(t, nineSvc, nineRun, 9, func(int) int { return Again })
	res, err := nineSvc.Next(ctx, nineRun)
	if err != nil || res.Done || res.Break || res.Card == nil || mustFind(t, nineSvc, res.Card.ID).Band != 0 {
		t.Fatalf("after 9 agains: %+v %v", res, err)
	}

	ratePrefix(t, svc, runID, StopMinRated, func(i int) int {
		if i < 6 {
			return Again
		}
		return Good
	})
	res, err = svc.Next(ctx, runID)
	if err != nil || !res.Done || res.Break || res.Card != nil {
		t.Fatalf("after 6/10: %+v %v", res, err)
	}
	stored, err := store.GetPlacementRun(ctx, runID)
	if err != nil || stored.FinishedAt.IsZero() || stored.StopReason != stopBand {
		t.Fatalf("run did not finish itself: %+v %v", stored, err)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	band := result.Bands[0]
	if result.StopReason != stopBand || result.Level != -1 || result.CardsRated != StopMinRated || band.Again != 6 || band.Good != 4 || !result.FinishedAt.Equal(stored.FinishedAt) {
		t.Fatalf("stop result %+v band0=%+v", result, band)
	}

	below, _ := openSvc(t, fixedClock{when})
	belowUser := insertLearner(t, below.store, "below@example.com", "sub-below")
	belowRun := start(t, below, belowUser)
	ratePrefix(t, below, belowRun, StopMinRated, func(i int) int {
		if i < 5 {
			return Again
		}
		return Good
	})
	res, err = below.Next(ctx, belowRun)
	if err != nil || res.Done || res.Break || res.Card == nil || mustFind(t, below, res.Card.ID).Band != 0 {
		t.Fatalf("after 5/10: %+v %v", res, err)
	}

	dontSvc, dontStore := openSvc(t, fixedClock{when})
	dontUser := insertLearner(t, dontStore, "dontstop@example.com", "sub-dontstop")
	dontRun := start(t, dontSvc, dontUser)
	ratePrefix(t, dontSvc, dontRun, StopMinRated, func(i int) int {
		if i < 6 {
			return DontKnow
		}
		return Good
	})
	res, err = dontSvc.Next(ctx, dontRun)
	if err != nil || !res.Done || res.Card != nil {
		t.Fatalf("after 6 dont-know: %+v %v", res, err)
	}
	dontResult, err := dontSvc.Finish(ctx, dontRun)
	if err != nil {
		t.Fatal(err)
	}
	if dontResult.StopReason != stopBand || dontResult.Bands[0].DontKnow != 6 || dontResult.Bands[0].Again != 0 {
		t.Fatalf("dont-know stop %+v", dontResult.Bands[0])
	}

	hardSvc, _ := openSvc(t, fixedClock{when})
	hardUser := insertLearner(t, hardSvc.store, "hard@example.com", "sub-hard")
	hardRun := start(t, hardSvc, hardUser)
	ratePrefix(t, hardSvc, hardRun, StopMinRated, func(int) int { return Hard })
	res, err = hardSvc.Next(ctx, hardRun)
	if err != nil || res.Done || res.Break || res.Card == nil || mustFind(t, hardSvc, res.Card.ID).Band != 0 {
		t.Fatalf("after 10 hard: %+v %v", res, err)
	}
}

func TestFullDeckExhaustsAtCardCap(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = many(25)
	ctx := context.Background()
	userID := insertLearner(t, store, "cap@example.com", "sub-cap")
	runID := start(t, svc, userID)
	var dealt int
	for {
		res, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if res.Break {
			if err := svc.ContinueSitting(ctx, userID); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if res.Done {
			break
		}
		if res.Card == nil {
			t.Fatal("next returned no card")
		}
		dealt++
		if dealt > CardCap {
			t.Fatalf("dealt %d", dealt)
		}
		if err := svc.Rate(ctx, runID, res.Card.ID, Good); err != nil {
			t.Fatal(err)
		}
	}
	if dealt != CardCap {
		t.Fatalf("dealt %d, want %d", dealt, CardCap)
	}
	result, err := svc.Finish(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != stopExhausted || result.Level != 4 || result.CardsRated != CardCap {
		t.Fatalf("full deck stop=%s level=%d rated=%d", result.StopReason, result.Level, result.CardsRated)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || place.Sittings != CardCap/SittingSize || place.StopReason != stopExhausted {
		t.Fatalf("latest %+v ok=%v err=%v", place, ok, err)
	}
	n, err := store.CountCards(ctx, userID)
	if err != nil || n != CardCap {
		t.Fatalf("cards = %d %v", n, err)
	}
	rows, err := store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	byBand := map[int]int{}
	for _, row := range rows {
		if !row.Rating.Valid {
			continue
		}
		it, err := svc.find(row.ItemID)
		if err != nil {
			t.Fatal(err)
		}
		byBand[it.Band]++
	}
	for band := 0; band < bands; band++ {
		if byBand[band] != BandMax {
			t.Fatalf("band %d rated %d, want %d", band, byBand[band], BandMax)
		}
	}
}

func TestBandMax(t *testing.T) {
	svc, _ := openSvc(t, fixedClock{when})
	svc.fixture = many(25)
	ctx := context.Background()
	userID := insertLearner(t, svc.store, "band@example.com", "sub-band")
	runID := start(t, svc, userID)
	ratePrefix(t, svc, runID, BandMax, func(int) int { return Good })
	res, err := svc.Next(ctx, runID)
	if err != nil || !res.Break || res.Done || res.Card != nil {
		t.Fatalf("after %d in the band: %+v %v", BandMax, res, err)
	}
	if err := svc.ContinueSitting(ctx, userID); err != nil {
		t.Fatal(err)
	}
	res, err = svc.Next(ctx, runID)
	if err != nil || res.Break || res.Done || res.Card == nil {
		t.Fatalf("card after the band cap: %+v %v", res, err)
	}
	if got := mustFind(t, svc, res.Card.ID).Band; got != 1 {
		t.Fatalf("card after band 0 is band %d", got)
	}
}

func TestLatestPlacementUnfinished(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = manyBand(0, 8)
	ctx := context.Background()
	userID := insertLearner(t, store, "open@example.com", "sub-open")
	runID := start(t, svc, userID)
	ratePrefix(t, svc, runID, 3, func(int) int { return Again })
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if place.Finished || place.StopReason != "" || place.CardsRated != 3 || place.Sittings != 1 || !place.TakenAt.Equal(when) || len(place.BandScores) != 0 {
		t.Fatalf("unfinished %+v", place)
	}
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil || !run.FinishedAt.IsZero() || run.StopReason != "" {
		t.Fatalf("run %+v %v", run, err)
	}
}

func TestSittingBreak(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = many(25)
	ctx := context.Background()
	userID := insertLearner(t, store, "sit@example.com", "sub-sit")
	runID := start(t, svc, userID)
	ratePrefix(t, svc, runID, SittingSize, func(int) int { return Good })
	res, err := svc.Next(ctx, runID)
	if err != nil || !res.Break || res.Done || res.Card != nil {
		t.Fatalf("break: %+v %v", res, err)
	}
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil || !run.FinishedAt.IsZero() || run.SittingNo != 1 || run.StopReason != "" {
		t.Fatalf("break finished the run: %+v %v", run, err)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || place.Finished || place.Sittings != 1 || place.StopReason != "" || place.CardsRated != SittingSize {
		t.Fatalf("latest %+v ok=%v err=%v", place, ok, err)
	}
	if err := svc.ContinueSitting(ctx, userID); err != nil {
		t.Fatal(err)
	}
	res, err = svc.Next(ctx, runID)
	if err != nil || res.Break || res.Done || res.Card == nil {
		t.Fatalf("after continue: %+v %v", res, err)
	}
	run, err = store.GetPlacementRun(ctx, runID)
	if err != nil || run.SittingNo != 2 {
		t.Fatalf("sitting after continue: %+v %v", run, err)
	}
	rows, err := store.ListPlacementAnswers(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	var first, second int
	for _, row := range rows {
		switch row.SittingNo {
		case 1:
			if !row.Rating.Valid {
				t.Fatalf("sitting 1 left a card open: %+v", row)
			}
			first++
		case 2:
			second++
			if row.ItemID != res.Card.ID || row.Rating.Valid {
				t.Fatalf("sitting 2 card %+v", row)
			}
		default:
			t.Fatalf("sitting %d", row.SittingNo)
		}
	}
	if first != SittingSize || second != 1 {
		t.Fatalf("sittings first=%d second=%d", first, second)
	}
	if err := svc.ContinueSitting(ctx, userID); err == nil {
		t.Fatal("continue on an open sitting succeeded")
	}
}

func TestResumeAcrossDays(t *testing.T) {
	now := when
	svc, store := openSvc(t, movingClock{&now})
	svc.fixture = many(25)
	ctx := context.Background()
	userID := insertLearner(t, store, "day@example.com", "sub-day")
	runID := start(t, svc, userID)
	ratePrefix(t, svc, runID, 5, func(int) int { return Good })

	now = now.Add(24 * time.Hour)
	if again := start(t, svc, userID); again != runID {
		t.Fatalf("next day run %d, want %d", again, runID)
	}
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil || run.SittingNo != 1 || !run.FinishedAt.IsZero() {
		t.Fatalf("partial next day: %+v %v", run, err)
	}
	res, err := svc.Next(ctx, runID)
	if err != nil || res.Break || res.Done || res.Card == nil {
		t.Fatalf("resume card: %+v %v", res, err)
	}

	ratePrefix(t, svc, runID, SittingSize-5, func(int) int { return Good })
	res, err = svc.Next(ctx, runID)
	if err != nil || !res.Break || res.Done {
		t.Fatalf("end of sitting: %+v %v", res, err)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok || place.Finished {
		t.Fatalf("done for today finished the run: %+v %v", place, err)
	}

	now = now.Add(24 * time.Hour)
	if again := start(t, svc, userID); again != runID {
		t.Fatalf("second day run %d, want %d", again, runID)
	}
	run, err = store.GetPlacementRun(ctx, runID)
	if err != nil || run.SittingNo != 2 || !run.FinishedAt.IsZero() {
		t.Fatalf("new sitting: %+v %v", run, err)
	}
	res, err = svc.Next(ctx, runID)
	if err != nil || res.Break || res.Done || res.Card == nil {
		t.Fatalf("second sitting card: %+v %v", res, err)
	}
	if again := start(t, svc, userID); again != runID {
		t.Fatalf("same day run %d, want %d", again, runID)
	}
	run, err = store.GetPlacementRun(ctx, runID)
	if err != nil || run.SittingNo != 2 {
		t.Fatalf("start opened another sitting: %+v %v", run, err)
	}
}

func TestConcurrentStartOneOpenRun(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	svc.fixture = manyBand(0, 1)
	userID := insertLearner(t, store, "race@example.com", "sub-race")
	const n = 16
	ids := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	ready := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-ready
			ids[i], errs[i] = svc.Start(context.Background(), userID)
		}(i)
	}
	close(ready)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("run %d id %d, want %d among %v", i, ids[i], ids[0], ids)
		}
	}
	if _, err := store.InsertPlacementRun(context.Background(), userID, when, 1); !errors.Is(err, db.ErrOpenPlacementRun) {
		t.Fatalf("second open run err %v", err)
	}
}

func TestDontKnowGradesAsAgain(t *testing.T) {
	svc, store := openSvc(t, fixedClock{when})
	const text = "לֹא"
	svc.fixture = []fixtureItem{real("dk", text, 0)}
	ctx := context.Background()
	userID := insertLearner(t, store, "dont@example.com", "sub-dont")
	runID := start(t, svc, userID)
	card := rateNext(t, svc, runID, DontKnow)

	id, _, _, _, ok, err := store.LexemeByText(ctx, text)
	if err != nil || !ok {
		t.Fatal(err)
	}
	cardID, has, err := store.CardID(ctx, userID, id, fsrs.SkillRead)
	if err != nil || !has {
		t.Fatal(err)
	}
	logs, err := store.ReviewLogs(ctx, cardID)
	if err != nil || len(logs) != 1 || logs[0].Rating != int(fsrs.Again) {
		t.Fatalf("logs %+v %v", logs, err)
	}
	stored, err := store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	param := gofsrs.DefaultParam()
	param.EnableFuzz = false
	first := gofsrs.NewFSRS(param).Next(gofsrs.NewCard(), when, gofsrs.Again)
	if stored.Stability != first.Card.Stability {
		t.Fatalf("stability %v, want %v", stored.Stability, first.Card.Stability)
	}
	rows, err := store.ListPlacementAnswers(ctx, runID)
	if err != nil || len(rows) != 1 || !rows[0].Rating.Valid || rows[0].Rating.Int64 != DontKnow || rows[0].ItemID != card.ID {
		t.Fatalf("answers %+v %v", rows, err)
	}
	learning, err := store.PlacementLearningLexemes(ctx, userID)
	if err != nil || len(learning) != 1 || learning[0] != text {
		t.Fatalf("learning %v %v", learning, err)
	}
	known, err := store.PlacementKnownLexemes(ctx, userID)
	if err != nil || len(known) != 0 {
		t.Fatalf("known %v %v", known, err)
	}
	ids, err := store.KnownLexemeIDs(ctx, userID)
	if err != nil || slices.Contains(ids, id) {
		t.Fatalf("known ids %v %v", ids, err)
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
	third, err := svc.Next(ctx, runID)
	if err != nil || third.Done || third.Break || third.Card == nil {
		t.Fatal(err)
	}
	if third.Card.ID == first.ID || third.Card.ID == second.ID {
		t.Fatalf("resume replayed %s", third.Card.ID)
	}
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	order := deck(run.Seed, svc.fixture)
	if third.Card.ID != order[2].ID {
		t.Fatalf("third card %s, want %s", third.Card.ID, order[2].ID)
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
	if _, err := svc.Next(ctx, runID); err == nil {
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
		res, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if res.Done {
			break
		}
		if res.Break || res.Card == nil {
			t.Fatalf("next %+v", res)
		}
		rating := want[n]
		n++
		if err := svc.Rate(ctx, runID, res.Card.ID, rating); err != nil {
			t.Fatal(err)
		}
		got[res.Card.TextNFC] = rating
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
		cardID, has, err := store.CardID(ctx, userID, id, fsrs.SkillRead)
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
	if res, err := svc.Next(ctx, second); err != nil || !res.Done || res.Break || res.Card != nil {
		t.Fatalf("retake %+v %v", res, err)
	}
	for text := range got {
		id, _, _, _, ok, err := store.LexemeByText(ctx, text)
		if err != nil || !ok {
			t.Fatal(err)
		}
		cardID, has, err := store.CardID(ctx, userID, id, fsrs.SkillRead)
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
		Skill:    fsrs.SkillRead,
		State:    fsrs.StateNew,
		Due:      when,
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := start(t, svc, userID)
	var seen []string
	for {
		res, err := svc.Next(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if res.Done {
			break
		}
		if res.Break || res.Card == nil {
			t.Fatalf("next %+v", res)
		}
		seen = append(seen, res.Card.TextNFC)
		if err := svc.Rate(ctx, runID, res.Card.ID, Good); err != nil {
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
	res, err := svc.Next(ctx, runID)
	if err != nil || res.Done || res.Break || res.Card == nil {
		t.Fatal(err)
	}
	card := res.Card
	if err := svc.Rate(ctx, runID, "nope", Good); err == nil {
		t.Fatal("wrong card was rated")
	}
	if err := svc.Rate(ctx, runID, card.ID, 5); err == nil {
		t.Fatal("rating 5 was accepted")
	}
	want := ""
	if card.TextNFC == text {
		want = "/audio/" + clip + ".mp3"
	}
	if card.AudioURL != want || card.GlossEN == "" || card.Translit == "" || card.Gender == "" {
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
	res, err := svc.Next(context.Background(), runID)
	if err != nil || res.Done || res.Break || res.Card == nil {
		t.Fatalf("next: %+v %v", res, err)
	}
	if err := svc.Rate(context.Background(), runID, res.Card.ID, rating); err != nil {
		t.Fatal(err)
	}
	return *res.Card
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
