package web

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-037/SPEC-004: graded ask buttons come only from taught words.

const (
	tbLehachin = "לְהָכִין"
	tbOseh     = "עוֹשֶׂה"
	tbSteak    = "סְטֵייק"
	tbAni      = "אֲנִי"
	tbGar      = "גָּר"
	tbRoeh     = "רוֹאֶה"
	tbOkhel    = "אוֹכֵל"
	tbShote    = "שׁוֹתֶה"
	tbUntaught = "כּוֹתֵב"
	tbPo       = "פֹּה"
	tbKen      = "כֵּן"
	tbLo       = "לֹא"
)

func TestSteakCheckpointOmitsUntaughtButtons(t *testing.T) {
	fx := openLessonPlayer(t)
	ids := upsertTaughtLexemes(t, fx, []lexicon.Input{
		verbIn(tbLehachin, "to prepare", "lehakhin"),
		verbIn(tbOseh, "does", "oseh"),
		nounIn(tbSteak, "steak", "steik"),
		pronIn(tbAni, "I", "ani"),
		verbIn(tbGar, "lives", "gar"),
		verbIn(tbRoeh, "sees", "roeh"),
		verbIn(tbOkhel, "eats", "okhel"),
		verbIn(tbShote, "drinks", "shote"),
	})
	// Rebuilt copy of unit 12's steak checkpoint: contrast lines put לְהָכִין
	// and עוֹשֶׂה in the unit word list, but they are not newWords.
	unitID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Steak checkpoint", Position: 0, NewWords: []string{tbGar, tbRoeh, tbOkhel, tbShote},
		Lessons: []units.Lesson{{
			Kind: units.KindCheckpoint,
			Activities: []units.Activity{
				{Type: units.ActivityPattern, Text: tbLehachin + " " + tbSteak + " — לַעֲשׂוֹת " + tbSteak},
				{Type: units.ActivityPattern, Text: tbAni + " " + tbOseh + " — " + tbAni + " לַעֲשׂוֹת"},
				// Bank line so the newWords enter lesson_items before the cloze.
				{Type: units.ActivityStory, Text: tbGar + " " + tbRoeh + " " + tbOkhel + " " + tbShote},
				{Type: units.ActivityCloze, Text: tbAni + " " + tbGar, Translation: "I live", Target: tbGar},
				{Type: units.ActivityRecall, Text: tbGar},
			},
		}},
	}, "shared")
	publish(t, fx.store, unitID)

	path := "/lesson/" + itoa(unitID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	for i := 0; i < 3; i++ {
		runID := mustRunID(t, body)
		ans := postLesson(t, fx.h, fx.cookie, path, itoa(runID), "continue", true, lessonCSRF)
		if ans.Code != http.StatusOK {
			t.Fatalf("continue %d: %d %s", i, ans.Code, ans.Body.String())
		}
		body = ans.Body.String()
	}
	if buttons := choiceValuesAllowEmpty(t, body); len(buttons) != 0 {
		t.Fatalf("cloze is a reveal, got buttons %v", buttons)
	}
	if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, fx.userID); n != 0 {
		t.Fatalf("cloze recorded %d attempts", n)
	}
	recall := postLesson(t, fx.h, fx.cookie, path, itoa(mustRunID(t, body)), "continue", true, lessonCSRF)
	if recall.Code != http.StatusOK {
		t.Fatalf("continue cloze: %d", recall.Code)
	}
	buttons := choiceValues(t, recall.Body.String())
	for _, id := range buttons {
		if id == itoa(ids[tbLehachin]) || id == itoa(ids[tbOseh]) {
			t.Fatalf("untaught button %s among %v", id, buttons)
		}
	}
	if !slices.Contains(buttons, itoa(ids[tbGar])) {
		t.Fatalf("target missing from buttons %v", buttons)
	}
}

func TestUnfinishedEarlierUnitNeverSuppliesButton(t *testing.T) {
	fx := openLessonPlayer(t)
	ids := upsertTaughtLexemes(t, fx, []lexicon.Input{
		verbIn(tbUntaught, "writes", "kotev"),
		verbIn(tbGar, "lives", "gar"),
		verbIn(tbRoeh, "sees", "roeh"),
		verbIn(tbOkhel, "eats", "okhel"),
		verbIn(tbShote, "drinks", "shote"),
	})
	gateID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Gate", Position: 0, NewWords: []string{tbGar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: tbGar},
		}}},
	}, "shared")
	unfinishedID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Earlier unfinished", Position: 1, NewWords: []string{tbUntaught},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: tbUntaught},
		}}},
	}, "shared")
	currentID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Current", Position: 2, NewWords: []string{tbGar, tbRoeh, tbOkhel, tbShote},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: tbGar + " " + tbRoeh + " " + tbOkhel + " " + tbShote},
			{Type: units.ActivityRecall, Text: tbGar},
		}}},
	}, "shared")
	publish(t, fx.store, gateID, unfinishedID, currentID)
	when := db.FormatTime(fx.now)
	// Finish the gate so Today can advance past it, then hide the unfinished
	// earlier unit so NextCourseLesson reaches current. The unfinished unit
	// stays published and earlier, so its words still reach EarlierUnitLexemes.
	if _, err := fx.store.ExecContext(fx.ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, fx.userID, gateID, when, when); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.store.ExecContext(fx.ctx, `
		INSERT INTO unit_reports (unit_id, user_id, reason, note, created_at)
		VALUES (?, ?, 'other', '', ?)`, unfinishedID, fx.userID, when); err != nil {
		t.Fatal(err)
	}

	// Pool-level pin: unfinished earlier word is not taught.
	_, earlier := loadTaughtPools(t, fx.store, fx.ctx, fx.userID, currentID, 2)
	for _, lex := range earlier {
		if lex.ID == ids[tbUntaught] {
			t.Fatalf("untaught unfinished earlier word in earlier pool: %+v", lex)
		}
	}
	raw, err := fx.store.EarlierUnitLexemes(fx.ctx, fx.userID, 2)
	if err != nil {
		t.Fatal(err)
	}
	foundRaw := false
	for _, lex := range raw {
		if lex.ID == ids[tbUntaught] {
			foundRaw = true
		}
	}
	if !foundRaw {
		t.Fatal("fixture: unfinished earlier word should still be in the unfiltered earlier pool")
	}

	path := "/lesson/" + itoa(currentID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	// Skip the story bank.
	ans := postLesson(t, fx.h, fx.cookie, path, itoa(mustRunID(t, body)), "continue", true, lessonCSRF)
	got := choiceValues(t, ans.Body.String())
	if slices.Contains(got, itoa(ids[tbUntaught])) {
		t.Fatalf("unfinished earlier word is a button: %v", got)
	}
}

func TestTaughtButtonsFiftySeedsAndRenderPostAgree(t *testing.T) {
	const seeds = 50
	emails := make([]string, seeds)
	for i := range emails {
		emails[i] = fmt.Sprintf("taught%d@example.com", i)
	}
	fx := openTaughtMultiPlayer(t, emails)
	ctx := fx.ctx
	ids := upsertTaughtLexemes(t, fx, []lexicon.Input{
		verbIn(tbGar, "lives", "gar"),
		verbIn(tbRoeh, "sees", "roeh"),
		verbIn(tbOkhel, "eats", "okhel"),
		verbIn(tbShote, "drinks", "shote"),
		pronIn(tbAni, "I", "ani"),
		advIn(tbPo, "here", "po"),
		particleIn(tbKen, "yes", "ken"),
		particleIn(tbLo, "no", "lo"),
		verbIn(tbUntaught, "writes", "kotev"),
	})
	finishedID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Finished earlier", Position: 0,
		NewWords: []string{tbAni, tbPo, tbKen, tbLo},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: tbAni + " " + tbPo + " " + tbKen + " " + tbLo},
		}}},
	}, "shared")
	unfinishedID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Unfinished earlier", Position: 1, NewWords: []string{tbUntaught},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: tbUntaught},
		}}},
	}, "shared")
	gradedActs := []units.Activity{
		{Type: units.ActivityRecall, Text: tbGar},
		{Type: units.ActivityCloze, Text: tbGar, Translation: "lives", Target: tbGar},
		{Type: units.ActivityListenPick, Text: tbRoeh, Translation: "sees"},
		// Write is typed now, so it has no buttons to count. lesson_typed_test.go covers it.
	}
	// One published unit per graded type so each ask is activity 0 (no walk).
	unitIDs := make([]int64, len(gradedActs))
	for i, act := range gradedActs {
		unitIDs[i] = putLesson(t, fx.store, nil, units.Unit{
			Title:    "Fixture " + act.Type,
			Position: 2 + i,
			NewWords: []string{tbGar, tbRoeh, tbOkhel, tbShote},
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityStory, Text: tbGar + " " + tbRoeh + " " + tbOkhel + " " + tbShote},
				act,
			}}},
		}, "shared")
	}
	publish(t, fx.store, append([]int64{finishedID, unfinishedID}, unitIDs...)...)
	when := db.FormatTime(fx.now)

	taught := map[int64]bool{
		ids[tbGar]: true, ids[tbRoeh]: true, ids[tbOkhel]: true, ids[tbShote]: true,
		ids[tbAni]: true, ids[tbPo]: true, ids[tbKen]: true, ids[tbLo]: true,
	}
	pl := player{store: fx.store}

	cookies := make([]*http.Cookie, seeds)
	userIDs := make([]int64, seeds)
	for i, email := range emails {
		user, err := fx.store.UserByEmail(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fx.store.ExecContext(ctx, `
			INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
			VALUES (?, ?, 0, ?, ?, 1)`, user.ID, finishedID, when, when); err != nil {
			t.Fatal(err)
		}
		if _, err := fx.store.ExecContext(ctx, `
			INSERT INTO unit_reports (unit_id, user_id, reason, note, created_at)
			VALUES (?, ?, 'other', '', ?)`, unfinishedID, user.ID, when); err != nil {
			t.Fatal(err)
		}
		cookies[i] = mintSession(t, fx.store, user.ID, lessonCSRF, fx.now.Add(time.Hour))
		userIDs[i] = user.ID
	}

	for step, act := range gradedActs {
		unitID := unitIDs[step]
		path := "/lesson/" + itoa(unitID) + "/0"
		// Hide sibling fixture units so NextCourseLesson opens this one, and
		// clear any prior hide on the unit under test so CourseLesson works.
		for _, userID := range userIDs {
			if _, err := fx.store.ExecContext(ctx, `DELETE FROM unit_reports WHERE unit_id = ? AND user_id = ?`, unitID, userID); err != nil {
				t.Fatal(err)
			}
			for j, other := range unitIDs {
				if j == step {
					continue
				}
				if _, err := fx.store.ExecContext(ctx, `
					INSERT INTO unit_reports (unit_id, user_id, reason, note, created_at)
					SELECT ?, ?, 'other', '', ?
					WHERE NOT EXISTS (
						SELECT 1 FROM unit_reports WHERE unit_id = ? AND user_id = ?
					)`, other, userID, when, other, userID); err != nil {
					t.Fatal(err)
				}
			}
		}
		lesson, err := fx.store.CourseLesson(ctx, fx.userID, unitID, 0)
		if err != nil {
			t.Fatal(err)
		}
		for seed := 0; seed < seeds; seed++ {
			cookie := cookies[seed]
			userID := userIDs[seed]
			body := getAuthed(t, fx.h, cookie, path)
			body = advanceTaughtStep(t, fx, cookie, path, body, units.Activity{Type: units.ActivityStory}, ids)
			runID := mustRunID(t, body)
			got := choiceValuesAllowEmpty(t, body)
			pool, earlier := loadTaughtPools(t, fx.store, ctx, userID, unitID, 2+step)
			targetText := act.Text
			if act.Target != "" {
				targetText = act.Target
			}
			target := viewByText(t, append(append([]db.LexemeView{}, pool...), earlier...), targetText)
			seedVal := choices.ChoiceSeed{RunID: runID, Step: 1, Attempt: 0, TargetID: target.ID}
			lex, err := fx.store.LexemesByText(ctx, actTokens(act))
			if err != nil {
				t.Fatal(err)
			}
			block := lineMateExclude(act, lex, target)
			wantN := len(askChoices(act.Type, pool, earlier, target, seedVal, block))
			if act.Type == units.ActivityListenPick {
				if lines := listenLines(act, pool, earlier, target, seedVal, block); lines != nil {
					wantN = len(lines)
				}
			}
			if len(got) != wantN {
				t.Fatalf("%s seed %d: render %d buttons, pool builder %d: %v", act.Type, seed, len(got), wantN, got)
			}
			for _, idStr := range got {
				id, err := strconv.ParseInt(idStr, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if id == ids[tbUntaught] {
					t.Fatalf("%s seed %d: unfinished earlier word on a button", act.Type, seed)
				}
				if id != target.ID && !taught[id] {
					t.Fatalf("%s seed %d: untaught button %s in %v", act.Type, seed, idStr, got)
				}
			}
			run, err := fx.store.PlayerRunFor(ctx, userID, runID)
			if err != nil {
				t.Fatal(err)
			}
			n, err := pl.askChoiceCount(ctx, userID, lesson, run, act, target)
			if err != nil {
				t.Fatal(err)
			}
			if n != len(got) {
				t.Fatalf("%s seed %d: post count %d != render %d", act.Type, seed, n, len(got))
			}
		}
	}
}

func openTaughtMultiPlayer(t *testing.T, emails []string) lessonFixture {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var first int64
	for i, email := range emails {
		user, err := store.CreateUser(ctx, email, "sub-"+email, auth.RoleLearner)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = user.ID
		}
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: emails,
	})
	if err != nil {
		t.Fatal(err)
	}
	return lessonFixture{
		h:      NewOptions(nil, svc, Options{Store: store}),
		store:  store,
		cookie: mintSession(t, store, first, lessonCSRF, now.Add(time.Hour)),
		ctx:    ctx,
		userID: first,
		now:    now,
	}
}

func TestNewWordCanBeButtonLaterInSameUnit(t *testing.T) {
	fx := openLessonPlayer(t)
	ids := upsertTaughtLexemes(t, fx, []lexicon.Input{
		verbIn(tbGar, "lives", "gar"),
		verbIn(tbRoeh, "sees", "roeh"),
		verbIn(tbOkhel, "eats", "okhel"),
		verbIn(tbShote, "drinks", "shote"),
	})
	unitID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Same unit new words", Position: 0,
		NewWords: []string{tbGar, tbRoeh, tbOkhel, tbShote},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: tbGar + " " + tbRoeh + " " + tbOkhel + " " + tbShote},
			{Type: units.ActivityRecall, Text: tbGar},
			{Type: units.ActivityRecall, Text: tbRoeh},
		}}},
	}, "shared")
	publish(t, fx.store, unitID)
	path := "/lesson/" + itoa(unitID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	body = advanceTaughtStep(t, fx, fx.cookie, path, body, units.Activity{Type: units.ActivityStory, Text: tbGar}, ids)
	body = advanceTaughtStep(t, fx, fx.cookie, path, body, units.Activity{Type: units.ActivityRecall, Text: tbGar}, ids)
	second := choiceValues(t, body)
	if !slices.Contains(second, itoa(ids[tbGar])) {
		t.Fatalf("new word from earlier step missing as button: %v", second)
	}
}

func TestFewerThanThreeTaughtRevealsAndPostGradesNothing(t *testing.T) {
	fx := openLessonPlayer(t)
	ids := upsertTaughtLexemes(t, fx, []lexicon.Input{
		verbIn(tbGar, "lives", "gar"),
		verbIn(tbRoeh, "sees", "roeh"),
	})
	unitID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Thin taught", Position: 0, NewWords: []string{tbGar, tbRoeh},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityCloze, Text: tbGar + " " + tbRoeh, Translation: "lives here", Target: tbGar},
		}}},
	}, "shared")
	publish(t, fx.store, unitID)
	path := "/lesson/" + itoa(unitID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	if vals := choiceValuesAllowEmpty(t, body); len(vals) != 0 {
		t.Fatalf("expected reveal, got buttons %v", vals)
	}
	if !strings.Contains(body, `value="continue"`) {
		t.Fatalf("reveal missing Continue:\n%s", body)
	}
	posted := postLesson(t, fx.h, fx.cookie, path, itoa(mustRunID(t, body)), itoa(ids[tbGar]), true, lessonCSRF)
	if strings.Contains(posted.Body.String(), "feedback--right") || strings.Contains(posted.Body.String(), "feedback--wrong") {
		t.Fatalf("post graded a reveal:\n%s", posted.Body.String())
	}
	if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, fx.userID); n != 0 {
		t.Fatalf("reveal recorded %d attempts", n)
	}
}

func advanceTaughtStep(t *testing.T, fx lessonFixture, cookie *http.Cookie, path, body string, act units.Activity, ids map[string]int64) string {
	t.Helper()
	runID := mustRunID(t, body)
	vals := choiceValuesAllowEmpty(t, body)
	answer := "continue"
	if len(vals) > 0 {
		target := act.Text
		if act.Target != "" {
			target = act.Target
		}
		want := itoa(ids[target])
		if slices.Contains(vals, want) {
			answer = want
		} else {
			answer = vals[0]
		}
	}
	ans := postLesson(t, fx.h, cookie, path, itoa(runID), answer, true, lessonCSRF)
	if ans.Code != http.StatusOK {
		t.Fatalf("advance %s: %d %s", act.Type, ans.Code, ans.Body.String())
	}
	body = ans.Body.String()
	if strings.Contains(body, "feedback--") {
		ans = postLesson(t, fx.h, cookie, path, itoa(mustRunID(t, body)), "continue", true, lessonCSRF)
		if ans.Code != http.StatusOK {
			t.Fatalf("continue after %s: %d", act.Type, ans.Code)
		}
		body = ans.Body.String()
	}
	return body
}

func choiceValuesAllowEmpty(t *testing.T, body string) []string {
	t.Helper()
	ms := choiceValueRE.FindAllStringSubmatch(body, -1)
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m[1]
	}
	return out
}

func upsertTaughtLexemes(t *testing.T, fx lessonFixture, inputs []lexicon.Input) map[string]int64 {
	t.Helper()
	saved, _, err := fx.store.UpsertLexemes(fx.ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	return ids
}

func verbIn(text, gloss, translit string) lexicon.Input {
	g, tr, pos := gloss, translit, "verb"
	return lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos}
}

func nounIn(text, gloss, translit string) lexicon.Input {
	g, tr, pos, gender := gloss, translit, "noun", "m"
	return lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos, Gender: &gender}
}

func pronIn(text, gloss, translit string) lexicon.Input {
	g, tr, pos := gloss, translit, "pron"
	return lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos}
}

func advIn(text, gloss, translit string) lexicon.Input {
	g, tr, pos := gloss, translit, "adv"
	return lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos}
}

func particleIn(text, gloss, translit string) lexicon.Input {
	g, tr, pos := gloss, translit, "particle"
	return lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos}
}
