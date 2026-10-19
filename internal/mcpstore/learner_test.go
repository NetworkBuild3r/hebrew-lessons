package mcpstore

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestFactsFromBrief(t *testing.T) {
	used := int64(41)
	br := brief.Brief{
		ID:           "AAAAAAAAAAAAAAAAAAAAAA",
		UsedByUnitID: &used,
		Plan: brief.Plan{
			V:    brief.Version,
			Tier: brief.TierFor(10),
			NewWords: []brief.NewWord{
				{Word: brief.Word{LexemeID: 1, Text: "a"}, MinUses: 4},
				{Word: brief.Word{LexemeID: 2, Text: "b"}},
			},
			StruggleWords:    []brief.StruggleWord{{Word: brief.Word{LexemeID: 3, Text: "c"}, Skill: "read", RequiredCount: 4}},
			ComfortableWords: []brief.Word{{LexemeID: 4, Text: "d"}},
			Targets:          brief.Targets{Coverage: 0.95, NewWordMinUses: 3},
		},
	}
	known := map[int64]bool{5: true}
	f := factsFromBrief(br, known)
	if f.Tier != "beginner" || f.MinKnownShare != 0.60 || f.MaxTextTokens != 20 {
		t.Fatalf("the beginner tier is not carried: %+v", f)
	}
	if !f.Used || f.UsedByUnitID != 41 {
		t.Fatalf("used: %+v", f)
	}
	if len(f.NewWords) != 2 || f.NewWords[0].MinUses != 4 || f.NewWords[1].MinUses != 3 {
		t.Fatalf("new words %+v", f.NewWords)
	}
	if len(f.StruggleWords) != 1 || f.StruggleWords[0].Skill != "read" || f.StruggleWords[0].RequiredCount != 4 {
		t.Fatalf("struggle words %+v", f.StruggleWords)
	}
	for _, id := range []int64{1, 2, 3, 4} {
		if !f.BriefLexemeIDs[id] {
			t.Fatalf("brief ids %v lack %d", f.BriefLexemeIDs, id)
		}
	}
	if f.BriefLexemeIDs[5] || !f.KnownLexemeIDs[5] {
		t.Fatalf("known ids are not the brief's: %v %v", f.BriefLexemeIDs, f.KnownLexemeIDs)
	}

	// The staged figure wins over the flat target. With no tier figure the flat
	// target is used, and an unused brief is not Used.
	br.Tier = brief.Tier{Name: "established", MinKnownShare: 0.95}
	br.UsedByUnitID = nil
	if f := factsFromBrief(br, nil); f.MinKnownShare != 0.95 || f.Used || f.UsedByUnitID != 0 || f.MaxTextTokens != 0 {
		t.Fatalf("established: %+v", f)
	}
	br.Tier = brief.Tier{Name: "odd"}
	br.Targets.Coverage = 0.9
	if f := factsFromBrief(br, nil); f.MinKnownShare != 0.9 {
		t.Fatalf("flat target fallback: %+v", f)
	}
}

func TestBriefFactsErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	raw, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock{now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	store := New(raw)
	a, err := raw.InsertUser(ctx, "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	b, err := raw.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(brief.Plan{V: brief.Version, Tier: brief.TierFor(0), Targets: brief.Targets{Coverage: 0.95, NewWordMinUses: 3}})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id string, user int64, created time.Time) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, `INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, 'general', ?, ?)`,
			id, user, string(payload), db.FormatTime(created)); err != nil {
			t.Fatal(err)
		}
	}
	insert("AAAAAAAAAAAAAAAAAAAAAA", a, now.Add(-time.Hour))
	insert("BBBBBBBBBBBBBBBBBBBBBQ", b, now.Add(-time.Hour))
	insert("CCCCCCCCCCCCCCCCCCCCCQ", a, now.Add(-brief.TTL-time.Second))

	got, err := store.BriefFacts(ctx, a, "AAAAAAAAAAAAAAAAAAAAAA")
	if err != nil || got.Tier != "beginner" || got.Used || len(got.KnownLexemeIDs) != 0 {
		t.Fatalf("own brief %+v %v", got, err)
	}
	for _, c := range []struct {
		name string
		user int64
		id   string
		want error
	}{
		{"another learner's brief", a, "BBBBBBBBBBBBBBBBBBBBBQ", mcp.ErrBriefNotFound},
		{"the same brief for its owner is not found for a stranger", b, "AAAAAAAAAAAAAAAAAAAAAA", mcp.ErrBriefNotFound},
		{"a malformed id", a, "x", mcp.ErrBriefNotFound},
		{"a missing id", a, "DDDDDDDDDDDDDDDDDDDDDQ", mcp.ErrBriefNotFound},
		{"no learner", 0, "AAAAAAAAAAAAAAAAAAAAAA", mcp.ErrBriefNotFound},
		{"an expired brief", a, "CCCCCCCCCCCCCCCCCCCCCQ", mcp.ErrBriefExpired},
	} {
		if _, err := store.BriefFacts(ctx, c.user, c.id); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	// A brief that built a unit comes back Used, not as an error.
	if _, err := conn.ExecContext(ctx, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope) VALUES (0, 't', 's', 'c', 'draft', 1, '{}', 'learner:1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE lesson_briefs SET used_by_unit_id = 1 WHERE id = 'AAAAAAAAAAAAAAAAAAAAAA'`); err != nil {
		t.Fatal(err)
	}
	got, err = store.BriefFacts(ctx, a, "AAAAAAAAAAAAAAAAAAAAAA")
	if err != nil || !got.Used || got.UsedByUnitID != 1 {
		t.Fatalf("used brief %+v %v", got, err)
	}
}
