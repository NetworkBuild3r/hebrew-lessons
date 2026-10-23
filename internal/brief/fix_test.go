package brief

import (
	"context"
	"fmt"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func TestBriefCarriesThreeFixMistakes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i, times := range []int{1, 5, 3, 2} {
		row := db.MistakeInput{
			Wrong: fmt.Sprintf("wrong-%d", i), Right: fmt.Sprintf("right-%d", i),
			Note: "contrast", Source: "app",
		}
		for n := 0; n < times; n++ {
			if err := e.store.RecordMistakes(ctx, e.user, []db.MistakeInput{row}); err != nil {
				t.Fatal(err)
			}
		}
	}
	br := e.build("")
	if len(br.FixMistakes) != 3 {
		t.Fatalf("fixMistakes = %d, want 3", len(br.FixMistakes))
	}
	if br.FixMistakes[0].Wrong != "wrong-1" || br.FixMistakes[0].Right != "right-1" || br.FixMistakes[0].Note != "contrast" {
		t.Fatalf("first = %#v", br.FixMistakes[0])
	}
	if br.FixMistakes[1].Wrong != "wrong-2" || br.FixMistakes[2].Wrong != "wrong-3" {
		t.Fatalf("order = %#v", br.FixMistakes)
	}
}
