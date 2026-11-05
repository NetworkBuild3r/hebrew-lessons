package mcp_test

// INIT-030/SPEC-005: revision brief build, race refuse, wrong unitId, backlog on SQLite.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// reviseDoc is apDoc with story padding so unit.drill_share stays under 40%.
// INIT-030/SPEC-004 made drill_share a brief block. INIT-030/SPEC-005.
func reviseDoc(title string) units.Unit {
	doc := apDoc(title)
	story := apShalom + " " + apBoker + " " + apToda
	for i := range doc.Lessons {
		if doc.Lessons[i].Kind == units.KindCheckpoint {
			continue
		}
		for k := 0; k < 8; k++ {
			doc.Lessons[i].Activities = append(doc.Lessons[i].Activities, units.Activity{
				Type: units.ActivityStory, Text: story,
			})
		}
	}
	return withSentenceTranslations(doc)
}

// TestReviseStartedRaceRefuse covers a run inserted after the revision brief:
// put_unit refuses with brief.revise_started and leaves the published revision.
// Also a revision brief cannot publish a different unit. INIT-030/SPEC-005.
func TestReviseStartedRaceRefuse(t *testing.T) {
	e := newAP(t)
	doc := reviseDoc("Cafe")
	e.audio(doc)

	// Seed a published learner unit without a brief (PublishUnit skips brief checks).
	draft := e.put(db.LearnerScope(e.a), nil, doc, "", false)
	if _, err := e.store.PublishUnit(e.t.Context(), draft.UnitID, nil); err != nil {
		t.Fatal(err)
	}
	publishedRev := draft.Revision
	if e.status(draft.UnitID) != "published" {
		t.Fatalf("seed status %s", e.status(draft.UnitID))
	}

	reviseID := "CCCCCCCCCCCCCCCCCCCCCQ"
	e.revisionBrief(reviseID, e.a, draft.UnitID)

	// A run starts after the brief was built.
	e.exec(`INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`,
		e.a, draft.UnitID, db.FormatTime(apNow))

	edited := reviseDoc("Cafe revised")
	e.audio(edited)
	out := e.put(db.LearnerScope(e.a), &draft.UnitID, edited, reviseID, true)
	if out.Published || !apHas(out.Issues, mcp.RuleBriefReviseStarted, "/briefId") {
		t.Fatalf("want brief.revise_started, got published=%v issues=%v reason=%q", out.Published, out.Issues, out.PublishReason)
	}

	// No new revision may be the published one with the revised title.
	if e.status(draft.UnitID) == "published" {
		view, err := e.adapter.LoadUnit(e.t.Context(), draft.UnitID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if view.Doc.Title == "Cafe revised" {
			t.Fatalf("revised title was published: rev=%d", view.Revision)
		}
		if view.Revision != publishedRev {
			t.Fatalf("published rev changed to %d, want %d", view.Revision, publishedRev)
		}
	}

	// A revision brief for unit A cannot publish unit B.
	other := e.put(db.LearnerScope(e.a), nil, reviseDoc("Other"), "", false)
	wrong := "DDDDDDDDDDDDDDDDDDDDDQ"
	e.revisionBrief(wrong, e.a, draft.UnitID)
	stolen := e.put(db.LearnerScope(e.a), &other.UnitID, reviseDoc("Stolen"), wrong, true)
	if stolen.Published || !apHas(stolen.Issues, units.RuleBriefNotFound, "/briefId") {
		t.Fatalf("revision brief on other unit: %+v", stolen)
	}
}

func (e *apEnv) revisionBrief(id string, user, unitID int64) {
	e.t.Helper()
	word := func(text, gloss string) brief.Word {
		return brief.Word{LexemeID: e.ids[text], Text: text, Gloss: gloss}
	}
	uid := unitID
	plan := brief.Plan{
		V: brief.Version, Theme: "cafe", Tier: brief.TierFor(10),
		ReviseUnitID: &uid,
		NewWords: []brief.NewWord{
			{Word: word(apShalom, "hello"), MinUses: 3}, {Word: word(apBoker, "morning"), MinUses: 3}, {Word: word(apToda, "thanks"), MinUses: 3},
		},
		StruggleWords: []brief.StruggleWord{{
			Word: word(apMayim, "water"), State: "struggling", Skill: "listen", RequiredCount: 3, Activities: []string{"listen_pick"},
		}},
		ComfortableWords: []brief.Word{}, Activities: []string{}, Notes: []string{}, CoachNotes: []brief.CoachNote{},
		Targets: brief.Targets{Coverage: brief.CoverageTarget, NewWordMinUses: brief.NewWordMinUses},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, 'cafe', ?, ?)`,
		id, user, string(raw), db.FormatTime(apNow.Add(-time.Hour)))
}

// TestBuildRevisionBriefSQL builds a real revision brief for an untouched learner unit.
// INIT-030/SPEC-005 AC: revision brief for an untouched unit builds.
func TestBuildRevisionBriefSQL(t *testing.T) {
	e := newAP(t)
	doc := reviseDoc("Cafe")
	e.audio(doc)
	draft := e.put(db.LearnerScope(e.a), nil, doc, "", false)
	if _, err := e.store.PublishUnit(e.t.Context(), draft.UnitID, nil); err != nil {
		t.Fatal(err)
	}

	raw := mustOK(t, e.session, "get_lesson_brief", map[string]any{"learnerId": e.a, "unitId": draft.UnitID})
	briefOut := decodeInto[mcp.LessonBrief](t, raw)
	if briefOut.BriefID == "" {
		t.Fatalf("empty revision brief: %s", raw)
	}
	payload := e.text(`SELECT payload_json FROM lesson_briefs WHERE id = ?`, briefOut.BriefID)
	if !strings.Contains(payload, fmt.Sprintf(`"reviseUnitId":%d`, draft.UnitID)) {
		t.Fatalf("payload lacks reviseUnitId: %s", payload)
	}

	e.exec(`INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`,
		e.a, draft.UnitID, db.FormatTime(apNow))
	fail := mustFail(t, e.session, "get_lesson_brief", map[string]any{"learnerId": e.a, "unitId": draft.UnitID})
	if !strings.Contains(strings.ToLower(fail), "started") {
		t.Fatalf("started refuse: %s", fail)
	}
}

// TestLessonBriefBacklogSQL covers the backlog cap on the real store.
// INIT-030/SPEC-005 AC: three untouched refuse; two allow.
func TestLessonBriefBacklogSQL(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), apClock{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	adapter := mcpstore.New(store)
	user, err := store.InsertUser(ctx, "backlog@example.com", "sub-backlog", "learner")
	if err != nil {
		t.Fatal(err)
	}
	e := &apEnv{t: t, store: store, adapter: adapter}
	e.session = e.connect(mcp.Options{
		AuthToken: "secret", Course: adapter, Lexicon: adapter, Lookup: adapter, Units: adapter,
	}, "secret")

	insertLearnerUnit := func(pos int) int64 {
		t.Helper()
		doc := units.Unit{Title: "U", Situation: "s", Concept: "c", Position: pos, Lessons: []units.Lesson{{Kind: units.KindLesson}}}
		saved, err := adapter.PutUnit(ctx, nil, &doc, db.LearnerScope(user))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PublishUnit(ctx, saved.UnitID, nil); err != nil {
			t.Fatal(err)
		}
		return saved.UnitID
	}
	u1 := insertLearnerUnit(0)
	_ = insertLearnerUnit(1)

	raw, isErr := toolJSON(t, e.session, "get_lesson_brief", map[string]any{"learnerId": user})
	if isErr && strings.Contains(raw, "untouched units") {
		t.Fatalf("two untouched refused: %s", raw)
	}

	_ = insertLearnerUnit(2)
	fail := mustFail(t, e.session, "get_lesson_brief", map[string]any{"learnerId": user})
	if !strings.Contains(fail, fmt.Sprintf("%d", u1)) || !strings.Contains(strings.ToLower(fail), "untouched") {
		t.Fatalf("three untouched must name %d: %s", u1, fail)
	}
}
