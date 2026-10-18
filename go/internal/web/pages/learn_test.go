package pages

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/config"
	lpath "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/learner/path"
)

func learnMux(svc *lpath.Service, gate lpath.GateAck, cfg config.Config) *http.ServeMux {
	mux := http.NewServeMux()
	NewLearning(svc, gate, cfg, lpath.DemoPlanID).Register(mux)
	return mux
}

func demoMux(t *testing.T) *http.ServeMux {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return learnMux(lpath.New(nil, cfg), lpath.Unavailable{}, cfg)
}

func TestPathAndTodayRender(t *testing.T) {
	mux := demoMux(t)
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
	if strings.Contains(pathHTML, "<script src=") == false || !strings.Contains(pathHTML, "/shared/vendor/htmx.min.js") {
		t.Fatal("path page did not load htmx")
	}
	if !strings.Contains(pathHTML, "showModal()") {
		t.Fatal("gate host does not open a modal dialog")
	}
	if strings.Contains(pathHTML, "of 12") {
		t.Fatal("unit count is hard-coded")
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
	cfg := config.Config{
		CheckpointPassMark:   "0.50",
		ReviewStopDueCount:   config.DefaultReviewStopDueCount,
		ReviewStopGapLessons: config.DefaultReviewStopGapLessons,
		LessonWarmupDue:      config.DefaultLessonWarmupDue,
	}
	mux := learnMux(lpath.New(nil, cfg), lpath.Unavailable{}, cfg)
	html := body(get(t, mux, "/lessons/", false))
	if strings.Contains(html, `data-pass-needed="16"`) || !strings.Contains(html, `data-pass-needed="10"`) {
		t.Fatal("pass_needed stayed at the example 16")
	}
	ok, err := cfg.MeetsPassMark(10, 20)
	if err != nil || !ok {
		t.Fatal("10 of 20 should meet 0.50")
	}
}

func TestDueZeroHidesCardAndStop(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	course := lpath.DemoCourse(cfg, 0)
	course.Units = course.Units[:len(course.Units)-1]
	svc := lpath.New(nil, cfg)
	svc.UseFixture(course, 0)
	mux := learnMux(svc, lpath.Unavailable{}, cfg)
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
	gate := lpath.Unavailable{}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mux := learnMux(lpath.New(nil, cfg), gate, cfg)
	page := get(t, mux, "/lessons/", false)
	tok := csrfToken(t, page)
	cookie := page.Result().Cookies()[0]

	ok := postGate(t, mux, cookie, tok, "http://example.com", "/worksheets/coffee/", "/worksheets/cafe-checkpoint/")
	if ok.Code != http.StatusOK {
		t.Fatalf("persisted failure status %d", ok.Code)
	}
	sheet := body(ok)
	if !strings.Contains(sheet, "<dialog") || strings.Contains(sheet, "<dialog open") {
		t.Fatal("warning was not a closed dialog for showModal")
	}
	if !strings.Contains(sheet, `hx-swap-oob="outerHTML"`) || !strings.Contains(sheet, `data-gate="open"`) {
		t.Fatal("gate-ack did not open the unit's links")
	}
	if strings.Contains(sheet, "gate-scrim") {
		t.Fatal("scrim wrapper is still in the response")
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
	seen, seenErr := gate.GateWarningSeen(context.Background(), lpath.DevUserID, lpath.DemoPlanID, "unit-3")
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
	proto := postGate(t, mux, cookie, tok, "http://example.com", "//evil.example/x", "/worksheets/cafe-checkpoint/")
	if proto.Code != http.StatusOK || strings.Contains(body(proto), "evil.example") {
		t.Fatal("protocol-relative redirect was kept")
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

func postConfirm(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, token, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/_hx/units/unit-3/gate-ack", strings.NewReader("confirm=1"))
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
	ver, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "shared", "vendor", "htmx.version"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "2.0.10" {
		t.Fatalf("htmx version %q", ver)
	}
	js, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "shared", "vendor", "htmx.min.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(js, []byte("2.0.10")) {
		t.Fatal("vendored htmx is not 2.0.10")
	}
}

func TestGateConfirmAndReload(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	gate := &lpath.MemoryGate{}
	mux := learnMux(lpath.New(nil, cfg), gate, cfg)
	page := get(t, mux, "/lessons/", false)
	tok := csrfToken(t, page)
	cookie := page.Result().Cookies()[0]

	opened := postGate(t, mux, cookie, tok, "http://example.com", "/worksheets/coffee/", "/worksheets/cafe-checkpoint/")
	if opened.Code != http.StatusOK {
		t.Fatalf("open status %d", opened.Code)
	}
	seen, seenErr := gate.GateWarningSeen(context.Background(), lpath.DevUserID, lpath.DemoPlanID, "unit-3")
	if seen || seenErr != nil {
		t.Fatal("opening the sheet stored the ack")
	}

	confirm := postConfirm(t, mux, cookie, tok, "http://example.com")
	if confirm.Code != http.StatusNoContent {
		t.Fatalf("confirm status %d", confirm.Code)
	}
	seen, seenErr = gate.GateWarningSeen(context.Background(), lpath.DevUserID, lpath.DemoPlanID, "unit-3")
	if !seen || seenErr != nil {
		t.Fatal("confirm did not store the ack")
	}
	html := body(get(t, mux, "/lessons/", false))
	if strings.Contains(html, `hx-post="/_hx/units/unit-3/gate-ack"`) {
		t.Fatal("a seen unit still posts the gate")
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
