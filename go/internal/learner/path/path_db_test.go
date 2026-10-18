package path

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

func TestPathFromDatabase(t *testing.T) {
	ctx := context.Background()
	_, ids, conn := seedFixture(t, map[string]any{
		"users": []string{
			"pathlearner", "pathshared", "pathother1", "pathnobody1",
			"pathcorrupt", "pathdraft01", "pathrounds1",
		},
		"reviews": []map[string]any{{
			"username":  "pathlearner",
			"word":      "שלום",
			"direction": "he_en",
			"due":       "2026-10-18T12:00:00.000Z",
			"suspended": 0,
		}, {
			"username":  "pathlearner",
			"word":      "שלום",
			"direction": "en_he",
			"due":       "2026-10-19T12:00:00.000Z",
			"suspended": 0,
		}},
		"progress": []map[string]any{
			{"username": "pathlearner", "slug": "my-morning", "completedRounds": 1},
			{"username": "pathrounds1", "slug": "half", "completedRounds": 0.5},
			{"username": "pathrounds1", "slug": "text", "completedRounds": "2"},
			{"username": "pathrounds1", "slug": "zero", "completedRounds": 0},
			{"username": "pathrounds1", "slug": "none", "completedRounds": nil},
		},
		"grants": []map[string]any{{
			"username": "pathshared",
			"slug":     "granted-lesson",
		}},
		"plans": []map[string]any{
			personal("plan-z", "pathlearner", unit("decoy", []map[string]any{
				lesson("decoy", "everyone", nil),
			}, nil)),
			personal("plan-a", "pathlearner",
				unit("u1", []map[string]any{
					lesson("morning", "everyone", map[string]any{
						"legacyStorageKey": "my-morning",
						"newWords":         []string{"שלום"},
						"blocks":           []any{map[string]string{"type": "spaced-review-checkpoint"}},
					}),
					lesson("later", "everyone", map[string]any{"newWords": []string{"בוקר"}}),
				}, nil),
				unit("u2", []map[string]any{
					lesson("nextone", "everyone", map[string]any{"newWords": []string{"עוד"}}),
				}, map[string]any{"prerequisites": []string{"u1"}}),
			),
			shared("plan-shared", unit("s", []map[string]any{
				lesson("open-lesson", "everyone", nil),
			}, nil)),
			shared("plan-hidden", unit("h", []map[string]any{
				lesson("secret", "hidden", nil),
			}, nil)),
			shared("plan-select", unit("g", []map[string]any{
				lesson("granted-lesson", "selected", nil),
			}, nil)),
			shared("plan-nogrant", unit("n", []map[string]any{
				lesson("closed-lesson", "selected", nil),
			}, nil)),
			personal("plan-other", "pathother1", unit("o", []map[string]any{
				lesson("other-lesson", "everyone", nil),
			}, nil)),
			{
				"id":           "plan-aaa",
				"audience":     "personal",
				"username":     "pathcorrupt",
				"status":       "published",
				"snapshotText": "{",
			},
			personal("plan-bbb", "pathcorrupt", unit("ok", []map[string]any{
				lesson("ok-lesson", "everyone", nil),
			}, nil)),
			{
				"id":       "plan-draft",
				"audience": "personal",
				"username": "pathdraft01",
				"status":   "draft",
				"snapshot": map[string]any{
					"units": []any{unit("d", []map[string]any{lesson("draft-lesson", "everyone", nil)}, nil)},
				},
			},
			personal("plan-rounds", "pathrounds1", unit("r", []map[string]any{
				lesson("half", "everyone", nil),
				lesson("text", "everyone", nil),
				lesson("zero", "everyone", nil),
				lesson("none", "everyone", nil),
			}, nil)),
		},
	})

	cfg := testConfig(t)
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	svc := New(conn, cfg).WithLocation(time.UTC).WithClock(func() time.Time { return now })
	learner := ids["pathlearner"]

	got, err := svc.Path(ctx, learner, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanID != "plan-a" || got.Title == "" {
		t.Fatalf("assigned %+v", got)
	}
	due, err := svc.DueCount(ctx, learner)
	if err != nil {
		t.Fatal(err)
	}
	if got.DueCount != due || due != 1 {
		t.Fatalf("due path %d count %d", got.DueCount, due)
	}
	if len(got.Units) != 2 || got.Units[0].ID != "u1" || !got.Units[0].Passed || got.Units[0].State != StateDone {
		t.Fatalf("u1 %+v", got.Units[0])
	}
	if !got.Units[0].Lessons[0].Done || got.Units[0].Lessons[1].State != StateNext {
		t.Fatalf("u1 lessons %+v", got.Units[0].Lessons)
	}
	if got.Units[0].NewWordsIntroduced != 2 || got.Units[0].NewWordsDone != 1 {
		t.Fatalf("words %d/%d", got.Units[0].NewWordsDone, got.Units[0].NewWordsIntroduced)
	}
	if got.Units[1].NotYet || got.Units[1].Lessons[0].State != StateCurrent {
		t.Fatalf("u2 %+v", got.Units[1])
	}
	for _, node := range got.Nodes {
		if node.Kind == KindReviewStop {
			t.Fatal("stop below the configured due count")
		}
		if node.LessonID == "decoy" {
			t.Fatal("higher plan id was selected")
		}
	}

	low := cfg
	low.ReviewStopDueCount = 1
	stopped, err := New(conn, low).WithLocation(time.UTC).WithClock(func() time.Time { return now }).Path(ctx, learner, "plan-a")
	if err != nil {
		t.Fatal(err)
	}
	var stops int
	for _, node := range stopped.Nodes {
		if node.Kind == KindReviewStop {
			stops++
			if node.AfterLessonID != "nextone" {
				t.Fatalf("stop %+v", node)
			}
		}
	}
	if stops != 1 {
		t.Fatalf("stops %d", stops)
	}

	sharedID := ids["pathshared"]
	if _, err := svc.Path(ctx, sharedID, ""); !errors.Is(err, ErrNoPlan) {
		t.Fatalf("shared fallback: %v", err)
	}
	open, err := svc.Path(ctx, sharedID, "plan-shared")
	if err != nil || open.Units[0].Lessons[0].State != StateCurrent {
		t.Fatalf("shared %+v err %v", open, err)
	}
	if _, err := svc.Path(ctx, sharedID, "plan-hidden"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden: %v", err)
	}
	granted, err := svc.Path(ctx, sharedID, "plan-select")
	if err != nil || granted.PlanID != "plan-select" {
		t.Fatalf("grant %+v err %v", granted, err)
	}
	if _, err := svc.Path(ctx, sharedID, "plan-nogrant"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no grant: %v", err)
	}
	if _, err := svc.Path(ctx, learner, "plan-other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other assignee: %v", err)
	}
	if _, err := svc.Path(ctx, ids["pathnobody1"], ""); !errors.Is(err, ErrNoPlan) {
		t.Fatalf("nobody: %v", err)
	}
	if _, err := svc.Path(ctx, 1_000_000_000, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	if _, err := svc.Path(ctx, 0, ""); !errors.Is(err, ErrBadUser) {
		t.Fatalf("bad user: %v", err)
	}
	if _, err := svc.Path(ctx, ids["pathcorrupt"], ""); err == nil || errors.Is(err, ErrNoPlan) || errors.Is(err, ErrNotFound) {
		t.Fatalf("corrupt snapshot: %v", err)
	}
	if _, err := svc.Path(ctx, ids["pathdraft01"], ""); !errors.Is(err, ErrNoPlan) {
		t.Fatalf("draft: %v", err)
	}

	rounds, err := svc.Path(ctx, ids["pathrounds1"], "plan-rounds")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]State{}
	for _, lesson := range rounds.Units[0].Lessons {
		states[lesson.ID] = lesson.State
	}
	if states["half"] != StateDone || states["text"] != StateCurrent || states["zero"] != StateNext || states["none"] != StateNext {
		t.Fatalf("rounds %+v", states)
	}

	ledger, err := db.CheckLedger(ctx, conn)
	if err != nil || !ledger.OK || len(ledger.Versions) != 11 {
		t.Fatalf("ledger %+v err %v", ledger, err)
	}
	for i, row := range ledger.Versions {
		want := db.ExpectedMigrations[i]
		if row.Version != want.Version || row.Name != want.Name {
			t.Fatalf("migration %d %+v", i, row)
		}
	}
	var tables int
	if err := conn.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'unit_progress'`,
	).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("unit_progress exists")
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('go-write', 'no')`); err == nil {
		t.Fatal("read-only connection wrote a row")
	}

	mux := http.NewServeMux()
	MountDev(mux, svc, true)
	dueRec := httptest.NewRecorder()
	mux.ServeHTTP(dueRec, httptest.NewRequest(http.MethodGet, "/api/dev/learner/due?user="+strconv.FormatInt(learner, 10), nil))
	if dueRec.Code != http.StatusOK || !strings.Contains(dueRec.Body.String(), `"dueCount":1`) {
		t.Fatalf("dev due %d %s", dueRec.Code, dueRec.Body.String())
	}
	pathRec := httptest.NewRecorder()
	mux.ServeHTTP(pathRec, httptest.NewRequest(http.MethodGet, "/api/dev/learner/path?user="+strconv.FormatInt(learner, 10), nil))
	if pathRec.Code != http.StatusOK || !strings.Contains(pathRec.Body.String(), `"planId":"plan-a"`) {
		t.Fatalf("dev path %d %s", pathRec.Code, pathRec.Body.String())
	}
	badRec := httptest.NewRecorder()
	mux.ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/api/dev/learner/due?user=0", nil))
	if badRec.Code != http.StatusBadRequest || !strings.Contains(badRec.Body.String(), `"error":"bad_request"`) {
		t.Fatalf("bad %d %s", badRec.Code, badRec.Body.String())
	}
	missRec := httptest.NewRecorder()
	mux.ServeHTTP(missRec, httptest.NewRequest(http.MethodGet, "/api/dev/learner/path?user="+strconv.FormatInt(ids["pathnobody1"], 10), nil))
	if missRec.Code != http.StatusNotFound || !strings.Contains(missRec.Body.String(), `"error":"not_found"`) {
		t.Fatalf("missing %d %s", missRec.Code, missRec.Body.String())
	}
	off := http.NewServeMux()
	MountDev(off, svc, false)
	offRec := httptest.NewRecorder()
	off.ServeHTTP(offRec, httptest.NewRequest(http.MethodGet, "/api/dev/learner/due?user=1", nil))
	if offRec.Code != http.StatusNotFound {
		t.Fatalf("flag off %d", offRec.Code)
	}
}

func TestServiceSourcesDoNotWrite(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(text), ".Exec(") || strings.Contains(string(text), ".ExecContext(") {
			return errors.New(path + " writes through database/sql")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func personal(id, username string, units ...map[string]any) map[string]any {
	return map[string]any{
		"id":       id,
		"audience": "personal",
		"username": username,
		"status":   "published",
		"snapshot": map[string]any{"title": id, "units": units},
	}
}

func shared(id string, units ...map[string]any) map[string]any {
	return map[string]any{
		"id":       id,
		"audience": "shared",
		"status":   "published",
		"snapshot": map[string]any{"title": id, "units": units},
	}
}

func unit(id string, lessons []map[string]any, extra map[string]any) map[string]any {
	item := map[string]any{
		"id":      id,
		"title":   id,
		"theme":   "theme",
		"lessons": lessons,
	}
	for key, value := range extra {
		item[key] = value
	}
	return item
}

func lesson(id, visibility string, extra map[string]any) map[string]any {
	item := map[string]any{
		"id":         id,
		"title":      id,
		"visibility": visibility,
		"newWords":   []string{},
	}
	for key, value := range extra {
		item[key] = value
	}
	return item
}
