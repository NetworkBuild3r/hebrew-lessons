package brief

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-030/SPEC-003 acceptance criteria.

func TestPaceForBandsAndConsolidate(t *testing.T) {
	tierCount := 8
	cases := []struct {
		name       string
		accuracy   float64
		reviews    int
		learning   int
		wantN      int
		wantCons   bool
		wantReason string
	}{
		{
			// SPEC AC names 0.72 for the low band; under 0.75 consolidates first, so the
			// band is checked at 0.78 (still below 0.80, at or above the consolidate line).
			name:     "accuracy 0.78 gives 2 new words",
			accuracy: 0.78, reviews: 50, learning: 10,
			wantN: PaceNewWordsLow, wantCons: false,
			wantReason: "Recent accuracy is under 80%, so this lesson takes 2 new words.",
		},
		{
			name:     "accuracy 0.72 consolidates",
			accuracy: 0.72, reviews: 50, learning: 10,
			wantN: 0, wantCons: true,
			wantReason: "Recent accuracy is under 75%, so this lesson consolidates with no new words.",
		},
		{
			name:     "45 learning words consolidates",
			accuracy: 0.90, reviews: 50, learning: 45,
			wantN: 0, wantCons: true,
			wantReason: "More than 40 words are still learning, so this lesson consolidates with no new words.",
		},
		{
			name:     "accuracy under 0.75 consolidates",
			accuracy: 0.74, reviews: 50, learning: 10,
			wantN: 0, wantCons: true,
			wantReason: "Recent accuracy is under 75%, so this lesson consolidates with no new words.",
		},
		{
			name:     "mid band takes 5",
			accuracy: 0.85, reviews: 50, learning: 10,
			wantN: PaceNewWordsMid, wantCons: false,
			wantReason: "Recent accuracy is between 80% and 90%, so this lesson takes 5 new words.",
		},
		{
			name:     "high band uses tier count",
			accuracy: 0.91, reviews: 50, learning: 10,
			wantN: tierCount, wantCons: false,
			wantReason: "Recent accuracy is above 90%, so this lesson takes the tier's new-word count.",
		},
		{
			name:     "no reviews keeps tier count",
			accuracy: 0, reviews: 0, learning: 10,
			wantN: tierCount, wantCons: false,
			wantReason: "No recent reviews yet, so this lesson uses the usual new-word count.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PaceFor(tc.accuracy, tc.reviews, tc.learning, tierCount)
			if got.NewWordCount != tc.wantN || got.Consolidate != tc.wantCons || got.Reason != tc.wantReason {
				t.Fatalf("PaceFor = %+v, want n=%d cons=%v reason=%q", got, tc.wantN, tc.wantCons, tc.wantReason)
			}
			if got.Accuracy != tc.accuracy || got.LearningWordCount != tc.learning {
				t.Fatalf("inputs not stored: %+v", got)
			}
		})
	}
}

func TestMinStoryTokensByTier(t *testing.T) {
	cases := []struct {
		tier Tier
		want int
	}{
		{TierFor(0), MinStoryTokensBeginner},
		{TierFor(BeginnerKnownBelow), MinStoryTokensDeveloping},
		{TierFor(DevelopingKnownBelow), MinStoryTokensEstablished},
	}
	for _, tc := range cases {
		if got := MinStoryTokensFor(tc.tier); got != tc.want {
			t.Errorf("%s: MinStoryTokensFor = %d, want %d", tc.tier.Name, got, tc.want)
		}
	}
	// Cap by MaxTextTokens when set and below the floor.
	tiny := Tier{Name: TierDeveloping, MaxTextTokens: 10}
	if got := MinStoryTokensFor(tiny); got != 10 {
		t.Fatalf("capped = %d, want 10", got)
	}
}

func TestNextPatternFollowsCatalogAndPrereqs(t *testing.T) {
	// AC: With patterns A, then B (requires A), then C, where A is taught, nextPattern is B.
	// With A and B taught, it is C.
	cases := []struct {
		name   string
		taught []string
		want   string
	}{
		{name: "A taught yields B", taught: []string{"a-start"}, want: "b-next"},
		{name: "A and B taught yields C", taught: []string{"a-start", "b-next"}, want: "c-last"},
		{name: "none taught yields A", taught: nil, want: "a-start"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			ctx := context.Background()
			if err := e.store.UpsertPatterns(ctx, []db.PatternInput{
				{Slug: "a-start", Title: "A", Summary: "first", Prereqs: nil},
				{Slug: "b-next", Title: "B", Summary: "second", Prereqs: []string{"a-start"}},
				{Slug: "c-last", Title: "C", Summary: "third", Prereqs: []string{"b-next"}},
			}); err != nil {
				t.Fatal(err)
			}
			for i, slug := range tc.taught {
				addPatternUnit(t, e, i+1, slug, "story "+slug, nil)
			}
			br := e.build("")
			if br.NextPattern != tc.want {
				t.Fatalf("nextPattern = %q, want %q", br.NextPattern, tc.want)
			}
		})
	}
}

func TestReusePatternsAreLastTwoTaught(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.UpsertPatterns(ctx, []db.PatternInput{
		{Slug: "pat-a", Title: "Pattern A", Summary: "a"},
		{Slug: "pat-b", Title: "Pattern B", Summary: "b"},
		{Slug: "pat-c", Title: "Pattern C", Summary: "c"},
	}); err != nil {
		t.Fatal(err)
	}
	addPatternUnit(t, e, 1, "pat-a", "first story", []string{"אֶחָד"})
	addPatternUnit(t, e, 2, "pat-b", "second story", []string{"שְׁנַיִם"})
	addPatternUnit(t, e, 3, "pat-c", "third story", []string{"שָׁלוֹשׁ"})
	br := e.build("")
	if len(br.ReusePatterns) != 2 {
		t.Fatalf("reusePatterns = %#v, want 2", br.ReusePatterns)
	}
	if br.ReusePatterns[0].Slug != "pat-c" || br.ReusePatterns[0].Title != "Pattern C" {
		t.Fatalf("first reuse = %#v", br.ReusePatterns[0])
	}
	if br.ReusePatterns[1].Slug != "pat-b" || br.ReusePatterns[1].Title != "Pattern B" {
		t.Fatalf("second reuse = %#v", br.ReusePatterns[1])
	}
}

func TestPreviousUnitsCarryStoriesAndDialogue(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.UpsertPatterns(ctx, []db.PatternInput{
		{Slug: "pat-a", Title: "A", Summary: "a"},
		{Slug: "pat-b", Title: "B", Summary: "b"},
		{Slug: "pat-c", Title: "C", Summary: "c"},
	}); err != nil {
		t.Fatal(err)
	}
	addPatternUnit(t, e, 1, "pat-a", "old story", []string{"יָשָׁן"})
	addFullUnit(t, e, 2, "pat-b", "Middle title", "at the cafe",
		[]string{"קָפֶה", "לֶחֶם"},
		[]units.Activity{
			{Type: units.ActivityStory, Text: "הִיא שׁוֹתָה קָפֶה"},
			{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
				{Speaker: "a", Text: "מַה נִּשְׁמָע"},
				{Speaker: "b", Text: "הַכֹּל טוֹב"},
			}},
		}, nil)
	addFullUnit(t, e, 3, "pat-c", "Latest title", "at home",
		[]string{"בַּיִת"},
		[]units.Activity{{Type: units.ActivityStory, Text: "אֲנִי בַּבַּיִת"}}, nil)

	br := e.build("")
	if len(br.PreviousUnits) != 2 {
		t.Fatalf("previousUnits = %d, want 2", len(br.PreviousUnits))
	}
	prev := br.PreviousUnits
	if prev[0].Title != "Middle title" || prev[0].Situation != "at the cafe" || prev[0].Pattern != "pat-b" {
		t.Fatalf("first previous = %#v", prev[0])
	}
	if got := prev[0].NewWords; len(got) != 2 || got[0] != "קָפֶה" {
		t.Fatalf("first newWords = %#v", got)
	}
	if got := prev[0].Texts; len(got) != 3 || got[0] != "הִיא שׁוֹתָה קָפֶה" || got[1] != "מַה נִּשְׁמָע" || got[2] != "הַכֹּל טוֹב" {
		t.Fatalf("first texts = %#v", got)
	}
	if prev[1].Title != "Latest title" || prev[1].Pattern != "pat-c" || prev[1].Texts[0] != "אֲנִי בַּבַּיִת" {
		t.Fatalf("second previous = %#v", prev[1])
	}
}

func TestPaceFromReviewsAndConsolidateInBrief(t *testing.T) {
	cases := []struct {
		name     string
		good     int
		again    int
		learning int
		wantN    int
		wantCons bool
	}{
		// 39/50 = 0.78: below 0.80 and not consolidating → 2 new words (SPEC AC band).
		{name: "0.78 accuracy band", good: 39, again: 11, learning: 0, wantN: 2, wantCons: false},
		// 36/50 = 0.72: under 0.75 → consolidate (SPEC description).
		{name: "0.72 accuracy consolidates", good: 36, again: 14, learning: 0, wantN: 0, wantCons: true},
		{name: "45 learning words", good: 45, again: 5, learning: 45, wantN: 0, wantCons: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			for range 8 {
				e.word("pool", 0)
			}
			seedReviews(t, e, tc.good, tc.again)
			for i := range tc.learning {
				id := e.word(fmt.Sprintf("learning %d", i), 0)
				e.attempts(e.user, id, fsrs.SkillRecall, "RR", time.Hour)
				e.card(e.user, id, fsrs.SkillRecall, 2, 1, 3, 3)
			}
			br := e.build("")
			if br.Pace.NewWordCount != tc.wantN || br.Pace.Consolidate != tc.wantCons {
				t.Fatalf("pace = %+v, want n=%d cons=%v", br.Pace, tc.wantN, tc.wantCons)
			}
			if tc.wantCons {
				if len(br.NewWords) != 0 || br.NextPattern != "" {
					t.Fatalf("consolidate left newWords=%d nextPattern=%q", len(br.NewWords), br.NextPattern)
				}
			} else if len(br.NewWords) != tc.wantN {
				t.Fatalf("newWords = %d, want %d", len(br.NewWords), tc.wantN)
			}
			if br.Targets.MinStoryTokens != MinStoryTokensBeginner {
				t.Fatalf("minStoryTokens = %d", br.Targets.MinStoryTokens)
			}
			if br.V != Version {
				t.Fatalf("v = %d, want %d", br.V, Version)
			}
		})
	}
}

func TestFixMistakesSkipAndRetire(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.UpsertPatterns(ctx, []db.PatternInput{
		{Slug: "pat-a", Title: "A", Summary: "a"},
		{Slug: "pat-b", Title: "B", Summary: "b"},
	}); err != nil {
		t.Fatal(err)
	}
	// Three pairs that appear in both previous units' final checkpoints (to skip),
	// plus fresher pairs that should fill the brief.
	skip := [][2]string{
		{"wrong-a", "right-a"},
		{"wrong-b", "right-b"},
		{"wrong-c", "right-c"},
	}
	keep := [][2]string{
		{"wrong-d", "right-d"},
		{"wrong-e", "right-e"},
		{"wrong-f", "right-f"},
	}
	res := e.exec(`INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`, "נָכוֹן", "retired right")
	retiredRight, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	retired := [2]string{"wrong-retire", "נָכוֹן"}
	// Record skip pairs many times so they would otherwise win.
	for _, pair := range skip {
		for range 5 {
			if err := e.store.RecordMistakes(ctx, e.user, []db.MistakeInput{
				{Wrong: pair[0], Right: pair[1], Note: "n", Source: "app"},
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, pair := range keep {
		for range 2 {
			if err := e.store.RecordMistakes(ctx, e.user, []db.MistakeInput{
				{Wrong: pair[0], Right: pair[1], Note: "n", Source: "app"},
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := e.store.RecordMistakes(ctx, e.user, []db.MistakeInput{
		{Wrong: retired[0], Right: retired[1], Note: "n", Source: "app", LexemeID: retiredRight},
	}); err != nil {
		t.Fatal(err)
	}
	// Three right answers at or after the mistake date retire that pair.
	for i := range 3 {
		at := testNow.Add(time.Duration(i+1) * time.Minute)
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			e.user, retiredRight, fsrs.SkillRecall, db.FormatTime(at))
	}

	checkActs := make([]units.Activity, 0, 3)
	for _, pair := range skip {
		checkActs = append(checkActs, units.Activity{Type: units.ActivityRecall, Text: pair[1]})
	}
	addFullUnit(t, e, 1, "pat-a", "U1", "s1", nil,
		[]units.Activity{{Type: units.ActivityStory, Text: "א"}}, checkActs)
	addFullUnit(t, e, 2, "pat-b", "U2", "s2", nil,
		[]units.Activity{{Type: units.ActivityStory, Text: "ב"}}, checkActs)

	br := e.build("")
	if len(br.FixMistakes) != 3 {
		t.Fatalf("fixMistakes = %#v, want 3", br.FixMistakes)
	}
	for _, m := range br.FixMistakes {
		for _, pair := range skip {
			if m.Right == pair[1] {
				t.Fatalf("skipped pair still present: %#v", m)
			}
		}
		if m.Right == retired[1] {
			t.Fatalf("retired pair still present: %#v", m)
		}
	}
	got := map[string]bool{}
	for _, m := range br.FixMistakes {
		got[m.Wrong] = true
	}
	for _, pair := range keep {
		if !got[pair[0]] {
			t.Fatalf("missing kept pair %s in %#v", pair[0], br.FixMistakes)
		}
	}
}

func TestRevisionBriefIgnoresUnitsAtOrAfterPosition(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.UpsertPatterns(ctx, []db.PatternInput{
		{Slug: "pat-a", Title: "A", Summary: "a"},
		{Slug: "pat-b", Title: "B", Summary: "b"},
		{Slug: "pat-c", Title: "C", Summary: "c"},
		{Slug: "pat-d", Title: "D", Summary: "d"},
	}); err != nil {
		t.Fatal(err)
	}
	addPatternUnit(t, e, 8, "pat-a", "eight", []string{"שְׁמוֹנֶה"})
	addPatternUnit(t, e, 9, "pat-b", "nine", []string{"תֵּשַׁע"})
	addPatternUnit(t, e, 10, "pat-c", "ten", []string{"עֶשֶׂר"})
	addPatternUnit(t, e, 11, "pat-d", "eleven", []string{"אַחַת עֶשְׂרֵה"})

	br, err := e.b.BuildAt(ctx, e.user, "", 10, 99) // INIT-030/SPEC-005: reviseUnitID on the plan
	if err != nil {
		t.Fatal(err)
	}
	if br.ReviseUnitID == nil || *br.ReviseUnitID != 99 {
		t.Fatalf("ReviseUnitID = %v, want 99", br.ReviseUnitID)
	}
	if len(br.PreviousUnits) != 2 {
		t.Fatalf("previousUnits = %#v", br.PreviousUnits)
	}
	if br.PreviousUnits[0].Pattern != "pat-a" || br.PreviousUnits[1].Pattern != "pat-b" {
		t.Fatalf("previous patterns = %#v", br.PreviousUnits)
	}
	for _, u := range br.PreviousUnits {
		if u.Pattern == "pat-c" || u.Pattern == "pat-d" {
			t.Fatalf("revision saw unit at or after 10: %#v", u)
		}
	}
	if len(br.ReusePatterns) != 2 || br.ReusePatterns[0].Slug != "pat-b" || br.ReusePatterns[1].Slug != "pat-a" {
		t.Fatalf("reusePatterns = %#v", br.ReusePatterns)
	}
	if br.NextPattern != "pat-c" {
		t.Fatalf("nextPattern = %q, want pat-c (A and B taught before 10)", br.NextPattern)
	}
}

func addPatternUnit(t *testing.T, e *env, position int, pattern, story string, newWords []string) {
	t.Helper()
	addFullUnit(t, e, position, pattern, "t-"+pattern, "s-"+pattern, newWords,
		[]units.Activity{{Type: units.ActivityStory, Text: story}}, nil)
}

func addFullUnit(t *testing.T, e *env, position int, pattern, title, situation string, newWords []string, lessonActs, checkActs []units.Activity) {
	t.Helper()
	doc := units.Unit{
		Title: title, Situation: situation, Concept: "c", NewWords: newWords, Position: position,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: lessonActs}},
	}
	if len(checkActs) > 0 {
		doc.Lessons = append(doc.Lessons, units.Lesson{Kind: units.KindCheckpoint, Activities: checkActs})
	}
	scope := fmt.Sprintf("learner:%d", e.user)
	res, err := e.store.PutUnit(context.Background(), nil, doc, scope, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetUnitMeta(context.Background(), res.UnitID, &pattern, nil); err != nil {
		t.Fatal(err)
	}
}

func seedReviews(t *testing.T, e *env, good, again int) {
	t.Helper()
	id := e.word("reviewed", 0)
	res := e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, 10, 5, ?, 0, ?, 0)`,
		e.user, id, fsrs.SkillRecall, fsrs.StateReview, db.FormatTime(testNow), good+again, db.FormatTime(testNow))
	cardID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for range good {
		n++
		at := testNow.Add(-time.Duration(n) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, ?, ?, 0, 1, 0)`, cardID, int(fsrs.Good), db.FormatTime(at))
	}
	for range again {
		n++
		at := testNow.Add(-time.Duration(n) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, ?, ?, 0, 1, 0)`, cardID, int(fsrs.Again), db.FormatTime(at))
	}
}
