package pages

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
)

const pointed = "חָשַׁב"

func guideSnapshot(planID, unitTitle, lessonID, title, visibility, recipe string, extra string) PlanSnapshot {
	if recipe == "" {
		recipe = "guide"
	}
	body := `{
		"id": "` + planID + `",
		"status": "published",
		"audience": "shared",
		"assigneeId": null,
		"units": [{
			"id": "unit",
			"title": "` + unitTitle + `",
			"lessons": [{
				"id": "` + lessonID + `",
				"title": "` + title + `",
				"visibility": "` + visibility + `",
				"recipe": "` + recipe + `",
				"blocks": [{
					"id": "rule",
					"type": "explainer",
					"title": "The root",
					"sections": [{
						"id": "roots",
						"heading": "Three letters",
						"paragraphs": ["A Hebrew verb has a root.", "Say <b>no</b>."],
						"examples": [{
							"id": "e1",
							"he": "` + pointed + `",
							"en": "he thought",
							"translit": "chashav",
							"root": "ח.ש.ב"
						}],
						"table": {
							"columns": ["Person", "Past tense"],
							"rows": [["I", "` + pointed + `"]]
						},
						"callouts": [{
							"id": "c1",
							"tone": "green",
							"title": "Three root letters",
							"text": "All three stay."
						}]
					}]
				}]
			}]
		}]
	}`
	if extra != "" {
		body = extra
	}
	return PlanSnapshot{ID: planID, Snapshot: body}
}

type memSource struct {
	rows   []PlanSnapshot
	grants map[string]map[int64]bool
	err    error
}

func (m memSource) Snapshots(context.Context) ([]PlanSnapshot, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rows, nil
}

func (m memSource) HasGrant(_ context.Context, lessonID string, userID int64) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.grants[lessonID][userID], nil
}

func newMux(src Source) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	NewHandler(src).Register(mux)
	return mux
}

func get(t *testing.T, mux *http.ServeMux, path string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func body(rec *httptest.ResponseRecorder) string {
	return rec.Body.String()
}

func TestShelfMatchesTheNodePage(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
		guideSnapshot("guides-shelf-he", "פעלים", "shoresh", pointed, "everyone", "guide", ""),
		guideSnapshot("secret-guides", "Hidden", "secret", "ZZ-HIDDEN-GUIDE", "hidden", "guide", ""),
		guideSnapshot("other-plan", "Other", "chat", "A conversation", "everyone", "topic-conversation", ""),
		guideSnapshot("bad id", "Nope", "nope", "Nope", "everyone", "guide", ""),
	}}
	mux := newMux(src)
	rec := get(t, mux, "/guides/", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	html := body(rec)
	for _, want := range []string{
		`<html lang="en" dir="ltr" translate="no"`,
		`class="has-hebrew-shell"`,
		`href="/shared/worksheet.css"`,
		`data-route-ready="guides"`,
		`id="guides-app"`,
		`id="app-shell"`,
		`id="app-main"`,
		`id="app-route-announce"`,
		`data-nav="guides"`,
		`aria-current="page"`,
		`מדריכים`,
		`lang="en" dir="ltr">Guides`,
		`data-guides-state="ready"`,
		`data-topic`,
		`data-plan-id="guides-shelf"`,
		`data-lesson-id="decoding"`,
		`href="/shared/plan/?id=guides-shelf&amp;lesson=decoding"`,
		`lang="en" dir="ltr"><a class="lesson-link" href="/shared/plan/?id=guides-shelf&amp;lesson=decoding">Decoding Hebrew Verbs</a>`,
		`lang="he" dir="rtl">פעלים`,
		`lang="he" dir="rtl"><a class="lesson-link" href="/shared/plan/?id=guides-shelf-he&amp;lesson=shoresh">` + pointed + `</a>`,
		`hebrew-spa has-hebrew-shell guides-page`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(html, "ZZ-HIDDEN-GUIDE") {
		t.Fatal("hidden guide was listed")
	}
	if strings.Contains(html, "A conversation") {
		t.Fatal("non-guide lesson was listed")
	}
	if strings.Contains(html, "fonts.googleapis.com") || strings.Contains(html, "cdn.") {
		t.Fatal("page references a font CDN")
	}
	if !strings.Contains(html, `role="radiogroup"`) || !strings.Contains(html, "אוטומטי") || !strings.Contains(html, "icon-theme") {
		t.Fatal("theme switch missing")
	}
	if !strings.Contains(html, `method="post"`) || !strings.Contains(html, `action="/_hx/ui"`) || !strings.Contains(html, `name="lang"`) || !strings.Contains(html, `name="theme"`) {
		t.Fatal("language and theme controls are not forms")
	}
	if strings.Contains(html, "data-say=") || strings.Contains(html, "data-clip=") || strings.Contains(html, "status-chip") {
		t.Fatal("shelf used a retired hook or a status chip")
	}
	if !strings.Contains(html, `guide-card__letter" lang="he" dir="rtl">ח</span>`) {
		t.Fatal("cover letter kept its nikud")
	}
	if strings.Contains(html, `hebrew-shell-account-btn`) {
		t.Fatal("account button rendered before sessions exist")
	}
	if strings.Count(html, "<script") != 2 {
		t.Fatalf("script count %d, want the theme pre-paint script and speech.js", strings.Count(html, "<script"))
	}
	if strings.Contains(html, "localStorage") {
		t.Fatal("page script reads localStorage")
	}
	if !strings.Contains(html, "prefers-color-scheme: dark") || !strings.Contains(html, "data-theme-choice") {
		t.Fatal("theme pre-paint script missing")
	}
	if !strings.Contains(html, "noto-sans-hebrew-hebrew.woff2") {
		t.Fatal("hebrew face was not preloaded")
	}
	if strings.Count(html, "<script src=") != 1 || !strings.Contains(html, `<script src="/shared/player/speech.js"></script>`) {
		t.Fatal("speech.js should be the only script src")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff missing")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
}

func TestEmptyShelf(t *testing.T) {
	rec := get(t, newMux(memSource{}), "/guides", false)
	html := body(rec)
	if !strings.Contains(html, `data-guides-state="empty"`) {
		t.Fatalf("empty state missing:\n%s", html)
	}
	if !strings.Contains(html, "עֲדַיִן אֵין מַדְרִיכִים") || !strings.Contains(html, "No guides published yet") {
		t.Fatalf("empty copy missing:\n%s", html)
	}
	if !strings.Contains(html, "empty-guides.svg") {
		t.Fatal("empty illustration missing")
	}
}

func TestIndexHTMLAndFragment(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
	}}
	mux := newMux(src)
	full := get(t, mux, "/guides/index.html", false)
	if !strings.Contains(body(full), "<!DOCTYPE html>") { // templ may lowercase doctype
		if !strings.Contains(strings.ToLower(body(full)), "<!doctype html>") {
			t.Fatal("full page has no doctype")
		}
	}
	frag := get(t, mux, "/guides/", true)
	html := body(frag)
	if strings.Contains(strings.ToLower(html), "<html") {
		t.Fatal("fragment included the document")
	}
	if !strings.Contains(html, `data-guides-state="ready"`) || !strings.Contains(html, "Decoding Hebrew Verbs") {
		t.Fatalf("fragment missing the shelf:\n%s", html)
	}
}

func TestGuideDetailRendersExplainerWithoutPictures(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
	}}
	mux := newMux(src)
	rec := get(t, mux, "/shared/plan/?id=guides-shelf&lesson=decoding", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, body(rec))
	}
	html := body(rec)
	for _, want := range []string{
		`id="plan-root"`,
		`data-plan-ready="1"`,
		`data-recipe="guide"`,
		`data-section-id="roots"`,
		`data-block-type="explainer"`,
		"Three letters",
		"A Hebrew verb has a root.",
		pointed,
		`class="root-letter"`,
		`lang="en" dir="ltr">he thought`,
		`<bdi class="translit" lang="en" dir="ltr">chashav</bdi>`,
		`<span class="example-row__root"><bdi class="meaning root-label" lang="he">ח.ש.ב</bdi></span>`,
		`data-speak="` + pointed + `"`,
		`class="audio-btn"`,
		`example-row__gloss`,
		`class="explainer-table"`,
		`data-tone="green"`,
		`lang="en" dir="ltr">Go`,
		`lang="he" dir="rtl">קדימה`,
		`class="guide-body"`,
		`Say &lt;b&gt;no&lt;/b&gt;`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	root := html[strings.Index(html, `id="plan-root"`):]
	if strings.Contains(html, "data-say=") || strings.Contains(html, "data-clip=") || strings.Contains(html, "data-audio=") {
		t.Fatal("detail used a hook other than data-speak")
	}
	if strings.Contains(html, "plan-page") || strings.Contains(html, "story-body") || strings.Contains(html, `class="page"`) {
		t.Fatal("detail still uses the player page classes")
	}
	if strings.Contains(root, "<img") {
		t.Fatal("guide rendered a picture")
	}
	if !strings.Contains(html, pointed) {
		t.Fatal("nikud was dropped")
	}
}

func TestDetailFragmentAndMisses(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
		guideSnapshot("secret-guides", "Hidden", "secret", "ZZ-HIDDEN-GUIDE", "hidden", "guide", ""),
	}}
	mux := newMux(src)
	frag := get(t, mux, "/shared/plan/?id=guides-shelf&lesson=decoding", true)
	if strings.Contains(strings.ToLower(body(frag)), "<html") {
		t.Fatal("detail fragment included the document")
	}
	if !strings.Contains(body(frag), `data-section-id="roots"`) {
		t.Fatal("detail fragment missing the section")
	}
	hidden := get(t, mux, "/shared/plan/?id=secret-guides&lesson=secret", false)
	if hidden.Code != http.StatusNotFound || strings.Contains(body(hidden), "ZZ-HIDDEN-GUIDE") {
		t.Fatalf("hidden guide status %d", hidden.Code)
	}
	if !strings.Contains(body(hidden), "התוכנית לא נמצאה.") {
		t.Fatal("not-found copy missing")
	}
	missing := get(t, mux, "/shared/plan/?id=NOT%20A%20SLUG&lesson=decoding", false)
	if missing.Code != http.StatusNotFound || !strings.Contains(body(missing), "חסר מזהה תוכנית.") {
		t.Fatalf("bad id status %d body %s", missing.Code, body(missing))
	}
	post := httptest.NewRequest(http.MethodPost, "/guides/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d", rec.Code)
	}
}

func TestEscapesTitleAndDropsUnpublished(t *testing.T) {
	hostile := guideSnapshot("guides-shelf", "Verbs", "decoding", `<img src=x onerror=alert(1)>`, "everyone", "guide", "")
	draft := PlanSnapshot{ID: "guides-shelf", Snapshot: strings.Replace(hostile.Snapshot, `"published"`, `"draft"`, 1)}
	draft.ID = "draft-guide"
	draft.Snapshot = strings.Replace(draft.Snapshot, `"id": "guides-shelf"`, `"id": "draft-guide"`, 1)
	personal := strings.Replace(hostile.Snapshot, `"audience": "shared"`, `"audience": "personal"`, 1)
	personal = strings.Replace(personal, `"assigneeId": null`, `"assigneeId": "learner"`, 1)
	personal = strings.Replace(personal, `"id": "guides-shelf"`, `"id": "mine"`, 1)
	src := memSource{rows: []PlanSnapshot{
		hostile,
		draft,
		{ID: "mine", Snapshot: personal},
		{ID: "broken", Snapshot: "{"},
	}}
	html := body(get(t, newMux(src), "/guides/", false))
	if strings.Contains(html, "<img src=x") || !strings.Contains(html, "&lt;img") {
		t.Fatalf("title was not escaped:\n%s", html)
	}
	if strings.Contains(html, "draft-guide") || strings.Contains(html, `data-plan-id="mine"`) {
		t.Fatal("unpublished or personal guide was listed")
	}
}

func TestCatalogCapSkipsAGuidePast200(t *testing.T) {
	var rows []PlanSnapshot
	for i := 0; i < 200; i++ {
		id := "chat-" + two(i)
		rows = append(rows, guideSnapshot(id, "Talk", "lesson", "Chat", "everyone", "topic-conversation", ""))
	}
	rows = append(rows, guideSnapshot("guides-shelf", "Verbs", "decoding", "Too late", "everyone", "guide", ""))
	html := body(get(t, newMux(memSource{rows: rows}), "/guides/", false))
	if strings.Contains(html, "Too late") || !strings.Contains(html, `data-guides-state="empty"`) {
		t.Fatal("guide past the catalog cap was shown, or the shelf was not empty")
	}
}

func two(n int) string {
	const digits = "abcdefghijklmnopqrstuvwxyz"
	return string([]byte{digits[n/26], digits[n%26]})
}

func TestSelectedNeedsAGrantAndAnonymousDoesNotHaveOne(t *testing.T) {
	row := guideSnapshot("guides-shelf", "Verbs", "decoding", "Selected", "selected", "guide", "")
	html := body(get(t, newMux(memSource{rows: []PlanSnapshot{row}, grants: map[string]map[int64]bool{
		"decoding": {7: true},
	}}), "/guides/", false))
	if strings.Contains(html, "Selected") {
		t.Fatal("selected guide was shown without a session")
	}
	list, err := Guides(context.Background(), memSource{rows: []PlanSnapshot{row}, grants: map[string]map[int64]bool{
		"decoding": {7: true},
	}}, &User{ID: 7, Username: "learner"})
	if err != nil || len(list.Topics) != 1 {
		t.Fatalf("granted reader: %+v %v", list, err)
	}
}

func TestReadErrorPaintsTheErrorState(t *testing.T) {
	html := body(get(t, newMux(memSource{err: errors.New("locked")}), "/guides/", false))
	if !strings.Contains(html, `data-guides-state="error"`) || !strings.Contains(html, "שגיאה בטעינת המדריכים.") {
		t.Fatalf("error state missing:\n%s", html)
	}
}

func TestHealthzSurvivesTheMount(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	t.Setenv(DevEnv, "")
	Mount(mux, nil)
	off := get(t, mux, "/guides/", false)
	if off.Code != http.StatusNotFound {
		t.Fatalf("flag off status %d", off.Code)
	}
	health := get(t, mux, "/healthz", false)
	if health.Code != http.StatusOK || body(health) != "ok" {
		t.Fatalf("healthz %d %q", health.Code, body(health))
	}
	t.Setenv(DevEnv, "1")
	Mount(mux, nil)
	on := get(t, mux, "/guides/", false)
	if on.Code != http.StatusOK || !strings.Contains(body(on), `data-guides-state="error"`) {
		t.Fatalf("flag on without a database: %d\n%s", on.Code, body(on))
	}
	health = get(t, mux, "/healthz", false)
	if body(health) != "ok" {
		t.Fatal("mount changed /healthz")
	}
}

func TestHeroIsNotRepeatedInTheGrid(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
		guideSnapshot("guides-shelf", "Verbs", "roots", pointed, "everyone", "guide", ""),
		guideSnapshot("nouns", "Nouns", "bayit", "House", "everyone", "guide", ""),
	}}
	html := body(get(t, newMux(src), "/guides/", false))
	if !strings.Contains(html, `data-guides-hero`) {
		t.Fatal("hero missing")
	}
	if strings.Count(html, `data-lesson-id="decoding"`) != 0 {
		t.Fatal("featured guide was also a card")
	}
	if !strings.Contains(html, "lesson=decoding") {
		t.Fatal("hero does not open the featured guide")
	}
	if !strings.Contains(html, `data-lesson-id="roots"`) || !strings.Contains(html, `data-lesson-id="bayit"`) {
		t.Fatal("the other guides left the grid")
	}
	if strings.Contains(html, `id="topic-verbs"`) && strings.Contains(html, `data-lesson-id="decoding"`) {
		t.Fatal("verbs topic still contains the hero card")
	}
}

func TestHebrewCardButtonSitsOutsideTheLink(t *testing.T) {
	row := guideSnapshot("guides-shelf", "Verbs", "shoresh", pointed, "everyone", "guide", "")
	html := body(get(t, newMux(memSource{rows: []PlanSnapshot{row}}), "/guides/", false))
	i := strings.Index(html, `data-lesson-id="shoresh"`)
	if i < 0 {
		t.Fatal("card missing")
	}
	article := html[i:]
	end := strings.Index(article, "</article>")
	if end < 0 {
		t.Fatal("article did not close")
	}
	article = article[:end]
	linkEnd := strings.Index(article, "</a>")
	btn := strings.Index(article, "audio-btn")
	if linkEnd < 0 || btn < 0 || btn < linkEnd {
		t.Fatalf("button is inside the link or missing:\n%s", article)
	}
	if !strings.Contains(article, `data-speak="`+pointed+`"`) {
		t.Fatal("card button has no data-speak")
	}
	if strings.Contains(article, "data-audio=") {
		t.Fatal("card invented a clip")
	}
}

func TestTwoGuidesHaveNoHero(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding Hebrew Verbs", "everyone", "guide", ""),
		guideSnapshot("guides-shelf", "Verbs", "roots", pointed, "everyone", "guide", ""),
	}}
	html := body(get(t, newMux(src), "/guides/", false))
	if strings.Contains(html, `data-guides-hero`) {
		t.Fatal("hero shown for fewer than three guides")
	}
	if !strings.Contains(html, `data-lesson-id="decoding"`) || !strings.Contains(html, `data-lesson-id="roots"`) {
		t.Fatal("both cards should stay in the grid")
	}
}

func TestMixedProseWrapsEachHebrewRun(t *testing.T) {
	extra := `{
		"id": "guides-shelf",
		"status": "published",
		"audience": "shared",
		"assigneeId": null,
		"units": [{
			"id": "unit",
			"title": "Verbs",
			"lessons": [{
				"id": "decoding",
				"title": "Decoding Hebrew Verbs",
				"visibility": "everyone",
				"recipe": "guide",
				"blocks": [{
					"id": "rule",
					"type": "explainer",
					"title": "The root",
					"sections": [{
						"id": "roots",
						"heading": "Three letters",
						"paragraphs": ["Say שָׁלוֹם and then בֹּקֶר טוֹב today.", "שָׁלוֹם"],
						"examples": [{
							"id": "e1",
							"he": "` + pointed + `",
							"en": "he thought",
							"translit": "chashav",
							"root": "ח.ש.ב"
						}]
					}]
				}]
			}]
		}]
	}`
	html := body(get(t, newMux(memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "decoding", "Decoding", "everyone", "guide", extra),
	}}), "/shared/plan/?id=guides-shelf&lesson=decoding", false))
	if !strings.Contains(html, `<p lang="en" dir="ltr">Say <bdi lang="he" class="he-inline">שָׁלוֹם</bdi>`) {
		t.Fatalf("mixed paragraph missing a Hebrew run:\n%s", html)
	}
	if !strings.Contains(html, `<bdi lang="he" class="he-inline">בֹּקֶר טוֹב</bdi>`) {
		t.Fatal("a Hebrew phrase was split")
	}
	if !strings.Contains(html, `<p lang="he" dir="rtl" class="he-text--pointed">`) {
		t.Fatal("a Hebrew-only paragraph lost dir=rtl")
	}
	if !strings.Contains(html, `class="example-row__he" lang="he" dir="rtl"`) {
		t.Fatal("Hebrew column lost dir=rtl")
	}
}

func TestHebrewDetailHeadwordButtonStaysInTheHeading(t *testing.T) {
	row := guideSnapshot("guides-shelf", "Usage", "shoresh", pointed, "everyone", "guide", "")
	html := body(get(t, newMux(memSource{rows: []PlanSnapshot{row}}), "/shared/plan/?id=guides-shelf&lesson=shoresh", false))
	heading := html[strings.Index(html, `data-page-heading`):]
	end := strings.Index(heading, "</header>")
	if end < 0 {
		t.Fatal("heading did not close")
	}
	heading = heading[:end]
	if !strings.Contains(heading, `class="page-heading-title-row"`) {
		t.Fatalf("headword row missing:\n%s", heading)
	}
	if !strings.Contains(heading, `data-speak="`+pointed+`"`) || !strings.Contains(heading, `class="audio-btn"`) {
		t.Fatalf("audio button is outside the heading:\n%s", heading)
	}
	rowStart := strings.Index(heading, `page-heading-title-row`)
	sub := strings.Index(heading, `page-heading-sub`)
	btn := strings.Index(heading, `audio-btn`)
	if rowStart < 0 || btn < rowStart || (sub >= 0 && btn > sub) {
		t.Fatalf("button is not inline with the headword:\n%s", heading)
	}
}

func TestHeroHebrewTitleIsNotRepeated(t *testing.T) {
	src := memSource{rows: []PlanSnapshot{
		guideSnapshot("guides-shelf", "Verbs", "shoresh", pointed, "everyone", "guide", ""),
		guideSnapshot("nouns", "Nouns", "bayit", "House", "everyone", "guide", ""),
		guideSnapshot("numbers", "Numbers", "eser", "Ten", "everyone", "guide", ""),
	}}
	html := body(get(t, newMux(src), "/guides/", false))
	hero := html[strings.Index(html, `data-guides-hero`):]
	hero = hero[:strings.Index(hero, `guides-filters`)]
	if !strings.Contains(hero, `guides-hero__lettering" lang="he" dir="rtl">`+pointed) {
		t.Fatalf("hero lettering missing:\n%s", hero)
	}
	titleAt := strings.Index(hero, `guides-hero__title`)
	if titleAt < 0 {
		t.Fatal("hero topic missing")
	}
	title := hero[titleAt:]
	end := strings.Index(title, "</p>")
	if end < 0 || strings.Contains(title[:end], pointed) {
		t.Fatalf("hero repeated the Hebrew title:\n%s", hero)
	}
	if !strings.Contains(hero, `guides-hero__title"><bdi lang="en" dir="ltr">Verbs</bdi>`) {
		t.Fatal("hero subtitle is not the topic")
	}
}

func TestClipOfLongTitle(t *testing.T) {
	title := strings.Repeat("א", 130)
	row := guideSnapshot("guides-shelf", "Verbs", "decoding", title, "everyone", "guide", "")
	html := body(get(t, newMux(memSource{rows: []PlanSnapshot{row}}), "/guides/", false))
	if strings.Contains(html, strings.Repeat("א", 121)) {
		t.Fatal("title was not clipped at 120")
	}
	if !strings.Contains(html, strings.Repeat("א", 120)) {
		t.Fatal("clipped title missing")
	}
}

func TestNoRuleOutsideADRResolvesToRubik(t *testing.T) {
	raw, err := os.ReadFile("../../../../shared/worksheet.css")
	if err != nil {
		t.Fatal(err)
	}
	rawCSS := string(raw)
	const startMark = "=== INIT-004/SPEC-013 ADR-0006 tokens ==="
	const endMark = "=== end INIT-004/SPEC-013 ADR-0006 tokens ==="
	start := strings.Index(rawCSS, startMark)
	end := strings.Index(rawCSS, endMark)
	if start < 0 || end < start {
		t.Fatal("ADR-0006 block missing")
	}
	if !strings.Contains(rawCSS[start:end], "--font-family: Rubik, sans-serif") {
		t.Fatal("ADR-0006 font token changed")
	}
	css := stripCSSComments(rawCSS)
	vars := map[string]string{}
	decl := regexp.MustCompile(`(?m)(--[a-z0-9-]+)\s*:\s*([^;]+);`)
	for _, m := range decl.FindAllStringSubmatch(css, -1) {
		vars[m[1]] = strings.TrimSpace(m[2])
	}
	outside := stripCSSComments(rawCSS[:start] + rawCSS[end+len(endMark):])
	outside = regexp.MustCompile(`@font-face\s*\{[^}]*\}`).ReplaceAllString(outside, "")
	face := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	last := map[string]string{}
	for _, m := range face.FindAllStringSubmatch(outside, -1) {
		body := m[2]
		idx := strings.LastIndex(body, "font-family")
		if idx < 0 {
			continue
		}
		rest := body[idx:]
		if strings.HasPrefix(strings.TrimSpace(rest), "--") {
			continue
		}
		colon := strings.Index(rest, ":")
		semi := strings.Index(rest, ";")
		if colon < 0 || semi < colon {
			continue
		}
		value := strings.TrimSpace(rest[colon+1 : semi])
		// Node plan pages still name the ADR font token. Guides do not.
		if value == "var(--font-family)" {
			continue
		}
		last[strings.TrimSpace(m[1])] = value
	}
	for sel, value := range last {
		resolved := resolveFont(value, vars)
		if strings.Contains(strings.ToLower(resolved), "rubik") {
			t.Errorf("%s resolves to Rubik via %s", sel, value)
		}
	}
}

func resolveFont(value string, vars map[string]string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "var(") && strings.HasSuffix(value, ")") {
		inner := value[len("var(") : len(value)-1]
		name, fallback, _ := strings.Cut(inner, ",")
		name = strings.TrimSpace(name)
		if v, ok := vars[name]; ok && v != value {
			return resolveFont(v, vars)
		}
		if strings.TrimSpace(fallback) != "" {
			return resolveFont(strings.TrimSpace(fallback), vars)
		}
	}
	return value
}

func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestUIFollowsCookies(t *testing.T) {
	mux := newMux(memSource{})
	req := httptest.NewRequest(http.MethodGet, "/guides/", nil)
	req.AddCookie(&http.Cookie{Name: "hebrew-ui-lang", Value: "he"})
	req.AddCookie(&http.Cookie{Name: "hebrew-theme", Value: "dark"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	html := body(rec)
	for _, want := range []string{
		`<html lang="he" dir="rtl"`,
		`data-ui-lang="he"`,
		`data-theme="dark"`,
		`data-theme-choice="dark"`,
		`name="lang" value="he"`,
		`aria-pressed="true"`,
		`content="#0E121B"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(html, `data-theme-choice="auto"`) {
		t.Fatal("dark choice was rendered as auto")
	}
}

func TestUIDefaultsToEnglishAndAuto(t *testing.T) {
	rec := get(t, newMux(memSource{}), "/guides/", false)
	html := body(rec)
	if !strings.Contains(html, `<html lang="en" dir="ltr"`) || !strings.Contains(html, `data-theme-choice="auto"`) || !strings.Contains(html, `data-theme="light"`) {
		t.Fatalf("defaults missing:\n%s", html[:min(400, len(html))])
	}
	csrf := responseCookie(rec, "hebrew-csrf")
	if csrf == nil || !csrf.HttpOnly || csrf.SameSite != http.SameSiteLaxMode || len(csrf.Value) != 64 {
		t.Fatalf("csrf cookie %#v", csrf)
	}
}

func TestSetUIPost(t *testing.T) {
	mux := newMux(memSource{})
	first := get(t, mux, "/guides/?id=kept", false)
	csrf := responseCookie(first, "hebrew-csrf")
	if csrf == nil {
		t.Fatal("no csrf cookie")
	}
	form := url.Values{}
	form.Set("csrf", csrf.Value)
	form.Set("lang", "he")
	form.Set("next", "/shared/plan/?id=guides-shelf&lesson=decoding")
	req := httptest.NewRequest(http.MethodPost, "/_hx/ui", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrf)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, body(rec))
	}
	if loc := rec.Header().Get("Location"); loc != "/shared/plan/?id=guides-shelf&lesson=decoding" {
		t.Fatalf("location %q", loc)
	}
	lang := responseCookie(rec, "hebrew-ui-lang")
	if lang == nil || lang.Value != "he" || !lang.HttpOnly {
		t.Fatalf("lang cookie %#v", lang)
	}

	hx := httptest.NewRequest(http.MethodPost, "/_hx/ui", strings.NewReader(form.Encode()))
	hx.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hx.Header.Set("HX-Request", "true")
	hx.AddCookie(csrf)
	hxRec := httptest.NewRecorder()
	mux.ServeHTTP(hxRec, hx)
	if hxRec.Code != http.StatusNoContent {
		t.Fatalf("htmx status %d", hxRec.Code)
	}
	if hxRec.Header().Get("HX-Redirect") != "/shared/plan/?id=guides-shelf&lesson=decoding" {
		t.Fatalf("HX-Redirect %q", hxRec.Header().Get("HX-Redirect"))
	}
}

func TestSetUIRejectsBadOriginCSRFAndOpenRedirect(t *testing.T) {
	mux := newMux(memSource{})
	first := get(t, mux, "/guides/", false)
	csrf := responseCookie(first, "hebrew-csrf")
	post := func(origin, token, next string) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{}
		form.Set("csrf", token)
		form.Set("theme", "dark")
		form.Set("next", next)
		req := httptest.NewRequest(http.MethodPost, "/_hx/ui", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Host = "example.com"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.AddCookie(csrf)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if post("https://evil.example", csrf.Value, "/guides/").Code != http.StatusForbidden {
		t.Fatal("cross-origin post was accepted")
	}
	if post("", "not-the-token", "/guides/").Code != http.StatusForbidden {
		t.Fatal("bad csrf was accepted")
	}
	for _, next := range []string{"https://evil.example/phish", "//evil.example", "/\\evil", "/guides/\r\nSet-Cookie: x=y"} {
		rec := post("", csrf.Value, next)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("next %q status %d", next, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/guides/" {
			t.Fatalf("next %q location %q", next, loc)
		}
	}
	ok := post("http://example.com", csrf.Value, "/guides/")
	if ok.Code != http.StatusSeeOther {
		t.Fatalf("same-origin status %d", ok.Code)
	}
	theme := responseCookie(ok, "hebrew-theme")
	if theme == nil || theme.Value != "dark" {
		t.Fatalf("theme cookie %#v", theme)
	}
}

func stripCSSComments(css string) string {
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			return css
		}
		j := strings.Index(css[i:], "*/")
		if j < 0 {
			return css[:i]
		}
		css = css[:i] + css[i+j+2:]
	}
}
