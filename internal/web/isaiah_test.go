package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestIsaiahCheckIsAnAdminAct(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "admin@example.com"},
		AdminEmails:   []string{"admin@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	admin := addLearner(t, store, "admin@example.com", "sub-admin", now)
	ctx := context.Background()
	word := units.Normalize("\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1")
	root := units.Normalize("\u05e7\u05d3\u05e9")
	if _, err := store.ExecContext(ctx, `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES ('isaiah', 6, 1, ?, ?, 'Holy.', 'test', 'Public Domain')`, word, units.Plain(word)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, 'holy', 'adj', ?)`, word, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, 'holiness', 'noun', ?)`, units.Normalize("\u05e7\u05b9\u05d3\u05b6\u05e9\u05c1"), root); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertVerseNotes(ctx, "isaiah", 6, 1, []db.VerseNoteInput{
		{Layer: db.LayerRoots, Body: "The root is set apart."},
		{Layer: db.LayerManner, Body: "DRAFTNOTE stays hidden."},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertVerseGlosses(ctx, "isaiah", 6, 1, []db.VerseGlossInput{
		{Position: 0, WordHe: word, GlossEn: "holy", Root: root},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckVerseNote(ctx, "isaiah", 6, 1, db.LayerRoots, admin.user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckVerseGloss(ctx, "isaiah", 6, 1, 0); err != nil {
		t.Fatal(err)
	}

	page := getLearner(t, h, learner, "/isaiah/6/1")
	if strings.Contains(page, "DRAFTNOTE") || !strings.Contains(page, "The root is set apart.") {
		t.Fatal("a draft reached the reader")
	}
	if !strings.Contains(page, "holy") || !strings.Contains(page, `href="/roots/`) {
		t.Fatalf("the word gloss or its root link is missing:\n%s", page)
	}
	if !strings.Contains(page, `class="isaiah-he"`) || !strings.Contains(page, `dir="rtl"`) {
		t.Fatal("the pointed line is missing")
	}
	if !strings.Contains(page, `class="isaiah-en"`) || !strings.Contains(page, "Show the English") || !strings.Contains(page, "Holy.") {
		t.Fatal("the English toggle is missing")
	}

	if code := postIsaiahCheck(t, h, learner, "/admin/isaiah/6/1"); code != http.StatusForbidden {
		t.Fatalf("learner check = %d", code)
	}
	if code := postIsaiahCheck(t, h, admin, "/admin/isaiah/6/1"); code != http.StatusSeeOther {
		t.Fatalf("admin check = %d", code)
	}
	notes, err := store.VerseNotes(ctx, "isaiah", 6, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, note := range notes {
		if note.Layer == db.LayerManner && (note.Status != "checked" || note.CheckedBy != admin.user.ID) {
			t.Fatalf("checker was not recorded: %+v", note)
		}
	}
}

func postIsaiahCheck(t *testing.T, h http.Handler, who learnerFix, path string) int {
	t.Helper()
	form := url.Values{}
	form.Set("_csrf", who.csrf)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", who.csrf)
	req.AddCookie(who.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}
