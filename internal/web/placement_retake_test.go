package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// TestRetakePlacement is two finished runs: an earlier placement and a later
// retake, with different levels and finish times. The page, Today,
// LatestPlacement, and the deck's Latest all name the later one. Posting
// /_hx/placement/start opens another run and leaves the finished rows as they were.
func TestRetakePlacement(t *testing.T) {
	const (
		firstLevel  = 2
		retakeLevel = 4
		firstWords  = 20
		retakeWords = 50
	)
	firstAt := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	retakeAt := time.Date(2026, 10, 2, 15, 30, 0, 0, time.UTC)
	wantAt := db.FormatTime(retakeAt)

	h, store, user, _ := newPlacementRouter(t)
	seedProgressSnapshots(t, store, user.id, 1, progEst)
	ctx := context.Background()
	firstID := finishV3(t, store, user.id, firstAt, firstLevel, firstWords)
	retakeID := finishV3(t, store, user.id, retakeAt, retakeLevel, retakeWords)

	body := getPlaced(t, h, user, "/progress")
	card := section(t, body, "c-card pplace", "")
	if !strings.Contains(card, `href="/placement"`) || !strings.Contains(card, "Retake placement") {
		t.Fatalf("placement card missing the quiet retake link:\n%s", card)
	}
	if strings.Contains(card, `action="/_hx/placement/start"`) || strings.Contains(card, "btn-primary") {
		t.Fatalf("placement card still has the old retake button:\n%s", card)
	}
	items := strings.Split(card, `<li class="pplace__row">`)[1:]
	if len(items) < 2 {
		t.Fatalf("%d placement rows, want the two finished runs:\n%s", len(items), card)
	}
	if !strings.Contains(items[0], "Oct 2") || !strings.Contains(items[0], "Latest") || strings.Contains(items[0], "Aug 2") {
		t.Fatalf("newest placement row:\n%s", items[0])
	}
	if !strings.Contains(items[1], "Aug 2") || strings.Contains(items[1], "Latest") {
		t.Fatalf("older placement row:\n%s", items[1])
	}

	today := getPlaced(t, h, user, "/")
	if !strings.Contains(today, `data-placement="finished"`) ||
		!strings.Contains(today, `data-placement-level="`+strconv.Itoa(retakeLevel)+`"`) ||
		!strings.Contains(today, `data-placement-at="`+wantAt+`"`) ||
		strings.Contains(today, `data-placement-level="`+strconv.Itoa(firstLevel)+`"`) {
		t.Fatalf("Today placement, want level %d at %s:\n%s", retakeLevel, wantAt, today)
	}

	latest, ok, err := store.LatestPlacement(ctx, user.id)
	if err != nil || !ok || !latest.Finished || latest.Level != retakeLevel || db.FormatTime(latest.TakenAt) != wantAt {
		t.Fatalf("LatestPlacement = finished %v level %d at %s, want %d at %s (%v)", latest.Finished, latest.Level, db.FormatTime(latest.TakenAt), retakeLevel, wantAt, err)
	}
	eng := placement.NewEngine(store, evidence.New(store, fsrs.New(store, fixedClock(retakeAt)), fixedClock(retakeAt)), fixedClock(retakeAt))
	deckLatest, ok, err := eng.Latest(ctx, user.id)
	if err != nil || !ok || deckLatest.RunID != retakeID || db.FormatTime(deckLatest.FinishedAt) != wantAt {
		t.Fatalf("deck.Latest run %d at %s, want %d at %s (%v)", deckLatest.RunID, db.FormatTime(deckLatest.FinishedAt), retakeID, wantAt, err)
	}
	if deckLatest.RunID == firstID {
		t.Fatal("deck.Latest returned the older run")
	}

	beforeFirst := finishedRunText(t, store, firstID)
	beforeRetake := finishedRunText(t, store, retakeID)
	rec := user.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {user.csrf}}, user.csrf, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="placement"`) {
		t.Fatalf("start = %d\n%s", rec.Code, rec.Body.String())
	}
	if got := finishedRunText(t, store, firstID); got != beforeFirst {
		t.Fatalf("older finished row changed:\n%s\n%s", beforeFirst, got)
	}
	if got := finishedRunText(t, store, retakeID); got != beforeRetake {
		t.Fatalf("retake finished row changed:\n%s\n%s", beforeRetake, got)
	}
	open, found, err := store.OpenPlacementRun(ctx, user.id)
	if err != nil || !found || open.ID == firstID || open.ID == retakeID || !open.FinishedAt.IsZero() {
		t.Fatalf("open run after retake = found %v id %d finished %v (%v)", found, open.ID, !open.FinishedAt.IsZero(), err)
	}
	// The open run is the latest run. A sort by finish time would still return the retake.
	opened, ok, err := store.LatestPlacement(ctx, user.id)
	if err != nil || !ok || opened.Finished || opened.Level != -1 {
		t.Fatalf("LatestPlacement after start = finished %v level %d (%v)", opened.Finished, opened.Level, err)
	}
	still, ok, err := eng.Latest(ctx, user.id)
	if err != nil || !ok || still.RunID != retakeID {
		t.Fatalf("deck.Latest after start = run %d, want the finished retake %d (%v)", still.RunID, retakeID, err)
	}
}

func finishV3(t *testing.T, store *db.Store, userID int64, at time.Time, level, words int) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := store.StartPlacementV3(ctx, userID, at, 1, "initial", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"estimated_known_words":` + strconv.Itoa(words) + `,"listening":{"tested":true,"level":` + strconv.Itoa(level) + `,"estimate":` + strconv.Itoa(words) + `},"reading":{"tested":false,"level":0,"estimate":0}}`
	if err := store.FinishPlacement(ctx, id, at, level, "exhausted", body); err != nil {
		t.Fatal(err)
	}
	return id
}

func getPlaced(t *testing.T, h http.Handler, user placedUser, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(user.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func finishedRunText(t *testing.T, store *db.Store, id int64) string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var run, side string
	var items int
	err = conn.QueryRowContext(ctx, `
		SELECT CAST(id AS TEXT) || '|' || CAST(user_id AS TEXT) || '|' || started_at || '|' ||
			COALESCE(finished_at, '') || '|' || COALESCE(CAST(level AS TEXT), '') || '|' ||
			COALESCE(stop_reason, '') || '|' || COALESCE(result, '') || '|' ||
			CAST(seed AS TEXT) || '|' || CAST(sitting_no AS TEXT)
		FROM placement_runs WHERE id = ?`, id).Scan(&run)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.QueryRowContext(ctx, `
		SELECT CAST(run_id AS TEXT) || '|' || kind || '|' || COALESCE(recheck_skill, '') || '|' ||
			COALESCE(CAST(recheck_band AS TEXT), '') || '|' || phase
		FROM placement_v3_runs WHERE run_id = ?`, id).Scan(&side)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_items WHERE run_id = ?`, id).Scan(&items); err != nil {
		t.Fatal(err)
	}
	return run + "\n" + side + "\n" + strconv.Itoa(items)
}
