package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestDefaultPaceFillsTheBudget(t *testing.T) {
	cards, minutes := CardsForPace(DefaultSecondsPerAnswer)
	if cards != SessionCap {
		t.Fatalf("default cards = %d, want SessionCap %d", cards, SessionCap)
	}
	if minutes < SessionMinMinutes || minutes > SessionMaxMinutes {
		t.Fatalf("default minutes = %d, want %d..%d", minutes, SessionMinMinutes, SessionMaxMinutes)
	}
	svc, store, _ := openSvc(t, 1)
	userID := mustUser(t, store)
	sess := mustBuild(t, svc, userID)
	if sess.Minutes() != minutes {
		t.Fatalf("minutes = %d, want %d", sess.Minutes(), minutes)
	}
}

func TestMedianPaceStaysInsideTheBudget(t *testing.T) {
	const seconds = 12.0
	cards, minutes := CardsForPace(seconds)
	if minutes < SessionMinMinutes || minutes > SessionMaxMinutes {
		t.Fatalf("planned minutes = %d at %.0fs, want %d..%d", minutes, seconds, SessionMinMinutes, SessionMaxMinutes)
	}
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	ctx := context.Background()
	lex := mustLexeme(t, store, "קֶצֶב")
	for i := 0; i < AnswerPaceWindow; i++ {
		at := clk.Now().Add(-time.Duration(AnswerPaceWindow-i) * time.Second)
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at)
			VALUES (?, ?, 'read', 'read_pick', 'review', 'p', 'r', 1, ?, ?)`,
			userID, lex, int(seconds*1000), db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < cards+5; i++ {
		due := clk.Now().Add(-time.Duration(i+1) * time.Minute)
		addCard(t, store, userID, fmt.Sprintf("pace-%03d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	}
	sess := mustBuild(t, svc, userID)
	if sess.Minutes() != minutes {
		t.Fatalf("minutes = %d, want %d", sess.Minutes(), minutes)
	}
	n := 0
	for {
		if _, ok := sess.Next(); !ok {
			break
		}
		n++
		if n > cards+MaxNewPerSession+DrillCap {
			t.Fatalf("queue did not end at %d", n)
		}
	}
	if n != cards {
		t.Fatalf("queue = %d, want %d cards at %.0fs", n, cards, seconds)
	}
}

func TestFastPaceStaysUnderTheCeiling(t *testing.T) {
	cards, minutes := CardsForPace(2)
	if cards > MaxCardsPerSitting || cards >= 100 {
		t.Fatalf("cards = %d, ceiling %d", cards, MaxCardsPerSitting)
	}
	if minutes != SessionTargetMinutes {
		t.Fatalf("minutes = %d, want %d", minutes, SessionTargetMinutes)
	}
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	ctx := context.Background()
	lex := mustLexeme(t, store, "מַהֵר")
	for i := 0; i < AnswerPaceWindow; i++ {
		at := clk.Now().Add(-time.Duration(AnswerPaceWindow-i) * time.Second)
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at)
			VALUES (?, ?, 'read', 'read_pick', 'review', 'p', 'r', 1, ?, ?)`,
			userID, lex, 2000, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	reviewed := clk.Now().Add(-48 * time.Hour)
	const due = 200
	for i := 0; i < due; i++ {
		at := clk.Now().Add(-time.Duration(i+1) * time.Minute)
		addCard(t, store, userID, fmt.Sprintf("fast-%03d", i), fsrs.SkillRead, at, 0, int(fsrs.Good), reviewed, false)
	}
	sess := mustBuild(t, svc, userID)
	n := 0
	for {
		if _, ok := sess.Next(); !ok {
			break
		}
		n++
		if n > MaxCardsPerSitting {
			t.Fatalf("queue passed %d", MaxCardsPerSitting)
		}
	}
	if n != cards {
		t.Fatalf("queue = %d, want %d", n, cards)
	}
}

func TestShortDueListStaysShort(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	const due = 3
	for i := 0; i < due; i++ {
		at := clk.Now().Add(-time.Duration(i+1) * time.Minute)
		addCard(t, store, userID, fmt.Sprintf("short-%d", i), fsrs.SkillRead, at, 0, int(fsrs.Good), reviewed, false)
	}
	sess := mustBuild(t, svc, userID)
	n := 0
	for {
		if _, ok := sess.Next(); !ok {
			break
		}
		n++
	}
	if n != due {
		t.Fatalf("queue = %d, want %d", n, due)
	}
}

func TestProductionBeforeMeaningPick(t *testing.T) {
	now := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)
	facts := activity.Facts{HasGloss: true, HasAudio: true, GlossedOthers: 40, VoicedOthers: 40}
	// Meaning pick is due soonest, so the old order would ask it first.
	read := knownCand(1, now.Add(-3*time.Hour))
	read.LexemeID = 7
	read.Facts = facts
	recall := knownCand(2, now.Add(-time.Hour))
	recall.LexemeID = 7
	recall.Skill = fsrs.SkillRecall
	recall.Facts = facts
	spell := knownCand(3, now.Add(-2*time.Hour))
	spell.LexemeID = 7
	spell.Skill = fsrs.SkillSpell
	spell.Facts = facts
	got := mix([]Candidate{read, recall, spell}, TargetSuccess, now, 1)
	if len(got) == 0 {
		t.Fatal("empty queue")
	}
	if got[0].Activity != activity.TypeIt && got[0].Activity != activity.Dictation {
		t.Fatalf("first = %s through %s, want type_it or dictation", got[0].Activity, got[0].AskSkill)
	}
	for _, ref := range got {
		if ref.LexemeID == 7 && (ref.Activity == activity.ReadPick || ref.Activity == activity.ListenPick) {
			return
		}
	}
}
