package learner

import (
	"context"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestCountReviewSkippedNoAudioUsesClipLookup(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	res, err := e.store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('שָׁלוֹם', 'hello')`)
	if err != nil {
		t.Fatal(err)
	}
	clipped, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.InsertAudio(ctx, db.Asset{
		ID:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TextNFC: "שָׁלוֹם", Voice: "v", Model: "legacy", Bytes: 4, CreatedAt: testNow,
	}); err != nil {
		t.Fatal(err)
	}
	bare := e.lexeme()
	e.card(e.user, clipped, fsrs.SkillListen, 4, 1, int(fsrs.Good))
	e.card(e.user, bare, fsrs.SkillListen, 4, 1, int(fsrs.Good))

	due, err := e.store.ListDueCards(ctx, e.user, testNow)
	if err != nil {
		t.Fatal(err)
	}
	sawClip := false
	for _, c := range due {
		if c.Skill == fsrs.SkillListen && c.LexemeID == clipped {
			sawClip = true
		}
		if c.LexemeID == bare {
			t.Fatal("clipless listen card is due")
		}
	}
	if !sawClip {
		t.Fatal("listen card whose clip is only in audio_assets is not due")
	}
	n, err := e.model.CountReviewSkippedNoAudio(ctx, e.user)
	if err != nil || n != 0 {
		t.Fatalf("skipped = %d, %v; want 0", n, err)
	}
}
