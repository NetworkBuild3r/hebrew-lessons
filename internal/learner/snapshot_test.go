package learner

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Snapshot is one read that gives the Summary, the state of every word with evidence, and the
// known share of every band. It must agree with Summary and List, which read the same data.
func TestSnapshotAgreesWithSummaryAndList(t *testing.T) {
	bands := fakeBands{}
	e := newEnv(t, WithBands(bands))
	a, b, c, d, x, f := e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme(), e.lexeme()
	bands[a], bands[b], bands[c], bands[d] = 2, 2, 2, 1
	e.known(e.user, a, fsrs.SkillListen, 10)
	e.known(e.user, b, fsrs.SkillListen, 30)
	e.attempts(e.user, c, fsrs.SkillListen, "WWW")
	e.known(e.user, d, fsrs.SkillRead, 10)
	e.expose(e.user, x) // Shown, no band.
	_ = f               // No evidence at all.

	ctx := context.Background()
	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap.Summary, summary) {
		t.Fatalf("Snapshot.Summary differs from Summary\n%+v\n%+v", snap.Summary, summary)
	}

	// Words are exactly those with evidence or an exposure.
	if len(snap.Words) != 5 {
		t.Fatalf("%d words, want 5", len(snap.Words))
	}
	if _, ok := snap.Words[f]; ok {
		t.Error("a word with no evidence is in Words")
	}
	// Each word agrees with List, which reads the same evidence.
	page, next, err := e.model.List(ctx, e.user, Filter{Limit: MaxPageSize})
	if err != nil || next != "" {
		t.Fatal(page, next, err)
	}
	seen := 0
	for _, w := range page {
		got, ok := snap.Words[w.LexemeID]
		if !ok {
			if w.State != Unseen {
				t.Errorf("word %d is %s in List and missing from Words", w.LexemeID, w.State)
			}
			continue
		}
		seen++
		w.Text, w.Gloss = "", ""
		if !reflect.DeepEqual(got, w) {
			t.Errorf("word %d differs\nsnapshot: %+v\nlist:     %+v", w.LexemeID, got, w)
		}
	}
	if seen != 5 {
		t.Errorf("%d words matched List, want 5", seen)
	}

	want := [5]BandCount{{Band: 0}, {Band: 1, Words: 1, Known: 1}, {Band: 2, Words: 3, Known: 2}, {Band: 3}, {Band: 4}}
	if snap.Bands != want {
		t.Fatalf("bands = %+v, want %+v", snap.Bands, want)
	}
	if got := snap.Bands[2].Share(); got < 0.666 || got > 0.667 {
		t.Errorf("band 2 share = %v, want 2/3", got)
	}
	if snap.Bands[3].Share() != 0 {
		t.Error("an empty band has a share")
	}
}

// A learner with no data gets a snapshot with no words and every word unseen.
func TestSnapshotWithNoEvidence(t *testing.T) {
	e := newEnv(t)
	e.lexeme()
	e.lexeme()
	snap, err := e.model.Snapshot(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Words) != 0 || snap.Summary.Words[Unseen] != 2 {
		t.Fatalf("snapshot = %+v", snap)
	}
	for _, s := range snap.Summary.Skills {
		if s.Counts[Unseen] != 2 || s.EstimatedKnown != 0 || s.Retention != nil {
			t.Errorf("skill %s = %+v", s.Skill, s)
		}
	}
}

func TestSnapshotRefusesNoLearner(t *testing.T) {
	e := newEnv(t)
	if _, err := e.model.Snapshot(context.Background(), 0); err == nil {
		t.Fatal("Snapshot(0) succeeded")
	}
	var nilModel *Model
	if _, err := nilModel.Snapshot(context.Background(), 1); err == nil {
		t.Fatal("a nil model gave a snapshot")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.model.Snapshot(ctx, e.user); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: err = %v", err)
	}
}

// BenchmarkSnapshot is the cost of one learner in the daily job: one read over 5,000 words and 50,000 attempts.
func BenchmarkSnapshot(b *testing.B) {
	m, user := seedLearner(b, perfWords, perfAttempts)
	ctx := context.Background()
	b.ResetTimer()
	for range b.N {
		if _, err := m.Snapshot(ctx, user); err != nil {
			b.Fatal(err)
		}
	}
}
