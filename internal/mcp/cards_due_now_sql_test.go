package mcp_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// Ten new cards and one learning card are due at the store clock. Six review
// cards are due later. Twenty-eight review_log rows from the same UTC day
// have elapsed_days 0, so model.dueReviews stays 0. cardsDueNow is the
// ListDueCards count, not that log.
func TestCardsDueNowMatchesListDueCards(t *testing.T) {
	e := newAP(t)
	skills := []string{"listen", "read", "recall", "spell"}
	var learningID int64
	for i := 0; i < 17; i++ {
		text := fmt.Sprintf("due-%02d", i)
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, 'g', 'noun')`, text, text)
		lex := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, text))
		state := "review"
		due := apNow.Add(time.Hour)
		if i < 10 {
			state = "new"
			due = apNow
		} else if i == 10 {
			state = "learning"
			due = apNow
		}
		e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
			VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0, 0)`,
			e.a, lex, skills[i%len(skills)], state, db.FormatTime(due))
		if i == 10 {
			learningID = int64(e.count(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, e.a, lex))
		}
	}
	for i := 0; i < 28; i++ {
		at := apNow.Add(-time.Duration(i) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 0)`,
			learningID, db.FormatTime(at))
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 28 {
		t.Fatalf("review_log rows %d", n)
	}

	listed, err := e.store.ListDueCards(t.Context(), e.a, apNow)
	if err != nil {
		t.Fatal(err)
	}
	raw := mustOK(t, e.session, "get_context", map[string]any{"position": 0, "learnerId": e.a})
	got := decodeInto[struct {
		CardsDueNow *struct {
			New        int `json:"new"`
			Learning   int `json:"learning"`
			Review     int `json:"review"`
			Relearning int `json:"relearning"`
			Total      int `json:"total"`
		} `json:"cardsDueNow"`
		Model *struct {
			DueReviews int `json:"dueReviews"`
			Skills     []struct {
				DueReviews int `json:"dueReviews"`
			} `json:"skills"`
		} `json:"model"`
	}](t, raw)
	if got.Model == nil || got.Model.DueReviews != 0 {
		t.Fatalf("dueReviews %+v", got.Model)
	}
	for _, skill := range got.Model.Skills {
		if skill.DueReviews != 0 {
			t.Fatalf("skill dueReviews %d", skill.DueReviews)
		}
	}
	if got.CardsDueNow == nil || got.CardsDueNow.Total != 11 || got.CardsDueNow.New != 10 || got.CardsDueNow.Learning != 1 || got.CardsDueNow.Review != 0 || got.CardsDueNow.Relearning != 0 {
		t.Fatalf("cardsDueNow %+v", got.CardsDueNow)
	}
	if got.CardsDueNow.Total != len(listed) {
		t.Fatalf("cardsDueNow.total %d ListDueCards %d", got.CardsDueNow.Total, len(listed))
	}

	omitted := mustOK(t, e.session, "get_context", map[string]any{"position": 0})
	if decodeInto[struct {
		CardsDueNow *struct{} `json:"cardsDueNow"`
	}](t, omitted).CardsDueNow != nil {
		t.Fatalf("omitted learner %s", omitted)
	}
}
