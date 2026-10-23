package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestLearnPageAndFirstStepLink(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	ctx := context.Background()
	if err := store.UpsertPatterns(ctx, []db.PatternInput{
		{Slug: "present-tense", Title: "Present tense", Summary: "The verb agrees with the subject.", Example: "אֲנִי הוֹלֵךְ"},
		{Slug: "past-tense", Title: "Past tense", Summary: "A draft stays off the page."},
	}); err != nil {
		t.Fatal(err)
	}
	body := "## How it works\n\nFish & chips are **bold** and *quiet*.\n\n### Detail\n\n- first\n- {he:שָׁלוֹם|shalom}\n\nRead [the note](/learn/present-tense)."
	if err := store.UpsertExplainer(ctx, db.Explainer{Slug: "present-tense", Title: "Present tense", Body: body, Status: "published"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertExplainer(ctx, db.Explainer{Slug: "past-tense", Title: "Past tense", Body: "## Later\n\nNot yet.", Status: "draft"}); err != nil {
		t.Fatal(err)
	}

	index := getLearner(t, h, learner, "/learn")
	if !strings.Contains(index, "Present tense") || strings.Contains(index, "Past tense") {
		t.Fatal("the index shows a draft or hides the published note")
	}
	if !strings.Contains(index, `lang="he" dir="rtl"`) {
		t.Fatal("the example is not marked Hebrew")
	}

	page := getLearner(t, h, learner, "/learn/present-tense")
	for _, want := range []string{
		"<h2>How it works</h2>", "<h3>Detail</h3>", "<strong>bold</strong>", "<em>quiet</em>",
		"Fish &amp; chips", `<bdi class="he" lang="he" dir="rtl">שָׁלוֹם</bdi>`,
		`href="/learn/present-tense"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("explainer lacks %s", want)
		}
	}
	if strings.Contains(page, "Fish & chips") {
		t.Fatal("the body was not escaped")
	}
	if code, _ := getLearnerCode(t, h, learner, "/learn/past-tense"); code != http.StatusNotFound {
		t.Fatalf("draft = %d", code)
	}
	if code, _ := getLearnerCode(t, h, learner, "/learn/missing"); code != http.StatusNotFound {
		t.Fatalf("unknown = %d", code)
	}
	sheet := getLearner(t, h, learner, "/_hx/learn/present-tense")
	if !strings.Contains(sheet, `id="how-title"`) || strings.Contains(sheet, "<html") {
		t.Fatal("the sheet is not the fragment")
	}

	id := shellUnit(t, store, "Greetings", 1, "shared", 2)
	slug := "present-tense"
	if err := store.SetUnitMeta(ctx, id, &slug, nil); err != nil {
		t.Fatal(err)
	}
	link := "/lesson/" + itoa(id) + "/0"
	first := getLearner(t, h, learner, link)
	if !strings.Contains(first, "How this works") || !strings.Contains(first, `href="/learn/present-tense"`) || !strings.Contains(first, `hx-get="/_hx/learn/present-tense"`) {
		t.Fatal("the first step has no explainer link")
	}
	second := postLesson(t, h, learner.cookie, link, runValue(t, first), "continue", true, learner.csrf).Body.String()
	if strings.Contains(second, "How this works") {
		t.Fatal("a later step still links the explainer")
	}
	again := getLearner(t, h, learner, "/learn/present-tense")
	if !strings.Contains(again, "/lesson/"+itoa(id)+"/0") {
		t.Fatal("the explainer does not link the lesson")
	}
}

func TestRootsHubAndCardLink(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	const family, lonely = "שׁ־מ־ע", "ל־ב"
	hear := units.Normalize("שָׁמַע")
	news := units.Normalize("שְׁמוּעָה")
	heart := units.Normalize("לֵב")
	for _, row := range []struct{ text, gloss, root string }{
		{hear, "hear", family},
		{news, "news", family},
		{heart, "heart", lonely},
	} {
		if _, err := store.ExecContext(context.Background(),
			`INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, ?, 'noun', ?)`,
			row.text, row.gloss, row.root); err != nil {
			t.Fatal(err)
		}
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: "Hearing", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hear + " " + heart},
		}}},
	}, "shared")
	publish(t, store, id)

	index := getLearner(t, h, learner, "/roots")
	if !strings.Contains(index, family) || !strings.Contains(index, "2 words") || strings.Contains(index, lonely) {
		t.Fatalf("index = %s", index)
	}
	if !strings.Contains(index, `lang="he" dir="rtl"`) {
		t.Fatal("the root letters are not marked Hebrew")
	}
	one := getLearner(t, h, learner, "/roots/"+url.PathEscape(family))
	if !strings.Contains(one, hear) || !strings.Contains(one, `lang="he" dir="rtl"`) || strings.Contains(one, `class="wcard__root"`) {
		t.Fatal("the root page is missing its words or links to itself")
	}
	if code, _ := getLearnerCode(t, h, learner, "/roots/"+url.PathEscape(lonely)); code != http.StatusNotFound {
		t.Fatalf("invisible root = %d", code)
	}
	if code, _ := getLearnerCode(t, h, learner, "/roots/abc"); code != http.StatusNotFound {
		t.Fatalf("bad path = %d", code)
	}

	lesson := getLearner(t, h, learner, "/lesson/"+itoa(id)+"/0")
	if !strings.Contains(lesson, `class="wcard__root" href="/roots/`+family+`"`) {
		t.Fatal("the visible root has no card link")
	}
	if strings.Contains(lesson, "/roots/"+lonely) {
		t.Fatal("the invisible root has a card link")
	}
}

func getLearnerCode(t *testing.T, h http.Handler, learner learnerFix, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}
