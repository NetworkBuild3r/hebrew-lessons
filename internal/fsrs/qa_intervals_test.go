package fsrs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// TestQADesiredRetentionAndGoodIntervals uses the injected clock.Clock.
// testharness.FixedClock is that same interface. This package cannot import
// testharness, because testharness imports app and app imports fsrs.
func TestQADesiredRetentionAndGoodIntervals(t *testing.T) {
	rev, store, clk := openReview(t)
	if rev.sched.RequestRetention != 0.9 {
		t.Fatalf("desired retention = %v, want 0.9", rev.sched.RequestRetention)
	}
	ctx := context.Background()
	cardID := mustCard(t, store, db.Card{
		UserID:   mustUser(t, store, "learner@example.com"),
		LexemeID: mustLexeme(t, store, "שָׁלוֹם"),
		Skill:    SkillRead,
		State:    StateNew,
		Due:      clk.Now(),
	})

	var intervals []time.Duration
	var scheduled []int
	var prior []db.ReviewLog
	for i := 0; i < 4; i++ {
		if i > 0 {
			card, err := rev.Card(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			clk.t = card.Due
		}
		now := clk.Now()
		if err := rev.Grade(ctx, cardID, Good, false); err != nil {
			t.Fatalf("good %d: %v", i+1, err)
		}
		card, err := rev.Card(ctx, cardID)
		if err != nil {
			t.Fatal(err)
		}
		gap := card.Due.Sub(now)
		intervals = append(intervals, gap)
		logs, err := store.ReviewLogs(ctx, cardID)
		if err != nil {
			t.Fatal(err)
		}
		if len(logs) != i+1 {
			t.Fatalf("log rows = %d, want %d", len(logs), i+1)
		}
		last := logs[i]
		if last.CardID != cardID || last.Rating != int(Good) || !last.ReviewedAt.Equal(now) {
			t.Fatalf("log %d = %+v, want card %d Good at %s", i, last, cardID, now)
		}
		if last.ScheduledDays < 0 || last.ElapsedDays < 0 {
			t.Fatalf("log %d elapsed/scheduled = %d/%d", i, last.ElapsedDays, last.ScheduledDays)
		}
		scheduled = append(scheduled, last.ScheduledDays)
		for j, prev := range prior {
			got := logs[j]
			if got.ID != prev.ID || got.CardID != prev.CardID || got.Rating != prev.Rating ||
				!got.ReviewedAt.Equal(prev.ReviewedAt) || got.ElapsedDays != prev.ElapsedDays ||
				got.ScheduledDays != prev.ScheduledDays || got.Peeked != prev.Peeked || got.Chosen != prev.Chosen {
				t.Fatalf("review_log row %d changed from %+v to %+v", j, prev, got)
			}
		}
		prior = append([]db.ReviewLog(nil), logs...)
	}
	for i := 1; i < len(intervals); i++ {
		if intervals[i] <= intervals[i-1] {
			t.Fatalf("Good intervals = %v, want each one longer than the last", intervals)
		}
		if scheduled[i] <= scheduled[i-1] {
			t.Fatalf("scheduled days = %v, want each one longer than the last", scheduled)
		}
	}
}

func TestQALapseShortensInterval(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	cardID := mustCard(t, store, db.Card{
		UserID:   mustUser(t, store, "learner@example.com"),
		LexemeID: mustLexeme(t, store, "בֹּקֶר"),
		Skill:    SkillRead,
		State:    StateNew,
		Due:      clk.Now(),
	})
	for i := 0; i < 2; i++ {
		if i > 0 {
			card, err := rev.Card(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			clk.t = card.Due
		}
		if err := rev.Grade(ctx, cardID, Good, false); err != nil {
			t.Fatal(err)
		}
	}
	before, err := rev.Card(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	pre := before.Due.Sub(before.LastReview)
	if pre <= 0 || before.Lapses != 0 {
		t.Fatalf("pre-lapse card %+v interval %s", before, pre)
	}
	logsBefore, err := store.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	clk.t = before.Due
	now := clk.Now()
	if err := rev.Grade(ctx, cardID, Again, false); err != nil {
		t.Fatal(err)
	}
	after, err := rev.Card(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	next := after.Due.Sub(now)
	if next >= pre {
		t.Fatalf("lapse interval %s, pre-lapse %s", next, pre)
	}
	if after.Lapses != before.Lapses+1 {
		t.Fatalf("lapses = %d, want %d", after.Lapses, before.Lapses+1)
	}
	logs, err := store.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != len(logsBefore)+1 {
		t.Fatalf("rows = %d, want %d", len(logs), len(logsBefore)+1)
	}
	for i, prev := range logsBefore {
		got := logs[i]
		if got.ID != prev.ID || got.Rating != prev.Rating || !got.ReviewedAt.Equal(prev.ReviewedAt) ||
			got.ElapsedDays != prev.ElapsedDays || got.ScheduledDays != prev.ScheduledDays {
			t.Fatalf("earlier log %d changed from %+v to %+v", i, prev, got)
		}
	}
	last := logs[len(logs)-1]
	if last.Rating != int(Again) || !last.ReviewedAt.Equal(now) || last.CardID != cardID {
		t.Fatalf("lapse log %+v", last)
	}
}

func TestQAHardGoodEasyOrder(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	cardID := mustCard(t, store, db.Card{
		UserID:   userID,
		LexemeID: mustLexeme(t, store, "עֶרֶב"),
		Skill:    SkillRead,
		State:    StateNew,
		Due:      clk.Now(),
	})
	for i := 0; i < 2; i++ {
		if i > 0 {
			card, err := rev.Card(ctx, cardID)
			if err != nil {
				t.Fatal(err)
			}
			clk.t = card.Due
		}
		if err := rev.Grade(ctx, cardID, Good, false); err != nil {
			t.Fatal(err)
		}
	}
	base, err := store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if base.State != StateReview {
		t.Fatalf("state = %s, want review before comparing ratings", base.State)
	}
	clk.t = base.Due
	now := clk.Now()
	ids := map[Rating]int64{Hard: cardID}
	for _, rating := range []Rating{Good, Easy} {
		copy := base
		copy.ID = 0
		copy.LexemeID = mustLexeme(t, store, fmt.Sprintf("עֶרֶב-%d", rating))
		ids[rating] = mustCard(t, store, copy)
	}
	gaps := map[Rating]time.Duration{}
	for _, rating := range []Rating{Hard, Good, Easy} {
		if err := rev.Grade(ctx, ids[rating], rating, false); err != nil {
			t.Fatal(err)
		}
		card, err := rev.Card(ctx, ids[rating])
		if err != nil {
			t.Fatal(err)
		}
		gaps[rating] = card.Due.Sub(now)
		logs, err := store.ReviewLogs(ctx, ids[rating])
		if err != nil {
			t.Fatal(err)
		}
		last := logs[len(logs)-1]
		if last.Rating != int(rating) || !last.ReviewedAt.Equal(now) || last.CardID != ids[rating] {
			t.Fatalf("rating %d log %+v", rating, last)
		}
	}
	if !(gaps[Hard] < gaps[Good] && gaps[Good] < gaps[Easy]) {
		t.Fatalf("intervals Hard %s Good %s Easy %s", gaps[Hard], gaps[Good], gaps[Easy])
	}
}

func TestNoTimeNowInScheduler(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for i, line := range strings.Split(string(body), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") {
				continue
			}
			if strings.Contains(line, "time.Now") {
				t.Errorf("%s:%d calls time.Now; scheduling must use clock.Clock", name, i+1)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no scheduler source files")
	}
}

func TestReviewLogSourceIsAppendOnly(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(file))
	checked := 0
	for _, dir := range []string{filepath.Join(root, "fsrs"), filepath.Join(root, "db")} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checked++
			lower := strings.ToLower(string(body))
			for _, phrase := range []string{"update review_log", "delete from review_log", "delete review_log"} {
				if strings.Contains(lower, phrase) {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s contains %q; review_log rows are append-only", rel, phrase)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked == 0 {
		t.Fatal("no source files")
	}
}
