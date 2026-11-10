package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// A word whose clip exists only in audio_assets is asked as listening and the
// item plays that clip. A listen card with no clip is left out of the queue.
func TestReviewAsksListenCardWhoseClipIsOnlyInAudioAssets(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	user := mustUser(t, store)
	now := clk.Now()

	const text = "שָׁלוֹם"
	clip := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: clip, TextNFC: text, Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	lex := mustLexeme(t, store, text)
	bare := mustLexeme(t, store, "אֵין")
	// Gloss tokens must not overlap, or listen_pick has no fair distractor.
	setGloss(t, store, lex, "peace")
	setGloss(t, store, bare, "none")
	for _, pair := range []struct{ text, gloss string }{
		{"מַיִם", "water"},
		{"בַּיִת", "house"},
		{"סֵפֶר", "book"},
	} {
		setGloss(t, store, mustLexeme(t, store, pair.text), pair.gloss)
	}

	addDue := func(lexeme int64, skill string) int64 {
		t.Helper()
		id, err := store.InsertCard(ctx, db.Card{
			UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateNew,
			Due: now.Add(-time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	wantID := addDue(lex, fsrs.SkillListen)
	addDue(bare, fsrs.SkillListen)

	sess, err := svc.BuildSessionFor(ctx, user, []int64{lex, bare})
	if err != nil {
		t.Fatal(err)
	}
	if sess.SkippedNoAudio != 1 {
		t.Fatalf("skipped = %d, want the clipless listen card", sess.SkippedNoAudio)
	}
	ref, ok := sess.Next()
	if !ok || ref.CardID != wantID || ref.Skill != fsrs.SkillListen || ref.Activity != activity.ListenPick {
		t.Fatalf("asked %+v ok=%v, want listen card %d as listen_pick", ref, ok, wantID)
	}
	if _, more := sess.Next(); more {
		t.Fatal("review asked another card")
	}

	eng := activity.New(store, clk, nil, activity.WithSeed(1))
	item, err := eng.Build(ctx, user, lex, activity.ListenPick, activity.WithSource(evidence.SourceReview))
	if err != nil {
		t.Fatal(err)
	}
	if item.Prompt.AudioID != clip || item.Skill != fsrs.SkillListen {
		t.Fatalf("played %+v, want clip %s", item.Prompt, clip)
	}
}

func setGloss(t *testing.T, store *db.Store, id int64, gloss string) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(),
		`UPDATE lexemes SET gloss_en = ? WHERE id = ?`, gloss, id); err != nil {
		t.Fatal(err)
	}
}
