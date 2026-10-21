package session

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// BuildSessionFor is the review set the band-slip alert on Today links to
// (INIT-015/SPEC-017): the cards of the words asked for, and no drill.
func TestBuildSessionForKeepsOnlyThoseWords(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	user := mustUser(t, store)
	now := clk.Now()
	var lex [4]int64
	for i, text := range []string{"אַ", "בַּ", "גַּ", "דַּ"} {
		id := addCard(t, store, user, text, fsrs.SkillRead, now.Add(-time.Duration(i+1)*time.Hour), 0, int(fsrs.Good), now.Add(-48*time.Hour), false)
		card, err := store.GetCard(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		lex[i] = card.LexemeID
	}

	all := mustBuild(t, svc, user)
	if _, total, _ := all.Place(); total != 4 {
		t.Fatalf("the whole queue holds %d cards, want 4", total)
	}

	set, err := svc.BuildSessionFor(ctx, user, []int64{lex[1], lex[3]})
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for {
		c, ok := set.Next()
		if !ok {
			break
		}
		got[c.LexemeID] = true
		if c.Kind == KindLetter || c.Kind == KindPattern {
			t.Fatalf("a drill in a review set: %+v", c)
		}
	}
	if len(got) != 2 || !got[lex[1]] || !got[lex[3]] {
		t.Fatalf("review set words = %v, want %d and %d", got, lex[1], lex[3])
	}

	none, err := svc.BuildSessionFor(ctx, user, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, total, _ := none.Place(); total != 0 {
		t.Fatalf("an empty set holds %d cards", total)
	}
	stranger, err := svc.BuildSessionFor(ctx, user, []int64{999999})
	if err != nil {
		t.Fatal(err)
	}
	if _, total, _ := stranger.Place(); total != 0 {
		t.Fatalf("a word with no card holds %d cards", total)
	}
}

// The band review uses the service seed for its miss gap, the same seed
// BuildSession would use.
func TestBuildSessionForUsesTheServiceSeed(t *testing.T) {
	const seed int64 = 1
	svc, store, clk := openSvc(t, seed)
	if svc.Seed() != seed {
		t.Fatalf("service seed = %d, want %d", svc.Seed(), seed)
	}
	ctx := context.Background()
	user := mustUser(t, store)
	now := clk.Now()
	lex := make([]int64, 8)
	for i := range lex {
		id := addCard(t, store, user, "seed-"+string(rune('a'+i)), fsrs.SkillRead, now.Add(-time.Duration(i+1)*time.Hour), 0, int(fsrs.Good), now.Add(-48*time.Hour), false)
		card, err := store.GetCard(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		lex[i] = card.LexemeID
	}
	sess, err := svc.BuildSessionFor(ctx, user, lex)
	if err != nil {
		t.Fatal(err)
	}
	again := measureAgain(t, sess)
	want := requeueSpan(svc.Seed())
	if again.appearances != 2 || again.between != want {
		t.Fatalf("band miss = %+v, want 2 appearances %d apart", again, want)
	}
}
