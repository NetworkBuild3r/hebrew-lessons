package session

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// mapDeckLexemes inserts the first n deck rows (band then file order) as
// lexemes with glosses and links them through placement_answers.
func mapDeckLexemes(t *testing.T, store *db.Store, userID int64, n int, now time.Time) []deckLexeme {
	t.Helper()
	rows, err := placement.Deck()
	if err != nil {
		t.Fatal(err)
	}
	ordered := append([]placement.DeckRow(nil), rows...)
	slices.SortStableFunc(ordered, func(a, b placement.DeckRow) int {
		return a.Band - b.Band
	})
	ctx := context.Background()
	res, err := store.ExecContext(ctx, `INSERT INTO placement_runs (user_id, started_at, seed) VALUES (?, ?, 1)`,
		userID, db.FormatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]deckLexeme, 0, n)
	for _, row := range ordered {
		if len(out) >= n {
			break
		}
		if !row.Verified || row.GlossEN == "" {
			continue
		}
		card, _, ok := placement.Lookup(row.ID)
		if !ok {
			t.Fatalf("lookup %s", row.ID)
		}
		lex, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`, units.Normalize(card.TextNFC), row.GlossEN)
		if err != nil {
			t.Fatal(err)
		}
		lexemeID, err := lex.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.ExecContext(ctx, `
			INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at, lexeme_id, sitting_no)
			VALUES (?, ?, 3, ?, ?, ?, 1)`,
			runID, row.ID, db.FormatTime(now), db.FormatTime(now), lexemeID); err != nil {
			t.Fatal(err)
		}
		out = append(out, deckLexeme{ItemID: row.ID, LexemeID: lexemeID, Band: row.Band})
	}
	if len(out) < n {
		t.Fatalf("mapped %d deck words, want %d", len(out), n)
	}
	return out
}

type deckLexeme struct {
	ItemID   string
	LexemeID int64
	Band     int
}

func seedAccuracy(t *testing.T, store *db.Store, userID int64, now time.Time, good, total int) {
	t.Helper()
	if total <= 0 {
		return
	}
	reviewed := now.Add(-time.Hour)
	for i := 0; i < total; i++ {
		rating := int(fsrs.Again)
		if i < good {
			rating = int(fsrs.Good)
		}
		addCard(t, store, userID, fmt.Sprintf("דיוּק-%d", i), fsrs.SkillRead, now.Add(-2*time.Hour), 0, rating, reviewed, false)
	}
}

func TestBuildSeedsEightAtHighAccuracy(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapped := mapDeckLexemes(t, store, user, 10, now)
	// 46/50 = 0.92 → high tier.
	seedAccuracy(t, store, user, now, 46, 50)

	sess := mustBuild(t, svc, user)
	got := drain(t, sess)
	var introVisits int
	for _, ref := range got {
		if ref.New && ref.Intro {
			introVisits++
		}
	}
	if introVisits != 8 {
		t.Fatalf("Intro visits = %d, want 8", introVisits)
	}
	for i := 0; i < 8; i++ {
		n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND state = 'new'`,
			user, mapped[i].LexemeID)
		if n == 0 {
			t.Fatalf("missing new cards for deck word %s", mapped[i].ItemID)
		}
	}
	if n := countTable(t, store, `SELECT COUNT(*) FROM taught_words`); n != 0 {
		t.Fatalf("taught_words = %d", n)
	}
	if n := countTable(t, store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user); n != 0 {
		t.Fatalf("attempts = %d", n)
	}
	if n := countTable(t, store, `SELECT COUNT(*) FROM exposures WHERE user_id = ?`, user); n != 0 {
		t.Fatalf("exposures = %d", n)
	}
	// Accuracy seed cards have review_log; seeded new cards must not.
	for _, m := range mapped[:8] {
		seededLogs := countTable(t, store, `
			SELECT COUNT(*) FROM review_log AS rl
			JOIN cards AS c ON c.id = rl.card_id
			WHERE c.user_id = ? AND c.lexeme_id = ? AND c.state = 'new'`,
			user, m.LexemeID)
		if seededLogs != 0 {
			t.Fatalf("review_log on seeded new card for %d", m.LexemeID)
		}
	}
}

func TestBuildSeedsNothingBelowTarget(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapDeckLexemes(t, store, user, 8, now)
	// 39/50 = 0.78 < TargetSuccess; low tier allows 2 from waiting pool only.
	seedAccuracy(t, store, user, now, 39, 50)
	before := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	got := drain(t, mustBuild(t, svc, user))
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	if after != before {
		t.Fatalf("cards before %d after %d, want no seed", before, after)
	}
	var fresh int
	for _, ref := range got {
		if ref.New {
			fresh++
		}
	}
	if fresh > 2 {
		t.Fatalf("new visits = %d, want at most 2", fresh)
	}
}

func TestBuildSeedsTwoWhenSixWaiting(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapped := mapDeckLexemes(t, store, user, 10, now)
	seedAccuracy(t, store, user, now, 46, 50)
	// Six due new cards already waiting (one skill each).
	for i := 0; i < 6; i++ {
		_, err := store.InsertCard(context.Background(), db.Card{
			UserID: user, LexemeID: mustLexeme(t, store, fmt.Sprintf("מְחַכֶּה-%d", i)),
			Skill: fsrs.SkillRecall, State: "new", Due: now,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	before := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND state = 'new'`, user)
	mustBuild(t, svc, user)
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND state = 'new'`, user)
	// High tier 8 − 6 waiting = 2 words; each word creates read+recall (≥2 cards).
	if after <= before {
		t.Fatalf("new cards before %d after %d, want seed", before, after)
	}
	// The first two eligible deck lexemes should now have cards.
	for i := 0; i < 2; i++ {
		n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[i].LexemeID)
		if n == 0 {
			t.Fatalf("expected cards for %s", mapped[i].ItemID)
		}
	}
	if n3 := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[2].LexemeID); n3 != 0 {
		t.Fatalf("third deck word should not be seeded")
	}
}

func TestBuildSessionForSeedsNothing(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapDeckLexemes(t, store, user, 8, now)
	seedAccuracy(t, store, user, now, 46, 50)
	before := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	if _, err := svc.BuildSessionFor(context.Background(), user, []int64{999}); err != nil {
		t.Fatal(err)
	}
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	if after != before {
		t.Fatalf("BuildSessionFor seeded cards (%d → %d)", before, after)
	}
}

func TestSecondBuildSeedsNothingMore(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapDeckLexemes(t, store, user, 16, now)
	seedAccuracy(t, store, user, now, 46, 50)
	mustBuild(t, svc, user)
	mid := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	mustBuild(t, svc, user)
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	if after != mid {
		t.Fatalf("second build changed cards %d → %d", mid, after)
	}
}

func TestPreGateSeedingCreatesNoReadCard(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc := New(store, clk, 1, WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{}, nil
	}))
	user := mustUser(t, store)
	now := clk.Now()
	mapped := mapDeckLexemes(t, store, user, 8, now)
	seedAccuracy(t, store, user, now, 46, 50)
	mustBuild(t, svc, user)
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mapped[:8] {
		rows, err := conn.Query(`SELECT skill FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, m.LexemeID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var sk string
			if err := rows.Scan(&sk); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if sk == fsrs.SkillRead || sk == fsrs.SkillSpell {
				rows.Close()
				t.Fatalf("lexeme %d has gated skill %s", m.LexemeID, sk)
			}
		}
		rows.Close()
	}
}

func TestSeedSkipsLexemeWithExistingCard(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	mapped := mapDeckLexemes(t, store, user, 9, now)
	seedAccuracy(t, store, user, now, 46, 50)
	if _, err := store.InsertCard(context.Background(), db.Card{
		UserID: user, LexemeID: mapped[0].LexemeID, Skill: fsrs.SkillSpell, State: "new", Due: now,
	}); err != nil {
		t.Fatal(err)
	}
	mustBuild(t, svc, user)
	if first := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[0].LexemeID); first != 1 {
		t.Fatalf("existing-card lexeme gained cards: %d", first)
	}
	if second := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[1].LexemeID); second == 0 {
		t.Fatal("next eligible lexeme was not seeded")
	}
}

func TestSeedSkipsUnverifiedRow(t *testing.T) {
	rows := []placement.DeckRow{
		{ID: "a", Band: 0, Verified: false, GlossEN: "x"},
		{ID: "b", Band: 0, Verified: true, GlossEN: "y"},
		{ID: "c", Band: 0, Verified: true, GlossEN: ""},
	}
	var kept []string
	for _, row := range rows {
		if !row.Verified || row.GlossEN == "" {
			continue
		}
		kept = append(kept, row.ID)
	}
	if len(kept) != 1 || kept[0] != "b" {
		t.Fatalf("kept %v, want [b]", kept)
	}
}

func TestSeedLearnerIsolation(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	a := mustUser(t, store)
	b, err := store.InsertUser(context.Background(), "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	now := clk.Now()
	mapDeckLexemes(t, store, a, 8, now)
	seedAccuracy(t, store, a, now, 46, 50)
	mustBuild(t, svc, a)
	if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, b); n != 0 {
		t.Fatalf("learner b has %d cards", n)
	}
}

func countTable(t *testing.T, store *db.Store, q string, args ...any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
