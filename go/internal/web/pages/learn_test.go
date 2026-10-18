package pages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/path"
)

func learnMux(reader path.Reader, gate path.GateAck, cfg path.Config) *http.ServeMux {
	mux := http.NewServeMux()
	NewLearning(reader, gate, cfg, path.DemoPlanID).Register(mux)
	return mux
}

func demoMux() *http.ServeMux {
	demo := path.Demo()
	return learnMux(
		path.Mem{Snap: demo, Due: demo.Due},
		&path.MemGate{Err: path.ErrNotPersisted},
		path.DefaultConfig(),
	)
}

func TestPathAndTodayRender(t *testing.T) {
	mux := demoMux()
	lessons := get(t, mux, "/lessons/", false)
	today := get(t, mux, "/", false)
	if lessons.Code != http.StatusOK || today.Code != http.StatusOK {
		t.Fatalf("status lessons %d today %d", lessons.Code, today.Code)
	}
	pathHTML := body(lessons)
	todayHTML := body(today)
	for _, html := range []string{pathHTML, todayHTML} {
		if !strings.Contains(html, `lang="en"`) || !strings.Contains(html, `dir="ltr"`) {
			t.Fatal("document does not default to English, left to right")
		}
		if strings.Contains(html, "streak") || strings.Contains(html, "XP") || strings.Contains(html, ">xp<") {
			t.Fatal("xp or streak rendered")
		}
		if !strings.Contains(html, ">Unit 2<") {
			t.Fatal("rail label is not Unit N")
		}
		if !strings.Contains(html, "5 of 8 new words this unit") {
			t.Fatal("rail words line missing")
		}
		if !strings.Contains(html, `data-value="5"`) || !strings.Contains(html, `data-max="8"`) {
			t.Fatal("ring counts do not match the words line")
		}
		if !strings.Contains(html, "<bdi lang=\"he\"") && !strings.Contains(html, "lang=\"he\"") {
			t.Fatal("hebrew run missing")
		}
		if strings.Contains(html, "aria-disabled") {
			t.Fatal("a node is disabled")
		}
		if strings.Count(html, `aria-current="step"`) != 1 {
			t.Fatalf("aria-current=step count %d", strings.Count(html, `aria-current="step"`))
		}
	}
	if strings.Contains(pathHTML, "<script src=") == false || !strings.Contains(pathHTML, "/_static/htmx.min.js") {
		t.Fatal("path page did not load htmx")
	}
	if !strings.Contains(pathHTML, `href="/lessons/all"`) || !strings.Contains(pathHTML, "All lessons (list)") {
		t.Fatal("list link missing")
	}
	if !strings.Contains(pathHTML, `data-pass-needed="16"`) || !strings.Contains(pathHTML, `data-total="20"`) {
		t.Fatal("checkpoint did not receive pass_needed from config")
	}
	if strings.Contains(pathHTML, "16 marks") {
		t.Fatal("template hardcoded the example score")
	}
	if !strings.Contains(pathHTML, `data-fading="true"`) {
		t.Fatal("fading chip missing")
	}
	if !strings.Contains(pathHTML, `data-state="review"`) || !strings.Contains(pathHTML, "9 words due") {
		t.Fatal("review stop missing")
	}
	assertNotYetLinks(t, pathHTML)
	if !strings.Contains(todayHTML, `data-due="9"`) || !strings.Contains(todayHTML, "Review now instead") {
		t.Fatal("due card missing")
	}
	if !strings.Contains(todayHTML, "3 go into the lesson warm-up; a review stop appears at 8+ due") {
		t.Fatal("explainer did not use the config counts")
	}
	if !strings.Contains(todayHTML, `data-class-date="2026-10-20"`) || !strings.Contains(todayHTML, "data-speak=") {
		t.Fatal("tutor card or audio hook missing")
	}
	if !strings.Contains(todayHTML, "Continue path") || !strings.Contains(todayHTML, "/worksheets/out-the-door/") {
		t.Fatal("continue link missing")
	}
	frag := get(t, mux, "/lessons/", true)
	if strings.Contains(body(frag), "<html") || !strings.Contains(body(frag), `class="path"`) {
		t.Fatal("fragment was a full document")
	}
}

func TestPassNeededFollowsConfig(t *testing.T) {
	cfg := path.DefaultConfig()
	cfg.CheckpointPassMark = 0.50
	demo := path.Demo()
	mux := learnMux(path.Mem{Snap: demo, Due: demo.Due}, &path.MemGate{}, cfg)
	html := body(get(t, mux, "/lessons/", false))
	if strings.Contains(html, `data-pass-needed="16"`) || !strings.Contains(html, `data-pass-needed="10"`) {
		t.Fatal("pass_needed stayed at the example 16")
	}
}

func TestDueZeroHidesCardAndStop(t *testing.T) {
	snap := path.Build(path.DefaultConfig(), 0, false)
	snap.Tutor = nil
	mux := learnMux(path.Mem{Snap: snap, Due: 0}, &path.MemGate{}, path.DefaultConfig())
	html := body(get(t, mux, "/", false))
	if strings.Contains(html, "due-card") {
		t.Fatal("due card shown with nothing due")
	}
	if strings.Contains(html, "tutor-card") {
		t.Fatal("tutor card shown with no class")
	}
	if strings.Contains(html, `data-state="review"`) {
		t.Fatal("review stop inserted below the threshold")
	}
}

func TestGateAck(t *testing.T) {
	gate := &path.MemGate{Err: path.ErrNotPersisted}
	demo := path.Demo()
	mux := learnMux(path.Mem{Snap: demo, Due: demo.Due}, gate, path.DefaultConfig())
	page := get(t, mux, "/lessons/", false)
	tok := csrfToken(t, page)
	cookie := page.Result().Cookies()[0]

	ok := postGate(t, mux, cookie, tok, "http://example.com", "/worksheets/coffee/", "/worksheets/cafe-checkpoint/")
	if ok.Code != http.StatusOK {
		t.Fatalf("persisted failure status %d", ok.Code)
	}
	sheet := body(ok)
	if !strings.Contains(sheet, "<dialog") || !strings.Contains(sheet, "open") {
		t.Fatal("warning did not open")
	}
	if !strings.Contains(sheet, "finished Unit 2.") {
		t.Fatalf("warning copy: %s", sheet)
	}
	if !strings.Contains(sheet, `href="/worksheets/coffee/"`) || !strings.Contains(sheet, `href="/worksheets/cafe-checkpoint/"`) {
		t.Fatal("warning did not keep the links")
	}
	if !strings.Contains(sheet, "Go anyway") || !strings.Contains(sheet, "Test out") {
		t.Fatal("warning actions missing")
	}
	seen, seenErr := gate.GateWarningSeen(context.Background(), path.DevUserID, path.DemoPlanID, "unit-3")
	if seen || seenErr == nil {
		t.Fatal("ErrNotPersisted still stored the ack")
	}

	badOrigin := postGate(t, mux, cookie, tok, "http://evil.example", "/worksheets/coffee/", "/worksheets/cafe-checkpoint/")
	if badOrigin.Code != http.StatusForbidden {
		t.Fatalf("origin status %d", badOrigin.Code)
	}
	badToken := postGate(t, mux, cookie, "nope", "http://example.com", "/worksheets/coffee/", "/worksheets/cafe-checkpoint/")
	if badToken.Code != http.StatusForbidden {
		t.Fatalf("token status %d", badToken.Code)
	}
	evil := postGate(t, mux, cookie, tok, "http://example.com", "https://evil.example/phish", "/worksheets/cafe-checkpoint/")
	if evil.Code != http.StatusOK || strings.Contains(body(evil), "evil.example") {
		t.Fatal("open redirect was kept")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/_hx/units/unit-3/gate-ack", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code == http.StatusOK {
		t.Fatal("GET mutated or returned the warning")
	}
}

func TestMountLearningOffWithoutFlag(t *testing.T) {
	t.Setenv(DevEnv, "")
	mux := http.NewServeMux()
	MountLearning(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lessons/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("flag off status %d", rec.Code)
	}
}

func TestMountLearningOn(t *testing.T) {
	t.Setenv(DevEnv, "1")
	mux := http.NewServeMux()
	MountLearning(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Continue path") {
		t.Fatalf("dev mount status %d", rec.Code)
	}
}

func assertNotYetLinks(t *testing.T, html string) {
	t.Helper()
	var sawWarn, sawOpen bool
	for _, chunk := range pathNodeChunks(html) {
		if !strings.Contains(chunk, `data-state="not-yet"`) {
			continue
		}
		if strings.Contains(chunk, "aria-disabled") {
			t.Fatal("not-yet node disabled")
		}
		if !strings.Contains(chunk, "<a ") {
			t.Fatalf("not-yet node is not a link: %s", chunk)
		}
		label := attr(chunk, "aria-label")
		if !strings.HasSuffix(label, "not yet started") {
			t.Fatalf("accessible name %q", label)
		}
		gate := attr(chunk, "data-gate")
		if gate != "warn" && gate != "open" {
			t.Fatalf("data-gate %q", gate)
		}
		if gate == "warn" {
			sawWarn = true
			if !strings.Contains(chunk, `hx-post="/_hx/units/`) {
				t.Fatal("warn node did not post to the gate")
			}
		}
		if gate == "open" {
			sawOpen = true
			if strings.Contains(chunk, "hx-post") {
				t.Fatal("open node still posts")
			}
		}
	}
	if !sawWarn || !sawOpen {
		t.Fatalf("warn %v open %v", sawWarn, sawOpen)
	}
}

func pathNodeChunks(html string) []string {
	var out []string
	rest := html
	for {
		i := strings.Index(rest, `<ol class="path`)
		if i < 0 {
			return out
		}
		rest = rest[i:]
		j := strings.Index(rest, "</ol>")
		if j < 0 {
			return out
		}
		list := rest[:j]
		parts := strings.Split(list, `<li class="path-node"`)
		out = append(out, parts[1:]...)
		rest = rest[j+5:]
	}
}

func attr(chunk, name string) string {
	key := name + `="`
	i := strings.Index(chunk, key)
	if i < 0 {
		return ""
	}
	rest := chunk[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func csrfToken(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	html := body(rec)
	i := strings.Index(html, "X-CSRF-Token")
	if i < 0 {
		t.Fatal("csrf header missing")
	}
	rest := html[i:]
	for j := 0; j+64 <= len(rest); j++ {
		if isHex(rest[j : j+64]) {
			return rest[j : j+64]
		}
	}
	t.Fatal("csrf token missing")
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

func postGate(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, token, origin, next, checkpoint string) *httptest.ResponseRecorder {
	t.Helper()
	form := "next=" + next + "&checkpoint=" + checkpoint
	req := httptest.NewRequest(http.MethodPost, "/_hx/units/unit-3/gate-ack", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	req.Header.Set("X-CSRF-Token", token)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHTMXAsset(t *testing.T) {
	mux := demoMux()
	rec := get(t, mux, "/_static/htmx.min.js", false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("htmx %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() < 1000 {
		t.Fatal("htmx body too small")
	}
}

func TestGuidesStayFreeOfHTMX(t *testing.T) {
	mux := newMux(memSource{})
	html := body(get(t, mux, "/guides/", false))
	if strings.Contains(html, "htmx") || strings.Contains(html, "rail-goal") {
		t.Fatal("guides picked up the path shell")
	}
	if strings.Count(html, "<script") != 3 {
		t.Fatalf("script count %d", strings.Count(html, "<script"))
	}
}
