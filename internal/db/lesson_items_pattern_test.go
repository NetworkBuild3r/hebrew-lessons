package db

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-037 SPEC-003 / ADR-0032 D2: pattern lines keep the wrong side off lesson_items.

func TestDeriveLessonItemsPatternSkipsWrongSide(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	const (
		shloshah = "שְׁלוֹשָׁה"
		yamim    = "יָמִים"
		shalosh  = "שָׁלוֹש"
	)
	shloshahID := insertLexeme(t, s, shloshah)
	yamimID := insertLexeme(t, s, yamim)
	_ = insertLexeme(t, s, shalosh)

	doc := units.Unit{
		Title: "Days", Situation: "home", Concept: "numbers", Position: 1,
		Lessons: []units.Lesson{{
			Kind: "lesson",
			Activities: []units.Activity{{
				Type: units.ActivityPattern,
				Text: shloshah + " " + yamim + " — " + shalosh + " " + yamim,
			}},
		}},
	}
	put, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	assertLessonItems(t, s, put.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shloshahID, role: "recycle", skill: units.ActivityPattern},
		{lesson: 0, lexemeID: yamimID, role: "recycle", skill: units.ActivityPattern},
	})
}

func TestDeriveLessonItemsPatternWithoutSepIsWhole(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	const (
		shloshah = "שְׁלוֹשָׁה"
		yamim    = "יָמִים"
	)
	shloshahID := insertLexeme(t, s, shloshah)
	yamimID := insertLexeme(t, s, yamim)

	doc := units.Unit{
		Title: "Days", Situation: "home", Concept: "numbers", Position: 1,
		Lessons: []units.Lesson{{
			Kind: "lesson",
			Activities: []units.Activity{{
				Type: units.ActivityPattern,
				Text: shloshah + " " + yamim,
			}},
		}},
	}
	put, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	assertLessonItems(t, s, put.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shloshahID, role: "recycle", skill: units.ActivityPattern},
		{lesson: 0, lexemeID: yamimID, role: "recycle", skill: units.ActivityPattern},
	})
}

func TestDeriveLessonItemsDialogueStaysWhole(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	const (
		right = "לְהָכִין"
		wrong = "לַעֲשׂוֹת"
	)
	rightID := insertLexeme(t, s, right)
	wrongID := insertLexeme(t, s, wrong)

	// A non-pattern activity with an em dash still tokenizes both sides.
	doc := units.Unit{
		Title: "Talk", Situation: "kitchen", Concept: "cook", Position: 1,
		Lessons: []units.Lesson{{
			Kind: "lesson",
			Activities: []units.Activity{{
				Type: units.ActivityDialogue,
				Lines: []units.DialogueLine{
					{Speaker: "a", Text: right + " — " + wrong},
				},
			}},
		}},
	}
	put, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	assertLessonItems(t, s, put.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: rightID, role: "recycle", skill: units.ActivityDialogue},
		{lesson: 0, lexemeID: wrongID, role: "recycle", skill: units.ActivityDialogue},
	})
}

func TestDeriveLessonItemsWrongSideNewWordLosesNewRole(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	const (
		right = "לְהָכִין"
		wrong = "לַעֲשׂוֹת"
	)
	rightID := insertLexeme(t, s, right)
	_ = insertLexeme(t, s, wrong)

	doc := units.Unit{
		Title: "Steak", Situation: "kitchen", Concept: "cook", Position: 1,
		NewWords: []string{wrong},
		Lessons: []units.Lesson{{
			Kind: "lesson",
			Activities: []units.Activity{{
				Type: units.ActivityPattern,
				Text: right + " — " + wrong,
			}},
		}},
	}
	put, err := s.PutUnit(ctx, nil, doc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	// Wrong-side-only newWord never enters lesson_items, so it has no new role.
	assertLessonItems(t, s, put.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: rightID, role: "recycle", skill: units.ActivityPattern},
	})
}

func TestRebuildAllLessonItemsUnit12Fixture(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	const (
		lehachin = "לְהָכִין"
		oseh     = "עוֹשֶׂה"
		laasot   = "לַעֲשׂוֹת"
		steak    = "סְטֵייק"
		ani      = "אֲנִי"
	)
	lehachinID := insertLexeme(t, s, lehachin)
	osehID := insertLexeme(t, s, oseh)
	laasotID := insertLexeme(t, s, laasot)
	steakID := insertLexeme(t, s, steak)
	aniID := insertLexeme(t, s, ani)

	// Modeled on unit 12's steak contrast: right forms stay, לַעֲשׂוֹת is wrong-only.
	patternDoc := units.Unit{
		Title: "Unit 12 fixture", Situation: "kitchen", Concept: "cook", Position: 11,
		Lessons: []units.Lesson{
			{
				Kind: "lesson",
				Activities: []units.Activity{
					{Type: units.ActivityPattern, Text: lehachin + " " + steak + " — " + laasot + " " + steak},
					{Type: units.ActivityPattern, Text: ani + " " + oseh + " — " + ani + " " + laasot},
					{Type: "cloze", Text: ani + " " + lehachin, Translation: "I prepare"},
				},
			},
		},
	}
	dialogueDoc := units.Unit{
		Title: "Dialogue", Situation: "street", Concept: "greet", Position: 2,
		Lessons: []units.Lesson{{
			Kind: "lesson",
			Activities: []units.Activity{{
				Type: units.ActivityDialogue,
				Lines: []units.DialogueLine{
					{Speaker: "a", Text: ani},
					{Speaker: "b", Text: oseh},
				},
			}},
		}},
	}

	pattern, err := s.PutUnit(ctx, nil, patternDoc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.conn.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, pattern.UnitID); err != nil {
		t.Fatal(err)
	}
	dialogue, err := s.PutUnit(ctx, nil, dialogueDoc, "learner:1", "admin-1")
	if err != nil {
		t.Fatal(err)
	}

	// Stale rows as if derived before D2: wrong-side לַעֲשׂוֹת is in the list.
	mustExec(t, s, `
		INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
		VALUES (?, 0, ?, 'recycle', 'pattern')`, pattern.UnitID, laasotID)

	before := snapshotAllLessonItems(t, s)
	if !lessonItemSnapshotHas(before, pattern.UnitID, laasotID) {
		t.Fatal("fixture setup: want stale לַעֲשׂוֹת row before rebuild")
	}

	rebuilt, err := s.RebuildAllLessonItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []int64{pattern.UnitID, dialogue.UnitID}
	slices.Sort(wantIDs)
	gotIDs := append([]int64(nil), rebuilt...)
	slices.Sort(gotIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("rebuilt %v, want %v", gotIDs, wantIDs)
	}

	after := snapshotAllLessonItems(t, s)
	if lessonItemSnapshotHas(after, pattern.UnitID, laasotID) {
		t.Fatal("after rebuild: לַעֲשׂוֹת must not be in lesson_items")
	}
	for _, id := range []int64{lehachinID, osehID, steakID, aniID} {
		if !lessonItemSnapshotHas(after, pattern.UnitID, id) {
			t.Fatalf("after rebuild: right-side lexeme %d missing from unit %d", id, pattern.UnitID)
		}
	}
	removed := lessonItemSnapshotDiff(before, after)
	for _, row := range removed {
		if row.unitID != pattern.UnitID || row.lexemeID != laasotID {
			t.Fatalf("unexpected removal %+v; only wrong-side לַעֲשׂוֹת should leave", row)
		}
	}
	if added := lessonItemSnapshotDiff(after, before); len(added) != 0 {
		t.Fatalf("unexpected additions %+v", added)
	}

	secondBefore := after
	if _, err := s.RebuildAllLessonItems(ctx); err != nil {
		t.Fatal(err)
	}
	secondAfter := snapshotAllLessonItems(t, s)
	if diff := lessonItemSnapshotDiff(secondBefore, secondAfter); len(diff) != 0 {
		t.Fatalf("second rebuild changed rows %+v", diff)
	}
	if diff := lessonItemSnapshotDiff(secondAfter, secondBefore); len(diff) != 0 {
		t.Fatalf("second rebuild added rows %+v", diff)
	}
}

type lessonItemSnap struct {
	unitID   int64
	lesson   int
	lexemeID int64
	role     string
	skill    string
}

func snapshotAllLessonItems(t *testing.T, s *Store) []lessonItemSnap {
	t.Helper()
	rows, err := s.conn.QueryContext(context.Background(), `
		SELECT unit_id, lesson_ix, lexeme_id, role, skill
		FROM lesson_items
		ORDER BY unit_id, lesson_ix, lexeme_id, role, skill`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []lessonItemSnap
	for rows.Next() {
		var row lessonItemSnap
		if err := rows.Scan(&row.unitID, &row.lesson, &row.lexemeID, &row.role, &row.skill); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func lessonItemSnapshotHas(rows []lessonItemSnap, unitID, lexemeID int64) bool {
	for _, row := range rows {
		if row.unitID == unitID && row.lexemeID == lexemeID {
			return true
		}
	}
	return false
}

// lessonItemSnapshotDiff returns rows in a that are not in b.
func lessonItemSnapshotDiff(a, b []lessonItemSnap) []lessonItemSnap {
	inB := map[string]bool{}
	for _, row := range b {
		inB[lessonItemSnapKey(row)] = true
	}
	var out []lessonItemSnap
	for _, row := range a {
		if !inB[lessonItemSnapKey(row)] {
			out = append(out, row)
		}
	}
	return out
}

func lessonItemSnapKey(row lessonItemSnap) string {
	return fmt.Sprintf("%d/%d/%d/%s/%s", row.unitID, row.lesson, row.lexemeID, row.role, row.skill)
}
