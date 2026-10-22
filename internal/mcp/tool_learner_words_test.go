package mcp

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestMixTargetMatchesBuildSession(t *testing.T) {
	got := currentMixTarget()
	if got.MaxNewWordsPerUnit != units.MaxNewWordsPerUnit || got.MinNewWordsPerLesson != units.MinNewWordsPerLesson || got.MaxNewWordsPerLesson != units.MaxNewWordsPerLesson {
		t.Fatalf("unit budgets %+v", got)
	}
	if got.MaxNewWordsPerUnit != 8 || got.MinNewWordsPerLesson != 2 || got.MaxNewWordsPerLesson != 3 {
		t.Fatalf("unit budgets %+v", got)
	}
	if got.TargetSuccess != session.TargetSuccess {
		t.Fatalf("targetSuccess %v session %v", got.TargetSuccess, session.TargetSuccess)
	}
	if got.TargetSuccess != sessionlimits.TargetSuccess {
		t.Fatalf("targetSuccess %v shared %v", got.TargetSuccess, sessionlimits.TargetSuccess)
	}
	if got.TargetSuccess != 0.85 {
		t.Fatalf("targetSuccess %v", got.TargetSuccess)
	}
	if got.MaxNewPerSession != session.MaxNewPerSession {
		t.Fatalf("maxNewPerSession %d session %d", got.MaxNewPerSession, session.MaxNewPerSession)
	}
	if got.MaxNewPerSession != sessionlimits.MaxNewPerSession {
		t.Fatalf("maxNewPerSession %d shared %d", got.MaxNewPerSession, sessionlimits.MaxNewPerSession)
	}
	if got.MaxNewPerSession != 5 {
		t.Fatalf("maxNewPerSession %d", got.MaxNewPerSession)
	}
	if got.MaxNewShare != session.MaxNewShare {
		t.Fatalf("maxNewShare %v session %v", got.MaxNewShare, session.MaxNewShare)
	}
	if got.MaxNewShare != sessionlimits.MaxNewShare {
		t.Fatalf("maxNewShare %v shared %v", got.MaxNewShare, sessionlimits.MaxNewShare)
	}
	if got.MinNewPerSession != sessionlimits.MinNewPerSession {
		t.Fatalf("minNewPerSession %d shared %d", got.MinNewPerSession, sessionlimits.MinNewPerSession)
	}
	if got.MaxNewShare != 0.25 {
		t.Fatalf("maxNewShare %v", got.MaxNewShare)
	}
	// BuildSession passes accuracyOf to mix. With no reviews that is
	// NoHistoryAccuracy, which is TargetSuccess, and the in-band share is
	// DefaultKnownShare.
	if session.NoHistoryAccuracy != session.TargetSuccess || session.KnownShare(session.TargetSuccess) != session.DefaultKnownShare {
		t.Fatalf("BuildSession default %v share %v", session.NoHistoryAccuracy, session.KnownShare(session.TargetSuccess))
	}
}

func TestGetContextCoachFieldsKeepExistingShape(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	mem.SetPlacement(7, Placement{
		TakenAt:                when,
		Finished:               true,
		Sittings:               1,
		Level:                  2,
		CardsRated:             4,
		StopReason:             "exhausted",
		EstimatedKnownWords:    10,
		SuggestedStartPosition: 1,
	}, []string{hello}, []string{coffee})
	mem.SetStruggle(7, []StruggleLexeme{{
		TextNFC: hello, Gloss: "hello; peace", Reviews: 3, Lapses: 1, AgainRate: 0.25,
	}}, nil)
	done := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	mem.SetCompletedSharedUnits(7, []UnitCompletion{{UnitID: 4, Position: 1, CompletedAt: done}})
	mem.SetCompletedUnitLexemes(7, []string{hello})
	translit := "shalom"
	rating := ratingGood
	reviewed := when.Add(time.Hour)
	mem.AddCard(7, LearnerCard{
		Lexeme: hello, Translit: &translit, POS: "interjection", Gloss: "hello; peace",
		Skill: "read", State: "review", Stability: 4, Difficulty: 5,
		Due: reviewed, Lapses: 2, Reps: 6, LastRating: &rating, LastReview: &reviewed,
	})

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	sharedRaw := callRaw(t, session, "get_context", getContextInput{Position: 0})
	if !strings.Contains(sharedRaw, `"placement":{"taken":false}`) || !strings.Contains(sharedRaw, `"cards":[]`) || !strings.Contains(sharedRaw, `"cardsTruncated":false`) || !strings.Contains(sharedRaw, `"recentAccuracy":null`) {
		t.Fatalf("shared %s", sharedRaw)
	}
	if strings.Contains(sharedRaw, `"cardsHint"`) {
		t.Fatalf("shared hint %s", sharedRaw)
	}
	shared := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0})
	if shared.Budgets != currentBudgets() || shared.MixTarget != currentMixTarget() || shared.RecentAccuracy != nil || len(shared.Cards) != 0 || shared.CardsTruncated || len(shared.Outline) != 0 || len(shared.KnownLexemes) != 0 || shared.Placement.Taken {
		t.Fatalf("shared %+v", shared)
	}
	if shared.MixTarget.MaxNewWordsPerUnit != 8 || shared.MixTarget.MinNewWordsPerLesson != 2 || shared.MixTarget.MaxNewWordsPerLesson != 3 || shared.MixTarget.TargetSuccess != 0.85 || shared.MixTarget.MaxNewPerSession != 5 || shared.MixTarget.MinNewPerSession != 2 || shared.MixTarget.MaxNewShare != 0.25 {
		t.Fatalf("mix %+v", shared.MixTarget)
	}
	if shared.CardsDueNow != nil {
		t.Fatalf("shared cardsDueNow %+v", shared.CardsDueNow)
	}

	id := int64(7)
	out := call[getContextOutput](t, session, "get_context", getContextInput{Position: 3, LearnerID: &id})
	if !out.Placement.Taken || out.Placement.Level != 2 || out.Placement.SuggestedStartPosition != 1 || out.Placement.StopReason != "exhausted" {
		t.Fatalf("placement %+v", out.Placement)
	}
	if len(out.KnownLexemes) != 1 || out.KnownLexemes[0] != hello {
		t.Fatalf("known %v", out.KnownLexemes)
	}
	if out.Budgets != currentBudgets() || len(out.Struggles.Lexemes) != 1 || out.Struggles.Lexemes[0].Lapses != 1 || len(out.CompletedSharedUnits) != 1 || out.CompletedSharedUnits[0].UnitID != 4 {
		t.Fatalf("existing %+v", out)
	}
	if len(out.Cards) != 1 || out.CardsTruncated || out.CardsHint != "" || out.RecentAccuracy == nil {
		t.Fatalf("coach %+v", out)
	}
	card := out.Cards[0]
	if card.Lexeme != hello || card.Translit == nil || *card.Translit != "shalom" || card.POS != "interjection" || card.Gloss != "hello; peace" || card.Skill != "read" || card.State != "review" || card.Lapses != 2 || card.Reps != 6 || card.LastRating == nil || *card.LastRating != ratingGood || card.LastReview == nil || !card.LastReview.Equal(reviewed) {
		t.Fatalf("card %+v", card)
	}
	if out.RecentAccuracy.WindowDays != AccuracyWindowDays || out.RecentAccuracy.LastReviewLimit != AccuracyReviewCap || out.RecentAccuracy.Reviews7d.Rate != nil || out.RecentAccuracy.Lessons7d.MeanScore != nil {
		t.Fatalf("empty accuracy %+v", out.RecentAccuracy)
	}
}

func TestGetContextCardCapAndAccuracy(t *testing.T) {
	mem := NewMemory()
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < contextCardCap+1; i++ {
		mem.AddCard(4, LearnerCard{
			Lexeme: cardName(i), POS: "noun", Gloss: "g", Skill: "read", State: "review",
			Due: base.Add(time.Duration(i) * time.Second),
		})
	}
	now := time.Now().UTC()
	mem.AddReview(4, ratingEasy, now.Add(-8*24*time.Hour))
	mem.AddReview(4, 1, now.Add(-48*time.Hour))
	mem.AddReview(4, ratingGood, now.Add(-24*time.Hour))
	mem.AddReview(4, ratingEasy, now.Add(time.Hour))
	scoreHalf := 0.5
	scoreOne := 1.0
	finished := []time.Time{now.Add(-time.Hour), now.Add(-2 * time.Hour), now.Add(-3 * time.Hour), now.Add(-8 * 24 * time.Hour)}
	mem.AddLessonRun(4, &finished[0], &scoreHalf)
	mem.AddLessonRun(4, &finished[1], nil)
	mem.AddLessonRun(4, &finished[2], &scoreOne)
	mem.AddLessonRun(4, &finished[3], &scoreOne)
	mem.AddLessonRun(4, nil, &scoreOne)
	mem.AddReview(9, ratingGood, now.Add(-time.Hour))

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	id := int64(4)
	out := call[getContextOutput](t, session, "get_context", getContextInput{LearnerID: &id})
	if !out.CardsTruncated || !strings.Contains(out.CardsHint, "get_learner_words") || len(out.Cards) != contextCardCap {
		t.Fatalf("cap %d truncated %v hint %q", len(out.Cards), out.CardsTruncated, out.CardsHint)
	}
	if out.Cards[0].Lexeme != cardName(0) || out.Cards[len(out.Cards)-1].Lexeme != cardName(contextCardCap-1) {
		t.Fatalf("ends %s %s", out.Cards[0].Lexeme, out.Cards[len(out.Cards)-1].Lexeme)
	}
	for _, card := range out.Cards {
		if card.Lexeme == cardName(contextCardCap) || card.Translit != nil || card.LastRating != nil || card.LastReview != nil {
			t.Fatalf("unexpected %+v", card)
		}
	}
	acc := out.RecentAccuracy
	if acc == nil || acc.Reviews7d.Count != 2 || acc.Reviews7d.GoodOrEasy != 1 || acc.Reviews7d.Rate == nil || !near(*acc.Reviews7d.Rate, 0.5) {
		t.Fatalf("7d %+v", acc)
	}
	if acc.LastReviews.Count != 3 || acc.LastReviews.GoodOrEasy != 2 || acc.LastReviews.Rate == nil || !near(*acc.LastReviews.Rate, 2.0/3.0) {
		t.Fatalf("last %+v", acc.LastReviews)
	}
	if acc.Lessons7d.Finished != 3 || acc.Lessons7d.Scored != 2 || acc.Lessons7d.MeanScore == nil || !near(*acc.Lessons7d.MeanScore, 0.75) {
		t.Fatalf("lessons %+v", acc.Lessons7d)
	}

	first := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, Limit: contextCardCap})
	if first.NextCursor == nil || len(first.Cards) != contextCardCap || first.Cards[0].Lexeme != out.Cards[0].Lexeme {
		t.Fatalf("page %+v", first.NextCursor)
	}
	second := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, Cursor: *first.NextCursor, Limit: contextCardCap})
	if second.NextCursor != nil || len(second.Cards) != 1 || second.Cards[0].Lexeme != cardName(contextCardCap) {
		t.Fatalf("tail %+v", second)
	}
}

func TestLearnerWordsPagesFiltersAndOrder(t *testing.T) {
	mem := NewMemory()
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	const n = 120
	for i := 0; i < n; i++ {
		state := "review"
		if i%2 == 0 {
			state = "learning"
		}
		mem.AddCard(3, LearnerCard{
			Lexeme: cardName(i), Gloss: "g", Skill: "recall", State: state,
			Due: base.Add(time.Duration(i) * time.Second),
		})
	}
	newer := base.Add(time.Hour)
	older := base.Add(30 * time.Minute)
	same := base
	mem.AddCard(3, LearnerCard{Lexeme: "tie-new", Gloss: "new", Skill: "read", State: "new", Due: same, LastReview: &newer})
	mem.AddCard(3, LearnerCard{Lexeme: "tie-old", Gloss: "old", Skill: "read", State: "new", Due: same, LastReview: &older})
	mem.AddCard(3, LearnerCard{Lexeme: "tie-none", Gloss: "none", Skill: "read", State: "new", Due: same})
	mem.AddCard(8, LearnerCard{Lexeme: "other", Gloss: "other", Skill: "read", State: "review", Due: base})

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	id := int64(3)
	full := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, Limit: learnerWordsMaxLimit})
	if full.NextCursor != nil || len(full.Cards) != n+3 {
		t.Fatalf("full %d cursor %v", len(full.Cards), full.NextCursor)
	}
	if full.Cards[0].Lexeme != "tie-new" || full.Cards[1].Lexeme != "tie-old" || full.Cards[2].Lexeme != cardName(0) || full.Cards[3].Lexeme != "tie-none" {
		t.Fatalf("order %s %s %s %s", full.Cards[0].Lexeme, full.Cards[1].Lexeme, full.Cards[2].Lexeme, full.Cards[3].Lexeme)
	}

	var paged []string
	var cursor string
	seen := map[string]bool{}
	for {
		in := learnerWordsInput{LearnerID: &id, Limit: learnerWordsDefaultLimit}
		if cursor != "" {
			in.Cursor = cursor
		}
		page := call[learnerWordsOutput](t, session, "get_learner_words", in)
		if len(page.Cards) == 0 {
			t.Fatal("empty page")
		}
		for _, card := range page.Cards {
			if seen[card.Lexeme] {
				t.Fatalf("duplicate %s", card.Lexeme)
			}
			seen[card.Lexeme] = true
			paged = append(paged, card.Lexeme)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(paged) != len(full.Cards) {
		t.Fatalf("paged %d full %d", len(paged), len(full.Cards))
	}
	for i := range full.Cards {
		if paged[i] != full.Cards[i].Lexeme {
			t.Fatalf("gap at %d %s want %s", i, paged[i], full.Cards[i].Lexeme)
		}
	}

	omitted := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id})
	if omitted.NextCursor == nil || len(omitted.Cards) != learnerWordsDefaultLimit {
		t.Fatalf("default page %d", len(omitted.Cards))
	}
	learning := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, State: "learning", Limit: learnerWordsMaxLimit})
	if learning.NextCursor != nil || len(learning.Cards) != n/2 {
		t.Fatalf("learning %d", len(learning.Cards))
	}
	for _, card := range learning.Cards {
		if card.State != "learning" {
			t.Fatalf("state %+v", card)
		}
	}
	before := base.Add(10 * time.Second)
	due := call[learnerWordsOutput](t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, DueBefore: &before, Limit: learnerWordsMaxLimit})
	// tie cards are due at base, plus cardName 0..9 (10 cards). 10 seconds is exclusive of card 10.
	if len(due.Cards) != 13 {
		t.Fatalf("dueBefore %d", len(due.Cards))
	}
	for _, card := range due.Cards {
		if !card.Due.Before(before) {
			t.Fatalf("not before %+v", card)
		}
	}
	if msg := callErr(t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, State: "sm2"}); !strings.Contains(msg, "state must be") {
		t.Fatalf("state %q", msg)
	}
	if msg := callErr(t, session, "get_learner_words", learnerWordsInput{LearnerID: &id, Cursor: "@@@"}); !strings.Contains(msg, "page cursor") {
		t.Fatalf("cursor %q", msg)
	}
	if msg := callErr(t, session, "get_learner_words", learnerWordsInput{}); !strings.Contains(msg, "learnerId is required") {
		t.Fatalf("admin omit %q", msg)
	}
}

func TestLearnerGetContextOmittedIDSeesOwnCards(t *testing.T) {
	mem, srv := roleServer(t)
	mem.AddCard(7, LearnerCard{Lexeme: hello, Gloss: "own-card", Skill: "read", State: "review", Due: time.Now().UTC()})
	mem.AddCard(9, LearnerCard{Lexeme: coffee, Gloss: "secret-card", Skill: "read", State: "review", Due: time.Now().UTC()})
	res := postTool(t, srv.URL, "learner", "get_context", getContextInput{Position: 0}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, "own-card") || strings.Contains(body, "secret-card") {
		t.Fatalf("omitted learnerId %d %s", res.StatusCode, body)
	}
}

func TestLearnerWordsRole(t *testing.T) {
	mem, srv := roleServer(t)
	mem.AddCard(9, LearnerCard{Lexeme: coffee, Gloss: "secret-latte", Skill: "read", State: "review", Due: time.Now().UTC()})
	mem.AddCard(7, LearnerCard{Lexeme: hello, Gloss: "own-word", Skill: "read", State: "learning", Due: time.Now().UTC()})

	other := int64(9)
	denied := postTool(t, srv.URL, "learner", "get_learner_words", learnerWordsInput{LearnerID: &other}, nil)
	body := assertDenied(t, denied, msgOtherLearner)
	if strings.Contains(body, "secret-latte") {
		t.Fatalf("leaked %s", body)
	}

	self := postTool(t, srv.URL, "learner", "get_learner_words", learnerWordsInput{}, nil)
	selfBody := readBody(t, self)
	if self.StatusCode != http.StatusOK || strings.Contains(selfBody, `"isError":true`) || !strings.Contains(selfBody, "own-word") || strings.Contains(selfBody, "secret-latte") {
		t.Fatalf("self %d %s", self.StatusCode, selfBody)
	}
	own := int64(7)
	named := postTool(t, srv.URL, "learner", "get_learner_words", learnerWordsInput{LearnerID: &own}, nil)
	namedBody := readBody(t, named)
	if named.StatusCode != http.StatusOK || strings.Contains(namedBody, `"isError":true`) || !strings.Contains(namedBody, "own-word") {
		t.Fatalf("named %d %s", named.StatusCode, namedBody)
	}

	admin := postTool(t, srv.URL, "admin", "get_learner_words", learnerWordsInput{LearnerID: &other}, nil)
	adminBody := readBody(t, admin)
	if admin.StatusCode != http.StatusOK || strings.Contains(adminBody, `"isError":true`) || !strings.Contains(adminBody, "secret-latte") {
		t.Fatalf("admin %d %s", admin.StatusCode, adminBody)
	}
	missing := postTool(t, srv.URL, "admin", "get_learner_words", learnerWordsInput{}, nil)
	missingBody := readBody(t, missing)
	if missing.StatusCode != http.StatusOK || !strings.Contains(missingBody, `"isError":true`) || !strings.Contains(missingBody, "learnerId is required") || strings.Contains(missingBody, "secret-latte") {
		t.Fatalf("admin omit %d %s", missing.StatusCode, missingBody)
	}

	anon := postTool(t, srv.URL, "", "get_learner_words", learnerWordsInput{LearnerID: &other}, nil)
	anonBody := readBody(t, anon)
	if anon.StatusCode != http.StatusUnauthorized || anonBody != "Unauthorized" || strings.Contains(anonBody, "secret-latte") {
		t.Fatalf("anon %d %s", anon.StatusCode, anonBody)
	}
}

func cardName(i int) string {
	return fmt.Sprintf("w%03d", i)
}

func near(got, want float64) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
