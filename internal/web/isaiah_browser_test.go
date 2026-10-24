//go:build browser

package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestIsaiahInBrowser(t *testing.T) {
	dir := os.Getenv("MOTION_PW_DIR")
	if dir == "" {
		t.Skip("set MOTION_PW_DIR to a directory with Playwright installed")
	}
	origin, cookie := startIsaiahServer(t)
	shots := t.TempDir()
	out := runNode(t, dir, "isaiah-browser.mjs", "ORIGIN="+origin, "SESSION="+cookie, "SHOTS="+shots)
	assertBrowserChecks(t, out, 8)
	for _, name := range []string{
		"isaiah-390-light.png",
		"isaiah-1440-dark.png",
	} {
		info, err := os.Stat(filepath.Join(shots, name))
		if err != nil || info.Size() == 0 {
			t.Errorf("screenshot %s: %v", name, err)
		}
	}
}

func startIsaiahServer(t *testing.T) (origin, cookie string) {
	t.Helper()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()
	srv := httptest.NewUnstartedServer(nil)
	origin = "http://" + srv.Listener.Addr().String()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	word := units.Normalize("\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1")
	if _, err := store.ExecContext(ctx, `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES ('isaiah', 6, 1, ?, ?, 'Holy, holy, holy.', 'test', 'Public Domain')`,
		word, units.Plain(word)); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertVerseNotes(ctx, "isaiah", 6, 1, []db.VerseNoteInput{
		{Layer: db.LayerRoots, Body: "The root is set apart."},
		{Layer: db.LayerManner, Body: "DRAFTNOTE stays hidden."},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertVerseGlosses(ctx, "isaiah", 6, 1, []db.VerseGlossInput{
		{Position: 0, WordHe: word, GlossEn: "holy", Root: units.Normalize("\u05e7\u05d3\u05e9")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckVerseNote(ctx, "isaiah", 6, 1, db.LayerRoots, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckVerseGloss(ctx, "isaiah", 6, 1, 0); err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: origin, Clock: clk, Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = NewOptions(nil, svc, Options{Store: store})
	srv.Start()
	t.Cleanup(srv.Close)
	cookie = mintSession(t, store, user.ID, "brian-csrf", now.Add(time.Hour)).Value
	req, err := http.NewRequest(http.MethodGet, origin+"/isaiah/6/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "hebrew_session", Value: cookie})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "isaiah-he") {
		t.Fatalf("reader %d\n%s", res.StatusCode, body)
	}
	return origin, cookie
}
